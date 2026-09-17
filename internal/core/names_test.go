// SPDX-License-Identifier: Apache-2.0

package core

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Liarea/lazyslice/internal/dsn"
	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pg"
)

// unclaimedCanaryError stands in for "a new error type anywhere in the tree"
// (T-0212's own framing): none of asStop's five typed refusals recognise it,
// so it falls all the way through to the exhaustive default at the end of
// asStop. Its Error() quotes a value the way a library's or a third-party
// masker's own error might.
type unclaimedCanaryError struct{ canary string }

func (e unclaimedCanaryError) Error() string {
	return fmt.Sprintf("could not process %q", e.canary)
}

// TestAsStopRedactsAnUnclaimedErrorsMessage is the T-0212 fix round's finding
// 1, tested at the exact line it named (asStop's exhaustive fallback,
// previously `wrap(CodeInternal, exitInternal, err, "%s", err.Error())`):
// that fallback used to copy an unrecognised error's whole Error() text into
// Stop.Message, and cmd/lazyslice's renderSafe prints every *Stop.Error()
// unconditionally by construction, so the exact case T-0212 exists to contain
// reached stderr at both settings of --show-row-values-in-errors. The
// existing renderSafe unit tests call it with a bare error and never through
// this function, so they could not have caught that — this test calls asStop
// itself, the one place a Stop's Message is actually built from a stage
// error, the same way every real refusal that reaches cmd/lazyslice does
// (run.go's asStop call sites).
func TestAsStopRedactsAnUnclaimedErrorsMessage(t *testing.T) {
	const canary = "victim.canary@bigcorp.com"
	err := unclaimedCanaryError{canary: canary}

	got := asStop(err)

	var stop *Stop
	if !errors.As(got, &stop) {
		t.Fatalf("asStop(%T) = %T, want *Stop", err, got)
	}
	if stop.Code != CodeInternal {
		t.Errorf("stop.Code = %s, want %s (nothing claimed this error)", stop.Code, CodeInternal)
	}
	if !stop.Unclaimed() {
		t.Errorf("stop.Unclaimed() = false, want true: cmd/lazyslice's renderSafe (T-0212 fix round, " +
			"finding 1) relies on this to redact even if a future regression here stops calling " +
			"PanicSummary")
	}
	if strings.Contains(stop.Message, canary) {
		t.Errorf("stop.Message = %q, want the canary withheld: an unrecognised error's own words "+
			"must never reach Stop.Message, which cmd/lazyslice's renderSafe prints in full for every "+
			"*core.Stop", stop.Message)
	}
	if strings.Contains(stop.Error(), canary) {
		t.Errorf("stop.Error() = %q, want the canary withheld", stop.Error())
	}
	if !strings.Contains(stop.Message, "unclaimedCanaryError") {
		t.Errorf("stop.Message = %q, want it to name the error's own type (PanicSummary's rule)", stop.Message)
	}
	// The original error is still reachable through Unwrap, for --debug: this
	// test is about what a Stop prints unconditionally, not about withholding
	// the cause from the one flag that asks for a full trail.
	if !errors.Is(stop.Unwrap(), err) {
		t.Errorf("stop.Unwrap() = %v, want the original error, still reachable for --debug", stop.Unwrap())
	}
}

// T-0241 (round-4 red team, docs/reviews/2026-09-15-redteam/round4-still-leaking.json):
// the recommended-role snippet must carry the pg_control_system grant, or the
// gap the round-4 replay measured — the cluster identity degrading to "the
// postmaster start time and nothing else" under the role this statement
// creates — is reintroduced by the very block that is supposed to close it.
func TestReadOnlyRoleStatementGrantsPgControlSystem(t *testing.T) {
	stmt := readOnlyRoleStatement(dsn.Ref{Database: "shop"})
	const want = "GRANT EXECUTE ON FUNCTION pg_control_system() TO lazyslice_ro;"
	if !strings.Contains(stmt, want) {
		t.Errorf("readOnlyRoleStatement = %q, want it to contain %q", stmt, want)
	}
	// The three original grants are unchanged, not replaced: an operator who
	// already ran an earlier version of this statement can still run the
	// whole thing again.
	for _, must := range []string{
		"CREATE ROLE lazyslice_ro LOGIN PASSWORD",
		"GRANT CONNECT ON DATABASE",
		"GRANT USAGE ON SCHEMA public TO lazyslice_ro",
		"GRANT SELECT ON ALL TABLES IN SCHEMA public TO lazyslice_ro",
	} {
		if !strings.Contains(stmt, must) {
			t.Errorf("readOnlyRoleStatement = %q, missing %q", stmt, must)
		}
	}
}

