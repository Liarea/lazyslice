// SPDX-License-Identifier: Apache-2.0

package verify

import (
	"context"
	"fmt"
	"regexp"
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
		hit := ""
		for _, lit := range lits {
			if h := strongCatalogHit(lit); h != "" {
				hit = "a literal in the " + o.kind + " parses as " + h
				break
			}
		}
		// T-0198's broadened rule, scoped to a DEFAULT or a generated
		// expression only (the T-0198 fix round's own re-measurement of
		// finding 1, mirroring internal/plan/ddlliteral.go's own fixedExpression
		// -- see its comment for the two real schemas, odoo's res_partner_
		// check_name and res_partner_mobile_partial_gin_idx, that found why a
		// CHECK or an index cannot safely carry this net at all: either object
		// class can name several columns, or can hold a literal that is a
		// transformation argument rather than a value about the one masked
		// column it does name, and this pass's only escape (--unmask) can
		// then require unmasking a column that has nothing to do with the
		// literal being refused -- a real person's name in one schema, a real
		// phone number in the other. unrewritableKind excludes the enum/
		// domain object classes too, which belong to a type rather than to a
		// column and have their own --allow-type-literal escape
		// (allowedTypeLiteral, above), never the per-column --unmask this
		// rule defers to.
		if hit == "" && unrewritableKind(o.kind) && s.maskedNamedColumn(o) {
			enumLabels, _ := s.enumColumnLabels(o)
			for _, lit := range lits {
				if unrewritableLiteral(o.expr, lit) && !enumLabelLiteral(enumLabels, lit) {
					hit = "masked column, unrewritable literal in its own constraint"
					break
				}
			}
		}
		if hit == "" {
			continue
		}
		s.fail(&Refusal{
			Code: CodeRefusedCatalogLiteral, Exit: exitResidual, Check: checkCatalog,
			Table: o.table, Column: o.object, Count: 1,
			Reason: hit,
		})
	}
	if !s.failed(checkCatalog) {
		s.pass(checkCatalog, CodeCatalogPassed, checked)
	}
	return nil
}

// unrewritableKind reports the object classes T-0198's broadened rule covers:
// a DEFAULT or a generated expression, both single-column by construction,
// once catalogExempt's own arm for a masked, rewritten DEFAULT has already
// been asked (that check runs first in catalogExempt, so a kindDefault or
// kindGenerated object reaching here is exactly the residual case this rule
// wants as a second look). kindConstraint, kindIndexPredicate and
// kindIndexExpression are deliberately excluded (the T-0198 fix round's own
// re-measurement of finding 1) -- see the call site's own comment for why a
// CHECK or an index cannot safely carry this net; each still refuses on a
// strongCatalogHit exactly as it always has. The enum and domain object
// classes are excluded for an unrelated reason: they belong to a *type* and
// their escape is --allow-type-literal, not the per-column --unmask this
// rule defers to, and maskedNamedColumn could not answer a meaningful
// question about them anyway.
func unrewritableKind(kind string) bool {
	switch kind {
	case kindDefault, kindGenerated:
		return true
	default:
		return false
	}
}

