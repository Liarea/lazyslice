// SPDX-License-Identifier: Apache-2.0

//go:build integration

package extract

import (
	"context"
	"fmt"
	"os"
	"runtime/pprof"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Liarea/lazyslice/internal/dsn"
	"github.com/Liarea/lazyslice/internal/introspect"
	"github.com/Liarea/lazyslice/internal/load"
	"github.com/Liarea/lazyslice/internal/pg"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/plan"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/internal/testutil"
	"github.com/Liarea/lazyslice/internal/transform"
	"github.com/Liarea/lazyslice/mask"
)

// T-PERF (docs/PERF.md). This file is the reproduction of the numbers
// recorded there, not a test CI runs: `make integration` does not select it
// (it needs LAZYSLICE_PERF=1, checked first, below) because seeding
// 2,000,000 rows and profiling every stage costs minutes an ordinary
// integration run should not pay on every push. Run it directly:
//
//	LAZYSLICE_PERF=1 go test -tags integration -run TestPerfProfile \
//	  -v -timeout 20m ./internal/extract/...
//
// docs/PERF.md names the exact commands used for the numbers it reports,
// including the -cpuprofile flag for the flame summary.
//
// It seeds its own schema rather than testdata/nasty.sql: nasty.sql has no
// parameter for "5,000 root rows, 2,000,000 child rows", and extending it
// with one is outside this task's paths (testdata/ is not under
// internal/extract/, internal/load/ or internal/transform/) — filed as
// T-0157's sibling rather than done here; see docs/PERF.md's concerns and
// the tracker entry it names.

func TestPerfProfile(t *testing.T) {
	if os.Getenv("LAZYSLICE_PERF") != "1" {
		t.Skip("perf profiling run; set LAZYSLICE_PERF=1 to run it (docs/PERF.md has the exact command)")
	}
	ctx := context.Background()
	testutil.SkipWithoutDocker(ctx, t)

	url := testutil.Postgres(ctx, t, "")
	t.Logf("seeding perf_root (5,000 rows) and perf_child (2,000,000 rows)...")
	seedNarrow(ctx, t, url)
	analysePerf(ctx, t, url)

	root := tref("public", "perf_root")
	req := pipeline.PlanRequest{
		Root:         &root,
		Take:         5000,
		Cap:          500, // > 400 children/root, so every child row is taken
		Depth:        1,
		RowBudget:    5_000_000,
		MemoryBudget: 1 << 30,
	}

	f := newPerfFixture(ctx, t, url, req)
	streamRows(ctx, t, f)
}

// TestPerfWideRowRSS is the finding-9 shape on its own (2,000 one-MiB rows,
// nothing else), so its process RSS can be measured externally — see
// docs/PERF.md's command, which wraps this one test in the platform's own
// peak-RSS tool rather than reading runtime.MemStats from inside the
// process being measured. Split out from TestPerfProfile above so that
// measurement does not also carry the 2,000,000-row narrow scenario's
// memory.
func TestPerfWideRowRSS(t *testing.T) {
	if os.Getenv("LAZYSLICE_PERF") != "1" {
		t.Skip("perf profiling run; set LAZYSLICE_PERF=1 to run it (docs/PERF.md has the exact command)")
	}
	ctx := context.Background()
	testutil.SkipWithoutDocker(ctx, t)
	url := testutil.Postgres(ctx, t, "")
	testWideRowBatching(ctx, t, url, "public.perf_wide", seedWide)
}

// TestPerfWideRowRSSJSON is TestPerfWideRowRSS's jsonb counterpart: rowEstimate
// (extract.go) walks the map[string]any/[]any shapes pgx decodes jsonb/json
// and array columns into (a review finding on the fixed-constant version of
// rowEstimate, which counted a large jsonb or array value at the same 16
// bytes as a small int), so this measures the byte-cap batching fix on that
// path too and not only on the text column TestPerfWideRowRSS covers.
func TestPerfWideRowRSSJSON(t *testing.T) {
	if os.Getenv("LAZYSLICE_PERF") != "1" {
		t.Skip("perf profiling run; set LAZYSLICE_PERF=1 to run it (docs/PERF.md has the exact command)")
	}
	ctx := context.Background()
	testutil.SkipWithoutDocker(ctx, t)
	url := testutil.Postgres(ctx, t, "")
	testWideRowBatching(ctx, t, url, "public.perf_wide_json", seedWideJSON)
}

