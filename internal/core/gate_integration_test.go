// SPDX-License-Identifier: Apache-2.0

//go:build integration

package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pg"
	"github.com/Liarea/lazyslice/internal/testutil"
)

// A gate refusal of the chosen target ends the run; the runner-up is never
// tried (T-0062).
//
// ADR-008 section 5's tie-break is written "among eligible targets", and
// eligibility is Target.Gate's verdict — which needs a live connection the
// ladder's 1 s dial budget does not allow for. So internal/discover ranks the
// target-shaped candidates, hands internal/core the winner, and Request carries
// that one target and not a list: when the gate says no, the run stops at exit 4
// with the gate's own refusal rather than falling through to the next candidate.
//
// This is the behaviour, and this test is what pins it. Two databases on the
// cluster are target-shaped: app_test, which the name rule ranks first and which
// holds a row, and app_spare, which is empty and would pass. The run must refuse
// app_test and leave app_spare untouched — not silently load into the runner-up,
// which is a write to a database the operator was never shown.
func TestAGateRefusalEndsTheRunInsteadOfTryingTheRunnerUp(t *testing.T) {
	ctx := t.Context()
	testutil.SkipWithoutDocker(ctx, t)

	admin := testutil.Postgres(ctx, t, "")
	// One cluster, three databases. A second database on the same cluster is
	// eligible by ARCHITECTURE.md section 9 rule 1 — it is the common compose
	// setup — so the refusal this test asserts is the emptiness rule and not
	// the identity one.
	source := createDatabase(ctx, t, admin, "app_source")
	refused := createDatabase(ctx, t, admin, "app_test")
	spare := createDatabase(ctx, t, admin, "app_spare")

	// The source carries the most tables, which is what makes it the source
	// (section 9: the most-local reachable candidate with the most tables).
	execOn(ctx, t, source,
		`CREATE TABLE customer (id int PRIMARY KEY, email text)`,
		`CREATE TABLE address (id int PRIMARY KEY, line text)`,
		`CREATE TABLE city (id int PRIMARY KEY, name text)`,
		`INSERT INTO customer VALUES (1, 'a@example.com')`,
	)
	// The winner: a database with a row in it, which the gate's rule 5 refuses.
	execOn(ctx, t, refused,
		`CREATE TABLE leftovers (id int PRIMARY KEY)`,
		`INSERT INTO leftovers VALUES (1)`,
	)

	dir := t.TempDir()
	quietRungs(t)
	// Rung 1 is where all three candidates come from, so that the ladder has a
	// runner-up to fall through to without this test depending on what else is
	// running on the machine's Docker daemon.
	envFile := "DATABASE_URL=" + source + "\nPOSTGRES_URL=" + refused + "\nPG_URL=" + spare + "\n"
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(envFile), 0o600); err != nil {
		t.Fatalf("writing .env: %v", err)
	}

	var codes []event.Code
	sink := event.SinkFunc(func(e event.Event) { codes = append(codes, e.Code) })

	req := Request{
		Mode:    ModeRun,
		Workdir: dir,
		// A non-local endpoint yields no rung 3 candidate and makes no socket
		// call (ADR-008 section 3), which keeps the containers this suite starts
		// off the ladder and this test off the daemon.
		DockerHost: "tcp://staging.example:2375",
		ConfigPath: filepath.Join(dir, "lazyslice.yml"),
		SecretFile: filepath.Join(dir, "lazyslice.secret"),
		NoConfig:   true,
		// Not --yes, and a scripted prompter standing in for a controlling
		// terminal: T-0184 / ADR-013 (proposed) refuses a headless run before
		// chooseTarget's tie-break ever runs when every target-shaped
		// candidate is on the source's own cluster, which both app_test and
		// app_spare are here. That refusal is a different assertion than the
		// one this test exists to pin — see
		// TestAHeadlessRunRefusesWhenEveryCandidateIsOnTheSourceCluster below
		// for it — so this test needs to stay interactive.
		//
		// Dropping Yes alone does not do that: "headless" is --yes OR no
		// controlling terminal (internal/discover's prompterFor/isHeadless),
		// and go test itself has no controlling terminal, so an unmodified
		// Request with Yes left false is still headless and still trips this
		// ADR's own refusal instead of the gate's (the 2026-09-16 reverify's
		// finding). prompter is what resolveEndpoints copies onto
		// discover.Options.Prompter, which is all isHeadless checks for — it
		// is never actually asked a question here, because target-shaped
		// candidates already exist and neither Q1 nor Q2 is reached — so its
		// mere presence is enough to make isHeadless treat this run as
		// interactive, exactly as a real controlling terminal would, and reach
		// chooseTarget's tie-break and the gate's own refusal below.
		prompter: stubPrompter{},
	}

	_, err := Run(ctx, req, sink)

	var stop *Stop
	if !errors.As(err, &stop) {
		t.Fatalf("Run = %v, want the gate's refusal", err)
	}
	if stop.Exit != exitTarget || stop.Code != pg.CodeNotEmpty {
		t.Errorf("stop = %s/exit %d, want %s/exit %d — the gate's own rule 5 refusal",
			stop.Code, stop.Exit, pg.CodeNotEmpty, exitTarget)
	}
	if got := stop.Args[event.ArgDatabase]; got != "app_test" {
		t.Errorf("the refusal names %q, want app_test — the candidate the tie-break ranked first", got)
	}
	// One refusal, not two: a run that tried the runner-up would have refused
	// twice, or refused once and then loaded.
	if n := count(codes, pg.CodeNotEmpty); n != 1 {
		t.Errorf("%s was emitted %d time(s), want once", pg.CodeNotEmpty, n)
	}

	// The runner-up was never opened, never truncated and never loaded. A
	// fall-through would have created the schema and the marker table in it.
	if n := userTables(ctx, t, spare); n != 0 {
		t.Errorf("app_spare holds %d table(s): the run fell through to the runner-up "+
			"and wrote to a database it never showed the operator", n)
	}
}

