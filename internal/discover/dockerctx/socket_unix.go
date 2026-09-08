// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package dockerctx

// defaultSocket is the platform default, as a build-tagged constant (ADR-008
// §3). There is deliberately no list of guessed vendor socket paths: a socket
// path we have not verified is a fact invented in the binary, and --docker-host
// is the answer for a daemon the context store does not register.
func defaultSocket() string { return "unix:///var/run/docker.sock" }
