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

	"github.com/Liarea/lazyslice/internal/discover"
	"github.com/Liarea/lazyslice/internal/emit"
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
	// AllowTypeLiterals is --allow-type-literal TYPE=REASON, repeatable: the per-type
	// opt-out from ARCHITECTURE.md §11.1's type-literal refusal, keyed by the
	// name as the operator typed it and resolved against the source's own
	// enums and domains in planRequest. It is §8's per-column --unmask for the
	// one object class that is not a column, and it exists because the
	// refusal it clears had no escape at all: --skip-table drops a table to
	// schema only, which still recreates the type (the T-REDFIX review's
	// fourth finding).
	AllowTypeLiterals map[string]string
	PlanOnly          bool // --plan

	// classify
	Unmask map[string]string // --unmask TABLE.COL=REASON, repeatable
	// Mask is --mask TABLE.COL[=CATEGORY], repeatable (T-0319): the
	// counterpart of Unmask, keyed by the name as the operator typed it, with
	// the category or "" for the bare form (DefaultMaskCategory). It is how an
	// operator answers verify.refused.second_net without declaring anything
	// safe; classifyPrior folds it into the classifier's prior and
	// checkMasks refuses a run where it did not take.
	Mask         map[string]string
	StrictSchema bool // --strict-schema
	// PhoneRegion is --phone-region REGION (T-0221): the libphonenumber
	// region a national-format phone column is read under, folded into the
	// classify prior in classifyPrior and carried forward to internal/emit
	// and internal/verify from there. Empty means none was passed on this
	// run; classifyPrior still resolves the committed yml's own
	// phone_region in that case (Config.PhoneRegion's own comment).
	PhoneRegion string

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

	// Reviewed pins this run to the snapshot an earlier pass showed the
	// operator, and is nil for every run built from flags alone. There is no
	// flag for it because it carries no operator intent: it is the identity of
	// what was reviewed, and only a caller that ran the preview can fill it
	// (Preview, cmd/lazyslice's runTUI). When it is set, Run compares the
	// endpoints and the schema after introspect and the classification after
	// classify, and refuses before the plan and before anything is written.
	Reviewed *Reviewed

	// prompter, when set, is copied onto discover.Options.Prompter by
	// resolveEndpoints instead of letting the ladder decide from Yes and the
	// controlling terminal; there is no flag for it, and today only a test
	// sets it (T-0184, ADR-013 review, the 2026-09-16 reverify).
	prompter discover.Prompter
	// noTerminal, when true, is copied onto discover.Options.NoControllingTerminal
	// by rootQuestion and askRoot (root.go) instead of letting the ladder probe
	// the real controlling terminal. Like prompter above, there is no flag for
	// it and today only a test sets it: unlike prompter, whose presence always
	// means "somebody answers", this is the one way to make "there is nobody
	// to ask" true on demand rather than depend on whether the process
	// running the test happens to have a controlling terminal of its own — a
	// fact that differs between an interactive shell and CI, and a test built
	// on it either hung a developer's terminal or never ran the branch it
	// claimed to (T-0271 review).
	noTerminal bool
}

