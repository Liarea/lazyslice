// SPDX-License-Identifier: Apache-2.0

package load

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/load/ddl"
	"github.com/Liarea/lazyslice/internal/pg"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// What the target ends up holding is a statement about a real Postgres and is
// asserted in load_integration_test.go. What is asserted here is the part that
// is a statement about the contract in ARCHITECTURE.md §2: one transaction per
// table, opened at Seq 0 and committed at Last, with the marker row written
// before the first drop. That property is THREAT_MODEL.md T8's, and a fake
// target is the only place a rollback can be observed without racing a server.

func tref(schema, name string) ref.TableRef { return ref.TableRef{Schema: schema, Name: name} }

// fakeWriter is a pipeline.Writer that records what was asked of it.
type fakeWriter struct {
	mu         sync.Mutex
	statements []string
	// args are the bind parameters of the statement at the same index. The
	// marker's status is a parameter and not text, so what a run recorded about
	// itself is only readable here.
	args     [][]any
	txs      []*fakeTx
	failCopy map[ref.TableRef]error
	failExec func(sql string) error
	// answer is how a test says what the target replies to the reads the
	// lock-and-recheck makes inside a drop's transaction (T-0130), or to
	// T-0242's whole-target recheck that runs before it. nil keeps the
	// default below: a target that holds no tables at all.
	answer func(sql string, args []any) (pipeline.Rows, error)
}

func (w *fakeWriter) Exec(_ context.Context, sql string, args ...any) error {
	w.mu.Lock()
	w.statements = append(w.statements, sql)
	w.args = append(w.args, args)
	w.mu.Unlock()
	if w.failExec != nil {
		return w.failExec(sql)
	}
	return nil
}

func (w *fakeWriter) CopyFrom(context.Context, ref.TableRef, []string, <-chan []any) (int64, error) {
	return 0, errors.New("the loader must copy inside a transaction, never on the writer")
}

// RegisterTypes is pipeline.Writer's fourth method (T-0093): every Writer the
// loader is handed can register the source's user-defined types, and a Writer
// that cannot no longer compiles, because an optional interface that misses is
// a load step that disappears without a compile error.
// registeringWriter below overrides this to record when it was called.
func (w *fakeWriter) RegisterTypes(context.Context, *pipeline.Schema) error { return nil }

func (w *fakeWriter) Begin(context.Context) (pipeline.Tx, error) {
	tx := &fakeTx{w: w}
	w.mu.Lock()
	w.txs = append(w.txs, tx)
	w.mu.Unlock()
	return tx, nil
}

// copyTxs are the transactions a table's rows went into.
//
// Since T-0130 the drops open transactions of their own — one per table, taking
// its ACCESS EXCLUSIVE lock and re-verifying what the gate approved before the
// DROP (ARCHITECTURE.md §11.2) — so `one transaction per table` is a statement
// about the copies and this is how a test says so. A transaction that reached
// CopyFrom is a copy's, whether or not the copy succeeded.
func (w *fakeWriter) copyTxs() []*fakeTx {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]*fakeTx, 0, len(w.txs))
	for _, tx := range w.txs {
		if tx.copied {
			out = append(out, tx)
		}
	}
	return out
}

func (w *fakeWriter) log() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]string(nil), w.statements...)
}

// lastArgs are the bind parameters of the last statement the writer was given.
func (w *fakeWriter) lastArgs() []any {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.args) == 0 {
		return nil
	}
	return w.args[len(w.args)-1]
}

type fakeTx struct {
	w         *fakeWriter
	table     ref.TableRef
	cols      []string
	rows      int64
	committed bool
	rolled    bool
	// copied marks a transaction a table's rows went into, which is what tells
	// it apart from the transaction each drop now opens (copyTxs).
	copied bool
	local  []string
}

// Exec records on the transaction and on the writer behind it.
//
// The writer's log is the one place a test can read the whole run in order, and
// since T-0130 the drops happen inside a transaction rather than on the writer
// (ARCHITECTURE.md §11.2's lock-and-recheck): without the forwarding, the two
// tests that assert the marker is written before the first drop, and that every
// drop is announced before it runs, would be asserting over a log the drops had
// left. failExec is applied here for the same reason — it is how a test makes
// one statement fail, and a DROP is now one of the statements it has to reach.
func (tx *fakeTx) Exec(_ context.Context, sql string, args ...any) error {
	tx.local = append(tx.local, sql)
	tx.w.mu.Lock()
	tx.w.statements = append(tx.w.statements, sql)
	tx.w.args = append(tx.w.args, args)
	tx.w.mu.Unlock()
	if tx.w.failExec != nil {
		return tx.w.failExec(sql)
	}
	return nil
}

// Query is pipeline.Tx's read, which the lock-and-recheck needs (T-0130).
//
// With no answer set the fake target holds no tables at all: T-0242's
// whole-target recheck (pg.ProbeEmptiness) lists none, so it finds nothing
// occupied and never refuses; to_regclass then answers "not there" for
// dropOne's own per-table recheck, which is not reached either, and a
// whole-Load test is about the transaction contract rather than about either
// recheck. A test that is about the per-table recheck sets fakeWriter.answer
// (recheckAnswers below) and drives dropOne or dropTable directly; a test
// about the whole-target recheck sets it too (see TestARecheckOfTheWholeTarget
// below); what a *real* target answers under a real lock is asserted in
// load_integration_test.go and in internal/core's race suite.
func (tx *fakeTx) Query(_ context.Context, sql string, args ...any) (pipeline.Rows, error) {
	tx.local = append(tx.local, sql)
	if tx.w.answer != nil {
		return tx.w.answer(sql, args)
	}
	if strings.Contains(sql, "FROM pg_class c") {
		return &fakeRows{none: true}, nil
	}
	return &fakeRows{values: []any{false}}, nil
}

