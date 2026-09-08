// SPDX-License-Identifier: Apache-2.0

package extract

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// The channel contract of ARCHITECTURE.md §2 "RowBatch" is what the loader
// reads and cannot re-derive: tables strictly sequential, Seq from 0 per table,
// exactly one Last per extracted table, and a boundary batch even for a table
// with no rows. It is checked here against a fake reader, because it is a
// property of this package and not of Postgres; the integration suite then
// checks the same properties against the real thing.

// ---------- fakes ----------

// fakeChunk is one unnest argument list.
type fakeChunk struct {
	casts []string
	cols  []any
	n     int
}

var _ pipeline.Chunk = fakeChunk{}

func (c fakeChunk) Len() int          { return c.n }
func (c fakeChunk) Column(i int) any  { return c.cols[i] }
func (c fakeChunk) Cast(i int) string { return c.casts[i] }

// fakeKeys is a key set of consecutive int8 keys, cut into chunks the way
// internal/plan's is.
type fakeKeys struct{ n int }

var _ pipeline.KeySet = fakeKeys{}

func (k fakeKeys) Len() int     { return k.n }
func (k fakeKeys) Bytes() int64 { return int64(k.n) * 8 * 2 }

func (k fakeKeys) chunk(start, end int) pipeline.Chunk {
	ids := make([]int64, 0, end-start)
	for i := start; i < end; i++ {
		ids = append(ids, int64(i))
	}
	return fakeChunk{casts: []string{"::int8[]"}, cols: []any{ids}, n: len(ids)}
}

func (k fakeKeys) Chunks(n int) []pipeline.Chunk {
	var out []pipeline.Chunk
	for start := 0; start < k.n; start += n {
		out = append(out, k.chunk(start, min(start+n, k.n)))
	}
	return out
}

func (k fakeKeys) EachChunk(n int, f func(pipeline.Chunk) error) error {
	for start := 0; start < k.n; start += n {
		if err := f(k.chunk(start, min(start+n, k.n))); err != nil {
			return err
		}
	}
	return nil
}

func (k fakeKeys) FirstChunk(n int) pipeline.Chunk {
	if k.n == 0 {
		return nil
	}
	return k.chunk(0, min(n, k.n))
}

// fakeRows returns rowsPerCall rows of ncols values each. err is what Err()
// answers after the rows run out, which is where pgx reports a read the server
// cancelled part-way through.
type fakeRows struct {
	rows [][]any
	err  error
	i    int
}

var _ pipeline.Rows = (*fakeRows)(nil)

func (r *fakeRows) Next() bool { r.i++; return r.i <= len(r.rows) }
func (r *fakeRows) Err() error { return r.err }
func (r *fakeRows) Close()     {}

func (r *fakeRows) Scan(dest ...any) error {
	row := r.rows[r.i-1]
	if len(dest) != len(row) {
		return fmt.Errorf("scan into %d destinations for %d values", len(dest), len(row))
	}
	for i := range dest {
		p, ok := dest[i].(*any)
		if !ok {
			return errors.New("extract must scan into *any so that pgx decodes each column to its own Go type")
		}
		*p = row[i]
	}
	return nil
}

// fakeReader answers every statement with rowsFor, and records what it was
// asked. queryErr is failed at the call, rowsErr after the rows: pgx reports a
// cancelled read either way depending on when the server said so, and extract
// has to recognise it in both places.
type fakeReader struct {
	rowsFor  func(sql string, args []any) [][]any
	queryErr error
	rowsErr  error
	sent     []string
	closed   bool
}

var _ pipeline.Reader = (*fakeReader)(nil)

func (r *fakeReader) Query(_ context.Context, sql string, args ...any) (pipeline.Rows, error) {
	r.sent = append(r.sent, sql)
	if r.queryErr != nil {
		return nil, r.queryErr
	}
	return &fakeRows{rows: r.rowsFor(sql, args), err: r.rowsErr}, nil
}

func (r *fakeReader) Close(context.Context) error { r.closed = true; return nil }

// ---------- fixtures ----------

func tbl(name string) ref.TableRef { return ref.TableRef{Schema: "public", Name: name} }

func schemaFor(tables ...pipeline.Table) *pipeline.Schema {
	return &pipeline.Schema{Tables: tables}
}

func intCol(name string) pipeline.Column {
	return pipeline.Column{Name: name, TypeName: "bigint", TypeOID: oidInt8}
}

func textCol(name string) pipeline.Column {
	return pipeline.Column{Name: name, TypeName: "text", TypeOID: oidText}
}

