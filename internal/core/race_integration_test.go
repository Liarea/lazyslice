// SPDX-License-Identifier: Apache-2.0

//go:build integration

package core

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/load"
	"github.com/Liarea/lazyslice/internal/pg"
	"github.com/Liarea/lazyslice/internal/testutil"
)

// This file is docs/reviews/2026-09-09 finding 1, turned into a regression.
//
// The finding: Target.Gate checks emptiness and releases its connection; core
// then introspects, classifies and plans before the loader acquires a writer and
// drops tables. Nothing owned the target through that interval. The reviewer
// started with an empty unmarked target, held up source introspection, waited
// for the target-selection event, inserted row 999 into the target and released
// the source — and lazyslice deleted row 999, loaded its own row, and exited 0.
// "A successful run deleting data that did not exist when deletion was
// approved" (docs/reviews/2026-09-09/evidence/target_race.py, race.log).
//
// The two controls are ARCHITECTURE.md §11.2's, amended 2026-09-14: a run lease
// held on a target connection of its own from before the gate until the marker
// is finished, and a per-table lock-and-recheck inside the transaction that
// drops each table. This file asserts one each, and two more besides: the
// marked-target branch of the recheck (the production reload path, where the
// authorisation is a marker row rather than an empty table), and the scope of
// the rollback when the refusal lands on a table the loop has already dropped
// others before.
//
// Every test here is deterministic and none sleeps for a window: the source is
// held under ACCESS EXCLUSIVE by a session of the test's own, so the run *cannot*
// get past introspect until the test lets it, and everything the test does in
// between happens strictly after the gate and strictly before the first drop.
// That is the same instrument the reviewer used, and it is why the original
// evidence is reproducible rather than a race that usually loses.

// sourceGuard is a session holding ACCESS EXCLUSIVE on the source's one table.
//
// A lazyslice run opens its snapshot and then reads rows from the source: with
// the table locked, it blocks there, which is a point strictly after the target
// gate and strictly before anything in the target is dropped.
type sourceGuard struct {
	conn *pgx.Conn
	once sync.Once
}

func holdSource(ctx context.Context, t *testing.T, url, table string) *sourceGuard {
	t.Helper()

	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatalf("connecting to hold the source: %v", err)
	}
	if _, err := conn.Exec(ctx, `BEGIN`); err != nil {
		t.Fatalf("opening the guard transaction: %v", err)
	}
	if _, err := conn.Exec(ctx, `LOCK TABLE `+pgx.Identifier{table}.Sanitize()+` IN ACCESS EXCLUSIVE MODE`); err != nil {
		t.Fatalf("locking %s on the source: %v", table, err)
	}
	g := &sourceGuard{conn: conn}
	t.Cleanup(func() { g.release(context.WithoutCancel(ctx)) })
	return g
}

func (g *sourceGuard) release(ctx context.Context) {
	g.once.Do(func() {
		_, _ = g.conn.Exec(ctx, `ROLLBACK`)
		_ = g.conn.Close(ctx)
	})
}

// gateWatcher is a sink that signals when the target has been chosen, which is
// the last event the gate emits and therefore the moment the reviewer's script
// waited for ("target t_race" in race.log).
type gateWatcher struct {
	mu     sync.Mutex
	codes  []event.Code
	gated  chan struct{}
	closed bool
}

func newGateWatcher() *gateWatcher {
	return &gateWatcher{gated: make(chan struct{})}
}

func (w *gateWatcher) Send(e event.Event) {
	w.mu.Lock()
	w.codes = append(w.codes, e.Code)
	if e.Code == CodeTargetChosen && !w.closed {
		w.closed = true
		close(w.gated)
	}
	w.mu.Unlock()
}

// waitForGate blocks until the run has passed the gate, or fails the test.
func (w *gateWatcher) waitForGate(t *testing.T) {
	t.Helper()
	select {
	case <-w.gated:
	case <-time.After(2 * time.Minute):
		t.Fatal("the run never announced its target, so the gate was never reached")
	}
}

// raceRequest is a run of the whole pipeline against two named endpoints, with
// nothing on the discovery ladder and nothing written to the working directory
// but the masking key.
func raceRequest(t *testing.T, source, target string) Request {
	t.Helper()

	dir := t.TempDir()
	return Request{
		Mode:       ModeRun,
		Workdir:    dir,
		Source:     source,
		Target:     target,
		Root:       "public.items",
		ConfigPath: filepath.Join(dir, "lazyslice.yml"),
		SecretFile: filepath.Join(dir, "lazyslice.secret"),
		NoConfig:   true,
		Yes:        true,
		// A non-local Docker endpoint yields no rung 3 candidate and makes no
		// socket call (ADR-008 §3), so this suite's own containers stay off the
		// ladder.
		DockerHost: "tcp://staging.example:2375",
	}
}

