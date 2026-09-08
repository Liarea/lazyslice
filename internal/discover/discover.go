// SPDX-License-Identifier: Apache-2.0

// Package discover walks the discovery ladder: lazyslice.yml, environment
// variables and .env files, libpq settings, running Postgres containers,
// exited containers, compose service names (ARCHITECTURE.md §9, ADR-008).
//
// Discovery is why the first run asks at most one question. Source is never a
// question: it is the most-local reachable candidate with the most tables that
// is not the target, and no source at all is exit 3 with the ladder printed and
// a command to run.
//
// The ladder has a 2 s listing budget and a 1 s per-candidate dial. Inside the
// dial it runs three statements: version, the pg_class count and hint, and
// to_regclass('lazyslice_meta'). It never probes emptiness table by table; that
// is the gate's job, in internal/pg, after discovery.
//
// This build implements rungs 0 to 3. Rung 4 (exited containers) and
// provisioning are phase 5 (ARCHITECTURE.md §14): rung 4 counts the stopped
// containers it saw and says so, and --create-target is a refusal that names
// what is missing rather than a silent no-op.
package discover

import (
	"context"
	"errors"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/moby/moby/client"

	"github.com/Liarea/lazyslice/internal/discover/dockerctx"
	"github.com/Liarea/lazyslice/internal/dsn"
	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pipeline"
)

// listBudget is ARCHITECTURE.md §9's listing budget: two seconds for the whole
// ladder to say what it can see, before any candidate is dialled.
const listBudget = 2 * time.Second

// found is one rung's answer: the redacted candidate the ladder prints, and the
// connection string it was built from.
//
// The connection string never leaves this package except as the Result the CLI
// hands to core.Request, which is the one carrier ARCHITECTURE.md §2 gives it.
// Everything an event sees comes off cand, whose Ref is redacted by
// construction.
type found struct {
	cand pipeline.Candidate
	dsn  dsn.DSN
	// also names the provenances of candidates that collapsed into this one,
	// printed in parentheses so the collapse is visible (ADR-008 §4).
	also []string
}

// Options is what the ladder needs from the CLI. cmd/lazyslice fills it from
// flags; a test fills it from a temporary directory.
type Options struct {
	// Workdir is where .env files and the compose file are read and which the
	// container working_dir filter is matched against.
	Workdir string
	// DockerHost is --docker-host, empty when the flag was not given.
	DockerHost string
	// Config is the committed lazyslice.yml, already read, or nil. It is rung 0.
	// It arrives read rather than as a path because reading it is
	// internal/emit's job and a stage package does not reach another stage
	// package (internal/CLAUDE.md).
	Config *pipeline.Config
	// Source and Target are the endpoints the operator named: --source, the
	// positional DSN, --target. Each short-circuits the ladder for its own side
	// and neither is a rung (ADR-008 §1).
	Source, Target string
	// NeedTarget is false for the four read-only modes, which never open a
	// target and must not stop for the want of one.
	NeedTarget bool
	// CreateTarget is --create-target. Provisioning is phase 5
	// (ARCHITECTURE.md §14), so in this build the flag selects which refusal
	// the no-target state produces and never creates anything.
	CreateTarget bool
	// dial builds the Docker client. It is unexported so that only this
	// package's tests can replace it.
	dial func(dockerctx.Endpoint) (dockerAPI, error)
}

// Result is what the ladder decided, in the form internal/core takes.
//
// Source and Target are connection strings and therefore credentials: hand them
// to the run and to nothing else (THREAT_MODEL.md A3).
//
// The provenance and label of each side travel with it. internal/core builds
// its pipeline.Candidate from them and internal/emit writes them into
// lazyslice.yml, so the file records the rung the endpoint actually came from
// (ARCHITECTURE.md §10). Three cases, and they are the whole of the rule:
//
//   - named on the command line: pipeline.FromFlag with no label;
//   - supplied by the committed yml (rung 0): the file's own provenance and
//     label, not pipeline.FromYml — the file records where the endpoint was
//     found, and a re-run that rewrote `compose` as `yml` would lose that on
//     the second run instead of the first;
//   - chosen from the ladder: the winning candidate's own provenance and label.
type Result struct {
	Source, Target string

	// SourceProvenance and SourceLabel describe the source: the rung, and the
	// compose service, container or environment variable name that goes with
	// it. Both are identifiers, never a credential.
	SourceProvenance pipeline.Provenance
	SourceLabel      string
	// TargetProvenance and TargetLabel are the same two facts about the target.
	TargetProvenance pipeline.Provenance
	TargetLabel      string
}