// recheckAnswers is a target's replies to the three reads a drop can make:
// to_regclass, the row count of an approved-empty table, and the status of the
// marker row that authorised a truncation. An empty status means that row is
// gone, which is one of the two things CodeRefusedMarkerChanged is for.
type recheckAnswers struct {
	present bool
	count   int64
	status  string
}

func (a recheckAnswers) answer(sql string, _ []any) (pipeline.Rows, error) {
	switch {
	case strings.Contains(sql, "to_regclass"):
		return &fakeRows{values: []any{a.present}}, nil
	case strings.Contains(sql, "FROM "+pg.MarkerTable):
		if a.status == "" {
			return &fakeRows{none: true}, nil
		}
		return &fakeRows{values: []any{a.status}}, nil
	case strings.Contains(sql, "count(*)"):
		return &fakeRows{values: []any{a.count}}, nil
	}
	return nil, errors.New("the fake target was asked something the recheck does not ask: " + sql)
}

// fakeRows is one row of one column, or none when the query matched nothing.
type fakeRows struct {
	values []any
	none   bool
	done   bool
}

func (r *fakeRows) Next() bool {
	if r.none || r.done {
		return false
	}
	r.done = true
	return true
}

func (r *fakeRows) Scan(dest ...any) error {
	if len(dest) != len(r.values) {
		return errors.New("the fake target was asked for a different number of columns")
	}
	for i, d := range dest {
		switch target := d.(type) {
		case *bool:
			v, ok := r.values[i].(bool)
			if !ok {
				return errors.New("the fake target was asked for a bool it does not hold")
			}
			*target = v
		case *int64:
			v, ok := r.values[i].(int64)
			if !ok {
				return errors.New("the fake target was asked for a count it does not hold")
			}
			*target = v
		case *string:
			v, ok := r.values[i].(string)
			if !ok {
				return errors.New("the fake target was asked for a string it does not hold")
			}
			*target = v
		default:
			return errors.New("the fake target holds no column of that type")
		}
	}
	return nil
}

func (r *fakeRows) Err() error { return nil }

func (r *fakeRows) Close() {}

func (tx *fakeTx) CopyFrom(ctx context.Context, table ref.TableRef, cols []string, rows <-chan []any) (int64, error) {
	tx.table = table
	tx.cols = cols
	tx.copied = true
	if err := tx.w.failCopy[table]; err != nil {
		return 0, err
	}
	var n int64
	for {
		select {
		case _, ok := <-rows:
			if !ok {
				return n, nil
			}
			n++
		case <-ctx.Done():
			return n, ctx.Err()
		}
	}
}

func (tx *fakeTx) Commit(context.Context) error {
	tx.committed = true
	tx.rows = 0
	return nil
}

func (tx *fakeTx) Rollback(context.Context) error {
	tx.rolled = true
	return nil
}

func testPlan() *pipeline.Plan {
	return &pipeline.Plan{Root: tref("public", "orders"), Take: 500}
}

func testSchema() *pipeline.Schema {
	return &pipeline.Schema{
		Tables: []pipeline.Table{
			{Ref: tref("public", "orders"), Columns: []pipeline.Column{{Name: "id", TypeName: "integer"}}},
			{Ref: tref("public", "order_items"), Columns: []pipeline.Column{{Name: "id", TypeName: "integer"}}},
		},
	}
}

// feed sends the batches and closes the channel, as extract does.
func feed(batches ...pipeline.RowBatch) <-chan pipeline.RowBatch {
	ch := make(chan pipeline.RowBatch, len(batches))
	for _, b := range batches {
		ch <- b
	}
	close(ch)
	return ch
}

func row(n int) []any { return []any{n} }

// ARCHITECTURE.md §2, RowBatch: the loader opens T's transaction on Seq 0 and
// commits it on Last; it never infers a boundary from a change of Table. Two
// tables in one transaction would make a failure at the second leave the first
// uncommitted too, which is the opposite of the property T8 names.
func TestOneTransactionPerTableCommittedAtLast(t *testing.T) {
	w := &fakeWriter{}
	l := New(Run{ToolVersion: "test"}, nil)

	orders, items := tref("public", "orders"), tref("public", "order_items")
	res, err := l.Load(context.Background(), w, testPlan(), testSchema(), feed(
		pipeline.RowBatch{Table: orders, Cols: []string{"id"}, Rows: [][]any{row(1), row(2)}, Seq: 0},
		pipeline.RowBatch{Table: orders, Cols: []string{"id"}, Rows: [][]any{row(3)}, Seq: 1, Last: true},
		pipeline.RowBatch{Table: items, Cols: []string{"id"}, Rows: [][]any{row(4)}, Seq: 0, Last: true},
	))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	txs := w.copyTxs()
	if len(txs) != 2 {
		t.Fatalf("expected one transaction per table, got %d", len(txs))
	}
	for i, tx := range txs {
		if !tx.committed || tx.rolled {
			t.Errorf("transaction %d: committed=%v rolled=%v", i, tx.committed, tx.rolled)
		}
		if len(tx.local) != 1 || !strings.Contains(tx.local[0], "synchronous_commit") {
			t.Errorf("transaction %d did not set synchronous_commit off: %v", i, tx.local)
		}
	}
	if txs[0].table != orders || txs[1].table != items {
		t.Errorf("the transactions are for %v and %v", txs[0].table, txs[1].table)
	}
	if res.Rows[orders] != 3 || res.Rows[items] != 1 {
		t.Errorf("row counts are %v", res.Rows)
	}
}

