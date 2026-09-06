// SPDX-License-Identifier: Apache-2.0

// Package render turns events into output. It holds the two sinks that ship in
// v1: Lines, the default human transcript, and NDJSON, what --json writes.
//
// A renderer never reaches a stage and a stage never formats a sentence. Every
// line comes from the code catalogue in internal/event, which is also the source
// of docs/ERRORS.md, so the text a user sees and the text the documentation
// promises cannot drift apart.
//
// Scaffold status: both sinks accept events and drop them; the catalogue and
// the line templates arrive with the rendering task.
package render

import (
	"io"

	"charm.land/lipgloss/v2"

	"github.com/Liarea/lazyslice/internal/event"
)

// Lines is the default sink: the transcript in CONCEPT.md, rendered from the
// event stream.
type Lines struct {
	w io.Writer
	// Style carries the colours. Colour is a property of the renderer, never of
	// a stage.
	Style lipgloss.Style
}

// NewLines returns the default line printer.
func NewLines(w io.Writer) *Lines { return &Lines{w: w} }

var _ event.Sink = (*Lines)(nil)

// Send renders one event.
//
// Scaffold status: no-op.
func (l *Lines) Send(_ event.Event) {}

// NDJSON writes each event verbatim as one JSON object per line, which is what
// --json produces and what a CI job parses.
type NDJSON struct{ w io.Writer }

// NewNDJSON returns the NDJSON sink.
func NewNDJSON(w io.Writer) *NDJSON { return &NDJSON{w: w} }

var _ event.Sink = (*NDJSON)(nil)

// Send writes one event.
//
// Scaffold status: no-op.
func (n *NDJSON) Send(_ event.Event) {}
