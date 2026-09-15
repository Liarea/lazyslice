// SPDX-License-Identifier: Apache-2.0

// Package load recreates the source schema in the target and copies the rows in.
//
// lazyslice owns the target schema (ADR-005): after the gate has passed, every
// user table in the target is dropped, each drop printed before it happens, and
// the object classes in ARCHITECTURE.md section 11.1 are recreated from the
// introspected schema by internal/load/ddl.
//
// Data goes in with CopyFrom under one transaction per table, opened at Seq 0
// and committed at Last. Indexes, foreign keys NOT VALID then VALIDATE, setval
// and ANALYZE all happen after the data, so no edge is deferred and load order
// inside a cycle does not matter.
//
// The property this package exists to hold is THREAT_MODEL.md T8's, stated as it
// is rather than as one would wish it: after any failure the target is either
// empty or carries a marker row at running or failed, and every table in it
// holds either none of its rows or all of them. One transaction per table is
// what makes the second half true; the marker row, written before the first
// drop, is what makes the first half survive a kill -9, where no cleanup of ours
// runs at all.
package load

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/load/ddl"
	"github.com/Liarea/lazyslice/internal/pg"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// Run is what this run records in the marker table (ARCHITECTURE.md section
// 11.2). Every field is a fingerprint, an identifier or a count: nothing here is
// a value from either database, and no field holds a DSN.
//
// Root and Take are not here — they are the plan's, and Load is given the plan.
// SchemaFingerprint is not here either, and deliberately: it is section 11.2's
// one field whose value must be computed by the same function the gate later
// recomputes, and there are two definitions of it in the tree (see
// SchemaFingerprint). A caller that could pass it in could pass the other one,
// and a marker written with one and checked with the other never binds — so
// Load computes it from the schema it is loading and offers the gate's end of
// the same binding as GateFingerprint.
type Run struct {
	ToolVersion               string
	SourceFingerprint         string
	SourceSystemID            string
	ClassificationFingerprint string
	SecretFingerprint         string

	// RunID is the id this run's marker row carries. It is the caller's because
	// the run lease on the target named itself with it before the gate ran
	// (internal/pg's Lease), and a marker under a different id would leave the
	// refusal a second run prints unconnected to the row this one writes. Empty
	// means "make one", which is what a direct caller with no lease gets.
	RunID string

	// What the gate approved, as it approved it (ARCHITECTURE.md section 11.2,
	// amended 2026-09-14). The loader re-verifies it under the ACCESS EXCLUSIVE
	// lock it takes before each drop, because a gate verdict is a remembered
	// decision by then and not continuing ownership of the target.
	//
	// MarkerBound false — the zero value, and what a caller that says nothing
	// gets — means the gate approved an *unmarked* target, whose authorisation
	// was that every user table was empty; the recheck is that each table it is
	// about to drop is still empty. MarkerBound true means a bound marker
	// authorised the truncation, and the recheck is that MarkerRunID's row is
	// still there with MarkerStatus. The fail-closed direction is the default: a
	// caller that forgets to say gets the stricter check, not the weaker one.
	MarkerBound  bool
	MarkerRunID  string
	MarkerStatus string

	// TargetTables are the user tables the target already holds, as the gate
	// enumerated them. pipeline.Writer has no way to read (section 2), so a table
	// the target holds under a name the source does not use is dropped only when
	// the caller names it here; one this
	// package is not told about is left alone rather than dropped silently.
	TargetTables []ref.TableRef
}

type loader struct {
	run  Run
	sink event.Sink
}

// New returns the target loader.
//
// sink may be nil, in which case nothing is printed. It is here, and Load's
// signature in ARCHITECTURE.md section 2 has no room for it, because section
// 11.1 requires each drop to be printed before it happens: the loader is the
// only stage that destroys anything, and a list of drops returned at the end of
// the stage is a list printed after the tables are gone. core.Run stays the only
// producer of events by handing its own sink in.
func New(run Run, sink event.Sink) pipeline.Loader {
	if sink == nil {
		sink = event.Discard
	}
	return loader{run: run, sink: sink}
}

var _ pipeline.Loader = loader{}

