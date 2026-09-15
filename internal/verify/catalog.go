// SPDX-License-Identifier: Apache-2.0

package verify

import (
	"context"
	"fmt"
	"strings"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/internal/textsig"
)

// The catalog pass (ARCHITECTURE.md §6 item 4a, added 2026-09-14 by T-0134;
// THREAT_MODEL.md T1).
//
// Every other check in this package reads the target's *rows*. The 2026-09-09
// review's finding 5 is the case where that is not enough: a masked email
// column whose DEFAULT was 'ddl.canary@example.org' had every row masked, kept
// the default in pg_attrdef, and the run exited 0 — so the database artefact
// held an address that no row scan could ever see, and the application's next
// INSERT would put it back into a row (docs/reviews/2026-09-09 finding 5,
// evidence/ddl_default.log).
//
// So this reads the target's own catalog: pg_attrdef, which holds both the
// column defaults and the generated-column expressions internal/load/ddl wrote,
// pg_constraint, which holds the CHECK and exclusion definitions (a domain's
// CHECK among them, since conrelid is 0 there rather than absent), and pg_index,
// which is where a partial index's predicate and an expression index's key
// expressions live — they have no pg_constraint row, and internal/load/ddl
// replays an index verbatim through pg_get_indexdef; pg_enum, whose labels
// internal/load/ddl writes into `CREATE TYPE ... AS ENUM (...)` as string
// literals; and pg_type.typdefault, where a domain's DEFAULT lives (a domain's
// CHECK is already in pg_constraint, with conrelid 0 rather than absent). The
// last two are the 2026-09-15 red team's A4b, A11 and A12 — an email address and
// a phone number in an enum label, and an address in a domain default, all three
// in the target under a green tick. Every string literal in each text goes
// through strongCatalogHit's validators and a hit is exit 9 naming the table
// and the object.
//
// It is the *second* look at the same boundary, not the first: internal/plan's
// checkDDLLiterals refuses or rewrites before anything is dropped, and this
// reads back what was actually written. The two are deliberately not the same
// code path — plan reads the source's *pipeline.Schema and this reads the
// target's catalog — so a literal that reached the target through a route the
// planner does not walk is still found here: an object somebody added to the
// target by hand, for one. **The index predicate was that route too, until
// T-0189 closed it (tracker T-0163):** internal/plan read no pg_index at all,
// so `CREATE UNIQUE INDEX ... WHERE email = 'x@y.test'` was seen by this pass
// and by nothing else, exit 9 with the target already loaded rather than exit
// 12 or 13 before anything was dropped. internal/plan's tableDDLLiterals now
// walks t.Indexes the same way it walks t.Constraints, so this pass's own
// third read is a genuine second look at that route now, not the only one.
//
// **Every validator runs, not only the five "strong" ones (amended
// 2026-09-15, T-0189, the round-2 red team's R2-07 and R2-09).** The narrowing
// this comment used to state — a CHECK is full of English words and a default
// is full of identifiers, so the dictionary-backed validators would refuse an
// already-loaded target over a column named after a street — is an argument
// about *identifiers*, and pipeline.Literals never returns one: only the
// quoted string constants a deparsed expression carries, which are the
// source's own values wherever they sit. THREAT_MODEL.md T1 records the
// amendment and what it costs; strongCatalogHit's own comment has the full
// validator list and the one category that stays out regardless
// (national_id's checksum-only six, for T-0194's own reason and not this
// one).
//
// And one narrowing that is about *provenance* rather than shape: a column this
// run masked, and a column the operator opted out of, is exempt for its own
// default (catalogExempt). internal/plan masks a masked column's DEFAULT through
// that column's own masker, and the masker's output for an address is another
// address — mask.Apply over 'ddl.canary@example.org' gives a working address in
// the generator's own domain — so without this arm a *correctly masked* default
// is exit 9 on every run, and the rule's central case could never pass. It is
// the same arm netMode has on the row side (secondnet.go: a masked column is not
// the second net's subject either, because its values are the masker's and not
// the source's), and it is narrow in the same way: it exempts the default of the
// masked column itself, never a CHECK, never a generated expression on it, and
// never an index predicate — none of those is ever rewritten, so a strong hit in
// one is still the source's literal.
//
// The masked arm is narrower still, and the review round that asked for it is
// T-0134's own: it exempts the default of a masked column only when the planner
// actually rewrote that default, which it records on Column.DefaultOriginal in
// the very *pipeline.Schema this stage is handed (internal/plan/ddlliteral.go's
// columnDefault; core passes one schema to both stages). "The classification
// says masked" and "the masker's output is what stands in pg_attrdef" are not
// the same claim, and while internal/core does not fill pipeline.PlanRequest.Key
// (tracker T-0161) the second is false for every run: the planner rewrites
// nothing, so an exemption keyed on the first would hold open the one arm of
// this pass that covers a default. Keyed on the rewrite it is closed until the
// rewrite exists and opens exactly where it happened.

