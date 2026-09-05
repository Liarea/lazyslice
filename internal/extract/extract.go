// Package extract streams the planned rows out of the source snapshot into a
// channel of pipeline.RowBatch.
//
// One table at a time, in plan order, in chunks of 2,000 keys joined through a
// typed unnest. Tables are strictly sequential on the channel and every table
// ends with a batch carrying Last, including a table with no rows, so the loader
// always sees a boundary rather than inferring one.
//
// Everything here runs inside the run's REPEATABLE READ READ ONLY snapshot and
// through the shape allowlist, so extract holds the snapshot open on production
// and nothing else does. The plan printed the hold estimate before this started.
//
// Scaffold status: no-op. Extract closes out and returns
// pipeline.ErrNotImplemented.
package extract

import (
	"context"
	"fmt"

	"github.com/Liarea/lazyslice/internal/pipeline"
)

type extractor struct{}

// New returns the streaming extractor.
func New() pipeline.Extractor { return extractor{} }

var _ pipeline.Extractor = extractor{}

func (extractor) Extract(_ context.Context, _ pipeline.Reader, _ *pipeline.Plan, out chan<- pipeline.RowBatch) error {
	close(out)
	return fmt.Errorf("extract: %w", pipeline.ErrNotImplemented)
}
