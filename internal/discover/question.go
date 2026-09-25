// SPDX-License-Identifier: Apache-2.0

package discover

import (
	"context"
	"errors"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/moby/moby/api/types/container"

	"github.com/Liarea/lazyslice/internal/discover/provision"
	"github.com/Liarea/lazyslice/internal/dsn"
	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pipeline"
)

// noTarget answers the state Q1 and Q1' exist for: a source was found and
// nothing reachable and target-shaped was.
//
// It is ADR-008 §6's order, and the order is the whole of it:
//
//  1. The locality guard, ahead of everything else. --create-target against a
//     Docker endpoint that is not local, or that does not answer, is exit 4
//     naming the endpoint and --docker-host, "before any container work"
//     (ADR-008 §3). THREAT_MODEL.md T2 is what forces it: a container created
//     on someone else's daemon publishes on that host's interfaces, is not
//     local, and lazyslice never removes a container.
//  2. --create-target, which creates without asking. A flag is an answer, so
//     no question is asked for it.
//  3. No usable Docker endpoint, and so nothing that could be started or
//     created: an item with no safe default is not asked, the run stops, and
//     it names --target (ARCHITECTURE.md §9's question rule).
//  4. Q1', when the only target-shaped candidate is a stopped container. It is
//     asked *before* the gate runs, because a stopped container is not
//     reachable and reachability is the precondition of every gate rule.
//  5. Q1 otherwise.
//
// Q1 and Q1' are mutually exclusive, which is what keeps the target to one
// blocking question. The root question (Q2, internal/core) is asked after it
// when it is open too (ADR-017, proposed, T-0343).
//
// The second return is Result.Asked: true only when Q1 or Q1' actually put a
// question to the controlling terminal, whatever the answer. Every branch
// that never reaches a terminal — --create-target (a flag is an answer, not a
// question), no usable Docker endpoint, a headless Q1 or Q1' that took its
// default with nobody to ask — reports false.
func noTarget(ctx context.Context, o Options, cands []found, source *found, dock dockerEndpoint, sink event.Sink) (*found, bool, error) {
	if o.CreateTarget && !dock.usable() {
		return nil, false, refuseDockerNotLocal(dock, sink)
	}
	if o.CreateTarget {
		f, err := provisionTarget(ctx, o, source, dock, sink)
		return f, false, err
	}
	if !dock.usable() {
		// Neither question can be honoured here, and a question whose yes
		// cannot be honoured is not a question. --target is named rather than
		// --create-target, because --create-target on this endpoint is the
		// refusal above and telling an operator to run into it is not a next
		// step.
		return nil, false, refuseNoTarget(sink, "--target")
	}
	if stopped := onlyStopped(cands, source); stopped != nil {
		return startStopped(ctx, o, stopped, dock, sink, "")
	}
	if f, asked, ok, err := reuseOwn(ctx, o, dock, sink); ok {
		return f, asked, err
	}
	return askQ1(ctx, o, source, dock, sink, q1Lead, func() error {
		return refuseNoTarget(sink, "--create-target")
	})
}

// q1Lead is Q1's first sentence in ADR-008 §6's own words: the state it fires
// on when the ladder found nothing target-shaped. ADR-016 asks the same
// question in one more state — a committed lazyslice.yml names a lazyslice
// container this machine's Docker does not have — and names that container in
// its lead instead (missingLead, recorded.go).
const q1Lead = "no local postgres found to load into."