// raceDatabases is the reviewer's fixture: a source holding one row and an
// empty, unmarked target with the same table in it.
func raceDatabases(ctx context.Context, t *testing.T) (source, target string) {
	t.Helper()

	admin := testutil.Postgres(ctx, t, "")
	source = createDatabase(ctx, t, admin, "app_race_source")
	target = createDatabase(ctx, t, admin, "app_race_target")
	execOn(ctx, t, source,
		`CREATE TABLE items (id integer PRIMARY KEY)`,
		`INSERT INTO items VALUES (1)`,
	)
	execOn(ctx, t, target, `CREATE TABLE items (id integer PRIMARY KEY)`)
	return source, target
}

// A row inserted into the target after the gate approved it is not deleted.
//
// This is the reviewed failure exactly: the gate approves an empty table, the
// run is held up on the source, somebody writes to the target, and the run
// resumes. Before T-0130 the DROP ran on a verdict several stages old and the
// row was gone with exit 0. Now the drop's own transaction takes the table's
// ACCESS EXCLUSIVE lock and re-verifies the emptiness that authorised it, so
// the run refuses at exit 4 naming the table and the row count, rolls back, and
// closes its marker row failed.
func TestARowInsertedAfterTheGateIsNotDropped(t *testing.T) {
	ctx := t.Context()
	testutil.SkipWithoutDocker(ctx, t)
	quietRungs(t)

	source, target := raceDatabases(ctx, t)

	// Held before the run starts, so there is no window in which the run could
	// reach the loader before the test has written its row.
	guard := holdSource(ctx, t, source, "items")

	watcher := newGateWatcher()
	done := make(chan error, 1)
	go func() {
		_, err := Run(ctx, raceRequest(t, source, target), watcher)
		done <- err
	}()

	watcher.waitForGate(t)

	// Strictly after the gate, strictly before the first drop.
	execOn(ctx, t, target, `INSERT INTO items VALUES (999)`)
	guard.release(ctx)

	var err error
	select {
	case err = <-done:
	case <-time.After(5 * time.Minute):
		t.Fatal("the run never finished")
	}

	var stop *Stop
	if !errors.As(err, &stop) {
		t.Fatalf("Run = %v, want the lock-and-recheck refusal", err)
	}
	if stop.Code != load.CodeRefusedTargetChanged || stop.Exit != exitTarget {
		t.Fatalf("stop = %s/exit %d, want %s/exit %d",
			stop.Code, stop.Exit, load.CodeRefusedTargetChanged, exitTarget)
	}
	if got := stop.Args[event.ArgTable]; got != "public.items" {
		t.Errorf("the refusal names table %q, want public.items", got)
	}
	if got := stop.Args[event.ArgCount]; got != "1" {
		t.Errorf("the refusal names %q rows, want 1 — the row somebody inserted", got)
	}

	// The whole point: the row is still there, and so is the table it is in.
	if n := scalarOn(ctx, t, target, `SELECT count(*)::int FROM items WHERE id = 999`); n != 1 {
		t.Errorf("row 999 is gone: the run deleted data that did not exist when the gate approved the target")
	}
	if n := scalarOn(ctx, t, target, `SELECT count(*)::int FROM items`); n != 1 {
		t.Errorf("the target's items holds %d rows, want the one row the test inserted and nothing of ours", n)
	}

	// And the run says so about itself: the marker it wrote before the drop is
	// closed failed, not left at running and not complete (THREAT_MODEL.md T8).
	status := stringOn(ctx, t, target,
		`SELECT status FROM lazyslice_meta ORDER BY started_at DESC LIMIT 1`)
	if status != pg.StatusFailed {
		t.Errorf("the marker says %q after the refusal, want %q", status, pg.StatusFailed)
	}
}

