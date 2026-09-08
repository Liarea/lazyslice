// SPDX-License-Identifier: Apache-2.0

//go:build integration

package provision

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/moby/moby/client"

	"github.com/Liarea/lazyslice/internal/discover/dockerctx"
	"github.com/Liarea/lazyslice/internal/testutil"
)

// major is the server the provisioned container runs. It matches the image the
// rest of the suite already pulls, so a laptop with a warm cache runs this test
// without a download and the CI matrix exercises whichever major it is set to.
func major(t *testing.T) int {
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

// Provisioning against a real daemon, end to end: the container comes up, a
// client connects with the generated password, and a second call reuses the
// same container rather than creating a second one.
//
// It removes what it made, which lazyslice itself never does: a test that left
// a container behind would be a test that changed the machine it ran on.
func TestProvisionCreatesAndReusesARealContainer(t *testing.T) {
	ctx := t.Context()
	testutil.SkipWithoutDocker(ctx, t)
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	endpoint, err := dockerctx.Resolve(ctx, "")
	if err != nil {
		t.Skipf("no docker endpoint: %v", err)
	}
	if !endpoint.Local() {
		t.Skipf("docker endpoint %s is not local; provisioning is refused before this point", endpoint)
	}
	api, err := dockerctx.Dial(endpoint)
	if err != nil {
		t.Skipf("docker endpoint %s did not open: %v", endpoint, err)
	}

	project := "lazyslicetest" + strconv.FormatInt(time.Now().UnixNano(), 36)
	t.Cleanup(func() { removeEverything(t, api, project) })

	p := New(api)
	req := Request{Project: project, Workdir: t.TempDir(), Major: major(t)}

	res, err := p.Provision(ctx, req)
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}
	if !res.Created {
		t.Error("Created is false for a container that did not exist")
	}
	if res.Container != Name(project) {
		t.Errorf("container = %q, want %q", res.Container, Name(project))
	}
	if res.Candidate.Ref.Host != "127.0.0.1" {
		t.Errorf("published on %s, want loopback", res.Candidate.Ref)
	}
	if !res.Candidate.Local {
		t.Error("a container on this machine's daemon is local")
	}

	// The endpoint answers, with the password only the Result carries.
	cfg, err := pgconn.ParseConfig(res.DSN)
	if err != nil {
		t.Fatalf("parsing the provisioned dsn: %v", err)
	}
	conn, err := pgconn.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("connecting to the provisioned container: %v", err)
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()
	if _, execErr := conn.Exec(ctx, "SELECT 1").ReadAll(); execErr != nil {
		t.Fatalf("the provisioned server did not answer: %v", execErr)
	}

	// A second call reuses it. ARCHITECTURE.md section 9: never recreated,
	// never removed.
	again, err := p.Provision(ctx, req)
	if err != nil {
		t.Fatalf("second Provision: %v", err)
	}
	if again.Created {
		t.Error("the second call created a second container")
	}
	if again.Candidate.Ref.Port != res.Candidate.Ref.Port {
		t.Errorf("second call published on %d, want the same container on %d",
			again.Candidate.Ref.Port, res.Candidate.Ref.Port)
	}
}

// The named volume outlives the container, and a container recreated onto it
// must still be able to log in.
//
// ARCHITECTURE.md section 9 asks for a *named* volume, and "docker rm -v
// <container>" removes anonymous volumes only — so removing the container and
// running --create-target again is an ordinary thing to do and leaves the
// cluster on disk. The postgres image skips initdb on a non-empty PGDATA and
// ignores POSTGRES_PASSWORD, so minting a fresh password there produced a
// container that failed authentication for the whole 60 s budget, on every
// retry, and destroyed the one credential that worked in the process.
func TestARecreatedContainerCanStillLogIntoItsSurvivingVolume(t *testing.T) {
	ctx := t.Context()
	testutil.SkipWithoutDocker(ctx, t)
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	endpoint, err := dockerctx.Resolve(ctx, "")
	if err != nil {
		t.Skipf("no docker endpoint: %v", err)
	}
	if !endpoint.Local() {
		t.Skipf("docker endpoint %s is not local; provisioning is refused before this point", endpoint)
	}
	api, err := dockerctx.Dial(endpoint)
	if err != nil {
		t.Skipf("docker endpoint %s did not open: %v", endpoint, err)
	}

	project := "lazyslicevol" + strconv.FormatInt(time.Now().UnixNano(), 36)
	t.Cleanup(func() { removeEverything(t, api, project) })

	p := New(api)
	req := Request{Project: project, Workdir: t.TempDir(), Major: major(t)}
	first, err := p.Provision(ctx, req)
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}

	// Exactly the command this package's doc names, and exactly what it leaves
	// behind: RemoveVolumes is "docker rm -v", which does not remove
	// lazyslice-target-<project>-data.
	if _, rmErr := api.ContainerRemove(ctx, Name(project), client.ContainerRemoveOptions{
		Force: true, RemoveVolumes: true,
	}); rmErr != nil {
		t.Fatalf("removing the container: %v", rmErr)
	}

	second, err := p.Provision(ctx, req)
	if err != nil {
		t.Fatalf("second Provision onto the surviving volume: %v", err)
	}
	if !second.Created {
		t.Error("Created is false for a container this call had to make again")
	}
	cfg, err := pgconn.ParseConfig(second.DSN)
	if err != nil {
		t.Fatalf("parsing the provisioned dsn: %v", err)
	}
	conn, err := pgconn.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("the recreated container refused the credential it was given: %v", err)
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()
	if _, execErr := conn.Exec(ctx, "SELECT 1").ReadAll(); execErr != nil {
		t.Fatalf("the recreated server did not answer: %v", execErr)
	}
	if second.DSN != first.DSN && second.Candidate.Ref.Port == first.Candidate.Ref.Port {
		t.Error("the recreated container was given a different credential for the same cluster")
	}
}

// removeEverything undoes what the test made. It is here and nowhere else:
// lazyslice never removes a container, an image or a volume.
func removeEverything(t *testing.T, api *client.Client, project string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if _, err := api.ContainerRemove(ctx, Name(project), client.ContainerRemoveOptions{
		Force: true, RemoveVolumes: true,
	}); err != nil {
		t.Logf("removing %s: %v", Name(project), err)
	}
	if _, err := api.VolumeRemove(ctx, Volume(project), client.VolumeRemoveOptions{Force: true}); err != nil {
		t.Logf("removing %s: %v", Volume(project), err)
	}
}