// ARCHITECTURE.md §11.2: every run inserts its row with status = running before
// the first drop. Without that ordering a run killed between the first drop and
// the marker leaves a target with tables gone and nothing saying who did it, and
// the next run's gate refuses it as a non-empty stranger.
//
// A successful Load no longer closes that row itself (T-0133, 2026-09-14,
// THREAT_MODEL.md T8 amendment): it used to send the UPDATE last, before core
// had run verify at all, which is docs/reviews/2026-09-09 finding 4 — a marker
// saying complete over a target a later verify failure found still held
// personal data. Closing the row is core's job now, once verify has had its
// say, so a Load that succeeds sends no UPDATE to lazyslice_meta at all and
// leaves the row at running for core to close.
func TestTheMarkerRowIsWrittenBeforeTheFirstDrop(t *testing.T) {
	w := &fakeWriter{}
	l := New(Run{ToolVersion: "test"}, nil)
	if _, err := l.Load(context.Background(), w, testPlan(), testSchema(), feed()); err != nil {
		t.Fatalf("Load: %v", err)
	}
	insert, drop := -1, -1
	for i, s := range w.log() {
		if insert < 0 && strings.HasPrefix(s, "INSERT INTO lazyslice_meta") {
			insert = i
		}
		if drop < 0 && strings.HasPrefix(s, "DROP TABLE") {
			drop = i
		}
		if strings.HasPrefix(s, "UPDATE lazyslice_meta") {
			t.Errorf("a successful Load sent %q; closing the marker row is core's job since T-0133,"+
				" once verify has passed", s)
		}
	}
	if insert < 0 || drop < 0 {
		t.Fatalf("expected an insert into the marker and a drop; got %v", w.log())
	}
	if insert > drop {
		t.Errorf("the marker row is written at statement %d, after the first drop at %d", insert, drop)
	}
}

// §11.1 requires each drop to be printed before it happens, which is why
// load.New takes a sink at all.
func TestEachDropIsAnnouncedBeforeItRuns(t *testing.T) {
	var seen []string
	w := &fakeWriter{}
	w.failExec = func(sql string) error {
		if strings.HasPrefix(sql, "DROP TABLE") {
			seen = append(seen, "ran "+sql)
		}
		return nil
	}
	sink := event.SinkFunc(func(e event.Event) {
		if e.Code == CodeDropping {
			seen = append(seen, `said "`+e.Table.Schema+`"."`+e.Table.Name+`"`)
		}
	})
	l := New(Run{ToolVersion: "test"}, sink)
	if _, err := l.Load(context.Background(), w, testPlan(), testSchema(), feed()); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(seen) < 2 {
		t.Fatalf("expected an announcement and a drop, got %v", seen)
	}
	for i := 0; i < len(seen); i += 2 {
		if !strings.HasPrefix(seen[i], "said ") || !strings.HasPrefix(seen[i+1], "ran ") {
			t.Fatalf("a drop ran before it was announced: %v", seen)
		}
		if !strings.Contains(seen[i+1], strings.TrimPrefix(seen[i], "said ")) {
			t.Fatalf("the announcement and the drop name different tables: %v", seen[i:i+2])
		}
	}
}

// THREAT_MODEL.md T8: a failure leaves the table empty, never half loaded. The
// transaction is rolled back, the following table is never opened, and the error
// carries ADR-005's exit 7.
func TestACopyFailureRollsBackAndStopsTheLoad(t *testing.T) {
	orders, items := tref("public", "orders"), tref("public", "order_items")
	w := &fakeWriter{failCopy: map[ref.TableRef]error{orders: errors.New("disk full")}}
	l := New(Run{ToolVersion: "test"}, nil)

	_, err := l.Load(context.Background(), w, testPlan(), testSchema(), feed(
		pipeline.RowBatch{Table: orders, Cols: []string{"id"}, Rows: [][]any{row(1)}, Seq: 0, Last: true},
		pipeline.RowBatch{Table: items, Cols: []string{"id"}, Rows: [][]any{row(2)}, Seq: 0, Last: true},
	))
	var refusal *Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("expected a *Refusal, got %v", err)
	}
	if refusal.Code != CodeRefusedCopy || refusal.Exit != exitLoad || refusal.Table != orders {
		t.Errorf("the refusal is %s exit %d on %v", refusal.Code, refusal.Exit, refusal.Table)
	}
	txs := w.copyTxs()
	if len(txs) != 1 {
		t.Fatalf("the load opened %d transactions after the first one failed", len(txs))
	}
	if txs[0].committed || !txs[0].rolled {
		t.Errorf("the failed table was committed=%v rolled=%v", txs[0].committed, txs[0].rolled)
	}
	log := w.log()
	if !strings.HasPrefix(log[len(log)-1], "UPDATE lazyslice_meta") {
		t.Errorf("a failed run did not close its marker row; last statement %q", log[len(log)-1])
	}
}

// The boundary is Seq and Last, never a change of Table. A stream that breaks
// the contract is an error rather than a guess, because guessing is how a table
// is committed in two halves.
func TestABatchStreamThatBreaksTheContractIsAnError(t *testing.T) {
	orders, items := tref("public", "orders"), tref("public", "order_items")
	cases := []struct {
		name    string
		batches []pipeline.RowBatch
		want    string
	}{
		{
			name: "a table opens at a sequence other than zero",
			batches: []pipeline.RowBatch{
				{Table: orders, Cols: []string{"id"}, Rows: [][]any{row(1)}, Seq: 1, Last: true},
			},
			want: "opens at Seq",
		},
		{
			name: "a second table arrives before the first was marked last",
			batches: []pipeline.RowBatch{
				{Table: orders, Cols: []string{"id"}, Rows: [][]any{row(1)}, Seq: 0},
				{Table: items, Cols: []string{"id"}, Rows: [][]any{row(2)}, Seq: 0, Last: true},
			},
			want: "was marked last",
		},
		{
			name: "the stream ends without a last batch",
			batches: []pipeline.RowBatch{
				{Table: orders, Cols: []string{"id"}, Rows: [][]any{row(1)}, Seq: 0},
			},
			want: "ended without one marked last",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := &fakeWriter{}
			l := New(Run{ToolVersion: "test"}, nil)
			_, err := l.Load(context.Background(), w, testPlan(), testSchema(), feed(c.batches...))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("expected an error containing %q, got %v", c.want, err)
			}
			for i, tx := range w.copyTxs() {
				if tx.committed {
					t.Errorf("transaction %d was committed on a broken stream", i)
				}
			}
		})
	}
}

