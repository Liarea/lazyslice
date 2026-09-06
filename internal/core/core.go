// SPDX-License-Identifier: Apache-2.0

// Package core wires the nine stages together. It is the only place that does.
//
// Run drives discover, introspect, classify, plan, extract, transform, load,
// verify and emit, in that order, and sends every stage transition into the
// event sink. The line printer, the NDJSON writer and the TUI are three sinks on
// one channel and none of them reaches a stage, so adding a renderer cannot
// change what a run does.
//
// Request mirrors the CLI flag surface (ARCHITECTURE.md section 8) field for
// field. cmd/lazyslice builds one from flags; internal/tui builds the same
// struct from keystrokes. Neither of them talks to a stage.
//
// Scaffold status: Run walks the nine stages, announcing each one and reporting
// that it is not implemented, then returns pipeline.ErrNotImplemented. It moves
// no data, opens no connection and touches no database.
package core

import (
	"context"
	"fmt"
	"time"

	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pipeline"
)

// Defaults are the v1 flag defaults from ARCHITECTURE.md section 8. They are
// constants here rather than strings in the flag definitions so that the TUI,
// the yml reader and the CLI agree on one set.
const (
	DefaultTake             = 500
	DefaultCap              = 100
	DefaultDepth            = 3
	DefaultRowBudget        = int64(1_000_000)
	DefaultMemoryBudget     = "256MiB"
	DefaultResidualProbeCap = 1000
	DefaultConfigPath       = "./lazyslice.yml"
	// DefaultSecretFile is a path, not a secret; the key lives in the file.
	DefaultSecretFile = "./lazyslice.secret" //nolint:gosec // G101: a filename, not a credential
)

// Request is one run, as the flags describe it. Every field is a flag in
// ARCHITECTURE.md section 8; nothing here is a value from either database.
type Request struct {
	// Workdir is where discovery looks for lazyslice.yml, .env files and the
	// compose project. It is the process working directory unless a test sets it.
	Workdir string

	// discover
	Source              string // positional DSN or --source
	Target              string // --target
	DockerHost          string // --docker-host
	PasswordCommand     string // --password-command
	CreateTarget        bool   // --create-target
	AllowRemoteTarget   string // --allow-remote-target HOST
	RequireReadOnlyRole bool   // --require-read-only-role
	Reconfigure         bool   // --reconfigure

	// plan
	Root         string              // --root
	Take         int                 // --take, -n
	Where        string              // --where
	Cap          int                 // --cap N
	TableCaps    map[string]int      // --cap TABLE=N, repeatable
	Depth        int                 // --depth
	RowBudget    int64               // --row-budget
	MemoryBudget string              // --memory-budget
	Keys         map[string][]string // --key TABLE=COL,COL, repeatable
	SkipTables   []string            // --skip-table, repeatable
	PlanOnly     bool                // --plan

	// classify
	Unmask       map[string]string // --unmask TABLE.COL=REASON, repeatable
	StrictSchema bool              // --strict-schema

	// transform
	SecretFile string // --secret-file
	RequireKey bool   // --require-key

	// extract
	SingleConnection bool // --single-connection

	// load and verify
	ShowRowValuesInErrors bool // --show-row-values-in-errors
	ResidualProbeCap      int  // --residual-probe-cap

	// all stages
	ConfigPath string // --config
	NoConfig   bool   // --no-config
	Yes        bool   // --yes

	// render
	JSON  bool // --json
	TUI   bool // --tui
	Debug bool // --debug
}

// NewRequest returns a Request carrying the v1 defaults. Flags overwrite fields
// on it, so a field nobody set holds the documented default rather than a zero.
func NewRequest() Request {
	return Request{
		Take:             DefaultTake,
		Cap:              DefaultCap,
		Depth:            DefaultDepth,
		RowBudget:        DefaultRowBudget,
		MemoryBudget:     DefaultMemoryBudget,
		ResidualProbeCap: DefaultResidualProbeCap,
		ConfigPath:       DefaultConfigPath,
		SecretFile:       DefaultSecretFile,
		TableCaps:        map[string]int{},
		Keys:             map[string][]string{},
		Unmask:           map[string]string{},
	}
}

// stages is the pipeline order (ADR-005). Run walks it; nothing else defines it.
var stages = []event.Stage{
	event.Discover,
	event.Introspect,
	event.Classify,
	event.Plan,
	event.Extract,
	event.Transform,
	event.Load,
	event.Verify,
	event.Emit,
}

// Run drives the pipeline and returns the report the exit code is computed from.
//
// A run holds the source snapshot from the start of introspect to the end of
// extract, releases it, loads, verifies, then emits. Stage transitions are
// events, so the transcript in CONCEPT.md is the event stream rendered as lines.
//
// Scaffold status: no stage is implemented. Run announces each one and returns
// pipeline.ErrNotImplemented, so that a scaffold build is loud rather than
// silently successful.
func Run(ctx context.Context, _ Request, sink event.Sink) (*pipeline.Report, error) {
	for _, s := range stages {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		sink.Send(event.Event{At: time.Now(), Stage: s, Kind: event.StageStart, Code: event.CodeStageStart})
		sink.Send(event.Event{
			At:    time.Now(),
			Stage: s,
			Kind:  event.Info,
			Code:  event.CodeNotImplemented,
			Args:  event.Args{event.ArgStage: s.String()},
		})
		sink.Send(event.Event{At: time.Now(), Stage: s, Kind: event.StageDone, Code: event.CodeStageDone})
	}
	return nil, fmt.Errorf("core: %w", pipeline.ErrNotImplemented)
}
