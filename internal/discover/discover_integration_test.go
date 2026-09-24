// SPDX-License-Identifier: Apache-2.0

//go:build integration

package discover

import (
	"context"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/moby/moby/client"

	"github.com/Liarea/lazyslice/internal/discover/dockerctx"
	"github.com/Liarea/lazyslice/internal/discover/provision"
	"github.com/Liarea/lazyslice/internal/dsn"
	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/testutil"
)

// Rung 3 against a real container: the ladder finds a running Postgres through
// the Docker endpoint ADR-008 §3 resolves, dials it inside the 1 s budget, and
// answers the three statements ARCHITECTURE.md §9 allows it.
//
// It is an integration test because every claim in it is a claim about a real
// daemon and a real server: a unit test with a stubbed client would prove that
// the code compiles against the types it was written against and nothing else.
func TestDiscoversARunningContainer(t *testing.T) {
	ctx := t.Context()
	testutil.SkipWithoutDocker(ctx, t)

	connURL := testutil.Postgres(ctx, t, "")
	u, err := testutil.URL(connURL)
	if err != nil {
		t.Fatalf("%v", err)
	}
	port := u.Port()

	// The developer's own environment must not decide the answer, but the
	// Docker endpoint has to stay the real one, so DOCKER_HOST is left alone
	// here and only the rung 1 and rung 2 variables are cleared.
	quietRungs(t)

	var events []event.Event
	sink := event.SinkFunc(func(e event.Event) { events = append(events, e) })

	cands, err := New().Discover(ctx, t.TempDir(), sink)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	got, ok := candidateOnPort(cands, port)
	if !ok {
		t.Fatalf("the container published on port %s is not on the ladder: %+v", port, cands)
	}
	if got.Provenance != pipeline.FromContainer {
		t.Errorf("provenance = %v, want rung 3", got.Provenance)
	}
	if !got.Local {
		t.Error("a container on this machine's daemon must be Local (ARCHITECTURE.md section 2)")
	}
	if !got.Reachable {
		t.Fatalf("the container did not answer inside the dial: %s", got.ConnectErr)
	}
	if got.Version < 14 {
		t.Errorf("server major = %d, want the container's own (ADR-003 supports 14 to 18)", got.Version)
	}
	if got.Marked {
		t.Error("a fresh container carries no lazyslice_meta")
	}
	if !got.EmptyHint {
		t.Error("a fresh container has no user table with relpages > 0, so the hint must read probably empty")
	}
	if got.Empty != nil {
		t.Error("discovery filled Empty; only Target.Gate may, because emptiness is a per-table probe")
	}
	if got.Label == "" {
		t.Error("the candidate has no display name, so the candidate list cannot say which container it is")
	}
}

// The same container reached twice — once at rung 1 through $DATABASE_URL and
// once at rung 3 through its published binding — is one candidate, and the
// lower-numbered rung wins the printed provenance (ADR-008 §4).
func TestContainerAndEnvVarCollapseToOneCandidate(t *testing.T) {
	ctx := t.Context()
	testutil.SkipWithoutDocker(ctx, t)

	connURL := testutil.Postgres(ctx, t, "")
	u, err := testutil.URL(connURL)
	if err != nil {
		t.Fatalf("%v", err)
	}
	quietRungs(t)
	t.Setenv("DATABASE_URL", connURL)

	cands, err := New().Discover(ctx, t.TempDir(), event.Discard)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	var matches int
	for _, c := range cands {
		if c.Ref.Port == portOf(u.Port()) {
			matches++
			if c.Provenance != pipeline.FromEnvVar {
				t.Errorf("provenance = %v, want the lower-numbered rung to win", c.Provenance)
			}
			if !c.Local {
				t.Error("the collapse dropped the locality the container rung established")
			}
			if !c.Reachable {
				t.Errorf("the collapsed candidate did not answer: %s", c.ConnectErr)
			}
		}
	}
	if matches != 1 {
		t.Errorf("the container appears %d times on the ladder, want once", matches)
	}
}

