// SPDX-License-Identifier: Apache-2.0

package discover

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/Liarea/lazyslice/internal/dsn"
)

// PasswordCommandTimeout is T-0213's 30 second budget for --password-command.
// It runs outside the per-candidate dial (ARCHITECTURE.md §9's 1 s budget is
// for the ladder's own probing of *discovered* candidates, never for a
// command the operator supplied for an endpoint they named), so it gets a
// budget of its own rather than borrowing one that was sized for something
// else.
const PasswordCommandTimeout = 30 * time.Second

// PasswordCommandError is ResolvePassword's failure: --password-command was
// run and did not supply a usable password. Reason names the exit status, the
// timeout or the empty-output case and nothing else — it is built without
// ever looking at the command's stdout, so there is no path from a value the
// command printed into this message, a refusal, an event or the trace
// (THREAT_MODEL.md T5).
type PasswordCommandError struct {
	Reason string
}

func (e *PasswordCommandError) Error() string {
	return "--password-command " + e.Reason
}

// ResolvePassword fills in a password for d from cmdline when nothing earlier
// in the precedence already supplied one.
//
// The precedence is ARCHITECTURE.md §9's Q4 row, unchanged: a password
// already in the connection string, then $PGPASSWORD, then ~/.pgpass (all
// three are pgconn.ParseConfig's own resolution, via passwordAvailable), and
// only then the command. d is returned unchanged whenever any of the first
// three already resolved, or cmdline is empty — the command never runs
// speculatively.
//
// On success the returned DSN carries the password the command printed; on
// failure d is returned unchanged alongside a *PasswordCommandError naming
// only the exit status, so the caller can refuse loudly without the value
// that would have gone into the connection ever reaching an error message.
func ResolvePassword(ctx context.Context, d dsn.DSN, cmdline string) (dsn.DSN, error) {
	if cmdline == "" || passwordAvailable(d) {
		return d, nil
	}
	pw, err := runPasswordCommand(ctx, cmdline)
	if err != nil {
		return d, err
	}
	return injectPassword(d, pw), nil
}

// runPasswordCommand runs cmdline through the shell — /bin/sh -c on every
// platform but Windows, cmd /C there — with stdin closed (Cmd.Stdin left nil
// connects it to the null device, so the command cannot block reading a
// password prompt back from a run that has none to give it) and a 30 second
// timeout. Its stderr is connected straight to this process's stderr, so an
// operator debugging their own command sees whatever it printed there as it
// printed it; only stdout is captured, and only stdout is ever discarded on
// an error path — this function's own error values are built from the exit
// status alone.
func runPasswordCommand(ctx context.Context, cmdline string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, PasswordCommandTimeout)
	defer cancel()

	shell, flag := "/bin/sh", "-c"
	if runtime.GOOS == "windows" {
		shell, flag = "cmd", "/C"
	}

	c := exec.CommandContext(ctx, shell, flag, cmdline)
	c.Stdin = nil
	c.Stderr = os.Stderr
	var out bytes.Buffer
	c.Stdout = &out
	// exec.CommandContext kills only the shell, not a grandchild that inherits
	// stdout, and c.Stdout being a *bytes.Buffer means os/exec is waiting on a
	// pipe it opened, not on the shell's own exit: without WaitDelay, a
	// credential helper that spawns something long-lived (gpg-agent, op, a
	// keychain daemon) that inherits the pipe keeps c.Run blocked past the
	// context's deadline, and the "timed out after 30s" refusal below would be
	// a budget this call did not actually keep. WaitDelay makes Run stop
	// waiting on that pipe one second after the context fires, whether or not
	// every descendant has exited.
	c.WaitDelay = time.Second

	runErr := c.Run()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "", &PasswordCommandError{Reason: "timed out after 30s"}
	}
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			return "", &PasswordCommandError{Reason: "exited " + exitErr.String()}
		}
		return "", &PasswordCommandError{Reason: "could not be run: " + runErr.Error()}
	}

	// TrimSuffix "\n" alone leaves a trailing "\r" on Windows, where this runs
	// through cmd /C (line 80-82 above) and cmd's own line ending is CRLF: a
	// script that prints the password normally would otherwise yield "pw\r"
	// as the password, which looks correct in every diagnostic and refuses
	// authentication anyway. Trimming "\r" as well removes exactly one
	// trailing line ending, on every platform, and nothing more — a
	// password that itself ends in "\r" was never going to round-trip
	// through a line-oriented pipe in the first place.
	pw := strings.TrimSuffix(strings.TrimSuffix(out.String(), "\n"), "\r")
	if pw == "" {
		return "", &PasswordCommandError{Reason: "printed no password"}
	}
	return pw, nil
}

