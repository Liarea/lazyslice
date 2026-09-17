// SPDX-License-Identifier: Apache-2.0

package discover

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"

	"github.com/Liarea/lazyslice/internal/discover/dockerctx"
	"github.com/Liarea/lazyslice/internal/discover/provision"
	"github.com/Liarea/lazyslice/internal/dsn"
	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pipeline"
)

// Q1, both answers (ADR-008 §6). Yes creates postgres:<source major> as
// lazyslice-target-<project> and the endpoint it returns is the one the run
// loads into; no is the same stop the headless path takes, exit 4 naming
// --create-target.
func TestQ1CreatesAContainerOnlyWhenItIsAnsweredYes(t *testing.T) {
	for _, tc := range []struct {
		name        string
		answer      bool
		wantCreated bool
	}{
		{name: "yes creates the container", answer: true, wantCreated: true},
		{name: "no creates nothing and stops", answer: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			quietEnvironment(t)
			dir := t.TempDir()
			p := &fakeProvisioner{result: provisioned("postgres://postgres:pw@127.0.0.1:5433/postgres")}
			asked := &fakePrompter{answer: tc.answer}

			res, err := Resolve(t.Context(), Options{
				Workdir: dir, NeedTarget: true,
				Source:        "postgres://app@127.0.0.1:1/shop",
				DockerHost:    localDockerHost,
				dial:          fakeDial(&fakeDocker{}),
				dialCandidate: sayVersion(16),
				provisioner:   handOut(p),
				Prompter:      asked,
			}, event.Discard)

			if len(asked.questions) != 1 {
				t.Fatalf("asked %d questions, want exactly one blocking question: %q", len(asked.questions), asked.questions)
			}
			q := asked.questions[0]
			for _, want := range []string{"postgres:16", provision.Name(projectName(dir)), "[Y/n]"} {
				if !strings.Contains(q, want) {
					t.Errorf("Q1 = %q, want it to name %q (ARCHITECTURE.md §9)", q, want)
				}
			}

			if !tc.wantCreated {
				r, ok := AsRefusal(err)
				if !ok {
					t.Fatalf("want a refusal after no, got %v", err)
				}
				if r.Exit != 4 || r.Code != CodeTargetNone || r.Args[event.ArgFlag] != "--create-target" {
					t.Errorf("refusal = %s exit %d flag %q, want target.refused.none exit 4 naming --create-target",
						r.Code, r.Exit, r.Args[event.ArgFlag])
				}
				if len(p.provisioned) != 0 {
					t.Error("a no answer reached the provisioner; nothing may be created without a yes")
				}
				return
			}

			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if len(p.provisioned) != 1 {
				t.Fatalf("provisioned %d times, want once", len(p.provisioned))
			}
			got := p.provisioned[0]
			if got.Major != 16 {
				t.Errorf("provisioned postgres:%d, want the source's own major", got.Major)
			}
			if got.Project != projectName(dir) || got.Workdir != dir {
				t.Errorf("request = %+v, want the compose project name and the working directory", got)
			}
			if got.Port == 0 {
				t.Error("Q1 named a port and the request carries none, so the question and the container can disagree")
			}
			if !strings.Contains(q, "port "+itoa(got.Port)) {
				t.Errorf("Q1 = %q, want it to name port %d, the one that was used", q, got.Port)
			}
			if res.Target != "postgres://postgres:pw@127.0.0.1:5433/postgres" {
				t.Errorf("target = %q, want the provisioned endpoint, password and all", res.Target)
			}
			if res.TargetProvenance != pipeline.FromContainer {
				t.Errorf("provenance = %v, want rung 3: the next run finds it as a running container", res.TargetProvenance)
			}
		})
	}
}

