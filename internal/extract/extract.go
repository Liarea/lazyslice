// SPDX-License-Identifier: Apache-2.0

// Package extract streams the planned rows out of the source snapshot into a
// channel of pipeline.RowBatch.
//
// One table at a time, in plan order, in chunks of 2,000 keys joined through a
// typed unnest. Tables are strictly sequential on the channel and every
// extracted table ends with a batch carrying Last, including a table with no
// rows, so the loader always sees a boundary rather than inferring one.
//
// Everything here runs inside the run's REPEATABLE READ READ ONLY snapshot and
// through the shape allowlist, so extract holds the snapshot open on production
// and nothing else does. The plan printed the hold estimate before this started
// (THREAT_MODEL.md T9).
//
// Memory is bounded by construction and not by hope, with one stated exception.
// No table's rows are ever accumulated: one batch is filled, handed to the
// channel and replaced, whatever the table's row count.
//
// The exception is the keys. pipeline.KeySet.Chunks materialises every chunk in
// one call, and a chunk holds its own copy of the keys, so at the top of a
// keyed step this package briefly holds a second copy of that step's key set
// alongside the planner's. It is released chunk by chunk as the step runs
// (step below), so the sustained cost is one chunk; the peak is one key set.
// Bounding the peak too needs a chunk-at-a-time iterator on pipeline.KeySet,
// which is an ARCHITECTURE.md §2 change. TestMemoryStaysBoundedOnTwoMillionRows
// runs the whole of nasty.sql's 2,000,000-row stream_rows table through it —
// keys included — under a runtime.MemStats ceiling.
package extract

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// itoa is strconv.FormatInt for an int, used by the shape templates and the
// statement builders alike so the two spell a bound the same way.
func itoa(n int) string { return strconv.Itoa(n) }

type extractor struct {
	schema *pipeline.Schema
	tables map[ref.TableRef]*pipeline.Table
}

// New returns the streaming extractor over the schema introspect read.
//
// ARCHITECTURE.md §2's Extractor.Extract takes the plan and not the schema,
// and the plan does not carry a column list: a Step names a table, a mode, an
// identity and a key set. The copied columns — every column except the
// generated ones, which the target recomputes (§11.1) — are a property of the
// schema, so the schema arrives here instead. Loader.Load is given both, and
// this constructor is the smallest way to give extract the same two without
// changing an interface §2 fixes.
func New(schema *pipeline.Schema) pipeline.Extractor {
	e := extractor{schema: schema}
	if schema != nil {
		e.tables = make(map[ref.TableRef]*pipeline.Table, len(schema.Tables))
		for i := range schema.Tables {
			e.tables[schema.Tables[i].Ref] = &schema.Tables[i]
		}
	}
	return e
}

var _ pipeline.Extractor = extractor{}

// Extract streams every step in plan order, one table at a time, into out and
// closes it.
//
// The channel is closed on every path, including an error path, because the
// loader on the other end reads until the channel closes and a run that failed
// mid-extract must not also hang.
func (e extractor) Extract(
	ctx context.Context,
	r pipeline.Reader,
	plan *pipeline.Plan,
	out chan<- pipeline.RowBatch,
) error {
	defer close(out)
	if e.schema == nil {
		return errors.New("extract: no schema")
	}
	if r == nil {
		return errors.New("extract: no reader")
	}
	if plan == nil {
		return errors.New("extract: no plan")
	}
	for _, step := range plan.Steps {
		if err := ctx.Err(); err != nil {
			return err
		}
		// A SchemaOnly step has no data phase at all: §2 gives it no key set
		// and §11.1 recreates its DDL and nothing else. It is the one step that
		// sends no batch, and verify reports it rather than counting rows for
		// it (§6 item 5).
		if step.Mode == pipeline.SchemaOnly {
			continue
		}
		if err := e.step(ctx, r, step, out); err != nil {
			return err
		}
	}
	return nil
}

// step streams one table. It always sends at least one batch, and the last one
// it sends carries Last, so the loader's boundary never has to be inferred from
// a change of Table (§2 "RowBatch").
func (e extractor) step(
	ctx context.Context,
	r pipeline.Reader,
	step pipeline.Step,
	out chan<- pipeline.RowBatch,
) error {
	table, ok := e.tables[step.Table]
	if !ok {
		return fmt.Errorf("extract: %s is in the plan and not in the schema", step.Table)
	}
	cols := copiedColumns(table)
	b := &batcher{table: step.Table, cols: cols, out: out}

	// A table whose every column is generated is copied as zero columns: the
	// target recomputes all of them. It still gets its boundary batch, so the
	// loader sees the table.
	if len(cols) == 0 {
		return b.finish(ctx)
	}

	if step.Mode == pipeline.Lookup {
		order, err := lookupOrder(table, cols)
		if err != nil {
			return err
		}
		if err := e.read(ctx, r, step.Table, lookupSQL(step.Table, cols, order), b); err != nil {
			return err
		}
		return b.finish(ctx)
	}

	if step.Keys == nil || step.Keys.Len() == 0 {
		return b.finish(ctx)
	}
	idCols := step.Identity.Columns
	if len(idCols) == 0 {
		return fmt.Errorf("extract: %s has keys and no identity columns", step.Table)
	}
	casts, err := joinCasts(table, idCols)
	if err != nil {
		return err
	}
	// pipeline.KeySet.Chunks builds every chunk up front and each chunk holds
	// its own copy of the keys (internal/plan's keyset.go allocates a fresh
	// typed array per chunk), so ranging over the call would keep a second copy
	// of the whole key set alive for the length of the table — 16 MiB for a
	// 2,000,000-row int8 identity, more for text or uuid. Each chunk is dropped
	// as it is consumed instead, so what is live is one chunk and one batch.
	// Removing the up-front copy needs a chunk-at-a-time iterator on
	// pipeline.KeySet, which is an ARCHITECTURE.md §2 change and is reported
	// rather than made here.
	chunks := step.Keys.Chunks(chunkSize)
	for i := range chunks {
		if err := ctx.Err(); err != nil {
			return err
		}
		ch := chunks[i]
		chunks[i] = nil
		sql := rowsSQL(step.Table, cols, idCols, casts, ch)
		if err := e.read(ctx, r, step.Table, sql, b, chunkArgs(ch, len(idCols))...); err != nil {
			return err
		}
	}
	return b.finish(ctx)
}

