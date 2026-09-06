// SPDX-License-Identifier: Apache-2.0

// Package transform applies the classification to every batch on its way from
// extract to load, and records what it masked in the residual filter.
//
// Transform is pure apart from Residual.Add: same key, same classification, same
// input, same output, which is what invariant I3 (two runs, byte-identical
// targets) rests on.
//
// It masks in place. There is no pass-through mode and no flag that turns
// masking off wholesale; CONCEPT.md refuses one and CLAUDE.md forbids adding
// one.
//
// Scaffold status: no-op. Transform returns pipeline.ErrNotImplemented.
package transform

import (
	"fmt"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/mask"
)

type transformer struct{}

// New returns the masking transformer.
func New() pipeline.Transformer { return transformer{} }

var _ pipeline.Transformer = transformer{}

func (transformer) Transform(
	b pipeline.RowBatch,
	_ *pipeline.Classification,
	_ *mask.Key,
	_ pipeline.Residual,
) (pipeline.RowBatch, error) {
	return b, fmt.Errorf("transform: %w", pipeline.ErrNotImplemented)
}