// THREAT_MODEL.md T8's v1-blocking control is "a target is never left with some
// tables loaded and status = complete, because complete is written last".
// internal/extract closes the batch channel on every path, its error returns
// included, so an extract that dies between tables reaches the loader as an
// orderly end of channel: an early prefix committed, the rest created and empty,
// every foreign key validating because an empty child satisfies any foreign key.
// The channel closing is not the stream finishing, and the plan is what says so.
func TestAStreamThatEndsBetweenTablesIsALoadFailure(t *testing.T) {
	orders, items := tref("public", "orders"), tref("public", "order_items")
	plan := testPlan()
	plan.Steps = []pipeline.Step{
		{Table: orders, Mode: pipeline.ChildOK},
		{Table: items, Mode: pipeline.ChildOK},
	}

	w := &fakeWriter{}
	l := New(Run{ToolVersion: "test"}, nil)
	_, err := l.Load(context.Background(), w, plan, testSchema(), feed(
		pipeline.RowBatch{Table: orders, Cols: []string{"id"}, Rows: [][]any{row(1)}, Seq: 0, Last: true},
	))
	if err == nil || !strings.Contains(err.Error(), "ended before") {
		t.Fatalf("expected the load to refuse a stream that stopped after the first table, got %v", err)
	}
	log := w.log()
	for _, s := range log {
		if strings.HasPrefix(s, "ANALYZE") {
			t.Errorf("the post-data ran on a half-copied target: %s", s)
		}
	}
	last := log[len(log)-1]
	if !strings.HasPrefix(last, "UPDATE lazyslice_meta") {
		t.Errorf("the marker was not closed; the last statement is %q", last)
	}
	if !slices.Contains(w.lastArgs(), any(pg.StatusFailed)) {
		t.Errorf("the marker was closed with %v, and a stream that stopped early is a failed run", w.lastArgs())
	}
}

// A step the plan marks SchemaOnly sends no batch at all (§2, §11.1), so it is
// not a missing boundary: the table is recreated and left empty on purpose.
func TestASchemaOnlyStepNeedsNoBatch(t *testing.T) {
	orders, items := tref("public", "orders"), tref("public", "order_items")
	plan := testPlan()
	plan.Steps = []pipeline.Step{
		{Table: orders, Mode: pipeline.ChildOK},
		{Table: items, Mode: pipeline.SchemaOnly},
	}
	w := &fakeWriter{}
	l := New(Run{ToolVersion: "test"}, nil)
	if _, err := l.Load(context.Background(), w, plan, testSchema(), feed(
		pipeline.RowBatch{Table: orders, Cols: []string{"id"}, Rows: [][]any{row(1)}, Seq: 0, Last: true},
	)); err != nil {
		t.Fatalf("Load: %v", err)
	}
}

// A table whose every column is generated is copied as zero columns: the target
// recomputes all of them (§11.1), and COPY with an empty column list is not a
// statement. It still gets its boundary batch and a row count.
func TestATableWithNoCopiedColumnsOpensNoTransaction(t *testing.T) {
	w := &fakeWriter{}
	l := New(Run{ToolVersion: "test"}, nil)
	orders := tref("public", "orders")
	res, err := l.Load(context.Background(), w, testPlan(), testSchema(), feed(
		pipeline.RowBatch{Table: orders, Seq: 0, Last: true},
	))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if n := len(w.copyTxs()); n != 0 {
		t.Errorf("a table with no copied columns opened %d transactions", n)
	}
	if n, ok := res.Rows[orders]; !ok || n != 0 {
		t.Errorf("the table is missing from the row counts: %v", res.Rows)
	}
}

// The gate's fingerprinter asks for the schema-only read when the introspector
// offers one, and that choice is a runtime type assertion with a silent
// fall-back: an introspector that arrives wrapped satisfies
// pipeline.Introspector and not pipeline.SchemaOnlyIntrospector, so the gate
// would go back to taking a TABLESAMPLE per table out of a database it has not
// yet agreed to touch (THREAT_MODEL.md T4) with nothing failing. These two
// tests are what fails instead.

// recordingIntrospector answers both reads with the same schema and records
// which one was asked for. It implements Introspect only; schemaOnlyIntrospector
// below adds the second method, so the two fakes differ in exactly the interface
// the assertion tests for.
type recordingIntrospector struct {
	schema *pipeline.Schema
	calls  []string
}

func (r *recordingIntrospector) Introspect(context.Context, pipeline.Reader) (*pipeline.Schema, error) {
	r.calls = append(r.calls, "Introspect")
	return r.schema, nil
}

type schemaOnlyIntrospector struct{ *recordingIntrospector }

func (r schemaOnlyIntrospector) IntrospectSchema(context.Context, pipeline.Reader) (*pipeline.Schema, error) {
	r.calls = append(r.calls, "IntrospectSchema")
	return r.schema, nil
}

func TestGateFingerprintAsksForTheSchemaOnlyRead(t *testing.T) {
	rec := &recordingIntrospector{schema: testSchema()}
	got, err := gateFingerprint(schemaOnlyIntrospector{rec})(context.Background(), nil)
	if err != nil {
		t.Fatalf("the gate's fingerprinter: %v", err)
	}
	if want := []string{"IntrospectSchema"}; !slices.Equal(rec.calls, want) {
		t.Fatalf("the gate called %v, want %v: an introspector that offers the schema-only read must not be sampled", rec.calls, want)
	}
	want, err := SchemaFingerprint(rec.schema)
	if err != nil {
		t.Fatalf("SchemaFingerprint: %v", err)
	}
	if got != want {
		t.Errorf("the gate's fingerprint = %q, want %q", got, want)
	}
}

