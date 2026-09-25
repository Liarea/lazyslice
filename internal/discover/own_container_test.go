// SPDX-License-Identifier: Apache-2.0

package discover

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moby/moby/api/types/container"

	"github.com/Liarea/lazyslice/internal/discover/provision"
	"github.com/Liarea/lazyslice/internal/dsn"
	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pipeline"
)

// ownEnv is the environment provisioning gives a container: POSTGRES_DB is the
// maintenance database's name, which is why T-0334 exists. The password is a
// synthetic string, not any provider's credential format.
var ownEnv = []string{"POSTGRES_USER=postgres", "POSTGRES_DB=postgres", "POSTGRES_PASSWORD=q7Rk2vXw9LmP4tZs"}

// ownLabels are the labels provision stamps on a container it creates.
func ownLabels(dir string) map[string]string {
	return map[string]string{
		provision.LabelProject:    projectName(dir),
		provision.LabelWorkingDir: dir,
	}
}

// T-0334, dogfood session 3: the second run from the directory whose first run
// created lazyslice-target-<project>, with that container still running, empty
// or carrying lazyslice's marker. The ladder lists it and must choose it: no
// Q1, no second container proposed on the next port, and no exit 4 headless.
// Its database is `postgres`, the maintenance database the 2026-09-15 red team
// rule keeps off every other container, which is what excluded it before.
func TestTheProjectsOwnRunningContainerIsChosenWithoutAQuestion(t *testing.T) {
	for _, tc := range []struct {
		name string
		// dial is what the running container answers: empty, or carrying
		// the marker a previous load left with rows behind it.
		dial     func(context.Context, *found)
		headless bool
	}{
		{name: "empty, at a terminal", dial: sayVersion(16)},
		{name: "empty, headless with no --yes", dial: sayVersion(16), headless: true},
		{name: "carries the marker, headless", headless: true, dial: func(_ context.Context, f *found) {
			f.cand.Reachable, f.cand.ConnectErr, f.cand.Version = true, "", 16
			f.cand.Tables, f.cand.EmptyHint, f.cand.Marked = 3, false, true
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			quietEnvironment(t)
			dir := t.TempDir()
			name := provision.Name(projectName(dir))
			api := &fakeDocker{
				list: []container.Summary{pgContainer("own", "/"+name, "postgres:16", 5433, ownLabels(dir))},
				env:  map[string][]string{"own": ownEnv},
			}
			asked := &fakePrompter{answer: true}
			o := Options{
				Workdir: dir, NeedTarget: true,
				Source:        "postgres://app@127.0.0.1:1/shop",
				DockerHost:    localDockerHost,
				dial:          fakeDial(api),
				dialCandidate: tc.dial,
				provisioner:   refuseToProvision(t),
				Prompter:      asked,
			}
			if tc.headless {
				o.Prompter, o.NoControllingTerminal = nil, true
			}

			res, err := Resolve(t.Context(), o, event.Discard)
			if err != nil {
				t.Fatalf("Resolve: %v, want the project's own container chosen", err)
			}
			if len(asked.questions) != 0 {
				t.Errorf("questions = %q, want none: the container this directory made is the target", asked.questions)
			}
			if res.Asked {
				t.Error("Result.Asked is true, but no question was put to the terminal")
			}
			if _, ref, perr := dsn.Parse(res.Target); perr != nil || ref.Port != 5433 || ref.Database != "postgres" {
				t.Errorf("target = %s (%v), want the running container's own database on 5433", ref, perr)
			}
			if res.TargetProvenance != pipeline.FromContainer || res.TargetLabel != name {
				t.Errorf("target from %v %q, want container %s", res.TargetProvenance, res.TargetLabel, name)
			}
			if res.TargetContainerID != "own" {
				t.Errorf("TargetContainerID = %q, want the container behind the target", res.TargetContainerID)
			}
		})
	}
}

