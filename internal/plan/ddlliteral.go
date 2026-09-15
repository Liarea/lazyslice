// SPDX-License-Identifier: Apache-2.0

package plan

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/internal/textsig"
	"github.com/Liarea/lazyslice/mask"
)

// The recreated-DDL literal rule (ARCHITECTURE.md §11.1, amended 2026-09-14,
// T-0134; THREAT_MODEL.md T1).
//
// §11.1 recreates a column's DEFAULT, its CHECK constraints and a generated
// column's expression as the catalog's own text, verbatim. A string literal
// inside one of them therefore crosses into the target exactly as a row value
// does, and until this check existed it crossed unexamined: the 2026-09-09
// review put DEFAULT 'ddl.canary@example.org' on an email column, watched the
// *row* be masked and the *default* survive into the target's pg_attrdef, and
// the run exited 0. A later INSERT that takes the default materialises the
// original address again, so a scan of the rows cannot establish that the
// database artefact holds no sensitive literal (finding 5,
// docs/reviews/2026-09-09/evidence/ddl_default.log).
//
// The rule this implements, stated once:
//
//   - A literal in a **masked column's DEFAULT** is masked through that
//     column's own masker — the same key, category and generator its rows go
//     through, so the default stays a *working* default whose value is the one
//     a row holding that literal would have. Nothing is dropped: the
//     application's first INSERT is the point of the tool (§11.1).
//   - A literal in a **CHECK or generated expression on a masked column** is
//     never rewritten: the predicate and the derivation are the application's,
//     and a masked literal inside one changes what the database accepts or
//     computes. A literal there that a *strong* validator hits is refused at
//     exit 13 naming the object, because leaving it is the leak and rewriting
//     it is not ours to do. A literal no strong validator hits is left alone —
//     `CHECK (status IN ('active','banned'))` on a masked column is a closed
//     value list the masker already honours (mask.Constraints.Checks), and
//     refusing on it would refuse most real schemas over labels that are not
//     personal data.
//   - A literal in **any of the three on an unmasked column** that a strong
//     validator hits is exit 12 at plan naming the object, with the --unmask
//     escape: the operator says in writing, per column and with a reason, that
//     the literal in that column's DDL is not a person's. A column that already
//     carries such an opt-out is not asked twice.
//
// A fourth class was added by the 2026-09-15 red team (checkTypeLiterals): an
// enum's labels and a domain's DEFAULT and CHECK, which §11.1 also recreates
// verbatim and which this pass did not read at all. Neither can be rewritten,
// so both are exit 13 with no rewrite arm — and the escape is
// `--allow-type-literal TYPE=REASON`, §8's per-column opt-out spelt for the one object
// class that is not a column (the T-REDFIX review's fourth finding; before it
// the refusal named --skip-table, which cannot clear it).
//
// A fifth class was added by the 2026-09-15 round-2 red team (R2-08,
// tableDDLLiterals' index loop): every index §11.1 recreates, whose
// pg_get_indexdef text can carry a partial index's WHERE predicate or an
// expression index's key expression. It is judged by the same never-rewritten
// rule a CHECK is — an index predicate is the application's, and internal/load/
// ddl replays it verbatim — so it joins fixedExpression's callers rather than
// gaining a rewrite arm of its own. Before this, internal/plan read no
// pg_index at all (tracker T-0163): such a literal was invisible here and
// reached internal/verify's catalog pass as the *only* look at it, exit 9 with
// the target already loaded rather than exit 12 or 13 before anything was
// dropped.
//
// **"Strong" was email, phone, payment card, IBAN and national_id until the
// 2026-09-15 round-2 red team's R2-07 and R2-09 (amended 2026-09-15, T-0189)**
// — the five value shapes a parser decides rather than a dictionary guesses
// (internal/textsig). Every other category the row pipeline masks that is
// *also* a parse or a shape now runs too — see strongValidators below for the
// full set, for why the argument that had kept the rest out (dictionary-backed
// signals refuse ordinary schemas when run over SQL text full of English
// identifiers) does not reach a *literal*, which is never an identifier, and
// for the one category (credential) that stays out on a different, measured
// argument. THREAT_MODEL.md T1 records the amendment; internal/verify's
// catalog pass is still the second, independent look at the same artefact,
// reading the target's own catalog rather than the source's *Schema.
//
// Where it runs: after checkUniqueDomain, because the masker a default is
// rewritten with must be the one the *rows* are masked with, and the equality
// group and unique-index rules overwrite Decision.Masker (unique.go,
// equality.go). It is still before assemble, so no key has left this stage and
// nothing in the target has been touched — which is all §11.1 requires of a
// refusal raised "at plan".

// strongValidator is one of the value shapes a parser decides. The names
// are the classifier's category names, so a refusal says `email` and not a
// value (THREAT_MODEL.md T4).
type strongValidator struct {
	name string
	ok   func(string) bool
}