// seedNarrow creates perf_root (5,000 rows) and perf_child (2,000,000 rows,
// 400 per root, narrow columns) with two set-based INSERT ... SELECT
// statements rather than 2,000,000 round trips.
func seedNarrow(ctx context.Context, t *testing.T, url string) {
	t.Helper()
	conn, err := pgconn.Connect(ctx, url)
	if err != nil {
		t.Fatalf("connecting to seed: %v", err)
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()

	script := `
CREATE TABLE public.perf_root (
    id   bigint PRIMARY KEY,
    name text NOT NULL
);
INSERT INTO public.perf_root (id, name)
SELECT i, 'root ' || i FROM generate_series(1, 5000) AS i;

CREATE TABLE public.perf_child (
    id      bigint PRIMARY KEY,
    root_id bigint NOT NULL REFERENCES public.perf_root (id),
    email   text NOT NULL,
    body    text NOT NULL
);
CREATE INDEX perf_child_root_id_idx ON public.perf_child (root_id);
INSERT INTO public.perf_child (id, root_id, email, body)
SELECT i, ((i - 1) % 5000) + 1, 'user' || i || '@example.test', repeat('x', 80)
FROM generate_series(1, 2000000) AS i;
`
	if err := execScript(ctx, conn, script); err != nil {
		t.Fatalf("seeding narrow schema: %v", err)
	}
}

// seedWide creates perf_wide: 2,000 rows of one ~1 MiB text value each, the
// exact shape docs/reviews/2026-09-09/REVIEW.md finding 9 names ("2,000
// one-MiB values").
func seedWide(ctx context.Context, t *testing.T, url string) {
	t.Helper()
	conn, err := pgconn.Connect(ctx, url)
	if err != nil {
		t.Fatalf("connecting to seed: %v", err)
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()

	script := `
CREATE TABLE public.perf_wide (
    id      bigint PRIMARY KEY,
    payload text NOT NULL
);
INSERT INTO public.perf_wide (id, payload)
SELECT i, repeat('y', 1048576) FROM generate_series(1, 2000) AS i;
`
	if err := execScript(ctx, conn, script); err != nil {
		t.Fatalf("seeding wide schema: %v", err)
	}
}

// seedWideJSON creates perf_wide_json: 2,000 rows of one jsonb array value
// each, roughly 1 MiB of string content per row (1,024 elements of 1,024
// bytes), the same finding-9 shape as seedWide but decoded by pgx into
// []any rather than string when scanned into *any (this package's CLAUDE.md
// "Rows are scanned into *any").
func seedWideJSON(ctx context.Context, t *testing.T, url string) {
	t.Helper()
	conn, err := pgconn.Connect(ctx, url)
	if err != nil {
		t.Fatalf("connecting to seed: %v", err)
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()

	script := `
CREATE TABLE public.perf_wide_json (
    id      bigint PRIMARY KEY,
    payload jsonb NOT NULL
);
INSERT INTO public.perf_wide_json (id, payload)
SELECT i, to_jsonb(array_fill(repeat('y', 1024), ARRAY[1024]))
FROM generate_series(1, 2000) AS i;
`
	if err := execScript(ctx, conn, script); err != nil {
		t.Fatalf("seeding wide jsonb schema: %v", err)
	}
}

func execScript(ctx context.Context, conn *pgconn.PgConn, sql string) error {
	_, err := conn.Exec(ctx, sql).ReadAll()
	return err
}

func analysePerf(ctx context.Context, t *testing.T, url string) {
	t.Helper()
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatalf("connecting to analyse: %v", err)
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()
	if _, err := conn.Exec(ctx, "ANALYZE"); err != nil {
		t.Fatalf("analysing: %v", err)
	}
}

type perfFixture struct {
	src    *pg.Source
	reader pipeline.Reader
	schema *pipeline.Schema
	plan   *pipeline.Plan
}

func newPerfFixture(ctx context.Context, t *testing.T, url string, req pipeline.PlanRequest) perfFixture {
	t.Helper()
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
		t.Fatalf("opening a reader: %v", err)
	}
	t.Cleanup(func() { _ = r.Close(context.WithoutCancel(ctx)) })

	schema, err := introspect.New().Introspect(ctx, r)
	if err != nil {
		t.Fatalf("introspecting: %v", err)
	}
	p, err := plan.New().Plan(ctx, r, schema, nil, req)
	if err != nil {
		t.Fatalf("planning: %v", err)
	}

	extractShapes := make([]pg.Shape, 0, 8)
	for _, s := range Shapes(p) {
		extractShapes = append(extractShapes, pg.Shape{Name: s.Name, SQL: s.SQL})
	}
	if err := src.Register(extractShapes...); err != nil {
		t.Fatalf("registering extract's shapes: %v", err)
	}
	return perfFixture{src: src, reader: r, schema: schema, plan: p}
}

// streamRows drives extract -> transform -> load over f's plan, timing each
// stage and profiling the whole run when LAZYSLICE_PERF_CPUPROFILE names a
// file.
func streamRows(ctx context.Context, t *testing.T, f perfFixture) {
	t.Helper()

	targetURL := testutil.Postgres(ctx, t, "")
	target, err := pg.OpenTarget(ctx, dsn.DSN(targetURL))
	if err != nil {
		t.Fatalf("opening the target: %v", err)
	}
	defer target.Close()
	w, err := target.Writer(ctx)
	if err != nil {
		t.Fatalf("opening the writer: %v", err)
	}

	cls := &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{
		{Table: tref("public", "perf_child"), Column: "email"}: {
			Col:      ref.ColumnRef{Table: tref("public", "perf_child"), Column: "email"},
			Category: pipeline.CatEmail,
			Masker:   mask.MaskerEmail,
			Masked:   true,
		},
	}}
	var runKey mask.Key
	for i := range runKey {
		runKey[i] = byte(i)
	}

	if path := os.Getenv("LAZYSLICE_PERF_CPUPROFILE"); path != "" {
		fh, err := os.Create(path)
		if err != nil {
			t.Fatalf("creating cpu profile: %v", err)
		}
		defer fh.Close()
		if err := pprof.StartCPUProfile(fh); err != nil {
			t.Fatalf("starting cpu profile: %v", err)
		}
		defer pprof.StopCPUProfile()
	}

	raw := make(chan pipeline.RowBatch, 8)
	masked := make(chan pipeline.RowBatch, 8)

	var extractElapsed, transformElapsed time.Duration
	extractErr := make(chan error, 1)
	go func() {
		start := time.Now()
		err := New(f.schema).Extract(ctx, f.reader, f.plan, raw)
		extractElapsed = time.Since(start)
		extractErr <- err
	}()

	transformErr := make(chan error, 1)
	go func() {
		defer close(masked)
		res := transform.NewResidual(2_000_000)
		masker := transform.New(f.schema)
		start := time.Now()
		total := 0
		for b := range raw {
			out, err := masker.Transform(b, cls, &runKey, res)
			if err != nil {
				transformErr <- err
				return
			}
			total += len(out.Rows)
			masked <- out
		}
		transformElapsed = time.Since(start)
		transformErr <- nil
	}()

	run := load.Run{ToolVersion: "perf", SecretFingerprint: "00000000"}
	loadStart := time.Now()
	res, loadErr := load.New(run, nil).Load(ctx, w, f.plan, f.schema, masked)
	loadElapsed := time.Since(loadStart)

	if err := <-extractErr; err != nil {
		t.Fatalf("extract: %v", err)
	}
	if err := <-transformErr; err != nil {
		t.Fatalf("transform: %v", err)
	}
	if loadErr != nil {
		t.Fatalf("load: %v", loadErr)
	}

	var rows int64
	for _, n := range res.Rows {
		rows += n
	}
	total := extractElapsed + transformElapsed + loadElapsed
	t.Logf("rows loaded: %d", rows)
	t.Logf("extract:   %v", extractElapsed)
	t.Logf("transform: %v", transformElapsed)
	t.Logf("load:      %v", loadElapsed)
	t.Logf("total (stage sum, stages overlap on the channel): %v", total)
	fmt.Printf("PERF narrow: rows=%d extract=%s transform=%s load=%s\n",
		rows, extractElapsed, transformElapsed, loadElapsed)
}

// testWideRowBatching seeds table (via seed) and reports how many batches
// the extractor now cuts over 2,000 one-MiB rows, so docs/PERF.md can quote
// a number instead of an assertion. table names the root for the plan;
// seed is seedWide or seedWideJSON. Peak RSS for this run is measured
// externally (docs/PERF.md's command uses /usr/bin/time), not here: this
// process's own RSS while running inside `go test` includes the test
// harness and every other test's leftovers, which is not a number about
// this batcher.
func testWideRowBatching(ctx context.Context, t *testing.T, url string, table string, seed func(context.Context, *testing.T, string)) {
	t.Helper()
	seed(ctx, t, url)
	analysePerf(ctx, t, url)

	schema, name, _ := strings.Cut(table, ".")
	root := tref(schema, name)
	req := pipeline.PlanRequest{
		Root: &root, Take: 2000, Cap: 1, Depth: 1,
		RowBudget: 10_000, MemoryBudget: 1 << 30,
	}
	f := newPerfFixture(ctx, t, url, req)

	// Buffer 8, ARCHITECTURE.md §1's own "chan RowBatch (2,000 rows × 8,
	// Last per table)": Extract runs in its own goroutine and is consumed as
	// it sends, exactly as core wires extract to transform, so this
	// process's RSS reflects a streaming consumer rather than an unread
	// channel holding every batch of a 2,000-row, ~2 GiB table at once.
	out := make(chan pipeline.RowBatch, 8)
	extractErr := make(chan error, 1)
	go func() { extractErr <- New(f.schema).Extract(ctx, f.reader, f.plan, out) }()

	batches, rows := 0, 0
	maxRows := 0
	for b := range out {
		batches++
		rows += len(b.Rows)
		if len(b.Rows) > maxRows {
			maxRows = len(b.Rows)
		}
	}
	if err := <-extractErr; err != nil {
		t.Fatalf("Extract: %v", err)
	}
	t.Logf("%s: %d rows across %d batches, largest batch %d rows", table, rows, batches, maxRows)
	fmt.Printf("PERF wide (%s): rows=%d batches=%d largest_batch_rows=%d\n", table, rows, batches, maxRows)
}
