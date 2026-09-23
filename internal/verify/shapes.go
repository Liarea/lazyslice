// SPDX-License-Identifier: Apache-2.0

package verify

import (
	"sort"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/mask"
)

// The statement shapes verify sends to the source. Every statement built in
// sql.go for the source matches one of the templates here, and a statement that
// matches none of them never reaches the server: the source's tracer refuses it
// with a cancelled context and records a violation that fails the run
// (ARCHITECTURE.md section 2 "Source", THREAT_MODEL.md T9).
//
// A caller registers these on the source allowlist before calling Verify. That
// caller is internal/core; this package cannot import internal/pg, for the same
// reason internal/extract and internal/plan cannot (ARCHITECTURE.md section 2
// "Import graph"), so Statement is converted to pg.Shape there.

// Statement is one statement this package sends to the source, with the name
// the allowlist and the trace record it under.
type Statement struct{ Name, SQL string }

// sampleShapeFor is the sample comparison's read of one table: the same chunked
// typed unnest join internal/extract uses, named for the table it reads.
//
// It is named per table rather than left table-agnostic for the reason
// internal/extract gives its lookup read: the allowlist is one additive union
// that every stage registers into, so a table-agnostic ordered read here would
// admit one of any relation the plan never named. Named, it admits exactly the
// reads the plan says will happen.
func sampleShapeFor(t ref.TableRef) Statement {
	return Statement{
		Name: "verify.sample." + t.String(),
		SQL: `SELECT {selectlist} FROM ` + quoteTable(t) +
			` t JOIN unnest({casts}) AS k({idents}) ON {keypred} ORDER BY {idents}`,
	}
}

// rowCheckShapeFor is ADR-015's row check of one table (rowCheckSQL): the
// sample's chunked typed unnest join under the alias `r`, named per table for
// the reason sampleShapeFor is.
func rowCheckShapeFor(t ref.TableRef) Statement {
	return Statement{
		Name: "verify.rowcheck." + t.String(),
		SQL: `SELECT {selectlist} FROM ` + quoteTable(t) +
			` r JOIN unnest({casts}) AS k({idents}) ON {keypred} ORDER BY {idents}`,
	}
}

// probeShapesFor are section 6 item 3's two confirmation probes for one masked
// column, indexable form first. Both name the column: a probe is built only for
// a column the classification says was masked, and a shape that named neither
// the table nor the column would admit an EXISTS test of any value against any
// relation.
func probeShapesFor(c ref.ColumnRef) []Statement {
	return []Statement{
		{Name: "verify.probe." + c.String(), SQL: probeSQL(c.Table, c.Column)},
		{Name: "verify.probe.folded." + c.String(), SQL: foldedProbeSQL(c.Table, c.Column)},
	}
}

// Shapes is every statement shape this package sends to the source for one plan
// and one classification: a sample read per keyed step, the two confirmation
// probes for every masked column of a loaded table, and ADR-015's row check
// for every loaded table with a masked column whose masker has a vocabulary
// and a row identity the row check can use.
//
// A nil plan yields nothing, because every statement here is built from a step.
func Shapes(plan *pipeline.Plan, cls *pipeline.Classification) []Statement {
	if plan == nil {
		return nil
	}
	var out []Statement
	for _, s := range plan.Steps {
		if s.Mode == pipeline.SchemaOnly {
			continue
		}
		if s.Keys != nil && s.Keys.Len() > 0 {
			out = append(out, sampleShapeFor(s.Table))
		}
		masked := maskedColumns(cls, s.Table)
		for _, col := range masked {
			out = append(out, probeShapesFor(col)...)
		}
		if rowCheckEligible(s, cls, masked) {
			out = append(out, rowCheckShapeFor(s.Table))
		}
	}
	return out
}

// rowCheckEligible is the part of explain.go's eligibility Shapes can decide
// from a plan and a classification alone, before any schema or value is read:
// a masked column whose masker has a vocabulary (mask.Emitting, asked without
// the column's labels, which only ever narrow it), and a step whose identity
// is a primary key or unique rung (uniqueIdentity; never a pseudo-key) with
// every column unmasked — or a lookup step, whose identity is its table's
// primary key and is judged at verify time. It is a superset of what verify
// sends, never a subset: a shape registered for a statement that is never sent
// admits nothing that runs, and a statement sent without a shape is refused.
func rowCheckEligible(s pipeline.Step, cls *pipeline.Classification, masked []ref.ColumnRef) bool {
	emitting := false
	for _, col := range masked {
		d := cls.Decisions[col]
		if mask.Emitting(d.Masker, mask.Constraints{Role: d.Role}) {
			emitting = true
			break
		}
	}
	if !emitting {
		return false
	}
	if s.Mode == pipeline.Lookup {
		return true
	}
	if !uniqueIdentity(s.Identity) {
		return false
	}
	for _, c := range s.Identity.Columns {
		if d, ok := cls.Decisions[ref.ColumnRef{Table: s.Table, Column: c}]; ok && d.Masked {
			return false
		}
	}
	return true
}

// maskedColumns is every column of one table the classification says was
// masked, in column order.
func maskedColumns(cls *pipeline.Classification, t ref.TableRef) []ref.ColumnRef {
	if cls == nil {
		return nil
	}
	var out []ref.ColumnRef
	for col, d := range cls.Decisions {
		if col.Table == t && d.Masked {
			out = append(out, col)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Column < out[j].Column })
	return out
}