// A table created in the target after the gate approved it — never a member
// of this run's plan — is not silently dropped, or loaded around.
//
// This is T-0242's round-4 replay
// (docs/reviews/2026-09-15-redteam/round4-still-leaking.json): the gate
// approves an empty, unmarked target, the run is held up on the source, a
// third party CREATEs a table the plan never named and puts production rows
// in it, and the run resumes. dropOne's own per-table recheck only ever
// visits the tables in this run's plan, so before T-0242 that table was
// simply invisible to it: the run printed "dropping public.items", dropped
// it, loaded its own row, and exited 0 with prod_secrets — and its rows —
// still sitting in the target it had just called a successful load. Now the
// whole-target recheck runs first, before any table is even locked, lists the
// target's current tables fresh, finds prod_secrets occupied and unaccounted
// for, and refuses before touching anything.
func TestATableCreatedAfterTheGateIsRefused(t *testing.T) {
	ctx := t.Context()
	testutil.SkipWithoutDocker(ctx, t)
	quietRungs(t)

	source, target := raceDatabases(ctx, t)

	guard := holdSource(ctx, t, source, "items")

	watcher := newGateWatcher()
	done := make(chan error, 1)
	go func() {
		_, err := Run(ctx, raceRequest(t, source, target), watcher)
		done <- err
	}()

	watcher.waitForGate(t)

	// Strictly after the gate, strictly before the first drop, and not a
	// table this run's plan will ever name.
	execOn(ctx, t, target,
		`CREATE TABLE prod_secrets (id integer PRIMARY KEY, email text)`,
		`INSERT INTO prod_secrets VALUES (1, 'ceo@bigcorp.example'), (2, 'cfo@bigcorp.example')`,
	)
	guard.release(ctx)

	var err error
	select {
	case err = <-done:
	case <-time.After(5 * time.Minute):
		t.Fatal("the run never finished")
	}

	var stop *Stop
	if !errors.As(err, &stop) {
		t.Fatalf("Run = %v, want the whole-target recheck refusal", err)
	}
	if stop.Code != load.CodeRefusedTargetChanged || stop.Exit != exitTarget {
		t.Fatalf("stop = %s/exit %d, want %s/exit %d",
			stop.Code, stop.Exit, load.CodeRefusedTargetChanged, exitTarget)
	}
	if got := stop.Args[event.ArgTable]; got != "public.prod_secrets" {
		t.Errorf("the refusal names table %q, want public.prod_secrets", got)
	}

	// Nothing was dropped at all: the whole-target recheck runs before the
	// first table-specific lock, so items — the one table in this run's own
	// plan — is exactly as the fixture left it.
	if n := scalarOn(ctx, t, target,
		`SELECT count(*)::int FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
          WHERE n.nspname = 'public' AND c.relname = 'items'`); n != 1 {
		t.Errorf("public.items is gone; the whole-target recheck must refuse before any table is dropped (count %d)", n)
	}
	if n := scalarOn(ctx, t, target, `SELECT count(*)::int FROM items`); n != 0 {
		t.Errorf("the target's items holds %d rows, want the 0 the fixture created it with", n)
	}

	// And the point of the whole thing: the rows nobody authorised are still
	// exactly as somebody left them.
	if n := scalarOn(ctx, t, target, `SELECT count(*)::int FROM prod_secrets`); n != 2 {
		t.Errorf("prod_secrets holds %d rows, want the 2 the test inserted: nothing here is lazyslice's to touch", n)
	}

	// The run says so about itself: the marker row it wrote before the drop
	// loop (ARCHITECTURE.md section 11.2) is closed failed, not left at
	// running and not complete (THREAT_MODEL.md T8) — the whole-target
	// recheck runs after that write and before the first drop, not before it.
	status := stringOn(ctx, t, target,
		`SELECT status FROM lazyslice_meta ORDER BY started_at DESC LIMIT 1`)
	if status != pg.StatusFailed {
		t.Errorf("the marker says %q after the refusal, want %q", status, pg.StatusFailed)
	}
}

// raceDatabasesWithASecondTable is the same fixture with one more table, so
// that a refusal can happen on the *second* drop rather than the first.
//
// ddl.DropTables drops in the reverse of (schema, name) order, so "notes" is
// dropped and committed before "items" is even looked at. That is the ordering
// the test below depends on, and it is stated here rather than discovered there.
func raceDatabasesWithASecondTable(ctx context.Context, t *testing.T) (source, target string) {
	t.Helper()

	admin := testutil.Postgres(ctx, t, "")
	source = createDatabase(ctx, t, admin, "app_race_source")
	target = createDatabase(ctx, t, admin, "app_race_target")
	execOn(ctx, t, source,
		`CREATE TABLE items (id integer PRIMARY KEY)`,
		`CREATE TABLE notes (id integer PRIMARY KEY)`,
		`INSERT INTO items VALUES (1)`,
	)
	execOn(ctx, t, target,
		`CREATE TABLE items (id integer PRIMARY KEY)`,
		`CREATE TABLE notes (id integer PRIMARY KEY)`,
	)
	return source, target
}

