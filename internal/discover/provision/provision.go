// Package provision is the --create-target path (ARCHITECTURE.md section 9
// "Provisioning"). It is the only code in lazyslice that creates or starts a
// container, and it is never called without --create-target or a "yes" to Q1.
//
// The container survives the run: it is the developer's local database from
// then on, and the next run finds it at ladder rung 3 with the marker. lazyslice
// never removes a container; the refusal text for a non-empty target names
// "docker rm -v lazyslice-target-<project>" as the command that would.
//
// Scaffold status: no-op. Provision returns pipeline.ErrNotImplemented.
package provision

import (
	"context"
	"fmt"
	"time"

	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pipeline"
)

type provisioner struct{}

// New returns the container provisioner.
func New() pipeline.Provisioner { return provisioner{} }

var _ pipeline.Provisioner = provisioner{}

func (provisioner) Provision(_ context.Context, _ string, _ int, sink event.Sink) (pipeline.Candidate, error) {
	sink.Send(event.Event{
		At:    time.Now(),
		Stage: event.Discover,
		Kind:  event.Info,
		Code:  event.CodeNotImplemented,
		Args:  event.Args{event.ArgStage: "provision"},
	})
	return pipeline.Candidate{}, fmt.Errorf("provision: %w", pipeline.ErrNotImplemented)
}