// Load recreates the object classes in section 11.1, copies every table under
// one transaction opened at Seq 0 and committed at Last, then creates indexes,
// adds foreign keys NOT VALID and validates them, resets sequences and runs
// ANALYZE.
func (l loader) Load(
	ctx context.Context,
	w pipeline.Writer,
	plan *pipeline.Plan,
	schema *pipeline.Schema,
	in <-chan pipeline.RowBatch,
) (*pipeline.LoadResult, error) {
	// The channel is drained on every path. Extract writes into it with a
	// select on its own context, so a load that stopped early and left the
	// channel full would hang the run rather than fail it.
	defer drain(ctx, in)

	if w == nil {
		return nil, errors.New("load: no target")
	}
	if plan == nil {
		return nil, errors.New("load: no plan")
	}
	if schema == nil {
		return nil, errors.New("load: no schema")
	}

	// ARCHITECTURE.md section 11.1's not-recreatable refusal is *not* raised
	// here. It is raised at plan, which is where section 11.1 says it is raised
	// -- "before the snapshot is used for keys and before anything in the target
	// is dropped" -- by internal/core's planStage, which calls the same
	// ddl.Recreatable this used to call as its first statement (T-0097). The
	// check was here because a stage package may not import another stage
	// package and the caller section 11.1 describes is core, which did not exist
	// when this loader landed; it does now. From inside Load the refusal arrived
	// after the snapshot had been used for every key and every row had been
	// extracted, which is what mastodon -- one of the ten schemas in
	// testdata/torture/, and the only one that reaches the refusal as the
	// fixtures stand -- paid. (GitLab reaches it upstream on two objects its
	// 43-table subset removes; docs/TORTURE.md records both halves.)
	//
	// Nothing replaces it here, deliberately. A second copy would be one refusal
	// two stages could raise, and the exit-13 message names the table, the
	// column and the dependency in one place.
	//
	// State the consequence plainly, because it is a real one: **Load now has an
	// unchecked precondition.** Its caller must have run ddl.Recreatable over
	// the same *pipeline.Schema it passes here, and nothing in this package
	// enforces that. core.Run does make the call (planStage), and
	// load_integration_test.go calls ddl.Recreatable over its fixture before
	// building a plan -- but as a fixture assertion, not as a guard on Load, so
	// it documents the precondition without enforcing it either. A future direct
	// caller that skips the check drops every table in the target and then fails
	// at CREATE TABLE with 42883 under exit 7, which is the failure the check
	// used to prevent from here. If a second in-tree caller of Load ever
	// appears, the check belongs at that call site too, or on a constructor that
	// cannot be built without it.

	// Section 11.2's schema_fingerprint, computed here rather than accepted from
	// the caller: the gate recomputes it over the target's catalog, and the two
	// ends only bind when they are the same function (see SchemaFingerprint and
	// GateFingerprint). It is computed before the marker row, because a schema
	// that cannot be rendered to DDL is a failure that should happen before the
	// target is touched at all.
	fingerprint, err := SchemaFingerprint(schema)
	if err != nil {
		return nil, err
	}

	started := time.Now()

	// The marker row is written before the first drop, so that a run which dies
	// between the drop and the last commit leaves the target with a row at
	// running — which the next run's gate treats exactly as it treats complete,
	// and truncates (ARCHITECTURE.md section 11.2).
	if err = pg.EnsureMarker(ctx, w); err != nil {
		return nil, err
	}
	runID, err := pg.StartRun(ctx, w, l.markerRow(plan, fingerprint))
	if err != nil {
		return nil, err
	}

	res, err := l.load(ctx, w, plan, schema, in)
	if err != nil {
		// Best effort, on a context of its own: a cancelled run still has a
		// marker row to close, and a row left at running authorises the next
		// run to truncate exactly as failed does, so a failure here changes
		// nothing about what happens next.
		closing, cancel := context.WithTimeout(context.WithoutCancel(ctx), MarkerCloseTimeout)
		defer cancel()
		//nolint:errcheck // A marker this cannot close stays at running, which
		// authorises the next run to truncate exactly as failed does (§11.2),
		// so there is nothing to do with the error and the failure being
		// returned is the one worth printing.
		pg.FinishRun(closing, w, runID, pg.StatusFailed, TotalRows(res))
		return nil, err
	}
	res.Elapsed = time.Since(started)

	// The row is deliberately left at StatusRunning here (T-0133, 2026-09-14,
	// docs/reviews/2026-09-09 finding 4): this package used to close it at
	// StatusComplete right here, before core had run verify at all, so a run
	// that went on to fail verification — a value the run masked still present
	// in the target, confirmed against the source — exited non-zero over a
	// target whose own marker said complete. core is the caller that holds
	// both halves of that question: whether Load succeeded, which this return
	// answers, and whether verify then passed, which happens after Load has
	// already returned. So core.Run closes this run's row now, at
	// StatusComplete once verify has actually passed and at StatusFailed after
	// any verify failure (internal/pg/marker.go has the reasoning for why that
	// is StatusFailed and not a fourth status).
	//
	// A run that dies between this return and core's close — kill -9, a crash,
	// core losing its connection — leaves the row at StatusRunning, which is
	// exactly the state a run killed mid-copy already leaves it in, and the
	// gate already treats running and complete identically: truncate and
	// print (ARCHITECTURE.md section 11.2). Nothing downstream of this
	// function needs a fourth status to know the row does not yet authorise
	// anything beyond that.
	return res, nil
}

// MarkerCloseTimeout bounds a marker update — or, since T-0133, a quarantine
// drop — made on a detached context after the run's own context may already be
// cancelled. It is exported so that core.closeRun can derive the same bounded,
// detached context for load.DropLoaded and pg.FinishRun that this package
// derives for its own close on Load's failure path (below): a cancelled run
// still has cleanup to do, and the signal that ended the run is not a reason to
// cancel it.
const MarkerCloseTimeout = 5 * time.Second

func (l loader) markerRow(plan *pipeline.Plan, fingerprint string) pg.MarkerRow {
	return pg.MarkerRow{
		RunID:                     l.run.RunID,
		ToolVersion:               l.run.ToolVersion,
		SourceFingerprint:         l.run.SourceFingerprint,
		SourceSystemID:            l.run.SourceSystemID,
		SchemaFingerprint:         fingerprint,
		ClassificationFingerprint: l.run.ClassificationFingerprint,
		RootTable:                 plan.Root.String(),
		Take:                      plan.Take,
		SecretFingerprint:         l.run.SecretFingerprint,
	}
}