// The scope of the refusal, asserted rather than assumed: the transaction that
// refuses rolls back, and the drops that were already committed stay committed.
//
// This is the case the test above cannot reach, because it has one table and so
// the loop never has a second iteration. The drop order here is notes, then
// items; the row goes into items. What the run must do is refuse on items
// without touching it — and what it does *not* do is put notes back, because
// notes was dropped in a transaction of its own that had already committed,
// after its own lock-and-recheck found it empty. Nothing unauthorised is
// destroyed; a table definition the run was authorised to drop is gone, the
// marker is closed failed, and the target is left part-way through a rebuild
// that the next run's gate refuses rather than silently finishes. Both
// amendments say this in those terms (ARCHITECTURE.md §11.2, THREAT_MODEL.md
// T2); an earlier wording said "nothing was destroyed", which is true of the one
// refusing transaction and not of the loop.
func TestARefusalOnTheSecondTableLeavesTheFirstDropped(t *testing.T) {
	ctx := t.Context()
	testutil.SkipWithoutDocker(ctx, t)
	quietRungs(t)

	source, target := raceDatabasesWithASecondTable(ctx, t)
	guard := holdSource(ctx, t, source, "items")

	watcher := newGateWatcher()
	done := make(chan error, 1)
	go func() {
		_, err := Run(ctx, raceRequest(t, source, target), watcher)
		done <- err
	}()

	watcher.waitForGate(t)

	// Strictly after the gate, strictly before the first drop, into the table
	// that is dropped second.
	execOn(ctx, t, target, `INSERT INTO items VALUES (999)`)
	guard.release(ctx)

	var err error
	select {
	case err = <-done:
	case <-time.After(5 * time.Minute):
		t.Fatal("the run never finished")
	}

	var stop *Stop
	if !errors.As(err, &stop) {
		t.Fatalf("Run = %v, want the lock-and-recheck refusal", err)
	}
	if stop.Code != load.CodeRefusedTargetChanged || stop.Exit != exitTarget {
		t.Fatalf("stop = %s/exit %d, want %s/exit %d",
			stop.Code, stop.Exit, load.CodeRefusedTargetChanged, exitTarget)
	}
	if got := stop.Args[event.ArgTable]; got != "public.items" {
		t.Errorf("the refusal names table %q, want public.items — the table the row went into", got)
	}

	// The table the refusal is about is untouched, rows and all.
	if n := scalarOn(ctx, t, target, `SELECT count(*)::int FROM items`); n != 1 {
		t.Errorf("the target's items holds %d rows, want the one row the test inserted and nothing of ours", n)
	}

	// And the table dropped before it is gone, which is the honest half.
	if n := scalarOn(ctx, t, target,
		`SELECT count(*)::int FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
          WHERE n.nspname = 'public' AND c.relname = 'notes'`); n != 0 {
		t.Errorf("notes is still there after a refusal on a later table; the drops are not one transaction, "+
			"so if this passes the amendments' account of the scope of a refusal is what is wrong (count %d)", n)
	}

	status := stringOn(ctx, t, target,
		`SELECT status FROM lazyslice_meta ORDER BY started_at DESC LIMIT 1`)
	if status != pg.StatusFailed {
		t.Errorf("the marker says %q after the refusal, want %q", status, pg.StatusFailed)
	}
}

