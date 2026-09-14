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
