// SPDX-License-Identifier: Apache-2.0

//go:build windows

package core

import "io/fs"

// hardLinkCount is unimplemented on windows: fs.FileInfo carries no link
// count there without a separate GetFileInformationByHandle call this build
// does not make, so the R2-15 check is skipped on this platform rather than
// refusing on a count it never read. THREAT_MODEL.md T6 records the gap.
func hardLinkCount(fs.FileInfo) (nlink uint64, ok bool) {
	return 0, false
}
