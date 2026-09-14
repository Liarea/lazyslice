// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Liarea/lazyslice/internal/classify"
	"github.com/Liarea/lazyslice/internal/discover"
	"github.com/Liarea/lazyslice/internal/dsn"
	"github.com/Liarea/lazyslice/internal/emit"
	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/extract"
	"github.com/Liarea/lazyslice/internal/introspect"
	"github.com/Liarea/lazyslice/internal/load"
	"github.com/Liarea/lazyslice/internal/load/ddl"
	"github.com/Liarea/lazyslice/internal/pg"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/plan"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/internal/repo"
	"github.com/Liarea/lazyslice/internal/transform"
	"github.com/Liarea/lazyslice/internal/verify"
	"github.com/Liarea/lazyslice/mask"
)

// Version is what the run records as `tool:` in the yml and as tool_version in
// the marker table. cmd/lazyslice sets it from the build's own version before
// it calls Run; "dev" is what a `go build` with no ldflags produces.
var Version = "dev"

// batchBuffer is how many RowBatches may sit between two stages. Extract,
// transform and load run concurrently over one channel each way, and the buffer
// is small on purpose: a batch is 2,000 rows of production data held in memory
// (ARCHITECTURE.md section 2), so the pipeline's resident set is bounded by this
// number and not by how far ahead extract can run.
const batchBuffer = 2

// residualBitsPerCell is the residual filter's cost per masked cell, the same
// number internal/plan estimates the filter's memory with (ARCHITECTURE.md
// section 6). It is how core turns the plan's byte estimate back into the cell
// count internal/transform sizes the filter by.
const residualBitsPerCell = 29

// Run drives the pipeline and returns the report the exit code is computed from.
//
// A run holds the source snapshot from the start of introspect to the end of
// extract, releases it, loads, verifies, then emits. Stage transitions are
// events, so the transcript in CONCEPT.md is the event stream rendered as lines.
func Run(ctx context.Context, req Request, sink event.Sink) (*pipeline.Report, error) {
	if sink == nil {
		sink = event.Discard
	}
	ch := newEventChannel(sink)
	r := &run{req: normalise(req), sink: ch}
	defer func() {
		r.close(ctx)
		ch.close()
	}()

	report, err := r.execute(ctx)
	if err != nil {
		r.report(err)
	}
	return report, err
}

// Introspect reads the source catalog and returns the summary
// `lazyslice introspect --json` prints (ARCHITECTURE.md section 8).
//
// It is a second entry point and not a second pipeline: it runs the same
// discover and introspect stages Run does, through the same run struct, and
// stops where ModeIntrospect stops. It exists because a SchemaSummary is a
// document and not an event, and events are the only thing Run returns.
//
// Fingerprint is ADR-009's value, computed by load.SchemaFingerprint like every
// other use of it in the tree. There is no second definition: a summary that
// printed a hash computed some other way would be a hash nothing else agrees
// with, and section 11.2's binding is exactly a comparison of two of them.
func Introspect(ctx context.Context, req Request, sink event.Sink) (*pipeline.SchemaSummary, error) {
	if sink == nil {
		sink = event.Discard
	}
	ch := newEventChannel(sink)
	r := &run{req: normalise(req), sink: ch}
	r.req.Mode = ModeIntrospect
	defer func() {
		r.close(ctx)
		ch.close()
	}()

	if err := r.readConfig(); err != nil {
		r.report(err)
		return nil, err
	}
	if err := r.discover(ctx); err != nil {
		r.report(err)
		return nil, err
	}
	if err := r.introspectStage(ctx); err != nil {
		r.report(err)
		return nil, err
	}
	return summaryOf(r.schema), nil
}

// Preview runs the pipeline as far as the plan and returns the identity of what
// it read, so that a later run can be pinned to it.
//
// It is the pass --tui makes before it opens the two screens: the operator
// reviews a classification and a plan built over one schema read of two
// resolved endpoints, and then a second pass writes the target. Without the
// value this returns nothing tied the two together — two snapshots, two walks
// of the discovery ladder, and no comparison — so the review could be of a
// database the run never touched (Reviewed, cmd/lazyslice's runTUI).
//
// It is a third entry point beside Run and Introspect for the same reason
// Introspect is a second one: a Reviewed is a document and not an event, and
// events are the only thing Run returns. It is not a second pipeline. It sets
// --plan on the request it was given and runs the same stages through the same
// run struct, stopping where PlanOnly stops, which is before the target is
// written.
func Preview(ctx context.Context, req Request, sink event.Sink) (*Reviewed, error) {
	if sink == nil {
		sink = event.Discard
	}
	ch := newEventChannel(sink)
	r := &run{req: normalise(req), sink: ch}
	// PlanOnly is set here and not only by the caller, so that Preview cannot be
	// the pass that writes a target whatever request it is handed.
	r.req.PlanOnly = true
	defer func() {
		r.close(ctx)
		ch.close()
	}()

	if _, err := r.execute(ctx); err != nil {
		r.report(err)
		return nil, err
	}
	return r.reviewed(), nil
}

// reviewed is what the preview pass read, as the run that writes compares it.
//
// The schema fingerprint is ADR-009's, filled by introspectStage from
// load.SchemaFingerprint; the classification fingerprint is the classifier's
// own verdicts, filled by classifyStage from classifierFingerprint; the two
// endpoints are the redacted dsn.Refs discover resolved. All four are
// identifiers, which is what makes them safe to carry (Reviewed).
//
// The classification fingerprint is empty for the two modes that stop before
// classify (ModeIntrospect, ModeDoctor); that is not a hole, because the run
// being pinned stops there too and never reaches the comparison.
func (r *run) reviewed() *Reviewed {
	out := &Reviewed{
		ClassFingerprint: r.classFP,
		Source:           r.sourceCand.Ref.String(),
		Target:           r.targetCand.Ref.String(),
	}
	if r.schema != nil {
		out.SchemaFingerprint = r.schema.Fingerprint
	}
	return out
}

// summaryOf is pipeline.SchemaSummary over a schema. Schema itself is never
// serialised, because it reaches Table.Samples (ARCHITECTURE.md section 2).
func summaryOf(s *pipeline.Schema) *pipeline.SchemaSummary {
	out := &pipeline.SchemaSummary{
		ServerVersion: s.ServerVersion,
		Schemas:       s.Schemas,
		Tables:        len(s.Tables),
		ForeignKeys:   len(s.FKs),
		NotRecreated:  map[string]int{},
		Fingerprint:   s.Fingerprint,
	}
	for i := range s.Tables {
		out.Columns += len(s.Tables[i].Columns)
	}
	for _, o := range s.NotRecreated {
		out.NotRecreated[o.Kind]++
	}
	return out
}

// run is one invocation. It holds what the stages hand each other, which is the
// whole of what "wiring" means here: no stage reaches into another, and nothing
// on this struct is read by anything outside this package.
type run struct {
	req  Request
	sink event.Sink

	prior *pipeline.Config

	// gate is the target's Eligibility, kept from discover because section
	// 11.2's three bound-marker warnings compare it against the secret
	// fingerprint and the classification, neither of which exists yet when the
	// gate runs (markerWarnings).
	gate pipeline.Eligibility

	source     *pg.Source
	sourceRef  dsn.Ref
	sourceCand pipeline.Candidate
	targetCand pipeline.Candidate
	// The rung each endpoint came from and the name that goes with it, as the
	// ladder handed them over (resolveEndpoints). They reach the emitted yml
	// through sourceCand and targetCand, which is why they are kept rather than
	// re-derived: internal/emit records the rung, and a run that rebuilt both
	// candidates as pipeline.FromFlag rewrote a committed `from: compose` on
	// every argument-free re-run (T-0060).
	sourceProv  pipeline.Provenance
	sourceLabel string
	targetProv  pipeline.Provenance
	targetLabel string
	target      *pg.Target
	targetPool  *pgxpool.Pool
	// lease is this run's ownership of the target: a dedicated target
	// connection holding an open transaction with pg_try_advisory_xact_lock over
	// a key derived from the target database name, taken before the gate's first
	// probe and released when the run is over (internal/pg's Lease,
	// THREAT_MODEL.md T2 amended 2026-09-14). The lock is transaction-scoped
	// because a target can be behind a transaction-pooling pooler, where a
	// session lock is left on a server connection somebody else is handed.
	// runID is the id it names itself with, and the id the marker row carries,
	// so a second run's refusal names the run that actually holds the target.
	lease    *pg.Lease
	runID    string
	reader   pipeline.Reader
	snapshot pipeline.SnapshotID
	released bool

	priv   pipeline.RolePrivileges
	schema *pipeline.Schema
	cls    *pipeline.Classification
	// classFP is the classifier's own verdicts without this run's --unmask
	// opt-outs, which is the half of the review pin a schema fingerprint cannot
	// stand for (classifierFingerprint, Reviewed.ClassFingerprint).
	classFP string
	plan    *pipeline.Plan

	key    mask.Key
	keyFP  string
	unmask map[ref.ColumnRef]string
}

