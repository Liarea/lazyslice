// SPDX-License-Identifier: Apache-2.0

// Package config resolves the machine-local state directory (ADR-004).
//
// This is not lazyslice.yml, which is committed and holds decisions. This is the
// per-machine directory holding what must not be committed and must not be lost
// between runs: the password of a container lazyslice provisioned, and nothing
// else in v1. There is no keyring (ADR-004).
//
// Scaffold status: no-op. Every function returns ErrNotImplemented.
package config

import "errors"

// ErrNotImplemented is returned by every placeholder in the scaffold.
var ErrNotImplemented = errors.New("config: not implemented")

// StateDir returns the machine-local state directory, creating it with 0o700 if
// it does not exist: $XDG_STATE_HOME/lazyslice, or the platform equivalent.
//
// Scaffold status: no-op.
func StateDir() (string, error) { return "", ErrNotImplemented }
