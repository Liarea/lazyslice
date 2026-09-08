// SPDX-License-Identifier: Apache-2.0

package discover

import (
	"context"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"

	"github.com/Liarea/lazyslice/internal/discover/dockerctx"
	"github.com/Liarea/lazyslice/internal/dsn"
	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pipeline"
)

// A .env file contributes a whole postgres:// URI or nothing, and a compose
// file contributes names and nothing else (ADR-008 §2).
//
// The two failures this guards against are the same failure one layer apart:
// assembling DB_HOST and DB_PORT into a DSN, and interpolating a compose file
// from .env, both produce a connection string that does not connect.
func TestEnvAndComposeAreNamingSourcesOnly(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, ".env", strings.Join([]string{
		"# the project's database",
		"DATABASE_URL=postgres://app@127.0.0.1:1/shop",
		"DB_HOST=db",
		"DB_PORT=5432",
		"POSTGRES_PASSWORD=hunter2",
	}, "\n"))
	write(t, dir, ".env.local", "POSTGRES_URL=${PROD_URL}\n")
	write(t, dir, "compose.yaml", strings.Join([]string{
		"name: shopproject",
		"services:",
		"  db:",
		"    image: postgres:16",
		"    ports: ['5432:5432']",
		"  db-test:",
		"    image: postgres:16",
		"  web:",
		"    image: nginx",
	}, "\n"))
	quietEnvironment(t)

	var events []event.Event
	sink := event.SinkFunc(func(e event.Event) { events = append(events, e) })

	cands, err := New().Discover(t.Context(), dir, sink)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	if len(cands) != 1 {
		t.Fatalf("got %d candidates, want the one whole URI in .env: %+v", len(cands), cands)
	}
	got := cands[0]
	if got.Provenance != pipeline.FromEnvVar || got.Label != "DATABASE_URL" {
		t.Errorf("provenance = %v/%q, want rung 1 and DATABASE_URL", got.Provenance, got.Label)
	}
	if got.Ref.Host != "127.0.0.1" || got.Ref.Port != 1 || got.Ref.Database != "shop" {
		t.Errorf("ref = %s, want the URI's own endpoint", got.Ref)
	}
	if got.Reachable {
		t.Error("nothing is listening on port 1; the candidate must resolve unreachable rather than be taken on trust")
	}
	for _, c := range cands {
		if c.Ref.Host == "db" {
			t.Fatalf("a DSN was assembled from .env fragments: %s", c.Ref)
		}
	}
	if !hasCode(events, CodeEnvUnusable) {
		t.Error("POSTGRES_URL=${PROD_URL} was dropped silently; ADR-008 §2 requires the reason to be printed")
	}

	// Compose is read for names, and the type it is read into has no field a
	// host, port, user, password or database name could arrive in.
	p := readCompose(dir)
	if p.Name != "shopproject" {
		t.Errorf("project name = %q, want the compose file's own name", p.Name)
	}
	if strings.Join(p.PostgresServices, ",") != "db,db-test" {
		t.Errorf("postgres services = %v, want db and db-test", p.PostgresServices)
	}
}

// The default project name is the directory, which is compose's own default and
// what Q1 names the container after.
func TestProjectNameFallsBackToTheDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "My Shop!")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("%v", err)
	}
	if got := readCompose(dir).Name; got != "myshop" {
		t.Errorf("project name = %q, want %q", got, "myshop")
	}
}

// A container publishing 0.0.0.0:5432 and a $DATABASE_URL naming localhost:5432
// are one database. The lower-numbered rung wins the printed provenance and the
// discarded one is shown in parentheses (ADR-008 §4).
func TestCandidatesCollapseAcrossRungs(t *testing.T) {
	in := []found{
		{cand: pipeline.Candidate{
			Ref:        mustRef(t, "postgres://app@127.0.0.1:5432/shop"),
			Provenance: pipeline.FromContainer, Label: "db", Local: true,
		}},
		{cand: pipeline.Candidate{
			Ref:        mustRef(t, "postgres://app@localhost:5432/shop"),
			Provenance: pipeline.FromEnvVar, Label: "DATABASE_URL",
		}},
		{cand: pipeline.Candidate{
			Ref:        mustRef(t, "postgres://app@localhost:5432/shop_test"),
			Provenance: pipeline.FromEnvVar, Label: "TEST_URL",
		}},
	}

	out := collapse(in)
	if len(out) != 2 {
		t.Fatalf("got %d candidates, want the two distinct databases: %+v", len(out), out)
	}
	if out[0].cand.Provenance != pipeline.FromEnvVar {
		t.Fatalf("the lower-numbered rung must win the printed provenance, got %v", out[0].cand.Provenance)
	}
	if prov := provenanceOf(out[0]); !strings.Contains(prov, "also") || !strings.Contains(prov, "container db") {
		t.Errorf("provenance = %q, want the discarded rung in parentheses", prov)
	}
	if !out[0].cand.Local {
		t.Error("the surviving candidate lost the locality the container rung established")
	}
}