// rewroteDefault reports that internal/plan masked this column's DEFAULT during
// this run — the provenance the masked arm of catalogExempt needs, read from the
// schema the planner mutated rather than guessed from the classification.
//
// A column with no recorded original was not rewritten, whatever its decision
// says: either it had no literal to rewrite (and this pass will find none
// either), or the planner could not rewrite it, in which case what stands in the
// target is the source's own text and this pass is the control over it.
func (s *state) rewroteDefault(t ref.TableRef, column string) bool {
	tab, ok := s.tables[t]
	if !ok || tab == nil {
		return false
	}
	for _, c := range tab.Columns {
		if c.Name == column {
			return c.DefaultOriginal != ""
		}
	}
	return false
}

// catalogObject is one row of the catalog reads: an expression the target's
// schema carries, and enough identifiers to name it in a refusal. No value from
// either database reaches a field of this struct except Expr, which is not
// rendered into an event — the refusal carries the validator's name.
type catalogObject struct {
	table  ref.TableRef
	object string
	kind   string // one of the kind constants below
	expr   string
}

// The object classes this pass reads, spelled as the statements in sql.go spell
// them, because the kind is rendered into the refusal's reason ("a literal in
// the default parses as email"). kindDefault and kindGenerated name a column in
// catalogObject.object; kindConstraint and the two index kinds name a
// constraint or an index; kindEnumLabel names the type and the label's ordinal
// and kindDomainDefault names the domain — never the label text
// (THREAT_MODEL.md T4).
const (
	kindDefault         = "default"
	kindGenerated       = "generated expression"
	kindConstraint      = "constraint"
	kindIndexPredicate  = "index predicate"
	kindIndexExpression = "index expression"
	// The two object classes the 2026-09-15 red team's A4b, A11 and A12 walked
	// through. Both are recreated verbatim by internal/load/ddl and neither
	// was read by this pass or by internal/plan's.
	kindEnumLabel     = "enum label"
	kindDomainDefault = "domain default"
	// kindDomainConstraint is a domain's CHECK. It arrives in the same read as
	// a table's, because pg_constraint carries it with conrelid 0 rather than
	// absent, and it is its own kind so that this pass can attribute it to the
	// type it belongs to — which is what --allow-type-literal names (catalogExempt).
	kindDomainConstraint = "domain constraint"
)

// catalog is ARCHITECTURE.md §6's catalog pass. Like every other check here it
// records what it found and never stops the run: the report names every object
// that is wrong, not the first one.
func (s *state) catalog(ctx context.Context) error {
	objects, err := s.catalogObjects(ctx)
	if err != nil {
		return err
	}
	checked := int64(0)
	for _, o := range objects {
		if s.catalogExempt(o) {
			continue
		}
		lits := pipeline.Literals(o.expr)
		if len(lits) == 0 {
			continue
		}
		checked++
		for _, lit := range lits {
			hit := strongCatalogHit(lit)
			if hit == "" {
				continue
			}
			s.fail(&Refusal{
				Code: CodeRefusedCatalogLiteral, Exit: exitResidual, Check: checkCatalog,
				Table: o.table, Column: o.object, Count: 1,
				Reason: "a literal in the " + o.kind + " parses as " + hit,
			})
			break
		}
	}
	if !s.failed(checkCatalog) {
		s.pass(checkCatalog, CodeCatalogPassed, checked)
	}
	return nil
}

