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
// Scaffold status: no-op. Load returns pipeline.ErrNotImplemented.
package load

import (
	"context"
	"fmt"

	"github.com/Liarea/lazyslice/internal/pipeline"
)

type loader struct{}

// New returns the target loader.
func New() pipeline.Loader { return loader{} }

var _ pipeline.Loader = loader{}

func (loader) Load(
	_ context.Context,
	_ pipeline.Writer,
	_ *pipeline.Plan,
	_ *pipeline.Schema,
	_ <-chan pipeline.RowBatch,
) (*pipeline.LoadResult, error) {
	return nil, fmt.Errorf("load: %w", pipeline.ErrNotImplemented)
}
