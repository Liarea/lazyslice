// SPDX-License-Identifier: Apache-2.0

package load

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/Liarea/lazyslice/internal/event"
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

func (w *fakeWriter) Begin(context.Context) (pipeline.Tx, error) {
	tx := &fakeTx{w: w}
	w.mu.Lock()
	w.txs = append(w.txs, tx)
	w.mu.Unlock()
	return tx, nil
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
	local     []string
}

func (tx *fakeTx) Exec(_ context.Context, sql string, _ ...any) error {
	tx.local = append(tx.local, sql)
	return nil
}

func (tx *fakeTx) CopyFrom(ctx context.Context, table ref.TableRef, cols []string, rows <-chan []any) (int64, error) {
	tx.table = table
	tx.cols = cols
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
	if len(w.txs) != 2 {
		t.Fatalf("expected one transaction per table, got %d", len(w.txs))
	}
	for i, tx := range w.txs {
		if !tx.committed || tx.rolled {
			t.Errorf("transaction %d: committed=%v rolled=%v", i, tx.committed, tx.rolled)
		}
		if len(tx.local) != 1 || !strings.Contains(tx.local[0], "synchronous_commit") {
			t.Errorf("transaction %d did not set synchronous_commit off: %v", i, tx.local)
		}
	}
	if w.txs[0].table != orders || w.txs[1].table != items {
		t.Errorf("the transactions are for %v and %v", w.txs[0].table, w.txs[1].table)
	}
	if res.Rows[orders] != 3 || res.Rows[items] != 1 {
		t.Errorf("row counts are %v", res.Rows)
	}
}

// ARCHITECTURE.md §11.2: every run inserts its row with status = running before
// the first drop. Without that ordering a run killed between the first drop and
// the marker leaves a target with tables gone and nothing saying who did it, and
// the next run's gate refuses it as a non-empty stranger.
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
	}
	if insert < 0 || drop < 0 {
		t.Fatalf("expected an insert into the marker and a drop; got %v", w.log())
	}
	if insert > drop {
		t.Errorf("the marker row is written at statement %d, after the first drop at %d", insert, drop)
	}
	last := w.log()[len(w.log())-1]
	if !strings.HasPrefix(last, "UPDATE lazyslice_meta") {
		t.Errorf("the run is not closed in the marker last; the last statement is %q", last)
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
	if len(w.txs) != 1 {
		t.Fatalf("the load opened %d transactions after the first one failed", len(w.txs))
	}
	if w.txs[0].committed || !w.txs[0].rolled {
		t.Errorf("the failed table was committed=%v rolled=%v", w.txs[0].committed, w.txs[0].rolled)
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
			for i, tx := range w.txs {
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
	if len(w.txs) != 0 {
		t.Errorf("a table with no copied columns opened %d transactions", len(w.txs))
	}
	if n, ok := res.Rows[orders]; !ok || n != 0 {
		t.Errorf("the table is missing from the row counts: %v", res.Rows)
	}
}