// The other half of T-0334's pin: a stopped container of the project's own is
// still ADR-008 §6's Q1', exactly as before — asked, started, nothing created.
func TestTheProjectsOwnStoppedContainerIsStillQ1Prime(t *testing.T) {
	quietEnvironment(t)
	dir := t.TempDir()
	name := provision.Name(projectName(dir))
	api := &fakeDocker{
		list: []container.Summary{stoppedPgContainer("own", "/"+name, "postgres:16", ownLabels(dir))},
		env:  map[string][]string{"own": ownEnv},
		host: map[string]*container.HostConfig{"own": binding5432(5433)},
	}
	p := &fakeProvisioner{result: provisioned("postgres://postgres:q7Rk2vXw9LmP4tZs@127.0.0.1:5433/postgres")}
	asked := &fakePrompter{answer: true}

	if _, err := Resolve(t.Context(), Options{
		Workdir: dir, NeedTarget: true,
		Source:        "postgres://app@127.0.0.1:1/shop",
		DockerHost:    localDockerHost,
		dial:          fakeDial(api),
		dialCandidate: sayVersion(16),
		provisioner:   handOut(p),
		Prompter:      asked,
	}, event.Discard); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(asked.questions) != 1 || !strings.Contains(asked.questions[0], "target "+name+" is stopped") {
		t.Fatalf("questions = %q, want one Q1' naming %s", asked.questions, name)
	}
	if len(p.started) != 1 || p.started[0] != "own" {
		t.Errorf("started = %v, want the stopped container", p.started)
	}
	if len(p.provisioned) != 0 {
		t.Error("Q1' created a container; it may only start one that already exists")
	}
}

// Q1 never proposes a container name that already exists (T-0334). The
// ladder could not rank the container — it did not answer the 1 s dial, or it
// is not lazyslice's — so Q1 would have fired naming it.
func TestQ1NeverProposesANameThatIsTaken(t *testing.T) {
	unreachable := func(_ context.Context, f *found) {
		f.cand.Reachable, f.cand.ConnectErr = false, "did not answer within 1s"
	}
	// otherCheckout is the same project made from another directory with the
	// same basename, such as a second checkout or a git worktree: the project
	// label matches and the working dir does not.
	otherCheckout := func(dir string) map[string]string {
		return map[string]string{
			provision.LabelProject:    projectName(dir),
			provision.LabelWorkingDir: filepath.Join(filepath.Dir(filepath.Dir(dir)), "elsewhere", filepath.Base(dir)),
		}
	}
	for _, tc := range []struct {
		name      string
		labels    func(dir string) map[string]string
		dial      func(context.Context, *found)
		wantStart bool
		wantCode  event.Code
	}{
		{name: "ours, still booting: reused through Start", labels: ownLabels, wantStart: true},
		{name: "not ours: refused, never touched", labels: func(string) map[string]string { return nil },
			wantCode: CodeTargetNameTaken},
		{name: "another checkout's, booting: refused, never touched", labels: otherCheckout,
			wantCode: CodeTargetNameTaken},
		{name: "another checkout's, running and empty: not chosen, refused", labels: otherCheckout,
			dial: sayVersion(16), wantCode: CodeTargetNameTaken},
	} {
		t.Run(tc.name, func(t *testing.T) {
			quietEnvironment(t)
			dir := t.TempDir()
			name := provision.Name(projectName(dir))
			dial := unreachable
			if tc.dial != nil {
				dial = tc.dial
			}
			api := &fakeDocker{
				list: []container.Summary{pgContainer("own", "/"+name, "postgres:16", 5433, tc.labels(dir))},
				env:  map[string][]string{"own": ownEnv},
			}
			p := &fakeProvisioner{result: provisioned("postgres://postgres:q7Rk2vXw9LmP4tZs@127.0.0.1:5433/postgres")}
			asked := &fakePrompter{answer: true}
			var events []event.Event
			sink := event.SinkFunc(func(e event.Event) { events = append(events, e) })

			_, err := Resolve(t.Context(), Options{
				Workdir: dir, NeedTarget: true,
				Source:        "postgres://app@127.0.0.1:1/shop",
				DockerHost:    localDockerHost,
				dial:          fakeDial(api),
				dialCandidate: dial,
				provisioner:   handOut(p),
				Prompter:      asked,
			}, sink)

			for _, q := range asked.questions {
				if strings.Contains(q, "start one?") {
					t.Errorf("Q1 proposed %s, which already exists: %q", name, q)
				}
			}
			if len(p.provisioned) != 0 {
				t.Errorf("provisioned %d container(s), want none", len(p.provisioned))
			}
			if tc.wantStart {
				if err != nil {
					t.Fatalf("Resolve: %v", err)
				}
				if len(p.started) != 1 || p.started[0] != name {
					t.Errorf("started = %v, want %s", p.started, name)
				}
				return
			}
			r, ok := AsRefusal(err)
			if !ok || r.Code != tc.wantCode || r.Exit != exitTarget {
				t.Fatalf("err = %v, want %s exit 4", err, tc.wantCode)
			}
			if r.Args[event.ArgContainer] != name || r.Args[event.ArgFlag] != "--target" {
				t.Errorf("args = %v, want the container and --target", r.Args)
			}
			if !hasError(events, tc.wantCode, exitTarget) {
				t.Error("the refusal never reached the sink")
			}
			if len(p.started) != 0 {
				t.Error("a container lazyslice did not create was started")
			}
		})
	}
}

