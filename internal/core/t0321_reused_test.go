// SPDX-License-Identifier: Apache-2.0

package core

import (
	"strconv"
	"testing"

	"github.com/Liarea/lazyslice/internal/classify"
	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// T-0321 review round: finding 1 (a re-run from a committed yml dropped every
// per-column decision event, emptying the --json stream and the TUI's reasons
// screen) and finding 2 (the reused line never named a column whose decision
// this run's re-derivation actually changed). Both are held here, over
// classifyStage on a hand-built schema, the way mask_test.go already tests
// classifyStage without a database.

var reusedPeople = ref.TableRef{Schema: "public", Name: "t321_people"}

func reusedSchema() *pipeline.Schema {
	return &pipeline.Schema{Tables: []pipeline.Table{{
		Ref: reusedPeople,
		Columns: []pipeline.Column{
			{Name: "id", TypeName: "bigint", TypeOID: 20},
			{Name: "email", TypeName: "text", Nullable: true},
			{Name: "status", TypeName: "text", Nullable: true},
		},
		PK: []string{"id"},
	}}}
}

func reusedRun(prior *pipeline.Config) (*run, *eventCollector) {
	sink := &eventCollector{}
	return &run{req: normalise(Request{}), sink: sink, schema: reusedSchema(), prior: prior}, sink
}

func decisionEvents(events []event.Event) []event.Event {
	var out []event.Event
	for _, e := range events {
		if e.Code == classify.CodeColumnMasked || e.Code == classify.CodeColumnCopied {
			out = append(out, e)
		}
	}
	return out
}

// A fresh run (no committed yml) gets one per-column decision line for every
// column, none of them settled, closed by classify.summary with the right
// counts.
func TestClassifyStageFreshRunSendsPerColumnLinesAndSummary(t *testing.T) {
	t.Parallel()
	r, sink := reusedRun(nil)
	if err := r.classifyStage(); err != nil {
		t.Fatalf("classifyStage: %v", err)
	}

	lines := decisionEvents(sink.events)
	if len(lines) != 3 {
		t.Fatalf("got %d per-column decision events, want 3 (one per column)", len(lines))
	}
	for _, e := range lines {
		if e.Settled {
			t.Errorf("%s.%s: Settled = true on a fresh run with nothing to reuse", e.Table, e.Column)
		}
	}

	var summary *event.Event
	for i, e := range sink.events {
		if e.Code == CodeClassifySummary {
			summary = &sink.events[i]
		}
		if e.Code == CodeClassifyReused {
			t.Error("a fresh run sent classify.reused, which only a run with a prior should send")
		}
	}
	if summary == nil {
		t.Fatal("no classify.summary event sent")
	}
	if summary.Args[event.ArgColumnCount] != "3" {
		t.Errorf("classify.summary column_count = %q, want 3", summary.Args[event.ArgColumnCount])
	}
	wantMasked := strconv.Itoa(1) // email is the one column classified as personal data by name
	if summary.Args[event.ArgMaskedCount] != wantMasked {
		t.Errorf("classify.summary masked_count = %q, want %s", summary.Args[event.ArgMaskedCount], wantMasked)
	}
}

// priorFromDecisions mimics internal/emit's columnConfig closely enough for
// applyPrior to read back "no change": a masked column carries a Masker and an
// unmasked one does not (the same rule this package's decisionChanged reads
// this state through).
func priorFromDecisions(cls *pipeline.Classification) *pipeline.Config {
	cols := map[ref.ColumnRef]pipeline.ColumnConfig{}
	for col, d := range cls.Decisions {
		cc := pipeline.ColumnConfig{Category: d.Category, Confidence: d.Confidence, TypeFP: d.TypeFP}
		if d.Masked {
			cc.Masker = d.Masker
		}
		cols[col] = cc
	}
	return &pipeline.Config{Columns: cols}
}

// A run with a prior that already agrees with every decision this run reaches
// sends no unsettled per-column line, no drift line, and one classify.reused
// line whose counts are all accounted for (T-0321 review finding 1 and 2).
func TestClassifyStageWithAgreeingPriorSendsSettledLinesAndReusedLine(t *testing.T) {
	t.Parallel()
	first, _ := reusedRun(nil)
	if err := first.classifyStage(); err != nil {
		t.Fatalf("first classifyStage: %v", err)
	}
	prior := priorFromDecisions(first.cls)

	r, sink := reusedRun(prior)
	if err := r.classifyStage(); err != nil {
		t.Fatalf("second classifyStage: %v", err)
	}

	lines := decisionEvents(sink.events)
	if len(lines) != 3 {
		t.Fatalf("got %d per-column decision events, want 3 (sent to the sink for every column, per finding 1)", len(lines))
	}
	for _, e := range lines {
		if !e.Settled {
			t.Errorf("%s.%s: Settled = false, want true — the prior already agreed with this decision", e.Table, e.Column)
		}
	}
	for _, e := range sink.events {
		if e.Code == classify.CodeColumnDrift {
			t.Errorf("unexpected drift event for %s.%s: every column was in the prior", e.Table, e.Column)
		}
		if e.Code == CodeClassifySummary {
			t.Error("a run with a prior sent classify.summary, which only a fresh run should send")
		}
	}

	var reused *event.Event
	for i, e := range sink.events {
		if e.Code == CodeClassifyReused {
			reused = &sink.events[i]
		}
	}
	if reused == nil {
		t.Fatal("no classify.reused event sent")
	}
	if reused.Args[event.ArgCount] != "3" {
		t.Errorf("classify.reused count = %q, want 3", reused.Args[event.ArgCount])
	}
	if reused.Args[event.ArgDriftCount] != "0" {
		t.Errorf("classify.reused drift_count = %q, want 0", reused.Args[event.ArgDriftCount])
	}
	if reused.Args[event.ArgChangedCount] != "0" {
		t.Errorf("classify.reused changed_count = %q, want 0", reused.Args[event.ArgChangedCount])
	}
}

// A column the prior never saw is drift: it gets the classify.column.drift
// warning and its own per-column decision line, unsettled — a drift column is
// never counted as changed, because it was never settled to begin with.
func TestClassifyStageWithPartialPriorReportsDriftSeparatelyFromChanged(t *testing.T) {
	t.Parallel()
	first, _ := reusedRun(nil)
	if err := first.classifyStage(); err != nil {
		t.Fatalf("first classifyStage: %v", err)
	}
	prior := priorFromDecisions(first.cls)
	delete(prior.Columns, ref.ColumnRef{Table: reusedPeople, Column: "status"})

	r, sink := reusedRun(prior)
	if err := r.classifyStage(); err != nil {
		t.Fatalf("second classifyStage: %v", err)
	}

	var driftCol string
	for _, e := range sink.events {
		if e.Code == classify.CodeColumnDrift {
			driftCol = e.Column
		}
	}
	if driftCol != "status" {
		t.Fatalf("drift column = %q, want status", driftCol)
	}

	for _, e := range decisionEvents(sink.events) {
		if e.Column == "status" && e.Settled {
			t.Error("status: Settled = true, want false — it drifted, so it was never settled")
		}
	}

	var reused *event.Event
	for i, e := range sink.events {
		if e.Code == CodeClassifyReused {
			reused = &sink.events[i]
		}
	}
	if reused == nil {
		t.Fatal("no classify.reused event sent")
	}
	if reused.Args[event.ArgDriftCount] != "1" {
		t.Errorf("classify.reused drift_count = %q, want 1", reused.Args[event.ArgDriftCount])
	}
	if reused.Args[event.ArgChangedCount] != "0" {
		t.Errorf("classify.reused changed_count = %q, want 0 — a drifted column is not a changed one", reused.Args[event.ArgChangedCount])
	}
}

// A column the prior recorded as copied that this run masks instead (a new
// --mask flag, standing in for new sampled evidence or a new pattern per the
// review finding) is unsettled and counted in changed_count, and its own
// per-column line still carries the reason (T-0321 review finding 2).
func TestClassifyStageCountsAColumnThatChangedSinceThePrior(t *testing.T) {
	t.Parallel()
	col := ref.ColumnRef{Table: reusedPeople, Column: "status"}
	first, _ := reusedRun(nil)
	if err := first.classifyStage(); err != nil {
		t.Fatalf("first classifyStage: %v", err)
	}
	if first.cls.Decisions[col].Masked {
		t.Fatalf("precondition: %s is masked with no flag, so changing it here proves nothing", col)
	}
	prior := priorFromDecisions(first.cls)

	sink := &eventCollector{}
	r := &run{
		req:  normalise(Request{Mask: map[string]string{"t321_people.status": ""}}),
		sink: sink, schema: reusedSchema(), prior: prior,
	}
	if err := r.classifyStage(); err != nil {
		t.Fatalf("classifyStage with --mask: %v", err)
	}
	if !r.cls.Decisions[col].Masked {
		t.Fatalf("%s is still copied after --mask", col)
	}

	var changedEvent *event.Event
	for i, e := range decisionEvents(sink.events) {
		if e.Column == "status" {
			changedEvent = &decisionEvents(sink.events)[i]
		}
	}
	if changedEvent == nil {
		t.Fatal("no per-column decision event for status")
	}
	if changedEvent.Settled {
		t.Error("status: Settled = true, want false — the prior recorded it as copied and this run masks it")
	}
	if changedEvent.Code != classify.CodeColumnMasked {
		t.Errorf("status decision code = %q, want %q", changedEvent.Code, classify.CodeColumnMasked)
	}

	var reused *event.Event
	for i, e := range sink.events {
		if e.Code == CodeClassifyReused {
			reused = &sink.events[i]
		}
	}
	if reused == nil {
		t.Fatal("no classify.reused event sent")
	}
	if reused.Args[event.ArgChangedCount] != "1" {
		t.Errorf("classify.reused changed_count = %q, want 1", reused.Args[event.ArgChangedCount])
	}
	if reused.Args[event.ArgDriftCount] != "0" {
		t.Errorf("classify.reused drift_count = %q, want 0 — status was in the prior, just disagreed with", reused.Args[event.ArgDriftCount])
	}
}

// planSummary counts every non-schema_only step as reached and every
// schema_only step as unreachable, and carries the plan's own row estimate —
// T-0321's other summary line, previously untested (review finding 3).
func TestPlanSummaryCountsReachedAndUnreachableTables(t *testing.T) {
	t.Parallel()
	p := &pipeline.Plan{
		Steps: []pipeline.Step{
			{Table: ref.TableRef{Schema: "public", Name: "a"}, Mode: pipeline.ChildOK},
			{Table: ref.TableRef{Schema: "public", Name: "b"}, Mode: pipeline.ParentOnly},
			{Table: ref.TableRef{Schema: "public", Name: "c"}, Mode: pipeline.Lookup},
			{Table: ref.TableRef{Schema: "public", Name: "d"}, Mode: pipeline.SchemaOnly},
			{Table: ref.TableRef{Schema: "public", Name: "e"}, Mode: pipeline.SchemaOnly},
		},
		Estimate: pipeline.Estimate{Rows: 4321},
	}

	var got event.Event
	r := &run{sink: event.SinkFunc(func(e event.Event) { got = e })}
	r.planSummary(p)

	if got.Code != CodePlanSummary {
		t.Fatalf("code = %q, want %q", got.Code, CodePlanSummary)
	}
	if got.Args[event.ArgTableCount] != "3" {
		t.Errorf("table_count (reached) = %q, want 3", got.Args[event.ArgTableCount])
	}
	if got.Args[event.ArgUnreachableCount] != "2" {
		t.Errorf("unreachable_count = %q, want 2", got.Args[event.ArgUnreachableCount])
	}
	if got.Args[event.ArgRowCount] != "4321" {
		t.Errorf("row_count = %q, want 4321 (Estimate.Rows)", got.Args[event.ArgRowCount])
	}
}