// askQ1 is ADR-008 §6's Q1: "no local postgres found to load into. start one?".
//
// The default is yes, against lazygit's (y/N) precedent for the structurally
// identical question, because the container Q1 creates is one lazyslice names,
// owns and exists to write into. Headless — no controlling terminal, or --yes —
// it is the hard failure the table gives it: exit 4 naming --create-target.
//
// lead is the sentence before "start one?", and refuse is the stop both a "no"
// and the headless path take. They are parameters because ADR-016 asks the
// same question about a committed target container that is gone, and that
// state's stop names --target as well as --create-target and names the
// container; everything else — the headless check first, the image, the port,
// the create — is one path so that the two cannot drift.
func askQ1(ctx context.Context, o Options, source *found, dock dockerEndpoint, sink event.Sink,
	lead string, refuse func() error,
) (*found, bool, error) {
	// Whether there is anyone to ask is settled before anything is computed to
	// ask them: a headless run takes Q1's hard failure without dialling the
	// source for a major it will not use and without claiming a port it will
	// not publish. Nobody to ask is also nothing asked (Result.Asked stays
	// false): the headless failure below is the same stop the answer "no"
	// would produce, and nobody was at a terminal to ask.
	p, done, ok := prompterFor(o)
	if !ok {
		return nil, false, refuse()
	}
	defer done()

	major, err := sourceMajor(ctx, o, source, sink)
	if err != nil {
		return nil, true, err
	}
	port, err := provision.FreePort()
	if err != nil {
		return nil, true, err
	}
	name := provision.Name(projectName(o.Workdir))

	answer, err := p.Confirm(lead+" start one? postgres:"+
		strconv.Itoa(major)+" as "+name+" on port "+strconv.Itoa(port)+" [Y/n]", true)
	if err != nil || !answer {
		return nil, true, refuse()
	}
	f, err := provisionWith(ctx, o, dock, sink, provision.Request{
		Project:  projectName(o.Workdir),
		Workdir:  o.Workdir,
		Major:    major,
		Port:     port,
		Progress: progressOf(o),
	})
	return f, true, err
}

// provisionTarget is --create-target: the same work as Q1's yes, with no
// question, because the flag is the answer.
//
// It checks the name the container would carry against a same-named container
// that already exists before doing anything else (T-0335): reuseOwn's
// name_taken guard above only fires on the path that asks Q1, which
// noTarget never reaches for --create-target (it returns from this function
// instead). Without this check here, provision.Name's own prefix-stripping
// (T-0333) makes two different directories collide on one container name —
// "shop" and "lazyslice-shop" in different directories both name
// lazyslice-target-shop — and provision.Provision adopts whatever already
// answers to that name with no ownership check of its own, loading into and
// starting another project's container with the credential that project's
// directory remembered.
func provisionTarget(ctx context.Context, o Options, source *found, dock dockerEndpoint, sink event.Sink) (*found, error) {
	if taken := nameTakenBySomeoneElse(ctx, o, dock); taken != "" {
		return nil, refuseNameTaken(sink, taken)
	}
	major, err := sourceMajor(ctx, o, source, sink)
	if err != nil {
		return nil, err
	}
	return provisionWith(ctx, o, dock, sink, provision.Request{
		Project:  projectName(o.Workdir),
		Workdir:  o.Workdir,
		Major:    major,
		Progress: progressOf(o),
	})
}

// nameTakenBySomeoneElse looks up the container provision.Name would propose
// for this project and reports its name when one already exists and is not
// ours (T-0334's ownContainer: the same check reuseOwn makes before Q1), or
// "" otherwise. A lookup that fails, or finds nothing, is not evidence the
// name is taken — the caller proceeds exactly as it would have with no
// container present, and provision.Provision makes the same byName check
// again before it creates or adopts anything.
func nameTakenBySomeoneElse(ctx context.Context, o Options, dock dockerEndpoint) string {
	name := provision.Name(projectName(o.Workdir))
	api, err := dialDocker(o, dock.endpoint)
	if err != nil {
		return ""
	}
	look, cancel := context.WithTimeout(ctx, listBudget)
	defer cancel()
	c, err := containerByName(look, api, name)
	if err != nil || c == nil {
		return ""
	}
	if !ownContainer(*c, o.Workdir) {
		return name
	}
	return ""
}

// provisionWith runs the provisioner and turns what it made into a candidate
// the rest of the ladder treats like any other.
//
// The container is dialled afterwards through the same probe every candidate
// gets, so the candidate line carries the same three facts, and Target.Gate
// then runs on it unrelaxed by the fact that we created it (ADR-008 §6 step 4).
func provisionWith(ctx context.Context, o Options, dock dockerEndpoint, sink event.Sink, req provision.Request) (*found, error) {
	p, err := provisionerFor(o, dock)
	if err != nil {
		return nil, err
	}
	res, err := p.Provision(ctx, req)
	if err != nil {
		return nil, provisionRefusal(err, sink)
	}
	return adopted(ctx, o, res, sink), nil
}