// The set was three until the 2026-09-15 red team's A20, which put
// "Alice Anderson, 42 Elm St, SSN 123-45-6789, IBAN GB33BUKB20201555555555"
// into a CHECK on an unmasked column and watched it cross into the target under
// exit 0. THREAT_MODEL.md T1 already stated the name-and-address half of that
// miss and gave the reason — a CHECK is full of English words, so a dictionary
// heuristic over one would refuse ordinary schemas — and that reason does not
// reach the other two: a US Social Security number and a UK National Insurance
// number are strict patterns and an IBAN is a mod-97 checksum, none of them a
// guess. So national_id and the IBAN half of financial_account joined the set,
// and — until the 2026-09-15 round-2 red team below — person_name and address
// stayed out for exactly the reason the document gave.
//
// IBAN could not have joined before this task: textsig.ValidIBAN matched any
// fifteen-to-thirty-four-character run of letters and digits that cleared the
// mod-97 check, which five of pagila's own film titles do. It now requires the
// two ISO 13616 check digits in positions three and four, which every real IBAN
// has and an all-caps title does not.
//
// national_id reads textsig.ValidNationalIDStructured, not the twelve-format
// union ValidNationalID (tracker T-0194, the T-0187 review round's finding 2):
// six of the twelve are a mod-N sum over an otherwise unconstrained digit run
// and clear a random string of the right length far too often for a
// one-occurrence refusal — 25.7% of random 9-digit strings, 11.0% of 11-digit,
// measured — so a single DEFAULT, CHECK or enum-label literal that happened to
// be an ordinary nine-digit reference number had roughly a one-in-four chance
// of refusing an otherwise clean plan at exit 12/13 with no ratio to weigh it
// against. ValidNationalIDStructured is the six formats that also constrain the
// value's shape (a dash, a letter, or (NIR) a fixed length under its own
// mod-97 check), which is what a one-hit refusal needs; internal/verify's own
// strong entries (catalog.go's strongCatalogHit, validators.go's text-family
// strong entry) made the identical change in the same review round, and this
// file was the one place still calling the union. That reasoning stands
// unchanged below: the checksum-only six are still not in this list, and
// neither is Luhn's digits-family arm or national_id's digits-family arm —
// every literal this scanner reads is a Go string built from a quoted SQL
// constant, never a bare integer, so there is no digits-family reading to be
// had here regardless.
//
// **Amended 2026-09-15 (T-0189, the round-2 red team's R2-07 and R2-09):**
// every remaining category the row pipeline masks that is a *parse or a
// shape*, rather than a *guess over anything*, joined the set — network_id,
// online_id, person_name, address and free_text — because the one argument
// that had kept person_name and address out (and, by the same reasoning,
// never let free_text in) is an argument about *identifiers*, not about
// *literals*: "a CHECK is full of English words" is a fact about the column
// names, keywords and operators that surround a literal, none of which this
// scanner ever reads — pipeline.Literals returns the quoted string constants
// alone. A table CHECK carrying `'Aurelio Nakamura-Okonkwo'` or `'1742
// Kestrel Hollow Lane, Ashford VT 05024'` (R2-07), or an enum label, a domain
// CHECK, a domain DEFAULT or a generated expression carrying the same shapes
// plus a special-category sentence (R2-09, the same four object classes
// A4b/A11/A12 already taught this file to read) all crossed into the target
// under exit 0, because the five validators above answer for a parsed shape
// and none of the five newly-admitted categories is one. THREAT_MODEL.md T1's
// sentence that named the three-then-five as the whole list is superseded by
// this amendment; see its own 2026-09-15 entry for what running the
// dictionary validators over a *value* rather than over *SQL text* costs and
// does not cost.
//
// **credential does not join, and this is not an oversight** (the same
// amendment's own review, run against `testdata/torture/`). textsig.
// LooksSecret is the one validator on internal/verify's row-scanning list that
// is not a parse or a dictionary shape at all — an entropy guess over *any*
// string sixteen characters or longer carrying two of {lowercase, uppercase,
// digit} — and a DEFAULT calling nextval embeds exactly that shape by
// construction: the literal this scanner reads out of `nextval('public.
// "AccessCode_id_seq"'::regclass)` is the sequence's own quoted, mixed-case,
// underscore-and-digit-free relation name, and three of the ten real-world
// schemas in testdata/torture/ (calcom, gitlab, discourse) and regression 006
// all refused over exactly that shape — a relation name, never a person's —
// the first time this amendment ran with credential included. The checksum-
// only national_id entries are excluded for the identical reason, on
// different evidence: T-0194's own measured false-accept rate on an
// unconstrained digit run. Running a validator over a literal instead of over
// a whole SQL expression closes the dictionary argument; it does nothing to
// close an argument about a validator's own precision, and both of these stay
// out on that second, unrelated argument.
var strongValidators = []strongValidator{
	{name: string(pipeline.CatEmail), ok: textsig.ValidEmail},
	{name: string(pipeline.CatPhone), ok: textsig.ValidPhone},
	{name: string(pipeline.CatFinancial), ok: textsig.ValidLuhn},
	{name: string(pipeline.CatFinancial), ok: textsig.ValidIBAN},
	{name: string(pipeline.CatNationalID), ok: textsig.ValidNationalIDStructured},
	{name: string(pipeline.CatNetworkID), ok: func(s string) bool { return textsig.ValidIP(s) || textsig.ValidMAC(s) }},
	{name: string(pipeline.CatOnlineID), ok: textsig.ValidURL},
	{name: string(pipeline.CatPersonName), ok: func(s string) bool { return textsig.Dictionary().NameShape(s) }},
	{name: string(pipeline.CatAddress), ok: addressLiteralShape},
	{name: string(pipeline.CatFreeText), ok: func(s string) bool { return textsig.Dictionary().ProseName(s) }},
}