// Refusal is a stop with the event code and the ADR-005 exit it carries.
//
// It mirrors core.Stop rather than being one: core imports this package, so the
// dependency cannot go the other way. cmd/lazyslice maps it to an exit code
// exactly as it maps a core.Stop.
type Refusal struct {
	Code    event.Code
	Exit    int
	Args    event.Args
	Message string
}

func (r *Refusal) Error() string {
	if r.Message == "" {
		return string(r.Code)
	}
	return string(r.Code) + ": " + r.Message
}

type ladder struct{}

// New returns the discovery ladder with the defaults: the working directory the
// caller passes to Discover, the Docker endpoint resolved from the environment,
// and no yml.
//
// It takes no arguments because internal/core calls it that way; a caller with
// flags to pass uses Resolve.
func New() pipeline.Discoverer { return ladder{} }

var _ pipeline.Discoverer = ladder{}

// Discover walks the ladder and returns the candidates it verified. It is the
// pipeline.Discoverer half: it decides nothing, and the selection rules in §9
// are Resolve's.
func (ladder) Discover(ctx context.Context, workdir string, sink event.Sink) ([]pipeline.Candidate, error) {
	found, _ := walk(ctx, Options{Workdir: workdir}, sink)
	out := make([]pipeline.Candidate, 0, len(found))
	for _, f := range found {
		out = append(out, f.cand)
	}
	return out, nil
}

// Resolve walks the ladder and answers the one blocking question, returning the
// source and target the run should use.
//
// It is the first-run entry point ARCHITECTURE.md §9 describes, and it is what
// `lazyslice` with no arguments runs. An endpoint the operator named
// short-circuits its own side, and a run that named both walks no rung and
// makes no Docker call at all (ADR-008 §1).
func Resolve(ctx context.Context, o Options, sink event.Sink) (Result, error) {
	var res Result
	// An endpoint the operator named is FromFlag, and the provenance is set
	// here rather than left at its zero value: pipeline.FromYml is that zero,
	// so an unset field would tell internal/emit the file named a database
	// nothing named.
	if o.Source != "" {
		res.Source, res.SourceProvenance = o.Source, pipeline.FromFlag
	}
	if o.Target != "" {
		res.Target, res.TargetProvenance = o.Target, pipeline.FromFlag
	}
	if o.Config != nil {
		// Rung 0. The yml short-circuits the same way a flag does, for whichever
		// of the two sides it supplies (ADR-008 §1). It carries its own
		// provenance forward — `from: compose` stays `compose` — because the
		// file records where the endpoint was found and not that it was read
		// back from a file (ARCHITECTURE.md §10).
		if res.Source == "" {
			if d, ok := refDSN(o.Config.SourceRef); ok {
				res.Source = d
				res.SourceProvenance, res.SourceLabel = o.Config.Source, o.Config.SourceLabel
			}
		}
		if res.Target == "" && o.NeedTarget {
			if d, ok := refDSN(o.Config.TargetRef); ok {
				res.Target = d
				res.TargetProvenance, res.TargetLabel = o.Config.Target, o.Config.TargetLabel
			}
		}
	}
	if res.Source != "" && (!o.NeedTarget || res.Target != "") {
		return res, nil
	}

	cands, dock := walk(ctx, o, sink)

	// A source the operator named is still the source for the purpose of the
	// target rules: the gate refuses a target that is the source, and a run that
	// discovered one of the two sides must not hand core the same database
	// twice (ARCHITECTURE.md §9 rule 1).
	source := namedSource(res.Source)
	if res.Source == "" {
		source = chooseSource(cands)
		if source == nil {
			return res, refuseNoSource(sink)
		}
		res.Source = string(source.dsn)
		res.SourceProvenance, res.SourceLabel = source.cand.Provenance, source.cand.Label
		send(sink, event.Decision, CodeSourceChosen, event.Args{
			event.ArgDatabase:   source.cand.Ref.Database,
			event.ArgHost:       source.cand.Ref.Host,
			event.ArgProvenance: provenanceOf(*source),
			event.ArgFlag:       "--source",
		})
	}
	if !o.NeedTarget || res.Target != "" {
		return res, nil
	}

	target, runnerUp := chooseTarget(cands, source)
	if target == nil {
		return res, refuseNoTarget(o, dock, sink)
	}
	res.Target = string(target.dsn)
	res.TargetProvenance, res.TargetLabel = target.cand.Provenance, target.cand.Label
	prov := provenanceOf(*target)
	if runnerUp != nil {
		prov += " (runner-up " + runnerUp.cand.Ref.String() + ")"
	}
	send(sink, event.Decision, CodeTargetChosen, event.Args{
		event.ArgDatabase:   target.cand.Ref.Database,
		event.ArgHost:       target.cand.Ref.Host,
		event.ArgProvenance: prov,
		event.ArgFlag:       "--target",
	})
	return res, nil
}