// The ladder asks nothing at all in this build — Q1 and the controlling
// terminal are phase 5 (ARCHITECTURE.md §14) — and stops with the
// ARCHITECTURE.md §8 exit code for the state it is in: 3 with no source, 4 with
// a source and no target. There is no --yes in Options because there is no
// question for it to answer; when Q1 lands, ADR-008 §7's headless path lands
// with it.
func TestHeadlessQuestionLadderAsksNothingAndExits(t *testing.T) {
	quietEnvironment(t)
	dir := t.TempDir()

	for _, tc := range []struct {
		name     string
		opts     Options
		wantExit int
		wantCode event.Code
		wantFlag string
	}{
		{
			name:     "no source is exit 3 with the ladder printed",
			opts:     Options{Workdir: dir, NeedTarget: true},
			wantExit: 3,
			wantCode: CodeSourceNone,
		},
		{
			name: "a source and no target, with provisioning not in this build, is exit 4 naming --target",
			opts: Options{
				Workdir: dir, NeedTarget: true,
				Source: "postgres://app@127.0.0.1:1/shop",
			},
			wantExit: 4,
			wantCode: CodeTargetNone,
			wantFlag: "--target",
		},
		{
			// The flag named is --target, the flag that names a database to
			// load into. Naming --create-target here would tell the operator to
			// pass the flag they just passed.
			name: "--create-target on a local endpoint says it is not in this build, and names --target",
			opts: Options{
				Workdir: dir, NeedTarget: true, CreateTarget: true,
				DockerHost: localDockerHost,
				dial:       fakeDial(&fakeDocker{}),
				Source:     "postgres://app@127.0.0.1:1/shop",
			},
			wantExit: 4,
			wantCode: CodeTargetNotImplemented,
			wantFlag: "--target",
		},
		{
			// ADR-008 §3: a container is never created on someone else's
			// daemon, and the refusal names the endpoint and --docker-host
			// before any container work (THREAT_MODEL.md T2). quietEnvironment
			// has pointed DOCKER_HOST at a non-local daemon.
			name: "--create-target against a non-local docker endpoint is refused, naming --docker-host",
			opts: Options{
				Workdir: dir, NeedTarget: true, CreateTarget: true,
				Source: "postgres://app@127.0.0.1:1/shop",
			},
			wantExit: 4,
			wantCode: CodeTargetDockerNotLocal,
			wantFlag: "--docker-host",
		},
		{
			name: "a read-only mode needs no target and stops for nothing",
			opts: Options{
				Workdir: dir,
				Source:  "postgres://app@127.0.0.1:1/shop",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var events []event.Event
			sink := event.SinkFunc(func(e event.Event) { events = append(events, e) })

			_, err := Resolve(t.Context(), tc.opts, sink)
			if tc.wantExit == 0 {
				if err != nil {
					t.Fatalf("want no refusal, got %v", err)
				}
				return
			}

			r, ok := AsRefusal(err)
			if !ok {
				t.Fatalf("want a discovery refusal, got %v", err)
			}
			if r.Exit != tc.wantExit || r.Code != tc.wantCode {
				t.Errorf("refusal = %s exit %d, want %s exit %d", r.Code, r.Exit, tc.wantCode, tc.wantExit)
			}
			if tc.wantFlag != "" && r.Args[event.ArgFlag] != tc.wantFlag {
				t.Errorf("refusal names %q, want %q", r.Args[event.ArgFlag], tc.wantFlag)
			}
			if !hasError(events, tc.wantCode, tc.wantExit) {
				t.Error("the refusal never reached the sink, so nothing was rendered from the catalogue")
			}
		})
	}
}

// A run that named both endpoints walks no rung and makes no Docker call
// (ADR-008 §1). The stub dialler fails the test if it is reached.
func TestBothEndpointsGivenSkipsDiscovery(t *testing.T) {
	quietEnvironment(t)
	opts := Options{
		Workdir:    t.TempDir(),
		NeedTarget: true,
		Source:     "postgres://app@127.0.0.1:1/shop",
		Target:     "postgres://app@127.0.0.1:2/shop_test",
	}
	var events []event.Event
	res, err := Resolve(t.Context(), opts, event.SinkFunc(func(e event.Event) { events = append(events, e) }))
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.Source != opts.Source || res.Target != opts.Target {
		t.Errorf("the given endpoints were not carried through: %+v", res)
	}
	if len(events) != 0 {
		t.Errorf("the ladder ran anyway and emitted %d event(s)", len(events))
	}
}