// stubPrompter is a discover.Prompter that answers as the old interactive run
// did, for a test that needs resolveEndpoints's ladder to treat a run as
// interactive with no controlling terminal available (T-0184, ADR-013 review,
// the 2026-09-16 reverify). It is never actually asked a question by
// TestAGateRefusalEndsTheRunInsteadOfTryingTheRunnerUp — target-shaped
// candidates already exist there, so neither Q1 nor Q2 is reached — but
// isHeadless only checks whether somebody is here to ask, so its presence
// alone is what keeps that test off this ADR's own refusal.
type stubPrompter struct{}

func (stubPrompter) Confirm(_ string, def bool) (bool, error) { return def, nil }
func (stubPrompter) Ask(_ string, def string) (string, error) { return def, nil }
func (stubPrompter) Close() error                             { return nil }

// count is how many times a code reached the sink.
func count(codes []event.Code, want event.Code) int {
	n := 0
	for _, got := range codes {
		if got == want {
			n++
		}
	}
	return n
}

// createDatabase makes one more database on the cluster admin points at and
// returns a connection URL for it.
func createDatabase(ctx context.Context, t *testing.T, admin, name string) string {
	t.Helper()

	conn, err := pgx.Connect(ctx, admin)
	if err != nil {
		t.Fatalf("connecting to create %s: %v", name, err)
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()

	if _, execErr := conn.Exec(ctx, `CREATE DATABASE `+pgx.Identifier{name}.Sanitize()); execErr != nil {
		t.Fatalf("creating %s: %v", name, execErr)
	}

	u, err := testutil.URL(admin)
	if err != nil {
		t.Fatalf("parsing the container URL: %v", err)
	}
	u.Path = "/" + name
	return u.String()
}

// execOn runs statements against one database, in order.
func execOn(ctx context.Context, t *testing.T, connURL string, statements ...string) {
	t.Helper()

	conn, err := pgx.Connect(ctx, connURL)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()

	for _, sql := range statements {
		if _, err := conn.Exec(ctx, sql); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
}

// userTables counts the ordinary and partitioned tables in a database, the same
// way the ladder's own dial does.
func userTables(ctx context.Context, t *testing.T, connURL string) int {
	t.Helper()

	conn, err := pgx.Connect(ctx, connURL)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()

	var n int
	const sql = `SELECT count(*)::int
  FROM pg_class c
  JOIN pg_namespace n ON n.oid = c.relnamespace
 WHERE c.relkind IN ('r', 'p')
   AND n.nspname NOT IN ('pg_catalog', 'information_schema')
   AND n.nspname NOT LIKE 'pg\_toast%'`
	if err := conn.QueryRow(ctx, sql).Scan(&n); err != nil {
		t.Fatalf("counting tables: %v", err)
	}
	return n
}

// quietRungs empties the environment every rung of the ladder reads, so that
// the developer's own $DATABASE_URL or libpq settings cannot decide what this
// test asserts. The Docker endpoint is set by the request, not here.
func quietRungs(t *testing.T) {
	t.Helper()

	for _, name := range []string{
		"DATABASE_URL", "POSTGRES_URL", "PG_URL", "DB_URL",
		"PGSERVICE", "PGHOST", "PGHOSTADDR", "PGDATABASE", "PGPORT", "PGUSER",
		"PGPASSWORD", "PGPASSFILE",
	} {
		t.Setenv(name, "")
	}
}

// ARCHITECTURE.md §9 rule 1 and THREAT_MODEL.md T2 both say a target on the
// source's own cluster is eligible *and* that the header carries
// `same cluster as source` as a warning. internal/pg has computed
// Eligibility.SameCluster since the gate was written and **nothing in this
// package or internal/render read the field** — the gate's own test asserted
// the boolean and never the rendered line, which is how the omission survived
// until the 2026-09-15 red team pointed --source at production with no --target
// and watched a masked slice land in the production server's own maintenance
// database under exit 0 with nothing said about it.
//
// This asserts the line and not the boolean, which is the whole point.
func TestASameClusterTargetIsWarnedAbout(t *testing.T) {
	ctx := t.Context()
	testutil.SkipWithoutDocker(ctx, t)

	admin := testutil.Postgres(ctx, t, "")
	source := createDatabase(ctx, t, admin, "warn_source")
	target := createDatabase(ctx, t, admin, "warn_target")

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
		t.Fatalf("Run = %v, want a run that completes: a second database on one cluster is eligible", err)
	}
	if n := count(codes, CodeTargetSameCluster); n != 1 {
		t.Errorf("%s was emitted %d time(s), want once: the run wrote to the source's own server "+
			"and said nothing about it", CodeTargetSameCluster, n)
	}
}