// --create-target is an answer, so it creates without asking: the run's one
// blocking question is not spent on a flag the operator already passed.
func TestCreateTargetProvisionsWithoutAQuestion(t *testing.T) {
	quietEnvironment(t)
	dir := t.TempDir()
	p := &fakeProvisioner{result: provisioned("postgres://postgres:pw@127.0.0.1:5433/postgres")}
	asked := &fakePrompter{answer: true}

	if _, err := Resolve(t.Context(), Options{
		Workdir: dir, NeedTarget: true, CreateTarget: true,
		Source:        "postgres://app@127.0.0.1:1/shop",
		DockerHost:    localDockerHost,
		dial:          fakeDial(&fakeDocker{}),
		dialCandidate: sayVersion(16),
		provisioner:   handOut(p),
		Prompter:      asked,
	}, event.Discard); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(asked.questions) != 0 {
		t.Errorf("--create-target asked %q; the flag is the answer", asked.questions)
	}
	if len(p.provisioned) != 1 {
		t.Fatalf("provisioned %d times, want once", len(p.provisioned))
	}
}

// sourceMajor's probe of a --source that short-circuited the ladder shares
// Options.pwCache with walk's own candidates, so a run with --password-command
// and --create-target does not panic dereferencing a nil *passwordCache
// (T-0213 review round: the two production probe call sites question.go
// reaches, sourceMajor and adopted, used to receive Resolve's own Options
// copy, whose pwCache was built only inside walk and so stayed nil on this
// path). dialCandidate is deliberately left unset — every other test in this
// file sets it, which is exactly why this gap went uncaught — so sourceMajor
// dials the named source through the real probe, on a port nothing answers;
// the assertion is that Resolve returns an ordinary error instead of
// panicking, not that provisioning succeeds.
func TestPasswordCommandDoesNotPanicResolvingANamedSourcesMajor(t *testing.T) {
	quietEnvironment(t)
	dir := t.TempDir()

	_, err := Resolve(t.Context(), Options{
		Workdir: dir, NeedTarget: true, CreateTarget: true,
		Source:          "postgres://app@127.0.0.1:1/shop",
		PasswordCommand: "echo hunter2",
		DockerHost:      localDockerHost,
		dial:            fakeDial(&fakeDocker{}),
		provisioner:     refuseToProvision(t),
	}, event.Discard)
	if err == nil {
		t.Fatal("Resolve: got nil error, want a refusal — the source on port 1 answers nothing")
	}
}

// --create-target is Q1's answer and nothing more (ADR-008 §6), and Q1 fires
// only when the ladder found nothing target-shaped. ADR-008 §1 enumerates what
// short-circuits the ladder — --source, --target, the positional DSN and rung 0
// — and this flag is not on that list, so it neither discards a target the
// tie-break chose nor outranks the committed yml. Widening it to do either is a
// change to a frozen ADR and needs a superseding one, not a task.
func TestCreateTargetDoesNotOutrankTheLadderOrTheYml(t *testing.T) {
	for _, tc := range []struct {
		name       string
		with       func(*Options, string)
		wantTarget string
	}{
		{
			name: "a running container the tie-break chose",
			with: func(o *Options, dir string) {
				o.dial = fakeDial(&fakeDocker{
					list: []container.Summary{pgContainer("db", "/shop-db-test-1", "postgres:16", 5455, map[string]string{
						labelWorkingDir: dir,
					})},
					env: map[string][]string{"db": {"POSTGRES_DB=shop_test"}},
				})
			},
			wantTarget: "postgres://postgres@127.0.0.1:5455/shop_test?sslmode=disable",
		},
		{
			name: "the target the committed yml records",
			with: func(o *Options, _ string) {
				o.dial = fakeDial(&fakeDocker{})
				o.Config = &pipeline.Config{
					Target:    pipeline.FromContainer,
					TargetRef: mustRef(t, "postgres://postgres@127.0.0.1:5456/committed"),
				}
			},
			wantTarget: "postgres://postgres@127.0.0.1:5456/committed",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			quietEnvironment(t)
			dir := t.TempDir()
			o := Options{
				Workdir: dir, NeedTarget: true, CreateTarget: true, Yes: true,
				Source:        "postgres://app@127.0.0.1:1/shop",
				DockerHost:    localDockerHost,
				dialCandidate: sayVersion(16),
				provisioner:   refuseToProvision(t),
			}
			tc.with(&o, dir)

			res, err := Resolve(t.Context(), o, event.Discard)
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if res.Target != tc.wantTarget {
				t.Errorf("target = %q, want %q", res.Target, tc.wantTarget)
			}
		})
	}
}