// drain collects every batch a run produced.
func drain(t *testing.T, e pipeline.Extractor, r pipeline.Reader, p *pipeline.Plan) ([]pipeline.RowBatch, error) {
	t.Helper()
	out := make(chan pipeline.RowBatch, 4)
	var batches []pipeline.RowBatch
	done := make(chan struct{})
	go func() {
		defer close(done)
		for b := range out {
			batches = append(batches, b)
		}
	}()
	err := e.Extract(context.Background(), r, p, out)
	<-done
	return batches, err
}

// ---------- tests ----------

func TestTablesAreSequentialAndEveryTableEndsWithLast(t *testing.T) {
	people := pipeline.Table{Ref: tbl("people"), Columns: []pipeline.Column{intCol("person_id"), textCol("email")}, PK: []string{"person_id"}}
	orders := pipeline.Table{Ref: tbl("orders"), Columns: []pipeline.Column{intCol("order_id")}, PK: []string{"order_id"}}
	empty := pipeline.Table{Ref: tbl("empty"), Columns: []pipeline.Column{intCol("id")}, PK: []string{"id"}}
	unreached := pipeline.Table{Ref: tbl("unreached"), Columns: []pipeline.Column{intCol("id")}}

	p := &pipeline.Plan{Steps: []pipeline.Step{
		{Table: people.Ref, Mode: pipeline.ChildOK, Identity: pipeline.Identity{Columns: []string{"person_id"}}, Keys: fakeKeys{n: 3}},
		{Table: orders.Ref, Mode: pipeline.ParentOnly, Identity: pipeline.Identity{Columns: []string{"order_id"}}, Keys: fakeKeys{n: 1}},
		{Table: empty.Ref, Mode: pipeline.ChildOK, Identity: pipeline.Identity{Columns: []string{"id"}}, Keys: fakeKeys{n: 0}},
		{Table: unreached.Ref, Mode: pipeline.SchemaOnly},
	}}

	r := &fakeReader{rowsFor: func(sql string, _ []any) [][]any {
		switch {
		case strings.Contains(sql, `"people"`):
			return [][]any{{int64(1), "a@example.com"}, {int64(2), "b@example.com"}, {int64(3), "c@example.com"}}
		case strings.Contains(sql, `"orders"`):
			return [][]any{{int64(9)}}
		}
		return nil
	}}

	batches, err := drain(t, New(schemaFor(people, orders, empty, unreached)), r, p)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	wantOrder := []ref.TableRef{people.Ref, orders.Ref, empty.Ref}
	var gotOrder []ref.TableRef
	seq := map[ref.TableRef]int{}
	lasts := map[ref.TableRef]int{}
	for _, b := range batches {
		if len(gotOrder) == 0 || gotOrder[len(gotOrder)-1] != b.Table {
			gotOrder = append(gotOrder, b.Table)
		}
		if b.Seq != seq[b.Table] {
			t.Errorf("%s batch out of order: Seq %d, want %d", b.Table, b.Seq, seq[b.Table])
		}
		seq[b.Table]++
		if b.Last {
			lasts[b.Table]++
		}
	}

	if len(gotOrder) != len(wantOrder) {
		t.Fatalf("tables on the channel: %v, want %v", gotOrder, wantOrder)
	}
	for i := range wantOrder {
		if gotOrder[i] != wantOrder[i] {
			t.Fatalf("tables on the channel: %v, want %v", gotOrder, wantOrder)
		}
	}
	for _, want := range wantOrder {
		if lasts[want] != 1 {
			t.Errorf("%s carried Last on %d batches, want exactly 1", want, lasts[want])
		}
	}
	if _, sent := seq[unreached.Ref]; sent {
		t.Errorf("a SchemaOnly step sent a batch; it has no data phase at all")
	}
}

func TestATableWithNoRowsStillSendsOneEmptyLastBatch(t *testing.T) {
	empty := pipeline.Table{Ref: tbl("empty"), Columns: []pipeline.Column{intCol("id")}, PK: []string{"id"}}
	p := &pipeline.Plan{Steps: []pipeline.Step{
		{Table: empty.Ref, Mode: pipeline.ChildOK, Identity: pipeline.Identity{Columns: []string{"id"}}, Keys: fakeKeys{n: 0}},
	}}
	r := &fakeReader{rowsFor: func(string, []any) [][]any { return nil }}

	batches, err := drain(t, New(schemaFor(empty)), r, p)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(batches) != 1 {
		t.Fatalf("a table with no rows sent %d batches, want 1", len(batches))
	}
	if !batches[0].Last || len(batches[0].Rows) != 0 || batches[0].Seq != 0 {
		t.Errorf("the boundary batch is %+v, want Seq 0, no rows, Last", batches[0])
	}
	if len(r.sent) != 0 {
		t.Errorf("a table with no keys was read anyway: %v", r.sent)
	}
}