// namedSource is the given --source (or the yml's) as a candidate, so that the
// target rules can exclude it. It carries no dsn, because nothing selects it.
func namedSource(s string) *found {
	if s == "" {
		return nil
	}
	_, ref, err := dsn.Parse(s)
	if err != nil {
		return nil
	}
	return &found{cand: pipeline.Candidate{Ref: ref, Provenance: pipeline.FromFlag}}
}

// dockerEndpoint is what rung 3 resolved, carried out of the walk so that the
// --create-target refusal can name the endpoint without resolving it twice.
//
// Locality is a THREAT_MODEL.md T2 control and not a detail of rung 3: ADR-008
// §3 makes it the precondition of container discovery, of Q1 and of anything
// that creates a container, so the answer has to leave rung 3 rather than being
// re-derived by whoever needs it next.
type dockerEndpoint struct {
	// name is the endpoint as it is printed, or "(none)" when it could not be
	// resolved at all.
	name string
	// local is ADR-008 §3's locality test: a unix:// or npipe:// socket, or a
	// tcp:// loopback literal. ssh:// and every hostname are not local.
	local bool
	// reachable is whether the daemon answered a ping within the listing
	// budget. ADR-008 §6 makes Q1 fire only on an endpoint that is both.
	reachable bool
}

// usable reports whether this endpoint is one a container may be created on:
// local and answering (ADR-008 §3, §6).
func (d dockerEndpoint) usable() bool { return d.local && d.reachable }

// why is the reason usable() is false, for the refusal that names it.
func (d dockerEndpoint) why() string {
	if !d.local {
		return "not local"
	}
	return "not reachable"
}

// walk runs rungs 0 to 3, verifies every candidate it built, collapses the
// duplicates and prints each one as it resolves.
//
// Verification runs *before* the collapse, which costs a dial on a duplicate
// and buys the thing ADR-008 §4 does not say: the lower rung wins the printed
// provenance, and the surviving candidate keeps a connection string that
// answers. A rung-3 container's POSTGRES_PASSWORD and a rung-1 $DATABASE_URL
// naming the same (host, port, database) without one are the same database and
// not the same credential.
func walk(ctx context.Context, o Options, sink event.Sink) ([]found, dockerEndpoint) {
	var cands []found

	if o.Config != nil {
		cands = append(cands, rung0(o.Config)...)
	}

	env, unusable := rung1(o.Workdir)
	cands = append(cands, env...)
	for _, u := range unusable {
		send(sink, event.Warn, CodeEnvUnusable, event.Args{
			event.ArgProvenance: u.Name + " in " + u.Where,
			event.ArgReason:     u.Reason,
		})
	}

	cands = append(cands, rung2()...)
	containerCands, dock := rung3(ctx, o, sink)
	cands = append(cands, containerCands...)

	for i := range cands {
		probe(ctx, &cands[i])
	}
	cands = collapse(cands)
	for i := range cands {
		emitCandidate(sink, cands[i])
	}
	return cands, dock
}

