// SPDX-License-Identifier: Apache-2.0

//go:build integration

package core

import (
	"errors"
	"testing"

	"github.com/Liarea/lazyslice/internal/dsn"
	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pg"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/internal/testutil"
	"github.com/Liarea/lazyslice/internal/verify"
)

// This file is docs/reviews/2026-09-09 finding 4, turned into a regression
// (T-0133, THREAT_MODEL.md T8 amendment 2026-09-14).
//
// The finding: load.Load wrote status = complete the moment its own copy
// finished, before core had run verify at all, so a run whose verify then
// failed exited non-zero over a target whose marker said complete and whose
// rows were still there. The reviewer's own reproduction is the fixture below,
// exactly: a two-row text column, one email and one ordinary string, small
// enough that classify's sampled ratio does not mask it but large enough that
// verify's second net — which scans the column's whole content, not a sample —
// recognises the email and refuses at exit 9
// (docs/reviews/2026-09-09/evidence/verify_failure.log, probes.py's
// "verify_failure" case).
//
// Fixed: load no longer closes the marker on success (internal/pg/marker.go),
// core closes it once verify has had its say, and a residual-class failure
// (exit 9) additionally drops every table the run loaded before writing
// failed. This test asserts all three: the run exits on the second net's own
// code, the target no longer has the table it loaded, and the marker says
// failed and not complete.
func TestASecondNetFailureEmptiesTheTargetAndMarksFailed(t *testing.T) {
	ctx := t.Context()
	testutil.SkipWithoutDocker(ctx, t)
	quietRungs(t)

	admin := testutil.Postgres(ctx, t, "")
	source := createDatabase(ctx, t, admin, "app_verify_lifecycle_source")
	target := createDatabase(ctx, t, admin, "app_verify_lifecycle_target")
	execOn(ctx, t, source,
		`CREATE TABLE items (id integer PRIMARY KEY, v text)`,
		`INSERT INTO items VALUES (1, 'verify.lifecycle.canary@example.org'), (2, 'ok')`,
	)

	_, err := Run(ctx, raceRequest(t, source, target), event.Discard)

	var stop *Stop
	if !errors.As(err, &stop) {
		t.Fatalf("Run = %v, want the second net's refusal", err)
	}
	if stop.Code != verify.CodeRefusedSecondNet || stop.Exit != 9 {
		t.Fatalf("stop = %s/exit %d, want %s/exit 9", stop.Code, stop.Exit, verify.CodeRefusedSecondNet)
	}

	// The target no longer holds the table this run loaded: a residual-class
	// failure means the target held personal data by definition, and closeRun
	// dropped it rather than leaving that on disk until the next run's gate
	// truncates it.
	if dropped := scalarOn(ctx, t, target, `SELECT (to_regclass('public.items') IS NULL)::int`); dropped != 1 {
		t.Errorf("public.items is still in the target after an exit-9 failure; it should have been dropped")
	}

	// And the marker — which DropLoaded leaves alone, same as an ordinary
	// drop — says failed, not complete and not running.
	status := stringOn(ctx, t, target,
		`SELECT status FROM lazyslice_meta ORDER BY started_at DESC LIMIT 1`)
	if status != pg.StatusFailed {
		t.Errorf("the marker says %q after a verify failure, want %q", status, pg.StatusFailed)
	}
}

// The success half of the same contract (2026-09-14 review finding 3): nothing
// in the tree asserted that a run whose verify passes ends with the marker at
// complete and rows_loaded equal to what load actually copied. load.Load no
// longer writes StatusComplete itself (T-0133) — closeRun is now the only
// writer of it — and because pg.FinishRun's error on that path is deliberately
// discarded (run.go, nolint:errcheck), a closeRun that never reached
// StatusComplete at all, or reached it through a broken path, would still pass
// the rest of the suite: every other status assertion in this package is on a
// failure path.
func TestACleanRunEndsCompleteWithRowsLoadedMatchingTheTarget(t *testing.T) {
	ctx := t.Context()
	testutil.SkipWithoutDocker(ctx, t)
	quietRungs(t)

	admin := testutil.Postgres(ctx, t, "")
	source := createDatabase(ctx, t, admin, "app_verify_lifecycle_ok_source")
	target := createDatabase(ctx, t, admin, "app_verify_lifecycle_ok_target")
	execOn(ctx, t, source,
		`CREATE TABLE items (id integer PRIMARY KEY, v text)`,
		`INSERT INTO items VALUES (1, 'ordinary one'), (2, 'ordinary two')`,
	)

	if _, err := Run(ctx, raceRequest(t, source, target), event.Discard); err != nil {
		t.Fatalf("Run = %v, want a clean pass", err)
	}

	status := stringOn(ctx, t, target,
		`SELECT status FROM lazyslice_meta ORDER BY started_at DESC LIMIT 1`)
	if status != pg.StatusComplete {
		t.Fatalf("the marker says %q after a clean run, want %q", status, pg.StatusComplete)
	}

	rowsLoaded := scalarOn(ctx, t, target,
		`SELECT rows_loaded FROM lazyslice_meta ORDER BY started_at DESC LIMIT 1`)
	actual := scalarOn(ctx, t, target, `SELECT count(*)::int FROM public.items`)
	if actual != 2 {
		t.Fatalf("the target holds %d rows in items, want 2", actual)
	}
	if rowsLoaded != actual {
		t.Errorf("rows_loaded = %d, want %d (load.TotalRows over what the target actually holds)", rowsLoaded, actual)
	}
}

