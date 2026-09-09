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

	// TargetTables are the user tables the target already holds, as the gate
	// enumerated them. pipeline.Writer has Exec, CopyFrom and Begin and no way
	// to read (section 2), so a table the target holds under a name the source
	// does not use is dropped only when the caller names it here; one this
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
		closing, cancel := context.WithTimeout(context.WithoutCancel(ctx), markerCloseTimeout)
		defer cancel()
		//nolint:errcheck // A marker this cannot close stays at running, which
		// authorises the next run to truncate exactly as failed does (§11.2),
		// so there is nothing to do with the error and the failure being
		// returned is the one worth printing.
		pg.FinishRun(closing, w, runID, pg.StatusFailed, totalRows(res))
		return nil, err
	}
	res.Elapsed = time.Since(started)
	if err := pg.FinishRun(ctx, w, runID, pg.StatusComplete, totalRows(res)); err != nil {
		return nil, err
	}
	return res, nil
}

// markerCloseTimeout bounds the marker update on a failure path, where the
// run's own context may already be cancelled.
const markerCloseTimeout = 5 * time.Second

func (l loader) markerRow(plan *pipeline.Plan, fingerprint string) pg.MarkerRow {
	return pg.MarkerRow{
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
// **A Writer that is not a pipeline.TypeRegistrar is an error here, not a
// skipped step.** The registration is reached through an optional interface —
// pipeline.Writer is ARCHITECTURE.md section 2's three methods and registration
// needs the schema, which is the loader's argument and not the connection's —
// and an optional interface that misses is a step that vanishes with no compile
// error. internal/core already wraps this same writer in a readableWriter for
// verify, and a readableWriter embeds the Writer interface, so it is one
// refactor away from being what the loader is handed: the assertion would miss,
// the load would carry on, and the failure would reappear as a mid-table encode
// error in the integration suite and nowhere else. It therefore names the type
// it was given and refuses. Every Writer in this tree registers types; a second
// engine that genuinely cannot is a change to pipeline.Writer, not a silent
// return (tracker T-0093).
func registerTypes(ctx context.Context, w pipeline.Writer, schema *pipeline.Schema) error {
	r, ok := w.(pipeline.TypeRegistrar)
	if !ok {
		return refuse(CodeRefusedDDL, exitLoad, ref.TableRef{}, "",
			fmt.Errorf("the target writer (%T) cannot register the source's user-defined "+
				"types, so a composite or an array of a user-defined type would fail mid-copy", w))
	}
	if err := r.RegisterTypes(ctx, schema); err != nil {
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
		if err := w.Exec(ctx, d.SQL); err != nil {
			return refuse(CodeRefusedDDL, exitLoad, d.Table, "", err)
		}
	}
	for _, sql := range ddl.DropObjects(schema) {
		if err := w.Exec(ctx, sql); err != nil {
			return refuse(CodeRefusedDDL, exitLoad, ref.TableRef{}, "", err)
		}
	}
	return nil
}

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
		n, err := tx.CopyFrom(ctx, tc.table, cols, tc.rows)
		tc.done <- copyResult{n: n, err: err}
	}()
	return tc, nil
}

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

func totalRows(res *pipeline.LoadResult) int64 {
	if res == nil {
		return 0
	}
	var n int64
	for _, v := range res.Rows {
		n += v
	}
	return n
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