// startStopped is ADR-008 §6's Q1': the only target-shaped candidate is an
// exited container.
//
// Yes is the default and the headless answer both, because starting a container
// the developer already has is neither creating nor destroying, so
// THREAT_MODEL.md T2's headless rule does not require a flag for it. No is the
// same stop the same state produces headlessly for Q1: exit 4 naming
// --create-target, since Q1 and Q1' are one question about one target and Q1
// cannot be the follow-up (ADR-008 §6 step 5, unchanged by ADR-017).
//
// database is the database a committed record names (ADR-016 §3), or empty for
// the ladder's own Q1', which loads into the container's POSTGRES_DB. Without
// it a restarted record would load into POSTGRES_DB while the same record,
// found running, loads into the database it names.
func startStopped(ctx context.Context, o Options, stopped *found, dock dockerEndpoint, sink event.Sink, database string) (*found, bool, error) {
	asked := false
	if p, done, ok := prompterFor(o); ok {
		asked = true
		answer, err := p.Confirm("target "+stopped.cand.Label+" is stopped — start it? [Y/n]", true)
		done()
		if err == nil && !answer {
			return nil, asked, refuseNoTarget(sink, "--create-target")
		}
	}

	p, err := provisionerFor(o, dock)
	if err != nil {
		return nil, asked, err
	}
	res, err := p.Start(ctx, stopped.containerID, provision.Request{
		Project:  projectName(o.Workdir),
		Workdir:  o.Workdir,
		Progress: progressOf(o),
	})
	if err != nil {
		return nil, asked, provisionRefusal(err, sink)
	}
	if database != "" {
		if res, err = onDatabase(res, database); err != nil {
			return nil, asked, refuseInvalidRef(sink, "target", err)
		}
	}
	return adopted(ctx, o, res, sink), asked, nil
}

// adopted turns a provision.Result into the ladder's own candidate and prints
// it on the candidate list.
func adopted(ctx context.Context, o Options, res provision.Result, sink event.Sink) *found {
	f := &found{cand: res.Candidate, dsn: dsn.DSN(res.DSN), containerID: res.Container}
	o.probe(ctx, f)
	emitCandidate(sink, *f)
	return f
}

// reuseOwn is what happens instead of Q1 when the container Q1 would propose,
// lazyslice-target-<project>, already exists (T-0334). Q1 never proposes a
// name that is taken: dogfood session 3's second run was offered "start one?
// ... as lazyslice-target-<project> on port 5434" with that container
// running, and "yes" created nothing, because provisioning reuses a container
// of that name.
//
// It is reached only when the ladder did not already choose the container,
// which since T-0334 it does whenever the container is running, answers the
// dial and carries this project's label (found.own). What is left is a
// container the ladder could not rank: stopped beside other candidates, still
// booting past the 1 s dial, or kept off the candidate list by the working-dir
// filter. So:
//
//   - not ours (ownContainer: no provision.LabelProject for this project, or
//     a provision.LabelWorkingDir that is not this directory): somebody
//     else's container, or another checkout's with the same basename, holds
//     the name. It is not read, started or reused; exit 4 naming --target
//     (target.refused.name_taken), at a terminal and headless alike.
//   - ours and stopped: ADR-008 §6's Q1', in its own words and with its own
//     default, which headless is to start it.
//   - ours and running: started-if-needed and waited for through
//     provision.Start, which creates nothing, with no question — it is the
//     container this directory already made.
//
// handled is false when no container has the name, or when Docker would not
// list (a daemon that answered a ping and then would not list is not evidence
// the name is taken): the caller asks Q1 as before. Whatever this returns,
// Target.Gate still runs every rule on the container afterwards.
func reuseOwn(ctx context.Context, o Options, dock dockerEndpoint, sink event.Sink) (f *found, asked, handled bool, err error) {
	project := projectName(o.Workdir)
	name := provision.Name(project)
	api, err := dialDocker(o, dock.endpoint)
	if err != nil {
		return nil, false, false, nil
	}
	look, cancel := context.WithTimeout(ctx, listBudget)
	defer cancel()
	c, err := containerByName(look, api, name)
	if err != nil || c == nil {
		return nil, false, false, nil
	}
	if !ownContainer(*c, o.Workdir) {
		return nil, false, true, refuseNameTaken(sink, name)
	}
	if c.State != container.StateRunning {
		stopped := &found{
			stopped: true, containerID: name, own: true,
			cand: pipeline.Candidate{
				Provenance: pipeline.FromStoppedContainer,
				Label:      name,
				Local:      true,
				ConnectErr: stoppedReason,
			},
		}
		started, startAsked, startErr := startStopped(ctx, o, stopped, dock, sink, "")
		return started, startAsked, true, startErr
	}
	p, err := provisionerFor(o, dock)
	if err != nil {
		return nil, false, true, err
	}
	res, err := p.Start(ctx, name, provision.Request{
		Project:  project,
		Workdir:  o.Workdir,
		Progress: progressOf(o),
	})
	if err != nil {
		return nil, false, true, provisionRefusal(err, sink)
	}
	return adopted(ctx, o, res, sink), false, true, nil
}