func TestBatchesAreCutAtTwoThousandRows(t *testing.T) {
	big := pipeline.Table{Ref: tbl("big"), Columns: []pipeline.Column{intCol("id")}, PK: []string{"id"}}
	const rows = batchRows*2 + 7
	p := &pipeline.Plan{Steps: []pipeline.Step{
		{Table: big.Ref, Mode: pipeline.ChildOK, Identity: pipeline.Identity{Columns: []string{"id"}}, Keys: fakeKeys{n: rows}},
	}}
	r := &fakeReader{rowsFor: func(_ string, args []any) [][]any {
		ids, ok := args[0].([]int64)
		if !ok {
			t.Fatalf("a chunk argument is %T, want []int64: []any is never passed to Query", args[0])
		}
		out := make([][]any, 0, len(ids))
		for _, id := range ids {
			out = append(out, []any{id})
		}
		return out
	}}

	batches, err := drain(t, New(schemaFor(big)), r, p)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	total := 0
	for i, b := range batches {
		total += len(b.Rows)
		if len(b.Rows) > batchRows {
			t.Errorf("batch %d holds %d rows, more than the %d one batch may hold", i, len(b.Rows), batchRows)
		}
		if b.Last != (i == len(batches)-1) {
			t.Errorf("batch %d has Last=%v", i, b.Last)
		}
	}
	if total != rows {
		t.Errorf("extracted %d rows, want %d", total, rows)
	}
	wantChunks := (rows + chunkSize - 1) / chunkSize
	if len(r.sent) != wantChunks {
		t.Errorf("read %d chunks for %d keys, want %d at %d keys a chunk", len(r.sent), rows, wantChunks, chunkSize)
	}
}

func TestGeneratedColumnsAreNotCopied(t *testing.T) {
	people := pipeline.Table{
		Ref: tbl("people"),
		Columns: []pipeline.Column{
			intCol("person_id"),
			textCol("given_name"),
			{Name: "display_name", TypeName: "text", TypeOID: oidText, Generated: "(given_name)"},
		},
		PK: []string{"person_id"},
	}
	p := &pipeline.Plan{Steps: []pipeline.Step{
		{Table: people.Ref, Mode: pipeline.ChildOK, Identity: pipeline.Identity{Columns: []string{"person_id"}}, Keys: fakeKeys{n: 1}},
	}}
	r := &fakeReader{rowsFor: func(string, []any) [][]any { return [][]any{{int64(1), "Ada"}} }}

	batches, err := drain(t, New(schemaFor(people)), r, p)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	want := []string{"person_id", "given_name"}
	if got := batches[0].Cols; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("Cols = %v, want %v: a generated column is recomputed by the target and never copied", got, want)
	}
	if strings.Contains(r.sent[0], "display_name") {
		t.Errorf("the read names a generated column:\n%s", r.sent[0])
	}
}

// A lookup step has no key set, so its ordering comes from the table: the
// primary key, else a unique index, else every copied column.
func TestALookupIsReadWholeAndOrdered(t *testing.T) {
	keyed := pipeline.Table{Ref: tbl("categories"), Columns: []pipeline.Column{intCol("id"), textCol("name")}, PK: []string{"id"}}
	unkeyed := pipeline.Table{Ref: tbl("labels"), Columns: []pipeline.Column{textCol("code"), textCol("label")}}
	p := &pipeline.Plan{Steps: []pipeline.Step{
		{Table: keyed.Ref, Mode: pipeline.Lookup},
		{Table: unkeyed.Ref, Mode: pipeline.Lookup},
	}}
	r := &fakeReader{rowsFor: func(string, []any) [][]any { return [][]any{{int64(1), "x"}} }}

	if _, err := drain(t, New(schemaFor(keyed, unkeyed)), r, p); err != nil {
		t.Fatalf("Extract: %v", err)
	}
	wantSQL := []string{
		`SELECT t."id", t."name" FROM "public"."categories" t ORDER BY t."id" LIMIT 1001`,
		`SELECT t."code", t."label" FROM "public"."labels" t ORDER BY t."code", t."label" LIMIT 1001`,
	}
	for i, want := range wantSQL {
		if r.sent[i] != want {
			t.Errorf("lookup read %d:\n got %s\nwant %s", i, r.sent[i], want)
		}
	}
}

