// SPDX-License-Identifier: Apache-2.0

package core

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// unclaimedCanaryError stands in for "a new error type anywhere in the tree"
// (T-0212's own framing): none of asStop's five typed refusals recognise it,
// so it falls all the way through to the exhaustive default at the end of
// asStop. Its Error() quotes a value the way a library's or a third-party
// masker's own error might.
type unclaimedCanaryError struct{ canary string }

func (e unclaimedCanaryError) Error() string {
	return fmt.Sprintf("could not process %q", e.canary)
}

// TestAsStopRedactsAnUnclaimedErrorsMessage is the T-0212 fix round's finding
// 1, tested at the exact line it named (asStop's exhaustive fallback,
// previously `wrap(CodeInternal, exitInternal, err, "%s", err.Error())`):
// that fallback used to copy an unrecognised error's whole Error() text into
// Stop.Message, and cmd/lazyslice's renderSafe prints every *Stop.Error()
// unconditionally by construction, so the exact case T-0212 exists to contain
// reached stderr at both settings of --show-row-values-in-errors. The
// existing renderSafe unit tests call it with a bare error and never through
// this function, so they could not have caught that — this test calls asStop
// itself, the one place a Stop's Message is actually built from a stage
// error, the same way every real refusal that reaches cmd/lazyslice does
// (run.go's asStop call sites).
func TestAsStopRedactsAnUnclaimedErrorsMessage(t *testing.T) {
	const canary = "victim.canary@bigcorp.com"
	err := unclaimedCanaryError{canary: canary}

	got := asStop(err)

	var stop *Stop
	if !errors.As(got, &stop) {
		t.Fatalf("asStop(%T) = %T, want *Stop", err, got)
	}
	if stop.Code != CodeInternal {
		t.Errorf("stop.Code = %s, want %s (nothing claimed this error)", stop.Code, CodeInternal)
	}
	if !stop.Unclaimed() {
		t.Errorf("stop.Unclaimed() = false, want true: cmd/lazyslice's renderSafe (T-0212 fix round, " +
			"finding 1) relies on this to redact even if a future regression here stops calling " +
			"PanicSummary")
	}
	if strings.Contains(stop.Message, canary) {
		t.Errorf("stop.Message = %q, want the canary withheld: an unrecognised error's own words "+
			"must never reach Stop.Message, which cmd/lazyslice's renderSafe prints in full for every "+
			"*core.Stop", stop.Message)
	}
	if strings.Contains(stop.Error(), canary) {
		t.Errorf("stop.Error() = %q, want the canary withheld", stop.Error())
	}
	if !strings.Contains(stop.Message, "unclaimedCanaryError") {
		t.Errorf("stop.Message = %q, want it to name the error's own type (PanicSummary's rule)", stop.Message)
	}
	// The original error is still reachable through Unwrap, for --debug: this
	// test is about what a Stop prints unconditionally, not about withholding
	// the cause from the one flag that asks for a full trail.
	if !errors.Is(stop.Unwrap(), err) {
		t.Errorf("stop.Unwrap() = %v, want the original error, still reachable for --debug", stop.Unwrap())
	}
}
