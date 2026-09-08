// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package discover

import "os"

// openControllingTerminal opens /dev/tty, which is the controlling terminal of
// the process and not whatever stdin happens to be (ADR-008 §7).
//
// A process with no controlling terminal — a CI runner, cron, a daemon — gets
// ENXIO here, and that error is the whole of the headless test. Nothing about
// os.Stdin is consulted.
func openControllingTerminal() (*os.File, error) {
	return os.OpenFile("/dev/tty", os.O_RDWR, 0)
}