// maskedNamedColumn reports whether o's own text names *exactly one* column
// this run masked and did not opt out of with --unmask -- the "unless the
// existing named opt-out names the column" half of T-0198's rule. A column
// opted out has d.Masked false (ADR-004's tighten-only rule), so the
// !optedOut(d) guard is belt and braces rather than the load-bearing half;
// it is kept so that this function reads the same two questions
// plan/ddlliteral.go's masked/unmasked branch already asks, rather than
// trusting d.Masked alone to have already answered the opt-out question.
//
// **Exactly one, not "any", since the T-0198 fix round's own re-measurement
// of finding 1** (internal/plan/ddlliteral.go's fixedExpression carries the
// full account and the two real schemas that found it: odoo's
// res_partner_check_name and supabase-auth's own oauth2 constraint). A
// `CHECK` or an index naming two or more masked columns has no single answer
// to which one a given literal is about, so the --unmask escape this rule
// implies is real only when there is one column to name; below that bound
// the object still refuses on a strongCatalogHit exactly as it always has,
// unaffected by this change.
func (s *state) maskedNamedColumn(o catalogObject) bool {
	tab, ok := s.tables[o.table]
	if !ok || tab == nil {
		return false
	}
	// kindDefault and kindGenerated name the column directly in o.object
	// (catalogDefaultsSQL selects a.attname); their own text -- the DEFAULT
	// expression, the generated expression -- has no reason to mention the
	// column's own name at all ("DEFAULT 'active'" names no column), so
	// namedColumns would find nothing there -- one name, by construction, so
	// the ambiguity this function now guards against cannot arise on that
	// path. kindConstraint, kindIndexPredicate and kindIndexExpression name a
	// constraint or an index, and it is their expr that has to be scanned for
	// which of the table's columns it mentions.
	names := []string{o.object}
	if o.kind != kindDefault && o.kind != kindGenerated {
		names = namedColumns(o.expr, tab.Columns)
	}
	matches := 0
	for _, name := range names {
		d, has := s.decision(ref.ColumnRef{Table: o.table, Column: name})
		if has && d.Masked && !optedOut(d) {
			matches++
		}
	}
	return matches == 1
}

// namedColumns is internal/plan/ddlliteral.go's function of the same name,
// duplicated for the reason every other entry in this file's own validator
// note already is (a stage package may not import another,
// internal/CLAUDE.md). pg_get_constraintdef and pg_get_expr's own texts —
// unlike pg_get_indexdef, which internal/plan reads for the same purpose —
// never open with a `CREATE ... ON schema.table` clause, so there is no
// table-name collision to trim away here and this copy is the plain token
// match.
func namedColumns(expr string, cols []pipeline.Column) []string {
	var out []string
	for _, col := range cols {
		quoted := `"` + strings.ReplaceAll(col.Name, `"`, `""`) + `"`
		if strings.Contains(expr, quoted) || containsWord(expr, col.Name) {
			out = append(out, col.Name)
		}
	}
	return out
}

// containsWord is internal/plan/ddlliteral.go's function of the same name.
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

// reClosedValueList and inClosedValueList are internal/plan/ddlliteral.go's
// of the same names -- see that file's own comment for the reasoning. mask's
// reIn/reAny are unexported, and internal/verify may import neither mask nor
// internal/plan (internal/CLAUDE.md), so the shape is duplicated a third
// time (mask, internal/plan, here) the way every other closed-form check in
// this file's own comment already records as a duplicate.
var reClosedValueList = regexp.MustCompile(`(?is)(?:\bNOT\s+)?\bIN\s*\([^()]*\)|=\s*ANY\s*\(\s*ARRAY\s*\[[^\]]*\]\s*\)`)

func inClosedValueList(def string, lit pipeline.Literal) bool {
	for _, span := range reClosedValueList.FindAllStringIndex(def, -1) {
		if lit.Start >= span[0] && lit.End <= span[1] {
			return true
		}
	}
	return false
}

// unrewritableLiteral is internal/plan/ddlliteral.go's function of the same
// name and the same rule: a Pattern operand's reduced text is a fragment of
// a shape rather than a value (testdata/nasty.sql's own CHECK ("EmailAddress"
// LIKE '%@%.%') reduces to "@", which must not refuse), an empty (or
// whitespace-only) plain literal was never worth refusing on, an empty
// collection literal ('{}' or '[]') is ARCHITECTURE.md §5's own rule for a
// masked column's DEFAULT — nothing in it to be a person's, the identical
// argument internal/invariants/i2_masking_test.go's preservedEmpty makes on
// the row side — a literal inside a closed value list is an ordinary
// status/tier/plan enum the masker already honours, never personal data, and
// a literal immediately cast to a non-text type (pipeline.CastToNonText,
// T-0198 fix round finding 3) is a sentinel the deparser proved can never be
// free text — `COALESCE(user_id, '-1'::integer)`, the shape odoo's and
// discourse's own torture schemas both carry.
func unrewritableLiteral(def string, lit pipeline.Literal) bool {
	if lit.Pattern {
		return false
	}
	text := strings.TrimSpace(lit.Text)
	if text == "" || text == "{}" || text == "[]" {
		return false
	}
	if pipeline.CastToNonText(def, lit) {
		return false
	}
	return !inClosedValueList(def, lit)
}