func (l loader) load(
	ctx context.Context,
	w pipeline.Writer,
	plan *pipeline.Plan,
	schema *pipeline.Schema,
	in <-chan pipeline.RowBatch,
) (*pipeline.LoadResult, error) {
	res := &pipeline.LoadResult{
		Rows:      map[ref.TableRef]int64{},
		Sequences: map[ref.TableRef][]string{},
	}

	if err := l.drop(ctx, w, schema); err != nil {
		return res, err
	}
	pre, err := ddl.PreData(schema, plan)
	if err != nil {
		return res, err
	}
	for _, sql := range pre {
		if err := w.Exec(ctx, sql); err != nil {
			return res, refuse(CodeRefusedDDL, exitLoad, ref.TableRef{}, "", err)
		}
	}
	if err := registerTypes(ctx, w, schema); err != nil {
		return res, err
	}
	if err := l.copy(ctx, w, plan, in, res); err != nil {
		return res, err
	}
	if err := l.postData(ctx, w, plan, schema, res); err != nil {
		return res, err
	}
	return res, nil
}

// registerTypes is the step between item 3 of ARCHITECTURE.md section 11.1 and
// the first CopyFrom: the source's enums, domains, composites and the array
// types over them are registered on the target's connections, which is what
// makes a value of one of them encodable at all (ADR-005, "types registered in
// AfterConnect").
//
// It runs here and not before the DDL because the types do not exist in the
// target until the DDL has created them — lazyslice owns the target schema — and
// not after the copy because the copy is what needs them. Without it a composite
// column fails 42804 and an enum array fails 54000, mid-table; T8's per-table
// transaction then leaves the target empty or complete, and the run still fails.
//
// Registration is a method on pipeline.Writer and
// not an optional second interface, so this step cannot go missing: until
// T-0093 it was reached through w.(pipeline.TypeRegistrar), and an optional
// interface that misses is a step that vanishes with no compile error.
// internal/core wraps this same writer in a readableWriter for verify, and that
// wrapper embeds the Writer interface, so it was one refactor away from being
// what the loader is handed — the assertion would have missed, the load would
// have carried on, and the failure would have reappeared as a mid-table encode
// error in the integration suite and nowhere else. A Writer that has nothing to
// register returns nil; a Writer that cannot register no longer compiles.
func registerTypes(ctx context.Context, w pipeline.Writer, schema *pipeline.Schema) error {
	if err := w.RegisterTypes(ctx, schema); err != nil {
		return refuse(CodeRefusedDDL, exitLoad, ref.TableRef{}, "", err)
	}
	return nil
}

// drop empties the target of everything ARCHITECTURE.md section 11.1 is about to
// recreate, printing each table before it goes.
//
// The marker table is never dropped: it is this run's own record, written a
// moment ago, and a source table that happens to share its name is not a reason
// to destroy it.
//
// Each table goes in a transaction of its own that takes the table's ACCESS
// EXCLUSIVE lock and then re-verifies what the gate approved, before the DROP
// (dropOne). Until T-0130 the drops ran on the writer in autocommit, and the
// only thing standing between `DROP TABLE` and somebody else's rows was a gate
// verdict reached several stages earlier: the 2026-09-09 review inserted a row
// into an approved-empty target after the gate and lazyslice deleted it and
// exited 0.
//
// What a refusal here does and does not undo, stated exactly, because the first
// version of both amendments said "nothing was destroyed" and that is a claim
// about one transaction and not about this loop. The transaction that refuses
// rolls back, so the table it refused about is untouched — that is the property
// the control exists for, and it holds for every table. Tables this loop already
// dropped in *earlier* transactions stay dropped: their drops were committed,
// each after its own lock-and-recheck, so nothing unauthorised was destroyed
// (on an unmarked target each was verified empty under its own lock; on a bound
// marker the truncation was authorised) — but their definitions are gone. The
// run's marker row is then closed failed and the target is left part-way through
// a rebuild, which the next run's gate refuses rather than silently finishes:
// the marker no longer binds, because its schema fingerprint is the source's and
// the target's catalog is now half of it, so the gate falls through to the
// emptiness check, finds the rows that caused the refusal, and prints the
// command that clears the database (ARCHITECTURE.md section 9, section 11.2 as
// amended).
//
// One transaction over every table would make the loop itself atomic, and is not
// what this does: the gate admits up to 2,000 user tables, DROP ... CASCADE
// takes a lock per dependent index, sequence and toast relation, and a single
// transaction over all of them is how a large target meets "out of shared
// memory: You might need to increase max_locks_per_transaction" instead of
// finishing. Refusing correctly on every table is worth more than undoing the
// drops that were already authorised.
func (l loader) drop(ctx context.Context, w pipeline.Writer, schema *pipeline.Schema) error {
	for _, d := range ddl.DropTables(schema, l.run.TargetTables) {
		if d.Table.Name == pg.MarkerTable {
			continue
		}
		l.sink.Send(event.Event{
			At:    time.Now(),
			Stage: event.Load,
			Kind:  event.Info,
			Code:  CodeDropping,
			Table: d.Table,
			Args:  event.Args{event.ArgTable: d.Table.String()},
		})
		if err := l.dropTable(ctx, w, d); err != nil {
			return err
		}
	}
	for _, sql := range ddl.DropObjects(schema) {
		if err := w.Exec(ctx, sql); err != nil {
			return refuse(CodeRefusedDDL, exitLoad, ref.TableRef{}, "", err)
		}
	}
	return nil
}

