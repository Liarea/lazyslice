// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Liarea/lazyslice/internal/dsn"
	"github.com/Liarea/lazyslice/internal/emit"
	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pg"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// closedHost, sourcePort and targetPort are two ports nothing listens on.
//
// Both tests below need the two stages that build the endpoint candidates to
// run and no database to exist: the candidate is built from the resolved
// provenance before the first statement is issued, so a refused connection
// stops each stage just after the lines under test.
const (
	closedHost = "127.0.0.1"
	sourcePort = 1
	targetPort = 2
)

// A second argument-free run in a directory that already holds a lazyslice.yml
// re-emits the endpoint blocks byte for byte.
//
// The ladder's rung 0 supplies both endpoints from the committed file, so the
// run names neither on the command line; the file's own `from:` and `service:`
// are what the next file has to carry. Rebuilding both candidates as
// pipeline.FromFlag with no label — which is what internal/core did before
// T-0060 — rewrote a committed `from: compose` / `service: db` as `from: flag`
// on every re-run, against ARCHITECTURE.md section 10's example. Before the
// ladder existed such a run stopped at exit 3 and never reached emit, which is
// why the loss only became reachable with it.
func TestARerunDoesNotDowngradeTheCommittedProvenance(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lazyslice.yml")

	root := ref.TableRef{Schema: "public", Name: "customer"}
	committed := &pipeline.Config{
		Version:     emit.Version,
		Tool:        "0.1.0",
		Source:      pipeline.FromCompose,
		SourceRef:   dsn.Ref{Host: closedHost, Port: sourcePort, Database: "pagila", User: "app_ro"},
		SourceLabel: "db",
		Target:      pipeline.FromCompose,
		TargetRef:   dsn.Ref{Host: closedHost, Port: targetPort, Database: "pagila_test", User: "app"},
		TargetLabel: "db-test",
		Root:        root,
		Take:        200,
	}
	if err := emit.New(emit.Options{Tool: "0.1.0"}).Write(path, committed); err != nil {
		t.Fatalf("writing the committed file: %v", err)
	}

	// The second run: no --source, no --target, this directory's yml.
	r := newRun(t, Request{Mode: ModeRun, Workdir: dir, ConfigPath: path})
	if err := r.readConfig(); err != nil {
		t.Fatalf("readConfig: %v", err)
	}
	// Rung 0 answers both sides, so this walks no other rung and makes no
	// Docker call (ADR-008 section 1).
	fillCandidates(t, r)

	if r.sourceProv != pipeline.FromCompose || r.sourceLabel != "db" {
		t.Errorf("source provenance/label = %v/%q, want compose/%q",
			r.sourceProv, r.sourceLabel, "db")
	}
	if r.targetProv != pipeline.FromCompose || r.targetLabel != "db-test" {
		t.Errorf("target provenance/label = %v/%q, want compose/%q",
			r.targetProv, r.targetLabel, "db-test")
	}
	// The candidates the emit stage will read, as discover and openTarget built
	// them. This is the pair the pre-T-0060 code rebuilt as FromFlag.
	if r.sourceCand.Provenance != pipeline.FromCompose || r.sourceCand.Label != "db" {
		t.Errorf("discover built the source candidate as %v/%q, want compose/%q",
			r.sourceCand.Provenance, r.sourceCand.Label, "db")
	}
	if r.targetCand.Provenance != pipeline.FromCompose || r.targetCand.Label != "db-test" {
		t.Errorf("openTarget built the target candidate as %v/%q, want compose/%q",
			r.targetCand.Provenance, r.targetCand.Label, "db-test")
	}

	r.plan = &pipeline.Plan{Root: root, Take: committed.Take}
	rewritten := filepath.Join(dir, "lazyslice.rerun.yml")
	emitAgain(t, r, rewritten)

	before, after := endpointBlock(t, path), endpointBlock(t, rewritten)
	if !strings.Contains(before, "from: compose") || !strings.Contains(before, "service: db") {
		t.Fatalf("the committed file does not record the compose provenance this test is about:%s", before)
	}
	if before != after {
		t.Errorf("the re-run rewrote the endpoint blocks.\nbefore:%s\nafter:%s", before, after)
	}
}