// normalise fills the fields a caller may have left at zero with section 3's
// and section 8's defaults.
//
// This is where ARCHITECTURE.md section 3's defaults are substituted, and it is
// the only place: pipeline.PlanRequest cannot tell an unset Take from an
// explicit zero, so the planner takes the request as given and cmd/lazyslice
// refuses --take 0, --cap 0 and --depth 0 with exit 2 (T-CORE, 2026-09-06). A
// zero reaching here is a caller that never set the field — internal/tui, or a
// test — and it means "the default", never "none".
func normalise(req Request) Request {
	if req.Take <= 0 {
		req.Take = DefaultTake
	}
	if req.Cap <= 0 {
		req.Cap = DefaultCap
	}
	if req.Depth <= 0 {
		req.Depth = DefaultDepth
	}
	if req.RowBudget <= 0 {
		req.RowBudget = DefaultRowBudget
	}
	if req.MemoryBudget == "" {
		req.MemoryBudget = DefaultMemoryBudget
	}
	if req.ResidualProbeCap == 0 {
		req.ResidualProbeCap = DefaultResidualProbeCap
	}
	if req.ConfigPath == "" {
		req.ConfigPath = DefaultConfigPath
	}
	if req.SecretFile == "" {
		req.SecretFile = DefaultSecretFile
	}
	if req.Workdir == "" {
		if wd, err := os.Getwd(); err == nil {
			req.Workdir = wd
		}
	}
	if req.Explicit == nil {
		req.Explicit = map[string]bool{}
	}
	if req.TableCaps == nil {
		req.TableCaps = map[string]int{}
	}
	if req.Keys == nil {
		req.Keys = map[string][]string{}
	}
	if req.Unmask == nil {
		req.Unmask = map[string]string{}
	}
	return req
}

// execute is the pipeline. Each block is one stage of ARCHITECTURE.md section 1,
// in that order, and each returns early for the modes that stop at it.
func (r *run) execute(ctx context.Context) (*pipeline.Report, error) {
	if err := r.readConfig(); err != nil {
		return nil, err
	}
	if err := r.discover(ctx); err != nil {
		return nil, err
	}
	if err := r.introspectStage(ctx); err != nil {
		return nil, err
	}
	// The review pin. It is checked here because this is the first point at
	// which both halves of it exist — discover resolved the endpoints,
	// introspect computed the fingerprint — and it is before classify, before
	// the plan and before every write (checkReviewed).
	if err := r.checkReviewed(); err != nil {
		return nil, err
	}
	if r.req.Mode == ModeIntrospect || r.req.Mode == ModeDoctor {
		return nil, nil
	}
	if err := r.classifyStage(); err != nil {
		return nil, err
	}
	// The second half of the review pin. The reasons screen is built from the
	// classification and the classifier reads samples, not DDL, so the schema
	// fingerprint above cannot stand for it. It is checked here, before the
	// plan and before every write (checkReviewedClassification).
	if err := r.checkReviewedClassification(); err != nil {
		return nil, err
	}
	if r.req.Mode == ModeClassify {
		return nil, nil
	}
	if err := r.planStage(ctx); err != nil {
		return nil, err
	}
	// The classification fingerprint is recomputed here and not at classify,
	// because the plan has just changed what this run will mask with
	// (refingerprint, T-0101). It runs before emitPlanOnly, so the yml a
	// --plan writes carries the same value a writing run would.
	if err := r.refingerprint(); err != nil {
		return nil, err
	}
	if r.req.Mode == ModePlan || r.req.PlanOnly {
		return nil, r.emitPlanOnly()
	}
	report, err := r.move(ctx)
	if err != nil {
		return report, err
	}
	if err := r.emitConfig(report); err != nil {
		return report, err
	}
	return report, nil
}

// ---------- the yml ----------

// readConfig reads the committed lazyslice.yml, which is the prior for
// classification and the source of every default the operator did not pass.
func (r *run) readConfig() error {
	if r.req.Reconfigure {
		return nil
	}
	cfg, err := emit.New(emit.Options{}).Read(r.req.ConfigPath)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil
	case err != nil:
		return wrap(CodeUsage, exitUsage, err, "%s could not be read", r.req.ConfigPath)
	}
	r.prior = cfg
	r.send(event.Emit, event.Info, CodeConfigRead, event.Args{event.ArgPath: r.req.ConfigPath})
	return nil
}

// ---------- discover ----------

// resolveEndpoints fills in the endpoints the operator did not name.
//
// It is ARCHITECTURE.md section 9's ladder, and it runs here because this is
// the stage that owns discovery: cmd/lazyslice builds a Request and calls Run,
// and reaches no stage package directly (cmd/CLAUDE.md). It lived in
// cmd/lazyslice's firstRun until T-0061 moved it, which changed nothing in
// internal/discover.
//
// A run that named both endpoints walks no rung and makes no Docker call
// (ADR-008 section 1), which is every run in CI and every run the invariant
// suite makes.
//
// The ladder *chooses* for `lazyslice` with no arguments and for nothing else
// in this build. The five stage subcommands still get it printed and stop at
// exit 3 without an endpoint chosen for them: five subcommands quietly gaining
// a network-touching first run is a widening no task has asked for
// (T-DISCOVER).
func (r *run) resolveEndpoints(ctx context.Context) error {
	// An endpoint the operator named is FromFlag with no label, whether it
	// arrived from a flag, the positional DSN or internal/tui. It is set before
	// anything else because pipeline.FromYml is Provenance's zero value, and a
	// field left at it would tell internal/emit the yml named this database.
	if r.req.Source != "" {
		r.sourceProv, r.sourceLabel = pipeline.FromFlag, ""
	}
	if r.req.Target != "" {
		r.targetProv, r.targetLabel = pipeline.FromFlag, ""
	}

	if r.req.Mode != ModeRun {
		if r.req.Source == "" {
			// The ladder is walked so that it is printed, and nothing is chosen
			// from it: a subcommand names its own source or stops.
			if _, err := discover.New().Discover(ctx, r.req.Workdir, r.sink); err != nil {
				return wrap(CodeSourceNone, exitNoSource, err, "no source: pass --source postgres://...")
			}
			return stop(CodeSourceNone, exitNoSource, "no source: pass --source postgres://...")
		}
		return nil
	}
	if r.req.Source != "" && (!r.req.Mode.needsTarget() || r.req.Target != "") {
		return nil
	}

	res, err := discover.Resolve(ctx, discover.Options{
		Workdir:    r.req.Workdir,
		DockerHost: r.req.DockerHost,
		// Rung 0 is the committed file readConfig has already read; --reconfigure
		// leaves it nil, which is what makes that flag "run the first-run path".
		Config:       r.prior,
		Source:       r.req.Source,
		Target:       r.req.Target,
		NeedTarget:   r.req.Mode.needsTarget(),
		CreateTarget: r.req.CreateTarget,
		// --yes and "no controlling terminal" are one path (ADR-008 section 7).
		// Without this line the flag stops at core: a run under an allocated
		// TTY (docker run -t, script(1), tmux) opens /dev/tty and blocks in the
		// prompt with no timeout instead of taking Q1's headless refusal.
		Yes: r.req.Yes,
	}, r.sink)
	if err != nil {
		return refusalStop(err)
	}
	r.req.Source, r.sourceProv, r.sourceLabel = res.Source, res.SourceProvenance, res.SourceLabel
	r.req.Target, r.targetProv, r.targetLabel = res.Target, res.TargetProvenance, res.TargetLabel
	return nil
}

// refusalStop turns internal/discover's Refusal into a Stop.
//
// The two carry the same three things and the dependency cannot go the other
// way: core imports internal/discover, so discover cannot return a core.Stop.
// The Error event has already reached the sink from the ladder itself, and the
// line the user reads was rendered from the catalogue there, so the Stop is
// marked sent and report does not print a second one under the same code.
func refusalStop(err error) error {
	refusal, ok := discover.AsRefusal(err)
	if !ok {
		return asStop(err)
	}
	return &Stop{
		Code: refusal.Code, Exit: refusal.Exit, Args: refusal.Args,
		Message: refusal.Message, err: err, sent: true,
	}
}

// unreachableTarget is target.refused.unreachable with the two arguments its
// catalogue row templates. The row is "target {host} did not respond: {reason}"
// (internal/event/catalogue.yml), so a Stop built by wrap alone — which sets no
// Args — reaches the operator with both placeholders unfilled.
//
// The host is the redacted reference and never the DSN, and the reason is
// pg.RenderAnyError with values off, collapsed to one line by oneLine: a
// driver error that wraps four dial attempts is a wall of text under a ✗.
func unreachableTarget(target dsn.Ref, err error, message string) *Stop {
	s := wrap(pg.CodeUnreachable, exitTarget, err, "%s", message)
	s.Args = event.Args{
		event.ArgHost:   target.String(),
		event.ArgReason: oneLine(pg.RenderAnyError(err, false)),
	}
	return s
}

// oneLine collapses s to a single line without dropping the cause. A dial
// failure from pgx is multi-line: line 1 names the gate step that failed
// ("pg: gate: connecting to the target: failed to connect to `...`:") and the
// driver's own reason — the only part that says *why* — is on the lines after
// it. Taking only the first line (as a naive truncation would) keeps the
// preamble and throws the reason away, so {reason} in the catalogue's
// "target {host} did not respond: {reason}" ends up a dangling colon with
// nothing after it.
//
// Instead: strip the "pg: gate: <step>: " preamble when present, so the
// remaining text starts at the driver's own words, then join what's left
// into one line — trimmed, non-empty lines only, each stripped of a trailing
// ":" so joining does not leave "foo:; bar" — capped at two source lines
// (pgx repeats the same dial error per address it tried, so two is enough to
// carry the cause without reproducing the whole multi-address wall of text).
func oneLine(s string) string {
	const gatePrefix = "pg: gate: "
	if strings.HasPrefix(s, gatePrefix) {
		rest := s[len(gatePrefix):]
		if i := strings.IndexByte(rest, ':'); i >= 0 {
			s = rest[i+1:]
		} else {
			s = rest
		}
	}

	var parts []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimRight(line, ":")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts = append(parts, line)
		if len(parts) == 2 {
			break
		}
	}
	return strings.Join(parts, "; ")
}

