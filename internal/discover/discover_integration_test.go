// SPDX-License-Identifier: Apache-2.0

//go:build integration

package discover

import (
	"strings"
	"testing"

	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/testutil"
)

// Rung 3 against a real container: the ladder finds a running Postgres through
// the Docker endpoint ADR-008 §3 resolves, dials it inside the 1 s budget, and
// answers the three statements ARCHITECTURE.md §9 allows it.
//
// It is an integration test because every claim in it is a claim about a real
// daemon and a real server: a unit test with a stubbed client would prove that
// the code compiles against the types it was written against and nothing else.
func TestDiscoversARunningContainer(t *testing.T) {
	ctx := t.Context()
	testutil.SkipWithoutDocker(ctx, t)

	connURL := testutil.Postgres(ctx, t, "")
	u, err := testutil.URL(connURL)
	if err != nil {
		t.Fatalf("%v", err)
	}
	port := u.Port()

	// The developer's own environment must not decide the answer, but the
	// Docker endpoint has to stay the real one, so DOCKER_HOST is left alone
	// here and only the rung 1 and rung 2 variables are cleared.
	quietRungs(t)

	var events []event.Event
	sink := event.SinkFunc(func(e event.Event) { events = append(events, e) })

	cands, err := New().Discover(ctx, t.TempDir(), sink)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	got, ok := candidateOnPort(cands, port)
	if !ok {
		t.Fatalf("the container published on port %s is not on the ladder: %+v", port, cands)
	}
	if got.Provenance != pipeline.FromContainer {
		t.Errorf("provenance = %v, want rung 3", got.Provenance)
	}
	if !got.Local {
		t.Error("a container on this machine's daemon must be Local (ARCHITECTURE.md section 2)")
	}
	if !got.Reachable {
		t.Fatalf("the container did not answer inside the dial: %s", got.ConnectErr)
	}
	if got.Version < 14 {
		t.Errorf("server major = %d, want the container's own (ADR-003 supports 14 to 18)", got.Version)
	}
	if got.Marked {
		t.Error("a fresh container carries no lazyslice_meta")
	}
	if !got.EmptyHint {
		t.Error("a fresh container has no user table with relpages > 0, so the hint must read probably empty")
	}
	if got.Empty != nil {
		t.Error("discovery filled Empty; only Target.Gate may, because emptiness is a per-table probe")
	}
	if got.Label == "" {
		t.Error("the candidate has no display name, so the candidate list cannot say which container it is")
	}
}

// The same container reached twice — once at rung 1 through $DATABASE_URL and
// once at rung 3 through its published binding — is one candidate, and the
// lower-numbered rung wins the printed provenance (ADR-008 §4).
func TestContainerAndEnvVarCollapseToOneCandidate(t *testing.T) {
	ctx := t.Context()
	testutil.SkipWithoutDocker(ctx, t)

	connURL := testutil.Postgres(ctx, t, "")
	u, err := testutil.URL(connURL)
	if err != nil {
		t.Fatalf("%v", err)
	}
	quietRungs(t)
	t.Setenv("DATABASE_URL", connURL)

	cands, err := New().Discover(ctx, t.TempDir(), event.Discard)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	var matches int
	for _, c := range cands {
		if c.Ref.Port == portOf(u.Port()) {
			matches++
			if c.Provenance != pipeline.FromEnvVar {
				t.Errorf("provenance = %v, want the lower-numbered rung to win", c.Provenance)
			}
			if !c.Local {
				t.Error("the collapse dropped the locality the container rung established")
			}
			if !c.Reachable {
				t.Errorf("the collapsed candidate did not answer: %s", c.ConnectErr)
			}
		}
	}
	if matches != 1 {
		t.Errorf("the container appears %d times on the ladder, want once", matches)
	}
}

// quietRungs clears the rung 1 and rung 2 variables. DOCKER_HOST and the Docker
// context are deliberately left alone: this file's subject is the real daemon.
func quietRungs(t *testing.T) {
	t.Helper()
	for _, n := range []string{
		"DATABASE_URL", "POSTGRES_URL", "PG_URL", "DB_URL",
		"PGSERVICE", "PGHOST", "PGHOSTADDR", "PGDATABASE", "PGPORT", "PGUSER",
		"PGPASSWORD", "PGPASSFILE",
	} {
		t.Setenv(n, "")
	}
}

func candidateOnPort(cands []pipeline.Candidate, port string) (pipeline.Candidate, bool) {
	for _, c := range cands {
		if c.Ref.Port == portOf(port) {
			return c, true
		}
	}
	return pipeline.Candidate{}, false
}

func portOf(s string) int {
	n := 0
	for _, r := range strings.TrimSpace(s) {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}