// addressSuffixWords corroborates addressLiteralShape (below): the word, case
// folded, that closes an ordinary postal address line. Deliberately small and
// deliberately literal-only — this is not a gazetteer, it is the one signal
// that told the false positives measured below apart from a real street
// address in every case tried.
var addressSuffixWords = map[string]bool{
	"street": true, "st": true, "avenue": true, "ave": true, "road": true, "rd": true,
	"lane": true, "ln": true, "drive": true, "dr": true, "boulevard": true, "blvd": true,
	"way": true, "court": true, "ct": true, "place": true, "pl": true, "circle": true,
	"cir": true, "terrace": true, "ter": true, "highway": true, "hwy": true,
	"parkway": true, "pkwy": true, "trail": true, "trl": true, "square": true, "sq": true,
	"loop": true, "alley": true, "row": true, "walk": true, "crescent": true,
	"close": true, "grove": true, "parade": true, "crossing": true,
}

// addressLiteralShape corroborates textsig.AddressShape for the one-hit
// refusal this scanner runs over a single DDL literal (the T-0189 fix round's
// finding 2, docs/reviews/2026-09-15-redteam's own review of the round-2
// fixes).
//
// textsig.AddressShape is "a digit somewhere, and at least two words that
// carry a letter" — ARCHITECTURE.md §10's own words for it — which is
// calibrated for internal/verify's second net (validators.go), where it is
// deliberately marked *not* strong: the net asks it of a whole column and
// fails only once at least validatorThreshold of many rows agree, so one
// stray hit costs nothing. A DDL literal gets exactly one look, and this
// scanner had been treating that same shape as a one-hit refusal since T-0189
// widened strongValidators to close R2-07 — which the shape does not survive:
// measured against ordinary CHECK value-list and enum-label text, it also
// hits "Basic 1 user", "Pro 5 users", "tier 2 plus", "level 1 support", "P1
// High Priority", "Top 10 sellers", "Building 4 Lobby" and "version 2 draft" —
// pricing tiers and priority labels, never a person's address — and a hit on
// a masked column is exit 13 with no escape at all, while a hit on an
// unmasked one is exit 12 whose only escape, --unmask, says the column itself
// holds nothing personal rather than that this one literal does not.
//
// The corroboration is the same shape national_id's one-hit entry already
// uses (ValidNationalIDStructured over the twelve-format union, above): ask
// for the feature that is actually diagnostic rather than the loosest one
// that is merely necessary. Almost every real address line carries a
// street-type word — street, avenue, road, lane, drive, and their kin, in
// addressSuffixWords — and none of the false positives above do; R2-07's own
// canary, "1742 Kestrel Hollow Lane, Ashford VT 05024", keeps its hit through
// "Lane". textsig.AddressShape is not touched — internal/textsig is outside
// this task's paths, and internal/verify's second net still wants the looser
// shape it already has, calibrated the way a many-row ratio can afford.
func addressLiteralShape(s string) bool {
	if !textsig.AddressShape(s) {
		return false
	}
	for _, f := range strings.Fields(s) {
		f = strings.Trim(f, ",.;:()\"'")
		if addressSuffixWords[strings.ToLower(f)] {
			return true
		}
	}
	return false
}