// candidateOf is one endpoint as internal/emit records it: the redacted
// reference, the rung it came from and the name that goes with it.
func candidateOf(ref dsn.Ref, prov pipeline.Provenance, label string) pipeline.Candidate {
	return pipeline.Candidate{Ref: ref, Provenance: prov, Label: label, Local: ref.Loopback()}
}

// discover resolves the source and the target.
//
// The ladder runs here (resolveEndpoints), which is what lets cmd/lazyslice
// build a Request and call Run and reach no stage package of its own
// (cmd/CLAUDE.md, T-0061). A run that names both endpoints — which is every run
// in CI and every run the invariant suite makes — walks no rung at all.
func (r *run) discover(ctx context.Context) error {
	r.start(event.Discover)
	defer r.done(event.Discover)

	if err := r.resolveEndpoints(ctx); err != nil {
		return err
	}

	d, sourceRef, err := dsn.Parse(r.req.Source)
	if err != nil {
		return wrap(CodeUsage, exitUsage, err, "--source is not a Postgres connection string")
	}
	r.sourceRef = sourceRef
	r.sourceCand = candidateOf(sourceRef, r.sourceProv, r.sourceLabel)

	shapes := sourceShapes()
	src, err := pg.OpenSource(ctx, d, shapes...)
	if err != nil {
		return wrap(CodeSourceNone, exitNoSource, err, "the source would not open")
	}
	r.source = src

	priv, err := src.Privileges(ctx)
	if err != nil {
		return wrap(CodeSourceNone, exitNoSource, err, "the source role's privileges could not be read")
	}
	r.priv = priv
	r.send(event.Discover, event.Decision, CodeSourceChosen, event.Args{
		event.ArgHost:       sourceRef.Host,
		event.ArgDatabase:   sourceRef.Database,
		event.ArgRole:       priv.Role,
		event.ArgProvenance: discover.ProvenanceName(r.sourceProv, r.sourceLabel),
		event.ArgFlag:       "--source",
	})
	if len(priv.Writable) > 0 {
		if r.req.RequireReadOnlyRole {
			return stop(CodeRoleRefusedWritable, exitWritableRole,
				"the source role %s can write to %d table(s) and --require-read-only-role is set",
				priv.Role, len(priv.Writable))
		}
		r.send(event.Discover, event.Warn, CodeRoleWritable, event.Args{
			event.ArgRole:      priv.Role,
			event.ArgDatabase:  sourceRef.Database,
			event.ArgCount:     strconv.Itoa(len(priv.Writable)),
			event.ArgStatement: readOnlyRoleStatement(sourceRef),
		})
	}

	if !r.req.Mode.needsTarget() {
		return nil
	}
	return r.openTarget(ctx)
}

// openTarget opens the write side and runs the gate (ARCHITECTURE.md section 9).
func (r *run) openTarget(ctx context.Context) error {
	if r.req.Target == "" {
		return &Stop{
			Code: CodeTargetUnset, Exit: exitTarget,
			Args:    event.Args{event.ArgFlag: "--target"},
			Message: "no target: pass --target postgres://...",
		}
	}
	d, targetRef, err := dsn.Parse(r.req.Target)
	if err != nil {
		return wrap(CodeUsage, exitUsage, err, "--target is not a Postgres connection string")
	}
	r.targetCand = candidateOf(targetRef, r.targetProv, r.targetLabel)

	// The gate's end of section 11.2's binding, wired to the one definition of
	// the schema fingerprint there is (ADR-009). A Target built without it can
	// never find a marker bound, which refuses a target lazyslice itself wrote.
	tgt, err := pg.OpenTarget(ctx, d, load.GateFingerprint(introspect.New()))
	if err != nil {
		return unreachableTarget(targetRef, err, "the target would not open")
	}
	r.target = tgt

	// The run lease, before the gate's first probe. The gate's verdict is acted
	// on several stages later — introspect, classify and plan all run between it
	// and the first DROP — and until T-0130 nothing owned the target across that
	// interval, so two runs could each pass the gate and take the same database
	// apart together. A lease that cannot be taken is a refusal, never a
	// fall-through (THREAT_MODEL.md T2).
	if leaseErr := r.acquireLease(ctx, tgt, targetRef); leaseErr != nil {
		return leaseErr
	}

	systemID, err := r.source.SystemID(ctx)
	if err != nil {
		// A source that will not answer pg_control_system is not a refusal: the
		// gate falls back to comparing the endpoint, which is section 9 rule 1's
		// other half.
		systemID = ""
	}
	e, err := tgt.Gate(ctx, r.sourceRef, systemID, r.req.AllowRemoteTarget)
	if err != nil {
		if gateCode(e) == pg.CodeUnreachable {
			return unreachableTarget(targetRef, err, "the target did not pass the gate")
		}
		return wrap(gateCode(e), exitTarget, err, "the target did not pass the gate")
	}
	if e.Verdict != pipeline.Eligible {
		exit := exitTarget
		if e.Reason == pg.CodeSameDatabase {
			// Section 9 rule 1 is the one refusal that is a usage error: the
			// operator named the source twice.
			exit = exitUsage
		}
		return &Stop{
			Code: gateCode(e), Exit: exit,
			Args: event.Args{
				event.ArgHost:     targetRef.Host,
				event.ArgDatabase: targetRef.Database,
				event.ArgCount:    strconv.Itoa(e.TableCount),
				event.ArgFlag:     "--allow-remote-target",
				event.ArgRole:     r.priv.Role,
				event.ArgReason:   refusedTables(e),
			},
			Message: fmt.Sprintf("the target %s is not eligible", targetRef),
		}
	}

	r.send(event.Discover, event.Decision, CodeTargetChosen, event.Args{
		event.ArgHost:     targetRef.Host,
		event.ArgDatabase: targetRef.Database,
		event.ArgFlag:     "--target",
	})
	if e.MarkerBound {
		r.send(event.Discover, event.Info, CodeTargetTruncating, event.Args{
			event.ArgDatabase: targetRef.Database,
		})
	}
	// The verdict is kept, not consumed: section 11.2's three warnings compare
	// the marker against this run's secret fingerprint and classification, and
	// neither exists until classify and resolveKey have run. markerWarnings is
	// called from move, before the first write to the target.
	r.gate = e

	// The read side of the target. pipeline.Writer writes and registers types and
	// has no way to read (ARCHITECTURE.md section 2), and every check verify
	// makes is a read of the target, so the wiring supplies one rather than
	// letting verify report a green tick over checks that did not happen. A
	// Query method on internal/pg's writer is the proper home for it and is
	// owed there; internal/core/CLAUDE.md records the deviation.
	pool, err := pg.Connect(ctx, d, nil)
	if err != nil {
		return unreachableTarget(targetRef, err, "the target's read side would not open")
	}
	r.targetPool = pool
	return nil
}

// acquireLease takes this run's ownership of the target (ARCHITECTURE.md
// section 11.2, amended 2026-09-14).
//
// The run id is made here rather than by the marker row, because the lease
// names itself with it on the target connection before the gate runs and the
// marker row is not written until halfway through the load: a second run
// refused at the lease can then name the run that holds the target, and that
// name is the same one the marker will carry.
//
// A held lease is exit 4 with the holder named. Anything else that goes wrong
// is exit 4 too: a lease that could not be taken is not a lease that is free.
func (r *run) acquireLease(ctx context.Context, tgt *pg.Target, targetRef dsn.Ref) error {
	id, err := pg.NewRunID()
	if err != nil {
		return wrap(CodeInternal, exitInternal, err, "a run id could not be made")
	}
	r.runID = id

	lease, err := tgt.AcquireLease(ctx, id)
	if err != nil {
		var held *pg.LeaseHeld
		if errors.As(err, &held) {
			return &Stop{
				Code: pg.CodeLeaseHeld, Exit: exitTarget,
				Args: event.Args{
					event.ArgDatabase: targetRef.Database,
					event.ArgHost:     targetRef.Host,
					event.ArgReason:   holderName(held.Holder),
				},
				Message: held.Error(), err: err,
			}
		}
		// Anything else is a target that would not answer the one question the
		// lease asks, which is the gate's reachability precondition arriving a
		// statement earlier than it used to: same code, same exit, same
		// {host}/{reason} in the rendered line.
		return unreachableTarget(targetRef, err, "the target would not answer")
	}
	r.lease = lease
	return nil
}

// holderName is what the refusal prints for the run that holds the target. A
// holder that could not be identified — a role that may not see another
// session's application_name — is still a refusal; only the name is missing.
func holderName(holder string) string {
	if holder == "" {
		return "another lazyslice run"
	}
	return holder
}

