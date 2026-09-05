// Package plan computes the subset: which rows of which tables the snapshot
// will hold, and why each one is there (ARCHITECTURE.md section 3).
//
// It is a client-side monotone worklist with provenance tags, never SQL
// pushdown. Pushdown would be faster and would cost the two things that matter:
// the plan could not say why a row is present, and the source would have to
// accept a temp table from us.
//
// Determinism is a property, not an accident. The queue is FIFO; tables iterate
// in (schema, name) order; edges iterate in constraint-name order; every key
// set, chunk and SQL result is ordered by the identity columns; nothing iterates
// a Go map. Two runs over one snapshot therefore produce byte-identical
// selected sets, which is what makes lazyslice.yml a record of what happened.
//
// A row's mode is decided once, when it is first popped. A table reached only as
// a parent does not have its own children pulled through, which is both the size
// control and the privacy control.
//
// Scaffold status: no-op. Plan returns pipeline.ErrNotImplemented.
package plan

import (
	"context"
	"fmt"

	"github.com/Liarea/lazyslice/internal/pipeline"
)

type planner struct{}

// New returns the subset planner.
func New() pipeline.Planner { return planner{} }

var _ pipeline.Planner = planner{}

func (planner) Plan(
	_ context.Context,
	_ pipeline.Reader,
	_ *pipeline.Schema,
	_ *pipeline.Classification,
	_ pipeline.PlanRequest,
) (*pipeline.Plan, error) {
	return nil, fmt.Errorf("plan: %w", pipeline.ErrNotImplemented)
}
