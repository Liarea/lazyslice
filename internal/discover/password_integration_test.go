// SPDX-License-Identifier: Apache-2.0

//go:build integration

package discover

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Liarea/lazyslice/internal/dsn"
	"github.com/Liarea/lazyslice/internal/testutil"
)

// TestPasswordCommandSuppliesEndpointPassword is T-0213's positive case: a
// connection string with no password, a --password-command script that
// prints the real one, and a real dial that only succeeds if ResolvePassword
// actually ran the script and put its output where pg.Connect reads a
// password from.
//
// It is an integration test, in the manner of TestDiscoversARunningContainer:
// the claim is about a real server accepting or refusing an authentication
// attempt, which a stubbed dial cannot prove either way.
func TestPasswordCommandSuppliesEndpointPassword(t *testing.T) {
	ctx := t.Context()
	testutil.SkipWithoutDocker(ctx, t)

	connURL := testutil.Postgres(ctx, t, "")
	u, err := url.Parse(connURL)
	if err != nil {
		t.Fatalf("parsing %s: %v", connURL, err)
	}
	realPassword, ok := u.User.Password()
	if !ok || realPassword == "" {
		t.Fatalf("testutil.Postgres returned a connection string with no password: %s", connURL)
	}

	// The same connection string with the password removed: the shape a
	// passwordless endpoint takes when internal/core calls ResolvePassword
	// directly, for --source/--target named on the command line or resolved
	// from a committed lazyslice.yml reference (internal/core/run.go). A
	// candidate the ladder itself discovers is a different call path —
	// probe resolves the password there, before dialling, rather than going
	// through this exported function — and is covered by
	// TestDiscoveredCandidateAuthenticatesWithPasswordCommand
	// (discover_integration_test.go) instead.
	noPasswordURL := *u
	noPasswordURL.User = url.User(u.User.Username())
	passwordless := noPasswordURL.String()

	if avail := passwordAvailable(dsn.DSN(passwordless)); avail {
		t.Fatalf("passwordAvailable(%q) = true, want false (the test needs a candidate with no password so the command is what supplies one, not $PGPASSWORD or ~/.pgpass on the machine running this test)", passwordless)
	}

	script := writePasswordScript(t, realPassword)

	resolved, err := ResolvePassword(ctx, dsn.DSN(passwordless), script)
	if err != nil {
		t.Fatalf("ResolvePassword(%q, %q) returned an error: %v", passwordless, script, err)
	}
	if resolved == dsn.DSN(passwordless) {
		t.Fatalf("ResolvePassword returned the connection string unchanged; the password never reached it")
	}

	cfg, err := pgxpool.ParseConfig(string(resolved))
	if err != nil {
		t.Fatalf("parsing the resolved DSN: %v", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("pgxpool.NewWithConfig: %v", err)
	}
	defer pool.Close()

	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(dialCtx); err != nil {
		t.Fatalf("the endpoint did not authenticate with the password --password-command supplied: %v", err)
	}
}

// TestPasswordCommandFailureRefusesWithoutTheOutput is the failure half: a
// script whose exit is non-zero, one that prints nothing, and one that runs
// past the 30 second budget each refuse with a reason that names only what
// happened — never anything the script printed.
func TestPasswordCommandFailureRefusesWithoutTheOutput(t *testing.T) {
	ctx := t.Context()

	passwordless := "postgres://lazyslice@127.0.0.1:1/lazyslice_test"

	t.Run("non-zero exit", func(t *testing.T) {
		script := writeScript(t, "#!/bin/sh\necho 'a secret nobody should see' >&2\nexit 3\n")
		_, err := ResolvePassword(ctx, dsn.DSN(passwordless), script)
		pce := requirePasswordCommandError(t, err)
		if pce.Reason != "exited exit status 3" {
			t.Errorf("Reason = %q, want the exit status and nothing else", pce.Reason)
		}
	})

	t.Run("empty stdout", func(t *testing.T) {
		script := writeScript(t, "#!/bin/sh\nexit 0\n")
		_, err := ResolvePassword(ctx, dsn.DSN(passwordless), script)
		pce := requirePasswordCommandError(t, err)
		if pce.Reason != "printed no password" {
			t.Errorf("Reason = %q, want %q", pce.Reason, "printed no password")
		}
	})

	t.Run("timeout", func(t *testing.T) {
		if testing.Short() {
			t.Skip("30s budget under -short")
		}
		script := writeScript(t, "#!/bin/sh\nsleep 35\necho too-late\n")
		start := time.Now()
		_, err := ResolvePassword(ctx, dsn.DSN(passwordless), script)
		elapsed := time.Since(start)
		pce := requirePasswordCommandError(t, err)
		if pce.Reason != "timed out after 30s" {
			t.Errorf("Reason = %q, want %q", pce.Reason, "timed out after 30s")
		}
		// The message names a 30 second budget; this proves it was actually
		// kept rather than only asserting the string a stub timeout can pass
		// too. c.Stdout is a *bytes.Buffer, so os/exec waits on a pipe and not
		// on the shell's own exit — without WaitDelay, "sleep 35" alone (with
		// nothing inheriting the pipe past the shell) would still return near
		// PasswordCommandTimeout, but a script that backgrounds a grandchild
		// holding the pipe open would not, and this bound catches that case
		// too by giving the whole call, WaitDelay included, a generous but
		// finite ceiling.
		if elapsed > PasswordCommandTimeout+10*time.Second {
			t.Errorf("ResolvePassword took %s to time out, want close to %s (WaitDelay should stop it from waiting on the command's own sleep)", elapsed, PasswordCommandTimeout)
		}
	})
}

// TestPasswordCommandDoesNotBlockOnAGrandchildHoldingStdout is the reviewed
// gap in T-0213's timeout: exec.CommandContext kills only the shell, not a
// process the shell backgrounds, and with c.Stdout a *bytes.Buffer, os/exec
// waits for that pipe to close and not for the shell's own exit. A script
// that backgrounds a long-lived grandchild inheriting the pipe — a credential
// helper spawning an agent, or exactly this reproduction — used to keep
// runPasswordCommand blocked until the grandchild exited, 45 seconds against
// a "30 second budget" the refusal text asserted and did not keep. WaitDelay
// is what closes the pipe out from under it once the shell itself has exited.
func TestPasswordCommandDoesNotBlockOnAGrandchildHoldingStdout(t *testing.T) {
	if testing.Short() {
		t.Skip("spawns a 45s background process")
	}
	ctx := t.Context()
	passwordless := "postgres://lazyslice@127.0.0.1:1/lazyslice_test"

	script := writeScript(t, "#!/bin/sh\nsleep 45 & echo ok; exit 0\n")
	start := time.Now()
	_, err := ResolvePassword(ctx, dsn.DSN(passwordless), script)
	elapsed := time.Since(start)

	// The shell itself exits almost immediately, printing "ok" first; only
	// the backgrounded sleep keeps the stdout pipe open past that. Without
	// WaitDelay, c.Run() waited on that pipe closing and returned after the
	// grandchild's own 45s sleep (measured 45.01s, the finding this pins).
	// With WaitDelay, the pipe is force-closed about a second after the
	// shell exits, which surfaces here as a *PasswordCommandError — Go's own
	// exec.ErrWaitDelay ("WaitDelay expired before I/O complete"), because
	// the forced close cuts the copy off before it sees a clean EOF. That is
	// the correct failure mode: a script that leaves a background job
	// holding the pipe open is not one runPasswordCommand can safely trust
	// the output of, and refusing fast beats hanging for 45 seconds.
	if err == nil {
		t.Fatal("ResolvePassword succeeded; want a *PasswordCommandError once WaitDelay force-closes the pipe out from under the grandchild")
	}
	var pce *PasswordCommandError
	if !errors.As(err, &pce) {
		t.Fatalf("error is %T, want *PasswordCommandError: %v", err, err)
	}
	if elapsed > 10*time.Second {
		t.Errorf("ResolvePassword took %s to return, want well under the grandchild's 45s sleep "+
			"(WaitDelay should have closed the pipe within a second or two of the shell itself exiting)", elapsed)
	}
}

// TestProbeResolvesPasswordCommandOnceAcrossCandidates is the R2-16 half of
// T-0213 the first landing missed entirely: rung0's own doc comment ("the
// candidate carries no password and the ordinary password sources ...
// --password-command ... supply one") describes a candidate the *ladder*
// found, not only one named with --source/--target, and until this fix round
// nothing in this package ever ran the command for one — passwordAvailable
// saw nothing, probe dialled with no password, and the candidate came back
// unreachable. It also proves the "cached" half of that fix: a credential
// helper must not run once per password-less candidate on the ladder.
func TestProbeResolvesPasswordCommandOnceAcrossCandidates(t *testing.T) {
	dir := t.TempDir()
	counterPath := filepath.Join(dir, "invocations")
	script := writeScript(t, fmt.Sprintf(
		"#!/bin/sh\nprintf 'x' >> %s\nprintf 'secret\\n'\n", shellQuote(counterPath)))

	cache := &passwordCache{}
	// Neither address answers (port 1 is a privileged port nothing listens on
	// in a test sandbox), so this proves what probe did *before* dialling —
	// the injection and the cache — without needing a real server.
	f1 := &found{dsn: dsn.DSN("postgres://lazyslice@127.0.0.1:1/lazyslice_test")}
	f2 := &found{dsn: dsn.DSN("postgres://lazyslice@127.0.0.1:1/lazyslice_test")}

	if passwordAvailable(f1.dsn) {
		t.Fatalf("the candidate DSN already carries a password; the test needs one that doesn't")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	probe(ctx, f1, script, cache)
	probe(ctx, f2, script, cache)

	if !passwordAvailable(f1.dsn) {
		t.Error("probe did not inject the --password-command result into the first candidate's DSN")
	}
	if !passwordAvailable(f2.dsn) {
		t.Error("probe did not inject the --password-command result into the second candidate's DSN")
	}

	b, err := os.ReadFile(counterPath)
	if err != nil {
		t.Fatalf("reading %s: %v", counterPath, err)
	}
	if got := strings.Count(string(b), "x"); got != 1 {
		t.Errorf("--password-command ran %d time(s) across two candidates sharing one cache, want 1", got)
	}
}

func requirePasswordCommandError(t *testing.T, err error) *PasswordCommandError {
	t.Helper()
	if err == nil {
		t.Fatal("ResolvePassword returned no error")
	}
	var pce *PasswordCommandError
	if !errors.As(err, &pce) {
		t.Fatalf("error is %T, want *PasswordCommandError: %v", err, err)
	}
	return pce
}

// writePasswordScript writes a script whose only output, on stdout, is
// password followed by a newline — the shape ResolvePassword's trim is meant
// to strip exactly one of.
func writePasswordScript(t *testing.T, password string) string {
	t.Helper()
	return writeScript(t, fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' %s\n", shellQuote(password)))
}

func writeScript(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "password-command.sh")
	if err := os.WriteFile(path, []byte(body), 0o700); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	if _, err := exec.LookPath("/bin/sh"); err != nil {
		t.Skip("/bin/sh not found")
	}
	return path
}

// shellQuote wraps v in single quotes for the generated script's own
// printf call, escaping any single quote v itself carries. It has nothing to
// do with quoteKeywordValue, which quotes a value for a libpq connection
// string — this quotes one for /bin/sh.
func shellQuote(v string) string {
	return "'" + replaceAllSingleQuotes(v) + "'"
}

func replaceAllSingleQuotes(v string) string {
	out := make([]byte, 0, len(v))
	for i := 0; i < len(v); i++ {
		if v[i] == '\'' {
			out = append(out, '\'', '\\', '\'', '\'')
			continue
		}
		out = append(out, v[i])
	}
	return string(out)
}