// lockAttempts and lockRetryPause bound the retry of a NOWAIT lock.
//
// NOWAIT is the point: a DROP that waits is a DROP that can sit behind an
// application's long transaction until somebody notices, and waiting is also
// how the window this whole mechanism closes gets *longer*. But the lock a
// freshly loaded target most often loses a race to is autovacuum's, which a
// waiting DROP would have cancelled automatically and a NOWAIT one simply fails
// against. Three attempts a tenth of a second apart is still bounded — a third
// of a second, not an application's transaction — and it is the difference
// between an honest refusal and a coin toss.
const (
	lockAttempts   = 3
	lockRetryPause = 100 * time.Millisecond
)

// dropTable is one table's drop, retried only for a lock that was not free.
func (l loader) dropTable(ctx context.Context, w pipeline.Writer, d ddl.TableDrop) error {
	for attempt := 1; ; attempt++ {
		err := l.dropOne(ctx, w, d)
		if err == nil {
			return nil
		}
		var refusal *Refusal
		if attempt >= lockAttempts || !errors.As(err, &refusal) || refusal.Code != CodeRefusedTargetLocked {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(lockRetryPause):
		}
	}
}

// dropOne is ARCHITECTURE.md section 11.2's lock-and-recheck for one table:
// take the table's ACCESS EXCLUSIVE lock without waiting, re-verify the thing
// that authorised the drop, drop, commit. A change refuses at exit 4 and the
// deferred rollback leaves the table exactly as it was found.
//
// The order is the whole of it. The lock comes first so that the recheck is a
// statement about a table nobody else can be writing to; the recheck comes
// before the DROP so that a refusal costs nothing; and both are in the
// transaction the DROP commits in, so no moment passes between "still as
// approved" and "gone".
func (l loader) dropOne(ctx context.Context, w pipeline.Writer, d ddl.TableDrop) error {
	tx, err := w.Begin(ctx)
	if err != nil {
		return refuse(CodeRefusedDDL, exitLoad, d.Table, "", err)
	}
	committed := false
	defer func() {
		if !committed {
			rollback(ctx, tx)
		}
	}()

	present, err := tableExists(ctx, tx, d.Table)
	if err != nil {
		return refuse(CodeRefusedDDL, exitLoad, d.Table, "", err)
	}
	if present {
		if lockErr := tx.Exec(ctx, lockStatement(d.Table)); lockErr != nil {
			// Only 55P03 is a contended target. A LOCK TABLE that failed for any
			// other reason is a load failure at exit 7, reported as itself
			// rather than as "something else is using this database" after a
			// third of a second of pointless retry (see lockNotAvailable).
			if !lockNotAvailable(lockErr) {
				return refuse(CodeRefusedDDL, exitLoad, d.Table, "", lockErr)
			}
			return refuse(CodeRefusedTargetLocked, exitTarget, d.Table, "", lockErr)
		}
		if recheckErr := l.recheck(ctx, tx, d.Table); recheckErr != nil {
			return recheckErr
		}
	}
	if dropErr := tx.Exec(ctx, d.SQL); dropErr != nil {
		return refuse(CodeRefusedDDL, exitLoad, d.Table, "", dropErr)
	}
	if commitErr := tx.Commit(ctx); commitErr != nil {
		return refuse(CodeRefusedDDL, exitLoad, d.Table, "", commitErr)
	}
	committed = true
	return nil
}

// recheck re-verifies the gate's authorisation for this table.
//
// Which question it asks is which question the gate answered: an unmarked target
// was approved because every user table was empty, so the table must still be
// empty; a marked one was approved because a bound marker row authorised the
// truncation, so that row must still be there unchanged. Asking the wrong one
// would either refuse every reload (a marked target's tables are full by
// design) or approve a stranger's rows.
func (l loader) recheck(ctx context.Context, tx pipeline.Tx, table ref.TableRef) error {
	if l.run.MarkerBound {
		return l.recheckMarker(ctx, tx, table)
	}
	n, err := rowCount(ctx, tx, table)
	if err != nil {
		return refuse(CodeRefusedDDL, exitLoad, table, "", err)
	}
	if n > 0 {
		return refuseChanged(CodeRefusedTargetChanged, table, n,
			"the target was approved empty and this table is not; nothing was dropped")
	}
	return nil
}

