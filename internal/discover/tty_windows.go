// SPDX-License-Identifier: Apache-2.0

//go:build windows

package discover

import "os"

// openControllingTerminal opens the console this process is attached to
// (ADR-008 §7). CONIN$ is Windows' equivalent of /dev/tty for reading: a
// process with no console cannot open it, which is the headless test.
func openControllingTerminal() (*os.File, error) {
	return os.OpenFile("CONIN$", os.O_RDWR, 0)
}
