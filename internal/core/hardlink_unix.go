// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package core

import (
	"io/fs"
	"reflect"
	"syscall"
)

// hardLinkCount reports the number of names a regular file has on disk, the
// R2-15 check: a masking key hard-linked into a cloud-synced folder is
// reachable under a name the repository rule never saw, with no path-based
// check able to tell. ok is false when the platform's FileInfo carries no
// *syscall.Stat_t (nothing this build runs on today), in which case the
// caller skips the check rather than refusing on a count it cannot trust.
func hardLinkCount(info fs.FileInfo) (nlink uint64, ok bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	// Stat_t.Nlink is uint16 on darwin, uint32 on some linux ports and uint64
	// on linux/amd64. A plain uint64(...) conversion is required on the first
	// two and is an unconvert finding on the third, and a //nolint for it is
	// an unused-directive finding everywhere else (nolintlint, allow-unused:
	// false) -- CI's lint job was red on exactly that from 2026-09-16. Reading
	// the field's value by kind is the one spelling every platform accepts;
	// it runs once per run.
	return reflect.ValueOf(st.Nlink).Uint(), true
}

// permissiveMode reports the permission bits of a secret file when group or
// other can reach it. The bits mean what they say on every platform this file
// builds for.
func permissiveMode(info fs.FileInfo) (perm fs.FileMode, permissive bool) {
	perm = info.Mode().Perm()
	return perm, perm&0o077 != 0
}
