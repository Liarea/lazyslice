// SPDX-License-Identifier: Apache-2.0

// Package discover walks the discovery ladder: lazyslice.yml, environment
// variables and .env files, libpq settings, running Postgres containers,
// exited containers, compose service names (ARCHITECTURE.md section 9).
//
// Discovery is why the first run asks at most one question. Source is never a
// question: it is the most-local reachable candidate with the most tables that
// is not the target, and no source at all is exit 3 with the ladder printed and
// a command to run.
//
// The ladder has a 2 s listing budget and a 1 s per-candidate dial. Inside the
// dial it runs three statements: version, the pg_class count and hint, and
// to_regclass('lazyslice_meta'). It never probes emptiness table by table;
// that is the gate's job, in internal/pg, after discovery.
//
// Scaffold status: no-op. Discover emits stage.not_implemented and returns
// pipeline.ErrNotImplemented.
package discover

import (
	"context"
	"fmt"
	"time"

	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pipeline"
)

type ladder struct{}

// New returns the discovery ladder.
func New() pipeline.Discoverer { return ladder{} }

var _ pipeline.Discoverer = ladder{}

func (ladder) Discover(_ context.Context, _ string, sink event.Sink) ([]pipeline.Candidate, error) {
	sink.Send(event.Event{
		At:    time.Now(),
		Stage: event.Discover,
		Kind:  event.Info,
		Code:  event.CodeNotImplemented,
		Args:  event.Args{event.ArgStage: event.Discover.String()},
	})
	return nil, fmt.Errorf("discover: %w", pipeline.ErrNotImplemented)
}