// markerWarnings prints what section 11.2 requires of a bound marker written by
// a run that is not this one: `secret changed`, `classification changed` and
// `lazyslice version changed`, each meaning the masked values already in the
// target will not match the ones this run is about to write.
//
// It is called from move and not from openTarget, which is where the gate runs.
// All three comparisons need something the gate does not have: the secret
// fingerprint comes from resolveKey (the first thing move does) and the
// classification fingerprint from classifyStage, both of which run after
// discover. Called from openTarget every branch was guarded on a field that was
// still zero, so none of the three could ever print — which is the whole of what
// section 11.2 asks for, silently missing.
//
// It runs before the loader's first write, so a developer reloading a marked
// target is told before it is truncated, not after.
func (r *run) markerWarnings() {
	if !r.gate.MarkerBound {
		return
	}
	if r.gate.PrevKeyFP != "" && r.keyFP != "" && r.gate.PrevKeyFP != r.keyFP {
		r.send(event.Discover, event.Warn, CodeSecretChanged, nil)
	}
	if r.gate.PrevClassFP != "" && r.cls != nil && r.gate.PrevClassFP != r.cls.Fingerprint {
		r.send(event.Discover, event.Warn, CodeClassChanged, nil)
	}
	// The third comparison is wired and its input is not: internal/pg reads the
	// marker row and does not yet copy tool_version onto Eligibility, so
	// PrevToolVersion is empty and this prints nothing. The field is declared in
	// internal/pipeline with that owed; when pg fills it the warning starts
	// printing with no change here.
	if r.gate.PrevToolVersion != "" && r.gate.PrevToolVersion != Version {
		r.send(event.Discover, event.Warn, CodeVersionChanged, nil)
	}
}

// checkReviewed refuses a run that is not the run the operator reviewed.
//
// Request.Reviewed is set by a caller that ran Preview and then showed somebody
// the result: --tui's two screens are built from one pass and the target is
// written by a second, and the second takes its own snapshot and walks the
// discovery ladder again. A schema that changed between the two, or a ladder
// that picked a different container the second time, would leave the operator
// having approved a plan over a database this run is not writing.
//
// This is the first half of the pin: the two endpoints and the schema. The
// second half is the classification, which the classifier derives from samples
// and not from DDL, so no schema fingerprint can stand for it; it is compared
// by checkReviewedClassification, one stage later, because that is the first
// point at which it exists.
//
// All three comparisons here are made whenever Reviewed is set; none of them is
// skipped for an empty value, because "empty" is how a review that never
// happened would look and skipping it is how a pin silently stops pinning. The
// target is compared only for the two modes that open one at all
// (Mode.needsTarget), where an empty reviewed target is a fact about the mode
// and not a missing review.
//
// The refusal names what changed rather than only that something did: the
// operator's next move differs entirely between "somebody migrated the source"
// and "the ladder chose the other container".
func (r *run) checkReviewed() error {
	rev := r.req.Reviewed
	if rev == nil {
		return nil
	}
	fingerprint := ""
	if r.schema != nil {
		fingerprint = r.schema.Fingerprint
	}

	var changed []string
	if got := r.sourceCand.Ref.String(); got != rev.Source {
		changed = append(changed, "the source is "+endpointName(got)+
			", and "+endpointName(rev.Source)+" was reviewed")
	}
	if r.req.Mode.needsTarget() {
		if got := r.targetCand.Ref.String(); got != rev.Target {
			changed = append(changed, "the target is "+endpointName(got)+
				", and "+endpointName(rev.Target)+" was reviewed")
		}
	}
	if fingerprint != rev.SchemaFingerprint {
		changed = append(changed, "the source schema is not the one that was read")
	}
	if len(changed) == 0 {
		return nil
	}

	reason := strings.Join(changed, "; ")
	return &Stop{
		Code: CodeReviewedChanged, Exit: exitReviewed,
		Args:    event.Args{event.ArgReason: reason},
		Message: "this run is not the one that was reviewed: " + reason,
	}
}

// endpointName is an endpoint as the review refusal names it, with a word for
// the one that is not there: "the target is , and ... was reviewed" is a
// sentence with a hole in it, and an absent endpoint is exactly the case this
// refusal exists to report.
func endpointName(s string) string {
	if s == "" {
		return "none"
	}
	return s
}

// checkReviewedClassification refuses a run whose columns were not classified
// the way the operator was shown.
//
// The reasons screen is the masking review: it lists every column, its category
// and why. That list is not a function of the DDL. internal/classify reads
// Table.Samples, which internal/introspect draws from the snapshot with
// TABLESAMPLE SYSTEM ... REPEATABLE — identical between two passes only while
// the data is identical — so a column that reaches the mask threshold on a
// value signal alone (THREAT_MODEL.md T1's `ref` column holding emails) can
// fall below it when the second pass draws different blocks, and be copied in
// clear by the run that writes while the operator's review showed it masked.
// The schema fingerprint cannot see that: the DDL did not change.
//
// What is compared is the classifier's own verdict with this run's --unmask
// opt-outs left out (classifierFingerprint). The reasons screen writes those
// opt-outs, so folding them in would refuse the operator for doing the one
// thing that screen is for, and the yml's opt-outs are the same file on both
// passes and stay in.
//
// It runs immediately after classifyStage and therefore before the plan, before
// extract and before every write.
func (r *run) checkReviewedClassification() error {
	rev := r.req.Reviewed
	if rev == nil {
		return nil
	}
	if r.classFP == rev.ClassFingerprint {
		return nil
	}
	const reason = "the classification is not the one that was reviewed"
	return &Stop{
		Code: CodeReviewedChanged, Exit: exitReviewed,
		Args:    event.Args{event.ArgReason: reason},
		Message: "this run is not the one that was reviewed: " + reason,
	}
}

// classifierFingerprint is pipeline.Classification.Fingerprint as the
// classifier alone decided it: the committed yml as prior, and none of this
// run's --unmask flags.
//
// The flags are left out because --tui's reasons screen writes them, and the
// two passes are therefore allowed to differ by exactly that much. Everything
// else the fingerprint covers — the rule-pack version and each column's
// category and masker — is what the operator read and what the pin exists to
// hold still.
//
// With no --unmask flag the run's own prior is the committed yml, so the
// classification already on the run is that classification and no second call
// is made. markSmallDomains is not in the way: Fingerprint is computed inside
// Classify, and Domain and SmallDomain are written after it.
func (r *run) classifierFingerprint() (string, error) {
	if r.cls == nil {
		return "", nil
	}
	if len(r.req.Unmask) == 0 {
		return r.cls.Fingerprint, nil
	}
	cls, err := classify.New().Classify(r.schema, schemaSampler{schema: r.schema}, r.prior)
	if err != nil {
		return "", wrap(CodeInternal, exitInternal, err, "the columns could not be classified")
	}
	return cls.Fingerprint, nil
}

// refingerprint recomputes Classification.Fingerprint over the decisions the
// plan left behind (T-0101).
//
// ARCHITECTURE.md §5 makes the fingerprint a function of the rule-pack version
// and, per column, its category and its masker, and §11.2 prints "classification
// changed -- masked values will differ" when a bound marker's recorded one
// differs from this run's. But the masker is not final when Classify returns:
// §5's own unique-index rule says the *plan* picks the widest registered
// generator for the category, and internal/plan/unique.go writes that pick back
// onto the decision -- which internal/transform masks with and internal/emit
// records. So a column that became unique between two runs (a new unique index,
// or a --take that pushed the table past d_required) changed every masked value
// in it and changed no fingerprint, and the one warning §11.2 has for that case
// did not print.
//
// The pick cannot move into classify, which has the samples and the unique
// indexes but no row count and no database. So the fingerprint moves instead:
// it is computed here, after the plan, and it is the classification *plus* the
// plan's picks. This is the same structural reason domain.go writes Domain and
// SmallDomain after Classify returns, and internal/core/CLAUDE.md records both.
//
// Nothing changes for a run with no escalation: with the decisions Classify
// left, classify.Refingerprint returns the value Classify computed, byte for
// byte.
//
// r.classFP is deliberately *not* touched. That is the review pin's value --
// the classifier's own verdicts, which is what --tui's reasons screen showed
// somebody -- and it is compared before the plan runs, by
// checkReviewedClassification, on both passes.
func (r *run) refingerprint() error {
	if r.cls == nil {
		return nil
	}
	fp, err := classify.Refingerprint(r.cls)
	if err != nil {
		return wrap(CodeInternal, exitInternal, err, "the classification could not be fingerprinted")
	}
	r.cls.Fingerprint = fp
	return nil
}

// ---------- introspect ----------

func (r *run) introspectStage(ctx context.Context) error {
	r.start(event.Introspect)
	defer r.done(event.Introspect)

	id, err := r.source.Snapshot(ctx)
	if err != nil {
		return wrap(CodeInternal, exitInternal, err, "the source would not export a snapshot")
	}
	r.snapshot = id

	reader, err := r.source.Reader(ctx, id)
	if err != nil {
		return wrap(CodeInternal, exitInternal, err, "the source would not open a reader on the snapshot")
	}
	r.reader = reader

	schema, err := introspect.New().Introspect(ctx, reader)
	if err != nil {
		return wrap(CodeInternal, exitInternal, err, "the source catalog could not be read")
	}
	dropMarkerTable(schema)
	// ADR-009: the schema fingerprint is the DDL text internal/load/ddl
	// generates, computed here, by the caller that has both ends of section
	// 11.2's binding. Introspect leaves the field empty and nothing else may
	// fill it: a marker written with one definition and checked with another
	// binds nothing.
	fp, err := load.SchemaFingerprint(schema)
	if err != nil {
		return wrap(CodeInternal, exitInternal, err, "the schema fingerprint could not be computed")
	}
	schema.Fingerprint = fp
	r.schema = schema

	columns := 0
	for i := range schema.Tables {
		columns += len(schema.Tables[i].Columns)
	}
	r.send(event.Introspect, event.Info, CodeSchemaRead, event.Args{
		event.ArgCount:   strconv.Itoa(len(schema.Tables)),
		event.ArgVersion: strconv.Itoa(schema.ServerVersion),
		event.ArgReason:  strconv.Itoa(columns) + " columns, " + strconv.Itoa(len(schema.FKs)) + " foreign keys",
	})
	return nil
}