// rung3 is running Postgres containers, behind the Docker endpoint ADR-008 §3
// resolves. Nothing here aborts the run: every way this rung can fail leaves
// rungs 0 to 2 answering, and prints why (ADR-008 §3 "Degradation").
func rung3(ctx context.Context, o Options, sink event.Sink) ([]found, dockerEndpoint) {
	endpoint, err := dockerctx.Resolve(ctx, o.DockerHost)
	if err != nil {
		send(sink, event.Warn, CodeDockerUnreachable, event.Args{
			event.ArgHost:   "(none)",
			event.ArgReason: "could not be resolved",
			event.ArgFlag:   "--docker-host",
		})
		return nil, dockerEndpoint{name: "(none)"}
	}
	dock := dockerEndpoint{name: endpoint.String(), local: endpoint.Local() && !endpoint.SSH()}
	if endpoint.Note != "" {
		send(sink, event.Warn, CodeDockerContextUnreadable, event.Args{
			event.ArgReason: endpoint.Note,
		})
	}
	send(sink, event.Info, CodeDockerEndpoint, event.Args{
		event.ArgHost:       endpoint.Host,
		event.ArgProvenance: endpoint.From,
	})

	switch {
	case endpoint.SSH():
		send(sink, event.Warn, CodeDockerSSH, event.Args{event.ArgHost: endpoint.Host})
		return nil, dock
	case !endpoint.Local():
		send(sink, event.Warn, CodeDockerNotLocal, event.Args{
			event.ArgHost:       endpoint.Host,
			event.ArgProvenance: endpoint.From,
			event.ArgFlag:       "--docker-host",
		})
		return nil, dock
	}

	list, cancel := context.WithTimeout(ctx, listBudget)
	defer cancel()

	api, err := dialDocker(o, endpoint)
	if err == nil {
		_, err = api.Ping(list, client.PingOptions{})
	}
	if err != nil {
		send(sink, event.Warn, CodeDockerUnreachable, event.Args{
			event.ArgHost:   endpoint.String(),
			event.ArgReason: "not reachable",
			event.ArgFlag:   "--docker-host",
		})
		return nil, dock
	}
	dock.reachable = true

	cands, stopped, matchedProject, err := containers(list, api, o.Workdir, endpoint.Local())
	if err != nil {
		send(sink, event.Warn, CodeDockerUnreachable, event.Args{
			event.ArgHost:   endpoint.String(),
			event.ArgReason: "containers could not be listed",
			event.ArgFlag:   "--docker-host",
		})
		return nil, dock
	}
	if !matchedProject && len(cands) > 0 {
		send(sink, event.Info, CodeComposeNoProject, nil)
	}
	if stopped > 0 {
		// Rung 4 is phase 5 (ARCHITECTURE.md §14). The count is printed so that
		// a developer whose database is stopped is told why it is not listed,
		// rather than being shown an empty ladder.
		send(sink, event.Info, CodeRungNotImplemented, event.Args{
			event.ArgProvenance: "rung 4 (stopped containers)",
			event.ArgCount:      strconv.Itoa(stopped),
			event.ArgFlag:       "--target",
		})
	}
	if compose := readCompose(o.Workdir); len(compose.PostgresServices) > 0 && len(cands) == 0 {
		// lazyslice never starts the developer's own compose services: their
		// images, volumes and networks are a larger surprise than ours
		// (ADR-008 §7). It prints the command that would.
		send(sink, event.Info, CodeComposeStopped, event.Args{
			event.ArgProvenance: strings.Join(compose.PostgresServices, " "),
		})
	}
	return cands, dock
}

// dialDocker opens the Docker client, through Options.dial when a test supplied
// one.
func dialDocker(o Options, e dockerctx.Endpoint) (dockerAPI, error) {
	if o.dial != nil {
		return o.dial(e)
	}
	return dockerctx.Dial(e)
}

// rung0 is ./lazyslice.yml. The file records references, not connection
// strings, so the candidate carries no password and the ordinary password
// sources (PGPASSWORD, ~/.pgpass, --password-command) supply one.
func rung0(cfg *pipeline.Config) []found {
	var out []found
	for _, side := range []struct {
		ref   dsn.Ref
		label string
		prov  pipeline.Provenance
	}{
		{cfg.SourceRef, cfg.SourceLabel, pipeline.FromYml},
		{cfg.TargetRef, cfg.TargetLabel, pipeline.FromYml},
	} {
		s, ok := refDSN(side.ref)
		if !ok {
			continue
		}
		d, ref, err := dsn.Parse(s)
		if err != nil {
			continue
		}
		out = append(out, found{
			dsn: d,
			cand: pipeline.Candidate{
				Ref:        ref,
				Provenance: side.prov,
				Label:      side.label,
				Local:      ref.Loopback(),
			},
		})
	}
	return out
}

