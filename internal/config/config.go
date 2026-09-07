// SPDX-License-Identifier: Apache-2.0

// Package config resolves the machine-local state directory (ADR-004).
//
// This is not lazyslice.yml, which is committed and holds decisions. This is the
// per-machine directory holding what must not be committed and must not be lost
// between runs: the password of a container lazyslice provisioned, and nothing
// else in v1. There is no keyring (ADR-004).
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// dirName is the one directory name every platform branch appends. It is a
// constant so that a second spelling cannot appear on one platform and leave a
// provisioned container's password stranded where the next run does not look.
const dirName = "lazyslice"

// dirPerm is the mode StateDir creates with and requires. Everything under this
// directory could be a credential, so nothing here is ever group- or
// world-readable (internal/config/CLAUDE.md).
const dirPerm os.FileMode = 0o700

// StateDir returns the machine-local state directory, creating it with 0o700 if
// it does not exist: $XDG_STATE_HOME/lazyslice, or the platform equivalent.
//
// The resolution is:
//
//   - $XDG_STATE_HOME/lazyslice when that variable names an absolute path. It
//     is honoured on every platform, because a developer who sets it has said
//     where their state goes and a tool that ignored it would write somewhere
//     they do not back up.
//   - $HOME/.local/state/lazyslice on anything Unix-like, which is the XDG
//     default.
//   - os.UserConfigDir()/lazyslice on macOS and Windows, which is
//     ~/Library/Application Support and %AppData% — the platform equivalents,
//     and the directories Go itself resolves rather than a spelling of ours.
//
// A directory that exists with looser permissions is tightened rather than
// accepted: the caller is about to write a password into it.
func StateDir() (string, error) {
	dir, err := stateRoot()
	if err != nil {
		return "", err
	}
	dir = filepath.Join(dir, dirName)

	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return "", fmt.Errorf("config: creating the state directory: %w", err)
	}
	// MkdirAll leaves an existing directory's mode alone, and umask can widen
	// the one it creates. Windows has no Unix mode bits and Chmod there is a
	// near no-op, so the tightening is skipped rather than failed on.
	if runtime.GOOS != "windows" {
		if err := os.Chmod(dir, dirPerm); err != nil {
			return "", fmt.Errorf("config: tightening the state directory: %w", err)
		}
	}
	return dir, nil
}

// stateRoot is the directory StateDir appends "lazyslice" to.
func stateRoot() (string, error) {
	if xdg := os.Getenv("XDG_STATE_HOME"); filepath.IsAbs(xdg) {
		return xdg, nil
	}
	if runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("config: resolving the home directory: %w", err)
		}
		return filepath.Join(home, ".local", "state"), nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("config: resolving the state directory: %w", err)
	}
	return dir, nil
}