// The fall-back is correct and only slower: the fields the full read adds are
// fields SchemaFingerprint does not hash, so both spellings give the marker's
// end and this one the same value. What it is not is free, which is why the
// test above exists.
func TestGateFingerprintFallsBackToTheFullRead(t *testing.T) {
	rec := &recordingIntrospector{schema: testSchema()}
	got, err := gateFingerprint(rec)(context.Background(), nil)
	if err != nil {
		t.Fatalf("the gate's fingerprinter: %v", err)
	}
	if want := []string{"Introspect"}; !slices.Equal(rec.calls, want) {
		t.Fatalf("the gate called %v, want %v", rec.calls, want)
	}
	viaSchemaOnly := &recordingIntrospector{schema: testSchema()}
	same, err := gateFingerprint(schemaOnlyIntrospector{viaSchemaOnly})(context.Background(), nil)
	if err != nil {
		t.Fatalf("the gate's fingerprinter: %v", err)
	}
	if got != same {
		t.Errorf("the full read hashed to %q and the schema-only read to %q; §11.2's binding needs one value", got, same)
	}
}

// A Target with no fingerprinter can never find a marker bound; an introspector
// that is nil is the same refusal one layer up.
func TestGateFingerprintWithNoIntrospectorFailsClosed(t *testing.T) {
	if _, err := gateFingerprint(nil)(context.Background(), nil); err == nil {
		t.Fatal("the gate's fingerprinter returned a fingerprint with no introspector to read the catalog")
	}
}

// registeringWriter is a fakeWriter whose RegisterTypes records when the loader
// called it, so that the ordering registerTypes exists for can be observed.
type registeringWriter struct {
	fakeWriter
	// at is the number of statements the writer had seen when RegisterTypes was
	// called; -1 until it is called.
	at     int
	schema *pipeline.Schema
	err    error
}

func newRegisteringWriter() *registeringWriter { return &registeringWriter{at: -1} }

func (w *registeringWriter) RegisterTypes(_ context.Context, s *pipeline.Schema) error {
	w.mu.Lock()
	w.at = len(w.statements)
	w.mu.Unlock()
	w.schema = s
	return w.err
}

// ARCHITECTURE.md §11.1 item 3 creates the source's enums, domains and
// composites in the target, and ADR-005 then loads with "types registered in
// AfterConnect". The registration has to sit between the two: before it the
// types do not exist in the target, and after the first CopyFrom is too late,
// because a value of one of them has no encode plan and the copy fails
// mid-table (54000 for an enum array, 42804 for a composite — measured, and
// testdata/nasty.sql trap 27 is the fixture).
//
// That the call happens at all is the compiler's business since T-0093 —
// RegisterTypes is one of pipeline.Writer's four methods — but *when* it happens
// is not, and a registration moved after the first CopyFrom would fail only in
// the integration suite, on the two tables of trap 27. This is what holds the
// ordering.
func TestLoadRegistersTheSourcesUserTypesBeforeTheFirstCopy(t *testing.T) {
	w := newRegisteringWriter()
	l := New(Run{ToolVersion: "test"}, nil)
	schema := testSchema()

	orders, items := tref("public", "orders"), tref("public", "order_items")
	if _, err := l.Load(context.Background(), w, testPlan(), schema, feed(
		pipeline.RowBatch{Table: orders, Cols: []string{"id"}, Rows: [][]any{row(1)}, Seq: 0, Last: true},
		pipeline.RowBatch{Table: items, Cols: []string{"id"}, Rows: [][]any{row(2)}, Seq: 0, Last: true},
	)); err != nil {
		t.Fatalf("Load: %v", err)
	}

	if w.at < 0 {
		t.Fatal("Load never registered the source's user-defined types on the target")
	}
	if w.schema != schema {
		t.Errorf("RegisterTypes was given %p, want the schema being loaded (%p)", w.schema, schema)
	}

	// The registration must come after the last CREATE TYPE and before the
	// first transaction, which is the first CopyFrom's.
	log := w.log()
	lastCreate := -1
	for i, s := range log[:w.at] {
		if strings.HasPrefix(s, "CREATE TYPE") || strings.HasPrefix(s, "CREATE DOMAIN") ||
			strings.HasPrefix(s, "CREATE TABLE") {
			lastCreate = i
		}
	}
	if lastCreate < 0 {
		t.Errorf("no CREATE ran before the registration; the target had no types to register: %v", log[:w.at])
	}
	if len(w.copyTxs()) == 0 {
		t.Fatal("the load opened no transaction, so there was no copy for the registration to precede")
	}
}

// A registration that fails fails the load, before any row is copied. The
// alternative is a run that carries on and fails at CopyFrom with the driver's
// own words about a value (THREAT_MODEL.md T4).
func TestALoadWhoseTypeRegistrationFailsCopiesNothing(t *testing.T) {
	w := newRegisteringWriter()
	w.err = errors.New("no codec for public.money_amount")
	l := New(Run{ToolVersion: "test"}, nil)

	orders := tref("public", "orders")
	_, err := l.Load(context.Background(), w, testPlan(), testSchema(), feed(
		pipeline.RowBatch{Table: orders, Cols: []string{"id"}, Rows: [][]any{row(1)}, Seq: 0, Last: true},
	))
	if err == nil {
		t.Fatal("Load succeeded though the type registration failed")
	}
	if !strings.Contains(err.Error(), "money_amount") {
		t.Errorf("Load failed with %q, want the registration's own reason", err)
	}
	if n := len(w.copyTxs()); n != 0 {
		t.Errorf("the load opened %d transactions after a failed registration, want none", n)
	}
	last := w.log()[len(w.log())-1]
	if !strings.HasPrefix(last, "UPDATE lazyslice_meta") {
		t.Errorf("the marker was not closed after the failure; the last statement is %q", last)
	}
}

