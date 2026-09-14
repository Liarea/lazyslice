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
// replays an index verbatim through pg_get_indexdef. Every string literal in
// each text goes through the three strong validators — email, phone, payment
// card — and a hit is exit 9 naming the table and the object.
//
// It is the *second* look at the same boundary, not the first: internal/plan's
// checkDDLLiterals refuses or rewrites before anything is dropped, and this
// reads back what was actually written. The two are deliberately not the same
// code path — plan reads the source's *pipeline.Schema and this reads the
// target's catalog — so a literal that reached the target through a route the
// planner does not walk is still found here. The index predicate is exactly
// that route today and is why the third read exists: internal/plan reads no
// pg_index at all (tracker T-0163), so `CREATE UNIQUE INDEX ... WHERE email =
// 'x@y.test'` is seen by this pass and by nothing else. An object somebody added
// to the target by hand is the other.
//
// Why only the strong three, when the second net runs nine validators: because
// this text is SQL and not data. A CHECK is full of English words and a default
// is full of identifiers, and the dictionary-backed validators would refuse an
// already-loaded target over a column named after a street. THREAT_MODEL.md T1
// states the narrowing rather than hiding it.
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

// catalogObject is one row of the three catalog reads: an expression the target's
// schema carries, and enough identifiers to name it in a refusal. No value from
// either database reaches a field of this struct except Expr, which is not
// rendered into an event — the refusal carries the validator's name.
type catalogObject struct {
	table  ref.TableRef
	object string
	kind   string // one of the five kind constants below
	expr   string
}

// The object classes this pass reads, spelled as the statements in sql.go spell
// them, because the kind is rendered into the refusal's reason ("a literal in
// the default parses as email"). kindDefault and kindGenerated name a column in
// catalogObject.object; the other three name a constraint or an index.
const (
	kindDefault         = "default"
	kindGenerated       = "generated expression"
	kindConstraint      = "constraint"
	kindIndexPredicate  = "index predicate"
	kindIndexExpression = "index expression"
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

// catalogObjects reads pg_attrdef, pg_constraint and pg_index in the target.
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
	if err := read(catalogDefaultsSQL, ""); err != nil {
		return nil, err
	}
	if err := read(catalogConstraintsSQL, kindConstraint); err != nil {
		return nil, err
	}
	if err := read(catalogIndexesSQL, ""); err != nil {
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
// Nothing else is exempt. A CHECK, an exclusion constraint and an index
// predicate are never rewritten by anything, so a strong hit in one is the
// source's own literal whatever the classification says about the columns it
// names — which is the whole reason this is a second look and not a re-run of
// the planner's.
func (s *state) catalogExempt(o catalogObject) bool {
	switch o.kind {
	case kindDefault, kindGenerated:
		// Below.
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

// strongCatalogHit is the first of the three strong validators a literal
// matches, or "". It is deliberately the same set internal/plan refuses on
// (ddlliteral.go); the two packages may not import each other, and this is the
// second of the two copies internal/verify/CLAUDE.md records.
//
// A pattern operand is never a hit, for the reason internal/plan's strongHit
// gives: CHECK (email LIKE '%@%.%') carries a shape and not a value, and
// net/mail reads that shape as a valid address.
func strongCatalogHit(lit pipeline.Literal) string {
	if lit.Pattern {
		return ""
	}
	s := strings.TrimSpace(lit.Text)
	switch {
	case textsig.ValidEmail(s):
		return string(pipeline.CatEmail)
	case textsig.ValidPhone(s):
		return string(pipeline.CatPhone)
	case textsig.ValidLuhn(s):
		return string(pipeline.CatFinancial)
	}
	return ""
}