// ARCHITECTURE.md §9: the container --create-target makes "is the developer's
// local database from then on". The password it minted is random and lives in
// the machine-local state dir alone — lazyslice.yml records a reference and
// never a credential (ADR-004) — so a second run whose target comes off the
// committed file dialled it with no password and stopped at exit 4 with
// "password authentication failed". Rung 0 puts the remembered credential back.
func TestASecondRunRecoversTheProvisionedTargetsPassword(t *testing.T) {
	quietEnvironment(t)
	dir := t.TempDir()
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)

	name := provision.Name(projectName(dir))
	targets := filepath.Join(state, "lazyslice", "targets")
	if err := os.MkdirAll(targets, 0o700); err != nil {
		t.Fatalf("creating %s: %v", targets, err)
	}
	if err := os.WriteFile(filepath.Join(targets, name+".password"), []byte("minted\n"), 0o600); err != nil {
		t.Fatalf("writing the remembered password: %v", err)
	}

	res, err := Resolve(t.Context(), Options{
		Workdir: dir, NeedTarget: true,
		Source:     "postgres://app@127.0.0.1:1/shop",
		DockerHost: localDockerHost,
		Config: &pipeline.Config{
			Target:      pipeline.FromContainer,
			TargetLabel: name,
			TargetRef:   mustRef(t, "postgres://postgres@127.0.0.1:5433/postgres"),
		},
		dial:        fakeDial(&fakeDocker{}),
		provisioner: refuseToProvision(t),
	}, event.Discard)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.Target != "postgres://postgres:minted@127.0.0.1:5433/postgres" {
		t.Errorf("target = %q, want the remembered credential put back", res.Target)
	}
}

// The remembered credential is only ever read for a container name this run
// would itself have used. The label in lazyslice.yml is committed text, so a
// file naming somebody else's container — or a path — must not become a file
// this reads.
func TestARememberedPasswordIsNotHandedToAnEndpointTheFileNames(t *testing.T) {
	quietEnvironment(t)
	dir := t.TempDir()
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)

	targets := filepath.Join(state, "lazyslice", "targets")
	if err := os.MkdirAll(targets, 0o700); err != nil {
		t.Fatalf("creating %s: %v", targets, err)
	}
	if err := os.WriteFile(filepath.Join(targets, provision.Name(projectName(dir))+".password"), []byte("minted\n"), 0o600); err != nil {
		t.Fatalf("writing the remembered password: %v", err)
	}

	for _, label := range []string{"someone-elses-db", "../" + provision.Name(projectName(dir)), ""} {
		res, err := Resolve(t.Context(), Options{
			Workdir: dir, NeedTarget: true,
			Source:     "postgres://app@127.0.0.1:1/shop",
			DockerHost: localDockerHost,
			Config: &pipeline.Config{
				Target:      pipeline.FromContainer,
				TargetLabel: label,
				TargetRef:   mustRef(t, "postgres://postgres@127.0.0.1:5433/postgres"),
			},
			dial:        fakeDial(&fakeDocker{}),
			provisioner: refuseToProvision(t),
		}, event.Discard)
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if strings.Contains(res.Target, "minted") {
			t.Errorf("label %q was handed a credential this run did not mint for it", label)
		}
	}
}

// ADR-008 §3's own owed test: with the endpoint resolved to a remote daemon, a
// container publishing 0.0.0.0:5432 produces no candidate, none is marked
// Local, Q1 does not fire, and --create-target exits 4 without calling the
// provisioner.
func TestRemoteDockerEndpointYieldsNoLocalCandidate(t *testing.T) {
	quietEnvironment(t) // points DOCKER_HOST at tcp://staging.example:2375
	dir := t.TempDir()
	api := &fakeDocker{list: []container.Summary{
		pgContainer("db", "/shop-db-1", "postgres:16", 5432, nil),
	}}
	asked := &fakePrompter{answer: true}

	_, err := Resolve(t.Context(), Options{
		Workdir: dir, NeedTarget: true, CreateTarget: true,
		Source:      "postgres://app@127.0.0.1:1/shop",
		dial:        fakeDial(api),
		provisioner: refuseToProvision(t),
		Prompter:    asked,
	}, event.Discard)

	r, ok := AsRefusal(err)
	if !ok {
		t.Fatalf("want a refusal, got %v", err)
	}
	if r.Code != CodeTargetDockerNotLocal || r.Exit != 4 {
		t.Errorf("refusal = %s exit %d, want target.refused.docker_not_local exit 4", r.Code, r.Exit)
	}
	if len(asked.questions) != 0 {
		t.Errorf("Q1 fired on a non-local endpoint: %q", asked.questions)
	}
}

