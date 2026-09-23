// SPDX-License-Identifier: Apache-2.0

package verify

import (
	"fmt"

	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// The event codes the verify stage renders.
//
// Verifier.Verify takes no event.Sink (ARCHITECTURE.md section 2), because
// core.Run is the only producer of events (section 7), so these are declared
// here beside the results that carry them: every pipeline.Check in the report
// carries one, and a failing check is also returned as a *Refusal that core
// renders from the catalogue. Every code here has a row in
// internal/event/catalogue.yml, which is the source of docs/ERRORS.md.
//
// A code's arguments are identifiers and counts and nothing else
// (THREAT_MODEL.md T4). {reason} carries a fixed phrase from reasons.go, a
// check name, a validator name or a category name — never a value read from a
// row, and never the candidate value a confirmation probe was built from.
const (
	// CodeRefusedFK is exit 8: a foreign key of the source does not hold over
	// the rows in the target, so the slice is not referentially complete.
	CodeRefusedFK event.Code = "verify.refused.fk"

	// CodeRefusedResidual is exit 9: a value the run masked is in the target as
	// the source holds it, confirmed against the source in the short second
	// transaction (ARCHITECTURE.md section 6 item 3, THREAT_MODEL.md T12).
	CodeRefusedResidual event.Code = "verify.refused.residual"

	// CodeRefusedUnconfirmable is exit 9: a residual hit could not be tested at
	// all. An unconfirmable hit is the case where failing closed costs least,
	// so it is never a printed note.
	CodeRefusedUnconfirmable event.Code = "verify.refused.residual_unconfirmable"

	// CodeRefusedSecondNet is exit 9: a column the target holds unmasked still
	// validates as a category (ARCHITECTURE.md section 6 item 4).
	CodeRefusedSecondNet event.Code = "verify.refused.second_net"

	// CodeRefusedCatalogLiteral is exit 9: the target's own catalog carries a
	// string literal that parses as an email address, a telephone number or a
	// payment card, in a column default, a generated expression or a CHECK
	// constraint (ARCHITECTURE.md section 11.1's 2026-09-14 amendment,
	// THREAT_MODEL.md T1). A row scan cannot see it and the application's next
	// INSERT can put it back into a row, which is what
	// docs/reviews/2026-09-09 finding 5 demonstrated under exit 0.
	CodeRefusedCatalogLiteral event.Code = "verify.refused.catalog_literal"

	// CodeRefusedRowCount is exit 7: a step does not hold the rows the plan
	// says it holds. ADR-005's table has no code of its own for this, and a
	// count that disagrees with the plan is a failure of the movement of rows;
	// internal/verify/CLAUDE.md records the reading.
	CodeRefusedRowCount event.Code = "verify.refused.row_count"

	// CodeRefusedSequence is exit 7: a sequence was not reset in the strict-NULL
	// form, so the application's first INSERT collides (THREAT_MODEL.md T8).
	CodeRefusedSequence event.Code = "verify.refused.sequence"

	// CodeUnconfirmed reports residual hits the source says are absent: a filter
	// false positive, or the source changed since the snapshot. It is printed
	// and counted, never failed (ARCHITECTURE.md section 6 item 3).
	CodeUnconfirmed event.Code = "verify.residual.unconfirmed"

	// CodeSampleDiffers reports sampled rows whose unmasked columns differ from
	// the source. The short transaction sees a newer snapshot than extract did,
	// so this is reported and counted, never failed (section 6 item 5).
	CodeSampleDiffers event.Code = "verify.sample.differs"

	// CodeSampleAbsent reports sampled rows the source no longer holds at all.
	CodeSampleAbsent event.Code = "verify.sample.absent"

	// The three things section 6 item 5 requires to be reported rather than
	// failed: a step the plan gives no count to check, a sequence owned by no
	// column, and a step the sample comparison could not fetch by identity.
	CodeRowCountReported event.Code = "verify.row_count.reported"
	CodeSequenceUnowned  event.Code = "verify.sequence.unowned"
	CodeSampleReported   event.Code = "verify.sample.reported"

	// One code per check that found nothing. They are separate codes rather
	// than one "passed" code with the check's name as an argument because
	// pipeline.Check carries no free-text field and event.ArgKey has no key for
	// a check name — and inventing one would be a place a sentence could grow
	// (ARCHITECTURE.md section 7).
	CodeFKPassed        event.Code = "verify.fk.passed"
	CodeResidualPassed  event.Code = "verify.residual.passed"
	CodeResidualProbes  event.Code = "verify.residual.probes"
	CodeSecondNetPassed event.Code = "verify.second_net.passed"
	CodeCatalogPassed   event.Code = "verify.catalog.passed"
	CodeRowCountPassed  event.Code = "verify.row_count.passed"
	CodeSequencesPassed event.Code = "verify.sequences.passed"
	CodeSamplePassed    event.Code = "verify.sample.passed"
)

// The exit codes ADR-005's table assigns: 7 "extract or load", 8 "FK
// verification", 9 "residual personal data, or residual hits that could not be
// confirmed".
const (
	exitLoad     = 7
	exitFK       = 8
	exitResidual = 9
)

// The fixed check names (ARCHITECTURE.md section 2, pipeline.Check).
const (
	checkFK             = "fk"
	checkResidual       = "residual"
	checkUnconfirmable  = "residual_unconfirmable"
	checkSecondNet      = "second_net"
	checkCatalog        = "catalog"
	checkSequences      = "sequences"
	checkRowCount       = "row_count"
	checkUnmaskedSameAs = "unmasked_identical"
)

// order is the order failures are reported in, which is ARCHITECTURE.md section
// 6's own order: the residual scan and the second net first, because a leak is
// the refusal that matters most, then the checks of item 5.
var order = []string{
	checkResidual,
	checkUnconfirmable,
	checkSecondNet,
	checkCatalog,
	checkFK,
	checkRowCount,
	checkSequences,
	checkUnmaskedSameAs,
}

// Refusal is a verify failure core can render from the catalogue. It names the
// table, the column and the check, and it never carries a value: not the
// residual candidate, not the row that differed, not the predicate.
type Refusal struct {
	Code event.Code
	Exit int
	// Check is the pipeline.Check name this refusal came from.
	Check  string
	Table  ref.TableRef
	Column string
	Count  int64
	// Reason is a fixed phrase, a check name, a validator name or a category
	// name. It is never a value (THREAT_MODEL.md T4).
	Reason string
	// err is the underlying error, kept for a caller that needs the driver's
	// own words. It is not rendered into the event.
	err error
}

func (r *Refusal) Error() string {
	where := r.Table.String()
	if r.Column != "" {
		where += "." + r.Column
	}
	if r.Reason != "" {
		return fmt.Sprintf("verify: %s on %s: %s", r.Code, where, r.Reason)
	}
	return fmt.Sprintf("verify: %s on %s", r.Code, where)
}

// Unwrap gives access to the driver's error, where there was one.
func (r *Refusal) Unwrap() error { return r.err }

// check builds the pipeline.Check a refusal corresponds to, so that the report
// and the error say the same thing.
func (r *Refusal) check() pipeline.Check {
	return pipeline.Check{
		Name:   r.Check,
		Passed: false,
		Code:   r.Code,
		Table:  r.Table,
		Column: r.Column,
		Count:  r.Count,
	}
}
