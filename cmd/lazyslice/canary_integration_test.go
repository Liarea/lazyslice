// SPDX-License-Identifier: Apache-2.0

//go:build integration

package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/Liarea/lazyslice/internal/testutil"
)

// This is docs/reviews/2026-09-09/REVIEW.md finding 2, turned into a
// regression at the level the review found it: the output sinks a real
// invocation of the binary actually writes to, not a package's own return
// value. internal/plan/polymorphic_test.go and internal/event's own tests
// hold the two pieces the fix is built from — that a `_type` value never
// reaches a message, and that Args can structurally carry nothing but a
// string — but neither proves that no *other* path (a future one, or one this
// task's audit missed) still slips a source value into stdout, stderr, the
// emitted lazyslice.yml, or a file the run writes. This test is that proof,
// at the boundary the reviewer actually probed: it runs the CLI exactly as an
// operator would, at --json and the highest verbosity this tool has, and greps
// every sink lazyslice can write to.
//
// One canary per text column, not one canary for the whole fixture: a leak
// through a path this suite does not otherwise exercise (a different
// polymorphic finding, a different classify reason, a different verify
// message) still has to name *some* column's canary to be caught, and a
// single shared canary could only ever prove that the one path the review
// found is fixed.
func TestNoCanaryReachesAnOutputSink(t *testing.T) {
	ctx := t.Context()
	testutil.SkipWithoutDocker(ctx, t)
	quietRungs(t)

	admin := testutil.Postgres(ctx, t, "")
	source := createDatabase(ctx, t, admin, "app_canary_source")
	target := createDatabase(ctx, t, admin, "app_canary_target")

	// One canary per text column, unique to this run so a leak can be
	// attributed to the column it came from and cannot collide with anything
	// else the run prints (a table name, a code, a fingerprint).
	canary := func(col string) string {
		var b [8]byte
		if _, err := rand.Read(b[:]); err != nil {
			t.Fatalf("generating a canary for %s: %v", col, err)
		}
		return "canary-" + col + "-" + hex.EncodeToString(b[:])
	}
	emailCanary := canary("email") + "@example.org"
	// thing_type/thing_id is a polymorphic pair (ARCHITECTURE.md §3.2): a
	// `_type` value that maps to no table is exactly finding 2's
	// reproduction (evidence/polymorphic_log.log's `poly.canary@example.org`).
	thingTypeCanary := canary("thing-type")
	noteCanary := canary("note")

	execOn(ctx, t, source,
		`CREATE TABLE public.widgets (
			id integer PRIMARY KEY,
			email text,
			thing_type text,
			thing_id integer,
			note text
		)`,
		fmt.Sprintf(
			`INSERT INTO public.widgets VALUES (1, %s, %s, 42, %s)`,
			quoteLiteral(emailCanary), quoteLiteral(thingTypeCanary), quoteLiteral(noteCanary),
		),
	)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "lazyslice.yml")
	secretPath := filepath.Join(dir, "lazyslice.secret")

	args := []string{
		"--source", source,
		"--target", target,
		"--root", "public.widgets",
		"--config", cfgPath,
		"--secret-file", secretPath,
		"--yes",
		// A non-local Docker endpoint yields no rung-3 candidate and makes no
		// socket call (ADR-008 §3), so this test's own containers stay off the
		// discovery ladder.
		"--docker-host", "tcp://staging.example:2375",
		"--json",
		// The highest verbosity this tool has: the statement trace and stack
		// traces (--debug), and PgError.Detail/Where/Hint no longer withheld
		// (--show-row-values-in-errors). If a canary is going to slip out
		// anywhere, asking for the most verbose transcript possible is where
		// it would.
		"--debug",
		"--show-row-values-in-errors",
	}

	var stdout, stderr bytes.Buffer
	exit := run(ctx, args, &stdout, &stderr)
	if exit != ExitOK {
		t.Fatalf("run(%v) = exit %d, want %d (a clean pass)\nstdout: %s\nstderr: %s",
			args, exit, ExitOK, stdout.String(), stderr.String())
	}

	canaries := map[string]string{
		"email":      emailCanary,
		"thing_type": thingTypeCanary,
		"note":       noteCanary,
	}

	assertNoCanary(t, "stdout", stdout.String(), canaries)
	assertNoCanary(t, "stderr", stderr.String(), canaries)

	cfgBytes, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("reading %s: %v", cfgPath, err)
	}
	assertNoCanary(t, cfgPath, string(cfgBytes), canaries)

	// Every other file the run wrote, secretPath included: lazyslice.secret
	// holds hex the run key, never data read from a row, but it costs nothing
	// to check it and everything else dir now holds alongside the two files
	// named above.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		p := filepath.Join(dir, entry.Name())
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("reading %s: %v", p, err)
		}
		assertNoCanary(t, p, string(b), canaries)
	}
}

// assertNoCanary fails the test, naming the column, if any canary appears in
// text. It does not print the canary itself past what the failing test output
// already would: the value exists only in this test's own memory and process,
// and the assertion names which column's canary was found, not the found text
// at any length beyond that.
func assertNoCanary(t *testing.T, sink, text string, canaries map[string]string) {
	t.Helper()
	for col, c := range canaries {
		if strings.Contains(text, c) {
			t.Errorf("%s carries the %s canary: a source row value reached an output sink (T-0131)", sink, col)
		}
	}
}

// quoteLiteral is a single-quoted SQL string literal with the one escape a
// canary (hex and a fixed prefix) can never need beyond: a literal single
// quote, doubled. It exists so this file does not need a query parameter path
// through execOn for one INSERT.
func quoteLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// ---------- test-only database helpers ----------
//
// internal/core's own integration suite (gate_integration_test.go) carries
// the same three helpers, unexported there as here: this package cannot
// import them, and duplicating three short functions is cheaper than a shared
// test-only package neither T-0131's paths nor internal/core's own CLAUDE.md
// name.

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

// quietRungs empties the environment every rung of the discovery ladder
// reads, so that the developer's own $DATABASE_URL or libpq settings cannot
// decide what this test asserts.
func quietRungs(t *testing.T) {
	t.Helper()

	for _, name := range []string{
		"DATABASE_URL", "POSTGRES_URL", "PG_URL", "DB_URL",
		"PGSERVICE", "PGHOST", "PGHOSTADDR", "PGDATABASE", "PGPORT", "PGUSER",
		"PGPASSWORD", "PGPASSFILE",
	} {
		t.Setenv(name, "")
	}

	// resolveKey treats $LAZYSLICE_SECRET as present the moment LookupEnv
	// says ok, even at "" (core/run.go), so t.Setenv(name, "") — which sets,
	// never unsets — would make it the empty string and fail the run with
	// secret.refused.no_key rather than leaving the ladder to write
	// secretPath's own ephemeral key. It needs a real unset.
	if old, ok := os.LookupEnv("LAZYSLICE_SECRET"); ok {
		t.Cleanup(func() { _ = os.Setenv("LAZYSLICE_SECRET", old) })
		_ = os.Unsetenv("LAZYSLICE_SECRET")
	}
}