// The lower-numbered rung wins the printed provenance; it does not win the
// credential. A rung-3 container carries POSTGRES_PASSWORD out of its own
// environment and a rung-1 $DATABASE_URL naming the same (host, port, database)
// may carry none, so collapsing on provenance alone drops a database rung 3
// could have used. Verification therefore runs before the collapse and the
// survivor keeps the endpoint that answered.
func TestCollapseKeepsTheCredentialThatAnswers(t *testing.T) {
	envDSN, envRef, err := dsn.Parse("postgres://app@localhost:5432/shop")
	if err != nil {
		t.Fatalf("%v", err)
	}
	containerDSN, containerRef, err := dsn.Parse("postgres://app:hunter2@127.0.0.1:5432/shop")
	if err != nil {
		t.Fatalf("%v", err)
	}

	out := collapse([]found{
		{
			dsn: envDSN,
			cand: pipeline.Candidate{
				Ref: envRef, Provenance: pipeline.FromEnvVar, Label: "DATABASE_URL",
				Reachable: false, ConnectErr: noPassword,
			},
		},
		{
			dsn: containerDSN,
			cand: pipeline.Candidate{
				Ref: containerRef, Provenance: pipeline.FromContainer, Label: "db",
				Local: true, Reachable: true, Tables: 12, EmptyHint: true,
			},
		},
	})

	if len(out) != 1 {
		t.Fatalf("got %d candidates, want the one database: %+v", len(out), out)
	}
	got := out[0]
	if got.cand.Provenance != pipeline.FromEnvVar {
		t.Errorf("provenance = %v, want the lower rung's (ADR-008 §4)", got.cand.Provenance)
	}
	if !got.cand.Reachable || got.cand.Tables != 12 {
		t.Errorf("the survivor is unreachable with %d tables; the rung that answered was discarded", got.cand.Tables)
	}
	if got.dsn != containerDSN {
		t.Error("the survivor kept a connection string that does not connect")
	}
}

// EmptyHint is a planner statistic and never a filter. relpages survives
// TRUNCATE, so a database the gate would find empty can carry EmptyHint false;
// dropping it here would be discovery deciding eligibility, which only
// Target.Gate does (ARCHITECTURE.md §9, internal/discover/CLAUDE.md).
func TestTargetShapeIsNotDecidedFromTheHints(t *testing.T) {
	truncated := found{cand: pipeline.Candidate{
		Ref:       mustRef(t, "postgres://app@127.0.0.1:5432/shop_scratch"),
		Reachable: true, Tables: 40, EmptyHint: false, Marked: false,
	}}
	source := found{cand: pipeline.Candidate{
		Ref:       mustRef(t, "postgres://app@10.0.0.9:5432/shop"),
		Reachable: true, Tables: 40,
	}}

	winner, _ := chooseTarget([]found{truncated, source}, &source)
	if winner == nil {
		t.Fatal("no target-shaped candidate: a TRUNCATEd database with stale relpages was filtered out before the gate ran")
	}
	if winner.cand.Ref.Database != "shop_scratch" {
		t.Errorf("winner = %s, want the candidate that is not the source", winner.cand.Ref)
	}
}

// ADR-008 §5's third step is "the one carrying our *bound* marker", and
// boundness is the gate's rule 4, not something a dial can see. So another
// project's marked, full target must not outrank an unmarked database that
// looks empty: they rank equally and byte order settles it.
func TestAMarkedDatabaseDoesNotOutrankAnEmptyOne(t *testing.T) {
	marked := found{cand: pipeline.Candidate{
		Ref:       mustRef(t, "postgres://app@127.0.0.1:5432/zz_other_project"),
		Reachable: true, Tables: 80, Marked: true, EmptyHint: false,
	}}
	empty := found{cand: pipeline.Candidate{
		Ref:       mustRef(t, "postgres://app@127.0.0.1:5432/aa_blank"),
		Reachable: true, Tables: 0, Marked: false, EmptyHint: true,
	}}

	winner, runnerUp := chooseTarget([]found{marked, empty}, nil)
	if winner == nil || runnerUp == nil {
		t.Fatal("both candidates are target-shaped and both must be ranked")
	}
	if winner.cand.Ref.Database != "aa_blank" {
		t.Errorf("winner = %s, want the byte-order winner: a marker discovery cannot see the binding of "+
			"must not beat an apparently empty database", winner.cand.Ref)
	}
}