// recheckMarker re-reads the marker row the gate approved, by its run id. The
// row this run wrote a moment ago is a different row and is not what is checked:
// the authorisation is the *previous* run's row, and it is what must not have
// moved.
func (l loader) recheckMarker(ctx context.Context, tx pipeline.Tx, table ref.TableRef) error {
	if l.run.MarkerRunID == "" {
		// A caller that says the marker bound the target and cannot say which
		// row did has given no authorisation to check. Fail closed.
		return refuseChanged(CodeRefusedMarkerChanged, table, 0,
			"the run names no marker row, so nothing authorises truncating this target")
	}
	rows, err := tx.Query(ctx, sqlMarkerStatus, l.run.MarkerRunID)
	if err != nil {
		return refuse(CodeRefusedDDL, exitLoad, table, "", err)
	}
	defer rows.Close()

	var status string
	found := rows.Next()
	if found {
		if scanErr := rows.Scan(&status); scanErr != nil {
			return refuse(CodeRefusedDDL, exitLoad, table, "", scanErr)
		}
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return refuse(CodeRefusedDDL, exitLoad, table, "", rowsErr)
	}
	if !found || status != l.run.MarkerStatus {
		return refuseChanged(CodeRefusedMarkerChanged, table, 0,
			"the marker row that authorised truncating this target is gone or has changed")
	}
	return nil
}

// sqlMarkerStatus reads one marker row's status by run id. The cast is explicit
// because run_id is a uuid column and the id travels as text.
const sqlMarkerStatus = `SELECT status FROM ` + pg.MarkerTable + ` WHERE run_id = $1::uuid`

// sqlTableExists answers whether the target still holds a relation of this name,
// so that the lock is taken on a table that is there. to_regclass returns NULL
// rather than raising for a name that resolves to nothing.
const sqlTableExists = `SELECT to_regclass($1) IS NOT NULL`

func tableExists(ctx context.Context, tx pipeline.Tx, table ref.TableRef) (bool, error) {
	rows, err := tx.Query(ctx, sqlTableExists, quoted(table))
	if err != nil {
		return false, err
	}
	defer rows.Close()

	var present bool
	if !rows.Next() {
		if rowsErr := rows.Err(); rowsErr != nil {
			return false, rowsErr
		}
		return false, fmt.Errorf("load: the target did not answer whether %s exists", table)
	}
	if scanErr := rows.Scan(&present); scanErr != nil {
		return false, scanErr
	}
	return present, rows.Err()
}

// rowCount is the count the refusal names. The table is under ACCESS EXCLUSIVE
// by the time this runs, so the count is exact and nobody can change it between
// the count and the DROP; it is bounded in practice by whatever was written in
// the seconds since the gate approved the table as empty.
func rowCount(ctx context.Context, tx pipeline.Tx, table ref.TableRef) (int64, error) {
	rows, err := tx.Query(ctx, `SELECT count(*) FROM `+quoted(table))
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var n int64
	if !rows.Next() {
		if rowsErr := rows.Err(); rowsErr != nil {
			return 0, rowsErr
		}
		return 0, fmt.Errorf("load: the target did not answer how many rows %s holds", table)
	}
	if scanErr := rows.Scan(&n); scanErr != nil {
		return 0, scanErr
	}
	return n, rows.Err()
}

// lockStatement is the drop's own lock. NOWAIT, so that a target something else
// is using is refused rather than waited on.
func lockStatement(table ref.TableRef) string {
	return `LOCK TABLE ` + quoted(table) + ` IN ACCESS EXCLUSIVE MODE NOWAIT`
}

// quoted is the qualified, quoted name of a table, taken from internal/load/ddl
// so that the LOCK, the recheck and the DROP name one relation between them.
func quoted(t ref.TableRef) string { return ddl.TableName(t) }

// copy runs the extract → transform → load contract's receiving end.
//
// Tables are strictly sequential on the channel and every table ends with a
// batch carrying Last (ARCHITECTURE.md section 2), so the transaction boundary
// is read from Seq and Last and never inferred from a change of Table. A batch
// that breaks the contract is an error rather than a guess: guessing is how a
// table ends up committed in two halves.
//
// The channel closing is not the same thing as the stream finishing.
// internal/extract closes it on every path, its error returns included, so a
// producer that dies between tables — a cancelled standby read, a read refused
// on a later table, a dropped source connection — reaches here as an orderly end
// of channel with an early prefix of tables committed, the rest created and
// empty, and every foreign key validating, because an empty child satisfies any
// foreign key. That is THREAT_MODEL.md T8's "some tables loaded and status =
// complete" exactly. The plan is therefore the second half of the contract:
// every step it does not mark SchemaOnly must have delivered its Last, and a
// missing boundary fails the load so the marker is closed failed.
func (l loader) copy(
	ctx context.Context,
	w pipeline.Writer,
	plan *pipeline.Plan,
	in <-chan pipeline.RowBatch,
	res *pipeline.LoadResult,
) error {
	delivered := map[ref.TableRef]bool{}
	var cur *tableCopy
	defer func() {
		if cur != nil {
			cur.abort(ctx)
		}
	}()

	for b := range in {
		if err := ctx.Err(); err != nil {
			return err
		}
		if cur == nil {
			if b.Seq != 0 {
				return fmt.Errorf("load: %s opens at Seq %d, not 0", b.Table, b.Seq)
			}
			// A table whose every column is generated is copied as zero
			// columns: the target recomputes all of them, and COPY with an
			// empty column list is not a statement. It still gets a row count.
			if len(b.Cols) == 0 {
				if _, ok := res.Rows[b.Table]; !ok {
					res.Rows[b.Table] = 0
				}
				if b.Last {
					delivered[b.Table] = true
					continue
				}
				return fmt.Errorf("load: %s has no columns and more than one batch", b.Table)
			}
			started, err := l.begin(ctx, w, b)
			if err != nil {
				return err
			}
			cur = started
		}
		if cur.table != b.Table {
			return fmt.Errorf("load: %s arrived before %s was marked last", b.Table, cur.table)
		}
		for _, row := range b.Rows {
			if err := cur.send(ctx, row); err != nil {
				cur.abort(ctx)
				cur = nil
				return err
			}
		}
		if !b.Last {
			continue
		}
		n, err := cur.commit(ctx)
		table := cur.table
		cur = nil
		if err != nil {
			return err
		}
		res.Rows[table] = n
		delivered[table] = true
		l.sink.Send(event.Event{
			At:    time.Now(),
			Stage: event.Load,
			Kind:  event.Progress,
			Code:  CodeTableLoaded,
			Table: table,
			Done:  n,
			Args: event.Args{
				event.ArgTable: table.String(),
				event.ArgCount: fmt.Sprintf("%d", n),
			},
		})
	}
	if cur != nil {
		t := cur.table
		cur.abort(ctx)
		cur = nil
		return fmt.Errorf("load: the batches for %s ended without one marked last", t)
	}
	for _, step := range plan.Steps {
		if step.Mode == pipeline.SchemaOnly {
			continue
		}
		if !delivered[step.Table] {
			return fmt.Errorf(
				"load: the row stream ended before %s, which the plan copies; a load that stops "+
					"between tables is not a complete load", step.Table)
		}
	}
	return nil
}

