// SPDX-License-Identifier: Apache-2.0

//go:build integration

package load

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/Liarea/lazyslice/internal/dsn"
	"github.com/Liarea/lazyslice/internal/extract"
	"github.com/Liarea/lazyslice/internal/pg"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/testutil"
)

// runALeaseName and runBLeaseName are fixed run ids so the test can name run
// A's lease exactly when it terminates its backend, without reading
// internal/pg's own unexported application_name prefix — "lazyslice run " is
// this package's documented contract with a lease holder (internal/pg/lease.go,
// THREAT_MODEL.md T2), not an implementation detail of this test.
const (
	runALeaseName = "11111111-0000-4000-8000-0000000000aa"
	runBLeaseName = "22222222-0000-4000-8000-0000000000bb"
)

// TestALeaseTerminatedBetweenTheGateAndTheLoadIsRefusedNotRaced is T-0252's own
// regression, out of the round-5 red team's still-leaking entry
// (docs/reviews/2026-09-15-redteam/round5-still-leaking.json): "Run lease,
// defeat by losing the holder". The lease is a connection sitting idle in
// transaction for the whole of a run, and nothing between the gate and the
// loader's first drop re-asked whether it still held — a server-side reaper, a
// pooler restart or a dropped connection ends it with nobody noticing, and a
// second lazyslice run can then take the target apart underneath the first,
// which is exactly what the red team's replay produced (exit 0 from the second
// run, and the first surfacing a raw, uncoded SQLSTATE 23505 when it resumed).
//
// This makes that race deterministic rather than leaving it to chance: run A
// takes its lease and holds its source snapshot exactly as a stalled run
// would, its lease's own backend is terminated, run B then takes the target
// with its own lease and loads into it *in full*, and only then is run A asked
// to load. The only way run A could still touch the target at that point is a
// fall-through this test would catch; internal/pg's Lease.Alive and the three
// checkLeaseAlive call sites in load.go (the top of Load, drop, dropOne) are
// what close it — the first of the three is what keeps run A's own resumed
// Load from ever reaching EnsureMarker/StartRun, and so from writing a row of
// its own into lazyslice_meta on top of run B's, which a round-6 review found
// missing here at T-0252 landing (this test's own row-count assertions below
// never looked at that table, so they did not catch it).
func TestALeaseTerminatedBetweenTheGateAndTheLoadIsRefusedNotRaced(t *testing.T) {
	ctx := context.Background()
	sourceURL := pagilaSource(ctx, t)
	targetURL := testutil.Postgres(ctx, t, "")

	// Run A's own gate: a lease taken on a target connection of its own, and a
	// snapshot held on the source, exactly as core.openTarget and core's
	// introspect/classify/plan stages leave a run holding both while nothing
	// re-asks whether the lease still holds.
	targetA, err := pg.OpenTarget(ctx, dsn.DSN(targetURL))
	if err != nil {
		t.Fatalf("opening the target for run A: %v", err)
	}
	defer targetA.Close()
	leaseA, err := targetA.AcquireLease(ctx, runALeaseName)
	if err != nil {
		t.Fatalf("acquiring run A's lease: %v", err)
	}
	// Released at the end regardless of how the test concludes: the lease's
	// connection is held out of targetA's pool for as long as it lives
	// (AcquireLease's own doc), and Release is what gives it back — without
	// this, targetA.Close() below waits forever on that one connection's
	// pgxpool accounting, dead backend or not.
	defer leaseA.Release(context.WithoutCancel(ctx))
	sA := openSource(ctx, t, sourceURL, planRequest())

	// The reaper, the pooler restart or the dropped connection this control
	// exists for: a second, independent connection ends run A's lease's
	// backend from outside, the way pg_terminate_backend always has.
	killer := connect(ctx, t, targetURL)
	tag, err := killer.Exec(ctx,
		`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE application_name = $1`,
		"lazyslice run "+runALeaseName)
	if err != nil {
		t.Fatalf("terminating run A's lease backend: %v", err)
	}
	if tag.RowsAffected() != 1 {
		t.Fatalf("pg_terminate_backend matched %d session(s), want exactly the one holding run A's lease",
			tag.RowsAffected())
	}

	// Run B: a second, independent run takes the same target with its own
	// lease and loads into it in full — what the red team's replay showed
	// happening underneath run A with nobody refused.
	sB := openSource(ctx, t, sourceURL, planRequest())
	targetB, err := pg.OpenTarget(ctx, dsn.DSN(targetURL))
	if err != nil {
		t.Fatalf("opening the target for run B: %v", err)
	}
	defer targetB.Close()
	leaseB, err := targetB.AcquireLease(ctx, runBLeaseName)
	if err != nil {
		t.Fatalf("acquiring run B's lease: %v", err)
	}
	wB, err := targetB.Writer(ctx)
	if err != nil {
		t.Fatalf("opening run B's writer: %v", err)
	}
	if _, errB := loadWithLease(ctx, sB, wB, runBLeaseName, leaseB); errB != nil {
		t.Fatalf("run B's load, which should have proceeded normally on a fresh lease: %v", errB)
	}
	leaseB.Release(ctx)

	// Run A resumes. Its lease is dead — confirmed directly first, so a
	// failure of the refusal below points at the right half of this test.
	if leaseA.Alive(ctx) {
		t.Fatal("run A's lease answers alive after pg_terminate_backend closed its backend")
	}
	wA, err := targetA.Writer(ctx)
	if err != nil {
		t.Fatalf("opening run A's writer: %v", err)
	}
	_, errA := loadWithLease(ctx, sA, wA, runALeaseName, leaseA)

	var refusal *Refusal
	if !errors.As(errA, &refusal) || refusal.Code != CodeRefusedLeaseLost {
		t.Fatalf("run A's load = %v, want a *Refusal carrying %s", errA, CodeRefusedLeaseLost)
	}

	// Not two loaders: run A never touched the target, so it still holds
	// exactly run B's rows and nothing of run A's own is interleaved with
	// them.
	targetConn := connect(ctx, t, targetURL)
	sourceConn := connect(ctx, t, sourceURL)
	want := expected(ctx, t, sB, sourceConn)
	for table, n := range want {
		got, countErr := countRows(ctx, targetConn, table)
		if countErr != nil {
			t.Errorf("counting %s in the target: %v", table, countErr)
			continue
		}
		if got != n {
			t.Errorf("%s holds %d rows after run A's refused resume, want run B's %d left untouched",
				table, got, n)
		}
	}

	// The round-6 finding this comment used to miss: run A's own resumed Load
	// reaches EnsureMarker/StartRun before any of the checks above if the
	// pre-marker checkLeaseAlive call is ever lost again, and that writes a
	// row of run A's into lazyslice_meta on top of run B's without touching
	// a single data row the loop above counts. lazyslice_meta must therefore
	// hold exactly one row, and it must be run B's.
	var markerRuns int
	if err := targetConn.QueryRow(ctx, "SELECT count(*) FROM lazyslice_meta").Scan(&markerRuns); err != nil {
		t.Fatalf("counting rows in lazyslice_meta: %v", err)
	}
	if markerRuns != 1 {
		t.Errorf("lazyslice_meta holds %d row(s) after run A's refused resume, want exactly run B's one", markerRuns)
	}
	var markerRunID string
	if err := targetConn.QueryRow(ctx, "SELECT run_id::text FROM lazyslice_meta").Scan(&markerRunID); err != nil {
		t.Fatalf("reading lazyslice_meta's run_id: %v", err)
	}
	if markerRunID != runBLeaseName {
		t.Errorf("lazyslice_meta's run_id is %s, want run B's %s: run A wrote to the marker after its lease died",
			markerRunID, runBLeaseName)
	}
}

// loadWithLease runs loadInto's own extract-then-load shape for one source
// against one writer, under one named lease — the pieces loadInto itself
// builds inline, pulled out here because this suite needs two runs alive
// against the same target rather than loadInto's usual one.
func loadWithLease(
	ctx context.Context, s *source, w pipeline.Writer, runID string, lease leaseChecker,
) (*pipeline.LoadResult, error) {
	batches := make(chan pipeline.RowBatch, 8)
	extractErr := make(chan error, 1)
	go func() {
		extractErr <- extract.New(s.schema).Extract(ctx, s.reader, s.plan, batches)
	}()

	run := Run{
		RunID:                     runID,
		ToolVersion:               "test",
		SourceFingerprint:         "test-fp",
		ClassificationFingerprint: "none",
		SecretFingerprint:         "00000000",
		Lease:                     lease,
	}
	res, loadErr := New(run, nil).Load(ctx, w, s.plan, s.schema, batches)

	if err := <-extractErr; err != nil {
		return res, fmt.Errorf("extract: %w", err)
	}
	return res, loadErr
}
