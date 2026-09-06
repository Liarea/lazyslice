// SPDX-License-Identifier: Apache-2.0

// Package introspect reads the source catalog into a pipeline.Schema.
//
// Definition text comes from the catalog's own deparser (pg_get_expr,
// pg_get_constraintdef, pg_get_indexdef), never from a printer of ours, so the
// target receives the source's expression verbatim and a schema we cannot
// recreate is a refusal at plan rather than a subtly different table.
//
// Sampling happens here too: up to 200 rows per table by TABLESAMPLE SYSTEM
// with a fixed seed, never LIMIT, because LIMIT samples the front of the table.
// TABLESAMPLE is not accepted on a partitioned table, so a partitioned root is
// sampled through its largest leaf and Table.SampledFrom names it.
//
// Table.Samples holds production values and is never serialised.
//
// Scaffold status: no-op. Introspect returns pipeline.ErrNotImplemented.
package introspect

import (
	"context"
	"fmt"

	"github.com/Liarea/lazyslice/internal/pipeline"
)

type introspector struct{}

// New returns the catalog introspector.
func New() pipeline.Introspector { return introspector{} }

var _ pipeline.Introspector = introspector{}

func (introspector) Introspect(_ context.Context, _ pipeline.Reader) (*pipeline.Schema, error) {
	return nil, fmt.Errorf("introspect: %w", pipeline.ErrNotImplemented)
}
