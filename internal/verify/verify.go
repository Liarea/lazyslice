// SPDX-License-Identifier: Apache-2.0

// Package verify proves what the run did, and is the only place that says the
// green tick is earned.
//
// Five things happen here (ARCHITECTURE.md section 6): every foreign key
// validates; the residual filter is tested against every masked column of the
// target and each hit confirmed against the source in a fresh short
// transaction; the classifier's own validators re-run over the full contents of
// every column that is not fully masked (the second net); sequences and row
// counts are checked; unmasked columns are compared byte for byte against the
// source on a sample.
//
// Verify fails closed. A residual hit that cannot be tested, because the source
// will not open, a probe errors, the role lacks SELECT or the probe cap is
// reached, is exit 9 with the reason, never a printed note: an unconfirmable hit
// is the case where failing closed costs least.
//
// It never uses the run snapshot, which has been released by the time it starts.
// Every read of the source goes through Source.Short, and every statement it
// sends there matches a shape this package exports (shapes.go).
//
// # Reading the target
//
// pipeline.Writer writes and registers types and
// has no way to read, while every check below
// is a read of the target. Verify
// therefore asks the Writer it is handed for a Query method and refuses to run
// without one, rather than reporting a green tick over checks that did not
// happen. internal/pg's writer does not have that method yet; adding it is owed
// there, and internal/verify/CLAUDE.md carries the deviation.
package verify

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// DefaultProbeCap is ARCHITECTURE.md section 8's --residual-probe-cap: the
// number of source confirmation probes one run may issue. The cap exists
// because a case-folded probe can be a sequential scan on production, and
// because a bound parameter lands in the source's log under
// log_statement = 'all' (THREAT_MODEL.md T4).
const DefaultProbeCap = 1000

// sampleRows is section 6 item 5's sample: 100 rows per table, fetched by
// identity through the short transaction.
const sampleRows = 100

// Options is what the flags in ARCHITECTURE.md section 8 give this stage.
type Options struct {
	// ProbeCap is --residual-probe-cap. Zero means DefaultProbeCap; a negative
	// value means no probe may be issued at all, which makes the first hit
	// unconfirmable and therefore exit 9.
	ProbeCap int
}

// New returns the verifier.
func New(o Options) pipeline.Verifier { return verifier{opts: o} }

type verifier struct{ opts Options }

var _ pipeline.Verifier = verifier{}

// targetReader is the read side of the target. See the package comment: it is
// not on pipeline.Writer, and this stage cannot run without it.
type targetReader interface {
	Query(ctx context.Context, sql string, args ...any) (pipeline.Rows, error)
}

// state is one Verify call.
type state struct {
	opts   Options
	src    pipeline.Source
	target targetReader
	schema *pipeline.Schema
	plan   *pipeline.Plan
	cls    *pipeline.Classification
	res    pipeline.Residual
	lr     *pipeline.LoadResult

	tables map[ref.TableRef]*pipeline.Table
	steps  []pipeline.Step

	checks   []pipeline.Check
	failures []*Refusal
	rows     map[ref.TableRef]int64

	// short is the second transaction of section 6 item 3, opened on the first
	// hit that needs it and reused for every probe and every sample fetch after
	// it. One short transaction rather than one per hit: each is a connection
	// and a snapshot on production, and reusing it also means every
	// confirmation in a run is answered against one consistent source.
	short     pipeline.Reader
	shortErr  error
	shortOpen bool

	probes      int64
	unconfirmed int64
}