// Reviewed is the snapshot an operator approved, carried into the run that
// writes.
//
// --tui runs the pipeline twice: once as far as the plan, to fill the reasons
// and plan screens, and once for real. Nothing tied the two passes together, so
// the review could be of a different schema read of a different pair of
// databases from the one the second pass wrote — two snapshots and two walks of
// the discovery ladder with no comparison between them (ADR-002, T-TUI's
// review). This is the tie: Preview returns one of these, cmd/lazyslice puts it
// on the request the operator built, and Run refuses rather than writing a
// target the review never covered.
//
// It holds identifiers only. A schema fingerprint is a hash, and an endpoint is
// a dsn.Ref rendered as text, which cannot carry a password because Ref does not
// hold one (internal/dsn) and cannot carry a row value at all
// (THREAT_MODEL.md T4).
type Reviewed struct {
	// SchemaFingerprint is pipeline.Schema.Fingerprint as the preview computed
	// it: ADR-009's hash of the DDL internal/load/ddl generates, from
	// load.SchemaFingerprint, which is the one definition of it in the tree.
	SchemaFingerprint string
	// ClassFingerprint is pipeline.Classification.Fingerprint as the classifier
	// alone decided it, with this run's --unmask opt-outs left out
	// (classifierFingerprint). It is what the reasons screen showed, and the
	// half of the review a schema fingerprint cannot stand for: the classifier
	// reads Table.Samples, so a second snapshot of a live source can move a
	// column that was `possible` only from a value signal below the mask
	// threshold and copy in clear what the operator was shown as masked
	// (THREAT_MODEL.md T1). The run's own --unmask opt-outs are excluded
	// because the reasons screen writes them: they are the operator changing
	// the review, not the source changing under it.
	ClassFingerprint string
	// Source and Target are the endpoints the preview resolved, each rendered
	// by dsn.Ref.String() as "user@host:port/database". Target is empty for the
	// four modes that open none (Mode.needsTarget).
	Source string
	Target string
	// Root is the table the preview pass actually planned from
	// (pipeline.Plan.Root), whichever of --root, the committed yml or ADR-008
	// §6's Q2 decided it. rootQuestion and planRequest (run.go) prefer this
	// field over Request.Root when the screens left that field empty, which is
	// what keeps a --tui run with no --root to ADR-008's one-blocking-question
	// total: without it, core.Run's own Q2 asked the operator the same
	// question a second time, and — since two passes are two schema reads —
	// could in principle answer it differently from the plan just reviewed on
	// the screens (T-0271 review, finding 5).
	//
	// It is a resolved ref.TableRef, not a re-rendered "schema.name" string,
	// for the same reason r.qRoot is (root.go): TableRef.String() is unquoted,
	// and a schema or table name containing a dot round-tripped through
	// Request.Root — the one text field the operator's own --root shares —
	// came back split on the wrong dot by planRequest's resolveTable, one
	// process boundary later than the finding-4 fix that first caught this for
	// Q2's own answer within a single pass (T-0271 review, finding 5's own
	// fix). Empty for a mode that never reaches the plan (ModeIntrospect,
	// ModeDoctor, ModeClassify), the same as ClassFingerprint above.
	Root ref.TableRef
}

// NewRequest returns a Request carrying the v1 defaults. Flags overwrite fields
// on it, so a field nobody set holds the documented default rather than a zero.
func NewRequest() Request {
	return Request{
		Take:              DefaultTake,
		Cap:               DefaultCap,
		Depth:             DefaultDepth,
		RowBudget:         DefaultRowBudget,
		MemoryBudget:      DefaultMemoryBudget,
		ResidualProbeCap:  DefaultResidualProbeCap,
		ConfigPath:        DefaultConfigPath,
		SecretFile:        DefaultSecretFile,
		TableCaps:         map[string]int{},
		Keys:              map[string][]string{},
		Unmask:            map[string]string{},
		Mask:              map[string]string{},
		AllowTypeLiterals: map[string]string{},
		Explicit:          map[string]bool{},
	}
}

// ParseMemoryBudget reads a --memory-budget value and returns the byte count.
//
// It is here so that cmd/lazyslice can refuse a misspelled size at the flag
// surface without reaching a stage package: that file builds a Request and
// calls Run, and importing internal/emit for one parser was the single
// exception to it (cmd/CLAUDE.md, T-0061). The parse itself is internal/emit's
// and planRequest calls the same function, so the flag and the planner can
// never disagree about what "256MiB" means.
func ParseMemoryBudget(s string) (int64, error) { return emit.ParseSize(s) }

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
	// unclaimed marks a Stop built by asStop's exhaustive fallback: an error
	// no stage's typed refusal recognised, whose Message is PanicSummary's
	// type-only description of it rather than a literal string this package
	// wrote. Everywhere else Message is either a format string with no error
	// text spliced in, or a typed refusal's own Error(), value-free by
	// construction (T-0191, T-0212). cmd/lazyslice's renderSafe (T-0212 fix
	// round, finding 1) reads this through Unclaimed rather than trusting
	// every *Stop's Error() the same way, so a future asStop case that copies
	// a raw error's words into Message without going through PanicSummary is
	// still caught at the egress and not just at the one call site this was
	// found at.
	unclaimed bool
	err       error
	// sent records that the Error event for this refusal has already reached
	// the sink. The discovery ladder sends its own (internal/discover's
	// Refusal), and Run's report would otherwise print a second line under the
	// same code and the same exit.
	sent bool
}

func (s *Stop) Error() string {
	if s.Message == "" {
		return string(s.Code)
	}
	return string(s.Code) + ": " + s.Message
}

// Unwrap reaches the underlying error, where there was one.
func (s *Stop) Unwrap() error { return s.err }

// Unclaimed reports whether this Stop is asStop's exhaustive fallback for an
// error no stage's typed refusal recognised, rather than one built from a
// literal string or an already-reviewed refusal's Error(). See the doc
// comment on the unclaimed field.
func (s *Stop) Unclaimed() bool { return s.unclaimed }

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