// refDSN rebuilds a connection string from a redacted reference. It is how
// rung 0 turns lazyslice.yml's source_ref into something dialable; the password
// is not in the file and comes from libpq's own sources.
func refDSN(r dsn.Ref) (string, bool) {
	if r.Host == "" || r.Database == "" {
		return "", false
	}
	port := r.Port
	if port == 0 {
		port = 5432
	}
	u := url.URL{Scheme: "postgres", Path: "/" + r.Database}
	if r.User != "" {
		u.User = url.User(r.User)
	}
	if strings.HasPrefix(r.Host, "/") {
		u.RawQuery = url.Values{"host": {r.Host}, "port": {strconv.Itoa(port)}}.Encode()
		return u.String(), true
	}
	host := r.Host
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}
	u.Host = host + ":" + strconv.Itoa(port)
	return u.String(), true
}

// collapse de-duplicates candidates on the normalised (host, port, database).
//
// The lower-numbered rung wins the printed provenance and the discarded one is
// shown in parentheses (ADR-008 §4): a container publishing 0.0.0.0:5432 and a
// $DATABASE_URL naming localhost:5432 are one database, and printing them twice
// is how a developer concludes the tool cannot see their setup.
func collapse(in []found) []found {
	sort.SliceStable(in, func(i, j int) bool {
		if in[i].cand.Provenance != in[j].cand.Provenance {
			return in[i].cand.Provenance < in[j].cand.Provenance
		}
		return in[i].cand.Ref.String() < in[j].cand.Ref.String()
	})

	byKey := map[string]int{}
	out := make([]found, 0, len(in))
	for _, f := range in {
		key := collapseKey(f.cand.Ref)
		if at, ok := byKey[key]; ok {
			out[at].also = append(out[at].also, provenanceName(f.cand))
			// A candidate known to be local through a route the winner did not
			// have stays local: locality only ever grows here.
			out[at].cand.Local = out[at].cand.Local || f.cand.Local
			adoptReachable(&out[at], f)
			continue
		}
		byKey[key] = len(out)
		out = append(out, f)
	}
	return out
}

// adoptReachable hands the surviving candidate a connection string that works.
//
// ADR-008 §4 gives the lower-numbered rung the *printed provenance*, and the
// provenance and the credential are not the same fact. A rung-3 container
// carries POSTGRES_USER and POSTGRES_PASSWORD out of its own environment; a
// rung-1 $DATABASE_URL naming the same (host, port, database) may carry no
// password at all. Keeping only the lower rung's dsn drops a database rung 3
// could have used, so the winner keeps its provenance and takes the endpoint of
// whichever collapsed candidate actually answered.
//
// It runs after probe, which is why walk verifies before it collapses.
func adoptReachable(winner *found, loser found) {
	if winner.cand.Reachable || !loser.cand.Reachable {
		return
	}
	winner.dsn = loser.dsn
	// Ref travels with the dsn. It is the redacted view of that same string, and
	// a Ref naming one role while the dsn dials another is a candidate list
	// describing a connection nobody makes.
	winner.cand.Ref = loser.cand.Ref
	winner.cand.Reachable = true
	winner.cand.ConnectErr = ""
	winner.cand.Version = loser.cand.Version
	winner.cand.Tables = loser.cand.Tables
	winner.cand.EmptyHint = loser.cand.EmptyHint
	winner.cand.Marked = loser.cand.Marked
}

func collapseKey(r dsn.Ref) string {
	n, err := r.Normalised()
	if err != nil {
		n = r
	}
	return n.Host + ":" + strconv.Itoa(n.Port) + "/" + n.Database
}

// chooseSource is ARCHITECTURE.md §9's source rule: the most-local reachable
// candidate with the most tables. Local beats remote regardless of table count
// (Scenario B2). Source is never a question.
func chooseSource(cands []found) *found {
	best := -1
	for i := range cands {
		if !cands[i].cand.Reachable {
			continue
		}
		if best < 0 || moreSourceLike(cands[i], cands[best]) {
			best = i
		}
	}
	if best < 0 {
		return nil
	}
	return &cands[best]
}

func moreSourceLike(a, b found) bool {
	if a.cand.Local != b.cand.Local {
		return a.cand.Local
	}
	if a.cand.Tables != b.cand.Tables {
		return a.cand.Tables > b.cand.Tables
	}
	if a.cand.Provenance != b.cand.Provenance {
		return a.cand.Provenance < b.cand.Provenance
	}
	return a.cand.Ref.String() < b.cand.Ref.String()
}

// targetNamePattern is ADR-008 §5's tie-break: a database whose name ends in
// one of these is the one a developer meant to be written to.
var targetNamePattern = regexp.MustCompile(`(test|local|dev|snapshot|scratch)$`)