// Q1' (ADR-008 §6): the only target-shaped candidate is a stopped container, so
// it is asked before the gate runs, yes starts that container, and the started
// endpoint is what the run loads into.
func TestStoppedTargetIsAskedBeforeTheGate(t *testing.T) {
	quietEnvironment(t)
	dir := t.TempDir()
	api := &fakeDocker{
		list: []container.Summary{stoppedPgContainer("db", "/shop-db-1", "postgres:16", map[string]string{
			labelWorkingDir: dir,
			labelService:    "db",
		})},
		env:  map[string][]string{"db": {"POSTGRES_USER=app", "POSTGRES_DB=shop", "POSTGRES_PASSWORD=hunter2"}},
		host: map[string]*container.HostConfig{"db": binding5432(5459)},
	}
	p := &fakeProvisioner{result: provisioned("postgres://app:hunter2@127.0.0.1:5459/shop")}
	asked := &fakePrompter{answer: true}

	res, err := Resolve(t.Context(), Options{
		Workdir: dir, NeedTarget: true,
		Source:        "postgres://app@127.0.0.1:1/shop",
		DockerHost:    localDockerHost,
		dial:          fakeDial(api),
		dialCandidate: sayVersion(16),
		provisioner:   handOut(p),
		Prompter:      asked,
	}, event.Discard)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(asked.questions) != 1 || !strings.Contains(asked.questions[0], "is stopped") {
		t.Fatalf("questions = %q, want one Q1' naming the stopped container", asked.questions)
	}
	if len(p.started) != 1 || p.started[0] != "db" {
		t.Errorf("started = %v, want the stopped container's own id", p.started)
	}
	if len(p.provisioned) != 0 {
		t.Error("Q1' created a container; it may only start one that already exists")
	}
	if res.Target != "postgres://app:hunter2@127.0.0.1:5459/shop" {
		t.Errorf("target = %q, want the started container", res.Target)
	}
}

// Q1' takes its default without a controlling terminal and under --yes, because
// starting a container the developer already has is neither creating nor
// destroying (ADR-008 §6 step 2). No is the same stop Q1 takes headlessly.
func TestStoppedTargetHeadlessStartsAndNoStops(t *testing.T) {
	for _, tc := range []struct {
		name      string
		opts      func(*Options)
		wantStart bool
	}{
		{name: "--yes takes the default and starts it", opts: func(o *Options) { o.Yes = true }, wantStart: true},
		{name: "no stops the run", opts: func(o *Options) { o.Prompter = &fakePrompter{answer: false} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			quietEnvironment(t)
			dir := t.TempDir()
			api := &fakeDocker{
				list: []container.Summary{stoppedPgContainer("db", "/shop-db-1", "postgres:16", map[string]string{
					labelWorkingDir: dir,
				})},
				host: map[string]*container.HostConfig{"db": binding5432(5460)},
			}
			p := &fakeProvisioner{result: provisioned("postgres://postgres@127.0.0.1:5460/postgres")}
			o := Options{
				Workdir: dir, NeedTarget: true,
				Source:        "postgres://app@127.0.0.1:1/shop",
				DockerHost:    localDockerHost,
				dial:          fakeDial(api),
				dialCandidate: sayVersion(16),
				provisioner:   handOut(p),
			}
			tc.opts(&o)

			_, err := Resolve(t.Context(), o, event.Discard)
			if tc.wantStart {
				if err != nil {
					t.Fatalf("Resolve: %v", err)
				}
				if len(p.started) != 1 {
					t.Fatalf("started %d containers, want the default yes to have started one", len(p.started))
				}
				return
			}
			r, ok := AsRefusal(err)
			if !ok {
				t.Fatalf("want a refusal after no, got %v", err)
			}
			if r.Code != CodeTargetNone || r.Args[event.ArgFlag] != "--create-target" {
				t.Errorf("refusal = %s flag %q, want target.refused.none naming --create-target (ADR-008 §6 step 5)",
					r.Code, r.Args[event.ArgFlag])
			}
			if len(p.started) != 0 {
				t.Error("a no answer started a container")
			}
		})
	}
}