// Verify runs the checks of ARCHITECTURE.md section 6 in that section's order
// and returns the report they produce.
//
// Every check runs, whatever the ones before it found, so that one report names
// everything wrong with the target rather than the first thing. The exit code is
// the first failure in section 6's order, and the same failure comes back as a
// *Refusal so that core can render its code from the catalogue; a report with a
// non-zero ExitCode and a nil error is not a state this function produces.
func (v verifier) Verify(
	ctx context.Context,
	src pipeline.Source,
	w pipeline.Writer,
	schema *pipeline.Schema,
	plan *pipeline.Plan,
	cls *pipeline.Classification,
	res pipeline.Residual,
	lr *pipeline.LoadResult,
) (*pipeline.Report, error) {
	started := time.Now()
	if schema == nil {
		return nil, errors.New("verify: no schema: the second net canonicalises by the column types in it")
	}
	if plan == nil {
		return nil, errors.New("verify: no plan: the row counts are checked against it")
	}
	if cls == nil {
		return nil, errors.New("verify: no classification: a report over no decisions would call every column unmasked")
	}
	if res == nil {
		// The residual filter is the only control THREAT_MODEL.md T12 has, so a
		// missing one is a refusal and never a check that quietly passes.
		return nil, errors.New("verify: no residual filter: there is nothing to test the masker against")
	}
	if lr == nil {
		return nil, errors.New("verify: no load result")
	}
	tr, ok := w.(targetReader)
	if !ok {
		return nil, fmt.Errorf("verify: %s: %w", reasonTargetNotReadable, errNoTargetReader)
	}

	s := &state{
		opts: v.opts, src: src, target: tr, schema: schema, plan: plan,
		cls: cls, res: res, lr: lr,
		tables: make(map[ref.TableRef]*pipeline.Table, len(schema.Tables)),
		rows:   map[ref.TableRef]int64{},
	}
	for i := range schema.Tables {
		s.tables[schema.Tables[i].Ref] = &schema.Tables[i]
	}
	s.steps = loadedSteps(plan, s.tables)
	defer s.closeShort(ctx)

	// Section 6's order: the residual scan and the second net first, then item
	// 5's checks. A leak is the refusal that matters most, so it is the one the
	// exit code names when more than one check failed.
	if err := s.residualScan(ctx); err != nil {
		return nil, err
	}
	if err := s.secondNet(ctx); err != nil {
		return nil, err
	}
	// The catalog pass is beside the second net and for the same reason: the
	// second net looks at every value the target holds in a row, and this looks
	// at every literal it holds in its *schema* (catalog.go, T-0134). A row scan
	// cannot see a default, and a default is a value the application's next
	// INSERT writes into a row.
	if err := s.catalog(ctx); err != nil {
		return nil, err
	}
	if err := s.foreignKeys(ctx); err != nil {
		return nil, err
	}
	if err := s.rowCounts(ctx); err != nil {
		return nil, err
	}
	if err := s.sequences(ctx); err != nil {
		return nil, err
	}
	if err := s.samples(ctx); err != nil {
		return nil, err
	}

	report := &pipeline.Report{
		Checks:      s.checks,
		Rows:        s.rows,
		Elapsed:     time.Since(started),
		Unconfirmed: s.unconfirmed,
		Probes:      s.probes,
	}
	refusal := s.firstFailure()
	if refusal == nil {
		return report, nil
	}
	report.ExitCode = refusal.Exit
	return report, refusal
}

// errNoTargetReader is the wiring failure the package comment describes. It is
// its own error so that a caller can tell "this build cannot verify" from "this
// target failed a check".
var errNoTargetReader = errors.New(
	"the pipeline.Writer handed to verify has no Query method, and every check in " +
		"ARCHITECTURE.md section 6 is a read of the target")

// firstFailure is the failing check the exit code comes from: the first one in
// section 6's order.
func (s *state) firstFailure() *Refusal {
	for _, name := range order {
		for _, r := range s.failures {
			if r.Check == name {
				return r
			}
		}
	}
	return nil
}

// fail records a failing check. It never stops the run: the report names every
// column that is wrong, not the first one.
func (s *state) fail(r *Refusal) {
	s.failures = append(s.failures, r)
	s.checks = append(s.checks, r.check())
}

// pass records a check that found nothing, with the number of things it looked
// at.
func (s *state) pass(name string, code event.Code, count int64) {
	s.checks = append(s.checks, pipeline.Check{
		Name: name, Passed: true, Code: code, Count: count,
	})
}

// report records something section 6 requires to be reported rather than
// failed.
func (s *state) report(name string, code event.Code, table ref.TableRef, column string, count int64) {
	s.checks = append(s.checks, pipeline.Check{
		Name: name, Passed: true, Code: code, Table: table, Column: column, Count: count,
	})
}

// loadedSteps is every step of the plan over a table the loader recreated. A
// partition leaf is not one: a partitioned source table becomes one plain table
// in the target (ARCHITECTURE.md section 11.1), so its leaves are steps with no
// relation of their own on the other side.
//
// A schema-only step stays in: its table exists in the target and is empty, and
// every check below reads that as the empty answer — no masked value to scan,
// no column with three values to run a validator over, no key to sample by, and
// a sequence reset over a column with no rows, which is the strict-NULL form's
// other half.
func loadedSteps(plan *pipeline.Plan, tables map[ref.TableRef]*pipeline.Table) []pipeline.Step {
	out := make([]pipeline.Step, 0, len(plan.Steps))
	for _, step := range plan.Steps {
		t, ok := tables[step.Table]
		if !ok || t.Parent != nil {
			continue
		}
		out = append(out, step)
	}
	return out
}

// shortReader opens the second transaction of section 6 item 3, or returns the
// error that stopped it. The error is remembered: a source that would not open
// once will not open per hit, and retrying it would be a probe storm against a
// database that is already refusing.
func (s *state) shortReader(ctx context.Context) (pipeline.Reader, error) {
	if s.shortOpen {
		return s.short, s.shortErr
	}
	s.shortOpen = true
	if s.src == nil {
		s.shortErr = errors.New("verify: no source to confirm against")
		return nil, s.shortErr
	}
	r, err := s.src.Short(ctx)
	if err != nil {
		s.shortErr = err
		return nil, err
	}
	s.short = r
	return r, nil
}

func (s *state) closeShort(ctx context.Context) {
	if s.short == nil {
		return
	}
	// The context this stage ran under may already be done; closing the
	// transaction still has to happen, and it is a rollback of a read.
	_ = s.short.Close(context.WithoutCancel(ctx))
	s.short = nil
}