// The channel is closed on every path, including the error paths, because the
// loader reads until it closes.
func TestTheChannelIsClosedOnAnErrorPath(t *testing.T) {
	out := make(chan pipeline.RowBatch)
	go func() {
		for range out { //nolint:revive // draining is the point
		}
	}()
	err := New(nil).Extract(context.Background(), &fakeReader{}, &pipeline.Plan{}, out)
	if err == nil {
		t.Fatal("Extract with no schema returned no error")
	}
	if _, open := <-out; open {
		t.Error("the channel is still open after a failed Extract")
	}
}

// ---------- the refusal a cancelled read raises ----------

// standbyError is what PostgreSQL sends when it cancels a query on a standby
// because the snapshot the extract is holding conflicts with replay. The
// fields that quote a row are filled in on purpose: the refusal must keep the
// SQLSTATE and drop them (THREAT_MODEL.md T4).
func standbyError() *pgconn.PgError {
	return &pgconn.PgError{
		Severity: "ERROR",
		Code:     "40001",
		Message:  "canceling statement due to conflict with recovery",
		Detail:   "User query might have needed to see row versions of ada@example.com.",
		Where:    "SQL statement \"SELECT t.\"email\" FROM \"public\".\"people\"\"",
		Hint:     "In a moment you should be able to reconnect.",
	}
}

// A standby cancellation is a coded refusal, not a wrapped driver error: it is
// exit 7 under extract.refused.standby_cancelled, because the remedy is the
// standby's max_standby_streaming_delay or a run against the primary and not
// the operator's SQL (ADR-005). pgx reports it either at the call or after the
// rows, so both are here.
func TestAStandbyCancellationIsACodedExitSeven(t *testing.T) {
	people := pipeline.Table{Ref: tbl("people"), Columns: []pipeline.Column{intCol("person_id"), textCol("email")}, PK: []string{"person_id"}}
	p := &pipeline.Plan{Steps: []pipeline.Step{
		{Table: people.Ref, Mode: pipeline.ChildOK, Identity: pipeline.Identity{Columns: []string{"person_id"}}, Keys: fakeKeys{n: 3}},
	}}

	for name, reader := range map[string]*fakeReader{
		"the server cancelled the query at the call": {
			rowsFor:  func(string, []any) [][]any { return nil },
			queryErr: fmt.Errorf("reading rows: %w", standbyError()),
		},
		"the server cancelled the query part-way through the rows": {
			rowsFor: func(string, []any) [][]any { return [][]any{{int64(1), "a@example.com"}} },
			rowsErr: standbyError(),
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := drain(t, New(schemaFor(people)), reader, p)
			if err == nil {
				t.Fatal("Extract returned no error for a cancelled read")
			}
			var r *Refusal
			if !errors.As(err, &r) {
				t.Fatalf("Extract returned %T (%v), want an *extract.Refusal", err, err)
			}
			if r.Code != CodeStandbyCancelled {
				t.Errorf("the refusal carries code %q, want %q", r.Code, CodeStandbyCancelled)
			}
			if r.Exit != 7 {
				t.Errorf("the refusal exits %d, want 7 (ADR-005: extract or load)", r.Exit)
			}
			if r.Table != people.Ref {
				t.Errorf("the refusal names %s, want %s", r.Table, people.Ref)
			}
			if r.SQLState != "40001" {
				t.Errorf("the refusal carries SQLSTATE %q, want 40001", r.SQLState)
			}
			// Nothing that quotes a row survives into the error string, which is
			// a way out of the process.
			e := standbyError()
			for _, leak := range []string{e.Detail, e.Where, e.Hint, "ada@example.com"} {
				if strings.Contains(r.Error(), leak) {
					t.Errorf("the refusal carries %q:\n%s", leak, r.Error())
				}
			}
		})
	}
}

// Every other read failure is a wrapped error naming the table, and not a
// refusal: a code is a promise that core can render a remedy from the
// catalogue, and there is no remedy for an undefined table.
func TestAnyOtherReadFailureIsNotARefusal(t *testing.T) {
	people := pipeline.Table{Ref: tbl("people"), Columns: []pipeline.Column{intCol("person_id"), textCol("email")}, PK: []string{"person_id"}}
	p := &pipeline.Plan{Steps: []pipeline.Step{
		{Table: people.Ref, Mode: pipeline.ChildOK, Identity: pipeline.Identity{Columns: []string{"person_id"}}, Keys: fakeKeys{n: 1}},
	}}
	for name, err := range map[string]error{
		"a server error with another SQLSTATE": &pgconn.PgError{Code: "42P01", Message: "relation does not exist"},
		"an error that is not the server's":    errors.New("connection reset by peer"),
	} {
		t.Run(name, func(t *testing.T) {
			r := &fakeReader{rowsFor: func(string, []any) [][]any { return nil }, queryErr: err}
			_, got := drain(t, New(schemaFor(people)), r, p)
			if got == nil {
				t.Fatal("Extract returned no error for a failed read")
			}
			var refusal *Refusal
			if errors.As(got, &refusal) {
				t.Fatalf("a read failure with no code became %v", refusal)
			}
			if !strings.Contains(got.Error(), people.Ref.String()) {
				t.Errorf("the error does not name the table it was reading: %v", got)
			}
		})
	}
}

