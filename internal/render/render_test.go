// SPDX-License-Identifier: Apache-2.0

package render

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/ref"
)

// Lines renders from the catalogue and nowhere else: the substitution has to
// put the event's identifiers into the row's own template.
func TestLinesRendersFromTheCatalogue(t *testing.T) {
	var b bytes.Buffer
	l := NewLines(&b)
	l.Send(event.Event{
		At:    time.Now(),
		Stage: event.Load,
		Kind:  event.Info,
		Code:  "load.target.dropping",
		Table: ref.TableRef{Schema: "public", Name: "customer"},
	})

	got := b.String()
	if !strings.Contains(got, "dropping public.customer in the target") {
		t.Errorf("Lines wrote %q, want the load.target.dropping message with the table substituted", got)
	}
}

// A stage bracket prints nothing. It is the pipeline's own structure and not a
// fact the transcript carries; --json still gets it.
func TestLinesSkipsStageBrackets(t *testing.T) {
	var b bytes.Buffer
	l := NewLines(&b)
	l.Send(event.Event{Stage: event.Plan, Kind: event.StageStart, Code: event.CodeStageStart})
	l.Send(event.Event{Stage: event.Plan, Kind: event.StageDone, Code: event.CodeStageDone})

	if b.Len() != 0 {
		t.Errorf("Lines wrote %q for the stage brackets, want nothing", b.String())
	}
}

// A settled Decision (T-0321) prints nothing: it is a column whose verdict
// this run reached is the one the committed yml already recorded, and
// classify.reused is what speaks for it on the human transcript. An unsettled
// one — Settled left false, the zero value, exactly like every Decision before
// T-0321 — still prints in full, reason and all.
func TestLinesSkipsASettledDecisionButPrintsAnUnsettledOne(t *testing.T) {
	var b bytes.Buffer
	l := NewLines(&b)
	l.Send(event.Event{
		Stage: event.Classify, Kind: event.Decision, Code: "classify.masked.column",
		Table: ref.TableRef{Schema: "public", Name: "customer"}, Column: "email",
		Settled: true,
		Args:    event.Args{event.ArgTable: "public.customer", event.ArgColumn: "email", event.ArgReason: "name matches email"},
	})
	if b.Len() != 0 {
		t.Errorf("Lines wrote %q for a settled decision, want nothing", b.String())
	}

	l.Send(event.Event{
		Stage: event.Classify, Kind: event.Decision, Code: "classify.masked.column",
		Table: ref.TableRef{Schema: "public", Name: "customer"}, Column: "phone",
		Args: event.Args{event.ArgTable: "public.customer", event.ArgColumn: "phone", event.ArgReason: "name matches phone"},
	})
	if !strings.Contains(b.String(), "customer.phone") {
		t.Errorf("Lines wrote %q, want the unsettled decision for customer.phone", b.String())
	}
}

// A code with no catalogue row is a bug in the tree. The line has to name it
// rather than print nothing, because a stage that emits an unknown code would
// otherwise be silent exactly where it had something to say.
func TestLinesNamesAMissingCode(t *testing.T) {
	var b bytes.Buffer
	NewLines(&b).Send(event.Event{Kind: event.Warn, Code: "nobody.wrote.this"})

	if !strings.Contains(b.String(), "nobody.wrote.this") {
		t.Errorf("Lines wrote %q, want it to name the missing code", b.String())
	}
}

// --json is a machine interface: one JSON object per line, with every field the
// event carries.
func TestNDJSONWritesTheEventVerbatim(t *testing.T) {
	var b bytes.Buffer
	n := NewNDJSON(&b)
	n.Send(event.Event{
		Stage:  event.Classify,
		Kind:   event.Decision,
		Code:   "classify.masked.column",
		Table:  ref.TableRef{Schema: "public", Name: "customer"},
		Column: "email",
		Args:   event.Args{event.ArgReason: "name matches email"},
	})
	n.Send(event.Event{Stage: event.Emit, Kind: event.Info, Code: "emit.config.written"})

	lines := strings.Split(strings.TrimSpace(b.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("NDJSON wrote %d line(s), want 2:\n%s", len(lines), b.String())
	}

	var first map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("line 1 is not JSON: %v\n%s", err, lines[0])
	}
	if first["Column"] != "email" {
		t.Errorf("Column = %v, want email", first["Column"])
	}
	table, ok := first["Table"].(map[string]any)
	if !ok || table["Schema"] != "public" || table["Name"] != "customer" {
		t.Errorf("Table = %v, want {public customer}", first["Table"])
	}
}