// A name whose value could not be used has not answered for that name: a later
// .env file may. `.env` with DATABASE_URL=${PROD_URL} beside `.env.local` with
// the real URI under the same name is the standard dotenv override (ADR-008 §2).
func TestALaterEnvFileSuppliesAValueTheEarlierOneCouldNot(t *testing.T) {
	quietEnvironment(t)
	dir := t.TempDir()
	write(t, dir, ".env", "DATABASE_URL=${PROD_URL}\n")
	write(t, dir, ".env.local", "DATABASE_URL=postgres://app@127.0.0.1:1/shop\n")

	out, unusable := rung1(dir)
	if len(out) != 1 {
		t.Fatalf("got %d candidates, want the usable value in .env.local: %+v", len(out), out)
	}
	if out[0].cand.Ref.Database != "shop" {
		t.Errorf("candidate = %s, want .env.local's own endpoint", out[0].cand.Ref)
	}
	if len(unusable) != 1 || unusable[0].Where != ".env" {
		t.Errorf("unusable = %+v, want the .env value reported and the .env.local one used", unusable)
	}
}

// Rung 3 through Options.dial: the oneoff skip, the working_dir filter, the
// 0.0.0.0 normalisation ADR-008 §4 permits only on a local endpoint, and the
// credentials that come off the container's own environment.
func TestContainerRungBuildsCandidatesWithoutADaemon(t *testing.T) {
	quietEnvironment(t)
	dir := t.TempDir()

	api := &fakeDocker{
		list: []container.Summary{
			pgContainer("oneoff", "/shop-db-run-1", "postgres:16", 1, map[string]string{
				labelWorkingDir: dir,
				labelService:    "db",
				labelOneOff:     "True",
			}),
			pgContainer("db", "/shop-db-1", "postgres:16", 1, map[string]string{
				labelWorkingDir: filepath.Dir(dir),
				labelService:    "db",
			}),
			pgContainer("elsewhere", "/other-db-1", "postgres:16", 2, map[string]string{
				labelWorkingDir: filepath.Join(t.TempDir(), "other"),
			}),
			{ID: "web", Names: []string{"/web-1"}, Image: "nginx", State: container.StateRunning},
		},
		env: map[string][]string{
			"db": {"POSTGRES_USER=app", "POSTGRES_DB=shop", "POSTGRES_PASSWORD=hunter2"},
		},
	}

	cands, dock := walk(t.Context(), Options{
		Workdir:    dir,
		DockerHost: localDockerHost,
		dial:       fakeDial(api),
	}, event.Discard)

	if !dock.usable() {
		t.Fatalf("endpoint %+v is not usable; --create-target would be refused on a local, answering daemon", dock)
	}
	if len(cands) != 1 {
		t.Fatalf("got %d candidates, want the one container in this project: %+v", len(cands), cands)
	}
	got := cands[0].cand
	if got.Label != "db" {
		t.Errorf("label = %q, want the compose service (ADR-008 §4)", got.Label)
	}
	if got.Ref.Host != "127.0.0.1" || got.Ref.Port != 1 {
		t.Errorf("binding = %s, want 0.0.0.0 normalised to loopback on a local endpoint", got.Ref)
	}
	if got.Ref.Database != "shop" || got.Ref.User != "app" {
		t.Errorf("ref = %s, want the role and database from the container's own environment", got.Ref)
	}
	if !got.Local {
		t.Error("a container on this machine is local (ARCHITECTURE.md §2)")
	}
	if strings.Contains(string(cands[0].dsn), "hunter2") == false {
		t.Error("POSTGRES_PASSWORD did not reach the connection string, so the dial cannot authenticate")
	}
}

// The working_dir filter's stated fallback: no container matches this project,
// so every Postgres container is shown and the header says so (ADR-008 §4).
// displayName's three branches are the three names below.
func TestWorkingDirFilterFallsBackToEveryContainer(t *testing.T) {
	labelled := pgContainer("db", "/shop-db-1", "postgres:16", 1, map[string]string{
		labelWorkingDir: filepath.Join(t.TempDir(), "elsewhere"),
		labelService:    "db",
	})
	named := pgContainer("named", "/pg-solo", "postgres:16", 2, nil)
	bare := pgContainer("bareid", "", "postgres:16", 3, nil)
	bare.Names = nil

	out, matched := filterToProject([]container.Summary{labelled, named, bare}, t.TempDir())
	if matched {
		t.Error("no container carries this project's working_dir; the header must say it is showing all of them")
	}
	if len(out) != 3 {
		t.Fatalf("got %d containers, want all three: %+v", len(out), out)
	}
	for _, tc := range []struct {
		c    container.Summary
		want string
	}{
		{labelled, "db"},
		{named, "pg-solo"},
		{bare, "bareid"},
	} {
		if got := displayName(tc.c); got != tc.want {
			t.Errorf("displayName(%s) = %q, want %q", tc.c.ID, got, tc.want)
		}
	}
}

