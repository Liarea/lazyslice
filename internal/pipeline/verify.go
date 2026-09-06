// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"context"
	"time"

	"github.com/Liarea/lazyslice/internal/event"
)

// Check is one verification result. Names are fixed: "fk", "residual",
// "residual_unconfirmable", "second_net", "sequences", "row_count",
// "unmasked_identical".
type Check struct {
	Name   string
	Passed bool
	Code   event.Code
	Table  TableRef
	Column string
	Count  int64
}

// Report is the run's outcome, and the thing the exit code is computed from.
// It is serialised by --json, so it holds no value.
type Report struct {
	Checks   []Check
	Rows     map[TableRef]int64
	Elapsed  time.Duration
	ExitCode int
	// Unconfirmed counts residual filter hits the source says are absent: a
	// filter false positive, or the source changed since the snapshot. It is
	// printed, not failed.
	Unconfirmed int64
	// Probes counts source confirmation probes issued, against the
	// --residual-probe-cap. A hit that could not be tested is exit 9, because an
	// unconfirmable hit is the case where failing closed costs least.
	Probes int64
}

// Verifier proves what the run did, against the target and a second look at the
// source.
type Verifier interface {
	// Verify uses src.Short for residual confirmation and for the unmasked
	// sample comparison, never the run snapshot, which has been released by now.
	// schema supplies the column types the second net canonicalises by and the
	// identity columns the sample comparison fetches by.
	Verify(
		ctx context.Context,
		src Source,
		w Writer,
		schema *Schema,
		plan *Plan,
		cls *Classification,
		res Residual,
		lr *LoadResult,
	) (*Report, error)
}
