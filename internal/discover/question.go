// SPDX-License-Identifier: Apache-2.0

package discover

import (
	"context"
	"errors"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/Liarea/lazyslice/internal/discover/provision"
	"github.com/Liarea/lazyslice/internal/dsn"
	"github.com/Liarea/lazyslice/internal/event"
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
//     the run's one blocking question is not spent on it.
//  3. No usable Docker endpoint, and so nothing that could be started or
//     created: an item with no safe default is not asked, the run stops, and
//     it names --target (ARCHITECTURE.md §9's one-question rule).
//  4. Q1', when the only target-shaped candidate is a stopped container. It is
//     asked *before* the gate runs, because a stopped container is not
//     reachable and reachability is the precondition of every gate rule.
//  5. Q1 otherwise.
//
// Q1 and Q1' are mutually exclusive, which is what keeps the run to one
// blocking question.
func noTarget(ctx context.Context, o Options, cands []found, source *found, dock dockerEndpoint, sink event.Sink) (*found, error) {
	if o.CreateTarget && !dock.usable() {
		return nil, refuseDockerNotLocal(dock, sink)
	}
	if o.CreateTarget {
		return provisionTarget(ctx, o, source, dock, sink)
	}
	if !dock.usable() {
		// Neither question can be honoured here, and a question whose yes
		// cannot be honoured is not a question. --target is named rather than
		// --create-target, because --create-target on this endpoint is the
		// refusal above and telling an operator to run into it is not a next
		// step.
		return nil, refuseNoTarget(sink, "--target")
	}
	if stopped := onlyStopped(cands, source); stopped != nil {
		return startStopped(ctx, o, stopped, dock, sink)
	}
	return askQ1(ctx, o, source, dock, sink)
}

// askQ1 is ADR-008 §6's Q1: "no local postgres found to load into. start one?".
//
// The default is yes, against lazygit's (y/N) precedent for the structurally
// identical question, because the container Q1 creates is one lazyslice names,
// owns and exists to write into. Headless — no controlling terminal, or --yes —
// it is the hard failure the table gives it: exit 4 naming --create-target.
func askQ1(ctx context.Context, o Options, source *found, dock dockerEndpoint, sink event.Sink) (*found, error) {
	// Whether there is anyone to ask is settled before anything is computed to
	// ask them: a headless run takes Q1's hard failure without dialling the
	// source for a major it will not use and without claiming a port it will
	// not publish.
	p, done, ok := prompterFor(o)
	if !ok {
		return nil, refuseNoTarget(sink, "--create-target")
	}
	defer done()

	major, err := sourceMajor(ctx, o, source, sink)
	if err != nil {
		return nil, err
	}
	port, err := provision.FreePort()
	if err != nil {
		return nil, err
	}
	name := provision.Name(projectName(o.Workdir))

	answer, err := p.Confirm("no local postgres found to load into. start one? postgres:"+
		strconv.Itoa(major)+" as "+name+" on port "+strconv.Itoa(port)+" [Y/n]", true)
	if err != nil || !answer {
		return nil, refuseNoTarget(sink, "--create-target")
	}
	return provisionWith(ctx, o, dock, sink, provision.Request{
		Project:  projectName(o.Workdir),
		Workdir:  o.Workdir,
		Major:    major,
		Port:     port,
		Progress: progressOf(o),
	})
}

// provisionTarget is --create-target: the same work as Q1's yes, with no
// question, because the flag is the answer.
func provisionTarget(ctx context.Context, o Options, source *found, dock dockerEndpoint, sink event.Sink) (*found, error) {
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
// --create-target, since one blocking question has already been asked and Q1
// cannot be the follow-up.
func startStopped(ctx context.Context, o Options, stopped *found, dock dockerEndpoint, sink event.Sink) (*found, error) {
	if p, done, ok := prompterFor(o); ok {
		answer, err := p.Confirm("target "+stopped.cand.Label+" is stopped — start it? [Y/n]", true)
		done()
		if err == nil && !answer {
			return nil, refuseNoTarget(sink, "--create-target")
		}
	}

	p, err := provisionerFor(o, dock)
	if err != nil {
		return nil, err
	}
	res, err := p.Start(ctx, stopped.containerID, provision.Request{
		Project:  projectName(o.Workdir),
		Workdir:  o.Workdir,
		Progress: progressOf(o),
	})
	if err != nil {
		return nil, provisionRefusal(err, sink)
	}
	return adopted(ctx, o, res, sink), nil
}

// adopted turns a provision.Result into the ladder's own candidate and prints
// it on the candidate list.
func adopted(ctx context.Context, o Options, res provision.Result, sink event.Sink) *found {
	f := &found{cand: res.Candidate, dsn: dsn.DSN(res.DSN), containerID: res.Container}
	o.probe(ctx, f)
	emitCandidate(sink, *f)
	return f
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
	if o.Yes {
		return nil, nil, false
	}
	if o.prompter != nil {
		return o.prompter, func() {}, true
	}
	opened, err := openPrompter()
	if err != nil {
		return nil, nil, false
	}
	return opened, func() { _ = opened.Close() }, true
}

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
				event.ArgSeconds: strconv.Itoa(int(notReady.Waited.Round(time.Second).Seconds())),
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
