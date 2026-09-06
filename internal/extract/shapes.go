// SPDX-License-Identifier: Apache-2.0

package extract

import (
	"github.com/Liarea/lazyslice/internal/pipeline"
)

// The statement shapes extract sends to the source. Every statement built in
// sql.go matches one of the templates here, and a statement that matches none
// of them never reaches the server: the source's tracer refuses it with a
// cancelled context and records a violation that fails the run
// (ARCHITECTURE.md §2 "Source", THREAT_MODEL.md T9).
//
// These templates were reviewed once before this package existed, as
// pg.ExtractShapes() in internal/pg/shapes_extract.go, because the task that
// reviewed them could write internal/pg and not internal/extract. That file is
// gone: the shapes now sit beside the statements they describe, and
// shapes_test.go builds a statement of every shape with the real builders and
// runs it through a real pg.Tracer, so a change to sql.go that this file does
// not follow fails that test rather than the first run against production.

// Statement is one statement this package sends to the source, with the name
// the allowlist and the trace record it under.
//
// It is not internal/pg's Shape, for the reason internal/plan's Statement is
// not either: ARCHITECTURE.md §2's import graph has the stage packages
// importing pipeline and nothing else of the tree, so the wiring that owns both
// — internal/core — converts these into pg.Shape and registers them before
// Extract is called.
type Statement struct{ Name, SQL string }

// rowsShape is §12's chunked typed unnest join: one table's copied columns for
// one chunk of that step's identity keys. It is table-agnostic on purpose —
// every keyed step builds it — and it is bounded by its own structure rather
// than by a LIMIT: without a chunk of keys on the other side of the join there
// is no statement of this shape at all, and the chunk is at most chunkSize
// tuples.
const rowsShape = `SELECT {selectlist} FROM {ident} t ` +
	`JOIN unnest({casts}) AS k({idents}) ON {keypred} ORDER BY {idents}`

// lookupShapeFor is the read for one Lookup step. It names the table and writes
// the bound as a literal, which is as narrow as the statement extract actually
// sends.
//
// Both halves matter. Bounded but table-agnostic, this template is
// byte-for-byte internal/plan's seed read with the bound moved, and every stage
// registers into one additive per-Source allowlist (pg.Tracer.Register removes
// nothing), so it would admit an ordered whole-table read of any relation the
// planner never named — pg_catalog.pg_authid included — with the planner's own
// LIMIT no longer the only spelling of the bound. Named and bounded, it admits
// exactly the reads the plan says will happen.
func lookupShapeFor(t pipeline.TableRef) Statement {
	return Statement{
		Name: "extract.lookup." + t.String(),
		SQL: `SELECT {selectlist} FROM ` + quoteTable(t) + ` t ORDER BY {idents} LIMIT ` +
			itoa(lookupLimit),
	}
}

// Shapes is every statement shape this package sends to the source for one
// plan. A caller registers them on the source allowlist before calling Extract;
// a statement whose shape is not registered never reaches the server
// (THREAT_MODEL.md T9).
//
// It takes the plan because the lookup reads are named per table, and the plan
// is what knows which tables those are. A nil plan yields the keyed read alone,
// which is what a caller compiling the allowlist to reason about its shape —
// rather than to run a plan through it — wants.
func Shapes(p *pipeline.Plan) []Statement {
	out := []Statement{{Name: "extract.rows", SQL: rowsShape}}
	if p == nil {
		return out
	}
	for _, s := range p.Steps {
		if s.Mode == pipeline.Lookup {
			out = append(out, lookupShapeFor(s.Table))
		}
	}
	return out
}