// ADR-008 §6 step 3: the container came up and did not answer, so the run stops
// at exit 4 with target.refused.start_timeout and the elapsed wait.
func TestAStartThatNeverAnswersIsAStartTimeout(t *testing.T) {
	quietEnvironment(t)
	dir := t.TempDir()
	api := &fakeDocker{
		list: []container.Summary{stoppedPgContainer("db", "/shop-db-1", "postgres:16", map[string]string{
			labelWorkingDir: dir,
		})},
		host: map[string]*container.HostConfig{"db": binding5432(5461)},
	}
	p := &fakeProvisioner{err: &provision.NotReadyError{Container: "shop-db-1", Waited: 60 * 1e9}}

	_, err := Resolve(t.Context(), Options{
		Workdir: dir, NeedTarget: true, Yes: true,
		Source:        "postgres://app@127.0.0.1:1/shop",
		DockerHost:    localDockerHost,
		dial:          fakeDial(api),
		dialCandidate: sayVersion(16),
		provisioner:   handOut(p),
	}, event.Discard)

	r, ok := AsRefusal(err)
	if !ok {
		t.Fatalf("want a refusal, got %v", err)
	}
	if r.Code != CodeTargetStartTimeout || r.Exit != 4 {
		t.Errorf("refusal = %s exit %d, want target.refused.start_timeout exit 4", r.Code, r.Exit)
	}
	if r.Args[event.ArgSeconds] != "60" {
		t.Errorf("seconds = %q, want the elapsed wait", r.Args[event.ArgSeconds])
	}
}

// A stopped container is on the ladder and is never eligible: it prints as
// unreachable with its own reason, and no gate rule can be evaluated for it,
// which is why Q1' is asked before the gate at all (ADR-008 §6).
func TestAStoppedContainerIsACandidateAndNeverReachable(t *testing.T) {
	quietEnvironment(t)
	dir := t.TempDir()
	api := &fakeDocker{
		list: []container.Summary{stoppedPgContainer("db", "/shop-db-1", "postgres:16", map[string]string{
			labelWorkingDir: dir,
			labelService:    "db",
		})},
		host: map[string]*container.HostConfig{"db": binding5432(5462)},
	}

	var events []event.Event
	sink := event.SinkFunc(func(e event.Event) { events = append(events, e) })

	cands, _ := walk(t.Context(), Options{
		Workdir:    dir,
		DockerHost: localDockerHost,
		dial:       fakeDial(api),
		dialCandidate: func(context.Context, *found) {
			t.Error("a stopped container was dialled; it cannot answer and the dial budget is 1 s per candidate")
		},
	}, sink)

	if len(cands) != 1 {
		t.Fatalf("got %d candidates, want the stopped container: %+v", len(cands), cands)
	}
	got := cands[0]
	if got.cand.Provenance != pipeline.FromStoppedContainer {
		t.Errorf("provenance = %v, want rung 4", got.cand.Provenance)
	}
	if got.cand.Reachable {
		t.Error("a stopped container is reachable, which would let the gate run rules on it")
	}
	if got.containerID != "db" {
		t.Errorf("containerID = %q, want the id Q1' hands to provision.Start", got.containerID)
	}
	if got.cand.Ref.Port != 5462 {
		t.Errorf("ref = %s, want the port the container is configured to publish", got.cand.Ref)
	}
	if !hasCode(events, CodeCandidateUnreachable) {
		t.Error("the stopped container never printed, so a developer sees an empty ladder instead of a reason")
	}
}

