// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"context"
	"time"

	"github.com/Liarea/lazyslice/internal/event"
)

// Check is one verification result. Names are fixed: "fk", "residual",
// "residual_unconfirmable", "second_net", "catalog", "sequences", "row_count",
// "unmasked_identical".
//
// "catalog" is the pass over the target's own pg_attrdef and pg_constraint
// (ARCHITECTURE.md section 11.1, amended 2026-09-14): a string literal inside a
// recreated default, generated expression or CHECK is inside the data boundary,
// and no scan of the rows can see one.
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
