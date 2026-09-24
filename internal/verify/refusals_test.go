// SPDX-License-Identifier: Apache-2.0

package verify

import (
	"context"
	"errors"
	"testing"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// TestEverySecondNetColumnIsReportedInOneRun is T-0319's verify half. Dogfood
// session 1 met verify.refused.second_net on one copied column per run, three
// runs in a row, each found only after the last was fixed. The net already
// scans every unmasked column; what the operator needs is every failure it
// found, in section 6's order, with the one the exit code comes from first and
// still reachable as a lone *Refusal.
func TestEverySecondNetColumnIsReportedInOneRun(t *testing.T) {
	table := ref.TableRef{Schema: "public", Name: "t319_feeds"}
	columns := []string{"cached_body", "payload", "interaction_ref"}
	decisions := map[ref.ColumnRef]pipeline.Decision{}
	cols := make([]pipeline.Column, 0, len(columns))
	for _, name := range columns {
		col := ref.ColumnRef{Table: table, Column: name}
		decisions[col] = pipeline.Decision{Col: col, Category: pipeline.CatNone, Source: pipeline.ByClassifier}
		cols = append(cols, pipeline.Column{Name: name, TypeName: "text"})
	}
	s := &state{
		schema: &pipeline.Schema{},
		// Every column reads the same rows: one synthetic address among
		// ordinary words, which the strong email entry refuses on one hit.
		target: oneColumn{vals: []any{"alpha", "bravo", "charlie", "t319.reader@example.org"}},
		steps:  []pipeline.Step{{Table: table, Mode: pipeline.ChildOK}},
		tables: map[ref.TableRef]*pipeline.Table{table: {Ref: table, Columns: cols}},
		cls:    &pipeline.Classification{Decisions: decisions},
	}
	// A failure of a later check recorded first: the order is section 6's,
	// not the order the checks happened to be appended in.
	s.fail(&Refusal{Code: CodeRefusedFK, Exit: exitFK, Check: checkFK, Table: table})
	if err := s.secondNet(context.Background()); err != nil {
		t.Fatalf("secondNet: %v", err)
	}

	got := s.orderedFailures()
	if len(got) != len(columns)+1 {
		t.Fatalf("recorded %d failures (%v), want one per column plus the fk one", len(got), got)
	}
	for i, name := range columns {
		if got[i].Check != checkSecondNet || got[i].Column != name {
			t.Errorf("failure %d is %s on %s, want second_net on %s", i, got[i].Check, got[i].Column, name)
		}
	}
	if last := got[len(got)-1]; last.Check != checkFK {
		t.Errorf("the last failure is %s, want fk after every second_net one", last.Check)
	}
	if got[0] != s.firstFailure() {
		t.Errorf("the first of the list is %v and the exit code's is %v", got[0], s.firstFailure())
	}

	var err error = got
	var first *Refusal
	if !errors.As(err, &first) || first != got[0] {
		t.Fatalf("errors.As found %v in Refusals, want its first member %v", first, got[0])
	}
}
