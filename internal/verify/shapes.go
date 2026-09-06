// SPDX-License-Identifier: Apache-2.0

package verify

import (
	"sort"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
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
// and one classification: a sample read per keyed step, and the two
// confirmation probes for every masked column of a loaded table.
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
		for _, col := range maskedColumns(cls, s.Table) {
			out = append(out, probeShapesFor(col)...)
		}
	}
	return out
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