// strongHit is the first validator a literal matches, or "".
//
// A pattern operand is detected under a reduced text and never rewritten under
// any (amended 2026-09-15, T-0189, the round-2 red team's R2-10). Pattern is a
// real distinction — the right-hand side of a LIKE or a regex operator is a
// shape and not a value, and testdata/nasty.sql's CHECK ("EmailAddress" LIKE
// '%@%.%') is why this file has always known that: % is an ordinary atext
// character, net/mail parses '%@%.%' as a valid address, and the whole fixture
// refused at exit 12 the first time this check ran. But "is a shape" and "can
// never also carry a value" are different claims, and R2-10 is the case that
// tells them apart: CHECK (email !~ '^ceo@bigcorp\.example$') carries the exact
// address ceo@bigcorp.example and, because the old rule exempted every pattern
// operand from every validator, crossed into the target under exit 0 — while
// the semantically identical CHECK (email <> 'ceo@bigcorp.example') refused at
// exit 13 on the same value written without the anchors. pipeline.
// StripPatternMeta is what tells the two cases apart: it removes the
// metacharacters a pattern reads as syntax (LIKE's %, _; SIMILAR TO's and the
// tilde operators' regular-expression grammar) and unescapes a
// backslash-escaped one to the literal character it stands for, so '%@%.%'
// reduces to "@" — which nothing here validates — and the red team's own regex
// reduces to "ceo@bigcorp.example" intact. A pattern literal is still never
// rewritten: this function only detects, and the caller that would write a
// replacement back into the expression (columnDefault's RewriteLiterals
// callback) still declines a Pattern literal outright, unconditionally, for
// the reason its own comment gives — rewriting one changes what the database
// accepts, where detecting one only decides whether to refuse.
func strongHit(lit pipeline.Literal) string {
	s := strings.TrimSpace(lit.Text)
	if lit.Pattern {
		s = strings.TrimSpace(pipeline.StripPatternMeta(s))
	}
	if s == "" {
		return ""
	}
	for _, v := range strongValidators {
		if v.ok(s) {
			return v.name
		}
	}
	return ""
}

// markerTable is the bookkeeping table of §11.2, which internal/load/ddl does
// not recreate and this check therefore does not read. The name is repeated
// rather than imported: internal/plan may not import another stage package
// (internal/CLAUDE.md), and internal/load/CLAUDE.md records the same constant
// on its side.
const markerTable = "lazyslice_meta"

// checkDDLLiterals runs the rule above over every object §11.1 recreates.
//
// The tables are every non-partition table of the schema, in (schema, name)
// order — not the in-scope ones. §11.1 recreates the DDL of a SchemaOnly table
// too, so its default reaches the target whether or not a row does, and a check
// that skipped it would let --skip-table carry the literal through.
func (p *run) checkDDLLiterals() error {
	if p.cls == nil {
		return nil
	}
	if err := p.checkTypeLiterals(); err != nil {
		return err
	}
	for i := range p.tables {
		t := &p.tables[i]
		if t.Ref.Name == markerTable {
			continue
		}
		if err := p.tableDDLLiterals(t); err != nil {
			return err
		}
	}
	return nil
}

// checkTypeLiterals is the object classes §11.1 recreates that belong to a
// *type* rather than to a table: an enum's labels and a domain's whole CREATE
// statement, which carries its DEFAULT and its CHECK.
//
// It is the 2026-09-15 red team's A4b, A11 and A12, and each of the three is
// one catalog table to the side of where the 2026-09-14 amendment (T-0134)
// looked. internal/load/ddl writes `CREATE TYPE ... AS ENUM ('a','b')` and
// replays a domain's definition verbatim, so a value in either crosses into the
// target exactly as a column DEFAULT does — and this pass read pg_attrdef,
// pg_constraint and pg_index only, so `CREATE TYPE assignee AS ENUM
// ('unassigned','enum.canary@bigcorp.com','+1-415-555-0199')` and
// `CREATE DOMAIN tenant_d AS text DEFAULT 'domdefault.canary@bigcorp.com'` both
// exited 0 with the values in the target's catalog.
//
// Everything here refuses at 13 and nothing here is rewritten, which is the one
// place this file's three-way rule (mask / refuse at 13 / refuse at 12) collapses
// to one outcome:
//
//   - An enum label cannot be rewritten. Every row of every column of that type
//     references the label *by value*, so a masked label either breaks the
//     column or silently remaps rows — and there is no --unmask that makes a
//     label a person's or not a person's, because a label is not a column.
//   - A domain's DEFAULT and CHECK belong to the type. The columns declared
//     over the domain may be masked under different categories, or not masked
//     at all, so there is no single masker whose output would be the right
//     replacement — which is exactly the argument columnDefault makes for a
//     column whose shape it declines.
//
// The refusal names the type and the position of the label, never the label
// text (THREAT_MODEL.md T4).
//
// **The escape is --allow-type-literal TYPE=REASON, and it had to be built for this**
// (the T-REDFIX review's fourth finding). The first version of this check named
// --skip-table, which cannot clear it by any route: --skip-table drops a table
// to *schema only*, so its DDL — and every type that DDL names — is still
// recreated, and nothing in internal/core prunes Schema.Enums or
// Schema.Domains while internal/load/ddl's typeOrder recreates every one of
// them regardless. A source schema with a single enum label or domain
// definition that a strong validator hit was therefore permanently unrunnable,
// under a refusal that told the operator to try a flag with no effect on it.
// The opt-out carries a reason for the same purpose --unmask does, it is
// resolved against the source's own type names by internal/core (so an opt-out
// that could never apply is exit 2 rather than silence), and
// internal/verify's catalog pass honours the same list through
// Plan.AllowedTypeLiterals — an escape one end of the pipeline grants and the other
// refuses is not an escape.
func (p *run) checkTypeLiterals() error {
	names := make([]string, 0, len(p.schema.Enums))
	for name := range p.schema.Enums {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if p.allowedTypeLiteral(name) {
			continue
		}
		for i, label := range p.schema.Enums[name] {
			// A label is the value itself and not an expression, so it is
			// judged directly rather than scanned for literals inside it.
			if hit := strongHit(pipeline.Literal{Text: label}); hit != "" {
				return p.refuseTypeLiteral(name, "label "+strconv.Itoa(i+1), hit)
			}
		}
	}
	domains := append([]pipeline.NamedDef(nil), p.schema.Domains...)
	sort.Slice(domains, func(a, b int) bool { return domains[a].Name < domains[b].Name })
	for _, d := range domains {
		if p.allowedTypeLiteral(d.Name) {
			continue
		}
		for _, lit := range pipeline.Literals(d.Def) {
			if hit := strongHit(lit); hit != "" {
				return p.refuseTypeLiteral(d.Name, "its definition", hit)
			}
		}
	}
	return nil
}

