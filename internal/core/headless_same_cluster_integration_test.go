// SPDX-License-Identifier: Apache-2.0

//go:build integration

package core

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Liarea/lazyslice/internal/discover"
	"github.com/Liarea/lazyslice/internal/dsn"
	"github.com/Liarea/lazyslice/internal/emit"
	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/testutil"
)

// TestAHeadlessRunRefusesWhenEveryCandidateIsOnTheSourceCluster reproduces the
// 2026-09-15 red team's run: --source pointing at a container, no --target,
// --yes. The only other database the ladder can see is on the source's own
// cluster, so T-0184 / ADR-013 (proposed) refuses at exit 4 naming --target,
// ahead of chooseTarget's tie-break, rather than letting the ladder pick that
// database and write there in silence the way the red team's run did.
func TestAHeadlessRunRefusesWhenEveryCandidateIsOnTheSourceCluster(t *testing.T) {
	ctx := t.Context()
	testutil.SkipWithoutDocker(ctx, t)

	admin := testutil.Postgres(ctx, t, "")
	// One cluster, two databases — the same shape as the red team's run: the
	// production container the source is pointed at, and nothing reachable
	// anywhere else.
	source := createDatabase(ctx, t, admin, "prod_app")
	sibling := createDatabase(ctx, t, admin, "prod_maint_test")

	execOn(ctx, t, source,
		`CREATE TABLE customer (id int PRIMARY KEY, email text)`,
		`INSERT INTO customer VALUES (1, 'a@example.com')`,
	)
	// Empty and eligible by ARCHITECTURE.md section 9 rule 1: nothing about
	// this database itself is refused. It is what the tie-break would have
	// picked, and it must never be opened.
	_ = sibling

	dir := t.TempDir()
	quietRungs(t)
	// Rung 1 is where both candidates come from, so this test does not depend
	// on what else is reachable on the machine's own Docker daemon.
	envFile := "DATABASE_URL=" + source + "\nPOSTGRES_URL=" + sibling + "\n"
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(envFile), 0o600); err != nil {
		t.Fatalf("writing .env: %v", err)
	}

	var codes []event.Code
	sink := event.SinkFunc(func(e event.Event) { codes = append(codes, e.Code) })

	req := Request{
		Mode:    ModeRun,
		Workdir: dir,
		// A non-local endpoint yields no rung 3 candidate and makes no socket
		// call (ADR-008 section 3), which keeps this test off the daemon.
		DockerHost: "tcp://staging.example:2375",
		ConfigPath: filepath.Join(dir, "lazyslice.yml"),
		SecretFile: filepath.Join(dir, "lazyslice.secret"),
		NoConfig:   true,
		Yes:        true,
	}

	_, err := Run(ctx, req, sink)

	var stop *Stop
	if !errors.As(err, &stop) {
		t.Fatalf("Run = %v, want the headless same-cluster refusal", err)
	}
	if stop.Exit != exitTarget {
		t.Errorf("exit = %d, want %d", stop.Exit, exitTarget)
	}
	if stop.Code != discover.CodeTargetHeadlessSameCluster {
		t.Errorf("code = %s, want %s", stop.Code, discover.CodeTargetHeadlessSameCluster)
	}
	if got := stop.Args[event.ArgFlag]; got != "--target" {
		t.Errorf("the refusal names %q, want --target", got)
	}
	if n := count(codes, discover.CodeTargetHeadlessSameCluster); n != 1 {
		t.Errorf("%s was emitted %d time(s), want once", discover.CodeTargetHeadlessSameCluster, n)
	}

	// The would-be target was never opened, never truncated and never
	// loaded — a run that fell through to it would have created the schema
	// and the marker table there, the way the red team's run did.
	if n := userTables(ctx, t, sibling); n != 0 {
		t.Errorf("%s holds %d table(s): the run wrote to the source's own cluster in silence", sibling, n)
	}
}

// TestAnExplicitTargetOnTheSourceClusterStaysEligibleHeadless is the other
// half of T-0184 / ADR-013 (proposed): a --target named on the command line,
// even on the source's own cluster, is never touched by the new refusal — it
// short-circuits the ladder before the check runs at all (ADR-008 section 1),
// exactly as ADR-008 section 5 says a second database on one cluster stays
// eligible.
func TestAnExplicitTargetOnTheSourceClusterStaysEligibleHeadless(t *testing.T) {
	ctx := t.Context()
	testutil.SkipWithoutDocker(ctx, t)

	admin := testutil.Postgres(ctx, t, "")
	source := createDatabase(ctx, t, admin, "named_source")
	target := createDatabase(ctx, t, admin, "named_target")

	execOn(ctx, t, source,
		`CREATE TABLE customer (id int PRIMARY KEY, email text)`,
		`INSERT INTO customer VALUES (1, 'a@example.com')`,
	)

	dir := t.TempDir()
	quietRungs(t)

	var codes []event.Code
	sink := event.SinkFunc(func(e event.Event) { codes = append(codes, e.Code) })

	req := Request{
		Mode:       ModeRun,
		Workdir:    dir,
		Source:     source,
		Target:     target,
		DockerHost: "tcp://staging.example:2375",
		ConfigPath: filepath.Join(dir, "lazyslice.yml"),
		SecretFile: filepath.Join(dir, "lazyslice.secret"),
		NoConfig:   true,
		Yes:        true,
		Root:       "public.customer",
	}

	if _, err := Run(ctx, req, sink); err != nil {
		t.Fatalf("Run = %v, want a run that completes: an explicit --target on the "+
			"source's cluster is still eligible", err)
	}
	if n := count(codes, discover.CodeTargetHeadlessSameCluster); n != 0 {
		t.Errorf("%s was emitted %d time(s), want zero: an explicitly named target "+
			"must not trip the headless same-cluster refusal", discover.CodeTargetHeadlessSameCluster, n)
	}
}