// The lock-and-recheck's own refusals (T-0130, ARCHITECTURE.md §11.2).
//
// These drive dropOne and dropTable directly rather than Load, because the
// branch each is about is reached only when the target still holds the table,
// and a fake target that says so through a whole Load would be a fake target
// pretending to be a database for the rest of the run as well. What is asserted
// is the part that is a statement about this package: which of the three
// refusals is raised, at which exit, with which count, and that the transaction
// rolled back with no DROP in it. That a real lock is taken, and that a real
// concurrent writer is what trips this, is asserted in internal/core's race
// suite against a real server.

// dropOf is the drop statement for one table, as ddl.DropTables writes it.
func dropOf(table ref.TableRef) ddl.TableDrop {
	return ddl.TableDrop{Table: table, SQL: "DROP TABLE IF EXISTS " + ddl.TableName(table) + " CASCADE"}
}

// refusalFrom is the *Refusal an error must be, or the test fails here.
func refusalFrom(t *testing.T, err error) *Refusal {
	t.Helper()

	var r *Refusal
	if !errors.As(err, &r) {
		t.Fatalf("the drop returned %v, want a *Refusal", err)
	}
	return r
}

// requireNothingDropped fails unless the transaction rolled back without ever
// issuing its DROP. A refusal that cost the table is not a refusal.
func requireNothingDropped(t *testing.T, w *fakeWriter) {
	t.Helper()

	if len(w.txs) == 0 {
		t.Fatal("the drop opened no transaction")
	}
	for _, tx := range w.txs {
		if tx.committed {
			t.Error("the refusing transaction committed")
		}
		if !tx.rolled {
			t.Error("the refusing transaction was not rolled back, so the table is left locked until the connection ends")
		}
		for _, sql := range tx.local {
			if strings.HasPrefix(sql, "DROP TABLE") {
				t.Errorf("the refusal ran %q; a refusal must cost nothing", sql)
			}
		}
	}
}

// An unmarked target was approved because every table was empty. A table that
// is not empty any more is somebody else's rows.
func TestADropRefusesWhenATableApprovedEmptyHasRows(t *testing.T) {
	w := &fakeWriter{answer: recheckAnswers{present: true, count: 3}.answer}
	l := loader{sink: event.Discard}

	r := refusalFrom(t, l.dropOne(t.Context(), w, dropOf(tref("public", "orders"))))
	if r.Code != CodeRefusedTargetChanged || r.Exit != exitTarget {
		t.Errorf("refusal = %s/exit %d, want %s/exit %d", r.Code, r.Exit, CodeRefusedTargetChanged, exitTarget)
	}
	if r.Rows != 3 {
		t.Errorf("the refusal names %d rows, want the 3 the table holds", r.Rows)
	}
	requireNothingDropped(t, w)
}

// A marked target was approved because a bound marker row authorised the
// truncation. The row being gone is that authorisation being gone.
func TestADropRefusesWhenTheMarkerRowThatAuthorisedItIsGone(t *testing.T) {
	w := &fakeWriter{answer: recheckAnswers{present: true}.answer}
	l := loader{
		run:  Run{MarkerBound: true, MarkerRunID: "3f1f0b6a-0000-4000-8000-000000000001", MarkerStatus: pg.StatusComplete},
		sink: event.Discard,
	}

	r := refusalFrom(t, l.dropOne(t.Context(), w, dropOf(tref("public", "orders"))))
	if r.Code != CodeRefusedMarkerChanged || r.Exit != exitTarget {
		t.Errorf("refusal = %s/exit %d, want %s/exit %d", r.Code, r.Exit, CodeRefusedMarkerChanged, exitTarget)
	}
	requireNothingDropped(t, w)
}

// The same refusal when the row is there and says something else: the gate read
// a status, and a row at another status is not the row it read.
func TestADropRefusesWhenTheMarkerRowChangedStatus(t *testing.T) {
	w := &fakeWriter{answer: recheckAnswers{present: true, status: pg.StatusRunning}.answer}
	l := loader{
		run:  Run{MarkerBound: true, MarkerRunID: "3f1f0b6a-0000-4000-8000-000000000001", MarkerStatus: pg.StatusComplete},
		sink: event.Discard,
	}

	r := refusalFrom(t, l.dropOne(t.Context(), w, dropOf(tref("public", "orders"))))
	if r.Code != CodeRefusedMarkerChanged || r.Exit != exitTarget {
		t.Errorf("refusal = %s/exit %d, want %s/exit %d", r.Code, r.Exit, CodeRefusedMarkerChanged, exitTarget)
	}
	requireNothingDropped(t, w)
}

// The fail-closed arm: a caller that says a marker bound the target and cannot
// say which row did has given nothing to check, and the drop refuses without
// asking the target anything. core fills all three fields together, so nothing
// in the tree reaches this today — which is why it is asserted here rather than
// left as a branch nobody has ever run.
func TestADropRefusesWhenTheRunNamesNoMarkerRow(t *testing.T) {
	w := &fakeWriter{answer: recheckAnswers{present: true}.answer}
	l := loader{run: Run{MarkerBound: true, MarkerStatus: pg.StatusComplete}, sink: event.Discard}

	r := refusalFrom(t, l.dropOne(t.Context(), w, dropOf(tref("public", "orders"))))
	if r.Code != CodeRefusedMarkerChanged || r.Exit != exitTarget {
		t.Errorf("refusal = %s/exit %d, want %s/exit %d", r.Code, r.Exit, CodeRefusedMarkerChanged, exitTarget)
	}
	for _, sql := range w.log() {
		if strings.Contains(sql, pg.MarkerTable) {
			t.Errorf("the drop read %q with no run id to read it by", sql)
		}
	}
	requireNothingDropped(t, w)
}