// enumColumnLabels is internal/plan/ddlliteral.go's enumDefaultLabels,
// restated for an object this pass already has rather than a
// pipeline.Column it is handed directly: a kindDefault or kindGenerated
// object's own name (o.object) is the column, so its declared type is looked
// up on s.tables and resolved against s.schema.Enums by both spellings, the
// same qualified-then-bare lookup internal/transform's own enumLabels uses.
// Every other kind names a constraint or an index rather than a single
// column, so there is no one type to resolve and this returns false for all
// of them -- testdata/nasty.sql's own trap 24 (`marital_status ... DEFAULT
// 'undisclosed'`) is a DEFAULT, not a CHECK, and this pass only reaches a
// masked column's *unrewritten* DEFAULT here in the first place
// (catalogExempt's own arm covers a rewritten one).
//
// The bare-name fallback exempts only when the bare name is unambiguous
// (T-0198 fix round, finding 5, mirroring internal/plan/ddlliteral.go's own
// enumDefaultLabels): it used to return the first bare-name match a Go map
// iteration produced, so whether a masked enum-typed column's DEFAULT was
// exempted as "one of the column's own labels" was nondeterministic run to
// run the moment two schemas each declared an enum of the same bare name
// (`app.status`, `audit.status`), and could exempt a literal read from the
// wrong schema's label list entirely. Counting every match and returning one
// only when there is exactly one is deterministic regardless of iteration
// order and refuses to guess on ambiguity, which is the fail-closed reading:
// no exemption here still leaves strongCatalogHit and the closed-list and
// cast exemptions to answer, and only ever widens what refuses.
func (s *state) enumColumnLabels(o catalogObject) ([]string, bool) {
	if o.kind != kindDefault && o.kind != kindGenerated {
		return nil, false
	}
	tab, ok := s.tables[o.table]
	if !ok || tab == nil {
		return nil, false
	}
	for _, c := range tab.Columns {
		if c.Name != o.object {
			continue
		}
		if labels, ok := s.schema.Enums[unquoteEnumType(c.TypeName)]; ok {
			return labels, true
		}
		bare := bareEnumType(c.TypeName)
		var match []string
		matches := 0
		for key, labels := range s.schema.Enums {
			if bareEnumType(key) != bare {
				continue
			}
			matches++
			match = labels
		}
		if matches != 1 {
			return nil, false
		}
		return match, true
	}
	return nil, false
}

// enumLabelLiteral is internal/plan/ddlliteral.go's function of the same
// name: lit's text is one of an enum-typed column's own labels, a closed
// domain by the column's TYPE rather than by a CHECK's IN (...) syntax.
func enumLabelLiteral(labels []string, lit pipeline.Literal) bool {
	for _, l := range labels {
		if l == lit.Text {
			return true
		}
	}
	return false
}

// unquoteEnumType and bareEnumType are mask.UnquoteType and mask.BareTypeName,
// restated byte for byte: internal/verify may not import mask
// (internal/CLAUDE.md's import graph has no edge from a stage package to
// it), so the two string reductions enumColumnLabels needs are duplicated
// here rather than pulling in the module for two functions.
func unquoteEnumType(name string) string { return strings.ReplaceAll(name, `"`, "") }

func bareEnumType(name string) string {
	inQuote := false
	cut := -1
	for i, r := range name {
		switch r {
		case '"':
			inQuote = !inQuote
		case '.':
			if !inQuote {
				cut = i
			}
		}
	}
	if cut < 0 {
		return unquoteEnumType(name)
	}
	return unquoteEnumType(name[cut+1:])
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
	case textsig.SpecialCategoryVocabulary(s):
		return string(pipeline.CatSpecial)
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