// notRecreated counts the source objects v1 leaves behind, per kind. It is
// section 10's `not_recreated:` block and section 11.1's promise that the count
// is always printed and never a refusal on its own.
func notRecreated(schema *pipeline.Schema) map[string]int {
	out := map[string]int{}
	for _, o := range schema.NotRecreated {
		out[o.Kind]++
	}
	return out
}

// dropMarkerTable removes lazyslice's own bookkeeping table from the catalog
// before any stage sees it.
//
// lazyslice_meta is section 11.2's marker: internal/pg creates it, internal/load
// writes it, and internal/load/ddl already refuses to recreate or fingerprint a
// source table of that name. A source that carries one is a target of some
// earlier run, and every column in it is a fingerprint, a status word or a
// count. Leaving it in the catalog has three consequences and all of them are
// wrong: the classifier scores `secret_fingerprint` as a credential and
// `lazyslice classify` pointed at a target lazyslice itself wrote reports
// lazyslice's own table as personal data; the planner gives it a step; and the
// row-count check compares a table the loader never loaded.
//
// It is filtered here rather than in internal/introspect because the catalog is
// the truth about the database and this is a fact about lazyslice: introspect
// should keep reporting what is there. Moving the filter to the gate — refusing
// a source that carries a marker outright — is what internal/load/CLAUDE.md
// records as owed, and it would make this unnecessary.
func dropMarkerTable(schema *pipeline.Schema) {
	tables := schema.Tables[:0]
	dropped := map[ref.TableRef]bool{}
	for _, t := range schema.Tables {
		if t.Ref.Name == pg.MarkerTable {
			dropped[t.Ref] = true
			continue
		}
		tables = append(tables, t)
	}
	if len(dropped) == 0 {
		return
	}
	schema.Tables = tables

	fks := schema.FKs[:0]
	for _, fk := range schema.FKs {
		if dropped[fk.Child] || dropped[fk.Parent] {
			continue
		}
		fks = append(fks, fk)
	}
	schema.FKs = fks
}

// ---------- classify ----------

func (r *run) classifyStage() error {
	r.start(event.Classify)
	defer r.done(event.Classify)

	prior, err := r.classifyPrior()
	if err != nil {
		return err
	}
	cls, err := classify.New().Classify(r.schema, schemaSampler{schema: r.schema}, prior)
	if err != nil {
		return wrap(CodeInternal, exitInternal, err, "the columns could not be classified")
	}
	// ARCHITECTURE.md section 5's small-domain rule, which nothing else in the
	// tree applies (domain.go). It runs before anything reads a decision: the
	// yml lists these columns under `small_domain:` and the residual filter
	// leaves them out, and both are downstream of here.
	markSmallDomains(r.schema, cls)
	r.cls = cls
	if r.classFP, err = r.classifierFingerprint(); err != nil {
		return err
	}

	for _, col := range sortedColumns(cls.Decisions) {
		d := cls.Decisions[col]
		code := classify.CodeColumnCopied
		if d.Masked {
			code = classify.CodeColumnMasked
		}
		r.sink.Send(event.Event{
			At: time.Now(), Stage: event.Classify, Kind: event.Decision, Code: code,
			Table: col.Table, Column: col.Column,
			Args: event.Args{
				event.ArgTable:  col.Table.String(),
				event.ArgColumn: col.Column,
				event.ArgReason: d.Reason,
			},
		})
	}
	for _, col := range cls.Drift {
		r.sink.Send(event.Event{
			At: time.Now(), Stage: event.Classify, Kind: event.Warn, Code: classify.CodeColumnDrift,
			Table: col.Table, Column: col.Column,
			Args: event.Args{
				event.ArgTable: col.Table.String(), event.ArgColumn: col.Column,
				event.ArgPath: r.req.ConfigPath,
			},
		})
	}
	for _, col := range cls.Expired {
		r.sink.Send(event.Event{
			At: time.Now(), Stage: event.Classify, Kind: event.Warn, Code: classify.CodeColumnOptOutExpired,
			Table: col.Table, Column: col.Column,
			Args: event.Args{event.ArgTable: col.Table.String(), event.ArgColumn: col.Column},
		})
	}
	if r.req.StrictSchema && len(cls.Drift) > 0 {
		return &Stop{
			Code: classify.CodeRefusedStrictSchema, Exit: exitDrift,
			Args: event.Args{
				event.ArgCount: strconv.Itoa(len(cls.Drift)),
				event.ArgPath:  r.req.ConfigPath,
				event.ArgFlag:  "--strict-schema",
			},
			Message: fmt.Sprintf("%d column(s) are not in %s", len(cls.Drift), r.req.ConfigPath),
		}
	}
	return nil
}

// classifyPrior is the committed file with the --unmask flags folded in.
//
// The flag and the file are one input to the classifier, because they are one
// question: is this column opted out? A flag opt-out carries no type
// fingerprint — it is made for this run and dies with it — and classify honours
// exactly that case by branching on `by: flag` (its honourOptOut).
func (r *run) classifyPrior() (*pipeline.Config, error) {
	r.unmask = map[ref.ColumnRef]string{}
	if len(r.req.Unmask) == 0 {
		return r.prior, nil
	}

	prior := &pipeline.Config{Columns: map[ref.ColumnRef]pipeline.ColumnConfig{}}
	if r.prior != nil {
		copied := *r.prior
		prior = &copied
		prior.Columns = make(map[ref.ColumnRef]pipeline.ColumnConfig, len(r.prior.Columns))
		for k, v := range r.prior.Columns {
			prior.Columns[k] = v
		}
	}
	for name, reason := range r.req.Unmask {
		col, err := resolveColumn(name, r.schema)
		if err != nil {
			return nil, wrap(CodeUsage, exitUsage, err, "--unmask %s", name)
		}
		cc := prior.Columns[col]
		cc.Unmask = &pipeline.Unmask{Reason: reason, By: "flag"}
		prior.Columns[col] = cc
		r.unmask[col] = reason
	}
	return prior, nil
}

// ---------- plan ----------

// virtualEvents prints one plan.polymorphic.inferred line per entry of
// p.Virtual: §3.5 requires the plan to state every virtual edge it will
// actually follow, not only the pairs it declined (ARCHITECTURE.md §3.2,
// amended 2026-09-08, T-POLY). fk.Name is "child.column" — the discriminator
// column the inference read, table-qualified by fk.Child — so the bare column
// name is read off the last segment.
func (r *run) virtualEvents(p *pipeline.Plan) {
	for _, fk := range p.Virtual {
		col := fk.Name
		if i := strings.LastIndex(fk.Name, "."); i >= 0 {
			col = fk.Name[i+1:]
		}
		r.send(event.Plan, event.Info, CodePlanPolymorphicInferred, event.Args{
			event.ArgColumn: col,
			event.ArgTable:  fmt.Sprintf("%s (%s)", fk.Child, strings.Join(fk.ChildCols, ", ")),
			event.ArgReason: fk.Parent.String(),
		})
	}
}