// A run that names both endpoints on the command line records `from: flag` and
// no service, whatever the ladder would have found.
//
// This is the other half of the same rule. pipeline.FromYml is Provenance's
// zero value, so the two stampings at the top of resolveEndpoints are the only
// thing between a flag-named endpoint and a file that says the committed yml
// named this database — the same corruption as above, in the other direction,
// and on the commonest run there is.
func TestAFlagNamedEndpointIsRecordedAsFromFlag(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lazyslice.yml")

	// No committed file in this directory, so rung 0 supplies nothing and the
	// flags are the only thing that can answer.
	r := newRun(t, Request{
		Mode:       ModeRun,
		Workdir:    dir,
		ConfigPath: path,
		Source:     "postgres://app_ro@127.0.0.1:1/pagila",
		Target:     "postgres://app@127.0.0.1:2/pagila_test",
	})
	if err := r.readConfig(); err != nil {
		t.Fatalf("readConfig: %v", err)
	}
	fillCandidates(t, r)

	if r.sourceCand.Provenance != pipeline.FromFlag || r.sourceCand.Label != "" {
		t.Errorf("discover built the source candidate as %v/%q, want flag with no label",
			r.sourceCand.Provenance, r.sourceCand.Label)
	}
	if r.targetCand.Provenance != pipeline.FromFlag || r.targetCand.Label != "" {
		t.Errorf("openTarget built the target candidate as %v/%q, want flag with no label",
			r.targetCand.Provenance, r.targetCand.Label)
	}

	r.plan = &pipeline.Plan{Root: ref.TableRef{Schema: "public", Name: "customer"}, Take: 200}
	emitAgain(t, r, path)

	block := endpointBlock(t, path)
	if n := strings.Count(block, "from: flag"); n != 2 {
		t.Errorf("%d of the two endpoint blocks record `from: flag`:%s", n, block)
	}
	if n := strings.Count(block, `service: ""`); n != 2 {
		t.Errorf("a flag-named endpoint was given a service name:%s", block)
	}
}

// newRun is a run wired the way Run wires one, minus the event channel: these
// tests call stages directly and read the run's fields rather than the sink.
func newRun(t *testing.T, req Request) *run {
	t.Helper()

	r := &run{req: normalise(req), sink: event.Discard}
	t.Cleanup(func() { r.close(context.Background()) })
	return r
}

// fillCandidates runs the two stages that build the endpoint candidates.
//
// It is what makes these tests tests of internal/core: r.sourceCand and
// r.targetCand are built inside discover and openTarget from the provenance the
// ladder resolved, and a test that built its own pair with candidateOf would
// still pass with those two lines reverted to `candidateOf(ref,
// pipeline.FromFlag, "")`, which is the state T-0060 fixed.
//
// Both stages refuse, because both endpoints name a closed port and neither
// stage gets a statement answered. Each builds its candidate before it issues
// one, so the part under test has already run.
func fillCandidates(t *testing.T, r *run) {
	t.Helper()

	if err := r.discover(t.Context()); err == nil {
		t.Fatal("discover reached a database on a closed port")
	}
	if r.sourceCand.IsZero() {
		t.Fatal("discover returned before it built the source candidate")
	}
	err := r.openTarget(t.Context())
	if err == nil {
		t.Fatal("openTarget reached a database on a closed port")
	}
	if r.targetCand.IsZero() {
		t.Fatal("openTarget returned before it built the target candidate")
	}

	// The gate itself refuses a closed port with CodeUnreachable (the dial
	// failure surfaces there, not at OpenTarget or pg.Connect, because
	// pgxpool.NewWithConfig never dials) and that refusal must still carry the
	// host and reason Args unreachableTarget fills — a bare wrap leaves the
	// catalogue row's {host}/{reason} unfilled (T-0071 review).
	var s *Stop
	if !errors.As(err, &s) {
		t.Fatalf("openTarget returned %T, want *Stop", err)
	}
	if s.Code != pg.CodeUnreachable {
		t.Fatalf("stop code = %s, want %s", s.Code, pg.CodeUnreachable)
	}
	if s.Args[event.ArgHost] == "" {
		t.Error("stop has no ArgHost: the rendered line would show {host}")
	}
	// Not just non-empty: the dial error pgx returns for a closed port is
	// multi-line, with the gate's own "pg: gate: connecting to the target:"
	// preamble on line 1 and the driver's cause ("...connect: connection
	// refused") on the lines after it. A reason that only proves non-empty
	// would still pass if oneLine regressed to keeping the preamble and
	// dropping the cause (T-0071 review).
	reason := s.Args[event.ArgReason]
	if reason == "" {
		t.Error("stop has no ArgReason: the rendered line would show {reason}")
	}
	if !strings.Contains(reason, "refused") {
		t.Errorf("ArgReason = %q, want the driver's dial cause (e.g. \"connection refused\")", reason)
	}
	if strings.HasSuffix(strings.TrimSpace(reason), ":") {
		t.Errorf("ArgReason = %q, ends in a dangling colon with nothing after it", reason)
	}
}

// emitAgain runs the run's own emit stage against path.
//
// The stages between discover and emit never ran, so the two outputs emit reads
// besides the plan are supplied here as empty: nothing in either test is about
// what they hold, and the endpoint blocks are built from the candidates.
func emitAgain(t *testing.T, r *run, path string) {
	t.Helper()

	r.schema = &pipeline.Schema{}
	r.cls = &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{}}
	r.req.ConfigPath = path
	if err := r.emitConfig(nil); err != nil {
		t.Fatalf("emitConfig: %v", err)
	}
}

// endpointBlock is the `source:` and `target:` blocks of an emitted file: every
// line from `source:` up to `root:`, which is what follows them (document.go).
func endpointBlock(t *testing.T, path string) string {
	t.Helper()

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	text := string(body)
	start := strings.Index(text, "\nsource:")
	end := strings.Index(text, "\nroot:")
	if start < 0 || end <= start {
		t.Fatalf("%s has no source/target block:\n%s", path, text)
	}
	return text[start:end]
}