// TestDiscoveredCandidateAuthenticatesWithPasswordCommand is R2-16's other
// half (the T-0213 fix round's high finding): rung0's own doc comment ("the
// candidate carries no password and the ordinary password sources ...
// --password-command ... supply one") and rung0Target's promise a candidate
// the *ladder* found, not only one named with --source, and until this round
// nothing in this package ever ran the command for one — Options carried no
// PasswordCommand field at all, probe dialled a password-less candidate with
// none, pg.Connect failed authentication, the candidate came back
// Reachable=false, and chooseSource skipped it. A committed lazyslice.yml plus
// --password-command — the reason the flag exists — still failed
// authentication before this test could pass.
//
// It drives a candidate through $DATABASE_URL (rung 1) rather than rung 0's
// committed yml, because rung 1 needs no on-disk fixture to prove the same
// claim: probe resolves the password before dialling, for any rung.
func TestDiscoveredCandidateAuthenticatesWithPasswordCommand(t *testing.T) {
	ctx := t.Context()
	testutil.SkipWithoutDocker(ctx, t)
	quietRungs(t)

	connURL := testutil.Postgres(ctx, t, "")
	u, err := testutil.URL(connURL)
	if err != nil {
		t.Fatalf("%v", err)
	}
	realPassword, ok := u.User.Password()
	if !ok || realPassword == "" {
		t.Fatalf("testutil.Postgres returned a connection string with no password: %s", connURL)
	}
	noPassword := *u
	noPassword.User = url.User(u.User.Username())
	t.Setenv("DATABASE_URL", noPassword.String())

	script := writePasswordScript(t, realPassword)

	res, err := Resolve(ctx, Options{Workdir: t.TempDir(), PasswordCommand: script}, event.Discard)
	if err != nil {
		t.Fatalf("Resolve: %v (a discovered candidate should have authenticated with --password-command's own output)", err)
	}
	if !passwordAvailable(dsn.DSN(res.Source)) {
		t.Errorf("Resolve's chosen source carries no password: --password-command's output never reached the candidate before it was dialled")
	}
	if res.Source == noPassword.String() {
		t.Errorf("Resolve returned the connection string unchanged; the password never reached it")
	}
}

// Rung 4 and Q1', end to end against a real daemon: a container this tool
// provisioned is stopped, the ladder shows it as a stopped candidate rather
// than counting it, Q1' takes its default headlessly and starts it, and the
// endpoint that comes back answers.
//
// It removes what it made, which lazyslice itself never does.
func TestAStoppedContainerIsOfferedAndStarted(t *testing.T) {
	ctx := t.Context()
	testutil.SkipWithoutDocker(ctx, t)
	quietRungs(t)
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	endpoint, err := dockerctx.Resolve(ctx, "")
	if err != nil || !endpoint.Local() {
		t.Skipf("no local docker endpoint: %v", endpoint)
	}
	api, err := dockerctx.Dial(endpoint)
	if err != nil {
		t.Skipf("docker endpoint %s did not open: %v", endpoint, err)
	}

	// The workdir is what the working_dir label carries, and it is what keeps
	// every other Postgres container on this machine off this run's ladder.
	dir := t.TempDir()
	project := "lazyslicerung4" + strconv.FormatInt(time.Now().UnixNano(), 36)
	t.Cleanup(func() {
		clean, cancel := context.WithTimeout(context.WithoutCancel(ctx), 60*time.Second)
		defer cancel()
		if _, rmErr := api.ContainerRemove(clean, provision.Name(project), client.ContainerRemoveOptions{
			Force: true, RemoveVolumes: true,
		}); rmErr != nil {
			t.Logf("removing %s: %v", provision.Name(project), rmErr)
		}
		if _, rmErr := api.VolumeRemove(clean, provision.Volume(project), client.VolumeRemoveOptions{Force: true}); rmErr != nil {
			t.Logf("removing %s: %v", provision.Volume(project), rmErr)
		}
	})

	made, err := provision.New(api).Provision(ctx, provision.Request{
		Project: project, Workdir: dir, Major: imageMajor(t),
	})
	if err != nil {
		t.Fatalf("provisioning the container this test then stops: %v", err)
	}
	if _, stopErr := api.ContainerStop(ctx, provision.Name(project), client.ContainerStopOptions{}); stopErr != nil {
		t.Fatalf("stopping %s: %v", provision.Name(project), stopErr)
	}

	// A source is named so that the ladder has one; what is under test is the
	// target side. The source is a container of its own and is not in this
	// project, so it never reaches the candidate list.
	source := testutil.Postgres(ctx, t, "")

	var events []event.Event
	sink := event.SinkFunc(func(e event.Event) { events = append(events, e) })

	res, err := Resolve(ctx, Options{
		Workdir: dir, NeedTarget: true, Yes: true,
		Source: source,
	}, sink)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.Target != made.DSN {
		t.Errorf("target = %q, want the started container %q", res.Target, made.DSN)
	}

	insp, err := api.ContainerInspect(ctx, provision.Name(project), client.ContainerInspectOptions{})
	if err != nil {
		t.Fatalf("inspecting the started container: %v", err)
	}
	if insp.Container.State == nil || !insp.Container.State.Running {
		t.Error("Q1' answered yes and the container is not running")
	}
	if !hasCode(events, CodeCandidateUnreachable) {
		t.Error("the stopped container never printed on the ladder")
	}
}