// ---------- what bounds the memory ----------

// releasingKeys hands out chunks that report when they are collected, so the
// claim "one chunk is live at a time" can be checked rather than asserted.
//
// pipeline.KeySet.Chunks builds every chunk in one call and each holds its own
// copy of the keys (internal/plan's keyset.go), so ranging over the call
// directly keeps a second copy of the whole key set alive for the length of the
// table. Chunks is still what this fake's Chunks does; EachChunk builds one
// chunk at a time, which is what extract takes, and this test is what says the
// consumed ones then go. That is invisible to the 2,000,000-row integration
// test, which is int8-keyed and passes on margin; this is the part of it a unit
// test can hold.
type releasingKeys struct {
	n        int
	released *atomic.Int64
}

var _ pipeline.KeySet = releasingKeys{}

func (k releasingKeys) Len() int     { return k.n }
func (k releasingKeys) Bytes() int64 { return int64(k.n) * 16 }

// payload stands for the typed array a real chunk copies its keys into.
type payload struct{ ids []int64 }

func (k releasingKeys) chunk(start, end int) pipeline.Chunk {
	p := &payload{ids: make([]int64, 0, end-start)}
	for i := start; i < end; i++ {
		p.ids = append(p.ids, int64(i))
	}
	runtime.AddCleanup(p, func(c *atomic.Int64) { c.Add(1) }, k.released)
	return releasingChunk{p: p}
}

func (k releasingKeys) Chunks(n int) []pipeline.Chunk {
	var out []pipeline.Chunk
	for start := 0; start < k.n; start += n {
		out = append(out, k.chunk(start, min(start+n, k.n)))
	}
	return out
}

func (k releasingKeys) EachChunk(n int, f func(pipeline.Chunk) error) error {
	for start := 0; start < k.n; start += n {
		if err := f(k.chunk(start, min(start+n, k.n))); err != nil {
			return err
		}
	}
	return nil
}

func (k releasingKeys) FirstChunk(n int) pipeline.Chunk {
	if k.n == 0 {
		return nil
	}
	return k.chunk(0, min(n, k.n))
}

type releasingChunk struct{ p *payload }

var _ pipeline.Chunk = releasingChunk{}

func (c releasingChunk) Len() int        { return len(c.p.ids) }
func (c releasingChunk) Column(int) any  { return c.p.ids }
func (c releasingChunk) Cast(int) string { return "::int8[]" }

// A chunk is dropped as it is consumed, so a table of many chunks does not hold
// a second copy of its key set until the last row is read.
func TestAChunkIsReleasedAsItIsConsumed(t *testing.T) {
	people := pipeline.Table{Ref: tbl("people"), Columns: []pipeline.Column{intCol("person_id")}, PK: []string{"person_id"}}
	var released atomic.Int64
	const chunks = 6
	p := &pipeline.Plan{Steps: []pipeline.Step{{
		Table:    people.Ref,
		Mode:     pipeline.ChildOK,
		Identity: pipeline.Identity{Columns: []string{"person_id"}},
		Keys:     releasingKeys{n: chunkSize * chunks, released: &released},
	}}}

	seen := 0
	r := &fakeReader{rowsFor: func(string, []any) [][]any {
		seen++
		if seen == chunks {
			// On the last read, every chunk but this one has been consumed.
			// Under Chunks they would all still be reachable through the slice
			// that call returned; under EachChunk nothing holds them.
			for range 20 {
				runtime.GC()
				if released.Load() > 0 {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			if released.Load() == 0 {
				t.Errorf("%d chunks were consumed and none was released; extract is holding "+
					"a second copy of the whole key set", chunks-1)
			}
		}
		return nil
	}}

	if _, err := drain(t, New(schemaFor(people)), r, p); err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if seen != chunks {
		t.Fatalf("read %d chunks, want %d", seen, chunks)
	}
}
