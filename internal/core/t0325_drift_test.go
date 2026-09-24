// SPDX-License-Identifier: Apache-2.0

package core

import (
	"testing"

	"github.com/Liarea/lazyslice/internal/classify"
	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/ref"
)

// T-0325: dogfood session 1 ran --unmask flags with no ./lazyslice.yml in the
// directory and got classify.column.drift for every column the flags did not
// name — 1,835 lines, none of them printed on the identical run with no
// --unmask flag. buildPrior folds a --unmask flag into a non-nil in-memory
// pipeline.Config even when r.prior (the committed file) is nil, so that the
// flag's opt-out reaches this run's classification; cls.Drift is computed
// against that Config's Columns map, which classifyStage's own drift loop
// used to read with no regard for whether it came from a file on disk. A
// first run — no committed yml, whatever flags it carries — must report no
// drift, because there is no file for a column to be missing from.
func TestUnmaskFlagWithNoCommittedFileSendsNoDriftWarnings(t *testing.T) {
	t.Parallel()
	r, sink := reusedRun(nil)
	r.req = normalise(Request{Unmask: map[string]string{
		"t321_people.email": "test fixture, not a real person",
	}})

	if err := r.classifyStage(); err != nil {
		t.Fatalf("classifyStage with --unmask and no committed yml: %v", err)
	}

	for _, e := range sink.events {
		if e.Code == classify.CodeColumnDrift {
			t.Errorf("unexpected classify.column.drift for %s.%s: no committed yml exists for anything to drift from",
				e.Table, e.Column)
		}
	}
	var summary bool
	for _, e := range sink.events {
		if e.Code == CodeClassifySummary {
			summary = true
		}
		if e.Code == CodeClassifyReused {
			t.Error("a run with no committed yml sent classify.reused, which only a run with a prior should send")
		}
	}
	if !summary {
		t.Error("a run with no committed yml did not send classify.summary")
	}
}

// The drift warning names what this run actually did with the column —
// "copied", or "masked as CATEGORY" — rather than the drift rule's fixed
// description of what drift means in general, which used to print "masked at
// or above possible" for a column this run had in fact left unmasked
// (dogfood session 1's boolean columns).
func TestDriftWarningNamesTheActualVerdict(t *testing.T) {
	t.Parallel()
	first, _ := reusedRun(nil)
	if err := first.classifyStage(); err != nil {
		t.Fatalf("first classifyStage: %v", err)
	}
	emailCol := ref.ColumnRef{Table: reusedPeople, Column: "email"}
	statusCol := ref.ColumnRef{Table: reusedPeople, Column: "status"}
	if !first.cls.Decisions[emailCol].Masked {
		t.Fatalf("precondition: email is not masked, so this test proves nothing about the masked branch")
	}
	if first.cls.Decisions[statusCol].Masked {
		t.Fatalf("precondition: status is masked, so this test proves nothing about the copied branch")
	}

	prior := priorFromDecisions(first.cls)
	delete(prior.Columns, emailCol)
	delete(prior.Columns, statusCol)

	r, sink := reusedRun(prior)
	if err := r.classifyStage(); err != nil {
		t.Fatalf("second classifyStage: %v", err)
	}

	verdicts := map[string]string{}
	for _, e := range sink.events {
		if e.Code == classify.CodeColumnDrift {
			verdicts[e.Column] = e.Args[event.ArgVerdict]
		}
	}
	if got, want := verdicts["email"], "masked as email"; got != want {
		t.Errorf("email drift verdict = %q, want %q", got, want)
	}
	if got, want := verdicts["status"], "copied"; got != want {
		t.Errorf("status drift verdict = %q, want %q", got, want)
	}
}