// allowedTypeLiteral reports that the operator has said in writing, with a reason,
// that this type's recreated definition carries no person's value
// (--allow-type-literal TYPE=REASON). The name is the catalog's qualified one, which
// is what internal/core resolves the flag to.
func (p *run) allowedTypeLiteral(name string) bool {
	_, ok := p.req.AllowTypeLiterals[name]
	return ok
}

// allowedTypeNames is the opt-outs that name a type this schema actually
// carries, in name order, for Plan.AllowedTypeLiterals. A name the schema does not
// carry is already exit 2 in internal/core; filtering again here keeps the plan
// from telling internal/verify to exempt an object nobody named.
func (p *run) allowedTypeNames() []string {
	if len(p.req.AllowTypeLiterals) == 0 {
		return nil
	}
	out := make([]string, 0, len(p.req.AllowTypeLiterals))
	for name := range p.req.AllowTypeLiterals {
		if _, ok := p.schema.Enums[name]; ok {
			out = append(out, name)
			continue
		}
		for _, d := range p.schema.Domains {
			if d.Name == name {
				out = append(out, name)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}

// refuseTypeLiteral is exit 13 naming a type. typeRef splits the qualified name
// the catalog recorded so that the refusal reads like every other one.
func (p *run) refuseTypeLiteral(typeName, where, hit string) error {
	t := typeRef(typeName)
	r := refuse(CodeTypeLiteral, exitSchema, t,
		fmt.Sprintf("type %s carries a literal in %s that parses as %s and cannot be rewritten, "+
			"so the target's schema would hold it as the source wrote it", typeName, where, hit),
		event.Args{
			event.ArgTable:  typeName,
			event.ArgColumn: where,
			event.ArgReason: "a literal in " + where + " parses as " + hit +
				" and lazyslice cannot mask a type in place: --allow-type-literal " +
				typeName + "=REASON says it is not a person's",
		})
	r.Column = where
	return r
}

// typeRef splits a catalog-qualified type name into a ref.TableRef so that a
// refusal about a type carries the same two identifiers a refusal about a table
// does. A name with no schema part keeps the whole string as the name.
func typeRef(name string) ref.TableRef {
	if i := strings.LastIndex(name, "."); i > 0 {
		return ref.TableRef{Schema: name[:i], Name: name[i+1:]}
	}
	return ref.TableRef{Name: name}
}

func (p *run) tableDDLLiterals(t *pipeline.Table) error {
	masked := map[string]pipeline.Decision{}
	for _, col := range t.Columns {
		d, ok := p.cls.Decisions[ref.ColumnRef{Table: t.Ref, Column: col.Name}]
		if ok && d.Masked {
			masked[col.Name] = d
		}
	}

	for j := range t.Columns {
		col := t.Columns[j]
		d, isMasked := masked[col.Name]
		if originalDefault(col) != "" {
			if err := p.columnDefault(t, j, col, d, isMasked); err != nil {
				return err
			}
		}
		if col.Generated != "" {
			if err := p.fixedExpression(t, col.Name, col.Generated, isMasked,
				[]string{col.Name}, "the generated expression on"); err != nil {
				return err
			}
		}
	}

	cons := append([]pipeline.Constraint(nil), t.Constraints...)
	sort.Slice(cons, func(a, b int) bool { return cons[a].Name < cons[b].Name })
	for _, con := range cons {
		if con.Kind != 'c' && con.Kind != 'x' {
			continue
		}
		named := namedColumns(con.Def, t)
		if err := p.fixedExpression(t, con.Name, con.Def,
			anyMasked(named, masked), named, "the constraint"); err != nil {
			return err
		}
	}

	// R2-08 (tracker T-0163): every index §11.1 recreates, in name order for
	// the same determinism reason the constraint loop above sorts. idx.Def is
	// pg_get_indexdef's whole text — the index's own name, its columns or
	// expression, and, for a partial or expression index, the WHERE predicate
	// or the key expression that has no pg_constraint row to be found through
	// instead. namedColumns reads whichever of the table's columns idx.Def
	// mentions by token, the same text-based match the constraint loop already
	// trusts for a CHECK's deparsed text, so an index over an unmasked column
	// still gets the --unmask escape and one over a masked column still
	// refuses outright — an index predicate is never rewritten by anything, so
	// the never-rewritten arm of fixedExpression is the whole of the rule
	// here, exactly as it is for a CHECK.
	idxs := append([]pipeline.Index(nil), t.Indexes...)
	sort.Slice(idxs, func(a, b int) bool { return idxs[a].Name < idxs[b].Name })
	for _, idx := range idxs {
		named := namedColumns(idx.Def, t)
		if err := p.fixedExpression(t, idx.Name, idx.Def,
			anyMasked(named, masked), named, "the index"); err != nil {
			return err
		}
	}
	return nil
}

// originalDefault is the catalog's own text of a column's default: Default on a
// column this stage has not rewritten, and Column.DefaultOriginal on one it has.
//
// Everything in this file reads the default through here, so the check is
// **idempotent** on the *pipeline.Schema it is given. The rewrite below writes
// the masked text back over Column.Default, and a second Plan over the same
// in-memory schema — internal/tui re-plans against a cached one, and
// plan_integration_test.go plans twice to compare two plans — would otherwise
// mask the already-masked literal and produce mask(mask(x)): a different default
// on the second plan than on the first, and a DEFAULT whose value is not the one
// a row holding the source's literal gets.
func originalDefault(col pipeline.Column) string {
	if col.DefaultOriginal != "" {
		return col.DefaultOriginal
	}
	return col.Default
}

// columnDefault masks a masked column's default in place, or refuses.
//
// The rewrite is written back through p.byRef, which points into p.schema —
// p.tables holds copies, and internal/load/ddl generates the target's DDL from
// the schema. That is the one thing this stage mutates outside the
// classification, and internal/plan/CLAUDE.md records it.
func (p *run) columnDefault(
	t *pipeline.Table,
	index int,
	col pipeline.Column,
	d pipeline.Decision,
	isMasked bool,
) error {
	def := originalDefault(col)
	lits := pipeline.Literals(def)
	if len(lits) == 0 {
		return nil
	}
	if !isMasked {
		return p.refuseUnmaskedLiteral(t, col.Name, lits, []string{col.Name}, "the default on")
	}

	c, judged := p.constraintsOf(col)
	shapeRewritable := judged && p.defaultIsRewritable(col)

	// No key yet is not the same finding as "cannot be rewritten": every shape
	// this stage declines below is a property of the column and stays wrong
	// however this run is invoked, but a nil Key *with* KeyPending set is
	// internal/core's keyBeforePlan reporting that a plan-only run has none to
	// resolve without creating one (T-0161). KeyPending is read explicitly
	// here rather than inferred from Key == nil alone (2026-09-14 review,
	// finding 1): a writing run always reaches here with a key — core
	// resolves it in full before planStage runs, and sets KeyPending false —
	// so a nil Key on such a run is drift, not the plan-only state, and falls
	// through to the refusal below instead of silently skipping the column.
	// This branch is what a `lazyslice plan` with no lazyslice.secret and no
	// $LAZYSLICE_SECRET takes, and refusing it would make planning impossible
	// before a secret exists at all. The column is reported instead, in the
	// order this pass visits it, so the operator can act.
	if shapeRewritable && p.req.Key == nil && p.req.KeyPending {
		p.pendingKeyDefaults = append(p.pendingKeyDefaults, t.Ref.String()+"."+col.Name)
		return nil
	}
	if !shapeRewritable || p.req.Key == nil {
		// Nothing here can produce a value for this column — either the shape
		// declines it, or (KeyPending false, drift guard) a key was expected
		// and is missing — so a literal that is plainly personal data cannot
		// be shipped and cannot be replaced.
		for _, lit := range lits {
			if hit := strongHit(lit); hit != "" {
				return p.refuseNotRewritable(t, col.Name, "the default on", hit)
			}
		}
		return nil
	}
	c.Unique = uniqueColumn(t, col.Name) || d.UniqueIndex

	var failed error
	out, ok := pipeline.RewriteLiterals(def, func(lit pipeline.Literal) (string, bool) {
		if failed != nil || lit.Pattern || strings.TrimSpace(lit.Text) == "" {
			return "", false
		}
		res, err := mask.Apply(*p.req.Key, mask.Category(d.Category), d.Masker,
			mask.Value{Text: lit.Text}, c)
		if err != nil {
			failed = err
			return "", false
		}
		return res.Out.Text, true
	})
	if failed != nil || !ok {
		for _, lit := range lits {
			if hit := strongHit(lit); hit != "" {
				return p.refuseNotRewritable(t, col.Name, "the default on", hit)
			}
		}
		return nil
	}
	// R2-10 on the DEFAULT path (2026-09-15 round-2 red team's fix round): the
	// callback above declines a Pattern literal and an empty one outright, and
	// RewriteLiterals leaves a declined literal's text exactly as it stood —
	// which closes the rewrite half of "exempt from rewriting only, never from
	// detection" (ddlliteral.go's own package comment) but left the detection
	// half open, because nothing downstream of a successful rewrite ever ran
	// strongHit over what RewriteLiterals left alone. A pattern operand inside a
	// rewritable masked column's DEFAULT — a boolean or CASE default whose
	// pattern operand carries a real address, say — crossed into the target
	// exactly as R2-10 already found for a CHECK, and internal/verify then
	// exempted the DEFAULT outright because DefaultOriginal was set
	// (catalog.go's rewroteDefault), so nothing looked at it a second time
	// either. Every literal this stage declined to rewrite is re-scanned here,
	// against the same strongHit R2-10 already reduces a pattern's text with,
	// before the rewrite is accepted.
	for _, lit := range lits {
		if !lit.Pattern && strings.TrimSpace(lit.Text) != "" {
			continue
		}
		if hit := strongHit(lit); hit != "" {
			return p.refuseNotRewritable(t, col.Name, "the default on", hit)
		}
	}
	p.byRef[t.Ref].Columns[index].Default = out
	p.byRef[t.Ref].Columns[index].DefaultOriginal = def
	t.Columns[index].Default = out
	t.Columns[index].DefaultOriginal = def
	return nil
}

// uniqueColumn reports that the column sits alone under a unique index or is the
// whole primary key — ARCHITECTURE.md §5's "the column is under a unique index",
// which mask.Constraints.Unique carries and which changes what a generator
// emits (an email masker on a unique column emits a hash-derived suffix).
//
// It is a **copy** of internal/transform's function of the same name
// (constraints.go), for the reason constraintsOf above is a copy of transform's
// reduction: a stage package may not import another (internal/CLAUDE.md), and a
// default masked under different Constraints than the column's rows is masked
// under is exactly the defect this rewrite exists to avoid — the default would
// come from the non-unique generator while every row came from the unique one.
// transform ORs Decision.UniqueIndex on top of it (transform.go) and so does the
// caller, because classify's field asks a narrower question than this one does
// (classify.go's own note) and neither answer subsumes the other. The two
// spellings are a cross-package contract with nothing but this comment holding
// them together; **T-0164** owes them a shared home, as T-0162 owes the literal
// scanner one.
func uniqueColumn(t *pipeline.Table, name string) bool {
	if t == nil {
		return false
	}
	if len(t.PK) == 1 && t.PK[0] == name {
		return true
	}
	for _, idx := range t.Indexes {
		if idx.Unique && !idx.Partial && !idx.Expression && len(idx.Columns) == 1 && idx.Columns[0] == name {
			return true
		}
	}
	return false
}

// defaultIsRewritable reports whether a masked column's default is one this
// stage may substitute literals into at all.
//
// Three shapes are declined, and each is declined because a masked literal in
// it would be wrong rather than merely different:
//
//   - An array column. §5 masks an array element-wise; the default of a text[]
//     column is one literal holding the whole array's text form, and masking it
//     as a scalar produces something that is not an array literal at all
//     (testdata/regressions/009 is the row-side version of that mistake).
//   - A json, jsonb or hstore column. §4 replaces a document whole, per leaf;
//     the same argument.
//   - A default calling nextval. Its literal is a sequence name cast to
//     regclass, and a masked sequence name is a relation the target does not
//     have — a CREATE TABLE that fails after every table has been dropped.
//
// Each is a refusal rather than a pass when the literal is plainly personal
// data, which is what the caller does with the false.
func (p *run) defaultIsRewritable(col pipeline.Column) bool {
	name := strings.ToLower(strings.TrimSpace(col.TypeName))
	if strings.HasSuffix(name, "[]") {
		return false
	}
	switch mask.BareTypeName(mask.StripTypmod(name)) {
	case "json", "jsonb", "hstore":
		return false
	}
	return !strings.Contains(strings.ToLower(originalDefault(col)), "nextval(")
}

// fixedExpression is the CHECK and generated-expression half: never rewritten,
// refused at 13 on a masked column and at 12 on an unmasked one.
func (p *run) fixedExpression(
	t *pipeline.Table,
	object, def string,
	onMasked bool,
	named []string,
	what string,
) error {
	lits := pipeline.Literals(def)
	if len(lits) == 0 {
		return nil
	}
	if !onMasked {
		return p.refuseUnmaskedLiteral(t, object, lits, named, what)
	}
	for _, lit := range lits {
		if hit := strongHit(lit); hit != "" {
			return p.refuseNotRewritable(t, object, what, hit)
		}
	}
	return nil
}

// namedColumns is the columns of the table a deparsed definition names, in the
// table's own column order. It is a token match over the catalog's own text,
// which quotes a name that needs quoting and writes a bare one otherwise, so
// both spellings are looked for.
//
// The scan starts after " USING " when the text has one, which an index's
// pg_get_indexdef always does (the access method is never omitted) and a
// table CHECK never does. Skipping to there is deliberate, not cosmetic:
// unlike pg_get_constraintdef, pg_get_indexdef always opens with
// `CREATE [UNIQUE] INDEX name ON schema.table`, so a token match over the
// whole text reads the table's own name as though it were a column whenever
// a column happens to share it — measured: table public.items with columns
// {id, items, email}, index `CREATE INDEX items_vip_idx ON public.items
// USING btree (email) WHERE (email = '...')`, returned named == [items
// email], where "items" is the table, not a column. That false match had two
// consequences downstream: anyMasked flipped true off a phantom column
// (exit 13 with no escape named, where the real hit wanted exit 12 with
// --unmask), and refuseUnmaskedLiteral's opt-out loop cleared a genuine hit
// the moment that phantom column carried an --unmask of its own — a
// different, unrelated column's opt-out silently clearing this one's
// refusal. An exclusion constraint's Def opens `EXCLUDE USING gist (...)`
// and has no table name to begin with, so trimming to it there only drops
// the leading keyword, which was never a column match target either way.
func namedColumns(def string, t *pipeline.Table) []string {
	scan := def
	if i := strings.Index(def, " USING "); i >= 0 {
		scan = def[i+len(" USING "):]
	}
	var out []string
	for _, col := range t.Columns {
		quoted := `"` + strings.ReplaceAll(col.Name, `"`, `""`) + `"`
		if strings.Contains(scan, quoted) || containsWord(scan, col.Name) {
			out = append(out, col.Name)
		}
	}
	return out
}

// anyMasked reports whether any of the named columns is masked.
func anyMasked(named []string, masked map[string]pipeline.Decision) bool {
	for _, name := range named {
		if _, ok := masked[name]; ok {
			return true
		}
	}
	return false
}

// containsWord reports whether def carries name as a whole identifier.
func containsWord(def, name string) bool {
	if name == "" {
		return false
	}
	for at := 0; ; {
		i := strings.Index(def[at:], name)
		if i < 0 {
			return false
		}
		i += at
		before := i == 0 || !identByte(def[i-1])
		end := i + len(name)
		after := end >= len(def) || !identByte(def[end])
		if before && after {
			return true
		}
		at = i + 1
	}
}

func identByte(c byte) bool {
	return c == '_' || c == '$' || c >= 0x80 ||
		(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// refuseUnmaskedLiteral is exit 12: a strong validator hit in the DDL of a
// column this run does not mask. The escape is the per-column --unmask, which
// is the only place in the tool an operator says in writing, with a reason,
// that a column's contents are not a person's.
func (p *run) refuseUnmaskedLiteral(
	t *pipeline.Table,
	object string,
	lits []pipeline.Literal,
	named []string,
	what string,
) error {
	escape := object
	for _, name := range named {
		if p.optedOut(t.Ref, name) {
			return nil
		}
		if escape == object && name != "" {
			escape = name
		}
	}
	for _, lit := range lits {
		hit := strongHit(lit)
		if hit == "" {
			continue
		}
		r := refuse(CodeDDLLiteral, exitPlan, t.Ref,
			fmt.Sprintf("%s %s.%s carries a literal that parses as %s, and the column is not masked, "+
				"so the literal would be recreated in the target as it stands",
				what, t.Ref, object, hit),
			event.Args{
				event.ArgTable:  t.Ref.String(),
				event.ArgColumn: object,
				event.ArgReason: "a literal in " + what + " this column parses as " + hit +
					": --unmask " + t.Ref.String() + "." + escape + "=REASON says it is not a person's",
			})
		r.Column = object
		return r
	}
	return nil
}

// optedOut reports whether the operator has already said in writing that this
// column's contents are not a person's. An opt-out that classify did not honour
// — its type fingerprint moved (THREAT_MODEL.md T3) — is not one, because
// Decision.Source is then still the classifier's.
func (p *run) optedOut(t ref.TableRef, column string) bool {
	d, ok := p.cls.Decisions[ref.ColumnRef{Table: t, Column: column}]
	if !ok {
		return false
	}
	return d.Source == pipeline.ByFlagUnmask || d.Source == pipeline.ByYmlUnmask
}

// refuseNotRewritable is exit 13: the object is inside the data boundary, the
// literal is plainly personal data, and this stage may not rewrite it.
func (p *run) refuseNotRewritable(t *pipeline.Table, object, what, hit string) error {
	r := refuse(CodeLiteralNotRewritable, exitSchema, t.Ref,
		fmt.Sprintf("%s %s.%s carries a literal that parses as %s and cannot be rewritten, "+
			"so the target's schema would hold it as the source wrote it",
			what, t.Ref, object, hit),
		event.Args{
			event.ArgTable:  t.Ref.String(),
			event.ArgColumn: object,
			event.ArgReason: "a literal in " + what + " this column parses as " + hit +
				" and lazyslice cannot mask it in place",
		})
	r.Column = object
	return r
}