// ADR-008 §7's own owed test, verbatim: stdin is a pipe holding "y", a
// controlling terminal is present and answers nothing, and the assertion is
// that no container was created and that the pipe was not drained.
//
// This is the behavioural half of the rule; TestPromptsNeverReadStandardInput
// below is the cheap static guard beside it. The reason the rule exists is
// safety rather than ergonomics: Q1's yes branch creates a container and Q4's
// answer is a password, so `echo y | lazyslice` from a terminal must ask the
// human at the terminal and leave the piped y sitting unread.
func TestPipedStdinIsNotAnAnswer(t *testing.T) {
	quietEnvironment(t)
	dir := t.TempDir()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer func() { _ = r.Close() }()
	if _, werr := io.WriteString(w, "y\n"); werr != nil {
		t.Fatalf("writing to the pipe: %v", werr)
	}
	if cerr := w.Close(); cerr != nil {
		t.Fatalf("closing the pipe: %v", cerr)
	}
	stdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = stdin })

	// A controlling terminal that is present and answers nothing. Its EOF is
	// the terminal going away mid-question, which takes the same path as no
	// terminal at all — and never the piped byte.
	var asked bytes.Buffer
	silent := &prompt{out: &asked, tty: io.NopCloser(strings.NewReader("")), in: bufio.NewReader(strings.NewReader(""))}

	p := &fakeProvisioner{result: provisioned("postgres://postgres:pw@127.0.0.1:5433/postgres")}
	_, err = Resolve(t.Context(), Options{
		Workdir: dir, NeedTarget: true,
		Source:        "postgres://app@127.0.0.1:1/shop",
		DockerHost:    localDockerHost,
		dial:          fakeDial(&fakeDocker{}),
		dialCandidate: sayVersion(16),
		provisioner:   handOut(p),
		Prompter:      silent,
	}, event.Discard)

	if !strings.Contains(asked.String(), "start one?") {
		t.Fatalf("Q1 was never asked: %q", asked.String())
	}
	if len(p.provisioned) != 0 || len(p.started) != 0 {
		t.Errorf("a container was created from a byte on stdin: provisioned %d, started %v", len(p.provisioned), p.started)
	}
	refusal, ok := AsRefusal(err)
	if !ok || refusal.Code != CodeTargetNone {
		t.Errorf("err = %v, want target.refused.none: a terminal that answers nothing is the headless answer", err)
	}
	left, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("reading the pipe back: %v", err)
	}
	if string(left) != "y\n" {
		t.Errorf("the pipe holds %q, want it undrained: stdin was read for an answer", left)
	}
}

// ADR-008 §7: prompts are read from the controlling terminal, whatever stdin
// is. os.Stdin is never read, never checked to decide whether to ask, and never
// consulted at all. This is the cheap static guard beside
// TestPipedStdinIsNotAnAnswer above: it catches the exact
// bufio.NewReader(os.Stdin) pattern the ADR names, in every non-test file of
// this package, and the behavioural test is what covers reaching stdin by
// another route.
func TestPromptsNeverReadStandardInput(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading the package directory: %v", err)
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		// Comments are stripped first: this file's own prose says os.Stdin
		// several times, and so does the code it is checking.
		parsed, err := parser.ParseFile(token.NewFileSet(), filepath.Clean(name), nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		var body bytes.Buffer
		if err := printer.Fprint(&body, token.NewFileSet(), parsed); err != nil {
			t.Fatalf("printing %s: %v", name, err)
		}
		if bytes.Contains(body.Bytes(), []byte("os.Stdin")) {
			t.Errorf("%s names os.Stdin; ADR-008 §7 reads answers from the controlling terminal only", name)
		}
	}
}

// ADR-008 §7's answer grammar: the bracketed letter is the default and a bare
// Enter selects it; anything else re-prompts once and then takes the default.
func TestConfirmGrammar(t *testing.T) {
	for _, tc := range []struct {
		typed    string
		def      bool
		want     bool
		wantAsks int
	}{
		{typed: "\n", def: true, want: true, wantAsks: 1},
		{typed: "\n", def: false, want: false, wantAsks: 1},
		{typed: "y\n", def: false, want: true, wantAsks: 1},
		{typed: "YES\n", def: false, want: true, wantAsks: 1},
		{typed: "n\n", def: true, want: false, wantAsks: 1},
		{typed: " No \n", def: true, want: false, wantAsks: 1},
		{typed: "maybe\ny\n", def: false, want: true, wantAsks: 2},
		{typed: "maybe\nperhaps\n", def: true, want: true, wantAsks: 2},
		{typed: "", def: true, want: true, wantAsks: 1},
	} {
		var out bytes.Buffer
		p := &prompt{out: &out, tty: io.NopCloser(strings.NewReader("")), in: bufio.NewReader(strings.NewReader(tc.typed))}
		got, err := p.Confirm("start it? [Y/n]", tc.def)
		if err != nil && !errors.Is(err, ErrNoTerminal) {
			t.Fatalf("Confirm(%q): %v", tc.typed, err)
		}
		if got != tc.want {
			t.Errorf("Confirm(%q, def=%v) = %v, want %v", tc.typed, tc.def, got, tc.want)
		}
		if asks := strings.Count(out.String(), "start it?"); asks != tc.wantAsks {
			t.Errorf("Confirm(%q) asked %d times, want %d", tc.typed, asks, tc.wantAsks)
		}
	}
}

