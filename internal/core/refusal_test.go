// SPDX-License-Identifier: Apache-2.0

package core

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/Liarea/lazyslice/internal/event"
)

// The ladder's refusal reaches the sink once, not twice.
//
// internal/discover renders its own refusal from its own catalogue and sends the
// Error event for it, so the Stop refusalStop builds is marked sent and
// run.report returns without sending a second one. Without that guard an
// argument-free run prints the same line twice — one refusal, two ✗ lines —
// and an exit-code assertion cannot see it, because both lines carry the same
// code and the same exit (T-0060).
func TestALadderRefusalIsSentOnceAndNotAgainByReport(t *testing.T) {
	// Every rung reads the environment, so it is emptied first: the developer's
	// own $DATABASE_URL or libpq settings would otherwise decide whether this
	// run refuses at all. The Docker endpoint is set on the request as well as
	// here, and is not local, so rung 3 makes no socket call (ADR-008 section 3).
	for _, name := range []string{
		"DATABASE_URL", "POSTGRES_URL", "PG_URL", "DB_URL",
		"PGSERVICE", "PGHOST", "PGHOSTADDR", "PGDATABASE", "PGPORT", "PGUSER",
		"PGPASSWORD", "PGPASSFILE", "DOCKER_CONTEXT", "DOCKER_CONFIG",
	} {
		t.Setenv(name, "")
	}
	t.Setenv("DOCKER_HOST", "tcp://staging.example:2375")

	dir := t.TempDir()
	var got []event.Event
	sink := event.SinkFunc(func(e event.Event) { got = append(got, e) })

	req := Request{
		Mode:       ModeRun,
		Workdir:    dir,
		DockerHost: "tcp://staging.example:2375",
		ConfigPath: filepath.Join(dir, "lazyslice.yml"),
		SecretFile: filepath.Join(dir, "lazyslice.secret"),
		NoConfig:   true,
		Yes:        true,
	}

	// Run returns after its event channel has drained, so every event the run
	// sent is in got by now.
	_, err := Run(t.Context(), req, sink)

	var stop *Stop
	if !errors.As(err, &stop) {
		t.Fatalf("Run = %v, want the ladder's refusal", err)
	}
	if stop.Code != CodeSourceNone || stop.Exit != exitNoSource {
		t.Fatalf("stop = %s/exit %d, want %s/exit %d",
			stop.Code, stop.Exit, CodeSourceNone, exitNoSource)
	}

	n := 0
	for _, e := range got {
		if e.Kind == event.Error && e.Code == CodeSourceNone {
			n++
		}
	}
	if n != 1 {
		t.Errorf("%s reached the sink %d time(s), want once: one refusal is one line", CodeSourceNone, n)
	}
}
