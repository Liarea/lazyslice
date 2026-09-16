// SPDX-License-Identifier: Apache-2.0

//go:build integration

package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Liarea/lazyslice/internal/testutil"
)

// TestPasswordCommandOutputReachesNoSink is T-0213's egress proof,
// TestNoCanaryReachesAnOutputSink's pattern (this file's neighbour) applied to
// the one value that test does not cover: what a --password-command script
// prints on stdout. internal/discover's ResolvePassword is the only reader of
// that stdout, and this drives it through the real binary — the boundary the
// other canary test also probes at — rather than trusting the package's own
// unit tests to be the whole story: a value that reaches internal/core's
// Request.Source, an event Args map, or lazyslice.yml is exactly the kind of
// "one other path this suite does not otherwise exercise" that test's own
// doc comment warns a single shared canary cannot catch, so this uses its
// own.
//
// Two runs, not one. A canary that fails authentication (the first subtest)
// exercises the refusal text and the driver's own error under --debug, but
// the run stops at pg.OpenSource and never reaches emit — so lazyslice.yml,
// the secret file and every other sink the second run's doc comment names
// are never written and never grepped by a failing-canary run alone. The
// second subtest's canary *is* a real database password — a role this test
// creates for the purpose — so the run authenticates, completes through
// emit, and every file it wrote gets checked too.
func TestPasswordCommandOutputReachesNoSink(t *testing.T) {
	ctx := t.Context()
	testutil.SkipWithoutDocker(ctx, t)
	quietRungs(t)

	admin := testutil.Postgres(ctx, t, "")

	t.Run("failing canary refuses without leaking it", func(t *testing.T) {
		source := createDatabase(ctx, t, admin, "app_pwcmd_source")
		// Named explicitly, like the neighbouring canary test's target, so
		// resolveEndpoints short-circuits the ladder entirely (ADR-008 §1) and
		// this run reaches the source's own --password-command dial directly
		// rather than first walking rungs 1-4 for a target none of this test's
		// containers are meant to answer for.
		target := createDatabase(ctx, t, admin, "app_pwcmd_target")

		u, err := testutil.URL(source)
		if err != nil {
			t.Fatalf("parsing %s: %v", source, err)
		}
		passwordUser := u.User.Username()
		noPassword := *u
		noPassword.User = url.User(passwordUser)
		passwordlessSource := noPassword.String()

		canary := randomCanary(t)

		dir := t.TempDir()
		scriptPath := writeCanaryScript(t, dir, canary)

		cfgPath := filepath.Join(dir, "lazyslice.yml")
		secretPath := filepath.Join(dir, "lazyslice.secret")

		args := []string{
			"--source", passwordlessSource,
			"--target", target,
			"--root", "public.widgets",
			"--password-command", scriptPath,
			"--config", cfgPath,
			"--secret-file", secretPath,
			"--yes",
			"--docker-host", "tcp://staging.example:2375",
			"--json",
			"--debug",
			"--show-row-values-in-errors",
		}

		var stdout, stderr bytes.Buffer
		exit := run(ctx, args, &stdout, &stderr)
		// The canary is not the source's real password, so authentication fails
		// and the run refuses; what matters here is where the canary did not go,
		// not the exit code — a leak in a *refusal* is exactly the failure mode
		// this subtest exists to catch, so an unexpected ExitOK is logged but
		// not itself a failure.
		if exit == ExitOK {
			t.Logf("run(%v) unexpectedly returned ExitOK; continuing the egress check regardless", args)
		}

		canaries := map[string]string{"password_command_stdout": canary}
		assertNoCanary(t, "stdout", stdout.String(), canaries)
		assertNoCanary(t, "stderr", stderr.String(), canaries)

		// This run never reaches emit (auth fails before pg.OpenSource
		// returns), so cfgPath is not expected to exist — that is exactly why
		// the second subtest below exists to check it. What is checked here is
		// that nothing this run *did* write, in dir, carries the canary.
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("reading %s: %v", dir, err)
		}
		for _, entry := range entries {
			if entry.IsDir() || entry.Name() == filepath.Base(scriptPath) {
				continue
			}
			p := filepath.Join(dir, entry.Name())
			b, err := os.ReadFile(p)
			if err != nil {
				t.Fatalf("reading %s: %v", p, err)
			}
			assertNoCanary(t, p, string(b), canaries)
		}
	})

	t.Run("real password reaches emit and is not written to any sink", func(t *testing.T) {
		source := createDatabase(ctx, t, admin, "app_pwcmd_source_ok")
		target := createDatabase(ctx, t, admin, "app_pwcmd_target_ok")

		canary := randomCanary(t)
		const role = "app_pwcmd_reader"
		execOn(ctx, t, source,
			`CREATE TABLE public.widgets (id integer PRIMARY KEY)`,
			fmt.Sprintf(`CREATE ROLE %s LOGIN PASSWORD %s`, role, quoteLiteral(canary)),
			fmt.Sprintf(`GRANT USAGE ON SCHEMA public TO %s`, role),
			fmt.Sprintf(`GRANT SELECT ON ALL TABLES IN SCHEMA public TO %s`, role),
		)

		u, err := testutil.URL(source)
		if err != nil {
			t.Fatalf("parsing %s: %v", source, err)
		}
		roleSource := *u
		roleSource.User = url.User(role)
		passwordlessSource := roleSource.String()

		dir := t.TempDir()
		scriptPath := writeCanaryScript(t, dir, canary)

		cfgPath := filepath.Join(dir, "lazyslice.yml")
		secretPath := filepath.Join(dir, "lazyslice.secret")

		args := []string{
			"--source", passwordlessSource,
			"--target", target,
			"--root", "public.widgets",
			"--password-command", scriptPath,
			"--config", cfgPath,
			"--secret-file", secretPath,
			"--yes",
			"--docker-host", "tcp://staging.example:2375",
			"--json",
			"--debug",
			"--show-row-values-in-errors",
		}

		var stdout, stderr bytes.Buffer
		exit := run(ctx, args, &stdout, &stderr)
		if exit != ExitOK {
			t.Fatalf("run(%v) = exit %d, want %d (the canary IS the role's real password, so this run should authenticate and complete)\nstdout: %s\nstderr: %s",
				args, exit, ExitOK, stdout.String(), stderr.String())
		}

		canaries := map[string]string{"password_command_stdout": canary}
		assertNoCanary(t, "stdout", stdout.String(), canaries)
		assertNoCanary(t, "stderr", stderr.String(), canaries)

		// This is the sink THREAT_MODEL.md T5 cares most about: lazyslice.yml's
		// own header promises it never contains a secret. A run that authenticated
		// with the canary must still not have written it there — failing this
		// read is a test failure, not a skipped assertion, because a completed
		// run that did not write its config file is itself a bug.
		cfgBytes, err := os.ReadFile(cfgPath)
		if err != nil {
			t.Fatalf("reading %s: %v (a completed run should have written its config)", cfgPath, err)
		}
		assertNoCanary(t, cfgPath, string(cfgBytes), canaries)

		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("reading %s: %v", dir, err)
		}
		for _, entry := range entries {
			if entry.IsDir() || entry.Name() == filepath.Base(scriptPath) {
				continue
			}
			p := filepath.Join(dir, entry.Name())
			b, err := os.ReadFile(p)
			if err != nil {
				t.Fatalf("reading %s: %v", p, err)
			}
			assertNoCanary(t, p, string(b), canaries)
		}
	})
}

// randomCanary returns a value unique to this call, so a leak can be
// attributed to the run that produced it and cannot collide with anything
// else a run prints.
func randomCanary(t *testing.T) string {
	t.Helper()
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatalf("generating the canary: %v", err)
	}
	return "canary-password-command-" + hex.EncodeToString(b[:])
}

// writeCanaryScript writes a --password-command script whose only output is
// canary followed by a newline, and returns its path.
func writeCanaryScript(t *testing.T, dir, canary string) string {
	t.Helper()
	scriptPath := filepath.Join(dir, "password-command.sh")
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' %s\n", shellQuoteCanary(canary))
	if err := os.WriteFile(scriptPath, []byte(script), 0o700); err != nil {
		t.Fatalf("writing %s: %v", scriptPath, err)
	}
	return scriptPath
}

// shellQuoteCanary wraps v in single quotes for the generated script's own
// printf call. The canary this file builds never contains one, but the
// escape costs nothing and matches internal/discover's own test helper.
func shellQuoteCanary(v string) string {
	return "'" + strings.ReplaceAll(v, "'", `'\''`) + "'"
}