// read runs one statement and hands every row to the batcher. One row's values
// are allocated per row and handed on; nothing here holds a row after the
// batcher has taken it, which is what keeps the resident set flat across a
// table of any size.
func (e extractor) read(
	ctx context.Context,
	r pipeline.Reader,
	t ref.TableRef,
	sql string,
	b *batcher,
	args ...any,
) error {
	rows, err := r.Query(ctx, sql, args...)
	if err != nil {
		if refusal := asRefusal(t, err); refusal != nil {
			return refusal
		}
		return fmt.Errorf("extract: reading %s: %w", t, err)
	}
	defer rows.Close()

	dest := make([]any, len(b.cols))
	for rows.Next() {
		row := make([]any, len(b.cols))
		for i := range row {
			dest[i] = &row[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return fmt.Errorf("extract: reading %s: %w", t, err)
		}
		if err := b.add(ctx, row); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		if refusal := asRefusal(t, err); refusal != nil {
			return refusal
		}
		return fmt.Errorf("extract: reading %s: %w", t, err)
	}
	return nil
}

// chunkArgs is one chunk as bound parameters: one typed array per identity
// column. []any is never among them — pgx cannot infer an array OID for it —
// because Chunk.Column returns the typed slice (ARCHITECTURE.md §2 "Chunk").
func chunkArgs(ch pipeline.Chunk, n int) []any {
	args := make([]any, n)
	for i := range n {
		args[i] = ch.Column(i)
	}
	return args
}

// copiedColumns is the column list §11.1 gives the load: every column except a
// generated one, which the target recomputes and which cannot be written.
// Order is the catalog's, so two runs build the same statement.
func copiedColumns(t *pipeline.Table) []string {
	cols := make([]string, 0, len(t.Columns))
	for _, c := range t.Columns {
		if c.Generated != "" {
			continue
		}
		cols = append(cols, c.Name)
	}
	return cols
}

// joinCasts is the per-identity-column cast the chunk side of the join carries
// (keys.go).
func joinCasts(t *pipeline.Table, idCols []string) ([]string, error) {
	casts := make([]string, len(idCols))
	for i, name := range idCols {
		col, ok := columnOf(t, name)
		if !ok {
			return nil, fmt.Errorf("extract: %s has no column %q, which its identity names", t.Ref, name)
		}
		casts[i] = joinCast(col)
	}
	return casts, nil
}

func columnOf(t *pipeline.Table, name string) (pipeline.Column, bool) {
	for _, c := range t.Columns {
		if c.Name == name {
			return c, true
		}
	}
	return pipeline.Column{}, false
}

// lookupOrder is the ordering of a Lookup read. A Lookup step carries no
// Identity (internal/plan builds it with the mode and nothing else), so the
// order comes from the table: its primary key, else the first unique index that
// could have been an identity, else every copied column.
//
// The last rung is a total order over distinct rows and is what makes an
// unkeyed lookup reproducible; it is also the one that can fail, on a table
// with no key at all and a column of a type with no ordering operator (json,
// xml, point). That is a loud 42883 naming the table, which is the right
// failure: the alternative is an unordered read whose row order two runs need
// not agree on, and invariant I3 compares two runs.
func lookupOrder(t *pipeline.Table, cols []string) ([]string, error) {
	if len(t.PK) > 0 {
		return t.PK, nil
	}
	for _, idx := range t.Indexes {
		if idx.Unique && !idx.Partial && !idx.Expression && idx.Immediate && len(idx.Columns) > 0 {
			return idx.Columns, nil
		}
	}
	if len(cols) == 0 {
		return nil, fmt.Errorf("extract: %s has no column to order a lookup read by", t.Ref)
	}
	return cols, nil
}

// batcher cuts a table's rows into pipeline.RowBatch values of batchRows rows,
// numbers them from 0 and marks the last one.
//
// It never holds more than one batch: the slice it fills is handed to the
// channel and replaced, so the rows of a table are in this process exactly once
// each and only until the loader has them.
type batcher struct {
	table ref.TableRef
	cols  []string
	out   chan<- pipeline.RowBatch

	rows [][]any
	seq  int
}

func (b *batcher) add(ctx context.Context, row []any) error {
	if b.rows == nil {
		b.rows = make([][]any, 0, batchRows)
	}
	b.rows = append(b.rows, row)
	if len(b.rows) < batchRows {
		return nil
	}
	return b.flush(ctx, false)
}

// finish sends whatever is left with Last set. A table with no rows at all
// sends exactly one empty batch with Last, which is the boundary §2 promises
// the loader.
func (b *batcher) finish(ctx context.Context) error { return b.flush(ctx, true) }

func (b *batcher) flush(ctx context.Context, last bool) error {
	batch := pipeline.RowBatch{
		Table: b.table,
		Cols:  b.cols,
		Rows:  b.rows,
		Seq:   b.seq,
		Last:  last,
	}
	b.rows = nil
	b.seq++
	select {
	case b.out <- batch:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
