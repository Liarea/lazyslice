// SPDX-License-Identifier: Apache-2.0

package discover

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"net/netip"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

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
// a source and no target. Every case sets Yes, which is one half of ADR-008 §7's
// single headless path — the other half is a process with no controlling
// terminal — so no case here can reach a prompt and block a test run started
// from a terminal.
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
			opts:     Options{Workdir: dir, NeedTarget: true, Yes: true},
			wantExit: 3,
			wantCode: CodeSourceNone,
		},
		{
			name: "a source and no target, with provisioning not in this build, is exit 4 naming --target",
			opts: Options{
				Workdir: dir, NeedTarget: true, Yes: true,
				Source: "postgres://app@127.0.0.1:1/shop",
			},
			wantExit: 4,
			wantCode: CodeTargetNone,
			wantFlag: "--target",
		},
		{
			// Q1's headless failure (ADR-008 §6): the endpoint is local and
			// answering, so a container could be created, and the flag named
			// is the one that would create it.
			name: "a source and no target on a usable endpoint is exit 4 naming --create-target",
			opts: Options{
				Workdir: dir, NeedTarget: true, Yes: true,
				DockerHost: localDockerHost,
				dial:       fakeDial(&fakeDocker{}),
				Source:     "postgres://app@127.0.0.1:1/shop",
			},
			wantExit: 4,
			wantCode: CodeTargetNone,
			wantFlag: "--create-target",
		},
		{
			// --create-target with a source that never answered cannot choose
			// postgres:<major>, so the run stops rather than guessing a version
			// to run.
			name: "--create-target with an unreachable source is exit 4 naming --target",
			opts: Options{
				Workdir: dir, NeedTarget: true, CreateTarget: true, Yes: true,
				DockerHost:  localDockerHost,
				dial:        fakeDial(&fakeDocker{}),
				provisioner: refuseToProvision(t),
				Source:      "postgres://app@127.0.0.1:1/shop",
			},
			wantExit: 4,
			wantCode: CodeTargetNone,
			wantFlag: "--target",
		},
		{
			// ADR-008 §3: a container is never created on someone else's
			// daemon, and the refusal names the endpoint and --docker-host
			// before any container work (THREAT_MODEL.md T2). quietEnvironment
			// has pointed DOCKER_HOST at a non-local daemon.
			name: "--create-target against a non-local docker endpoint is refused, naming --docker-host",
			opts: Options{
				Workdir: dir, NeedTarget: true, CreateTarget: true, Yes: true,
				provisioner: refuseToProvision(t),
				Source:      "postgres://app@127.0.0.1:1/shop",
			},
			wantExit: 4,
			wantCode: CodeTargetDockerNotLocal,
			wantFlag: "--docker-host",
		},
		{
			name: "a read-only mode needs no target and stops for nothing",
			opts: Options{
				Workdir: dir, Yes: true,
				Source: "postgres://app@127.0.0.1:1/shop",
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

// T-0184, ADR-013 review finding 1: the headless same-cluster refusal must
// fire on this package's own definition of headless — --yes OR no controlling
// terminal (Options.Yes doc comment; prompterFor) — not on --yes alone. A CI
// job or cron entry that simply omits --yes is still nobody at a terminal, and
// before this test the refusal was gated on o.Yes and so missed exactly that
// case: reachable, but only on the source's own cluster.
//
// No Yes and no o.Prompter here: this pins the equivalence against a process
// with no controlling terminal, which is what go test itself is
// (TestHeadlessQuestionLadderAsksNothingAndExits's own comment notes the same
// reliance, and unlike that test's Q1 prompt, isHeadless opens and closes the
// controlling terminal without ever calling Confirm, so this cannot block a
// run started from one).
func TestHeadlessSameClusterRefusalDoesNotNeedYes(t *testing.T) {
	quietEnvironment(t)
	dir := t.TempDir()
	write(t, dir, ".env", "DATABASE_URL=postgres://app@10.0.0.5:5432/shop_test\n")

	opts := Options{
		Workdir:       dir,
		NeedTarget:    true,
		Source:        "postgres://app@10.0.0.5:5432/shop",
		dialCandidate: sayVersion(16),
	}

	_, err := Resolve(t.Context(), opts, event.Discard)
	r, ok := AsRefusal(err)
	if !ok {
		t.Fatalf("Resolve = %v, want the headless same-cluster refusal with no --yes at all", err)
	}
	if r.Code != CodeTargetHeadlessSameCluster || r.Exit != exitTarget {
		t.Errorf("refusal = %s/exit %d, want %s/exit %d", r.Code, r.Exit, CodeTargetHeadlessSameCluster, exitTarget)
	}
}

// The sibling of the test above (T-0184, ADR-013 review finding 4): a headless
// run with a reachable target-shaped candidate on a genuinely different
// cluster must still be chosen and loaded, not swept into the same-cluster
// refusal by an over-broad check. Two reachable candidates on different
// host:port pairs, one sharing the source's cluster and one not, and --yes:
// Resolve returns the off-cluster candidate and never emits
// target.refused.headless_same_cluster.
func TestHeadlessWithAnOffClusterCandidateChoosesItInstead(t *testing.T) {
	quietEnvironment(t)
	dir := t.TempDir()
	write(t, dir, ".env",
		"DATABASE_URL=postgres://app@10.0.0.5:5432/shop_test\n"+
			"POSTGRES_URL=postgres://app@10.0.0.9:5432/shop_scratch\n")

	opts := Options{
		Workdir:       dir,
		NeedTarget:    true,
		Yes:           true,
		Source:        "postgres://app@10.0.0.5:5432/shop",
		dialCandidate: sayVersion(16),
	}

	var codes []event.Code
	sink := event.SinkFunc(func(e event.Event) { codes = append(codes, e.Code) })

	res, err := Resolve(t.Context(), opts, sink)
	if err != nil {
		t.Fatalf("Resolve = %v, want the off-cluster candidate chosen, not a refusal", err)
	}
	if !strings.Contains(res.Target, "10.0.0.9:5432/shop_scratch") {
		t.Errorf("target = %q, want the candidate on the source's cluster passed over "+
			"for the one that is not", res.Target)
	}
	for _, c := range codes {
		if c == CodeTargetHeadlessSameCluster {
			t.Error("emitted the headless same-cluster refusal even though a genuinely " +
				"off-cluster candidate was reachable")
		}
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
	// Both sides are stamped FromFlag rather than left at Provenance's zero
	// value, which is pipeline.FromYml: an unset field would tell internal/emit
	// that the committed file named a database the operator named on the command
	// line, and the next run would read it back as rung 0's (T-0060).
	if res.SourceProvenance != pipeline.FromFlag || res.SourceLabel != "" {
		t.Errorf("source provenance/label = %v/%q, want flag with no label",
			res.SourceProvenance, res.SourceLabel)
	}
	if res.TargetProvenance != pipeline.FromFlag || res.TargetLabel != "" {
		t.Errorf("target provenance/label = %v/%q, want flag with no label",
			res.TargetProvenance, res.TargetLabel)
	}
	if len(events) != 0 {
		t.Errorf("the ladder ran anyway and emitted %d event(s)", len(events))
	}
}

// docs/reviews, 2026-09-14 re-review: the sslrootcert retry's first landing
// lived only in rung0's own loop, which Resolve never reaches when the
// committed yml already supplies both endpoints — Resolve's own rung-0
// short-circuit (o.Source == "" case) called refDSN directly, with no
// retry, so the ordinary committed lazyslice.yml (this exact scenario) had
// the unreadable cert string handed straight through and failed far from
// here, in internal/core's own dsn.Parse. This drives Resolve itself, not
// rung0, so a regression to the short-circuit is caught here again.
func TestResolveRetriesWithoutAnUnreadableCertPath(t *testing.T) {
	source := mustRef(t, "postgres://app@db.example.com:6432/shop")
	source.Params = map[string]string{
		"sslmode":     "verify-full",
		"sslrootcert": filepath.Join(t.TempDir(), "does-not-exist.pem"),
	}
	target := mustRef(t, "postgres://app@127.0.0.1:6432/shop_test")
	cfg := &pipeline.Config{
		SourceRef:   source,
		SourceLabel: "prod",
		TargetRef:   target,
		TargetLabel: "local",
	}
	var buf bytes.Buffer
	opts := Options{Config: cfg, NeedTarget: true, progress: &buf}

	res, err := Resolve(t.Context(), opts, event.SinkFunc(func(event.Event) {}))
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	_, ref, err := dsn.Parse(res.Source)
	if err != nil {
		t.Fatalf("res.Source = %q did not parse: %v", res.Source, err)
	}
	if ref.Host != "db.example.com" || ref.Database != "shop" {
		t.Errorf("source = %s, want the endpoint the file recorded", ref)
	}
	if _, ok := ref.Params["sslrootcert"]; ok {
		t.Errorf("source params = %#v, want sslrootcert dropped since it could not be read", ref.Params)
	}
	if ref.Params["sslmode"] != "verify-full" {
		t.Errorf("source params = %#v, want sslmode to survive the retry", ref.Params)
	}
	if res.SourceProvenance != cfg.Source || res.SourceLabel != "prod" {
		t.Errorf("source provenance/label = %v/%q, want the yml's own", res.SourceProvenance, res.SourceLabel)
	}
	msg := buf.String()
	if !strings.Contains(msg, "sslrootcert") {
		t.Errorf("progress = %q, want it to name sslrootcert", msg)
	}
	if !strings.Contains(msg, "cannot read") {
		t.Errorf("progress = %q, want it to say the path could not be read", msg)
	}
}

// docs/reviews, 2026-09-14, finding 2: a committed lazyslice.yml's source_ref
// that fails dsn.Parse for a reason refDSNValidated's sslrootcert retry
// cannot fix — a typoed sslmode is the finding's own example, and there is no
// file-valued param here for withoutFileParams to strip — used to come back
// as ("", false), which Resolve's rung-0 short-circuit read as "nothing at
// rung 0" and would have let the run fall through to discovery instead of
// refusing over a file it could not honour. Resolve must refuse loudly
// instead: exit 2, naming the field and the parameter, never falling through.
func TestResolveRefusesATypoedSslmode(t *testing.T) {
	source := mustRef(t, "postgres://app@db.example.com:6432/shop")
	source.Params = map[string]string{"sslmode": "verify-ful"}
	cfg := &pipeline.Config{SourceRef: source, SourceLabel: "prod"}
	opts := Options{Config: cfg}

	res, err := Resolve(t.Context(), opts, event.SinkFunc(func(event.Event) {}))
	if err == nil {
		t.Fatalf("Resolve: want a refusal for the typoed sslmode, got res = %+v, nil", res)
	}
	r, ok := AsRefusal(err)
	if !ok {
		t.Fatalf("Resolve error = %T, want *Refusal", err)
	}
	if r.Exit != 2 {
		t.Errorf("exit = %d, want 2 (ADR-005 usage)", r.Exit)
	}
	if r.Code != CodeSourceRefInvalid {
		t.Errorf("code = %s, want %s", r.Code, CodeSourceRefInvalid)
	}
	if !strings.Contains(r.Args[event.ArgReason], "sslmode") {
		t.Errorf("reason = %q, want it to name sslmode", r.Args[event.ArgReason])
	}
	// dsn.ParseError's own test (internal/dsn) pins that a password never
	// reaches this text; pgconn's error otherwise legitimately quotes the
	// (password-free) connection string it failed on alongside the parameter
	// name, which is not itself sensitive — Ref's own fields are printable.
	if res.Source != "" {
		t.Errorf("Resolve returned Source = %q on a refusal, want it left unset", res.Source)
	}
}

// The target side goes through rung0Target rather than refDSNValidated
// directly, because rung0Target also recovers a provisioned target's
// password — this pins that the error still surfaces through it rather than
// being swallowed by the password lookup. connect_timeout is the finding's
// other example of a parameter withoutFileParams cannot strip.
func TestResolveRefusesANonNumericConnectTimeoutOnTarget(t *testing.T) {
	source := mustRef(t, "postgres://app@db.example.com:6432/shop")
	target := mustRef(t, "postgres://app@127.0.0.1:6432/shop_test")
	target.Params = map[string]string{"connect_timeout": "soon"}
	cfg := &pipeline.Config{
		SourceRef:   source,
		SourceLabel: "prod",
		TargetRef:   target,
		TargetLabel: "local",
	}
	opts := Options{Config: cfg, NeedTarget: true}

	_, err := Resolve(t.Context(), opts, event.SinkFunc(func(event.Event) {}))
	if err == nil {
		t.Fatal("Resolve: want a refusal for the non-numeric connect_timeout, got nil")
	}
	r, ok := AsRefusal(err)
	if !ok {
		t.Fatalf("Resolve error = %T, want *Refusal", err)
	}
	if r.Exit != 2 {
		t.Errorf("exit = %d, want 2 (ADR-005 usage)", r.Exit)
	}
	if r.Code != CodeTargetRefInvalid {
		t.Errorf("code = %s, want %s", r.Code, CodeTargetRefInvalid)
	}
	if !strings.Contains(r.Args[event.ArgReason], "connect_timeout") {
		t.Errorf("reason = %q, want it to name connect_timeout", r.Args[event.ArgReason])
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

// refDSN is rung 0's half of docs/reviews/2026-09-09/REVIEW.md finding 6: a
// dsn.Ref carrying sslmode=verify-full and sslrootcert has to come back out as
// query parameters a connection string carries, or the fix that keeps Params
// on Ref does nothing for the one rung Params exists to fix (finding 3). This
// round-trips it end to end: build the Ref by hand, rebuild a connection
// string with refDSN, and Parse that string back — the same two steps rung 0
// itself performs.
func TestRefDSNRoundTripsSslrootcert(t *testing.T) {
	caPath := writeTestCA(t)
	r := dsn.Ref{
		Host: "db.example.com", Port: 6432, Database: "shop", User: "app",
		Params: map[string]string{"sslmode": "verify-full", "sslrootcert": caPath},
	}

	s, ok := refDSN(r)
	if !ok {
		t.Fatalf("refDSN(%+v) = false, want a connection string", r)
	}
	_, got, err := dsn.Parse(s)
	if err != nil {
		t.Fatalf("Parse(refDSN(r)) = %v", err)
	}
	want := map[string]string{"sslmode": "verify-full", "sslrootcert": caPath}
	if !reflect.DeepEqual(got.Params, want) {
		t.Errorf("round-tripped Params = %#v, want %#v", got.Params, want)
	}
	if got.Host != r.Host || got.Port != r.Port || got.Database != r.Database || got.User != r.User {
		t.Errorf("round-tripped identity = %+v, want it unchanged from %+v", got, r)
	}
}

// warnDroppedParams names a dropped key on progress and never prints its
// value: dsn.ExtractParams's dropped list already withholds the value, but
// this is the boundary that would leak one right back in if a caller ever
// passed the raw string through instead.
func TestWarnDroppedParamsNamesTheKeyNeverTheValue(t *testing.T) {
	var buf bytes.Buffer
	s := "postgres://app:hunter2@db.example.com:6432/shop" +
		"?sslmode=verify-full&target_session_attrs=read-write"

	warnDroppedParams(&buf, s)

	out := buf.String()
	if !strings.Contains(out, "target_session_attrs") {
		t.Errorf("progress = %q, want it to name the dropped key target_session_attrs", out)
	}
	if strings.Contains(out, "read-write") || strings.Contains(out, "hunter2") {
		t.Errorf("progress = %q, want no dropped or credential value in it", out)
	}
}

// docs/reviews/2026-09-14, finding 2: a committed lazyslice.yml's sslrootcert
// naming a path this machine cannot read used to make dsn.Parse fail at rung
// 0 and silently drop that whole endpoint from the candidate list, letting
// the walk fall through to a different database. rung0 must instead retry
// without the unreadable file-valued params and warn by name, keeping the
// endpoint the file recorded.
func TestRung0RetriesWithoutAnUnreadableCertPath(t *testing.T) {
	source := mustRef(t, "postgres://app@db.example.com:6432/shop")
	source.Params = map[string]string{
		"sslmode":     "verify-full",
		"sslrootcert": filepath.Join(t.TempDir(), "does-not-exist.pem"),
	}
	cfg := &pipeline.Config{
		SourceRef:   source,
		SourceLabel: "prod",
	}
	var buf bytes.Buffer
	o := Options{Config: cfg, progress: &buf}

	out := rung0(o)

	if len(out) != 1 {
		t.Fatalf("got %d candidates, want the source recovered despite the unreadable cert path: %+v", len(out), out)
	}
	got := out[0].cand.Ref
	if got.Host != "db.example.com" || got.Database != "shop" {
		t.Errorf("candidate = %s, want the endpoint the file recorded", got)
	}
	if _, ok := got.Params["sslrootcert"]; ok {
		t.Errorf("candidate Params = %#v, want sslrootcert dropped since it could not be read", got.Params)
	}
	if got.Params["sslmode"] != "verify-full" {
		t.Errorf("candidate Params = %#v, want sslmode to survive the retry", got.Params)
	}
	msg := buf.String()
	if !strings.Contains(msg, "sslrootcert") {
		t.Errorf("progress = %q, want it to name sslrootcert", msg)
	}
	if !strings.Contains(msg, "cannot read") {
		t.Errorf("progress = %q, want it to say the path could not be read", msg)
	}
}

// A lazyslice.yml whose params all Parse cleanly is unaffected: no retry, no
// warning, and the candidate carries every one of them.
func TestRung0KeepsParamsWhenTheyAllParse(t *testing.T) {
	caPath := writeTestCA(t)
	source := mustRef(t, "postgres://app@db.example.com:6432/shop")
	source.Params = map[string]string{"sslmode": "verify-full", "sslrootcert": caPath}
	cfg := &pipeline.Config{SourceRef: source, SourceLabel: "prod"}
	var buf bytes.Buffer
	o := Options{Config: cfg, progress: &buf}

	out := rung0(o)

	if len(out) != 1 {
		t.Fatalf("got %d candidates, want 1: %+v", len(out), out)
	}
	want := map[string]string{"sslmode": "verify-full", "sslrootcert": caPath}
	if !reflect.DeepEqual(out[0].cand.Ref.Params, want) {
		t.Errorf("candidate Params = %#v, want %#v", out[0].cand.Ref.Params, want)
	}
	if buf.Len() != 0 {
		t.Errorf("progress = %q, want no warning when nothing was dropped", buf.String())
	}
}

// writeTestCA writes a self-signed certificate to a file in t.TempDir and
// returns its path, for a sslrootcert value pgconn.ParseConfig will accept.
// Mirrors internal/dsn/dsn_test.go's helper of the same name, which is
// unexported there and so not reusable here.
func writeTestCA(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating test CA key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		IsCA:         true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("creating test CA certificate: %v", err)
	}
	path := filepath.Join(t.TempDir(), "ca.pem")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("creating %s: %v", path, err)
	}
	defer f.Close()
	if err := pem.Encode(f, &pem.Block{Type: "CERTIFICATE", Bytes: der}); err != nil {
		t.Fatalf("encoding test CA certificate: %v", err)
	}
	return path
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
	// host is a container ID's HostConfig, which is where a stopped
	// container's port binding comes from: a container that is not running
	// publishes nothing, so rung 4 reads what it is configured to publish.
	host map[string]*container.HostConfig
}

func (f *fakeDocker) Ping(context.Context, client.PingOptions) (client.PingResult, error) {
	return client.PingResult{}, nil
}

func (f *fakeDocker) ContainerList(context.Context, client.ContainerListOptions) (client.ContainerListResult, error) {
	return client.ContainerListResult{Items: f.list}, nil
}

func (f *fakeDocker) ContainerInspect(_ context.Context, id string, _ client.ContainerInspectOptions) (client.ContainerInspectResult, error) {
	return client.ContainerInspectResult{
		Container: container.InspectResponse{
			Config:     &container.Config{Env: f.env[id]},
			HostConfig: f.host[id],
		},
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