// The marked target's half of the recheck: the marker row that authorised the
// truncation is deleted after the gate read it, and the run refuses instead of
// truncating.
//
// This is the production reload path — a target lazyslice itself wrote, whose
// tables are full by design — and it is the branch the emptiness recheck can say
// nothing about. The authorisation is the row, so a row that is gone authorises
// nothing, and the rows the previous run loaded are still there afterwards.
func TestAMarkerDeletedAfterTheGateIsNotTruncated(t *testing.T) {
	ctx := t.Context()
	testutil.SkipWithoutDocker(ctx, t)
	quietRungs(t)

	source, target := raceDatabases(ctx, t)

	// A first run, unheld, so that the target is one lazyslice wrote: it holds
	// the source's row and a marker row bound to this source and this catalog.
	if _, err := Run(ctx, raceRequest(t, source, target), event.Discard); err != nil {
		t.Fatalf("the run that marks the target: %v", err)
	}

	guard := holdSource(ctx, t, source, "items")
	watcher := newGateWatcher()
	done := make(chan error, 1)
	go func() {
		_, err := Run(ctx, raceRequest(t, source, target), watcher)
		done <- err
	}()

	watcher.waitForGate(t)

	// Strictly after the gate read the marker, strictly before this run writes
	// its own row and drops anything.
	execOn(ctx, t, target, `DELETE FROM lazyslice_meta`)
	guard.release(ctx)

	var err error
	select {
	case err = <-done:
	case <-time.After(5 * time.Minute):
		t.Fatal("the run never finished")
	}

	var stop *Stop
	if !errors.As(err, &stop) {
		t.Fatalf("Run = %v, want the marker recheck refusal", err)
	}
	if stop.Code != load.CodeRefusedMarkerChanged || stop.Exit != exitTarget {
		t.Fatalf("stop = %s/exit %d, want %s/exit %d",
			stop.Code, stop.Exit, load.CodeRefusedMarkerChanged, exitTarget)
	}
	if got := stop.Args[event.ArgTable]; got != "public.items" {
		t.Errorf("the refusal names table %q, want public.items", got)
	}

	// Nothing was truncated: the first run's row is still in the target.
	if n := scalarOn(ctx, t, target, `SELECT count(*)::int FROM items`); n != 1 {
		t.Errorf("the target's items holds %d rows, want the 1 the previous run loaded", n)
	}

	// And this run says so about itself.
	status := stringOn(ctx, t, target,
		`SELECT status FROM lazyslice_meta ORDER BY started_at DESC LIMIT 1`)
	if status != pg.StatusFailed {
		t.Errorf("the marker says %q after the refusal, want %q", status, pg.StatusFailed)
	}
}

// A marker-bound reload is not refused for a table the previous run legitimately
// filled, just because today's source has since dropped the table that used to
// produce it.
//
// This is T-0242 round 5
// (docs/reviews/2026-09-16-redteam/round5-marker-bound-false-positive.json):
// on a bound marker, pg.Target.Gate returns Eligible as soon as the marker binds
// — before checkEmpty, ARCHITECTURE.md §9 rule 5, ever runs — so the gate never
// asked anything about any table on this path. The whole-target recheck (T-0242
// round 4) compared the target's occupied tables against *this run's plan*,
// which for a marker-bound reload is built from today's source schema alone;
// a table the previous run loaded, whose source table has since been dropped,
// is not in today's plan and not in anything the gate looked at either, so it
// read as "appeared since the gate" and refused load.refused.target_changed on
// a table nobody touched. Now the whole-target recheck skips entirely on a
// bound marker, and this asserts the reload it used to break succeeds, leaving
// the orphaned table exactly as the previous run left it.
func TestAMarkerBoundReloadIgnoresATableItsSourceNoLongerHas(t *testing.T) {
	ctx := t.Context()
	testutil.SkipWithoutDocker(ctx, t)
	quietRungs(t)

	source, target := raceDatabasesWithASecondTable(ctx, t)

	// A first run, unheld: the target ends up marker-bound, holding items and
	// notes. notes has no foreign key onto items (the fixture's root), so the
	// plan recreates it but copies no rows into it — the ordinary shape for a
	// table outside the root's own reach (ARCHITECTURE.md §3's
	// schema-only step).
	if _, err := Run(ctx, raceRequest(t, source, target), event.Discard); err != nil {
		t.Fatalf("the run that marks the target: %v", err)
	}

	// A row in notes, put there directly rather than through a second run:
	// on a bound marker the target's rows are the previous run's own by
	// design (TestARecheckOfTheWholeTargetIgnoresAnOccupiedPlanTable's own
	// comment says so for a plan table, and notes is no different once it is
	// no longer in the plan at all) — what matters here is only that notes
	// is *occupied* when the second run's gate reads the target, which is
	// the whole-target recheck's own rule 5 question. An empty orphaned
	// table never reached the bug: an unoccupied table was never in
	// ProbeEmptiness's own "occupied" answer to begin with, whichever plan
	// it was compared against.
	execOn(ctx, t, target, `INSERT INTO notes VALUES (1)`)

	// Strictly between the two runs, the source drops the table the target's
	// notes came from — the shape of a schema that has moved on, not of
	// anything the target is answerable for.
	execOn(ctx, t, source, `DROP TABLE notes`)

	if _, err := Run(ctx, raceRequest(t, source, target), event.Discard); err != nil {
		t.Fatalf("the marker-bound reload = %v, want nil: notes is the previous run's own table, "+
			"not a table that appeared since the gate", err)
	}

	// The reload's own plan — today's source, which no longer has notes —
	// touched only items; notes and its row are exactly as the first run left
	// them.
	if n := scalarOn(ctx, t, target,
		`SELECT count(*)::int FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
          WHERE n.nspname = 'public' AND c.relname = 'notes'`); n != 1 {
		t.Errorf("notes is gone after the reload; the reload's plan never named it and must not have touched it (count %d)", n)
	}
	if n := scalarOn(ctx, t, target, `SELECT count(*)::int FROM notes`); n != 1 {
		t.Errorf("notes holds %d rows after the reload, want the 1 row the first run left — untouched, not truncated", n)
	}
	if n := scalarOn(ctx, t, target, `SELECT count(*)::int FROM items`); n != 1 {
		t.Errorf("the target's items holds %d rows, want the 1 the reload loaded", n)
	}

	status := stringOn(ctx, t, target,
		`SELECT status FROM lazyslice_meta ORDER BY started_at DESC LIMIT 1`)
	if status != pg.StatusComplete {
		t.Errorf("the marker says %q after the reload, want %q", status, pg.StatusComplete)
	}
}

