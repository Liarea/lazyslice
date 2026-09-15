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
// Memory is bounded by construction and not by hope. No table's rows are ever
// accumulated: one batch is filled, handed to the channel and replaced,
// whatever the table's row count. Neither are its keys: a keyed step is walked
// with pipeline.KeySet.EachChunk, which builds one chunk at a time, so this
// package's peak over a step is one chunk of keys and one batch of rows and not
// a second copy of the step's key set. (It was that second copy until
// EachChunk existed; the Chunks call it replaced materialised every chunk
// before the first read.) TestMemoryStaysBoundedOnTwoMillionRows and
// TestMemoryStaysBoundedOnATextKeyedTable run the whole of nasty.sql's
// 2,000,000-row stream_rows and its text-keyed stream_docs through it — keys
// included — under a runtime.MemStats ceiling.
package extract

import (
	"context"
	"errors"
	"fmt"
	"reflect"
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
	// EachChunk and not Chunks. Chunks builds every chunk before it returns any,
	// and a chunk holds its own copy of the keys it carries (internal/plan's
	// keyset.go allocates a fresh typed array per identity column per chunk), so
	// ranging over that call held a second copy of the whole key set — 16 MiB
	// for the 2,000,000-row int8 identity below, proportionally more for a text
	// or uuid one — for the length of the table. The iterator builds one chunk,
	// reads it and drops it, so this package's peak over a keyed step is one
	// chunk and one batch whatever the step's key count.
	if err := step.Keys.EachChunk(chunkSize, func(ch pipeline.Chunk) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		sql := rowsSQL(step.Table, cols, idCols, casts, ch)
		return e.read(ctx, r, step.Table, sql, b, chunkArgs(ch, len(idCols))...)
	}); err != nil {
		return err
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

// batcher cuts a table's rows into pipeline.RowBatch values of at most
// batchRows rows and at most batchBytes of estimated row content, numbers
// them from 0 and marks the last one.
//
// It never holds more than one batch: the slice it fills is handed to the
// channel and replaced, so the rows of a table are in this process exactly once
// each and only until the loader has them.
type batcher struct {
	table ref.TableRef
	cols  []string
	out   chan<- pipeline.RowBatch

	rows  [][]any
	bytes int
	seq   int
}

func (b *batcher) add(ctx context.Context, row []any) error {
	if b.rows == nil {
		b.rows = make([][]any, 0, batchRows)
	}
	b.rows = append(b.rows, row)
	b.bytes += rowEstimate(row)
	if len(b.rows) < batchRows && b.bytes < batchBytes {
		return nil
	}
	return b.flush(ctx, false)
}

// rowEstimate is a cheap, mostly allocation-free lower bound on one row's
// byte content: it is not a claim about wire or in-memory size (a string is
// scanned once into its own allocation, a driver value carries its own
// overhead beyond its content), only a size the batcher can compare against
// batchBytes without decoding a value a second time. A string or []byte
// contributes its own length; a jsonb/json or array value (pgx's default
// decode for a column scanned into *any: map[string]any, []any, or a typed
// slice) is walked so a large document or array still counts as something
// close to its size rather than the fixed constant; every other value —
// every fixed-width scalar, every driver type this package does not
// otherwise recognise — contributes a small constant, deliberately not
// zero, so that a table of many narrow columns still counts as something.
// The point is 2,000 one-MiB values no longer forming one two-GiB batch
// (docs/reviews/2026-09-09/REVIEW.md finding 9), for a jsonb or array
// column exactly as much as a text one; it does not need to be exact to do
// that.
const rowEstimateFixed = 16

// rowEstimateMaxDepth bounds the recursion into a decoded jsonb/array value
// so a pathological nesting depth cannot make the estimate itself expensive.
// Past this depth every remaining value counts as rowEstimateFixed, same as
// any other unrecognised type.
const rowEstimateMaxDepth = 8

func rowEstimate(row []any) int {
	n := 0
	for _, v := range row {
		n += valueEstimate(v, rowEstimateMaxDepth)
	}
	return n
}

// valueEstimate is rowEstimate's per-value walk, recursing into the shapes
// pgx actually decodes a jsonb, json or array column into when scanned as
// *any: map[string]any and []any from jsonb/json, and []any or a typed
// slice ([]int64, []string, ...) from an array. depth guards against
// unbounded recursion on a deeply nested document.
func valueEstimate(v any, depth int) int {
	switch val := v.(type) {
	case string:
		return len(val)
	case []byte:
		return len(val)
	case nil:
		return rowEstimateFixed
	}
	if depth <= 0 {
		return rowEstimateFixed
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Map:
		n := 0
		iter := rv.MapRange()
		for iter.Next() {
			n += valueEstimate(iter.Key().Interface(), depth-1)
			n += valueEstimate(iter.Value().Interface(), depth-1)
		}
		return n
	case reflect.Slice, reflect.Array:
		n := 0
		for i := 0; i < rv.Len(); i++ {
			n += valueEstimate(rv.Index(i).Interface(), depth-1)
		}
		return n
	default:
		return rowEstimateFixed
	}
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
	b.bytes = 0
	b.seq++
	select {
	case b.out <- batch:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