func (r *run) planStage(ctx context.Context) error {
	r.start(event.Plan)
	defer r.done(event.Plan)

	// ARCHITECTURE.md §11.1's not-recreatable refusal, raised where §11.1 says
	// it is raised: "at plan -- before the snapshot is used for keys and before
	// anything in the target is dropped" (T-0097).
	//
	// ddl.Recreatable is a pure function of the schema, and it used to be called
	// by load.Load as its first statement, because a stage package may not
	// import another stage package and the caller §11.1 describes is this one,
	// which did not exist when the loader landed. Nothing about the check
	// changes here; where it runs does. One of the ten schemas in
	// testdata/torture/ reaches it as the fixture stands -- Mastodon's
	// timestamp_id on nine primary keys, carried unedited for exactly that
	// reason. GitLab reaches the same refusal upstream on two objects
	// (organizations.uuid's DEFAULT gen_random_uuid_v7(), and the index
	// index_todos_coalesced_snoozed_until_created_at on timestamp_coalesce),
	// and its 43-table subset drops both, so the fixture does not
	// (testdata/torture/gitlab/README.md, docs/TORTURE.md). From inside the
	// loader an operator with either schema paid for a whole extract, holding
	// the source snapshot throughout, before being told the target could not be
	// built.
	//
	// It is before planRequest and before Plan, so it is also before the first
	// key query: the refusal costs one introspect and nothing else. asStop maps
	// *ddl.Refusal to its catalogued code and exit 13 (names.go,
	// testdata/regressions/002-function-default-refusal-uncoded.sql), and both
	// of ddl's codes carry stage: plan in internal/event/catalogue.yml, which
	// is now where they are raised.
	if err := ddl.Recreatable(r.schema); err != nil {
		return asStop(err)
	}

	req, err := r.planRequest()
	if err != nil {
		return err
	}
	p, err := plan.New().Plan(ctx, r.reader, r.schema, r.cls, req)
	if err != nil {
		return asStop(err)
	}
	p.SnapshotID = r.snapshot
	r.plan = p

	for _, s := range p.Steps {
		r.sink.Send(event.Event{
			At: time.Now(), Stage: event.Plan, Kind: event.Info, Code: CodePlanStep,
			Table: s.Table,
			Args: event.Args{
				event.ArgTable:  s.Table.String(),
				event.ArgCount:  strconv.FormatInt(stepRows(s), 10),
				event.ArgReason: modeName(s.Mode) + "; " + s.Why,
			},
		})
	}
	r.virtualEvents(p)
	for _, pair := range p.Polymorphic {
		r.send(event.Plan, event.Warn, CodePlanPolymorphic, event.Args{event.ArgReason: pair})
	}
	for _, value := range p.Unmapped {
		r.send(event.Plan, event.Warn, CodePlanUnmapped, event.Args{event.ArgReason: value})
	}
	r.send(event.Plan, event.Info, CodePlanEstimate, event.Args{
		event.ArgCount:   strconv.FormatInt(p.Estimate.Rows, 10),
		event.ArgSeconds: strconv.FormatFloat(p.Estimate.HoldSeconds, 'f', 1, 64),
		event.ArgReason: strconv.FormatInt(p.Estimate.KeyMemory>>10, 10) + " KiB of keys, " +
			strconv.FormatInt(p.Estimate.FilterMemory>>10, 10) + " KiB of residual filter",
	})
	return nil
}

// planRequest builds the planner's input from the flags and the committed file.
//
// The file supplies a value the operator did not pass, and never one they did:
// Request.Explicit is what tells the two apart. Section 10 calls the yml "a
// default the flag overrides, never a way to widen".
func (r *run) planRequest() (pipeline.PlanRequest, error) {
	req := pipeline.PlanRequest{
		Take:      r.req.Take,
		Cap:       r.req.Cap,
		Depth:     r.req.Depth,
		RowBudget: r.req.RowBudget,
		Where:     r.req.Where,
		TableCaps: map[ref.TableRef]int{},
		Keys:      map[ref.TableRef][]string{},
		Priv:      r.priv,
	}
	budget, err := emit.ParseSize(r.req.MemoryBudget)
	if err != nil {
		return req, wrap(CodeUsage, exitUsage, err, "--memory-budget")
	}
	req.MemoryBudget = budget

	if r.prior != nil {
		p := r.prior
		if !r.req.set("take") && p.Take > 0 {
			req.Take = p.Take
		}
		if !r.req.set("cap") && p.Cap > 0 {
			req.Cap = p.Cap
		}
		if !r.req.set("depth") && p.Depth > 0 {
			req.Depth = p.Depth
		}
		if !r.req.set("row-budget") && p.RowBudget > 0 {
			req.RowBudget = p.RowBudget
		}
		if !r.req.set("memory-budget") && p.MemoryBudget > 0 {
			req.MemoryBudget = p.MemoryBudget
		}
		if r.req.Where == "" {
			req.Where = p.Where
		}
		for t, n := range p.Caps {
			req.TableCaps[t] = n
		}
		for t, cols := range p.Keys {
			req.Keys[t] = cols
		}
		req.Skip = append(req.Skip, p.Skipped...)
	}

	// The withheld predicate. A file that records a fingerprint and a run that
	// passes no --where would silently slice something else, so it stops
	// (ARCHITECTURE.md section 10, THREAT_MODEL.md T5).
	if r.prior != nil && r.prior.WhereFingerprint != "" && req.Where == "" {
		return req, &Stop{
			Code: CodeConfigWhereWithheld, Exit: exitUsage,
			Args:    event.Args{event.ArgPath: r.req.ConfigPath, event.ArgFlag: "--where"},
			Message: fmt.Sprintf("%s records a withheld --where predicate; pass it again", r.req.ConfigPath),
		}
	}

	// The flags, last, so that they win.
	for name, n := range r.req.TableCaps {
		t, err := resolveTable(name, r.schema)
		if err != nil {
			return req, wrap(CodeUsage, exitUsage, err, "--cap %s", name)
		}
		req.TableCaps[t] = n
	}
	for name, cols := range r.req.Keys {
		t, err := resolveTable(name, r.schema)
		if err != nil {
			return req, wrap(CodeUsage, exitUsage, err, "--key %s", name)
		}
		req.Keys[t] = cols
	}
	for _, name := range r.req.SkipTables {
		t, err := resolveTable(name, r.schema)
		if err != nil {
			return req, wrap(CodeUsage, exitUsage, err, "--skip-table %s", name)
		}
		req.Skip = append(req.Skip, t)
	}

	switch {
	case r.req.Root != "":
		t, err := resolveTable(r.req.Root, r.schema)
		if err != nil {
			return req, wrap(plan.CodeNoRoot, exitUsage, err, "--root %s", r.req.Root)
		}
		req.Root = &t
	case r.prior != nil && r.prior.Root != (ref.TableRef{}):
		t := r.prior.Root
		req.Root = &t
	}
	return req, nil
}

// ---------- extract, transform, load, verify ----------

// move is the four stages that touch rows: extract, transform, load and verify.
//
// Extract and transform run as goroutines feeding the loader, which is what
// makes the run bounded in memory rather than bounded by the size of the slice.
// The snapshot is released when extract has finished with it, before the load
// has drained, because ARCHITECTURE.md section 1 holds it "from the start of
// introspect to the end of extract" and every second past that is an xmin pin on
// production (THREAT_MODEL.md T9).
func (r *run) move(ctx context.Context) (*pipeline.Report, error) {
	if err := r.resolveKey(); err != nil {
		return nil, err
	}
	// Section 11.2's bound-marker warnings, here because this is the first point
	// at which all three of their inputs exist and the last before the loader
	// truncates the target.
	r.markerWarnings()
	if err := r.registerShapes(); err != nil {
		return nil, err
	}

	writer, err := r.target.Writer(ctx)
	if err != nil {
		return nil, unreachableTarget(r.targetCand.Ref, err, "the target would not open a writer")
	}

	residual := smallDomainAware{
		Residual: transform.NewResidual(r.plan.Estimate.FilterMemory * 8 / residualBitsPerCell),
		exempt:   smallDomainColumns(r.cls),
	}

	// WithCancelCause and not WithCancel: transform cancels extract when a masker
	// refuses a value, and extract then returns a bare context.Canceled. Without
	// the cause there is no way to tell that cancellation from a Ctrl-C, so the
	// masking refusal — the one error that says a value could not be masked —
	// was discarded and the run reported "interrupted", exit 130.
	extractCtx, cancelExtract := context.WithCancelCause(ctx)
	defer cancelExtract(nil)

	raw := make(chan pipeline.RowBatch, batchBuffer)
	masked := make(chan pipeline.RowBatch, batchBuffer)

	r.start(event.Extract)
	extractDone := make(chan error, 1)
	go func() {
		extractErr := extract.New(r.schema).Extract(extractCtx, r.reader, r.plan, raw)
		r.releaseSnapshot(ctx)
		extractDone <- extractErr
	}()

	r.start(event.Transform)
	transformDone := make(chan error, 1)
	go func() {
		defer close(masked)
		var failure error
		for b := range raw {
			if failure != nil {
				continue // drain, so extract is never blocked on a channel nobody reads
			}
			out, maskErr := transform.New(r.schema).Transform(b, r.cls, &r.key, residual)
			if maskErr != nil {
				failure = maskErr
				cancelExtract(maskErr)
				continue
			}
			select {
			case masked <- out:
			case <-ctx.Done():
				failure = ctx.Err()
			}
		}
		transformDone <- failure
	}()

	r.start(event.Load)
	lr, loadErr := load.New(r.loadRun(), r.sink).Load(ctx, writer, r.plan, r.schema, masked)
	extractErr := <-extractDone
	transformErr := <-transformDone
	r.done(event.Extract)
	r.done(event.Transform)
	r.done(event.Load)

	// Extract stopped because transform refused: the refusal is the reason, and
	// the cancellation is this code's own doing. Without this the masking
	// refusal — its code, its exit and the column it names — was thrown away in
	// favour of a bare context.Canceled, which asStop maps to exit 130 and
	// cmd/lazyslice prints as "interrupted". context.Cause is what tells the two
	// cancellations apart; a Ctrl-C leaves the cause a context.Canceled and falls
	// through to the ordinary path.
	if extractErr != nil && errors.Is(extractErr, context.Canceled) {
		if cause := context.Cause(extractCtx); cause != nil && !errors.Is(cause, context.Canceled) {
			extractErr = nil
		}
	}

	// The order is the order the failures happen in: a load that failed because
	// extract stopped feeding it must report extract's reason, not its own.
	switch {
	case extractErr != nil:
		return nil, asStop(extractErr)
	case transformErr != nil:
		return nil, asStop(transformErr)
	case loadErr != nil:
		return nil, asStop(loadErr)
	}
	if violation := r.source.Violation(); violation != nil {
		return nil, wrap(CodeSourceViolation, exitExtractLoad, violation,
			"the source's statement allowlist refused a statement this run sent")
	}

	r.start(event.Verify)
	report, verifyErr := verify.New(verify.Options{ProbeCap: r.req.ResidualProbeCap}).Verify(
		ctx, r.source, readableWriter{Writer: writer, pool: r.targetPool},
		r.schema, r.plan, r.cls, residual, lr,
	)
	r.closeRun(ctx, writer, lr, verifyErr)
	r.done(event.Verify)
	if verifyErr != nil {
		return report, asStop(verifyErr)
	}
	return report, nil
}

