// SPDX-License-Identifier: Apache-2.0

//go:build integration

package core

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/client"

	"github.com/Liarea/lazyslice/internal/discover"
	"github.com/Liarea/lazyslice/internal/discover/dockerctx"
	"github.com/Liarea/lazyslice/internal/discover/provision"
	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/testutil"
)

// TestTwoRunsFromOneDirectoryReuseTheContainerTheFirstMade is T-0334 (dogfood
// session 3) end to end: two runs from one directory with nothing but --source,
// headless and without --yes, against the container lazyslice-target-<project>
// a first run's Q1 created and left running — empty, labelled with the
// project, its database the `postgres` that provisioning creates.
//
// The first run is the one the dogfood session broke on: the ladder listed the
// container, dropped it under the maintenance-database rule, and headless
// stopped at exit 4 target.refused.none (at a terminal, it asked Q1 again for
// the same name on the next port). It must choose the container and load. The
// second run then starts from the lazyslice.yml the first one wrote and must
// load into the same container again. Neither run may ask anything, create a
// second container, or stop.
//
// The container is made by calling the provisioner Q1's yes calls, rather than
// by a first Run answering Q1: whether Q1 fires at all on a run from a fresh
// directory depends on what else is running on the machine's Docker daemon
// (ADR-008 §4's working-dir filter falls back to every container), and a test
// that let the ladder rank another test's database could write into it.
func TestTwoRunsFromOneDirectoryReuseTheContainerTheFirstMade(t *testing.T) {
	ctx := t.Context()
	testutil.SkipWithoutDocker(ctx, t)
	quietRungs(t)
	// provision remembers the generated password in the machine-local state
	// dir; this keeps it out of the developer's own.
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	endpoint, err := dockerctx.Resolve(ctx, "")
	if err != nil || !endpoint.Local() || endpoint.SSH() {
		t.Skipf("no local docker endpoint: %v", endpoint)
	}
	api, err := dockerctx.Dial(endpoint)
	if err != nil {
		t.Skipf("docker endpoint %s did not open: %v", endpoint, err)
	}

	// A directory whose base name is already a clean compose project name, so
	// the container name below is the one discover computes for it.
	project := "t0334-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	dir := filepath.Join(t.TempDir(), project)
	if mkErr := os.Mkdir(dir, 0o700); mkErr != nil {
		t.Fatalf("making the working directory: %v", mkErr)
	}
	name := provision.Name(project)
	t.Cleanup(func() {
		clean, cancel := context.WithTimeout(context.WithoutCancel(ctx), 60*time.Second)
		defer cancel()
		if _, rmErr := api.ContainerRemove(clean, name, client.ContainerRemoveOptions{
			Force: true, RemoveVolumes: true,
		}); rmErr != nil {
			t.Logf("removing %s: %v", name, rmErr)
		}
		if _, rmErr := api.VolumeRemove(clean, provision.Volume(project), client.VolumeRemoveOptions{Force: true}); rmErr != nil {
			t.Logf("removing %s: %v", provision.Volume(project), rmErr)
		}
	})

	source := testutil.Postgres(ctx, t, "")
	execOn(ctx, t, source,
		`CREATE TABLE customer (id int PRIMARY KEY, email text)`,
		`INSERT INTO customer VALUES (1, 'a@example.com'), (2, 'b@example.com')`,
	)

	made, err := provision.New(api).Provision(ctx, provision.Request{
		Project: project, Workdir: dir, Major: sourceImageMajor(t),
	})
	if err != nil {
		t.Fatalf("provisioning what the first run's Q1 left behind: %v", err)
	}
	if made.Candidate.Ref.Database != "postgres" {
		t.Fatalf("provisioned database %q: this test pins the `postgres` database provisioning creates",
			made.Candidate.Ref.Database)
	}

	for run := 1; run <= 2; run++ {
		var codes []event.Code
		sink := event.SinkFunc(func(e event.Event) { codes = append(codes, e.Code) })

		// --source and nothing else. noTerminal is "no controlling terminal"
		// on demand — the CI shape, and never --yes — so that a Q1 that should
		// not fire cannot open the developer's /dev/tty and wait.
		_, err := Run(ctx, Request{
			Mode:       ModeRun,
			Workdir:    dir,
			Source:     source,
			ConfigPath: filepath.Join(dir, "lazyslice.yml"),
			SecretFile: filepath.Join(dir, "lazyslice.secret"),
			noTerminal: true,
		}, sink)
		if err != nil {
			t.Fatalf("run %d from %s = %v, want it to load into %s without a question", run, dir, err, name)
		}
		for _, refused := range []event.Code{discover.CodeTargetNone, discover.CodeTargetNameTaken} {
			if n := count(codes, refused); n != 0 {
				t.Errorf("run %d emitted %s %d time(s)", run, refused, n)
			}
		}
		if n := count(codes, CodeTargetChosen); n != 1 {
			t.Errorf("run %d printed the target decision %d time(s), want once", run, n)
		}
		if n := userTables(ctx, t, made.DSN); n == 0 {
			t.Errorf("run %d left %s's postgres database with no tables: it loaded somewhere else", run, name)
		}

		list, err := api.ContainerList(ctx, client.ContainerListOptions{
			All:     true,
			Filters: make(client.Filters).Add("label", provision.LabelProject+"="+project),
		})
		if err != nil {
			t.Fatalf("listing the project's containers: %v", err)
		}
		if len(list.Items) != 1 {
			var names []string
			for _, c := range list.Items {
				names = append(names, strings.Join(c.Names, ","))
			}
			t.Errorf("after run %d the project has %d container(s) %v, want the one it already had",
				run, len(list.Items), names)
		}
	}
}

// sourceImageMajor is the server major of the image testutil.Postgres runs,
// which is the major a first run's Q1 would have provisioned for this source.
func sourceImageMajor(t *testing.T) int {
	t.Helper()
	image := os.Getenv(testutil.ImageEnv)
	if image == "" {
		image = testutil.DefaultImage
	}
	_, tag, ok := strings.Cut(image, ":")
	if !ok {
		t.Skipf("cannot read a major out of %q", image)
	}
	n, err := strconv.Atoi(strings.SplitN(tag, ".", 2)[0])
	if err != nil {
		t.Skipf("cannot read a major out of %q", image)
	}
	return n
}
