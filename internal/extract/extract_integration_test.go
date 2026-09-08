// SPDX-License-Identifier: Apache-2.0

//go:build integration

package extract

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/Liarea/lazyslice/internal/dsn"
	"github.com/Liarea/lazyslice/internal/introspect"
	"github.com/Liarea/lazyslice/internal/pg"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/plan"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/internal/testutil"
	"github.com/Liarea/lazyslice/internal/transform"
	"github.com/Liarea/lazyslice/mask"
)

// What extract reads is a statement about a real Postgres — the typed unnest
// join, the cast back for a key that travels as text, a partitioned root, a
// composite key — so these run against testdata/nasty.sql through the real
// introspector, the real planner and a real internal/pg.Source with the real
// allowlist. A statement no registered shape covers does not merely get logged:
// the tracer hands the call a cancelled context and the read fails
// (THREAT_MODEL.md T9).
//
// Every run over nasty.sql carries --skip-table public.click_stream, because
// trap 12's table has no row identity at all and §3.4 ends in exit 12 for it
// (testdata/README.md).

type fixture struct {
	src    *pg.Source
	reader pipeline.Reader
	schema *pipeline.Schema
	plan   *pipeline.Plan
}

// newFixture loads nasty.sql, opens one snapshot over it, introspects, plans,
// and registers extract's shapes for that plan on the same allowlist.
func newFixture(ctx context.Context, t *testing.T, big bool, req pipeline.PlanRequest) fixture {
	t.Helper()
	testutil.SkipWithoutDocker(ctx, t)

	url := testutil.Postgres(ctx, t, "")
	if err := testutil.LoadNasty(ctx, url, big); err != nil {
		t.Fatalf("loading nasty.sql: %v", err)
	}
	analyse(ctx, t, url)

	shapes := make([]pg.Shape, 0, 32)
	for _, s := range introspect.Shapes() {
		shapes = append(shapes, pg.Shape{Name: s.Name, SQL: s.SQL})
	}
	for _, s := range plan.Shapes() {
		shapes = append(shapes, pg.Shape{Name: s.Name, SQL: s.SQL})
	}
	src, err := pg.OpenSource(ctx, dsn.DSN(url), shapes...)
	if err != nil {
		t.Fatalf("opening the source: %v", err)
	}
	t.Cleanup(src.Close)

	id, err := src.Snapshot(ctx)
	if err != nil {
		t.Fatalf("exporting the snapshot: %v", err)
	}
	t.Cleanup(func() { _ = src.Release(context.WithoutCancel(ctx)) })

	r, err := src.Reader(ctx, id)
	if err != nil {
		t.Fatalf("opening a reader on the snapshot: %v", err)
	}
	t.Cleanup(func() { _ = r.Close(context.WithoutCancel(ctx)) })
	// Cleanups run last-registered first, so this one runs while the reader and
	// the snapshot are still open, which is where a refusal would have happened.
	t.Cleanup(func() {
		if refused := src.Violation(); refused != nil {
			t.Errorf("the source allowlist refused a statement: %v", refused)
		}
	})

	schema, err := introspect.New().Introspect(ctx, r)
	if err != nil {
		t.Fatalf("introspecting: %v", err)
	}
	p, err := plan.New().Plan(ctx, r, schema, nil, req)
	if err != nil {
		t.Fatalf("planning: %v", err)
	}

	// Extract's shapes are the plan's: the lookup reads are named per table.
	// They are registered here, after planning and before the first read, which
	// is the order internal/core will use.
	extractShapes := make([]pg.Shape, 0, len(Shapes(p)))
	for _, s := range Shapes(p) {
		extractShapes = append(extractShapes, pg.Shape{Name: s.Name, SQL: s.SQL})
	}
	if err := src.Register(extractShapes...); err != nil {
		t.Fatalf("registering extract's shapes: %v", err)
	}
	return fixture{src: src, reader: r, schema: schema, plan: p}
}

// analyse fills pg_class.reltuples, which is -1 until something analyses: the
// root default and the lookup probe both read it. It runs on a connection of
// its own, because ANALYZE is not a statement the source's allowlist carries.
func analyse(ctx context.Context, t *testing.T, url string) {
	t.Helper()
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatalf("connecting to analyse the fixture: %v", err)
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()
	if _, err := conn.Exec(ctx, "ANALYZE"); err != nil {
		t.Fatalf("analysing the fixture: %v", err)
	}
}

func tref(schema, name string) ref.TableRef { return ref.TableRef{Schema: schema, Name: name} }

// nastyRequest is the plan request every case here starts from.
func nastyRequest() pipeline.PlanRequest {
	root := tref("public", "people")
	return pipeline.PlanRequest{
		Root: &root,
		Skip: []ref.TableRef{tref("public", "click_stream")},
	}
}