// closeRun is core's half of T-0133 (THREAT_MODEL.md T8, amended 2026-09-14,
// docs/reviews/2026-09-09 finding 4): the marker row says complete only once
// verify has actually passed, and failed after any verify failure. load.Load no
// longer closes the row on success — it leaves it at StatusRunning, which
// already authorises the next run to truncate exactly as a run killed mid-copy
// does (internal/pg/marker.go has the reasoning) — so this is the only place
// that writes StatusComplete at all, and the only place downstream of Load that
// writes StatusFailed.
//
// It is not called when Load itself failed: move returns before reaching verify
// on that path, and Load's own failure branch already closed the row to
// StatusFailed (nothing about that changed here).
//
// On a residual-class failure — a *verify.Refusal whose Exit is 9: the residual
// scan, an unconfirmable hit, or the second net — the target holds personal
// data by definition, so this also drops every table the run just loaded before
// closing the row, rather than leaving that data on disk until the next run's
// gate truncates it (load.DropLoaded). Exit 8 (a foreign key) and exit 7 (a row
// count or a sequence) are not that: the loaded rows are what an operator
// diagnoses the failure against, so they stay, exactly as the task names it.
//
// Both load.DropLoaded and pg.FinishRun run on a context of this function's
// own, detached from ctx and bounded by load.MarkerCloseTimeout — the same
// thing load.Load's own close does on its failure path (load.go), and for the
// same reason (2026-09-14 review finding 2): a run whose ctx is already
// cancelled by the time closeRun is reached — Ctrl-C, a deadline, exactly the
// state a residual scan that itself blew a deadline leaves behind — still has
// a target to empty and a marker row to close, and the signal that ended the
// run must not also cancel the cleanup that follows it. Passing ctx here used
// to mean every statement DropLoaded and FinishRun issued failed immediately
// with context.Canceled: the drop never ran, the confirmed leak stayed in the
// target, and the marker was left at running rather than failed.
func (r *run) closeRun(ctx context.Context, w pipeline.Writer, lr *pipeline.LoadResult, verifyErr error) {
	if r.runID == "" {
		return
	}
	closing, cancel := context.WithTimeout(context.WithoutCancel(ctx), load.MarkerCloseTimeout)
	defer cancel()

	status := pg.StatusComplete
	if verifyErr != nil {
		status = pg.StatusFailed
		var refusal *verify.Refusal
		if errors.As(verifyErr, &refusal) && refusal.Exit == exitResidual {
			if dropErr := load.DropLoaded(closing, w, r.schema, r.sink); dropErr != nil {
				// DropLoaded is best-effort across the whole table list
				// (2026-09-14 review finding 1): it returns every table's
				// failure, joined, rather than the first, so every one gets
				// its own warning here — a single warning naming only the
				// first failure would tell a transcript reader far less is
				// still in the target than actually is.
				for _, one := range joinedErrors(dropErr) {
					var lr *load.Refusal
					table := ref.TableRef{}
					if errors.As(one, &lr) {
						table = lr.Table
					}
					r.send(event.Verify, event.Warn, CodeQuarantineFailed, event.Args{event.ArgTable: table.String()})
				}
			}
		}
	}
	//nolint:errcheck // Best effort, the same as load.Load's own close on its
	// failure path: a marker this cannot close stays at StatusRunning, which
	// authorises the next run to truncate exactly as StatusFailed does
	// (ARCHITECTURE.md section 11.2), so there is nothing further to do with
	// the error, and the verify failure (when there is one) is already what
	// this run reports and exits non-zero for.
	pg.FinishRun(closing, w, r.runID, status, load.TotalRows(lr))
}

// joinedErrors returns the constituent errors of an error built by
// errors.Join (load.DropLoaded's return, once it failed on more than zero
// tables), or err itself as a single-element slice when it was not one — so a
// caller iterating the failures never has to know which shape DropLoaded chose
// this time.
func joinedErrors(err error) []error {
	if u, ok := err.(interface{ Unwrap() []error }); ok {
		return u.Unwrap()
	}
	return []error{err}
}

// loadRun is what the marker table records (ARCHITECTURE.md section 11.2).
func (r *run) loadRun() load.Run {
	systemID := ""
	if id, err := r.source.SystemID(context.Background()); err == nil {
		systemID = id
	}
	return load.Run{
		RunID:                     r.runID,
		ToolVersion:               Version,
		SourceFingerprint:         r.sourceRef.Fingerprint(),
		SourceSystemID:            systemID,
		ClassificationFingerprint: r.cls.Fingerprint,
		SecretFingerprint:         r.keyFP,
		// What the gate approved, carried to the loader so that each drop can
		// re-verify it under its own lock (ARCHITECTURE.md section 11.2).
		MarkerBound:  r.gate.MarkerBound,
		MarkerRunID:  r.gate.MarkerRunID,
		MarkerStatus: r.gate.MarkerStatus,
	}
}

// registerShapes adds the statement shapes extract and verify will send to the
// source allowlist. They are built from the plan, so they cannot be registered
// when the pool is opened; a statement whose shape is not registered never
// reaches the server (THREAT_MODEL.md T9).
func (r *run) registerShapes() error {
	var shapes []pg.Shape
	for _, s := range extract.Shapes(r.plan) {
		shapes = append(shapes, pg.Shape{Name: s.Name, SQL: s.SQL})
	}
	for _, s := range verify.Shapes(r.plan, r.cls) {
		shapes = append(shapes, pg.Shape{Name: s.Name, SQL: s.SQL})
	}
	if err := r.source.Register(shapes...); err != nil {
		return wrap(CodeInternal, exitInternal, err, "the source allowlist would not take the run's shapes")
	}
	return nil
}

// ---------- emit ----------

func (r *run) emitConfig(report *pipeline.Report) error {
	r.start(event.Emit)
	defer r.done(event.Emit)

	if r.req.NoConfig {
		return nil
	}
	cfg, err := r.buildConfig(report)
	if err != nil {
		return err
	}
	if err := r.emitter().Write(r.req.ConfigPath, cfg); err != nil {
		return wrap(CodeInternal, exitInternal, err, "lazyslice.yml could not be written")
	}
	r.send(event.Emit, event.Info, CodeConfigWritten, event.Args{event.ArgPath: r.req.ConfigPath})
	return nil
}

// emitPlanOnly is --plan's own emit: nothing is written unless --config was
// given explicitly, and then with plan_only: true and no snapshot_id
// (ARCHITECTURE.md section 10).
func (r *run) emitPlanOnly() error {
	r.start(event.Emit)
	defer r.done(event.Emit)

	if r.req.NoConfig || !r.req.set("config") {
		return nil
	}
	cfg, err := r.buildConfig(nil)
	if err != nil {
		return err
	}
	cfg.PlanOnly = true
	cfg.SnapshotID = ""
	if err := r.emitter().Write(r.req.ConfigPath, cfg); err != nil {
		return wrap(CodeInternal, exitInternal, err, "lazyslice.yml could not be written")
	}
	r.send(event.Emit, event.Info, CodeConfigWritten, event.Args{event.ArgPath: r.req.ConfigPath})
	return nil
}

func (r *run) emitter() pipeline.Emitter {
	return emit.New(emit.Options{
		Tool:              Version,
		SchemaFingerprint: r.schema.Fingerprint,
		Prior:             r.prior,
		Unmask:            r.unmask,
		PasswordCommand:   r.req.PasswordCommand,
		NotRecreated:      notRecreated(r.schema),
	})
}

func (r *run) buildConfig(report *pipeline.Report) (*pipeline.Config, error) {
	req, err := r.planRequest()
	if err != nil {
		return nil, err
	}
	cfg, err := r.emitter().Emit(r.plan, r.cls, report, req, r.sourceCand, r.targetCand, r.keyFP)
	if err != nil {
		return nil, wrap(CodeInternal, exitInternal, err, "lazyslice.yml could not be built")
	}
	return cfg, nil
}

// ---------- the masking key ----------