// injectPassword returns d with password set. d is never operator-typed
// output at this call site — it is a connection string already accepted by
// dsn.Parse or built from a dsn.Ref (rung0's refDSN) — so both connection
// string forms pgconn accepts are handled: the postgres:// / postgresql://
// URL form, by setting the userinfo, and the keyword/value form, by
// appending a password= setting. A string that already carries a password
// never reaches here — ResolvePassword's passwordAvailable check above —
// so there is no "already set" case to preserve.
func injectPassword(d dsn.DSN, password string) dsn.DSN {
	s := string(d)
	if strings.HasPrefix(s, "postgres://") || strings.HasPrefix(s, "postgresql://") {
		if u, err := url.Parse(s); err == nil {
			user := ""
			if u.User != nil {
				user = u.User.Username()
			}
			u.User = url.UserPassword(user, password)
			return dsn.DSN(u.String())
		}
	}
	return dsn.DSN(strings.TrimRight(s, " ") + " password=" + quoteKeywordValue(password))
}

// quoteKeywordValue quotes password for libpq's "key=value key='quoted
// value'" grammar (the form readKeywordValueSettings in internal/dsn/dsn.go
// reads back): backslash and single quote are the only characters that
// grammar treats specially inside a quoted value, and both are escaped with
// a backslash.
func quoteKeywordValue(v string) string {
	v = strings.ReplaceAll(v, `\`, `\\`)
	v = strings.ReplaceAll(v, `'`, `\'`)
	return "'" + v + "'"
}

// passwordCache runs --password-command at most once per discovery walk, no
// matter how many candidates on the ladder have no password from anywhere
// else: a credential helper can prompt, call out to an agent, or simply be
// slow, and rung0's own doc comment ("the candidate carries no password and
// the ordinary password sources ... supply one") promises the command covers
// a discovered candidate, not one invocation per candidate that needs it.
//
// The zero value is not usable, so resolve treats a nil receiver as "no
// command was configured" and returns ("", nil) rather than dereferencing
// it — probe also checks pwCmd != "" before calling resolve, but the two
// checks (a non-empty command string, a non-nil cache) come from different
// copies of Options on some call paths, and disagreeing about a nil cache is
// exactly how that used to panic (Options.withPasswordCache's doc comment).
type passwordCache struct {
	once sync.Once
	pw   string
	err  error
	// w is where a resolution failure is reported, once, the same channel
	// warnDroppedParams already prints to (progressOf(o), ordinarily
	// os.Stderr): internal/event/catalogue.yml has no row for this and is
	// outside the paths that would add one (this package's CLAUDE.md, "the
	// pull and the start print outside the event catalogue"). Nil is silence,
	// which is what a caller that does not care about the warning passes.
	w io.Writer
}

// resolve runs cmdline the first time it is called and returns the cached
// result — success or failure — on every call after. cmdline is assumed
// constant across the calls sharing one cache: it is Options.PasswordCommand,
// set once per run and never varied per candidate.
func (c *passwordCache) resolve(ctx context.Context, cmdline string) (string, error) {
	if c == nil {
		return "", nil
	}
	c.once.Do(func() {
		c.pw, c.err = runPasswordCommand(ctx, cmdline)
		if c.err != nil && c.w != nil {
			msg := "lazyslice: " + c.err.Error() + ": candidates on the discovery ladder will be probed with no password"
			_, _ = io.WriteString(c.w, msg+"\n") //nolint:errcheck // best-effort progress line; nothing to do if it fails
		}
	})
	return c.pw, c.err
}