// collect runs one extraction and returns the batches in the order they were
// sent.
func collect(ctx context.Context, t *testing.T, f fixture) []pipeline.RowBatch {
	t.Helper()
	// 2,000 rows x 8, which is the depth ARCHITECTURE.md §1 gives the channel.
	out := make(chan pipeline.RowBatch, 8)
	var batches []pipeline.RowBatch
	done := make(chan struct{})
	go func() {
		defer close(done)
		for b := range out {
			batches = append(batches, b)
		}
	}()
	if err := New(f.schema).Extract(ctx, f.reader, f.plan, out); err != nil {
		t.Fatalf("Extract: %v", err)
	}
	<-done
	return batches
}

func TestEveryPlannedStepIsExtractedExactlyOnceAndInOrder(t *testing.T) {
	ctx := context.Background()
	f := newFixture(ctx, t, false, nastyRequest())
	batches := collect(ctx, t, f)

	var want []ref.TableRef
	keyed := map[ref.TableRef]int{}
	for _, s := range f.plan.Steps {
		if s.Mode == pipeline.SchemaOnly {
			continue
		}
		want = append(want, s.Table)
		if s.Keys != nil {
			keyed[s.Table] = s.Keys.Len()
		}
	}

	var order []ref.TableRef
	rows := map[ref.TableRef]int{}
	lasts := map[ref.TableRef]int{}
	seq := map[ref.TableRef]int{}
	for _, b := range batches {
		if len(order) == 0 || order[len(order)-1] != b.Table {
			order = append(order, b.Table)
		}
		if b.Seq != seq[b.Table] {
			t.Errorf("%s batch Seq %d, want %d", b.Table, b.Seq, seq[b.Table])
		}
		seq[b.Table]++
		rows[b.Table] += len(b.Rows)
		if b.Last {
			lasts[b.Table]++
		}
	}

	if fmt.Sprint(order) != fmt.Sprint(want) {
		t.Fatalf("tables on the channel:\n got %v\nwant %v", order, want)
	}
	for _, table := range want {
		if lasts[table] != 1 {
			t.Errorf("%s carried Last on %d batches, want exactly 1", table, lasts[table])
		}
	}
	for table, n := range keyed {
		if rows[table] != n {
			t.Errorf("%s: extracted %d rows for a key set of %d", table, rows[table], n)
		}
	}
	for _, s := range f.plan.Steps {
		if s.Mode != pipeline.Lookup {
			continue
		}
		if rows[s.Table] > lookupLimit {
			t.Errorf("lookup %s returned %d rows, past the %d bound the read carries",
				s.Table, rows[s.Table], lookupLimit)
		}
	}
	if len(keyed) == 0 {
		t.Fatal("the plan has no keyed step, so this test proved nothing")
	}
}

// A composite key, a uuid key and a key whose type travels as text and is cast
// back all appear in nasty.sql; this is where the join predicate for each is
// exercised against the server rather than against a regexp.
func TestTheChunkJoinReadsCompositeAndCastBackKeys(t *testing.T) {
	ctx := context.Background()
	f := newFixture(ctx, t, false, nastyRequest())
	batches := collect(ctx, t, f)

	rows := map[ref.TableRef]int{}
	for _, b := range batches {
		rows[b.Table] += len(b.Rows)
	}
	// device_readings is keyed on (uuid, timestamptz): the uuid travels as a
	// uuid array and the timestamptz as text with a cast back to its own type,
	// which is the one case where getting the cast wrong joins zero rows.
	readings := tref("public", "device_readings")
	step, ok := stepFor(f.plan, readings)
	if !ok {
		t.Fatal("public.device_readings is not in the plan")
	}
	if step.Keys == nil || step.Keys.Len() == 0 {
		t.Skip("the plan selected no device_readings rows, so the cast-back join is not exercised")
	}
	if rows[readings] != step.Keys.Len() {
		t.Errorf("device_readings: %d rows for %d keys; a wrong cast back joins nothing",
			rows[readings], step.Keys.Len())
	}
}

func stepFor(p *pipeline.Plan, t ref.TableRef) (pipeline.Step, bool) {
	for _, s := range p.Steps {
		if s.Table == t {
			return s, true
		}
	}
	return pipeline.Step{}, false
}

// Two extractions over one snapshot produce the same rows in the same order.
// It is the half of invariant I3 this package owns: the loader can only write
// byte-identical targets if it is handed byte-identical batches.
func TestTwoExtractionsOverOneSnapshotAgree(t *testing.T) {
	ctx := context.Background()
	f := newFixture(ctx, t, false, nastyRequest())
	first := render(collect(ctx, t, f))
	second := render(collect(ctx, t, f))
	if first != second {
		t.Error("two extractions over one snapshot produced different batches")
	}
	if first == "" {
		t.Fatal("the extraction produced nothing, so this test proved nothing")
	}
}