// chooseTarget applies ADR-008 §5's terminating tie-break over the
// target-shaped candidates and returns the winner and the runner-up.
//
// Target-shaped is reachable and not the source, and that is the whole of it.
// Eligibility is internal/pg's Target.Gate, which ARCHITECTURE.md §9 runs after
// discovery and never inside the 1 s dial; where §5 says "among eligible
// targets", this applies the same order among the candidates the gate will
// judge. Only the winner is handed on — core.Request carries one target and not
// a list — so when the gate refuses it the gate's own refusal stands verbatim
// and the run ends there. The runner-up is never tried, and no future change
// should make it fall through: that would load into a database the operator was
// never shown a decision line for (THREAT_MODEL.md T2). ADR-008 §5 and
// ARCHITECTURE.md §9 are owed the matching amendment (T-0062); until they carry
// it, this comment and internal/core's
// TestAGateRefusalEndsTheRunInsteadOfTryingTheRunnerUp are the statement of the
// rule.
//
// EmptyHint is not a filter here and must never become one. It is
// coalesce(bool_and(relpages = 0), true) over pg_class, and relpages is a
// planner statistic that TRUNCATE and DELETE do not reset, so a database that
// is empty by the gate's own SELECT EXISTS probe can carry EmptyHint false for
// as long as nothing has ANALYZEd it. Dropping such a candidate would be
// discovery deciding a target is ineligible from a statistic, which is exactly
// what internal/discover/CLAUDE.md forbids. The hint renders as
// "probably empty" and gates nothing.
func chooseTarget(cands []found, source *found) (winner, runnerUp *found) {
	var shaped []int
	for i := range cands {
		c := cands[i].cand
		if !c.Reachable {
			continue
		}
		if source != nil && collapseKey(c.Ref) == collapseKey(source.cand.Ref) {
			continue
		}
		shaped = append(shaped, i)
	}
	if len(shaped) == 0 {
		return nil, nil
	}
	sort.SliceStable(shaped, func(x, y int) bool {
		a, b := cands[shaped[x]].cand, cands[shaped[y]].cand
		if an, bn := targetNamePattern.MatchString(a.Ref.Database), targetNamePattern.MatchString(b.Ref.Database); an != bn {
			return an
		}
		if ap, bp := plausibleTarget(a), plausibleTarget(b); ap != bp {
			return ap
		}
		return collapseKey(a.Ref) < collapseKey(b.Ref)
	})
	winner = &cands[shaped[0]]
	if len(shaped) > 1 {
		runnerUp = &cands[shaped[1]]
	}
	return winner, runnerUp
}

// plausibleTarget is as far towards ADR-008 §5's "then the one carrying our
// bound marker" as discovery can honestly go.
//
// The marker is visible from here; its *binding* is not — that is the gate's
// rule 4, which needs the source's system_identifier and catalog. So another
// project's lazyslice target, marked and full of rows, must not outrank an
// unmarked database that looks empty: the two rank equally and the byte order
// of (host, port, database) settles it, which keeps the choice deterministic
// without inventing a fact. What ranks last is a candidate that is neither
// marked nor apparently empty, because that is the one the gate is most likely
// to refuse.
func plausibleTarget(c pipeline.Candidate) bool { return c.Marked || c.EmptyHint }

// refuseNoSource is exit 3: the ladder is already printed above this line, and
// the message is a command to run (ARCHITECTURE.md §9).
func refuseNoSource(sink event.Sink) error {
	r := &Refusal{
		Code:    CodeSourceNone,
		Exit:    exitNoSource,
		Message: "nothing on the ladder answered: pass --source postgres://...",
	}
	sendError(sink, r)
	return r
}