// catalogObjects reads pg_attrdef, pg_constraint, pg_index, pg_enum and
// pg_type in the target.
func (s *state) catalogObjects(ctx context.Context) ([]catalogObject, error) {
	var out []catalogObject
	read := func(sql, kindColumn string) error {
		rows, err := s.target.Query(ctx, sql)
		if err != nil {
			return fmt.Errorf("verify: reading the target's catalog: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var schema, table, object, kind, expr string
			if err := rows.Scan(&schema, &table, &object, &kind, &expr); err != nil {
				return fmt.Errorf("verify: reading the target's catalog: %w", err)
			}
			if kindColumn != "" {
				kind = kindColumn
			}
			out = append(out, catalogObject{
				table:  ref.TableRef{Schema: schema, Name: table},
				object: object,
				kind:   kind,
				expr:   expr,
			})
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("verify: reading the target's catalog: %w", err)
		}
		return nil
	}
	// readLiteral is read for a column that holds the value itself rather than
	// an expression: pg_enum.enumlabel is the label, not SQL text, so it is
	// quoted into the literal form the rest of this pass scans.
	readLiteral := func(sql, kind string) error {
		before := len(out)
		if err := read(sql, kind); err != nil {
			return err
		}
		for i := before; i < len(out); i++ {
			out[i].expr = quoteLiteral(out[i].expr)
		}
		return nil
	}
	if err := read(catalogDefaultsSQL, ""); err != nil {
		return nil, err
	}
	// The constraint read spells its own kind: a domain's CHECK and a table's
	// arrive in one statement and are not the same object class here.
	if err := read(catalogConstraintsSQL, ""); err != nil {
		return nil, err
	}
	if err := read(catalogIndexesSQL, ""); err != nil {
		return nil, err
	}
	// An enum label is not an expression, so the literal scanner would find
	// nothing in it: the label is the value. It is wrapped in the spelling
	// internal/load/ddl writes it in — a single-quoted string literal — so
	// that one scanner answers for every object class here.
	if err := readLiteral(catalogEnumLabelsSQL, kindEnumLabel); err != nil {
		return nil, err
	}
	if err := read(catalogDomainDefaultsSQL, kindDomainDefault); err != nil {
		return nil, err
	}
	return out, nil
}

// catalogExempt reports an object whose literals this pass may not judge on
// their shape, because the shape is the masker's own output rather than the
// source's value.
//
// Two columns are exempt, and only for their own DEFAULT:
//
//   - A column this run masked **whose default the planner actually rewrote**.
//     internal/plan rewrites it through the column's masker (ARCHITECTURE.md
//     §11.1's 2026-09-14 amendment, arm 1), and an email masker's output is a
//     valid address by construction, so judging it here would fail exit 9 on
//     precisely the run that did the right thing. This stage cannot tell the
//     masker's output from the source's value by *inspection* — it holds no key,
//     and the residual filter holds cells the transformer masked, never a
//     default — so it asks provenance instead of shape, and asks it of the
//     rewrite rather than of the decision: rewroteDefault reads the
//     Column.DefaultOriginal the planner wrote into this same schema. A masked
//     column whose default was left as the source wrote it is judged like any
//     other object, which is what keeps this arm from standing open over the
//     whole class while T-0161 is unlanded and the planner rewrites nothing.
//   - A column carrying an --unmask opt-out. internal/plan does not refuse a
//     literal in that column's DDL either (ddlliteral.go's optedOut), because
//     the operator has said in writing, with a reason, that the column's
//     contents are not a person's; ARCHITECTURE.md §8's escape has to mean the
//     same thing at both ends or it is not an escape. Unlike the masked arm this
//     one covers the column's generated expression too, because that is the
//     object plan admits for the same reason.
//
// And one *type* is exempt for every object that belongs to it: one the
// operator opted out of with --allow-type-literal TYPE=REASON, which the planner
// records on Plan.AllowedTypeLiterals (allowedTypeLiteral, below). It is the same
// both-ends rule as the line above, for the object class §11.1 recreates that
// is not a column: internal/plan's checkTypeLiterals refuses an enum label or
// a domain definition at exit 13 and honours that opt-out, so a pass here that
// did not honour it would load the target and then refuse at exit 9 over the
// very object the operator was told they had allowed — which is worse than no
// escape at all (the T-REDFIX review's fourth finding).
//
// Nothing else is exempt. A CHECK, an exclusion constraint and an index
// predicate are never rewritten by anything, so a strong hit in one is the
// source's own literal whatever the classification says about the columns it
// names — which is the whole reason this is a second look and not a re-run of
// the planner's.
func (s *state) catalogExempt(o catalogObject) bool {
	switch o.kind {
	case kindDefault, kindGenerated:
		// Below.
	case kindEnumLabel, kindDomainDefault, kindDomainConstraint:
		// Never exempt by classification: an enum label cannot be rewritten at
		// all — the rows reference it, so masking it would either break the
		// column or silently remap rows — and a domain's DEFAULT and CHECK
		// belong to a type rather than to a column, so there is no single
		// masker that could have produced what stands there. internal/plan
		// refuses all three at exit 13 for the same reason; if one reaches this
		// pass, the planner did not see it and this is the only control over
		// it. The one thing that exempts them is the operator's own per-type
		// opt-out, which the planner honoured to let the run get this far.
		return s.allowedTypeLiteral(o)
	case kindConstraint, kindIndexPredicate, kindIndexExpression:
		return false
	default:
		return false
	}
	d, has := s.decision(ref.ColumnRef{Table: o.table, Column: o.object})
	if !has {
		return false
	}
	if optedOut(d) {
		return true
	}
	return o.kind == kindDefault && d.Masked && s.rewroteDefault(o.table, o.object)
}

// allowedTypeLiteral reports that this object belongs to a type the operator opted
// out of with --allow-type-literal TYPE=REASON, which internal/plan resolved against
// the source's own type names and recorded on Plan.AllowedTypeLiterals.
//
// The three object classes name their type in three places, because the
// catalog does: an enum label's object is "<typname> label <n>" (the ordinal is
// what a refusal names, never the label), a domain default's object is the
// typname itself, and a domain constraint's object is the constraint name with
// the typname in the table position. Every one of them carries the type's
// schema, so the name compared is the qualified one the flag is resolved to.
func (s *state) allowedTypeLiteral(o catalogObject) bool {
	if s.plan == nil || len(s.plan.AllowedTypeLiterals) == 0 {
		return false
	}
	name := ""
	switch o.kind {
	case kindEnumLabel:
		name = o.object
		if i := strings.Index(name, " label "); i > 0 {
			name = name[:i]
		}
	case kindDomainDefault:
		name = o.object
	case kindDomainConstraint:
		name = o.table.Name
	}
	if name == "" {
		return false
	}
	if o.table.Schema != "" {
		name = o.table.Schema + "." + name
	}
	for _, opted := range s.plan.AllowedTypeLiterals {
		if opted == name {
			return true
		}
	}
	return false
}

// addressSuffixWords and addressLiteralShape are the identical corroboration
// internal/plan/ddlliteral.go carries under the same two names (the T-0189
// fix round's finding 2): textsig.AddressShape is calibrated for this
// package's own second net, which asks it of a whole column and fails only
// once many rows agree (validators.go marks it explicitly not strong for
// that reason), and a DDL literal gets one look, not many. Measured against
// ordinary CHECK value-list and enum-label text, the bare shape ("a digit
// somewhere, at least two letter-bearing words") also hits "Basic 1 user",
// "Pro 5 users", "tier 2 plus", "level 1 support", "P1 High Priority", "Top
// 10 sellers", "Building 4 Lobby" and "version 2 draft" — pricing tiers and
// priority labels loaded straight into the target, refusing this pass at
// exit 9 on an already-loaded run over a value that was never a person's.
// The two copies exist for the reason strongCatalogHit's own comment below
// gives for the rest of this list: the packages may not import each other.
var addressSuffixWords = map[string]bool{
	"street": true, "st": true, "avenue": true, "ave": true, "road": true, "rd": true,
	"lane": true, "ln": true, "drive": true, "dr": true, "boulevard": true, "blvd": true,
	"way": true, "court": true, "ct": true, "place": true, "pl": true, "circle": true,
	"cir": true, "terrace": true, "ter": true, "highway": true, "hwy": true,
	"parkway": true, "pkwy": true, "trail": true, "trl": true, "square": true, "sq": true,
	"loop": true, "alley": true, "row": true, "walk": true, "crescent": true,
	"close": true, "grove": true, "parade": true, "crossing": true,
}

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

// strongCatalogHit is the first validator a literal matches, or "". It is
// deliberately the same set internal/plan refuses on (ddlliteral.go's
// strongValidators/strongHit); the two packages may not import each other, and
// this is the second of the two copies internal/verify/CLAUDE.md records.
//
// The national_id branch is textsig.ValidNationalIDStructured, not
// textsig.ValidNationalID, as of the T-0187 review round (finding 2):
// ValidNationalID is the twelve-format union, six of which are a mod-N sum
// over an otherwise unconstrained digit run and clear a random string of the
// right length far too often for a one-occurrence refusal (9.1% of random
// 8-digit strings, 25.7% of 9-digit, 11.0% of 11-digit, measured) —
// ValidNationalIDStructured is the six that also constrain the value's shape
// (a dash, a letter, or a fixed length under its own mod-97 check) and is
// precise enough for exactly this use; textsig.go's own comment on both
// functions has the reasoning. internal/plan/ddlliteral.go's strongHit calls
// the identical function now (tracker T-0194), so the two passes are back to
// "the same set" for this category, as every other branch below already was.
// The checksum-only six stay out of this function for the identical reason,
// and the 2026-09-15 amendment below does not touch that: it answers the
// dictionary argument, not T-0194's measured false-accept rate on an
// unconstrained digit run.
//
// **Amended 2026-09-15 (T-0189, the round-2 red team's R2-07 and R2-09).**
// Until this amendment the set was five — email, phone, the Luhn and IBAN
// halves of financial_account, and national_id — and every category the row
// pipeline masks that is a parse or a shape rather than a guess over anything
// (network_id, online_id, person_name, address, free_text) crossed the DDL
// boundary untouched: a person's name or a postal address in a table CHECK
// (R2-07), or in an enum label, a domain CHECK, a domain DEFAULT or a
// generated expression (R2-09, the same four object classes A4b/A11/A12
// already taught this pass to read), all crossed under exit 9's own green
// tick — the *catalog* pass, not only the plan-time one, since this function
// is the one both ARCHITECTURE.md §11.1's comment and THREAT_MODEL.md T1's
// narrowing named directly. The argument that had kept those five out was
// about *identifiers*: a CHECK is full of English words and a default is full
// of them too, so a dictionary-backed signal run over the whole expression
// text would refuse ordinary schemas over a column named after a street. That
// argument has no purchase here, because pipeline.Literals never returns an
// identifier — only the quoted string constants, which are the source's own
// values wherever they appear. A pass that judges only those five extra
// categories over the literals of an already-loaded target costs nothing new
// to an ordinary schema and closes the gap R2-07 and R2-09 found.
//
// **credential does not join.** textsig.LooksSecret is the one validator on
// internal/verify's own row-scanning list (validators.go) that is not a parse
// or a dictionary shape — an entropy guess over any string, sixteen
// characters or longer, carrying two of {lowercase, uppercase, digit} — and a
// DEFAULT calling nextval embeds exactly that shape by construction: the
// sequence's own quoted, mixed-case relation name. Running this amendment
// with credential included refused three of testdata/torture/'s ten
// real-world schemas and one regression fixture over exactly that shape, a
// relation name and never a person's; internal/plan/ddlliteral.go's own
// comment on strongValidators has the measured evidence. The dictionary
// argument this amendment answers has no bearing on that: an entropy guess is
// unreliable evidence from a single occurrence whether the text around it is
// an identifier or not, which is the same reason the checksum-only
// national_id entries stay out on T-0194's own, unrelated argument.
//
// A pattern operand is detected under a reduced text and never a hit outright
// (amended 2026-09-15, T-0189, R2-10): pipeline.StripPatternMeta removes the
// syntax a pattern operator reads as a wildcard or an anchor and unescapes a
// backslash-escaped metacharacter to the literal character it stands for, so
// CHECK (email LIKE '%@%.%') still reduces to "@", which nothing here
// validates, while CHECK (email !~ '^ceo@bigcorp\.example$') reduces to
// "ceo@bigcorp.example" intact — the value the old blanket exemption let
// through under exit 9's own green tick. Nothing here ever rewrites a literal
// in the first place — this pass only reads the target's catalog back — so
// Pattern's rewrite exemption was never this function's to grant or withhold.
func strongCatalogHit(lit pipeline.Literal) string {
	s := strings.TrimSpace(lit.Text)
	if lit.Pattern {
		s = strings.TrimSpace(pipeline.StripPatternMeta(s))
	}
	if s == "" {
		return ""
	}
	switch {
	case textsig.ValidEmail(s):
		return string(pipeline.CatEmail)
	case textsig.ValidPhone(s):
		return string(pipeline.CatPhone)
	case textsig.ValidLuhn(s):
		return string(pipeline.CatFinancial)
	case textsig.ValidIBAN(s):
		return string(pipeline.CatFinancial)
	case textsig.ValidNationalIDStructured(s):
		return string(pipeline.CatNationalID)
	case textsig.ValidIP(s), textsig.ValidMAC(s):
		return string(pipeline.CatNetworkID)
	case textsig.ValidURL(s):
		return string(pipeline.CatOnlineID)
	case textsig.Dictionary().NameShape(s):
		return string(pipeline.CatPersonName)
	case addressLiteralShape(s):
		return string(pipeline.CatAddress)
	case textsig.Dictionary().ProseName(s):
		return string(pipeline.CatFreeText)
	}
	return ""
}

// quoteLiteral renders a value as the SQL string literal internal/load/ddl
// would write it as, so that pipeline.Literals finds it. It is only ever
// applied to a catalog value this pass read back, never to anything sent to a
// database.
func quoteLiteral(v string) string {
	return "'" + strings.ReplaceAll(v, "'", "''") + "'"
}