// render is a stable text form of a run's batches, for comparing two runs. It
// is a test-only rendering of production values and never leaves the process.
func render(batches []pipeline.RowBatch) string {
	var b strings.Builder
	for _, batch := range batches {
		fmt.Fprintf(&b, "%s seq=%d last=%v cols=%v\n", batch.Table, batch.Seq, batch.Last, batch.Cols)
		for _, row := range batch.Rows {
			fmt.Fprintf(&b, "  %v\n", row)
		}
	}
	return b.String()
}

// The streaming claim: resident memory does not grow with the row count.
//
// stream_rows holds 2,000,000 rows hanging off one person (testdata/README.md
// trap 22). A run that buffered a table would need hundreds of megabytes for
// this one; extract holds one chunk of keys and one batch of rows, so the heap
// during the extraction stays inside the ceiling below whatever the table's
// size is. The key set the plan built is already allocated when the baseline is
// taken, so what is measured is extract's own working set.
func TestMemoryStaysBoundedOnTwoMillionRows(t *testing.T) {
	ctx := context.Background()
	req := nastyRequest()
	// The whole table, so the claim is about 2,000,000 rows and not about a
	// capped hundred of them. The budgets are raised for the same reason: the
	// point of this test is the extraction, not the planner's own limits.
	req.Cap = 3_000_000
	req.Depth = 1
	req.RowBudget = 5_000_000
	req.MemoryBudget = 1 << 30
	// stream_docs is the other half of the same gate and the subject of
	// TestMemoryStaysBoundedOnATextKeyedTable; skipped here so this test
	// measures the key encoding it is about and nothing else. The skip is a
	// plan-time one and not a saving: a big load runs the whole gate, so both
	// tables are filled for either test and this one carries stream_docs'
	// million rows on disk whether it reads them or not.
	req.Skip = append(req.Skip, tref("public", "stream_docs"))
	f := newFixture(ctx, t, true, req)

	streamRows := tref("public", "stream_rows")
	step, ok := stepFor(f.plan, streamRows)
	if !ok || step.Keys == nil {
		t.Fatal("public.stream_rows is not a keyed step of the plan")
	}
	if step.Keys.Len() < 2_000_000 {
		t.Fatalf("the plan selected %d stream_rows, want the whole 2,000,000-row table", step.Keys.Len())
	}

	// The masker runs on every batch too, because the claim is about the pair:
	// transform is what stands between extract and the loader, and a transform
	// that accumulated would blow the same ceiling. Its residual filter is sized
	// once, from the plan, before the baseline is taken, exactly as core will
	// size it — the plan prints that figure and counts it against the memory
	// budget (§6 item 1).
	cls := &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{
		{Table: streamRows, Column: "email"}: {
			Col:      ref.ColumnRef{Table: streamRows, Column: "email"},
			Category: pipeline.CatEmail,
			Masker:   mask.MaskerEmail,
			Masked:   true,
		},
	}}
	var runKey mask.Key
	for i := range runKey {
		runKey[i] = byte(i)
	}
	res := transform.NewResidual(int64(step.Keys.Len()))
	masker := transform.New(f.schema)

	runtime.GC()
	var base runtime.MemStats
	runtime.ReadMemStats(&base)

	out := make(chan pipeline.RowBatch, 8)
	peak := base.HeapAlloc
	total := 0
	done := make(chan struct{})
	go func() {
		defer close(done)
		n := 0
		for b := range out {
			if _, err := masker.Transform(b, cls, &runKey, res); err != nil {
				t.Errorf("Transform: %v", err)
				continue
			}
			total += len(b.Rows)
			n++
			if n%100 == 0 {
				runtime.GC()
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				peak = max(peak, m.HeapAlloc)
			}
		}
	}()
	if err := New(f.schema).Extract(ctx, f.reader, f.plan, out); err != nil {
		t.Fatalf("Extract: %v", err)
	}
	<-done

	if total < 2_000_000 {
		t.Fatalf("extracted %d rows in total, want at least the 2,000,000 of stream_rows", total)
	}

	if got := res.Cells(); got < 2_000_000 {
		t.Errorf("the residual filter holds %d cells for %d masked rows", got, total)
	}

	// The ceiling. One batch is 2,000 rows of four columns and the channel is
	// eight deep, so a streaming extract and an in-place transform live in tens
	// of megabytes above the baseline; a run that buffered the table would need
	// hundreds. 64 MiB is above the first and well below the second, which is
	// what makes the test able to fail.
	const ceiling = 64 << 20
	growth := int64(peak) - int64(base.HeapAlloc)
	t.Logf("heap: baseline %d MiB, peak %d MiB, growth %d MiB over %d rows",
		base.HeapAlloc>>20, peak>>20, growth>>20, total)
	if growth > ceiling {
		t.Errorf("the heap grew %d MiB while extracting %d rows, past the %d MiB ceiling: extract is buffering",
			growth>>20, total, ceiling>>20)
	}
}