// postData is item 6 of section 11.1: indexes, foreign keys NOT VALID then
// VALIDATE CONSTRAINT, setval, ANALYZE — in that order, after every table has
// been committed.
func (l loader) postData(
	ctx context.Context,
	w pipeline.Writer,
	plan *pipeline.Plan,
	schema *pipeline.Schema,
	res *pipeline.LoadResult,
) error {
	steps, err := ddl.PostDataSteps(schema, plan)
	if err != nil {
		return err
	}
	for _, s := range steps {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := w.Exec(ctx, s.SQL); err != nil {
			if s.Kind == ddl.StepValidate {
				return refuse(CodeRefusedFKInvalid, exitFK, s.Table, s.Object, err)
			}
			return refuse(CodeRefusedDDL, exitLoad, s.Table, s.Object, err)
		}
		if s.Kind == ddl.StepSetval {
			res.Sequences[s.Table] = append(res.Sequences[s.Table], s.Object)
		}
	}
	return nil
}

// tableCopy is one table's transaction and the CopyFrom running inside it.
//
// CopyFrom reads a channel, so it runs in a goroutine of its own and the batches
// are fed to it here. Every wait selects on that goroutine's result as well, so
// a COPY the server has already refused fails the table rather than deadlocking
// against a channel nobody is reading.
type tableCopy struct {
	table  ref.TableRef
	tx     pipeline.Tx
	rows   chan []any
	done   chan copyResult
	result *copyResult
}

type copyResult struct {
	n   int64
	err error
}

func (l loader) begin(ctx context.Context, w pipeline.Writer, b pipeline.RowBatch) (*tableCopy, error) {
	tx, err := w.Begin(ctx)
	if err != nil {
		return nil, refuse(CodeRefusedCopy, exitLoad, b.Table, "", err)
	}
	// ADR-005: synchronous_commit = off per transaction. The target is a
	// disposable local copy; a lost commit after a power cut costs one re-run
	// and the marker says which one.
	if err := tx.Exec(ctx, "SET LOCAL synchronous_commit = off"); err != nil {
		rollback(ctx, tx)
		return nil, refuse(CodeRefusedCopy, exitLoad, b.Table, "", err)
	}
	tc := &tableCopy{
		table: b.Table,
		tx:    tx,
		rows:  make(chan []any),
		done:  make(chan copyResult, 1),
	}
	cols := append([]string(nil), b.Cols...)
	go func() {
		// Recovered here, not left to guardedExecute: that recover only wraps
		// root.ExecuteContext on the main goroutine, and CopyFrom is fed
		// arbitrary masked production data over a driver library this package
		// does not control. A panic in it must become an error on this
		// goroutine's own result channel, the same as any other copy failure,
		// rather than an unredacted stack trace straight to stderr whatever
		// --debug says (2026-09-14 review of T-FAILUX, finding 1). tc.send and
		// tc.wait already select on tc.done, so a result that arrives this way
		// is read exactly like a normal one.
		defer func() {
			if v := recover(); v != nil {
				tc.done <- copyResult{err: copyPanicError{val: v, stack: debug.Stack()}}
			}
		}()
		n, err := tx.CopyFrom(ctx, tc.table, cols, tc.rows)
		tc.done <- copyResult{n: n, err: err}
	}()
	return tc, nil
}

// copyPanicError is a panic recovered off CopyFrom's own goroutine, converted
// into an ordinary error so it can travel the same Refusal path every other
// copy failure does. Its Stack method is picked up structurally by
// cmd/lazyslice's --debug reporting (errors.As against an unexported
// interface), the same as internal/core's own panicError.
type copyPanicError struct {
	val   any
	stack []byte
}

