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
package core

import (
	"errors"
	"fmt"

	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/ref"
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

// Mode is which of ARCHITECTURE.md section 8's entry points this request is.
//
// It exists because the five stage subcommands stop the pipeline at five
// different places and Request had no field that told them apart: without it,
// `lazyslice introspect` and `lazyslice doctor` build the same Request and Run
// has nothing to branch on (T-0020's log, carried into T-CORE).
type Mode int

// The six entry points. ModeRun is the zero value, so a Request built by hand
// runs the whole pipeline, which is what `lazyslice` with no subcommand does.
const (
	ModeRun        Mode = iota // lazyslice [DSN]
	ModeIntrospect             // lazyslice introspect
	ModeClassify               // lazyslice classify
	ModePlan                   // lazyslice plan, and --plan
	ModeVerify                 // lazyslice verify
	ModeDoctor                 // lazyslice doctor
)

var modeNames = [...]string{
	ModeRun:        "run",
	ModeIntrospect: "introspect",
	ModeClassify:   "classify",
	ModePlan:       "plan",
	ModeVerify:     "verify",
	ModeDoctor:     "doctor",
}

// String names the mode, for an error message and for the --json stream.
func (m Mode) String() string {
	if m < 0 || int(m) >= len(modeNames) {
		return "unknown"
	}
	return modeNames[m]
}

// needsTarget reports whether the mode writes to, or reads, a target. The four
// read-only modes never open one, which is what lets `lazyslice classify
// --source URL` answer without a second database to point at.
func (m Mode) needsTarget() bool { return m == ModeRun || m == ModeVerify }

// Request is one run, as the flags describe it. Every field is a flag in
// ARCHITECTURE.md section 8; nothing here is a value from either database.
type Request struct {
	// Mode is which entry point this is (section 8's subcommand list).
	Mode Mode

	// Workdir is where discovery looks for lazyslice.yml, .env files and the
	// compose project. It is the process working directory unless a test sets it.
	Workdir string

	// Explicit names the flags the operator actually passed, by their long
	// name. It is how the yml can supply a default without overriding a flag:
	// an int flag bound to its own default is indistinguishable from one the
	// operator typed, and "the file wins over the flag" is the wrong way round
	// for every value in section 10.
	Explicit map[string]bool

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
		Explicit:         map[string]bool{},
	}
}

// set reports whether the operator passed a flag by that name.
func (r Request) set(flag string) bool { return r.Explicit[flag] }

// Stop is a refusal with the event code and the process exit it carries.
//
// Every stage that can refuse returns its own refusal type — plan.Refusal,
// load.Refusal, verify.Refusal — because a stage takes no event.Sink and core
// is the only producer of events (ARCHITECTURE.md section 7). Run converts each
// into one of these, sends the Error event for it, and returns it, so that
// cmd/lazyslice has one type to map to an exit code and no stage-specific
// knowledge at all.
type Stop struct {
	Code event.Code
	Exit int
	// Table, Column and Args are what the catalogue's template for Code
	// substitutes. They carry identifiers and counts only: a refusal is an
	// error message leaving the process, which is the last place a row value
	// could hide (THREAT_MODEL.md T4).
	Table  ref.TableRef
	Column string
	Args   event.Args
	// Message is developer-facing. What a user sees is rendered from
	// internal/event/catalogue.yml by Code.
	Message string
	err     error
}

func (s *Stop) Error() string {
	if s.Message == "" {
		return string(s.Code)
	}
	return string(s.Code) + ": " + s.Message
}

// Unwrap reaches the underlying error, where there was one.
func (s *Stop) Unwrap() error { return s.err }

// stop builds a Stop.
func stop(code event.Code, exit int, format string, a ...any) *Stop {
	return &Stop{Code: code, Exit: exit, Message: fmt.Sprintf(format, a...)}
}

// wrap builds a Stop around an existing error, keeping it reachable so that
// --debug and --show-row-values-in-errors can still get at the driver's words.
func wrap(code event.Code, exit int, err error, format string, a ...any) *Stop {
	return &Stop{Code: code, Exit: exit, Message: fmt.Sprintf(format, a...), err: err}
}

// ErrNoSource is the exit-3 case: nothing to read from. It is its own error
// because discovery is a scaffold, so "no --source and nothing found" is the
// common path today and must say what to do rather than what failed.
var ErrNoSource = errors.New("core: no source")