// A second run against a target another run holds is refused at exit 4, naming
// the run that holds it.
//
// The lease is the other half of finding 1: the emptiness the gate approved can
// also be taken away by a *second lazyslice*, which would pass the same gate and
// drop the same tables in parallel. One run holds the target from before the
// gate's first probe until its marker row is closed.
func TestASecondRunIsRefusedWhileAnotherHoldsTheTarget(t *testing.T) {
	ctx := t.Context()
	testutil.SkipWithoutDocker(ctx, t)
	quietRungs(t)

	source, target := raceDatabases(ctx, t)
	guard := holdSource(ctx, t, source, "items")

	watcher := newGateWatcher()
	done := make(chan error, 1)
	go func() {
		_, err := Run(ctx, raceRequest(t, source, target), watcher)
		done <- err
	}()

	// The first run is past the gate and holding the target; it is blocked on
	// the source and will stay there until this test lets it go.
	watcher.waitForGate(t)

	_, err := Run(ctx, raceRequest(t, source, target), event.Discard)

	var stop *Stop
	if !errors.As(err, &stop) {
		t.Fatalf("the second run = %v, want the lease refusal", err)
	}
	if stop.Code != pg.CodeLeaseHeld || stop.Exit != exitTarget {
		t.Fatalf("stop = %s/exit %d, want %s/exit %d",
			stop.Code, stop.Exit, pg.CodeLeaseHeld, exitTarget)
	}
	holder := stop.Args[event.ArgReason]
	if holder == "" || holder == "another lazyslice run" {
		t.Fatalf("the refusal names the holder as %q; it must name the run id that holds the target", holder)
	}
	if got := stop.Args[event.ArgDatabase]; got != "app_race_target" {
		t.Errorf("the refusal names database %q, want app_race_target", got)
	}

	guard.release(ctx)
	select {
	case err = <-done:
	case <-time.After(5 * time.Minute):
		t.Fatal("the first run never finished")
	}
	if err != nil {
		t.Fatalf("the run that held the target: %v", err)
	}

	// The name the second run printed is the first run's, and the marker row is
	// what proves it: the lease and the marker carry one id.
	runID := stringOn(ctx, t, target,
		`SELECT run_id::text FROM lazyslice_meta ORDER BY started_at DESC LIMIT 1`)
	if !strings.EqualFold(runID, holder) {
		t.Errorf("the refusal named run %q and the run that held the target recorded itself as %q",
			holder, runID)
	}

	// And the lease is given back: a third run against the same target passes
	// the gate rather than finding a lock nobody released.
	third := newGateWatcher()
	if _, err := Run(ctx, raceRequest(t, source, target), third); err != nil {
		t.Fatalf("a run after the lease was released: %v", err)
	}
}

// scalarOn and stringOn read one value out of one database.
func scalarOn(ctx context.Context, t *testing.T, connURL, sql string) int {
	t.Helper()

	conn, err := pgx.Connect(ctx, connURL)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()

	var n int
	if err := conn.QueryRow(ctx, sql).Scan(&n); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return n
}

func stringOn(ctx context.Context, t *testing.T, connURL, sql string) string {
	t.Helper()

	conn, err := pgx.Connect(ctx, connURL)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()

	var s string
	if err := conn.QueryRow(ctx, sql).Scan(&s); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return s
}