// T-0241. standbySenderReason is CodeSourceStandby's {reason} placeholder:
// the primary a standby streams from when pg_stat_wal_receiver's columns were
// readable, and nothing when they were not — an unreadable sender must not be
// mistaken for "not a standby" anywhere downstream of this function, so it
// degrades to an empty reason rather than an error.
func TestStandbySenderReason(t *testing.T) {
	for _, c := range []struct {
		name string
		st   pg.ReplicaStatus
		want string
	}{
		{"host and port readable", pg.ReplicaStatus{SenderHost: "10.0.0.5", SenderPort: "5432"},
			" (its primary is 10.0.0.5:5432)"},
		{"only the host readable", pg.ReplicaStatus{SenderHost: "10.0.0.5"},
			" (its primary is 10.0.0.5)"},
		{"neither readable", pg.ReplicaStatus{}, ""},
		// A port with no host is not a shape pg_stat_wal_receiver can
		// actually produce (sender_host is set whenever sender_port is), but
		// the function must still not print a bare port with nothing to
		// attach it to.
		{"a port with no host", pg.ReplicaStatus{SenderPort: "5432"}, ""},
	} {
		if got := standbySenderReason(c.st); got != c.want {
			t.Errorf("%s: standbySenderReason(%+v) = %q, want %q", c.name, c.st, got, c.want)
		}
	}
}

// T-0241, round-4 red team's second reproduction ("a target URL taken from a
// compose file that points at prod"): a headless run with no --target has
// nobody to show the standby warning to, and the discovery ladder's
// clusterKey comparison cannot tell a standby's own primary apart from an
// unrelated server — the two ordinarily publish on different ports, which is
// exactly what let that reproduction's ladder pick a target on the standby's
// own primary with no refusal and no same-cluster line. This is a pure-
// function pin over the decision, the way TestSameClusterVerdict
// (internal/pg/cluster_test.go) pins sameClusterVerdict: no database is
// needed to prove the four inputs are wired to the right outcome.
func TestStandbyNoTargetRefusal(t *testing.T) {
	ref := dsn.Ref{Host: "10.0.0.5", Port: 5432, Database: "app"}
	for _, c := range []struct {
		name                         string
		needsTarget, headless, named bool
		refused                      bool
	}{
		{"headless, no target, mode writes: refused", true, true, false, true},
		{"a --target the operator named is unaffected", true, true, true, false},
		{"interactive: the operator sees the warning and can stop it", true, false, false, false},
		{"a mode with no target has nothing to refuse for", false, true, false, false},
	} {
		got := standbyNoTargetRefusal(c.needsTarget, c.headless, c.named, ref)
		if refused := got != nil; refused != c.refused {
			t.Errorf("%s: standbyNoTargetRefusal(...) refused = %v, want %v", c.name, refused, c.refused)
		}
		if !c.refused {
			continue
		}
		if got.Code != CodeSourceStandbyNoTarget {
			t.Errorf("%s: Code = %s, want %s", c.name, got.Code, CodeSourceStandbyNoTarget)
		}
		if got.Exit != exitTarget {
			t.Errorf("%s: Exit = %d, want %d", c.name, got.Exit, exitTarget)
		}
		if got.Args[event.ArgFlag] != "--target" {
			t.Errorf("%s: Args[flag] = %q, want --target", c.name, got.Args[event.ArgFlag])
		}
		if got.Args[event.ArgHost] != ref.Host {
			t.Errorf("%s: Args[host] = %q, want %q", c.name, got.Args[event.ArgHost], ref.Host)
		}
	}
}