// T-0335: --create-target names lazyslice-target-<project> the same way Q1
// does, and since T-0333 that name strips a leading "lazyslice-"/"lazyslice_"
// segment from the project — so "shop" and "lazyslice-shop", checked out in
// different directories, both name lazyslice-target-shop. --create-target
// never asks a question and so never went through reuseOwn's name_taken
// guard; this pins that provisionTarget refuses instead of adopting, starting
// or loading into the other directory's container.
func TestCreateTargetRefusesAnotherProjectsSameNamedContainer(t *testing.T) {
	quietEnvironment(t)
	shopDir := filepath.Join(t.TempDir(), "shop")
	if err := os.Mkdir(shopDir, 0o700); err != nil {
		t.Fatal(err)
	}
	// otherDir is a different directory whose project name is "lazyslice-shop"
	// so provision.Name strips the prefix and collides with shopDir's own
	// "shop" -> lazyslice-target-shop.
	otherDir := filepath.Join(t.TempDir(), "lazyslice-shop")
	if err := os.Mkdir(otherDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if projectName(shopDir) == projectName(otherDir) {
		t.Fatalf("test setup: projects must differ (%q, %q)", projectName(shopDir), projectName(otherDir))
	}
	name := provision.Name(projectName(shopDir))
	if name != provision.Name(projectName(otherDir)) {
		t.Fatalf("test setup: both projects must name %s", name)
	}

	api := &fakeDocker{
		// otherDir's own container already answers to the collided name.
		list: []container.Summary{pgContainer("other", "/"+name, "postgres:16", 5433, ownLabels(otherDir))},
		env:  map[string][]string{"other": ownEnv},
	}
	p := &fakeProvisioner{result: provisioned("postgres://postgres:q7Rk2vXw9LmP4tZs@127.0.0.1:5433/postgres")}
	var events []event.Event
	sink := event.SinkFunc(func(e event.Event) { events = append(events, e) })

	_, err := Resolve(t.Context(), Options{
		Workdir: shopDir, NeedTarget: true, CreateTarget: true,
		Source:        "postgres://app@127.0.0.1:1/shop",
		DockerHost:    localDockerHost,
		dial:          fakeDial(api),
		dialCandidate: sayVersion(16),
		provisioner:   handOut(p),
	}, sink)

	if len(p.provisioned) != 0 {
		t.Errorf("provisioned %d container(s), want none", len(p.provisioned))
	}
	if len(p.started) != 0 {
		t.Errorf("started = %v, want none — the other directory's container must not be started", p.started)
	}
	r, ok := AsRefusal(err)
	if !ok || r.Code != CodeTargetNameTaken || r.Exit != exitTarget {
		t.Fatalf("err = %v, want %s exit 4", err, CodeTargetNameTaken)
	}
	if r.Args[event.ArgContainer] != name || r.Args[event.ArgFlag] != "--target" {
		t.Errorf("args = %v, want the container and --target", r.Args)
	}
	if !hasError(events, CodeTargetNameTaken, exitTarget) {
		t.Error("the refusal never reached the sink")
	}
}