func (p copyPanicError) Error() string { return fmt.Sprintf("panic: %v", p.val) }
func (p copyPanicError) Stack() []byte { return p.stack }

func (tc *tableCopy) send(ctx context.Context, row []any) error {
	select {
	case tc.rows <- row:
		return nil
	case res := <-tc.done:
		tc.result = &res
		if res.err != nil {
			return refuse(CodeRefusedCopy, exitLoad, tc.table, "", res.err)
		}
		return fmt.Errorf("load: the copy into %s ended before its rows did", tc.table)
	case <-ctx.Done():
		return ctx.Err()
	}
}

// commit closes the row channel, waits for CopyFrom and commits. The
// transaction is rolled back on any failure, so the table is empty rather than
// part-written.
func (tc *tableCopy) commit(ctx context.Context) (int64, error) {
	res := tc.wait()
	if res.err != nil {
		rollback(ctx, tc.tx)
		return 0, refuse(CodeRefusedCopy, exitLoad, tc.table, "", res.err)
	}
	if err := tc.tx.Commit(ctx); err != nil {
		return 0, refuse(CodeRefusedCopy, exitLoad, tc.table, "", err)
	}
	return res.n, nil
}

// abort ends the table's transaction without committing. It is what makes a
// failure leave the table empty rather than holding the batches that did arrive.
func (tc *tableCopy) abort(ctx context.Context) {
	tc.wait()
	rollback(ctx, tc.tx)
}

// rollback ends a transaction that must not commit.
//
// The error is dropped here, in one place rather than at three call sites,
// because there is nothing to do with it: a rollback that itself fails leaves
// the transaction to end with the connection, which is the same outcome — the
// table holds none of its rows — and the failure being reported instead is the
// one worth printing.
func rollback(ctx context.Context, tx pipeline.Tx) {
	//nolint:errcheck // deliberate: see the comment above.
	tx.Rollback(ctx)
}

func (tc *tableCopy) wait() copyResult {
	if tc.result != nil {
		return *tc.result
	}
	close(tc.rows)
	res := <-tc.done
	tc.result = &res
	return res
}

// TotalRows sums a LoadResult's per-table counts. It is exported so that
// core.Run's own close of the marker row — now that Load no longer closes it
// on success (T-0133) — records the same rows_loaded value this package always
// has, rather than a second definition of the same sum.
func TotalRows(res *pipeline.LoadResult) int64 {
	if res == nil {
		return 0
	}
	var n int64
	for _, v := range res.Rows {
		n += v
	}
	return n
}

// DropLoaded drops every table section 11.1 recreates, after a residual-class
// verify failure (a *verify.Refusal whose Exit is 9: the residual scan, an
// unconfirmable hit, or the second net). core calls this, not Load, because
// verify — and therefore whether the failure is residual-class at all — runs
// after Load has already returned (T-0133, 2026-09-09 review finding 4): the
// target then holds personal data by definition, the check that just failed is
// what found it, and waiting for the next run's gate to truncate it leaves that
// data on disk in the meantime.
//
// It is best-effort across the whole list, not an abort on the first failure
// (2026-09-14 review finding 1). A single failing drop — a stray session
// holding a lock, a dependent object CASCADE cannot reach, a privilege error —
// used to end the loop right there, and every table after it in drop's order,
// every one of which holds the personal data this call exists to remove, was
// left exactly as it was. THREAT_MODEL.md's stated property is that the target
// ends the run either empty or holding nothing this run wrote; a caller that
// stops at the first failure cannot make that true for the tables it never
// tried. So this tries every table regardless of whether an earlier one
// failed, and returns every failure it collected, joined, rather than the
// first — core reports one warning per table named in the returned error
// rather than one for the whole call, so a transcript reader is told exactly
// how much is still there and not just that something is.
//
// Each drop takes its own ACCESS EXCLUSIVE lock NOWAIT first and retries a
// contended lock the same bounded way drop's dropTable does (lockAttempts,
// lockRetryPause): the lock a freshly loaded target most often loses a race to
// is autovacuum's, and a transient hold the ordinary reload survives should
// not be the reason a confirmed leak is left in the target. It is still
// deliberately not dropOne's lock-and-recheck: recheck re-verifies an *outside
// party's* authorisation for a truncation approved a moment earlier by
// something other than this run, and there is no such party here — every table
// this drops is one this same run recreated and copied into itself, seconds
// ago. lazyslice_meta is left alone, same as drop, so the row core is about to
// close to StatusFailed still has somewhere to land. sink may be nil.
func DropLoaded(ctx context.Context, w pipeline.Writer, schema *pipeline.Schema, sink event.Sink) error {
	if sink == nil {
		sink = event.Discard
	}
	if w == nil || schema == nil {
		return nil
	}
	var failures []error
	for _, d := range ddl.DropTables(schema, nil) {
		if d.Table.Name == pg.MarkerTable {
			continue
		}
		sink.Send(event.Event{
			At: time.Now(), Stage: event.Load, Kind: event.Info,
			Code: CodeQuarantineDropping, Table: d.Table,
			Args: event.Args{event.ArgTable: d.Table.String()},
		})
		if err := dropLoadedTable(ctx, w, d); err != nil {
			failures = append(failures, err)
		}
	}
	// The non-table objects this run created, after the tables that depend on
	// them (the 2026-09-15 red team's A07). THREAT_MODEL.md T8's amendment
	// claimed the target ends a content-class failure "either empty or holding
	// nothing this run wrote"; that was true of tables and false of everything
	// else the loader creates. A domain whose CHECK carried an address — the
	// object internal/verify's catalog pass had just refused the run over —
	// stayed in the target after the quarantine, and stayed again on every
	// rerun, because DropTables is a list of tables.
	//
	// There is no CASCADE here, for the reason DropObjects' own comment gives:
	// a type something still depends on after every table this run knows about
	// has been dropped is something the run has not been told about, and a loud
	// failure naming it is better than dropping a stranger's column. A failure
	// here joins the others rather than stopping the sweep, same as a table's.
	for _, o := range ddl.ObjectDrops(schema) {
		sink.Send(event.Event{
			At: time.Now(), Stage: event.Load, Kind: event.Info,
			Code: CodeQuarantineDroppingObject,
			Args: event.Args{event.ArgTable: o.Name},
		})
		if err := dropLoadedObject(ctx, w, o); err != nil {
			failures = append(failures, err)
		}
	}
	if len(failures) == 0 {
		return nil
	}
	return errors.Join(failures...)
}

