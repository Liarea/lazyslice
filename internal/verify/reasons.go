// SPDX-License-Identifier: Apache-2.0

package verify

// The fixed phrases a Refusal.Reason and a Check may carry.
//
// They are a closed set for the same reason internal/classify's reason
// templates are: a reason string leaves this process through an event and a
// --json report, and a free-form one is a place a row value can end up
// (THREAT_MODEL.md T4, ARCHITECTURE.md section 2 "Value-free types"). Nothing
// here interpolates anything but an identifier, a category name or a count.
const (
	// reasonSourceClosed is section 6 item 3's "Source.Short cannot open".
	reasonSourceClosed = "the source would not open a second transaction"
	// reasonProbeFailed is "a probe errors, the role lacks SELECT".
	reasonProbeFailed = "a confirmation probe failed, or the role may not read the source table"
	// reasonProbeCap is "the cap is reached with hits still untested".
	reasonProbeCap = "the probe cap was reached with hits still untested"
	// reasonNoProbe is a hit no probe of section 6 can express: a JSON leaf,
	// whose value is inside a document and not the column's own value. It is
	// untestable, which item 3 makes exit 9.
	reasonNoProbe = "a masked JSON leaf cannot be confirmed by an equality probe on the column"
	// reasonArrayLiteral is a masked array column whose target value is neither
	// a slice nor a Postgres array literal. internal/transform records one
	// filter entry per element for such a column, so a value this stage cannot
	// split into elements is a value none of those entries can be tested
	// against — untestable for the same reason a leaf is, and therefore exit 9
	// rather than a scan that passes green over it (tracker T-0129,
	// THREAT_MODEL.md T12).
	reasonArrayLiteral = "a value in this array column is not a Postgres array literal, so its elements cannot be tested one by one"

	// reasonStillHolds is a confirmed residual hit: the column probe found the
	// value in the source column.
	reasonStillHolds = "the source still holds this value in this column"
	// reasonOverCount is ADR-015's count check: a value inside the masker's
	// own vocabulary that the target holds more often than internal/transform
	// emitted it, and that the column probe then confirmed in the source. A
	// copy the masker never produced came from somewhere else.
	reasonOverCount = "the target holds this value more often than the masker produced it"
	// reasonSameRow is ADR-015's row check: the source row a target row was
	// copied from, matched on the row identity both sides hold verbatim, holds
	// the target row's masked value in this column, canonical-equal or equal
	// over its letters and digits. The masker is redrawn until its output never
	// reads as its own input (mask.maskCell), so only a bypass puts it there.
	reasonSameRow = "the source row this row was copied from still holds this value in this column"

	// The things section 6 item 5 reports rather than fails — a schema-only
	// step, a lookup with no source rows, a sequence owned by no column, a step
	// with no keys to sample by — carry no reason: pipeline.Check has no field
	// for one, so each of them is its own event.Code instead (codes.go).

	// reasonNotCalled and reasonLastValue name which half of the strict-NULL
	// form the target disagrees with.
	reasonNotCalled = "is_called does not match whether the column holds a row"
	reasonLastValue = "last_value is not the column's maximum"
	// reasonNoSequence is the third failure of the same check and a different
	// claim: the column owns a sequence in the source and the target resolves
	// none for it, under either name sequenceNameSQL tries. That is a sequence
	// §11.1's DDL did not create, so nothing reset it and the application's
	// first INSERT has no number to take (THREAT_MODEL.md T8). It is a failure
	// and not a CodeSequenceUnowned report: "the loader could not attribute this
	// sequence to a column" is a thing §6 item 5 says to report, and "the target
	// has no such relation" is a defect wearing the same word.
	reasonNoSequence = "the target has no sequence behind this column"

	// reasonTargetNotReadable is the wiring failure: the writer this stage was
	// handed cannot be read from, so no check below could run at all. See the
	// package comment and internal/verify/CLAUDE.md.
	reasonTargetNotReadable = "the target connection cannot be read from"
)