// ---------- helpers ----------

// fakePrompter answers without a terminal, which is also what keeps a test run
// started from a terminal from blocking on a real question.
type fakePrompter struct {
	answer    bool
	questions []string
}

func (f *fakePrompter) Confirm(question string, _ bool) (bool, error) {
	f.questions = append(f.questions, question)
	return f.answer, nil
}

// Ask is unused by this package's own tests — Q1 and Q1' are Confirm's only —
// and exists so fakePrompter still satisfies Prompter now that it carries Q2's
// method too (internal/core's own Q2 tests have their own fake).
func (f *fakePrompter) Ask(question string, def string) (string, error) {
	f.questions = append(f.questions, question)
	return def, nil
}

func (f *fakePrompter) Close() error { return nil }

// fakeProvisioner records what it was asked to create or start.
type fakeProvisioner struct {
	provisioned []provision.Request
	started     []string
	result      provision.Result
	err         error
}

func (f *fakeProvisioner) Provision(_ context.Context, req provision.Request) (provision.Result, error) {
	f.provisioned = append(f.provisioned, req)
	if f.err != nil {
		return provision.Result{}, f.err
	}
	return f.result, nil
}

func (f *fakeProvisioner) Start(_ context.Context, id string, _ provision.Request) (provision.Result, error) {
	f.started = append(f.started, id)
	if f.err != nil {
		return provision.Result{}, f.err
	}
	return f.result, nil
}

func handOut(p provision.Provisioner) func(dockerctx.Endpoint) (provision.Provisioner, error) {
	return func(dockerctx.Endpoint) (provision.Provisioner, error) { return p, nil }
}

// refuseToProvision fails the test if a provisioner is built at all, which is
// the assertion for every path that must stop before any container work.
func refuseToProvision(t *testing.T) func(dockerctx.Endpoint) (provision.Provisioner, error) {
	t.Helper()
	return func(dockerctx.Endpoint) (provision.Provisioner, error) {
		t.Error("a provisioner was built on a path that must create nothing")
		return nil, errors.New("no provisioner here")
	}
}

// provisioned is a fake provisioner's answer: the candidate and the connection
// string, which is the widening this task landed — a Candidate's Ref cannot
// carry the generated password.
func provisioned(connURL string) provision.Result {
	_, ref, err := dsn.Parse(connURL)
	if err != nil {
		panic(err)
	}
	return provision.Result{
		Candidate: pipeline.Candidate{
			Ref:        ref,
			Provenance: pipeline.FromContainer,
			Label:      "lazyslice-target-test",
			Local:      true,
		},
		DSN:       connURL,
		Container: "lazyslice-target-test",
		Created:   true,
	}
}

// sayVersion is a dial that answers, so that a test can exercise the paths that
// need a server major without a server.
func sayVersion(major int) func(context.Context, *found) {
	return func(_ context.Context, f *found) {
		f.cand.Reachable = true
		f.cand.ConnectErr = ""
		f.cand.Version = major
		f.cand.EmptyHint = true
	}
}

// stoppedPgContainer is an exited Postgres container. It publishes nothing,
// because a container that is not running does not: rung 4 reads the binding it
// is configured with instead.
func stoppedPgContainer(id, name, image string, labels map[string]string) container.Summary {
	return container.Summary{
		ID:     id,
		Names:  []string{name},
		Image:  image,
		Labels: labels,
		State:  container.StateExited,
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

func binding5432(port int) *container.HostConfig {
	return &container.HostConfig{PortBindings: network.PortMap{
		network.MustParsePort("5432/tcp"): {{HostPort: itoa(port)}},
	}}
}
