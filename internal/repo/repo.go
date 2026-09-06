// SPDX-License-Identifier: Apache-2.0

// Package repo protects the files lazyslice writes that must never be committed:
// ./lazyslice.secret and any mapping_file named by the yml (THREAT_MODEL.md T6,
// ARCHITECTURE.md section 9 "The repository").
//
// The order is fixed and every branch prints what it did:
//
//  1. No repository: the secret is written and the header says it is not
//     protected by .gitignore.
//  2. Repository with a writable .gitignore: the missing entries are appended
//     and the addition is printed.
//  3. Repository whose .gitignore is absent or unwritable: the secret file is
//     not created at all, the run uses an ephemeral key and says so, and
//     --require-key makes that exit 5.
//  4. Tracked check: with git on PATH, exit 0 from ls-files --error-unmatch
//     means the file is tracked, which is exit 2 naming "git rm --cached".
//     Without git the check is skipped and the header says so.
//
// git is one of the binary's two subprocesses; --password-command is the other.
// It receives a path and never a row value.
//
// Scaffold status: no-op. Every function returns ErrNotImplemented.
package repo

import "errors"

// ErrNotImplemented is returned by every placeholder in the scaffold.
var ErrNotImplemented = errors.New("repo: not implemented")

// State is what Protect found, so the caller can print it and decide whether the
// masking key may be written to disk.
type State struct {
	// Root is the nearest ancestor of the working directory containing .git, ""
	// when there is no repository.
	Root string
	// GitignoreWritable reports whether the entries could be appended.
	GitignoreWritable bool
	// Added lists the entries this run appended.
	Added []string
	// Tracked lists paths git says are tracked, which is exit 2.
	Tracked []string
	// GitFound reports whether git was on PATH; when false the tracked check was
	// skipped and the header says so.
	GitFound bool
}

// Root finds the repository root: the nearest ancestor of dir containing .git.
//
// Scaffold status: no-op.
func Root(_ string) (string, error) { return "", ErrNotImplemented }

// Protect runs the four steps above for the secret file and every mapping file,
// and reports what it did.
//
// Scaffold status: no-op.
func Protect(_ string, _ []string) (State, error) { return State{}, ErrNotImplemented }