// The other half of the discrimination the task names: exit 8 (a foreign key)
// and exit 7 (a row count or a sequence) are not residual-class, so closeRun
// must leave the loaded rows exactly where they are — they are what an
// operator diagnoses the failure against — and only write the marker failed.
// A closeRun that dropped the target on *every* verify failure, ignoring
// refusal.Exit entirely, would still pass
// TestASecondNetFailureEmptiesTheTargetAndMarksFailed and every other test in
// the tree; only a sibling on the non-residual side pins the discrimination.
//
// This calls (*run).closeRun directly rather than driving a real exit-7
// through core.Run end to end. Every event.Sink in this package's runs — the
// line printer, the TUI, a test's own — receives events from eventChannel's
// own drain goroutine (channel.go), one call removed from the goroutine
// driving the pipeline; a sink that tried to reach into the target between
// Load's stage_done and Verify's own first read would be racing that drain
// goroutine against Verify's row-count query with nothing ordering the two,
// which is a race the first version of this test had and lost exactly once in
// ten thousand-odd runs, under load, in `make integration`. `closeRun` is
// itself the unit the finding is about — "pins the Exit==exitResidual
// discrimination in place" — so this drives it with the same real Postgres
// target, the same marker helpers `internal/pg` exports to `internal/load`
// and this package alike, and a `*verify.Refusal` built by hand at exit 7,
// which is deterministic and exercises exactly the branch in question.
func TestARowCountFailureLeavesTheTargetLoadedAndMarksFailed(t *testing.T) {
	ctx := t.Context()
	testutil.SkipWithoutDocker(ctx, t)

	admin := testutil.Postgres(ctx, t, "")
	target := createDatabase(ctx, t, admin, "app_verify_lifecycle_rc_target")
	execOn(ctx, t, target,
		`CREATE TABLE items (id integer PRIMARY KEY, v text)`,
		// Three rows already in the target, standing in for a load that
		// copied them: what matters to closeRun is only what it finds there
		// and what verify told it, not how the rows arrived.
		`INSERT INTO items VALUES (1, 'ordinary one'), (2, 'ordinary two'), (3, 'not in the plan')`,
	)

	pgTarget, err := pg.OpenTarget(ctx, dsn.DSN(target))
	if err != nil {
		t.Fatalf("opening the target: %v", err)
	}
	defer pgTarget.Close()
	w, err := pgTarget.Writer(ctx)
	if err != nil {
		t.Fatalf("Writer: %v", err)
	}

	if markerErr := pg.EnsureMarker(ctx, w); markerErr != nil {
		t.Fatalf("EnsureMarker: %v", markerErr)
	}
	runID, err := pg.StartRun(ctx, w, pg.MarkerRow{
		ToolVersion:               "test",
		SourceFingerprint:         "test",
		SchemaFingerprint:         "test",
		ClassificationFingerprint: "test",
		RootTable:                 "public.items",
		Take:                      2,
		SecretFingerprint:         "test",
	})
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}

	r := &run{runID: runID, sink: event.Discard}
	refusal := &verify.Refusal{Code: verify.CodeRefusedRowCount, Exit: 7, Table: ref.TableRef{Schema: "public", Name: "items"}}
	lr := &pipeline.LoadResult{Rows: map[ref.TableRef]int64{{Schema: "public", Name: "items"}: 2}}
	r.closeRun(ctx, w, lr, refusal)

	// The loaded table is still there, and still holds every row: exit 7 is
	// not residual-class, so closeRun must not have called load.DropLoaded.
	present := scalarOn(ctx, t, target, `SELECT (to_regclass('public.items') IS NOT NULL)::int`)
	if present != 1 {
		t.Fatalf("public.items was dropped after an exit-7 failure; it should have been left for diagnosis")
	}
	count := scalarOn(ctx, t, target, `SELECT count(*)::int FROM public.items`)
	if count != 3 {
		t.Errorf("public.items holds %d rows after an exit-7 failure, want 3 (nothing touched)", count)
	}

	status := stringOn(ctx, t, target,
		`SELECT status FROM lazyslice_meta ORDER BY started_at DESC LIMIT 1`)
	if status != pg.StatusFailed {
		t.Errorf("the marker says %q after a verify failure, want %q", status, pg.StatusFailed)
	}
}