// refuseNoTarget is the state Q1 exists for: a source was found and nothing
// target-shaped was.
//
// Q1 is not asked in this build. Provisioning is phase 5 (ARCHITECTURE.md §14),
// and a question whose yes cannot be honoured is not a question, so the state
// takes the stop §9's one-question rule prescribes for an item with no safe
// default: the run stops and names the flag that settles it.
//
// The locality guard comes first and is not phase 5. ADR-008 §3 makes
// --create-target against a non-local or unreachable endpoint a refusal, exit
// 4, naming the endpoint and --docker-host — "never a silent no-op and never a
// container created elsewhere" — and THREAT_MODEL.md T2 is what forces it: a
// container created on someone else's daemon publishes on that host's
// interfaces, is not local, and lazyslice never removes a container. The guard
// sits here, ahead of every other answer to this state, so that phase 5 cannot
// wire a provisioner past it.
func refuseNoTarget(o Options, dock dockerEndpoint, sink event.Sink) error {
	if o.CreateTarget && !dock.usable() {
		r := &Refusal{
			Code: CodeTargetDockerNotLocal, Exit: exitTarget,
			Args: event.Args{
				event.ArgHost:   dock.name,
				event.ArgReason: dock.why(),
				event.ArgFlag:   "--docker-host",
			},
			Message: "--create-target needs a local, reachable docker endpoint",
		}
		sendError(sink, r)
		return r
	}

	// Both refusals name --target, because --target is the flag that names a
	// database to load into. --create-target changes only which of the two the
	// run gets: a run that passed it is told that starting a container is not in
	// this build, rather than being told to pass the flag it just passed.
	code := CodeTargetNone
	if o.CreateTarget {
		code = CodeTargetNotImplemented
	}
	r := &Refusal{
		Code: code, Exit: exitTarget,
		Args:    event.Args{event.ArgFlag: "--target"},
		Message: "no local postgres to load into, and starting one is not in this build",
	}
	sendError(sink, r)
	return r
}

// ---------- events ----------

func emitCandidate(sink event.Sink, f found) {
	args := event.Args{
		event.ArgHost:       f.cand.Ref.String(),
		event.ArgProvenance: provenanceOf(f),
	}
	if !f.cand.Reachable {
		args[event.ArgReason] = f.cand.ConnectErr
		send(sink, event.Info, CodeCandidateUnreachable, args)
		return
	}
	args[event.ArgCount] = strconv.Itoa(f.cand.Tables)
	args[event.ArgVersion] = strconv.Itoa(f.cand.Version)
	args[event.ArgReason] = hint(f.cand)
	send(sink, event.Info, CodeCandidate, args)
}

// hint is what the candidate list says about the target side without probing
// it: "carries our marker", "probably empty", or nothing it can promise.
func hint(c pipeline.Candidate) string {
	switch {
	case c.Marked:
		return "carries lazyslice's marker"
	case c.EmptyHint:
		return "probably empty"
	default:
		return "not empty"
	}
}

func provenanceOf(f found) string {
	name := provenanceName(f.cand)
	if len(f.also) > 0 {
		name += " (also " + strings.Join(f.also, ", ") + ")"
	}
	return name
}

func provenanceName(c pipeline.Candidate) string { return ProvenanceName(c.Provenance, c.Label) }

// ProvenanceName is the rung an endpoint came from, in the words the candidate
// list and the decision header print it in.
//
// It takes the two fields rather than a Candidate because internal/core prints
// the header from what Resolve handed it and has no candidate to rebuild: a run
// whose source came from a compose service must not print "flag" there any more
// than the emitted yml may record it (ARCHITECTURE.md §9 "Decision header").
func ProvenanceName(p pipeline.Provenance, label string) string {
	switch p {
	case pipeline.FromYml:
		return "lazyslice.yml"
	case pipeline.FromEnvVar:
		return envProvenance(label)
	case pipeline.FromLibpq:
		return "libpq environment"
	case pipeline.FromContainer:
		return containerProvenance(label)
	case pipeline.FromStoppedContainer:
		return "stopped container " + label
	case pipeline.FromCompose:
		return "compose service " + label
	case pipeline.FromFlag:
		return "flag"
	default:
		return "unknown"
	}
}

func envProvenance(label string) string {
	if label == "" {
		return "environment"
	}
	return "$" + label
}

func containerProvenance(label string) string {
	if label == "" {
		return "container"
	}
	return "container " + label
}

func send(sink event.Sink, kind event.Kind, code event.Code, args event.Args) {
	if sink == nil {
		return
	}
	sink.Send(event.Event{At: time.Now(), Stage: event.Discover, Kind: kind, Code: code, Args: args})
}

func sendError(sink event.Sink, r *Refusal) {
	if sink == nil {
		return
	}
	sink.Send(event.Event{
		At: time.Now(), Stage: event.Discover, Kind: event.Error,
		Code: r.Code, Args: r.Args, Exit: r.Exit,
	})
}

// AsRefusal reaches the Refusal in an error, for a caller mapping it to an exit
// code.
func AsRefusal(err error) (*Refusal, bool) {
	var r *Refusal
	ok := errors.As(err, &r)
	return r, ok
}
