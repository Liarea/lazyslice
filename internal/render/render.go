// SPDX-License-Identifier: Apache-2.0

// Package render turns events into output. It holds the two sinks that ship in
// v1: Lines, the default human transcript, and NDJSON, what --json writes.
//
// A renderer never reaches a stage and a stage never formats a sentence. Every
// line comes from the code catalogue in internal/event, which is also the source
// of docs/ERRORS.md, so the text a user sees and the text the documentation
// promises cannot drift apart.
package render

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"charm.land/lipgloss/v2"

	"github.com/Liarea/lazyslice/internal/event"
)

// Lines is the default sink: the transcript in CONCEPT.md, rendered from the
// event stream.
//
// Every line is two spaces and a marker, which is the shape CONCEPT.md's
// transcript has: a decision, a warning and a refusal are told apart by the
// marker rather than by indentation, so a terminal with no colour reads the
// same as one with it.
type Lines struct {
	mu sync.Mutex
	w  io.Writer
	// Style carries the colours. Colour is a property of the renderer, never of
	// a stage.
	Style lipgloss.Style
}

// NewLines returns the default line printer.
func NewLines(w io.Writer) *Lines { return &Lines{w: w} }

var _ event.Sink = (*Lines)(nil)

// markers are the one-character prefixes each kind prints under. They are the
// whole of this renderer's own vocabulary: everything after the marker comes
// from the catalogue.
var markers = map[event.Kind]string{
	event.Progress: "  ",
	event.Decision: "  ",
	event.Info:     "  ",
	event.Question: "? ",
	event.Warn:     "! ",
	event.Error:    "✗ ",
}

// Send renders one event.
//
// StageStart and StageDone print nothing. They are the pipeline's own brackets
// and they carry no fact a reader of the transcript needs: CONCEPT.md's
// transcript is a list of decisions and results, not a list of stages, and the
// --json stream still carries both for a caller that wants them.
//
// A settled Decision (e.Settled, T-0321) prints nothing either: it is a
// column whose verdict this run reached is the one the committed yml already
// recorded, and classify.reused is the one line that speaks for the settled
// majority on a re-run. The event still reaches every sink — dropping it here
// is a rendering decision, the kind this package's own CLAUDE.md already
// gives Lines (StageStart/StageDone above) — so a caller wanting the full,
// unfolded per-column record still has it in the --json stream.
func (l *Lines) Send(e event.Event) {
	marker, ok := markers[e.Kind]
	if !ok {
		return
	}
	if e.Kind == event.Decision && e.Settled {
		return
	}
	line := text(e)
	if line == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintln(l.w, marker+l.Style.Render(line))
}

// NDJSON writes each event verbatim as one JSON object per line, which is what
// --json produces and what a CI job parses.
type NDJSON struct {
	mu  sync.Mutex
	enc *json.Encoder
}

// NewNDJSON returns the NDJSON sink.
func NewNDJSON(w io.Writer) *NDJSON { return &NDJSON{enc: json.NewEncoder(w)} }

var _ event.Sink = (*NDJSON)(nil)

// Send writes one event.
//
// The event is written as it is, with no field dropped, reordered or
// reformatted: --json is a machine interface, and a renderer that summarised it
// would be a second, quieter opinion about what happened. Nothing is added
// either — event.Event carries no value field by construction
// (THREAT_MODEL.md T4), which is what makes writing it whole safe.
func (n *NDJSON) Send(e event.Event) {
	n.mu.Lock()
	defer n.mu.Unlock()
	// A write that fails has nowhere to go: the sink is the output. It is
	// dropped rather than panicking a run that has a target to finish.
	_ = n.enc.Encode(e) //nolint:errcheck // the sink is the output; a failed write has nowhere to go
}