// The dial's three catalog reads run inside one REPEATABLE READ READ ONLY
// transaction, and the tracer that would refuse them otherwise is satisfied.
//
// Both halves matter. The transaction is the read-only rail: pg.Connect set
// default_transaction_read_only on every source connection until T-0076 removed
// it as a pooler-poisoning session GUC, and for as long as that took to notice,
// this dial sent three statements to production candidates with no rail under
// them at all (T-0081). internal/pg now refuses a statement on an idle source
// connection outright (T-0082), so a dial that lost its BEGIN would fail here
// as an unreachable candidate rather than as a paragraph nobody re-read.
//
// It is an integration test because pgx reports a transaction status only from
// a real server's ReadyForQuery: a unit test can assert what the tracer does
// with a status and not what the server actually said.
func TestTheDialRunsInsideAReadOnlyTransaction(t *testing.T) {
	ctx := t.Context()
	testutil.SkipWithoutDocker(ctx, t)

	f := &found{dsn: dsn.DSN(testutil.Postgres(ctx, t, ""))}
	tracer := probe(ctx, f, "", nil)
	if tracer == nil {
		t.Fatal("the dial did not get as far as an allowlist")
	}
	if !f.cand.Reachable {
		t.Fatalf("the container did not answer inside the dial: %s", f.cand.ConnectErr)
	}
	if err := tracer.Violation(); err != nil {
		t.Fatalf("the dial broke a source rail: %v", err)
	}

	var got []string
	for _, s := range tracer.Trace() {
		if s.Shape == "source.connect" {
			continue
		}
		got = append(got, s.Shape)
	}
	want := []string{
		"source.begin",
		"discover.version",
		"discover.tables",
		"discover.marker",
		"source.rollback",
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("the dial sent %v, want %v", got, want)
	}
}