// The same claim where the keys cost what keys usually cost.
//
// stream_rows above is keyed on one bigint, which internal/plan stores as a
// []int64 and a chunk carries as a []int64: eight bytes a key on both sides,
// 16 MiB for the whole set, so a second copy of it fits inside the ceiling and
// that test passes whether or not extract makes one. stream_docs is the same
// shape of table keyed on a 36-character text primary key (testdata/README.md
// trap 26), which is stored as a slab of encoded tuples with an eight-byte span
// index and handed to the server as a []string of separately allocated
// strings — about 64 bytes a key in a chunk. At StreamDocs keys that is ~64 MiB
// for one copy of the set, so the difference between "one chunk at a time" and
// "every chunk up front" is the difference between passing and failing the same
// ceiling. This is the test that can fail on the thing the other one asserts.
func TestMemoryStaysBoundedOnATextKeyedTable(t *testing.T) {
	ctx := context.Background()
	req := nastyRequest()
	req.Cap = 3_000_000
	req.Depth = 1
	req.RowBudget = 5_000_000
	req.MemoryBudget = 1 << 30
	// The int8-keyed table is the subject of the test above. Skipped in the plan,
	// not in the fixture: LoadNasty(big) fills both tables whichever of the two
	// tests asks for it (see the note in the other one).
	req.Skip = append(req.Skip, tref("public", "stream_rows"))
	f := newFixture(ctx, t, true, req)

	streamDocs := tref("public", "stream_docs")
	step, ok := stepFor(f.plan, streamDocs)
	if !ok || step.Keys == nil {
		t.Fatal("public.stream_docs is not a keyed step of the plan")
	}
	if step.Keys.Len() < testutil.StreamDocs {
		t.Fatalf("the plan selected %d stream_docs, want the whole %d-row table",
			step.Keys.Len(), testutil.StreamDocs)
	}
	if got := step.Identity.Columns; len(got) != 1 || got[0] != "doc_key" {
		t.Fatalf("stream_docs identity is %v, want the text key [doc_key]: this test is about "+
			"what a text key set costs, and an integer identity would not measure it", got)
	}

	cls := &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{
		{Table: streamDocs, Column: "email"}: {
			Col:      ref.ColumnRef{Table: streamDocs, Column: "email"},
			Category: pipeline.CatEmail,
			Masker:   mask.MaskerEmail,
			Masked:   true,
		},
	}}
	var runKey mask.Key
	for i := range runKey {
		runKey[i] = byte(i)
	}
	res := transform.NewResidual(int64(step.Keys.Len()))
	masker := transform.New(f.schema)

	runtime.GC()
	var base runtime.MemStats
	runtime.ReadMemStats(&base)

	out := make(chan pipeline.RowBatch, 8)
	peak := base.HeapAlloc
	total := 0
	done := make(chan struct{})
	go func() {
		defer close(done)
		n := 0
		for b := range out {
			if _, err := masker.Transform(b, cls, &runKey, res); err != nil {
				t.Errorf("Transform: %v", err)
				continue
			}
			total += len(b.Rows)
			n++
			if n%100 == 0 {
				runtime.GC()
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				peak = max(peak, m.HeapAlloc)
			}
		}
	}()
	if err := New(f.schema).Extract(ctx, f.reader, f.plan, out); err != nil {
		t.Fatalf("Extract: %v", err)
	}
	<-done

	if total < testutil.StreamDocs {
		t.Fatalf("extracted %d rows in total, want at least the %d of stream_docs",
			total, testutil.StreamDocs)
	}

	// The ceiling is half the int8 test's, because the margin here is measured
	// rather than assumed: with EachChunk the growth is 0 to 5 MiB, and with the
	// Chunks call it replaced — reintroduced on purpose to check this test can
	// fail — it is 63 MiB. 32 MiB sits between them with room on both sides,
	// where 64 MiB would have let the buffering version pass by one megabyte.
	const ceiling = 32 << 20
	growth := int64(peak) - int64(base.HeapAlloc)
	t.Logf("heap: baseline %d MiB, peak %d MiB, growth %d MiB over %d text-keyed rows",
		base.HeapAlloc>>20, peak>>20, growth>>20, total)
	if growth > ceiling {
		t.Errorf("the heap grew %d MiB while extracting %d text-keyed rows, past the %d MiB "+
			"ceiling: extract is holding more than one chunk of keys",
			growth>>20, total, ceiling>>20)
	}
}
