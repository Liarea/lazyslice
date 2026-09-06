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

	// The things section 6 item 5 reports rather than fails — a schema-only
	// step, a lookup with no source rows, a sequence owned by no column, a step
	// with no keys to sample by — carry no reason: pipeline.Check has no field
	// for one, so each of them is its own event.Code instead (codes.go).

	// reasonNotCalled and reasonLastValue name which half of the strict-NULL
	// form the target disagrees with.
	reasonNotCalled = "is_called does not match whether the column holds a row"
	reasonLastValue = "last_value is not the column's maximum"

	// reasonTargetNotReadable is the wiring failure: the writer this stage was
	// handed cannot be read from, so no check below could run at all. See the
	// package comment and internal/verify/CLAUDE.md.
	reasonTargetNotReadable = "the target connection cannot be read from"
)