// dropLoadedObject drops one non-table object for DropLoaded. It takes no lock
// ahead of the statement: there is no ACCESS EXCLUSIVE lock to take NOWAIT on a
// type or a sequence the way there is on a table, and DROP TYPE on an object
// nothing references does not block. A failure is returned and collected, never
// retried: the lock-contention case dropLoadedTable retries does not arise here.
func dropLoadedObject(ctx context.Context, w pipeline.Writer, o ddl.ObjectDrop) error {
	tx, err := w.Begin(ctx)
	if err != nil {
		return refuse(CodeRefusedDDL, exitLoad, ref.TableRef{}, "", err)
	}
	committed := false
	defer func() {
		if !committed {
			rollback(ctx, tx)
		}
	}()
	if dropErr := tx.Exec(ctx, o.SQL); dropErr != nil {
		return refuse(CodeRefusedDDL, exitLoad, ref.TableRef{}, "", dropErr)
	}
	if commitErr := tx.Commit(ctx); commitErr != nil {
		return refuse(CodeRefusedDDL, exitLoad, ref.TableRef{}, "", commitErr)
	}
	committed = true
	return nil
}

// dropLoadedTable drops one table for DropLoaded, retried only for a lock that
// was not free — the same bound drop's dropTable uses (lockAttempts,
// lockRetryPause). A plain DROP TABLE with no NOWAIT lock ahead of it would
// simply wait behind whatever holds the table, which is a quarantine that
// cannot afford to wait on a confirmed leak.
func dropLoadedTable(ctx context.Context, w pipeline.Writer, d ddl.TableDrop) error {
	for attempt := 1; ; attempt++ {
		err := dropLoadedOne(ctx, w, d)
		if err == nil {
			return nil
		}
		var refusal *Refusal
		if attempt >= lockAttempts || !errors.As(err, &refusal) || refusal.Code != CodeRefusedTargetLocked {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(lockRetryPause):
		}
	}
}

// dropLoadedOne takes the table's own ACCESS EXCLUSIVE lock NOWAIT and drops
// it in one transaction: the lock so a contended table fails fast and retries
// rather than DROP TABLE blocking indefinitely, and not dropOne's recheck, for
// the reason DropLoaded's own comment gives.
func dropLoadedOne(ctx context.Context, w pipeline.Writer, d ddl.TableDrop) error {
	tx, err := w.Begin(ctx)
	if err != nil {
		return refuse(CodeRefusedDDL, exitLoad, d.Table, "", err)
	}
	committed := false
	defer func() {
		if !committed {
			rollback(ctx, tx)
		}
	}()

	present, err := tableExists(ctx, tx, d.Table)
	if err != nil {
		return refuse(CodeRefusedDDL, exitLoad, d.Table, "", err)
	}
	if present {
		if lockErr := tx.Exec(ctx, lockStatement(d.Table)); lockErr != nil {
			if !lockNotAvailable(lockErr) {
				return refuse(CodeRefusedDDL, exitLoad, d.Table, "", lockErr)
			}
			return refuse(CodeRefusedTargetLocked, exitTarget, d.Table, "", lockErr)
		}
	}
	if dropErr := tx.Exec(ctx, d.SQL); dropErr != nil {
		return refuse(CodeRefusedDDL, exitLoad, d.Table, "", dropErr)
	}
	if commitErr := tx.Commit(ctx); commitErr != nil {
		return refuse(CodeRefusedDDL, exitLoad, d.Table, "", commitErr)
	}
	committed = true
	return nil
}

// drain empties the batch channel so that a producer blocked on a full channel
// can finish. It stops early when the context is done, which is the case where
// extract is stopping too.
func drain(ctx context.Context, in <-chan pipeline.RowBatch) {
	for {
		select {
		case _, ok := <-in:
			if !ok {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}