// resolveKey finds the masking key, in the order ARCHITECTURE.md section 8 and
// section 9 give: $LAZYSLICE_SECRET, then --secret-file, then a new key written
// to that file — but only where section 9's repository rules allow it.
func (r *run) resolveKey() error {
	if text, ok := os.LookupEnv("LAZYSLICE_SECRET"); ok {
		// The environment variable is the one branch with no file, so section 9's
		// repository rules have nothing to protect: the key is not on disk, and
		// .gitignore cannot ignore what is not there.
		k, err := mask.ParseKey(text)
		if err != nil {
			return wrap(CodeSecretRefusedKey, exitCredential, err, "$LAZYSLICE_SECRET is not a key")
		}
		r.key, r.keyFP = k, k.Fingerprint()
		return nil
	}

	// Section 9 "The repository" runs before the file is written *or used*, not
	// only before it is created. A lazyslice.secret that is already in the
	// worktree is exactly the state of a clone whose key was committed, and that
	// is the case the tracked check exists for (THREAT_MODEL.md T6): checking
	// only on the create path made it fire solely for a key that is in the index
	// and missing from the worktree, which is nobody's repository.
	state, protectErr := repo.Protect(r.req.SecretFile, nil)
	if protectErr != nil {
		return wrap(CodeInternal, exitInternal, protectErr, "the repository could not be checked")
	}
	for _, added := range state.Added {
		r.send(event.Transform, event.Info, CodeGitignoreAdded, event.Args{event.ArgPath: added})
	}
	if len(state.Tracked) > 0 {
		return &Stop{
			Code: CodeSecretTracked, Exit: exitUsage,
			Args: event.Args{
				event.ArgPath:      state.Tracked[0],
				event.ArgStatement: repo.RemoveFromIndex(state.Tracked[0]),
			},
			Message: fmt.Sprintf("%s is tracked by git", state.Tracked[0]),
		}
	}
	if state.Root != "" && !state.GitFound {
		r.send(event.Transform, event.Warn, CodeGitAbsent, event.Args{event.ArgPath: r.req.SecretFile})
	}

	body, err := os.ReadFile(r.req.SecretFile)
	if err == nil {
		k, parseErr := mask.ParseKey(string(body))
		if parseErr != nil {
			return wrap(CodeSecretRefusedKey, exitCredential, parseErr, "%s is not a key", r.req.SecretFile)
		}
		r.key, r.keyFP = k, k.Fingerprint()
		return nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return wrap(CodeSecretRefusedKey, exitCredential, err, "%s could not be read", r.req.SecretFile)
	}

	// No key yet, and the repository has already said whether one may be written.
	k, err := mask.NewKey()
	if err != nil {
		return wrap(CodeInternal, exitInternal, err, "a masking key could not be generated")
	}
	r.key, r.keyFP = k, k.Fingerprint()

	if !state.MayWriteSecret() {
		// Section 9 step 3: the key is ephemeral, and --require-key makes that
		// exit 5 rather than a run whose masking nobody can reproduce.
		if r.req.RequireKey {
			return stop(CodeSecretRefusedKey, exitCredential,
				"%s cannot be protected by .gitignore and --require-key is set", r.req.SecretFile)
		}
		r.send(event.Transform, event.Warn, CodeSecretEphemeral, event.Args{event.ArgPath: r.req.SecretFile})
		return nil
	}
	if err := os.WriteFile(r.req.SecretFile, []byte(hexKey(k)+"\n"), 0o600); err != nil {
		return wrap(CodeSecretRefusedKey, exitCredential, err, "%s could not be written", r.req.SecretFile)
	}
	r.send(event.Transform, event.Info, CodeSecretWritten, event.Args{event.ArgPath: r.req.SecretFile})
	if state.Root == "" {
		r.send(event.Transform, event.Warn, CodeSecretUnprotected, event.Args{event.ArgPath: r.req.SecretFile})
	}
	return nil
}

// ---------- events and teardown ----------

func (r *run) start(s event.Stage) {
	r.sink.Send(event.Event{At: time.Now(), Stage: s, Kind: event.StageStart, Code: event.CodeStageStart})
}

func (r *run) done(s event.Stage) {
	r.sink.Send(event.Event{At: time.Now(), Stage: s, Kind: event.StageDone, Code: event.CodeStageDone})
}

func (r *run) send(s event.Stage, k event.Kind, code event.Code, args event.Args) {
	r.sink.Send(event.Event{At: time.Now(), Stage: s, Kind: k, Code: code, Args: args})
}

// report sends the Error event for a refusal. It is the only place a run's
// failure becomes an event, so an exit code and the line that explains it can
// never disagree.
func (r *run) report(err error) {
	var s *Stop
	if !errors.As(err, &s) {
		s = wrap(CodeInternal, exitInternal, err, "%s", err.Error())
	}
	if s.sent {
		// The ladder already sent this one (refusalStop). One refusal is one
		// line, whichever package rendered it.
		return
	}
	r.sink.Send(event.Event{
		At: time.Now(), Kind: event.Error, Code: s.Code, Exit: s.Exit,
		Table: s.Table, Column: s.Column, Args: s.Args,
	})
}

// releaseSnapshot ends the holder transaction. It is idempotent: extract's
// goroutine calls it as soon as it is done with the snapshot, and close calls it
// again on the paths that never reached extract.
func (r *run) releaseSnapshot(ctx context.Context) {
	if r.released || r.source == nil {
		return
	}
	r.released = true
	if r.reader != nil {
		_ = r.reader.Close(context.WithoutCancel(ctx))
		r.reader = nil
	}
	// A release that fails has nowhere to go: the run is over, and the holder
	// transaction ends with the connection when the pool closes.
	_ = r.source.Release(context.WithoutCancel(ctx)) //nolint:errcheck // teardown; the pool closes next
}

// close gives every connection back, whichever way the run ended.
func (r *run) close(ctx context.Context) {
	r.releaseSnapshot(ctx)
	// The lease is given up before the pool it lives on is closed, and last of
	// the target's business: it is held "until the marker is finished"
	// (ARCHITECTURE.md section 11.2), and the marker is closed inside Load, so by
	// the time close runs there is nothing left of this run that writes.
	if r.lease != nil {
		r.lease.Release(context.WithoutCancel(ctx))
		r.lease = nil
	}
	if r.targetPool != nil {
		r.targetPool.Close()
	}
	if r.target != nil {
		r.target.Close()
	}
	if r.source != nil {
		r.source.Close()
	}
}

// ---------- adapters ----------

// readableWriter is pipeline.Writer with the Query method verify needs. See
// openTarget: the read side of the target is not on section 2's Writer, and
// verify refuses to run without it rather than reporting a green tick over
// checks that did not happen.
type readableWriter struct {
	pipeline.Writer
	pool *pgxpool.Pool
}

func (w readableWriter) Query(ctx context.Context, sql string, args ...any) (pipeline.Rows, error) {
	return w.pool.Query(ctx, sql, args...)
}

// smallDomainAware is the residual filter with the small-domain columns left out
// of it (ARCHITECTURE.md section 6 item 6).
//
// Section 6 item 1 adds every masked cell to the filter and section 6 item 3
// makes a confirmed hit exit 9. A column whose admissible domain is small
// breaks that pairing, and testdata/nasty.sql's people.marital_status is the
// worked example: section 5 collapses a small-domain special category to one
// fixed enum label, so *every* value in the target equals a real value of the
// source by construction, every one of them is a filter hit, and every one
// confirms. The run would refuse itself for doing exactly what section 5 tells
// it to do.
//
// The rule that resolves it is section 6 item 6's own list of stated false
// negatives, which names "masked columns with a small admissible domain, where
// the substitution is recoverable by frequency" and "a masked value coinciding
// with another row's real value". A match on such a column is evidence about
// the size of the domain and not about the masker, which is the argument
// internal/transform already made when it kept boolean and number JSON leaves
// out of the filter. internal/invariants reads it the same way: I2's
// assertMaskedValuesAreNew exempts every column the yml lists under
// `small_domain:` from the "target and source values must not overlap" check.
//
// It is a wrapper here because internal/core is the wiring that owns both ends —
// the classification that says which columns they are, and the filter the two
// stages share. Its proper home is internal/transform, which builds the filter
// and already excludes two other small domains from it; moving it there is owed
// and recorded in internal/core/CLAUDE.md.
type smallDomainAware struct {
	pipeline.Residual
	exempt map[ref.ColumnRef]bool
}

// Add records a masked cell unless its column's domain is small.
func (f smallDomainAware) Add(col ref.ColumnRef, path string, canonical []byte) {
	if f.exempt[col] {
		return
	}
	f.Residual.Add(col, path, canonical)
}

// smallDomainColumns is every masked column the classifier marked small-domain.
func smallDomainColumns(cls *pipeline.Classification) map[ref.ColumnRef]bool {
	out := map[ref.ColumnRef]bool{}
	for col, d := range cls.Decisions {
		if d.Masked && d.SmallDomain {
			out[col] = true
		}
	}
	return out
}

// schemaSampler is pipeline.Sampler over the rows introspect already took. The
// classifier issues no SQL of its own (ARCHITECTURE.md section 4), so this is
// how a sample reaches it.
type schemaSampler struct{ schema *pipeline.Schema }

func (s schemaSampler) Samples(c ref.ColumnRef) []any {
	for i := range s.schema.Tables {
		t := &s.schema.Tables[i]
		if t.Ref != c.Table {
			continue
		}
		idx := -1
		for j, col := range t.Columns {
			if col.Name == c.Column {
				idx = j
				break
			}
		}
		if idx < 0 {
			return nil
		}
		out := make([]any, 0, len(t.Samples))
		for _, row := range t.Samples {
			if idx < len(row) {
				out = append(out, row[idx])
			}
		}
		return out
	}
	return nil
}

// sortedColumns is the decision map in (schema, table, column) order. Nothing
// iterates a Go map: two runs over one snapshot print the same lines in the same
// order, which is half of what makes the yml a record of what happened.
func sortedColumns(m map[ref.ColumnRef]pipeline.Decision) []ref.ColumnRef {
	out := make([]ref.ColumnRef, 0, len(m))
	for c := range m {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Less(out[j]) })
	return out
}
