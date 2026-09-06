// SPDX-License-Identifier: Apache-2.0

// Package verify proves what the run did, and is the only place that says the
// green tick is earned.
//
// Five things happen here (ARCHITECTURE.md section 6): every foreign key
// validates; the residual filter is tested against every masked column of the
// target and each hit confirmed against the source in a fresh short
// transaction; the classifier's own validators re-run over the full contents of
// every column that is not fully masked (the second net); sequences and row
// counts are checked; unmasked columns are compared byte for byte against the
// source on a sample.
//
// Verify fails closed. A residual hit that cannot be tested, because the source
// will not open, a probe errors, the role lacks SELECT or the probe cap is
// reached, is exit 9 with the reason, never a printed note: an unconfirmable hit
// is the case where failing closed costs least.
//
// It never uses the run snapshot, which has been released by the time it starts.
//
// Scaffold status: no-op. Verify returns pipeline.ErrNotImplemented.
package verify

import (
	"context"
	"fmt"

	"github.com/Liarea/lazyslice/internal/pipeline"
)

type verifier struct{}

// New returns the verifier.
func New() pipeline.Verifier { return verifier{} }

var _ pipeline.Verifier = verifier{}

func (verifier) Verify(
	_ context.Context,
	_ pipeline.Source,
	_ pipeline.Writer,
	_ *pipeline.Schema,
	_ *pipeline.Plan,
	_ *pipeline.Classification,
	_ pipeline.Residual,
	_ *pipeline.LoadResult,
) (*pipeline.Report, error) {
	return nil, fmt.Errorf("verify: %w", pipeline.ErrNotImplemented)
}
