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

// permissiveMode never reports a permissive file on windows. Go synthesises
// the permission bits there from the read-only attribute alone, so every
// writable file reads as 0666: judged by those bits, every secret file this
// tool itself wrote 0600 was refused on its second run with advice to chmod,
// which means nothing on this platform (CI's windows leg, red from 2026-09-16
// on exactly that). Who can read the file is an ACL question this build does
// not ask; THREAT_MODEL.md T6 records that gap beside the link count's.
func permissiveMode(info fs.FileInfo) (perm fs.FileMode, permissive bool) {
	return info.Mode().Perm(), false
}
