// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime/debug"
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
func Run(ctx context.Context, req Request, sink event.Sink) (report *pipeline.Report, err error) {
	if sink == nil {
		sink = event.Discard
	}
	ch := newEventChannel(sink)
	r := &run{req: normalise(req), sink: ch}
	defer func() {
		// Checked, and reported, before ch.close(): a sink panic is recovered
		// on the drain goroutine (eventChannel's own recover) rather than left
		// to crash the process, and if execute otherwise succeeded that panic
		// is the only thing wrong with the run and must still turn it into a
		// failure, not a silent green one (2026-09-14 review of T-FAILUX,
		// finding 1). r.report sends the Error event through r.sink, which is
		// ch — doing that ahead of ch.close() is what lets the event still
		// reach the sink at all, rather than the closed channel silently
		// dropping it (see eventChannel.Send's doc).
		if perr := ch.panicked(); perr != nil && err == nil {
			err = asStop(perr)
			r.report(err)
		}
		r.close(ctx)
		ch.close()
	}()

	report, err = r.execute(ctx)
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
func Introspect(ctx context.Context, req Request, sink event.Sink) (summary *pipeline.SchemaSummary, err error) {
	if sink == nil {
		sink = event.Discard
	}
	ch := newEventChannel(sink)
	r := &run{req: normalise(req), sink: ch}
	r.req.Mode = ModeIntrospect
	defer func() {
		// Same promotion as Run's defer, and for the same reason: a panic in a
		// caller-supplied sink is recovered on the drain goroutine, and without
		// this it is simply never reported — introspect returns its summary and
		// nil error, exit 0, whatever the sink's panic actually was (2026-09-14
		// review of T-FAILUX, finding 2).
		if perr := ch.panicked(); perr != nil && err == nil {
			err = asStop(perr)
			r.report(err)
		}
		r.close(ctx)
		ch.close()
	}()

	if err = r.readConfig(); err != nil {
		r.report(err)
		return nil, err
	}
	if err = r.discover(ctx); err != nil {
		r.report(err)
		return nil, err
	}
	if err = r.introspectStage(ctx); err != nil {
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
func Preview(ctx context.Context, req Request, sink event.Sink) (reviewed *Reviewed, err error) {
	if sink == nil {
		sink = event.Discard
	}
	ch := newEventChannel(sink)
	r := &run{req: normalise(req), sink: ch}
	// PlanOnly is set here and not only by the caller, so that Preview cannot be
	// the pass that writes a target whatever request it is handed.
	r.req.PlanOnly = true
	defer func() {
		// Same promotion as Run's defer, and for the same reason: a panic in a
		// caller-supplied sink is recovered on the drain goroutine, and without
		// this it is simply never reported — preview returns its Reviewed and
		// nil error, exit 0, whatever the sink's panic actually was (2026-09-14
		// review of T-FAILUX, finding 2).
		if perr := ch.panicked(); perr != nil && err == nil {
			err = asStop(perr)
			r.report(err)
		}
		r.close(ctx)
		ch.close()
	}()

	if _, err = r.execute(ctx); err != nil {
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
	if r.plan != nil {
		out.Root = r.plan.Root
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
	// targetContainerID is discover.Result.TargetContainerID: the real
	// container name or ID behind the target, set only when the ladder found
	// or provisioned that container itself (rungs 3 and 4, and Q1/Q1'
	// adoption). It is empty for a rung-0 or --target-named target, including
	// one whose targetLabel came from a committed lazyslice.yml's compose
	// service name — that label is not a valid `docker exec` argument for a
	// container whose real name differs (T-0320 fix round, review finding
	// 1). targetConnectLine keys the docker-exec line on this, never on
	// targetLabel.
	targetContainerID string
	// targetNamed is discover.Result.TargetNamed (resolveEndpoints): true when
	// the operator named the target themselves — --target, the positional DSN,
	// or a committed lazyslice.yml's target: block — false when the ladder
	// picked it. It is what openTarget's gate step keys the T-0184 escalation
	// on (ADR-013 review finding 1), rather than targetProv: rung 0 carries the
	// *file's own* provenance forward (FromEnvVar, FromContainer, FromCompose —
	// see discover.Result's doc comment), never pipeline.FromYml, so a target
	// a committed yml named and a target the ladder chose can carry the exact
	// same targetProv and are distinguishable only by this field. It is always
	// set — false, not the zero value's ambiguity — whenever resolveEndpoints
	// ran the ladder at all; a run that named both endpoints outright sets it
	// via the FromFlag branches above instead and never calls discover.Resolve.
	targetNamed bool
	// headless is discover.Headless(opts) from the same discover.Options the
	// ladder was resolved with (resolveEndpoints) — --yes OR no controlling
	// terminal, ADR-008 §7's own definition. It is what openTarget's gate
	// step asks before escalating Eligibility.SameCluster on a ladder-chosen
	// target (T-0184, ADR-013 review finding 3); it is false, and unused,
	// whenever both endpoints were named and the ladder never ran.
	headless bool
	// askedQ1 is discover.Result.Asked (resolveEndpoints): true when the
	// ladder actually put Q1 or Q1' to the controlling terminal this run,
	// whatever the answer. rootQuestion (ADR-008 §6's Q2) reads it so the run
	// asks at most one blocking question: it is false, correctly, for every
	// mode that never calls discover.Resolve at all (resolveEndpoints's own
	// early return for anything but ModeRun) — those modes never asked Q1 or
	// Q1' because no ladder walk with a target-shaped decision ever ran.
	askedQ1 bool
	// qRoot is Q2's own answer when it is a genuine override — named at the
	// prompt, whether typed directly or after a "?" — kept as a resolved
	// ref.TableRef rather than fed back through r.req.Root as a re-rendered
	// "schema.name" string: ref.TableRef.String() does not quote, so a table
	// or schema name containing a dot round-tripped through it and a second,
	// disagreeing parse in planRequest could split it on the wrong dot
	// (T-0271 review). planRequest prefers it over r.req.Root, the same way
	// it already prefers r.prior.Root. nil whenever Q2 took the default,
	// asked nothing, or was never reached.
	qRoot      *ref.TableRef
	target     *pg.Target
	targetPool *pgxpool.Pool
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

	priv pipeline.RolePrivileges
	// replica is discover's one read of Source.Replica (T-0255 fix round,
	// review): the header warning, the headless refusal and openTarget's
	// SetSourceReplica all rest on this single answer rather than each
	// re-reading the source, which could disagree with itself between calls
	// (a promotion, a dropped connection) and left the gate comparing under a
	// stale or silently-false standby answer while the header printed the
	// true one.
	replica pg.ReplicaStatus
	schema  *pipeline.Schema
	cls     *pipeline.Classification
	// classFP is the classifier's own verdicts without this run's --unmask
	// opt-outs, which is the half of the review pin a schema fingerprint cannot
	// stand for (classifierFingerprint, Reviewed.ClassFingerprint).
	classFP string
	plan    *pipeline.Plan

	key    mask.Key
	keyFP  string
	unmask map[ref.ColumnRef]string
	// masks is every column this run was asked to mask, by --mask or by a
	// `mask:` block in the committed yml (T-0319), filled by classifyPrior and
	// read by checkMasks and, for the flag's half, by emitter.
	masks map[ref.ColumnRef]maskRequest
	// phoneRegion is the resolved --phone-region / phone_region this run
	// classified with (T-0221), filled by classifyPrior: r.req.PhoneRegion
	// when the flag was given, else the committed yml's own value, else "".
	// internal/emit and internal/verify both read it from here rather than
	// from r.req directly, so a re-run with no flag still classifies and
	// verifies under the region the committed file already recorded.
	phoneRegion string
	// typeAllow is --allow-type-literal TYPE=REASON merged with the committed
	// yml's own types: block (pipeline.Config.Types), filled by planRequest and
	// read by emitter: the record internal/emit writes back verbatim, exactly
	// as unmask is for columns. A prior entry whose TypeFP no longer matches
	// the type's current fingerprint is left out here, which is how it expires
	// (ARCHITECTURE.md §11.1's fourth arm; the same tighten-only rule ADR-004
	// states for Unmask).
	typeAllow map[string]pipeline.TypeAllow
	// typeExpired names, in the committed yml's types: block, every entry
	// planRequest did not carry forward into typeAllow because its recorded
	// fingerprint no longer matches (or the type is gone) — the type-literal
	// counterpart of classify.Classification.Expired, filled by the same pass
	// that fills typeAllow and read once, by planStage, to send
	// CodeTypeLiteralOptOutExpired.
	typeExpired []string

	// extractDone and transformDone are move's own join channels, kept here
	// rather than as move's local variables so that close can join them too.
	// move's ordinary joins (<-r.extractDone, <-r.transformDone near the end of
	// move) are ordinary statements a panic skips; without this, a panic
	// unwinding out of move left close free to release the snapshot and close
	// the target pools while extract or transform was still running against
	// them (2026-09-14 review of T-FAILUX, finding 2). Both are nil except
	// between the point move starts the goroutines and the point it joins them
	// — normally, that is entirely within move, and close's own join is a
	// no-op; only a panic in between leaves one or both non-nil for close to
	// wait on.
	extractDone   chan error
	transformDone chan error

	// abortStages cancels the pipeline context that both extract's extractCtx
	// and transform's masked-send select are derived from, so that close can
	// unblock the stage goroutines on the panic-unwind path (2026-09-14 review
	// of T-FAILUX, finding 1). A panic in load.Load, on the main goroutine,
	// unwinds through move and runs its `defer cancelExtract(nil)`, which
	// unblocks extract — but transform's `select { case masked <- out: case
	// <-ctx.Done(): }` was selecting on Run's outer ctx, never cancelled by
	// that defer, so with batchBuffer at 2 transform fills masked within two
	// batches and blocks forever, and close's own joins on extractDone and
	// transformDone then block forever too. close calls abortStages before
	// those joins so transform's select always has somewhere to unblock to;
	// nil until move sets it, and safe to call more than once (a no-op on the
	// ordinary path, where move's own defer already cancelled the same
	// context).
	abortStages context.CancelFunc
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
	if req.Mask == nil {
		req.Mask = map[string]string{}
	}
	if req.AllowTypeLiterals == nil {
		req.AllowTypeLiterals = map[string]string{}
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
	// ADR-008's Q2, the root table question: every mode still running at this
	// point goes on to plan (ModeIntrospect, ModeDoctor and ModeClassify have
	// already returned above), which is one of rootQuestion's own
	// preconditions. It runs after introspect (r.schema exists) and before the
	// plan asks internal/plan's own defaultRoot for the same answer silently.
	if err := r.rootQuestion(); err != nil {
		return nil, err
	}
	// The masking key, resolved ahead of the plan stage (T-0161): §11.1 arm 1
	// masks a masked column's DEFAULT at plan, in internal/plan/ddlliteral.go,
	// and needs the key there rather than at move, a stage later.
	if err := r.keyBeforePlan(); err != nil {
		return nil, err
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
	if r.planOnly() {
		return nil, r.emitPlanOnly()
	}
	report, err := r.move(ctx)
	if err != nil {
		return report, err
	}
	if err := r.emitConfig(report); err != nil {
		return report, err
	}
	// T-0320: the last two lines of every green run — this point is only
	// reached once move (extract, transform, load, verify) and emitConfig
	// have all returned with no error, which is exactly "a green run" for
	// dogfood session 1's own report.
	r.verifySummary(report)
	r.targetConnectLine()
	return report, nil
}

// ---------- T-0320: the verify summary and target lines ----------

// verifySummary sends CodeVerifySummary, one line folding every check
// ARCHITECTURE.md section 6 ran into a sentence: dogfood session 1 found that
// a green run said nothing about foreign keys validating, row counts
// matching, the residual scan or the second net — the per-check codes
// (reportVerifyRefusals) only ever announce a *failure*, never a count on the
// green path. report is never nil here: it is move's own return value on its
// only success path (verify.Verify's report, after verifyErr == nil).
func (r *run) verifySummary(report *pipeline.Report) {
	// Keyed on Code, not Name: "row_count" and "residual" each carry more
	// than one Passed entry — counts.go's per-table report() alongside its
	// own final pass(), and residual.go's report() (CodeUnconfirmed,
	// CodeResidualExplained) alongside its own pass() — so matching on Name
	// alone and keeping "the last one" depended on report() always being
	// appended before pass(), an ordering nothing enforces (T-0320 fix round,
	// review finding 2). The four *Passed codes are each written exactly
	// once, by the check's own final pass() call.
	var fk, tablesChecked, residual, secondNet int64
	for _, c := range report.Checks {
		if !c.Passed {
			continue // a green run has no failing check; belt and suspenders
		}
		switch c.Code {
		case verify.CodeFKPassed:
			fk = c.Count
		case verify.CodeRowCountPassed:
			tablesChecked = c.Count
		case verify.CodeResidualPassed:
			residual = c.Count
		case verify.CodeSecondNetPassed:
			secondNet = c.Count
		default:
			// Every other Passed code (report()'s per-table entries, and
			// every other check's own pass()) contributes nothing to this
			// summary line.
		}
	}
	var rows int64
	for _, n := range report.Rows {
		rows += n
	}
	r.send(event.Emit, event.Info, CodeVerifySummary, event.Args{
		event.ArgFKCount:       strconv.FormatInt(fk, 10),
		event.ArgTableCount:    strconv.FormatInt(tablesChecked, 10),
		event.ArgRowCount:      strconv.FormatInt(rows, 10),
		event.ArgResidualCount: strconv.FormatInt(residual, 10),
		event.ArgColumnCount:   strconv.FormatInt(secondNet, 10),
	})
}

// targetConnectLine sends one of the three T-0320 target-connect codes: host,
// port, database and user for the target this run just wrote, and where its
// password lives — never the password (THREAT_MODEL.md T5). r.targetCand.Ref
// is filled by openTarget before the gate runs, so it is always set by the
// time move has returned successfully.
func (r *run) targetConnectLine() {
	tref := r.targetCand.Ref
	args := event.Args{
		event.ArgDatabase: tref.Database,
		event.ArgHost:     tref.Host,
		event.ArgPort:     strconv.Itoa(tref.Port),
		event.ArgRole:     tref.User,
	}
	// A container-backed target — one the discovery ladder found running, or
	// one --create-target provisioned — carries its password in the
	// container's own environment (ARCHITECTURE.md section 9), never in
	// lazyslice.yml (ADR-004) and never resolved through
	// --password-command (passwordAvailable is already true for it). That is
	// exactly dogfood session 1's own complaint: "psql fails until docker
	// inspect".
	//
	// The docker exec line is keyed on targetContainerID, never targetLabel:
	// for a compose-managed database the ladder found running or stopped,
	// targetLabel is the compose *service* name (e.g. "db"), which is what
	// the candidate list prints, but "docker exec db ..." fails whenever the
	// real container name differs (e.g. "myproj-db-1") — the common case.
	// targetContainerID is only ever set when the ladder itself resolved a
	// real container (containers.go, question.go's adopted), so a rung-0
	// target with no validated container — including one whose committed
	// lazyslice.yml names a provenance that looks container-shaped — falls
	// through to the generic target.connect line instead of printing a
	// command built from unvalidated committed text (T-0320 fix round,
	// review finding 1).
	switch {
	case (r.targetProv == pipeline.FromContainer || r.targetProv == pipeline.FromStoppedContainer) && r.targetContainerID != "":
		args[event.ArgContainer] = r.targetContainerID
		r.send(event.Emit, event.Info, CodeTargetConnectContainer, args)
	case r.req.PasswordCommand != "":
		r.send(event.Emit, event.Info, CodeTargetConnectPasswordCommand, args)
	default:
		r.send(event.Emit, event.Info, CodeTargetConnect, args)
	}
}

// ---------- the yml ----------

// readConfig reads the committed lazyslice.yml, which is the prior for
// classification and the source of every default the operator did not pass.
func (r *run) readConfig() error {
	if r.req.Reconfigure {
		return nil
	}
	cfg, err := emit.New(emit.Options{}).Read(r.req.ConfigPath)
	var mfErr *emit.MappingFileError
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil
	case errors.As(err, &mfErr):
		// ADR-012: mapping_file is a v1 escape hatch nothing implements yet, so
		// naming it in the committed file is a usage error rather than a
		// silently ignored field.
		return &Stop{
			Code: CodeConfigMappingFileUnsupported, Exit: exitUsage,
			Args: event.Args{
				event.ArgPath:   r.req.ConfigPath,
				event.ArgTable:  mfErr.Table,
				event.ArgColumn: mfErr.Column,
			},
			Message: err.Error(),
		}
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
		r.targetNamed = true
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

	opts := discover.Options{
		Workdir:    r.req.Workdir,
		DockerHost: r.req.DockerHost,
		// Rung 0 is the committed file readConfig has already read; --reconfigure
		// leaves it nil, which is what makes that flag "run the first-run path".
		Config:       r.prior,
		Source:       r.req.Source,
		Target:       r.req.Target,
		NeedTarget:   r.req.Mode.needsTarget(),
		CreateTarget: r.req.CreateTarget,
		// R2-16 (THREAT_MODEL.md T5, T6, reviewed T-0213 fix round): rung0's
		// own doc comment and ARCHITECTURE.md §9 Q4 both promise
		// --password-command supplies a password for a candidate the ladder
		// itself found, not only one named with --source/--target — and
		// without this line the ladder probed every password-less candidate
		// with none, failed auth, and left it Reachable=false, so
		// chooseSource/chooseTarget could never choose it. discover.probe
		// resolves this at most once per walk (Options.pwCache), so setting
		// it here costs nothing on a run where nothing needs it.
		PasswordCommand: r.req.PasswordCommand,
		// --yes and "no controlling terminal" are one path (ADR-008 section 7).
		// Without this line the flag stops at core: a run under an allocated
		// TTY (docker run -t, script(1), tmux) opens /dev/tty and blocks in the
		// prompt with no timeout instead of taking Q1's headless refusal.
		Yes: r.req.Yes,
		// Prompter carries Request's own unexported prompter through to the
		// ladder unchanged; nil (the case for every real run) leaves
		// prompterFor deciding from Yes and the controlling terminal exactly
		// as before this field existed. TestEveryLadderOptionTheRequestCarriesIsCopied
		// does not require this line — Request's field is unexported and so
		// never matched against discover.Options.Prompter by name — but the
		// ladder still needs the value copied for a test to construct an
		// interactive run at all (T-0184, ADR-013 review, the 2026-09-16
		// reverify).
		Prompter: r.req.prompter,
	}
	res, err := discover.Resolve(ctx, opts, r.sink)
	if err != nil {
		return refusalStop(err)
	}
	r.req.Source, r.sourceProv, r.sourceLabel = res.Source, res.SourceProvenance, res.SourceLabel
	r.req.Target, r.targetProv, r.targetLabel = res.Target, res.TargetProvenance, res.TargetLabel
	r.targetContainerID = res.TargetContainerID
	r.targetNamed = res.TargetNamed
	// ADR-008's one-question rule, for rootQuestion (Q2): res.Asked is true
	// only when Q1 or Q1' actually reached the controlling terminal, so a
	// headless Q1/Q1' that took its default with nobody to ask still leaves
	// Q2 free to ask its own.
	r.askedQ1 = res.Asked
	// Kept for openTarget's gate step (T-0184, ADR-013 review finding 3):
	// discover.Headless(opts) over the same Options the ladder was resolved
	// with, rather than a second discover.Options literal built later —
	// TestEveryLadderOptionTheRequestCarriesIsCopied reads run.go's
	// discover.Options literal as exactly one.
	r.headless = discover.Headless(opts)
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

// passwordCommandStop is secret.password_command.failed (T-0213): --password-
// command ran for ref and did not supply a password. err is always a
// *discover.PasswordCommandError here — resolveEndpoints and discover's own
// dial have already been through — so the reason it carries is the exit
// status, the timeout or "printed no password", never the command's stdout.
func passwordCommandStop(ref dsn.Ref, err error) *Stop {
	reason := err.Error()
	var pce *discover.PasswordCommandError
	if errors.As(err, &pce) {
		reason = pce.Reason
	}
	s := wrap(CodePasswordCommandFailed, exitCredential, err, "no password for %s: --password-command %s", ref.String(), reason)
	s.Args = event.Args{event.ArgHost: ref.String(), event.ArgReason: reason}
	return s
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
	d, err = discover.ResolvePassword(ctx, d, r.req.PasswordCommand)
	if err != nil {
		return passwordCommandStop(sourceRef, err)
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

	// T-0241 (round-4 red team, docs/reviews/2026-09-15-redteam/round4-still-leaking.json,
	// "a read replica"): a positive, privilege-free check beside the identity
	// comparison openTarget's Gate call makes below, independent of it —
	// pg_is_in_recovery() is executable by PUBLIC on every supported version,
	// and this still catches a standby the cluster identity's own
	// data_directory field could otherwise clear (a standby and its primary
	// can genuinely differ there). A source that will not answer is not a
	// refusal here either, the same swallow-the-error convention SystemID and
	// ClusterID already use two lines up — only what a "yes" reveals is.
	//
	// r.replica keeps this one answer for the rest of the run (T-0255 fix
	// round, review): openTarget's SetSourceReplica reads it below rather
	// than calling Source.Replica a second time, so the header line, this
	// refusal and the gate's data_directory carve-out can never disagree
	// about whether the source is a standby.
	if replica, replicaErr := src.Replica(ctx); replicaErr == nil {
		r.replica = replica
	}
	if r.replica.Standby {
		r.send(event.Discover, event.Warn, CodeSourceStandby, event.Args{
			event.ArgHost:   sourceRef.Host,
			event.ArgReason: standbySenderReason(r.replica),
		})
		// A headless run with no --target has nobody to show that warning to
		// and nothing this cheap to tell the standby's own primary apart from
		// an unrelated server by address alone — the discovery ladder's
		// clusterKey comparison is exactly the address-only check the
		// round-4 red team's second reproduction (the compose-file/env-var
		// shape) walked straight past, because a standby and its primary
		// ordinarily publish on different ports. A --target the operator
		// named, on the standby's primary or anywhere else, is unaffected:
		// this rail is about what the ladder may pick unsupervised, exactly
		// as ADR-013's own escalations are.
		if stop := standbyNoTargetRefusal(r.req.Mode.needsTarget(), r.headless, r.targetNamed, sourceRef); stop != nil {
			return stop
		}
	}

	if !r.req.Mode.needsTarget() {
		return nil
	}
	return r.openTarget(ctx)
}

// standbyNoTargetRefusal is T-0241's headless rail (round-4 red team,
// docs/reviews/2026-09-15-redteam/round4-still-leaking.json), as a pure
// function so it can be pinned without a database — the same reason
// sameClusterVerdict (internal/pg/target.go, pinned by cluster_test.go's
// TestSameClusterVerdict) is factored out rather than left inline in Gate.
// It is called only once discover has already confirmed the source answered
// pg_is_in_recovery() true; a nil result means proceed, a non-nil one is the
// refusal to return verbatim.
func standbyNoTargetRefusal(needsTarget, headless, targetNamed bool, sourceRef dsn.Ref) *Stop {
	if !needsTarget || !headless || targetNamed {
		return nil
	}
	return &Stop{
		Code: CodeSourceStandbyNoTarget, Exit: exitTarget,
		Args: event.Args{
			event.ArgHost: sourceRef.Host,
			event.ArgFlag: "--target",
		},
		Message: "the source " + sourceRef.String() + " is a streaming standby and no --target " +
			"was given: lazyslice cannot tell its primary apart from an unrelated server",
	}
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
	d, err = discover.ResolvePassword(ctx, d, r.req.PasswordCommand)
	if err != nil {
		return passwordCommandStop(targetRef, err)
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
	// The cluster identity rule 1 falls back to when system_identifier is
	// unreadable, which it is for the SELECT-only source role §9 recommends
	// (the 2026-09-15 red team's identity-rule-1 finding). Like the system
	// identifier, a source that will not answer is not a refusal here — the
	// gate decides what an unknown identity means, and it now fails closed for
	// a target carrying the source's own database name.
	if clusterID, clusterErr := r.source.ClusterID(ctx); clusterErr == nil {
		tgt.SetSourceCluster(clusterID)
	}
	// The replica status Gate needs for rule 1's standby carve-outs (T-0255,
	// round-5 red team: docs/reviews/2026-09-15-redteam/round5-still-leaking.json).
	// discover already read this once, above, for the header warning and the
	// headless refusal, and kept the answer on r.replica (T-0255 fix round,
	// review) instead of discarding it — a second read here could disagree
	// with the first (a promotion, a pool error between the two calls) and
	// leave the header printing "standby" while the gate compared as if it
	// were not, which is the exact leak the carve-out exists to close.
	tgt.SetSourceReplica(r.replica)

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
	// §9 rule 1's warning, which has been owed since the gate was written: the
	// target is a different database on the source's own cluster, which is
	// eligible by design and is the loudest thing about the write's location.
	// internal/pg computed Eligibility.SameCluster and nothing read it, so a
	// headless run whose target the discovery ladder chose on the production
	// server wrote there in silence (the 2026-09-15 red team).
	if e.SameCluster {
		// T-0184, ADR-013 review finding 3: discover's own same-cluster check
		// (allOnSourceCluster) is the cheap clusterKey comparison — host:port
		// only — and two candidates can be on one physical cluster and compare
		// unequal there (host.docker.internal vs 127.0.0.1, a pooler, an SSH
		// tunnel). The gate's Eligibility.SameCluster is the authoritative
		// signal (system_identifier, or cluster identity/endpoint as a
		// fallback) and arrives too late for discover to have refused on it.
		// So when the target was chosen by the ladder rather than named by the
		// operator (--target, the positional DSN, or a committed
		// lazyslice.yml — r.targetNamed is false) and the run is headless, the
		// gate's own signal is a refusal here too, not only a warning: the
		// control ADR-013 exists to add must not be bypassable by an address
		// spelling discover's cheap check missed. An operator who named
		// --target on the source's cluster on purpose is unaffected, exactly
		// as ADR-013 says for the cheap check — and so is one whose committed
		// lazyslice.yml already names it: r.targetNamed is the fact that
		// matters here, not r.targetProv, because rung 0 carries the file's
		// own provenance forward (FromEnvVar, FromContainer, FromCompose) and
		// never stamps pipeline.FromYml, so a yml-named target can carry the
		// same provenance a ladder-chosen one would (ADR-013 review finding 1).
		if !r.targetNamed && r.headless {
			return &Stop{
				Code: CodeTargetGateSameCluster, Exit: exitTarget,
				Args: event.Args{
					event.ArgHost: targetRef.Host,
					event.ArgFlag: "--target",
				},
				Message: "the target " + targetRef.String() + " is on " + targetRef.Host +
					", the source's own cluster: pass --target to write there on purpose",
			}
		}
		r.send(event.Discover, event.Warn, CodeTargetSameCluster, event.Args{
			event.ArgDatabase: targetRef.Database,
		})
	}
	if e.MarkerBound {
		r.send(event.Discover, event.Info, CodeTargetTruncating, event.Args{
			event.ArgDatabase: targetRef.Database,
		})
	}
	// The verdict is kept, not consumed: section 11.2's three warnings compare
	// the marker against this run's secret fingerprint and classification, and
	// neither exists until classify and keyBeforePlan have run. markerWarnings
	// is called from move, before the first write to the target.
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
// fingerprint comes from keyBeforePlan (T-0161; resolveKey until it moved
// ahead of the plan stage) and the classification fingerprint from
// classifyStage, both of which run after discover. Called from openTarget
// every branch was guarded on a field that was still zero, so none of the
// three could ever print — which is the whole of what section 11.2 asks for,
// silently missing.
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
// classifier alone decided it: the committed yml and this run's other flags as
// prior (buildPrior), and none of this run's --unmask flags.
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
	// Everything classifyPrior folded in except the --unmask flags: the
	// --mask flags and --phone-region stay, because the reasons screen writes
	// neither and both passes of a review carry them alike (T-0319).
	prior, _, err := r.buildPrior(false, true)
	if err != nil {
		return "", err
	}
	cls, err := classify.New().Classify(r.schema, schemaSampler{schema: r.schema}, prior)
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
	// A column the run was asked to mask that the classifier left unmasked
	// is exit 2 here, after every decision line has been printed (so the
	// reason the raise was declined is on screen) and before the plan.
	if err := r.checkMasks(cls); err != nil {
		return err
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

// classifyPrior is the committed file with the --unmask and --mask flags
// folded in (buildPrior).
//
// The flag and the file are one input to the classifier, because they are one
// question: is this column opted out, or asked to be masked? A flag opt-out
// carries no type fingerprint — it is made for this run and dies with it — and
// classify honours exactly that case by branching on `by: flag` (its
// honourOptOut).
func (r *run) classifyPrior() (*pipeline.Config, error) {
	r.unmask = map[ref.ColumnRef]string{}
	r.masks = map[ref.ColumnRef]maskRequest{}
	// T-0221: the flag, when given, wins over the committed yml's own
	// phone_region -- "the flags, last, so they win", the same rule
	// planRequest states for --allow-type-literal. r.phoneRegion is read by
	// the emitter and by verify.Options further down execute, so it is set
	// here whether or not the unmask-driven copy below runs.
	r.phoneRegion = r.req.PhoneRegion
	if r.phoneRegion == "" && r.prior != nil {
		r.phoneRegion = r.prior.PhoneRegion
	}
	prior, in, err := r.buildPrior(true, true)
	if err != nil {
		return nil, err
	}
	r.unmask = in.unmask
	r.masks = in.masks
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
	// T-0186: one line per --allow-type-literal opt-out this run honours (the
	// flag, or a still-fingerprint-matching entry carried forward from the
	// committed yml's types: block) and one warn per yml entry that did not
	// carry forward because its fingerprint no longer matches or the type is
	// gone — the type-literal counterpart of classifyStage's own
	// CodeColumnOptOutExpired loop, raised here because a type opt-out is
	// merged and expired in this package rather than by the classifier.
	for _, name := range sortedTypeNames(r.typeAllow) {
		t := r.typeAllow[name]
		r.send(event.Plan, event.Info, CodeTypeLiteralAllowed, event.Args{
			event.ArgTable:  name,
			event.ArgReason: t.Reason + " (opt-out by " + t.By + ")",
		})
	}
	expired := append([]string(nil), r.typeExpired...)
	sort.Strings(expired)
	for _, name := range expired {
		r.send(event.Plan, event.Warn, CodeTypeLiteralOptOutExpired, event.Args{event.ArgTable: name})
	}
	p, err := plan.New().Plan(ctx, r.reader, r.schema, r.cls, req)
	if err != nil {
		// internal/plan collects every no_identity, unwritable, skip_parent
		// and unique_domain/equality_group refusal it finds in one pass
		// rather than stopping at the first (T-0318: dogfood session 1 took
		// nine runs to reach a green verify, each one refused on the next
		// single cause). plan.Refusals is that collection; asStop's own
		// *plan.Refusal case still applies to it (Refusals.Unwrap exposes
		// the first member), so this is the one place that also sends the
		// rest as their own Error events, ahead of returning the *Stop every
		// other refusal produces.
		var refusals plan.Refusals
		if errors.As(err, &refusals) {
			return r.reportPlanRefusals(refusals)
		}
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
	// §11.1 arm 1's finding for a plan-only run with no key yet (T-0161): one
	// line naming every masked default this run would mask if it held a key,
	// rather than the exit-13 refusal a writing run's key-bearing plan would
	// hit if it, too, could not rewrite them.
	if len(p.PendingKeyDefaults) > 0 {
		r.send(event.Plan, event.Info, CodePlanPendingKeyDefault, event.Args{
			event.ArgReason: strings.Join(p.PendingKeyDefaults, ", "),
		})
	}
	r.send(event.Plan, event.Info, CodePlanEstimate, event.Args{
		event.ArgCount:   strconv.FormatInt(p.Estimate.Rows, 10),
		event.ArgSeconds: strconv.FormatFloat(p.Estimate.HoldSeconds, 'f', 1, 64),
		event.ArgReason: strconv.FormatInt(p.Estimate.KeyMemory>>10, 10) + " KiB of keys, " +
			strconv.FormatInt(p.Estimate.FilterMemory>>10, 10) + " KiB of residual filter",
	})
	return nil
}

// reportPlanRefusals sends one Error event per refusal internal/plan
// collected (T-0318) and returns the *Stop the rest of Run treats like any
// other refusal.
//
// Every member is already ADR-005's exit 12 (plan.Refusals' own doc comment
// has the full account of why), so the process exit code is never in
// question; what this function decides is that the transcript names every
// independent cause the run found, in the order plan() found them, rather
// than only the one that answers for the exit code and the one line
// cmd/lazyslice's report prints after it. It sends refusals itself instead
// of leaving that to Run's own r.report(err) call because report sends
// exactly one Error event for whatever *Stop it is given (its own doc
// comment: "an exit code and the line that explains it can never
// disagree") — sending N here and a further one there would print the first
// refusal twice. The returned *Stop is marked sent for exactly that reason,
// the same way the discovery ladder's own refusal is (asStop's
// refusalStop doc).
func (r *run) reportPlanRefusals(refusals plan.Refusals) error {
	for _, ref := range refusals {
		r.sink.Send(event.Event{
			At: time.Now(), Kind: event.Error, Code: ref.Code, Exit: ref.Exit,
			Table: ref.Table, Column: ref.Column, Args: ref.Args,
		})
	}
	first := refusals[0]
	return &Stop{
		Code: first.Code, Exit: first.Exit, Table: first.Table, Column: first.Column,
		Args: first.Args, Message: first.Message, err: refusals, sent: true,
	}
}

// planRequest builds the planner's input from the flags and the committed file.
//
// The file supplies a value the operator did not pass, and never one they did:
// Request.Explicit is what tells the two apart. Section 10 calls the yml "a
// default the flag overrides, never a way to widen".
func (r *run) planRequest() (pipeline.PlanRequest, error) {
	// Rebuilt fresh on every call (planStage and buildConfig each call this):
	// deterministic from r.req, r.prior and r.schema, so there is nothing to
	// carry over between calls and an earlier call's entries must not linger.
	r.typeAllow = map[string]pipeline.TypeAllow{}
	r.typeExpired = nil
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
	// T-0161: keyBeforePlan has already run by the time planStage calls this
	// (execute), so r.keyFP is set exactly when a key was resolved — by a
	// writing run in full, or by a plan-only run that found one already
	// present. A plan-only run with no key leaves it unset, and req.Key stays
	// nil: internal/plan/ddlliteral.go reports what it would mask instead of
	// refusing over the absence of a secret nobody asked to create.
	if r.keyFP != "" {
		req.Key = &r.key
	}
	req.KeyPending = r.planOnly()
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
		// The committed file's own types: block, the same shape --unmask has
		// for columns (pipeline.Config.Types; ARCHITECTURE.md §11.1's fourth
		// arm, §10). It expires when the type's own fingerprint no longer
		// matches — a redefined enum or domain, or one no longer in the source
		// — rather than being carried forward blind; a flag naming the same
		// type below overwrites this and wins.
		for name, t := range p.Types {
			if t.Reason == "" {
				continue
			}
			if fp, ok := typeFingerprint(name, r.schema); !ok || fp != t.TypeFP {
				r.typeExpired = append(r.typeExpired, name)
				continue
			}
			if req.AllowTypeLiterals == nil {
				req.AllowTypeLiterals = map[string]string{}
			}
			req.AllowTypeLiterals[name] = t.Reason
			r.typeAllow[name] = t
		}
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
	// --allow-type-literal is resolved against the source's own enums and domains, so
	// an opt-out that could never apply is exit 2 here rather than a rail the
	// operator believes they lifted and did not (the same rule --unmask's
	// column names follow).
	for name, reason := range r.req.AllowTypeLiterals {
		typeName, err := resolveType(name, r.schema)
		if err != nil {
			return req, wrap(CodeUsage, exitUsage, err, "--allow-type-literal %s", name)
		}
		if req.AllowTypeLiterals == nil {
			req.AllowTypeLiterals = map[string]string{}
		}
		req.AllowTypeLiterals[typeName] = reason
		// Recorded with this run's own fingerprint, `by: flag`, so the next run
		// needs no flag and the opt-out still expires when the type changes —
		// the flag's own opt-out for a column follows the identical rule
		// (columnConfig, internal/emit). It overwrites any entry the yml prior
		// put here above, which is how the flag wins on the same type.
		fp, _ := typeFingerprint(typeName, r.schema)
		r.typeAllow[typeName] = pipeline.TypeAllow{Reason: reason, By: "flag", TypeFP: fp}
	}

	switch {
	case r.req.Root != "":
		t, err := resolveTable(r.req.Root, r.schema)
		if err != nil {
			return req, wrap(plan.CodeNoRoot, exitUsage, err, "--root %s", r.req.Root)
		}
		req.Root = &t
	case r.qRoot != nil:
		// Q2's own answer (root.go), already resolved and already checked
		// against the same scope chooseRoot enforces: no second parse of it,
		// and in particular no re-rendering through ref.TableRef.String()'s
		// unquoted "schema.name", which is what fed a quoted identifier
		// containing a dot back into this function's own resolveTable call
		// wrongly split before this field existed (T-0271 review).
		t := *r.qRoot
		req.Root = &t
	case r.req.Reviewed != nil && r.req.Reviewed.Root != (ref.TableRef{}):
		// The root the preview pass already planned from and showed the
		// operator (core.Reviewed.Root), carried here the same way r.qRoot is
		// above and for the identical reason: it is a resolved ref.TableRef,
		// not a string cmd/lazyslice's pinned() re-rendered onto Request.Root
		// for a second, disagreeing parse to misread (T-0271 review, finding
		// 5's own fix). Checked ahead of r.prior.Root because a --tui second
		// pass's Reviewed is what the operator actually reviewed on the
		// screens, which the run's own committed yml — read independently on
		// each pass — is not guaranteed to still agree with.
		t := r.req.Reviewed.Root
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
	// The key is already resolved: execute calls keyBeforePlan ahead of
	// planStage now (T-0161), because ARCHITECTURE.md section 11.1 arm 1 masks
	// a masked column's DEFAULT at plan and needs it there. move is reached
	// only by a run that writes, and keyBeforePlan resolves such a run's key in
	// full, so r.key and r.keyFP are already what they used to become here.
	//
	// Belt: move must never mask with a key it never resolved. keyBeforePlan
	// leaves r.key and r.keyFP unresolved for a plan-only run (planOnly()), and
	// mask.Key is a fixed-size array, so a zero r.key is structurally valid and
	// silently masks every value under an all-zero key instead of failing. A
	// drifted plan-only dispatch (execute's early return narrowing relative to
	// planOnly()) must not reach here; if it does, refuse instead of writing.
	if r.keyFP == "" {
		return nil, wrap(CodeInternal, exitInternal, errors.New("move: masking key not resolved"), "the masking key was not resolved before load")
	}

	// Section 11.2's bound-marker warnings still run here, the last point
	// before the loader truncates the target.
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

	// pipelineCtx is what both stage goroutines below select on to stop: extract
	// through extractCtx (a child of it, for the cause-carrying reason below) and
	// transform directly, in its masked-send select. r.abortStages lets close
	// cancel it on the panic-unwind path, where move's own defers below still
	// run (cancelExtract) but move's ordinary joins do not — see abortStages's
	// doc (2026-09-14 review of T-FAILUX, finding 1).
	pipelineCtx, cancelPipeline := context.WithCancelCause(ctx)
	defer cancelPipeline(nil)
	r.abortStages = func() { cancelPipeline(nil) }

	// WithCancelCause and not WithCancel: transform cancels extract when a masker
	// refuses a value, and extract then returns a bare context.Canceled. Without
	// the cause there is no way to tell that cancellation from a Ctrl-C, so the
	// masking refusal — the one error that says a value could not be masked —
	// was discarded and the run reported "interrupted", exit 130.
	extractCtx, cancelExtract := context.WithCancelCause(pipelineCtx)
	defer cancelExtract(nil)

	raw := make(chan pipeline.RowBatch, batchBuffer)
	masked := make(chan pipeline.RowBatch, batchBuffer)

	r.start(event.Extract)
	r.extractDone = make(chan error, 1)
	go func() {
		// Recovered here, not left to guardedExecute: that recover only wraps
		// root.ExecuteContext on the main goroutine, and extract is the
		// pipeline's own reader of production rows — a panic in it must become
		// an error on this goroutine's own done channel, the same as any other
		// extract failure, rather than an unredacted stack trace straight to
		// stderr whatever --debug says (2026-09-14 review of T-FAILUX, finding
		// 1). extract.Extract's own `defer close(out)` still runs during the
		// unwind, before this recover, so raw closes either way and transform
		// is never left ranging over a channel nobody will close.
		defer func() {
			if v := recover(); v != nil {
				r.releaseSnapshot(ctx)
				r.extractDone <- panicError{val: v, stack: debug.Stack()}
			}
		}()
		extractErr := extract.New(r.schema).Extract(extractCtx, r.reader, r.plan, raw)
		r.releaseSnapshot(ctx)
		r.extractDone <- extractErr
	}()

	r.start(event.Transform)
	r.transformDone = make(chan error, 1)
	go func() {
		defer close(masked)
		// Recovered for the same reason as extract's goroutine above: transform
		// is where masking runs, the most panic-prone code in the tree (every
		// masker in internal/transform, operating on whatever the source
		// actually holds). The recover is its own defer, run before
		// `defer close(masked)` above (LIFO), so a recovered panic still closes
		// masked like an ordinary return would.
		defer func() {
			if v := recover(); v != nil {
				perr := panicError{val: v, stack: debug.Stack()}
				// A recovered panic leaves the `for b := range raw` loop below
				// without draining it, so extract must be told to stop or its
				// next send blocks forever and move() never receives extractDone
				// (2026-09-14 review of T-FAILUX, finding 4). cancelExtract
				// unblocks extract's send select; extract then closes raw.
				cancelExtract(perr)
				r.transformDone <- perr
			}
		}()
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
			case <-pipelineCtx.Done():
				failure = pipelineCtx.Err()
			}
		}
		r.transformDone <- failure
	}()

	r.start(event.Load)
	lr, loadErr := load.New(r.loadRun(), r.sink).Load(ctx, writer, r.plan, r.schema, masked)
	extractErr := <-r.extractDone
	r.extractDone = nil
	transformErr := <-r.transformDone
	r.transformDone = nil
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
	report, verifyErr := verify.New(verify.Options{
		ProbeCap:    r.req.ResidualProbeCap,
		PhoneRegion: r.phoneRegion,
	}).Verify(
		ctx, r.source, readableWriter{Writer: writer, pool: r.targetPool},
		r.schema, r.plan, r.cls, residual, lr,
	)
	r.closeRun(ctx, writer, lr, verifyErr)
	r.done(event.Verify)
	if verifyErr != nil {
		// Verify runs every check and collects every failure (T-0319:
		// dogfood session 1 met one second-net column per run, three runs in
		// a row, because only the refusal the exit code came from was ever
		// printed). verify.Refusals is that collection when there is more
		// than one; each is sent as its own Error event here, the way
		// reportPlanRefusals does for the plan.
		var refusals verify.Refusals
		if errors.As(verifyErr, &refusals) {
			return report, r.reportVerifyRefusals(refusals)
		}
		return report, asStop(verifyErr)
	}
	return report, nil
}

// reportVerifyRefusals sends one Error event per failing verify check, in
// ARCHITECTURE.md section 6's order, and returns the *Stop for the first,
// which is the one the exit code comes from. The Stop is marked sent so that
// Run's own report does not print the first a second time
// (reportPlanRefusals, which this mirrors).
func (r *run) reportVerifyRefusals(refusals verify.Refusals) error {
	var first *Stop
	for _, refusal := range refusals {
		var s *Stop
		if !errors.As(asStop(refusal), &s) {
			continue
		}
		r.sink.Send(event.Event{
			At: time.Now(), Kind: event.Error, Code: s.Code, Exit: s.Exit,
			Table: s.Table, Column: s.Column, Args: s.Args,
		})
		if first == nil {
			first = s
		}
	}
	if first == nil {
		return asStop(refusals)
	}
	first.err = refusals
	first.sent = true
	return first
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
		// The run lease itself, so the loader can re-assert it is still held
		// before it acts on any of the above (T-0252). r.lease is set by
		// acquireLease before the gate's first probe, and a run that could not
		// take it never reaches this call at all — see openTarget.
		Lease: r.lease,
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
		Mask:              r.flagMasks(),
		Types:             r.typeAllow,
		PasswordCommand:   r.req.PasswordCommand,
		PhoneRegion:       r.phoneRegion,
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
	// R2-16 (THREAT_MODEL.md T5, T6): a --password-command whose argv looks
	// like it embeds a value rather than fetching one is withheld from the
	// file, not written verbatim into one whose header says it never contains
	// a secret. The check runs here, on the Config Emit already built, rather
	// than inside Emit itself: the warning names the config path
	// (r.req.ConfigPath), which Emit's own signature (ARCHITECTURE.md §2's
	// pipeline.Emitter) has no room for.
	if cfg.PasswordCommand != "" && emit.PasswordCommandSuspicious(cfg.PasswordCommand) {
		cfg.PasswordCommand = ""
		r.send(event.Emit, event.Warn, CodePasswordCommandWithheld, event.Args{event.ArgPath: r.req.ConfigPath})
	}
	return cfg, nil
}

// ---------- the masking key ----------

// keyBeforePlan resolves the run key ahead of the plan stage (T-0161), which
// ARCHITECTURE.md section 11.1 arm 1 needs to mask a masked column's DEFAULT:
// the rewrite happens at plan, in internal/plan/ddlliteral.go, and until this
// call moved there the key did not exist until move() ran a stage later, so
// every masked default was refused at exit 13 under arm 2's last clause
// instead of masked (testdata/regressions/011).
//
// A run that will write resolves the key in full — creating and persisting
// one if section 9 allows it and none exists yet, exactly what resolveKey
// always did, just a stage earlier. A plan-only run (ModePlan, or Preview's
// forced PlanOnly) must not conjure a secret nobody asked for merely by being
// planned, so it resolves only a key that already exists and leaves r.key
// unresolved otherwise; ddlliteral.go's columnDefault then reports what it
// would mask once a key exists (Plan.PendingKeyDefaults) rather than
// refusing over the absence of one.
func (r *run) keyBeforePlan() error {
	if r.planOnly() {
		return r.resolveKeyIfPresent()
	}
	return r.resolveKey()
}

// planOnly reports whether this run must not create a masking key merely by
// being planned: ModePlan (`lazyslice plan`), or Preview's forced PlanOnly.
//
// It exists so keyBeforePlan and planRequest read one condition rather than
// two hand-kept copies of it (2026-09-14 review of T-0161, finding 1):
// planRequest's pipeline.PlanRequest.KeyPending has to agree with which
// branch keyBeforePlan took, or internal/plan/ddlliteral.go's "no key yet"
// case and internal/core's "this run may not create one" case can drift
// apart silently.
func (r *run) planOnly() bool {
	return r.req.Mode == ModePlan || r.req.PlanOnly
}

// resolveKey finds the masking key, in the order ARCHITECTURE.md section 8 and
// section 9 give: $LAZYSLICE_SECRET, then --secret-file, then a new key written
// to that file — but only where section 9's repository rules allow it.
// checkSecretFile is ARCHITECTURE.md §9 "The repository" applied to the file
// itself rather than to its path, and it is the 2026-09-15 red team's two
// findings against THREAT_MODEL.md T6.
//
// **A symlink is refused.** os.WriteFile follows one, and repo.Protect's
// .gitignore entry and `git ls-files --error-unmatch` check are both applied to
// the link path — so `ln -s Dropbox/leaked.key lazyslice.secret` put the
// masking key in a cloud-synced folder while the transcript said the file had
// been added to .gitignore and written. The key is T13's guess-confirmation
// oracle for every snapshot ever made with it, so the direction that fails safe
// is a refusal: resolving the link and protecting the target instead would mean
// silently writing the key to a path the operator did not name, and a repository
// rule that follows a link out of the repository is not a repository rule.
// The check is on the path the run was given, so `--secret-file` pointing
// straight at a file outside the repository is unaffected — that is an operator
// naming a location, and repo.Protect already reports it as unprotected.
//
// **A mode granting group or other any bit is refused**, with the chmod to run.
// T6 promises the file is created 0600 and nothing re-checked an existing one,
// so a 0644 key in a CI image or on a shared machine was readable by every
// account under exit 0. The refusal is the shape the tracked-by-git one already
// has: a command to run, at exit 5, rather than a warning nobody acts on. It is
// not a silent chmod, because a key that has been world-readable may already
// have been read, and the operator is the one who knows whether that matters.
//
// A file that does not exist yet passes both: there is nothing to judge, and
// resolveKey creates it 0600.
//
// **Amendment, 2026-09-16 (R2-14 and R2-15).** The two checks above judge the
// final path component; both are on the file the run was given, which two
// more paths reach past:
//
//   - **A symlinked *parent directory*.** `ln -s ../Dropbox cloudkeys` followed
//     by `--secret-file ./cloudkeys/lazyslice.secret` names a file that does
//     not exist yet, so os.Lstat above returns ErrNotExist and both checks
//     pass — the symlink is in a directory component this function never
//     looked at. `checkSecretParentSymlink` walks every directory component
//     between the repository root and the file, with os.Lstat, and refuses
//     with the same secret.refused.symlink/exit 5 the direct case uses. It is
//     bounded to the repository (repo.Root, the same boundary repo.Protect
//     uses for the .gitignore entry) rather than resolved from the filesystem
//     root: an ambient symlink outside the repository — /tmp -> /private/tmp
//     on macOS is one — is not a path the operator wrote, and walking past the
//     repository root would refuse every run on such a system for a link
//     nobody added. A repository the operator's own directory component names
//     is exactly what the .gitignore entry and the tracked check are computed
//     against, so that is the boundary the resolution has to match.
//   - **A hard link.** `ln lazyslice.secret ../Dropbox/leaked.key` after a key
//     already exists gives the copy in Dropbox a name of its own; no path
//     check, symlink or otherwise, sees it, because the file the run reads and
//     writes is still exactly the file it thinks it is — it merely has more
//     than one name. `checkSecretHardlink` stats the file's link count and
//     refuses (secret.refused.hardlink, exit 5) above one, paired with the
//     permissive-mode refusal in the same function because both say "this key
//     may already have been read under a name .gitignore never protected".
//
// Both are documented in THREAT_MODEL.md T6.
func (r *run) checkSecretFile() error {
	if stop := r.checkSecretParentSymlink(); stop != nil {
		return stop
	}
	info, err := os.Lstat(r.req.SecretFile)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil
	case err != nil:
		return wrap(CodeSecretRefusedKey, exitCredential, err, "%s could not be read", r.req.SecretFile)
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		return &Stop{
			Code: CodeSecretSymlink, Exit: exitCredential,
			Args:    event.Args{event.ArgPath: r.req.SecretFile},
			Message: fmt.Sprintf("%s is a symbolic link", r.req.SecretFile),
		}
	}
	if perm, permissive := permissiveMode(info); permissive {
		return &Stop{
			Code: CodeSecretPermissive, Exit: exitCredential,
			Args: event.Args{
				event.ArgPath:      r.req.SecretFile,
				event.ArgStatement: "chmod 600 " + r.req.SecretFile,
			},
			Message: fmt.Sprintf("%s is mode %04o", r.req.SecretFile, perm),
		}
	}
	if nlink, ok := hardLinkCount(info); ok && nlink > 1 {
		return &Stop{
			Code: CodeSecretHardlink, Exit: exitCredential,
			Args:    event.Args{event.ArgPath: r.req.SecretFile},
			Message: fmt.Sprintf("%s has %d names on disk: it may be readable under a path .gitignore never protected", r.req.SecretFile, nlink),
		}
	}
	return nil
}

// checkSecretParentSymlink refuses when a directory component between the
// repository root and the secret file is a symbolic link (R2-14). It walks
// with os.Lstat rather than filepath.EvalSymlinks plus a string comparison,
// and it starts at repo.Root's own answer rather than the filesystem root, for
// the reason given on checkSecretFile: an ambient symlink above the
// repository (macOS's /tmp and /var) is not the operator's doing and must not
// turn every run on such a system into a refusal.
//
// The repository root is resolved from r.req.Workdir, never from
// filepath.Dir(SecretFile) or any other path that walks through the secret
// file's own directory chain. repo.Root's ancestor search does an os.Lstat on
// each candidate ancestor for ".git", and Lstat follows every *intermediate*
// path component (it only declines to follow the final one) — so resolving
// root from a directory that is itself, or sits under, a symlink lets that
// symlink's target supply the ".git" repo.Root finds, and root then comes
// back *as* (or under) the symlinked path. filepath.Rel(root, abs) is then
// "." or a suffix that starts below the symlinked component, and the walk
// below never Lstats the component that is actually the link (2026-09-16
// round 2 red team, R2-14 not closed). Workdir is a known-good anchor — it is
// where the process was started, never a path this function is asked to
// protect — so a symlink an attacker places anywhere under it, including at
// or below the secret file's own directory, is still on the walk below.
//
// A directory that does not exist yet is not a link (nothing has been made
// there to be one), and outside a repository entirely there is no .gitignore
// boundary for a link to defeat — repo.Root's ErrNoRepository is not this
// function's business either, the same carve-out entryFor already gives a
// secret file named straight at a location outside the repository.
func (r *run) checkSecretParentSymlink() error {
	dir := filepath.Dir(r.req.SecretFile)
	// normalise always fills Workdir from os.Getwd before a run reaches here
	// (run.go's normalise); dir is the fallback only for a caller that built
	// a *run by hand without it (as this package's own tests do) or for the
	// rare process where even os.Getwd failed.
	anchor := r.req.Workdir
	if anchor == "" {
		anchor = dir
	}
	root, err := repo.Root(anchor)
	if errors.Is(err, repo.ErrNoRepository) {
		return nil
	}
	if err != nil {
		return wrap(CodeSecretRefusedKey, exitCredential, err, "%s could not be resolved", r.req.SecretFile)
	}

	abs, err := filepath.Abs(dir)
	if err != nil {
		return wrap(CodeSecretRefusedKey, exitCredential, err, "%s could not be resolved", r.req.SecretFile)
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return wrap(CodeSecretRefusedKey, exitCredential, err, "%s could not be resolved", r.req.SecretFile)
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		// The secret file's directory is the repository root itself, or
		// outside the repository — nothing between root and file to walk.
		return nil
	}

	cur := root
	for _, comp := range strings.Split(rel, string(filepath.Separator)) {
		cur = filepath.Join(cur, comp)
		info, statErr := os.Lstat(cur)
		switch {
		case errors.Is(statErr, fs.ErrNotExist):
			return nil
		case statErr != nil:
			return wrap(CodeSecretRefusedKey, exitCredential, statErr, "%s could not be resolved", r.req.SecretFile)
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return &Stop{
				Code: CodeSecretSymlink, Exit: exitCredential,
				Args:    event.Args{event.ArgPath: r.req.SecretFile},
				Message: fmt.Sprintf("%s is inside %s, a symbolic link: the masking key would be written outside the repository, where .gitignore does not reach it", r.req.SecretFile, cur),
			}
		}
	}
	return nil
}

// ephemeralKeyCause turns repo.State's diagnosis of why the masking key
// cannot be written into the warn code that names it (for the non-
// --require-key path) and a developer-facing sentence about r.req.SecretFile
// that fmt.Sprintf has already filled in (for the --require-key Stop, whose
// Message is what the operator reads — see core.Stop's own doc comment and
// cmd/lazyslice's report()).
//
// The four causes are mutually exclusive and state carries exactly the bits
// needed to tell them apart (repo.State's own doc comments): .gitignore
// itself could not be read or appended to (GitignoreAppendable false) is the
// original T6 refusal and keeps CodeSecretEphemeral's wording; the other
// three all follow a *successful* append, so none of them may say ".gitignore
// cannot be written" (T-0251 round 6) — git absent from PATH so the entry
// could never be checked (GitFound false), a later rule un-ignoring the
// entry Protect just appended (GitignoreNegatedBy, which names it), or git
// checking and finding the path simply not ignored by anything.
func ephemeralKeyCause(state repo.State, path string) (code event.Code, cause string) {
	switch {
	case !state.GitignoreAppendable:
		return CodeSecretEphemeral, fmt.Sprintf("%s cannot be protected by .gitignore", path)
	case !state.GitFound:
		return CodeSecretGitignoreUnverifiable, fmt.Sprintf(
			"%s was added to .gitignore, but git is not on PATH to verify it is actually ignored", path)
	case state.GitignoreNegatedBy != "":
		return CodeSecretGitignoreNegated, fmt.Sprintf(
			"%s was added to .gitignore, but a later rule un-ignores it (%s)", path, state.GitignoreNegatedBy)
	default:
		return CodeSecretGitignoreNotIgnored, fmt.Sprintf(
			"%s was added to .gitignore, but git does not consider it ignored", path)
	}
}

func (r *run) resolveKey() error {
	found, state, err := r.resolveKeyState()
	if err != nil || found {
		return err
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
		//
		// Which is true — .gitignore itself could not be written to, or it
		// was written to and the secret is still not actually protected —
		// is asked of state, not assumed: printing "cannot write .gitignore"
		// when the append succeeded, and the entry sits right there in the
		// file, points the operator at a file that is already correct
		// (T-0251 round 6).
		warnCode, cause := ephemeralKeyCause(state, r.req.SecretFile)
		args := event.Args{event.ArgPath: r.req.SecretFile}
		if warnCode == CodeSecretGitignoreNegated {
			args[event.ArgReason] = state.GitignoreNegatedBy
		}
		if r.req.RequireKey {
			return &Stop{
				Code: CodeSecretRefusedKey, Exit: exitCredential, Args: args,
				Message: fmt.Sprintf("%s and --require-key is set", cause),
			}
		}
		r.send(event.Transform, event.Warn, warnCode, args)
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

// resolveKeyIfPresent fills r.key and r.keyFP from a key that already
// exists — the environment variable, or the committed secret file — and
// leaves both at their zero value when neither does. It never creates a key
// and never writes lazyslice.secret (T-0161): a plan-only run must not
// conjure a secret nobody asked for merely by being planned.
//
// It is deliberately its own read path and does not call resolveKeyState or
// repo.Protect (2026-09-14 review of T-0161, finding 2): repo.Protect is not
// read-only — appendMissing opens .gitignore O_APPEND|O_WRONLY and writes
// lazyslice.secret and snapshots/ into it — so `lazyslice plan` and the TUI's
// Preview pass were mutating the operator's .gitignore and could hard-abort
// at CodeSecretTracked on a command that writes nothing to the database or
// the filesystem otherwise. The tracked-file check (THREAT_MODEL.md T6) is
// therefore not run on this path either: it exists to stop a *write* using a
// key a clone should not trust, and a plan-only run performs no write for it
// to protect. A writing run still gets the full check, through resolveKey.
func (r *run) resolveKeyIfPresent() error {
	if text, ok := os.LookupEnv("LAZYSLICE_SECRET"); ok {
		k, kerr := mask.ParseKey(text)
		if kerr != nil {
			return wrap(CodeSecretRefusedKey, exitCredential, kerr, "$LAZYSLICE_SECRET is not a key")
		}
		r.key, r.keyFP = k, k.Fingerprint()
		return nil
	}
	// The environment variable is the one branch with no file, so the file
	// checks come after it on this path as they do on resolveKeyState's.
	if err := r.checkSecretFile(); err != nil {
		return err
	}
	body, err := os.ReadFile(r.req.SecretFile)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil
	case err != nil:
		return wrap(CodeSecretRefusedKey, exitCredential, err, "%s could not be read", r.req.SecretFile)
	}
	k, err := mask.ParseKey(string(body))
	if err != nil {
		return wrap(CodeSecretRefusedKey, exitCredential, err, "%s is not a key", r.req.SecretFile)
	}
	r.key, r.keyFP = k, k.Fingerprint()
	return nil
}

// resolveKeyState is resolveKey's read half: the environment variable, then
// section 9's repository check, then the committed file, in that order.
// found is true once r.key and r.keyFP are filled; state is repo.Protect's
// answer, which only resolveKey's own create branch below needs.
//
// resolveKeyIfPresent does not call this — see its own comment — so this is
// no longer shared between the two, and the tracked-file check below runs
// only for a run that may write.
func (r *run) resolveKeyState() (found bool, state repo.State, err error) {
	if text, ok := os.LookupEnv("LAZYSLICE_SECRET"); ok {
		// The environment variable is the one branch with no file, so section 9's
		// repository rules have nothing to protect: the key is not on disk, and
		// .gitignore cannot ignore what is not there.
		k, kerr := mask.ParseKey(text)
		if kerr != nil {
			return false, state, wrap(CodeSecretRefusedKey, exitCredential, kerr, "$LAZYSLICE_SECRET is not a key")
		}
		r.key, r.keyFP = k, k.Fingerprint()
		return true, state, nil
	}

	// Before repo.Protect, which opens .gitignore for append and asks git about
	// the *link* path: a secret file that is a symlink is refused before either
	// happens, so the transcript never claims to have protected a file it did
	// not (the 2026-09-15 red team).
	if err := r.checkSecretFile(); err != nil {
		return false, state, err
	}

	state, protectErr := repo.Protect(r.req.SecretFile, nil)
	if protectErr != nil {
		return false, state, wrap(CodeInternal, exitInternal, protectErr, "the repository could not be checked")
	}
	for _, added := range state.Added {
		r.send(event.Transform, event.Info, CodeGitignoreAdded, event.Args{event.ArgPath: added})
	}
	if len(state.Tracked) > 0 {
		return false, state, &Stop{
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

	body, readErr := os.ReadFile(r.req.SecretFile)
	if readErr == nil {
		k, parseErr := mask.ParseKey(string(body))
		if parseErr != nil {
			return false, state, wrap(CodeSecretRefusedKey, exitCredential, parseErr, "%s is not a key", r.req.SecretFile)
		}
		r.key, r.keyFP = k, k.Fingerprint()
		return true, state, nil
	}
	if !errors.Is(readErr, fs.ErrNotExist) {
		return false, state, wrap(CodeSecretRefusedKey, exitCredential, readErr, "%s could not be read", r.req.SecretFile)
	}
	return false, state, nil
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
	// Cancel the pipeline context before joining move's stage goroutines: on the
	// panic-unwind path move's own defers still ran (cancelExtract), but that
	// only ever unblocked extract, never transform's masked-send select — see
	// abortStages's doc. Nil until move sets it, and a no-op to call when move
	// already returned normally (its own deferred cancelPipeline got there
	// first).
	if r.abortStages != nil {
		r.abortStages()
	}
	// Join move's stage goroutines before anything below runs, in case a panic
	// unwound out of move before its own joins did (see extractDone's doc).
	// Normally move has already read both and set them back to nil, so this is
	// two nil checks and nothing else.
	if r.extractDone != nil {
		<-r.extractDone
		r.extractDone = nil
	}
	if r.transformDone != nil {
		<-r.transformDone
		r.transformDone = nil
	}
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

// sortedTypeNames orders m's keys so that T-0186's plan.type_literal.allowed
// events, and any test over them, do not depend on map iteration order.
func sortedTypeNames(m map[string]pipeline.TypeAllow) []string {
	out := make([]string, 0, len(m))
	for name := range m {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