// And the other direction, so that a recheck stubbed out to return nil would not
// pass: a marker row still as the gate read it lets the drop through.
func TestADropProceedsWhenTheMarkerRowIsStillAsApproved(t *testing.T) {
	w := &fakeWriter{answer: recheckAnswers{present: true, status: pg.StatusComplete}.answer}
	l := loader{
		run:  Run{MarkerBound: true, MarkerRunID: "3f1f0b6a-0000-4000-8000-000000000001", MarkerStatus: pg.StatusComplete},
		sink: event.Discard,
	}

	if err := l.dropOne(t.Context(), w, dropOf(tref("public", "orders"))); err != nil {
		t.Fatalf("the drop refused a target still as the gate approved it: %v", err)
	}
	if !w.txs[0].committed {
		t.Error("the drop did not commit")
	}
	if !slices.ContainsFunc(w.txs[0].local, func(s string) bool { return strings.HasPrefix(s, "DROP TABLE") }) {
		t.Error("the drop committed without dropping anything")
	}
}

// pgErr is a server error carrying one SQLSTATE and nothing a row could hide in.
func pgErr(code string) error {
	return &pgconn.PgError{Code: code, Message: "from the fake target"}
}

// A lock that is not free is retried, three attempts a tenth of a second apart,
// and then refused as a contended target at exit 4.
func TestALockThatIsNotFreeIsRetriedAndRefused(t *testing.T) {
	w := &fakeWriter{answer: recheckAnswers{present: true}.answer}
	w.failExec = func(sql string) error {
		if strings.HasPrefix(sql, "LOCK TABLE") {
			return pgErr(sqlStateLockNotAvailable)
		}
		return nil
	}
	l := loader{sink: event.Discard}

	r := refusalFrom(t, l.dropTable(t.Context(), w, dropOf(tref("public", "orders"))))
	if r.Code != CodeRefusedTargetLocked || r.Exit != exitTarget {
		t.Errorf("refusal = %s/exit %d, want %s/exit %d", r.Code, r.Exit, CodeRefusedTargetLocked, exitTarget)
	}
	if r.SQLState != sqlStateLockNotAvailable {
		t.Errorf("the refusal carries SQLSTATE %q, want %q", r.SQLState, sqlStateLockNotAvailable)
	}
	var locks int
	for _, sql := range w.log() {
		if strings.HasPrefix(sql, "LOCK TABLE") {
			locks++
		}
	}
	if locks != lockAttempts {
		t.Errorf("the drop took the lock %d times, want %d", locks, lockAttempts)
	}
	requireNothingDropped(t, w)
}

// Every other failure of that statement is the load failing, not the target
// being used by somebody else. to_regclass answers non-NULL for any relation
// kind, so "is not a table" (42809) is reachable; so are 42501 and a relation
// that went away between the two statements (42P01). None of them is retried,
// and none of them tells an operator to go and find who is using the database.
func TestALockFailureThatIsNotContentionIsALoadFailure(t *testing.T) {
	for _, state := range []string{"42809", "42501", "42P01"} {
		t.Run(state, func(t *testing.T) {
			w := &fakeWriter{answer: recheckAnswers{present: true}.answer}
			w.failExec = func(sql string) error {
				if strings.HasPrefix(sql, "LOCK TABLE") {
					return pgErr(state)
				}
				return nil
			}
			l := loader{sink: event.Discard}

			r := refusalFrom(t, l.dropTable(t.Context(), w, dropOf(tref("public", "orders"))))
			if r.Code != CodeRefusedDDL || r.Exit != exitLoad {
				t.Errorf("refusal = %s/exit %d, want %s/exit %d", r.Code, r.Exit, CodeRefusedDDL, exitLoad)
			}
			if r.SQLState != state {
				t.Errorf("the refusal carries SQLSTATE %q, want %q", r.SQLState, state)
			}
			var locks int
			for _, sql := range w.log() {
				if strings.HasPrefix(sql, "LOCK TABLE") {
					locks++
				}
			}
			if locks != 1 {
				t.Errorf("the drop took the lock %d times; only a lock that was held is worth retrying", locks)
			}
			requireNothingDropped(t, w)
		})
	}
}

// T-0242's whole-target recheck (docs/reviews/2026-09-15-redteam/round4-still-
// leaking.json): before the first drop, under the run lease, the loader lists
// the target's *current* user tables — not only the ones its own plan
// names — and refuses over any it finds occupied that the plan does not
// already know about.

// fakeTableRows is a multi-row, three-column answer: pg.ProbeEmptiness's own
// listing query (schema, name, row-level-security).
type fakeTableRows struct {
	rows [][]any
	at   int
}

func (r *fakeTableRows) Next() bool {
	if r.at >= len(r.rows) {
		return false
	}
	r.at++
	return true
}

func (r *fakeTableRows) Scan(dest ...any) error {
	row := r.rows[r.at-1]
	if len(dest) != len(row) {
		return errors.New("the fake target was asked for a different number of columns")
	}
	for i, d := range dest {
		switch target := d.(type) {
		case *string:
			v, ok := row[i].(string)
			if !ok {
				return errors.New("the fake target was asked for a string it does not hold")
			}
			*target = v
		case *bool:
			v, ok := row[i].(bool)
			if !ok {
				return errors.New("the fake target was asked for a bool it does not hold")
			}
			*target = v
		default:
			return errors.New("the fake target holds no column of that type")
		}
	}
	return nil
}

func (r *fakeTableRows) Err() error { return nil }
func (r *fakeTableRows) Close()     {}

// wholeTargetAnswers is a target's reply to T-0242's whole-target recheck:
// every current user table pg.ProbeEmptiness's listing query would return,
// and which of them answer its SELECT EXISTS as true.
type wholeTargetAnswers struct {
	tables   []ref.TableRef
	occupied map[ref.TableRef]bool
}

