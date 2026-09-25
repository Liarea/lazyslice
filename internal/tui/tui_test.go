// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Liarea/lazyslice/internal/core"
)

// These two drive the whole program the way the framework's own tests do —
// tea.NewProgram over a reader and a writer, with the keys written into the
// reader — rather than calling Update directly. What they prove is what only a
// real program can: that the model reaches tea.Quit, that Run reads the request
// back off it, and that the screen is echoed into scrollback on the way out.

func runWithKeys(t *testing.T, keys string) (Result, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	in := bytes.NewBufferString(keys)
	var out bytes.Buffer
	res, err := Run(ctx, Input{
		Request: core.NewRequest(),
		Events:  fixture(),
		In:      in,
		Out:     &out,
	})
	if err == nil && !strings.Contains(out.String(), res.Transcript) {
		t.Error("the screen was not echoed into scrollback on the way out")
	}
	return res, err
}

// TestRunLeavesWithoutRunning: "q" is a decision and not a failure. The caller
// prints nothing further and exits 0. The screen itself is not echoed — it was
// not run — and scrollback instead gets the one line saying nothing was
// written (T-0345).
func TestRunLeavesWithoutRunning(t *testing.T) {
	res, err := runWithKeys(t, "q")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Run {
		t.Error("q left with the run accepted")
	}
	if res.Interrupted {
		t.Error("q reported an interrupt")
	}
	if !strings.Contains(res.Transcript, "stopped before writing anything") {
		t.Errorf("the transcript does not say nothing was written:\n%s", res.Transcript)
	}
	for _, unwanted := range []string{"public.customer.email", "name rule: email"} {
		if strings.Contains(res.Transcript, unwanted) {
			t.Errorf("a leave still echoed the screen (%q):\n%s", unwanted, res.Transcript)
		}
	}
}

// TestCtrlCLeavesAsAnInterrupt: ctrl+c is the terminal's own interrupt, not a
// decision, and Result.Interrupted is how the caller tells the two apart
// (T-0345).
func TestCtrlCLeavesAsAnInterrupt(t *testing.T) {
	res, err := runWithKeys(t, "\x03")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Run {
		t.Error("ctrl+c left with the run accepted")
	}
	if !res.Interrupted {
		t.Error("ctrl+c did not report an interrupt")
	}
}

// TestPlanOnlyLeaveNamesNoTarget: `--plan --tui` builds a request with
// PlanOnly set even though Mode is still ModeRun's zero value. core stops
// before the target is touched on PlanOnly alone, so a leave from that
// request must print the plain "nothing was written" line, not closingLine's
// "nothing changed in <target>" — that clause claims a write which this
// request was never going to make (T-0345 review).
func TestPlanOnlyLeaveNamesNoTarget(t *testing.T) {
	req := core.NewRequest()
	req.PlanOnly = true

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	in := bytes.NewBufferString("q")
	var out bytes.Buffer
	res, err := Run(ctx, Input{
		Request: req,
		Events:  fixture(),
		In:      in,
		Out:     &out,
		Target:  "nobody@127.0.0.1:5432/target",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Run {
		t.Error("q left with the run accepted")
	}
	if !strings.Contains(res.Transcript, "stopped before writing anything") {
		t.Errorf("the transcript does not say nothing was written:\n%s", res.Transcript)
	}
	if strings.Contains(res.Transcript, "nothing changed in") {
		t.Errorf("a PlanOnly leave named a target as if it would have been rewritten:\n%s", res.Transcript)
	}
}

// TestRunBuildsTheRequestItLeavesWith walks the reasons screen the way an
// operator does — opt a column out, give the reason, confirm, leave and run —
// through a real program, and reads the core.Request back. The third enter is
// T-0345: the first two build the opt-out, and ModeRun's Accept confirms
// before it leaves.
func TestRunBuildsTheRequestItLeavesWith(t *testing.T) {
	res, err := runWithKeys(t, "usupport ticket\r\r\r")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.Run {
		t.Error("enter did not leave with the run accepted")
	}
	if got := res.Request.Unmask["public.customer.email"]; got != "support ticket" {
		t.Errorf("Unmask[public.customer.email] = %q, want %q", got, "support ticket")
	}
	want := `--unmask "public.customer.email=support ticket"`
	if len(res.Flags) != 1 || res.Flags[0] != want {
		t.Errorf("Flags = %q, want [%s]", res.Flags, want)
	}
	if !strings.Contains(res.Transcript, want) {
		t.Errorf("the transcript does not name the flag that would do the same:\n%s", res.Transcript)
	}
}

// TestRunRefusesWithNoTerminal: the caller's TTY check and this package's must
// not be able to disagree about whether the screens were entered.
func TestRunRefusesWithNoTerminal(t *testing.T) {
	if _, err := Run(t.Context(), Input{Request: core.NewRequest()}); !errors.Is(err, ErrNoTerminal) {
		t.Errorf("Run with no terminal returned %v, want ErrNoTerminal", err)
	}
}