// underOrEqual is the "this container belongs to this project" relation, and it
// is a locality signal (ARCHITECTURE.md §9, THREAT_MODEL.md T2): a sibling
// directory is not this project and a parent is.
func TestUnderOrEqual(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "svc", "api")
	for _, tc := range []struct {
		dir  string
		want bool
	}{
		{root, true},
		{child, true},
		{filepath.Join(root, "svc"), true},
		{filepath.Join(root, "other"), false},
		{filepath.Join(filepath.Dir(root), "elsewhere"), false},
	} {
		if got := underOrEqual(child, tc.dir); got != tc.want {
			t.Errorf("underOrEqual(%s, %s) = %v, want %v", child, tc.dir, got, tc.want)
		}
	}
}

// ---------- helpers ----------

// quietEnvironment removes every variable a rung reads, so that the developer's
// own DATABASE_URL or Docker context cannot change the answer, and points the
// Docker endpoint at a non-local daemon so that rung 3 short-circuits without a
// socket call.
func quietEnvironment(t *testing.T) {
	t.Helper()
	for _, n := range []string{
		"DATABASE_URL", "POSTGRES_URL", "PG_URL", "DB_URL",
		"PGSERVICE", "PGHOST", "PGHOSTADDR", "PGDATABASE", "PGPORT", "PGUSER",
		"PGPASSWORD", "PGPASSFILE",
		"DOCKER_CONTEXT", "DOCKER_CONFIG",
	} {
		t.Setenv(n, "")
	}
	t.Setenv("DOCKER_HOST", "tcp://staging.example:2375")
}

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatalf("writing %s: %v", name, err)
	}
}

func mustRef(t *testing.T, s string) dsn.Ref {
	t.Helper()
	_, ref, err := dsn.Parse(s)
	if err != nil {
		t.Fatalf("parsing %s: %v", s, err)
	}
	return ref
}

func hasCode(events []event.Event, code event.Code) bool {
	for _, e := range events {
		if e.Code == code {
			return true
		}
	}
	return false
}

func hasError(events []event.Event, code event.Code, exit int) bool {
	for _, e := range events {
		if e.Code == code && e.Kind == event.Error && e.Exit == exit {
			return true
		}
	}
	return false
}

// localDockerHost is a local endpoint by ADR-008 §3's test (a unix:// socket).
// No socket is opened: every test that uses it also supplies Options.dial.
const localDockerHost = "unix:///var/run/docker.sock"

// fakeDocker is the part of the Docker client rung 3 uses, with crafted
// containers instead of a daemon.
type fakeDocker struct {
	list []container.Summary
	// env is a container ID's Config.Env, which is where POSTGRES_USER,
	// POSTGRES_DB and POSTGRES_PASSWORD come from.
	env map[string][]string
}

func (f *fakeDocker) Ping(context.Context, client.PingOptions) (client.PingResult, error) {
	return client.PingResult{}, nil
}

func (f *fakeDocker) ContainerList(context.Context, client.ContainerListOptions) (client.ContainerListResult, error) {
	return client.ContainerListResult{Items: f.list}, nil
}

func (f *fakeDocker) ContainerInspect(_ context.Context, id string, _ client.ContainerInspectOptions) (client.ContainerInspectResult, error) {
	return client.ContainerInspectResult{
		Container: container.InspectResponse{Config: &container.Config{Env: f.env[id]}},
	}, nil
}

func fakeDial(f *fakeDocker) func(dockerctx.Endpoint) (dockerAPI, error) {
	return func(dockerctx.Endpoint) (dockerAPI, error) { return f, nil }
}

// pgContainer is a running Postgres container publishing 0.0.0.0:port.
func pgContainer(id, name, image string, port uint16, labels map[string]string) container.Summary {
	return container.Summary{
		ID:     id,
		Names:  []string{name},
		Image:  image,
		Labels: labels,
		State:  container.StateRunning,
		Ports: []container.PortSummary{{
			IP:          netip.MustParseAddr("0.0.0.0"),
			PrivatePort: postgresPort,
			PublicPort:  port,
			Type:        "tcp",
		}},
	}
}