// refuseNameTaken is exit 4: the container Q1 would have proposed already
// exists and lazyslice did not create it for this directory (T-0334) —
// somebody else made it, or another checkout with the same basename did.
func refuseNameTaken(sink event.Sink, name string) error {
	r := &Refusal{
		Code: CodeTargetNameTaken, Exit: exitTarget,
		Args: event.Args{
			event.ArgContainer: name,
			event.ArgFlag:      "--target",
		},
		Message: "a container named " + name + " already exists and lazyslice did not create it for this directory",
	}
	sendError(sink, r)
	return r
}

// onlyStopped is Q1's precondition: exactly one candidate is target-shaped and
// it is a rung-4 stopped container (ADR-008 §6).
//
// "Exactly one" counts every candidate that is not the source, reachable or
// not: a second unreachable candidate means the run has more than one thing it
// could be being asked about, and the question names one container.
func onlyStopped(cands []found, source *found) *found {
	var candidate *found
	for i := range cands {
		if source != nil && collapseKey(cands[i].cand.Ref) == collapseKey(source.cand.Ref) {
			continue
		}
		if !cands[i].stopped || cands[i].containerID == "" {
			return nil
		}
		if candidate != nil {
			return nil
		}
		candidate = &cands[i]
	}
	return candidate
}

// sourceMajor is the server major ARCHITECTURE.md §9 creates postgres:<major>
// from.
//
// A source chosen off the ladder was probed and carries it. A source that
// short-circuited the ladder — --source, the positional DSN, rung 0 — was not,
// so it is dialled once here, inside the same 1 s budget every candidate gets.
// It is never guessed: a target one major behind the source is a restore that
// fails halfway through a load.
func sourceMajor(ctx context.Context, o Options, source *found, sink event.Sink) (int, error) {
	if source != nil {
		if source.cand.Version == 0 && source.dsn != "" {
			o.probe(ctx, source)
		}
		if source.cand.Version != 0 {
			return source.cand.Version, nil
		}
	}
	// The image cannot be chosen, so neither question can be honoured. It is
	// the same stop as any other no-target state and it names --target, the
	// flag that needs neither a daemon nor a reachable source.
	r := &Refusal{
		Code: CodeTargetNone, Exit: exitTarget,
		Args:    event.Args{event.ArgFlag: "--target"},
		Message: "the source did not report a server version, so postgres:<major> cannot be chosen",
	}
	sendError(sink, r)
	return 0, r
}

// prompterFor answers "is there anybody to ask", which ADR-008 §7 makes one
// question with one answer: --yes and a process with no controlling terminal
// are the same path. The caller decides what that path means, because Q1 and
// Q1' differ there — Q1 is a hard failure and Q1' takes its default.
//
// done releases the terminal and is safe to call once; it is never nil when ok
// is true.
func prompterFor(o Options) (p Prompter, done func(), ok bool) {
	if o.Yes || o.NoControllingTerminal {
		return nil, nil, false
	}
	if o.Prompter != nil {
		return o.Prompter, func() {}, true
	}
	opened, err := openPrompter()
	if err != nil {
		return nil, nil, false
	}
	return opened, func() { _ = opened.Close() }, true
}

