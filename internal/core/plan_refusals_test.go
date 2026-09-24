// SPDX-License-Identifier: Apache-2.0

package core

import (
	"errors"
	"testing"

	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/plan"
	"github.com/Liarea/lazyslice/internal/ref"
)

// reportPlanRefusals, held without a database (T-0318).
//
// internal/plan/refusals_test.go pins the collection itself -- that
// resolveIdentities, checkWriteBack, applySkipAndPrivileges and
// checkUniqueDomain gather every no_identity, unwritable, skip_parent and
// unique_domain/equality_group refusal into one plan.Refusals instead of
// stopping at the first. This file is the other half: what core does with
// that collection once planStage gets it back -- one Error event per member,
// in order, and a *Stop that answers for the process's exit code with the
// first member's Code and Exit, sent so run.report's own single-event send
// does not print the first refusal a second time.
func planRefusals() plan.Refusals {
	t1 := ref.TableRef{Schema: "public", Name: "line_items_products"}
	t2 := ref.TableRef{Schema: "public", Name: "line_items_variants"}
	t3 := ref.TableRef{Schema: "public", Name: "credential_tokens"}
	return plan.Refusals{
		{
			Code: plan.CodeNoIdentity, Exit: 12, Table: t1,
			Args:    event.Args{event.ArgTable: t1.String(), event.ArgFlag: "--key " + t1.String() + "=order_id,product_id or --skip-table " + t1.String()},
			Message: t1.String() + " has no row identity: pass --key " + t1.String() + "=order_id,product_id or --skip-table " + t1.String(),
		},
		{
			Code: plan.CodeNoIdentity, Exit: 12, Table: t2,
			Args:    event.Args{event.ArgTable: t2.String(), event.ArgFlag: "--key " + t2.String() + "=order_id,variant_id or --skip-table " + t2.String()},
			Message: t2.String() + " has no row identity: pass --key " + t2.String() + "=order_id,variant_id or --skip-table " + t2.String(),
		},
		{
			Code: plan.CodeUniqueDomain, Exit: 12, Table: t3, Column: "handle",
			Args:    event.Args{event.ArgTable: t3.String(), event.ArgColumn: "handle"},
			Message: t3.String() + ".handle is under a unique index: no masker fits",
		},
	}
}

func TestReportPlanRefusalsSendsOneEventPerRefusalInOrder(t *testing.T) {
	t.Parallel()
	refusals := planRefusals()
	sink := &eventCollector{}
	r := &run{sink: sink}

	err := r.reportPlanRefusals(refusals)

	if len(sink.events) != len(refusals) {
		t.Fatalf("sink got %d events, want one per refusal (%d)", len(sink.events), len(refusals))
	}
	for i, e := range sink.events {
		want := refusals[i]
		if e.Kind != event.Error {
			t.Errorf("event %d Kind = %v, want event.Error", i, e.Kind)
		}
		if e.Code != want.Code {
			t.Errorf("event %d Code = %q, want %q", i, e.Code, want.Code)
		}
		if e.Exit != want.Exit {
			t.Errorf("event %d Exit = %d, want %d", i, e.Exit, want.Exit)
		}
		if e.Table != want.Table {
			t.Errorf("event %d Table = %s, want %s", i, e.Table, want.Table)
		}
		if e.Column != want.Column {
			t.Errorf("event %d Column = %q, want %q", i, e.Column, want.Column)
		}
	}

	var stop *Stop
	if !errors.As(err, &stop) {
		t.Fatalf("reportPlanRefusals returned %v, want a *core.Stop", err)
	}
	first := refusals[0]
	if stop.Code != first.Code || stop.Exit != first.Exit {
		t.Errorf("stop = %s/exit %d, want the first refusal's %s/exit %d",
			stop.Code, stop.Exit, first.Code, first.Exit)
	}
	if stop.Table != first.Table || stop.Message != first.Message {
		t.Errorf("stop does not carry the first refusal's table and message")
	}
	if !stop.sent {
		t.Fatal("stop.sent = false: run.report would send the first refusal's Error event a second time")
	}
}

// TestReportPlanRefusalsStopIsNotSentTwice is the consequence
// TestALadderRefusalIsSentOnceAndNotAgainByReport already pins for the
// discovery ladder's own refusal: a *Stop whose Error event this function
// already sent must not reach the sink a second time through run.report.
func TestReportPlanRefusalsStopIsNotSentTwice(t *testing.T) {
	t.Parallel()
	refusals := planRefusals()
	sink := &eventCollector{}
	r := &run{sink: sink}

	err := r.reportPlanRefusals(refusals)
	before := len(sink.events)
	r.report(err)
	if len(sink.events) != before {
		t.Fatalf("run.report sent %d more event(s) after reportPlanRefusals; the first refusal was sent twice",
			len(sink.events)-before)
	}
}