func (a wholeTargetAnswers) answer(sql string, _ []any) (pipeline.Rows, error) {
	if strings.Contains(sql, "FROM pg_class c") {
		rows := make([][]any, len(a.tables))
		for i, t := range a.tables {
			rows[i] = []any{t.Schema, t.Name, false}
		}
		return &fakeTableRows{rows: rows}, nil
	}
	if strings.HasPrefix(sql, "SELECT EXISTS") {
		for _, t := range a.tables {
			if strings.Contains(sql, `"`+t.Name+`"`) {
				return &fakeRows{values: []any{a.occupied[t]}}, nil
			}
		}
	}
	return nil, errors.New("the fake target was asked something the whole-target recheck does not ask: " + sql)
}

// A table the plan never named, holding rows, is exactly what dropOne's own
// per-table recheck cannot see: it only ever visits the plan's own tables.
// The whole-target recheck lists the target fresh and refuses over it before
// any table-specific lock is taken at all.
func TestARecheckOfTheWholeTargetRefusesATableThePlanDoesNotName(t *testing.T) {
	orders := tref("public", "orders")
	items := tref("public", "order_items")
	secrets := tref("public", "prod_secrets")

	w := &fakeWriter{answer: wholeTargetAnswers{
		tables:   []ref.TableRef{orders, items, secrets},
		occupied: map[ref.TableRef]bool{secrets: true},
	}.answer}
	l := loader{sink: event.Discard}

	r := refusalFrom(t, l.recheckWholeTarget(t.Context(), w, testSchema()))
	if r.Code != CodeRefusedTargetChanged || r.Exit != exitTarget {
		t.Errorf("refusal = %s/exit %d, want %s/exit %d", r.Code, r.Exit, CodeRefusedTargetChanged, exitTarget)
	}
	if len(r.Tables) != 1 || r.Tables[0] != secrets {
		t.Errorf("the refusal names %v, want exactly [%s]", r.Tables, secrets)
	}
	requireNothingDropped(t, w)
}

// A bound marker skips the whole-target recheck entirely — it never opens a
// transaction, never lists the target and never asks a single EXISTS (T-0242
// round 5, docs/reviews/2026-09-16-redteam/round5-marker-bound-false-positive.json).
//
// On that path the gate never reached rule 5 at all: pg.Target.Gate returns
// Eligible as soon as a bound marker is found, before checkEmpty ever runs, so
// there is no "every current user table was approved empty" verdict here to
// re-verify — this pass would only ever be comparing the target's current
// tables against *today's source schema*, not against anything the gate
// looked at. A target a previous run legitimately filled still holds every
// table that run's source had; a source that has since dropped or renamed one
// of them makes it vanish from today's plan with the target not having
// changed at all. Before this fix that read as "appeared since the gate" and
// refused load.refused.target_changed on a table nobody touched — reproduced
// here with an occupied table (legacy_users) that is in neither the plan nor
// the fake's answer at all, which would have made recheckWholeTarget error
// asking about a table the fake was never told to answer for, had it been
// reached.
func TestARecheckOfTheWholeTargetSkipsEntirelyOnAMarkerBoundRun(t *testing.T) {
	w := &fakeWriter{answer: wholeTargetAnswers{
		tables:   []ref.TableRef{tref("public", "orders"), tref("public", "order_items")},
		occupied: map[ref.TableRef]bool{tref("public", "legacy_users"): true},
	}.answer}
	l := loader{run: Run{MarkerBound: true}, sink: event.Discard}

	if err := l.recheckWholeTarget(t.Context(), w, testSchema()); err != nil {
		t.Errorf("recheckWholeTarget = %v, want nil: a bound marker's gate never asked rule 5 about anything, "+
			"so this pass has no verdict to re-verify", err)
	}
	if len(w.txs) != 0 {
		t.Errorf("recheckWholeTarget opened %d transaction(s) on a marker-bound run, want 0: it must not "+
			"probe the target at all on this path", len(w.txs))
	}
}

// Nothing appeared: the sweep finds every current table accounted for by the
// plan (or empty) and the load proceeds.
func TestARecheckOfTheWholeTargetPassesWhenNothingAppeared(t *testing.T) {
	orders := tref("public", "orders")
	items := tref("public", "order_items")

	w := &fakeWriter{answer: wholeTargetAnswers{
		tables: []ref.TableRef{orders, items},
	}.answer}
	l := loader{sink: event.Discard}

	if err := l.recheckWholeTarget(t.Context(), w, testSchema()); err != nil {
		t.Errorf("recheckWholeTarget = %v, want nil", err)
	}
	if tx := w.txs[len(w.txs)-1]; !tx.rolled || tx.committed {
		t.Errorf("the whole-target recheck's own transaction wrote nothing and should roll back, not commit (rolled=%v committed=%v)",
			tx.rolled, tx.committed)
	}
}

// A table the plan already knows about is left alone by this pass even when
// the sweep finds it occupied — it is not "somebody else's" the way a table
// the plan never named is (TestARecheckOfTheWholeTargetRefusesATableThePlanDoesNotName,
// above): a plan table is full by design on a reload, and dropOne's own
// per-table recheck (recheck, below in load.go) is what asks the *right*
// question about it later, under its own lock. Asking that question here
// too, before any table-specific lock is taken, would refuse every ordinary
// reload on the very rows it exists to overwrite.
func TestARecheckOfTheWholeTargetIgnoresAnOccupiedPlanTable(t *testing.T) {
	orders := tref("public", "orders")
	items := tref("public", "order_items")

	w := &fakeWriter{answer: wholeTargetAnswers{
		tables:   []ref.TableRef{orders, items},
		occupied: map[ref.TableRef]bool{orders: true},
	}.answer}
	l := loader{sink: event.Discard}

	if err := l.recheckWholeTarget(t.Context(), w, testSchema()); err != nil {
		t.Errorf("recheckWholeTarget = %v, want nil: orders is in the plan, so being occupied is not "+
			"this pass's business", err)
	}
	requireNothingDropped(t, w)
}
