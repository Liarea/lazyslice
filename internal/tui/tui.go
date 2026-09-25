// SPDX-License-Identifier: Apache-2.0

// Package tui holds the two Bubble Tea screens --tui opens: the classification
// reasons screen and the plan screen (ADR-002).
//
// The line printer is the default and stays the default. ADR-002's reason is
// the artefact and not the cost: the transcript survives the run and pastes
// into a compliance ticket, while an alternate-screen app erases itself on
// exit. So `lazyslice` prints lines, and these two screens are entered only on
// a TTY, only when --tui or a question's "?" asks for them, and only for the
// two tables that do not fit a screen — the classifier's reasons on a
// two-hundred-column schema, and the plan on forty tables.
//
// The TUI owns no logic. It is one more sink on the same event channel as
// render.Lines (every event.Event arrives as a tea.Msg), it reaches no stage,
// and the only thing it produces is a core.Request — the same struct
// cmd/lazyslice builds from flags, field for field. Every action it offers is a
// flag in ARCHITECTURE.md section 8 first, which Binding.Flag records and
// TestEveryTUIActionHasFlag in cmd/lazyslice enforces.
//
// Leaving either screen returns to the line printer with the screen's contents
// echoed into scrollback, together with the flags that would make the same
// request again.
package tui

import (
	"context"
	"errors"
	"fmt"
	"io"

	tea "charm.land/bubbletea/v2"

	"github.com/Liarea/lazyslice/internal/core"
	"github.com/Liarea/lazyslice/internal/event"
)

// Input is one visit to the two screens.
type Input struct {
	// Request is what the flags built. The screens start from it and hand back
	// the same struct with what the operator changed.
	Request core.Request
	// Events are the classification decisions and the plan steps a run has
	// already produced, from a Collector. Each is applied to the model as a
	// tea.Msg, which is the same path a live event would take.
	Events []event.Event
	// Dropped is the collector's refused count, printed in the header so that a
	// truncated screen says so.
	Dropped int
	// Target is the endpoint the plan pass resolved (core.Reviewed.Target),
	// named in the confirmation Accept opens on a mode that would drop and
	// rewrite it, and in the line printed when the operator leaves instead
	// (T-0345). Empty for a mode that opens no target (classify, plan).
	Target string
	// In and Out are the terminal. Both are required in a real run; a test
	// passes a buffer to each.
	In  io.Reader
	Out io.Writer
}

// Result is what the operator left with.
type Result struct {
	// Request is what the screens built.
	Request core.Request
	// Flags is the command line that would build the same request without the
	// TUI, in the order ARCHITECTURE.md section 8 groups them.
	Flags []string
	// Run reports that the operator left with "leave and run" rather than with
	// cancel. A cancel is not an error: it is a decision, and the caller prints
	// nothing further and exits 0.
	Run bool
	// Interrupted reports that Run is false because ctrl+c reached the
	// screens, rather than esc or "q". The caller reports this the same way it
	// reports a SIGINT during the run itself: exit 130, not the plain 0 a
	// decision to leave gets (T-0345, ADR-005).
	Interrupted bool
	// Transcript is what was echoed into scrollback.
	Transcript string
}

// ErrNoTerminal is returned when Run is called with no terminal to draw on. It
// exists so that the caller's TTY check and this package's own cannot disagree
// about whether the screens were entered.
var ErrNoTerminal = errors.New("tui: no terminal")

// Run opens the two screens over the events a run produced and returns the
// request the operator built.
//
// It is the whole of this package's surface. It starts nothing, reads no
// database and holds no credential: the caller runs the pipeline, hands over
// what the classifier and the planner said, and calls core.Run again with the
// Result's request if the operator asked for it.
func Run(ctx context.Context, in Input) (Result, error) {
	if in.In == nil || in.Out == nil {
		return Result{}, ErrNoTerminal
	}

	start := newModel(in.Request).seed(in.Events).setDropped(in.Dropped).setTarget(in.Target)

	final, err := tea.NewProgram(start,
		tea.WithContext(ctx),
		tea.WithInput(in.In),
		tea.WithOutput(in.Out),
	).Run()
	if err != nil {
		return Result{}, fmt.Errorf("tui: %w", err)
	}

	done, ok := final.(model)
	if !ok {
		// tea.Program returns the model it was given; a different type would be
		// a framework change, and guessing at it would hand the caller a
		// request nobody built.
		return Result{}, fmt.Errorf("tui: the program returned a %T", final)
	}

	out := Result{
		Request:     done.request(),
		Flags:       done.flags(),
		Run:         done.accepted,
		Interrupted: done.interrupted,
	}
	// The screen contents go into scrollback on the way out, because the two
	// screens must not be the one part of a run that leaves nothing behind
	// (ADR-002). But that is the artefact for a run that is about to happen —
	// on a leave, the operator did not run the screen just reviewed, and
	// echoing it back read as a truncated second copy of a plan or a
	// classification already in scrollback above it (T-0345, dogfood session
	// 3, finding 4). A leave gets the one line that says what did not happen
	// instead.
	switch {
	case out.Run:
		out.Transcript = done.transcript()
	case done.writesTarget():
		// Only ModeRun and ModeVerify would have dropped and rewritten
		// anything; naming a target on a leave from classify or plan would
		// claim a write that was never going to happen.
		out.Transcript = closingLine(in.Target)
	default:
		out.Transcript = "stopped before writing anything\n"
	}
	if _, err := io.WriteString(in.Out, out.Transcript); err != nil {
		return out, fmt.Errorf("tui: echoing the screen: %w", err)
	}
	return out, nil
}

// closingLine is what a leave prints instead of the screen it leaves, on a
// mode that would have dropped and rewritten a target: nothing was written,
// named against the endpoint the confirmation would also have named (T-0345).
func closingLine(target string) string {
	if target == "" {
		target = "the target"
	}
	return "stopped before writing anything; nothing changed in " + target + "\n"
}
