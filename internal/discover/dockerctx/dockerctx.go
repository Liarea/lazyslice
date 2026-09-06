// SPDX-License-Identifier: Apache-2.0

// Package dockerctx resolves the Docker endpoint lazyslice talks to: the
// --docker-host flag, then DOCKER_HOST, then the active Docker context, then
// the default sockets.
//
// It exists as its own package because "which Docker am I talking to" is the
// question that makes container discovery either work or silently find nothing
// on a machine running Colima, Rancher Desktop or a rootless daemon, and that
// question should be answerable in isolation.
//
// Scaffold status: no-op. Resolve returns ErrNotImplemented.
package dockerctx

import (
	"context"
	"errors"

	"github.com/moby/moby/client"
)

// ErrNotImplemented is returned by every placeholder in the scaffold.
var ErrNotImplemented = errors.New("dockerctx: not implemented")

// Endpoint is a resolved Docker endpoint and how it was found, so that the
// candidate list can say "no containers found (docker: colima context)" rather
// than nothing at all.
type Endpoint struct {
	Host string
	// From is "flag", "env", "context" or "default".
	From string
}

// Resolve picks the Docker endpoint. hostFlag is the value of --docker-host,
// empty when the flag was not given.
//
// Scaffold status: no-op.
func Resolve(_ context.Context, _ string) (Endpoint, error) {
	return Endpoint{}, ErrNotImplemented
}

// Dial opens a Docker API client against a resolved endpoint. The client is
// used read-only by discovery; only internal/discover/provision creates or
// starts anything.
//
// Scaffold status: no-op.
func Dial(_ Endpoint) (*client.Client, error) {
	return nil, ErrNotImplemented
}
