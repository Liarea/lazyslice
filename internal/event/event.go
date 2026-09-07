// SPDX-License-Identifier: Apache-2.0

// Package event is the progress event model (ARCHITECTURE.md section 7).
//
// core.Run is the only producer. The line printer, the NDJSON writer and the
// TUI are three sinks on one channel and none of them reaches a stage directly,
// so adding a renderer can never change what a stage does.
//
// An Event carries identifiers and counts and nothing else. There is no value
// field, by construction: Event's transitive field types exclude dsn.DSN,
// pipeline.RowBatch and pipeline.Table, and therefore Table.Samples
// (THREAT_MODEL.md T4). The import graph in ARCHITECTURE.md section 2 is what
// makes that checkable — event imports only ref, never pipeline.
package event

import (
	_ "embed"
	"time"

	"github.com/Liarea/lazyslice/internal/ref"
)

// catalogueYML is internal/event/catalogue.yml, the code catalogue
// (ARCHITECTURE.md §7 and §12): the one place a message a user reads is
// written, and the source docs/ERRORS.md is generated from.
//
// The embed lives here because this is the package the file belongs to, and
// Go's embed directive cannot reach outside its own directory — a path with
// ".." is rejected by the compiler. internal/render used to carry a byte-for-
// byte copy of the file for that reason, with a test comparing the two; the
// copy and the test are gone and Catalogue() is what a renderer reads
// (tracker T-0058).
//
//go:embed catalogue.yml
var catalogueYML []byte

// Catalogue returns the raw bytes of the code catalogue.
//
// It is the bytes rather than a parsed structure because the row type is a
// renderer's concern — internal/render unmarshals it, and docs/ERRORS.md is
// generated from the same file — while this package must stay free of anything
// but the event model. It returns a fresh slice per call so that no caller can
// write through the embedded one.
func Catalogue() []byte {
	out := make([]byte, len(catalogueYML))
	copy(out, catalogueYML)
	return out
}

// Stage is one of the nine pipeline stages (ADR-005).
type Stage int

// The nine stages, in the order core.Run drives them.
const (
	Discover Stage = iota
	Introspect
	Classify
	Plan
	Extract
	Transform
	Load
	Verify
	Emit
)

var stageNames = [...]string{
	Discover:   "discover",
	Introspect: "introspect",
	Classify:   "classify",
	Plan:       "plan",
	Extract:    "extract",
	Transform:  "transform",
	Load:       "load",
	Verify:     "verify",
	Emit:       "emit",
}

// String renders the stage name as it appears in a rendered line and in NDJSON.
func (s Stage) String() string {
	if s < 0 || int(s) >= len(stageNames) {
		return "unknown"
	}
	return stageNames[s]
}

// Kind is what happened, independent of which stage it happened in.
type Kind int

// The event kinds. Progress events are dropped when the sink channel is full;
// every other kind blocks.
const (
	StageStart Kind = iota
	StageDone
	Progress
	Decision
	Question
	Info
	Warn
	Error
)

var kindNames = [...]string{
	StageStart: "stage_start",
	StageDone:  "stage_done",
	Progress:   "progress",
	Decision:   "decision",
	Question:   "question",
	Info:       "info",
	Warn:       "warn",
	Error:      "error",
}

// String renders the kind name.
func (k Kind) String() string {
	if k < 0 || int(k) >= len(kindNames) {
		return "unknown"
	}
	return kindNames[k]
}

// Code identifies what to say. Every Code is a row in internal/event's
// catalogue and therefore a row in docs/ERRORS.md; CI fails on a code in the
// tree that the catalogue does not carry. Renderers look the text up by Code,
// so no stage ever formats a sentence.
type Code string

// The codes the scaffold itself emits. The rest arrive with the catalogue.
const (
	// CodeStageStart and CodeStageDone bracket every stage.
	CodeStageStart Code = "stage.start"
	CodeStageDone  Code = "stage.done"
	// CodeNotImplemented is emitted by every no-op stage in the scaffold. It
	// exists so that a scaffold run is loud rather than silently successful.
	CodeNotImplemented Code = "stage.not_implemented"
)

// ArgKey is the fixed key set for Args. Args is not a free-form map: a test
// asserts that no Code's template references a key outside this enum, which is
// how a row value is kept out of a rendered line.
type ArgKey string

// The argument keys. Values are identifiers or formatted numbers, never data.
const (
	ArgTable      ArgKey = "table"
	ArgColumn     ArgKey = "column"
	ArgCount      ArgKey = "count"
	ArgFlag       ArgKey = "flag"
	ArgProvenance ArgKey = "provenance"
	ArgRole       ArgKey = "role"
	ArgVersion    ArgKey = "version"
	ArgPath       ArgKey = "path"
	ArgSeconds    ArgKey = "seconds"
	// ArgStatement carries SQL rendered from a catalogue template with
	// identifiers substituted, such as the GRANT block in ARCHITECTURE.md
	// section 9. It never carries a statement the run executed.
	ArgStatement ArgKey = "statement"
	ArgHost      ArgKey = "host"
	ArgDatabase  ArgKey = "database"
	ArgStage     ArgKey = "stage"
	ArgReason    ArgKey = "reason"
)

// Args is a fixed-key map of identifiers and counts.
type Args map[ArgKey]string

// Event is one thing that happened. It is the only structure a renderer sees.
type Event struct {
	At     time.Time
	Stage  Stage
	Kind   Kind
	Code   Code
	Table  ref.TableRef
	Column string
	Done   int64
	Total  int64
	Args   Args
	// Exit is non-zero only with Kind == Error, and its value is the process
	// exit code ADR-005 assigns to that Code.
	Exit int
}

// Sink receives events. Send must not block indefinitely and must be safe to
// call from the goroutine driving the pipeline.
type Sink interface {
	Send(Event)
}

// SinkFunc adapts a function to Sink.
type SinkFunc func(Event)

// Send implements Sink.
func (f SinkFunc) Send(e Event) { f(e) }

// Discard is a Sink that drops every event. It is the default in tests and in
// any caller that only wants the returned Report.
var Discard Sink = SinkFunc(func(Event) {})