// isHeadless answers "is there nobody to ask", reusing prompterFor's own
// predicate rather than testing o.Yes alone (T-0184, ADR-013 review finding
// 1): this package's definition of headless has always been --yes OR no
// controlling terminal (Options.Yes doc comment; prompterFor above), and a
// caller that tested o.Yes by itself would treat a CI job or cron entry that
// simply omits --yes as interactive. It opens and immediately releases the
// controlling terminal prompterFor would open, so it costs one open/close and
// asks no question.
func isHeadless(o Options) bool {
	_, done, ok := prompterFor(o)
	if done != nil {
		done()
	}
	return !ok
}

// Headless is isHeadless, exported so that internal/core can ask the same
// question outside this package — the gate's own same-cluster signal
// (internal/pg's Eligibility.SameCluster) needs it too (T-0184, ADR-013
// review finding 3), and internal/core has no controlling-terminal test of
// its own to duplicate it with.
func Headless(o Options) bool { return isHeadless(o) }

// OpenPrompter is prompterFor, exported so that internal/core's Q2 (ADR-008
// §6) reads the controlling terminal through the same seam Q1 and Q1' use
// rather than a second one: a Prompter, a release func safe to call once, and
// whether there was anybody to ask. Q1, Q1' and isHeadless keep calling the
// unexported prompterFor directly; this is the one door opened for a caller
// outside the package, the same reason Headless exists beside isHeadless.
func OpenPrompter(o Options) (Prompter, func(), bool) { return prompterFor(o) }

// provisionerFor builds the write-capable Docker client, on the endpoint rung 3
// already resolved and already established is local and reachable.
func provisionerFor(o Options, dock dockerEndpoint) (provision.Provisioner, error) {
	if o.provisioner != nil {
		return o.provisioner(dock.endpoint)
	}
	return provision.Dial(dock.endpoint)
}

// progressOf is where the pull and the start print. ADR-008 §7 puts the prompt
// on stderr, and these are the two waits that happen after it.
func progressOf(o Options) io.Writer {
	if o.progress != nil {
		return o.progress
	}
	return os.Stderr
}

// projectName is the compose project name the container is named after: the
// file's `name:`, else the directory (ADR-008 §2 — compose is a naming source
// and nothing else).
func projectName(workdir string) string { return readCompose(workdir).Name }

// provisionRefusal maps a failure to create or start a container onto the exit
// code ADR-005 gives it.
//
// The 60 s budget running out has its own code, because it is the one failure
// the developer can act on without lazyslice: the container is left running and
// `docker logs` says why it did not come up (ADR-008 §6 step 3). Everything
// else is a Docker failure this run cannot classify, and it stops at exit 4
// naming --target, which is the flag that needs no daemon.
func provisionRefusal(err error, sink event.Sink) error {
	var notReady *provision.NotReadyError
	if errors.As(err, &notReady) {
		r := &Refusal{
			Code: CodeTargetStartTimeout, Exit: exitTarget,
			Args: event.Args{
				event.ArgContainer: notReady.Container,
				event.ArgSeconds:   strconv.Itoa(int(notReady.Waited.Round(time.Second).Seconds())),
			},
			Message: notReady.Error(),
		}
		sendError(sink, r)
		return r
	}
	r := &Refusal{
		Code: CodeTargetNone, Exit: exitTarget,
		Args:    event.Args{event.ArgFlag: "--target"},
		Message: err.Error(),
	}
	sendError(sink, r)
	return r
}

// refuseNoTarget is exit 4 with no target and no question left to ask. flag is
// what settles it: --create-target where a container could be created, --target
// where one could not.
func refuseNoTarget(sink event.Sink, flag string) error {
	r := &Refusal{
		Code: CodeTargetNone, Exit: exitTarget,
		Args:    event.Args{event.ArgFlag: flag},
		Message: "no local postgres to load into",
	}
	sendError(sink, r)
	return r
}

// refuseDockerNotLocal is ADR-008 §3's guard, before any container work.
func refuseDockerNotLocal(dock dockerEndpoint, sink event.Sink) error {
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