// TestACommittedYmlTargetOnTheSourceClusterStaysEligibleHeadless is ADR-013
// review finding 1: a target named by rung 0 — a committed lazyslice.yml's
// target: block, not a --target flag — is an endpoint the operator named too,
// and it must stay eligible headlessly exactly as an explicit --target does.
//
// Before this fix, rung 0 carried the *file's own* provenance forward rather
// than pipeline.FromYml (internal/discover/discover.go's doc comment on why:
// the file records where the endpoint was found, not that it was read back
// from a file), so a committed yml recording `target: from: env` produced the
// same TargetProvenance a ladder-chosen environment-variable candidate would
// have — and openTarget's gate step, which used to key its T-0184 escalation
// on TargetProvenance being neither FromFlag nor FromYml, could not tell the
// two apart. A run with `--yes` and no --target would read the committed
// target at rung 0 (short-circuiting the ladder, so discover's own
// same-cluster check never ran) and then refuse at the gate for a database
// the operator's own file already named. This test pins the fix: the run
// completes and only warns.
func TestACommittedYmlTargetOnTheSourceClusterStaysEligibleHeadless(t *testing.T) {
	ctx := t.Context()
	testutil.SkipWithoutDocker(ctx, t)

	admin := testutil.Postgres(ctx, t, "")
	source := createDatabase(ctx, t, admin, "yml_named_source")
	target := createDatabase(ctx, t, admin, "yml_named_target")

	execOn(ctx, t, source,
		`CREATE TABLE customer (id int PRIMARY KEY, email text)`,
		`INSERT INTO customer VALUES (1, 'a@example.com')`,
	)

	dir := t.TempDir()
	quietRungs(t)
	// The reconstructed rung-0 DSN carries no password (ADR-004: a yml records
	// a reference, never a credential), so it is recovered from the ordinary
	// PGPASSWORD source exactly as an operator's own database would be.
	// quietRungs cleared it above; set it back after.
	t.Setenv("PGPASSWORD", "lazyslice")

	_, targetRef, err := dsn.Parse(target)
	if err != nil {
		t.Fatalf("parsing the target URL to build its committed reference: %v", err)
	}
	targetRef.Params = nil

	configPath := filepath.Join(dir, "lazyslice.yml")
	committed := &pipeline.Config{
		Version:     emit.Version,
		Target:      pipeline.FromEnvVar,
		TargetRef:   targetRef,
		TargetLabel: "DATABASE_URL",
	}
	if err := emit.New(emit.Options{}).Write(configPath, committed); err != nil {
		t.Fatalf("writing the committed yml: %v", err)
	}

	var codes []event.Code
	sink := event.SinkFunc(func(e event.Event) { codes = append(codes, e.Code) })

	req := Request{
		Mode:       ModeRun,
		Workdir:    dir,
		Source:     source,
		DockerHost: "tcp://staging.example:2375",
		ConfigPath: configPath,
		SecretFile: filepath.Join(dir, "lazyslice.secret"),
		Yes:        true,
		Root:       "public.customer",
	}

	if _, err := Run(ctx, req, sink); err != nil {
		t.Fatalf("Run = %v, want a run that completes: a target named by a committed "+
			"lazyslice.yml on the source's cluster is still eligible headlessly", err)
	}
	if n := count(codes, CodeTargetGateSameCluster); n != 0 {
		t.Errorf("%s was emitted %d time(s), want zero: a target named by rung 0 "+
			"must not trip the gate's headless same-cluster refusal", CodeTargetGateSameCluster, n)
	}
	if n := count(codes, discover.CodeTargetHeadlessSameCluster); n != 0 {
		t.Errorf("%s was emitted %d time(s), want zero: naming the target at rung 0 "+
			"short-circuits the ladder's own check too", discover.CodeTargetHeadlessSameCluster, n)
	}
	if n := count(codes, CodeTargetSameCluster); n != 1 {
		t.Errorf("%s was emitted %d time(s), want once: the operator still sees the "+
			"same-cluster warning even though the run is not refused", CodeTargetSameCluster, n)
	}
}
