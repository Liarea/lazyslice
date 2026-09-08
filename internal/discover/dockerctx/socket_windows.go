// SPDX-License-Identifier: Apache-2.0

//go:build windows

package dockerctx

// defaultSocket is the platform default, as a build-tagged constant (ADR-008
// §3).
func defaultSocket() string { return "npipe:////./pipe/docker_engine" }
