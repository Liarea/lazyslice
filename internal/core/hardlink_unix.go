// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package core

import (
	"io/fs"
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
	return uint64(st.Nlink), true
}