// The second run of a provisioned target, end to end against a real daemon.
//
// ARCHITECTURE.md section 9 says the container survives the run and is "the
// developer's local database from then on", and ADR-004 says the second run
// asks nothing. lazyslice.yml records a reference and never a password, and the
// password --create-target minted is random, so rung 0 alone handed the run a
// passwordless DSN and the run stopped at exit 4 with "password authentication
// failed for user postgres". What is asserted here is the thing that failed:
// the endpoint Resolve returns actually opens.
func TestASecondRunOpensTheTargetItProvisioned(t *testing.T) {
	ctx := t.Context()
	testutil.SkipWithoutDocker(ctx, t)
	quietRungs(t)
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	endpoint, err := dockerctx.Resolve(ctx, "")
	if err != nil || !endpoint.Local() {
		t.Skipf("no local docker endpoint: %v", endpoint)
	}
	api, err := dockerctx.Dial(endpoint)
	if err != nil {
		t.Skipf("docker endpoint %s did not open: %v", endpoint, err)
	}

	// The project name is the one a real run would use, because it is what
	// names the container and therefore keys the remembered credential.
	dir := t.TempDir()
	project := projectName(dir)
	t.Cleanup(func() {
		clean, cancel := context.WithTimeout(context.WithoutCancel(ctx), 60*time.Second)
		defer cancel()
		if _, rmErr := api.ContainerRemove(clean, provision.Name(project), client.ContainerRemoveOptions{
			Force: true, RemoveVolumes: true,
		}); rmErr != nil {
			t.Logf("removing %s: %v", provision.Name(project), rmErr)
		}
		if _, rmErr := api.VolumeRemove(clean, provision.Volume(project), client.VolumeRemoveOptions{Force: true}); rmErr != nil {
			t.Logf("removing %s: %v", provision.Volume(project), rmErr)
		}
	})

	made, err := provision.New(api).Provision(ctx, provision.Request{
		Project: project, Workdir: dir, Major: imageMajor(t),
	})
	if err != nil {
		t.Fatalf("provisioning the target the second run then reopens: %v", err)
	}

	// What internal/emit committed on the first run: the provenance, the label
	// and the redacted reference, and no credential of any kind.
	committed := &pipeline.Config{
		Target:      pipeline.FromContainer,
		TargetLabel: made.Candidate.Label,
		TargetRef:   made.Candidate.Ref,
	}

	source := testutil.Postgres(ctx, t, "")
	// The same directory, and a different one holding a copy of the same
	// file: dogfood session 2's shape (T-0327, ADR-016), where the state dir's
	// password is keyed by a project name the new directory does not have and
	// only the container's own environment can supply it.
	for _, workdir := range []string{dir, t.TempDir()} {
		res, err := Resolve(ctx, Options{
			Workdir: workdir, NeedTarget: true, Yes: true,
			Source: source,
			Config: committed,
		}, event.Discard)
		if err != nil {
			t.Fatalf("Resolve on the second run from %s: %v", workdir, err)
		}
		// What has to match is the endpoint, and what has to be added back is
		// the credential.
		if _, ref, parseErr := dsn.Parse(res.Target); parseErr != nil || !reflect.DeepEqual(ref, made.Candidate.Ref) {
			t.Errorf("target = %q (%v), want the endpoint the first run provisioned, %s",
				redactDSN(res.Target), parseErr, made.Candidate.Ref)
		}

		cfg, err := pgconn.ParseConfig(res.Target)
		if err != nil {
			t.Fatalf("parsing the target the second run resolved: %v", err)
		}
		conn, err := pgconn.ConnectConfig(ctx, cfg)
		if err != nil {
			t.Fatalf("the second run from %s could not open the target it provisioned: %v", workdir, err)
		}
		if _, execErr := conn.Exec(ctx, "SELECT 1").ReadAll(); execErr != nil {
			t.Errorf("the target did not answer: %v", execErr)
		}
		_ = conn.Close(context.WithoutCancel(ctx))
	}
}

// redactDSN keeps a failing assertion from printing a password.
func redactDSN(connURL string) string {
	at := strings.LastIndex(connURL, "@")
	if at < 0 {
		return connURL
	}
	return "postgres://…" + connURL[at:]
}

// imageMajor is the major of the image the rest of the suite already pulls, so
// this test needs no download on a warm cache.
func imageMajor(t *testing.T) int {
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

// quietRungs clears the rung 1 and rung 2 variables. DOCKER_HOST and the Docker
// context are deliberately left alone: this file's subject is the real daemon.
func quietRungs(t *testing.T) {
	t.Helper()
	for _, n := range []string{
		"DATABASE_URL", "POSTGRES_URL", "PG_URL", "DB_URL",
		"PGSERVICE", "PGHOST", "PGHOSTADDR", "PGDATABASE", "PGPORT", "PGUSER",
		"PGPASSWORD", "PGPASSFILE",
	} {
		t.Setenv(n, "")
	}
}

func candidateOnPort(cands []pipeline.Candidate, port string) (pipeline.Candidate, bool) {
	for _, c := range cands {
		if c.Ref.Port == portOf(port) {
			return c, true
		}
	}
	return pipeline.Candidate{}, false
}

func portOf(s string) int {
	n := 0
	for _, r := range strings.TrimSpace(s) {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}
