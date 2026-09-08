// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"strings"

	"github.com/Liarea/lazyslice/internal/tui"
)

// generateKeybindings renders docs/KEYBINDINGS.md from internal/tui.Bindings(),
// the same table TestEveryTUIActionHasFlag in cmd/lazyslice walks (ADR-002
// enforcement mechanism 1). Order is Bindings()'s own — actions before
// navigation, keyMap.All()'s order — and this file leaves it alone rather
// than re-sorting it.
func generateKeybindings() (string, error) {
	bindings := tui.Bindings()
	if len(bindings) == 0 {
		return "", fmt.Errorf("tui.Bindings() returned no bindings")
	}

	var b strings.Builder
	b.WriteString(generatedHeader("Keybindings", "`internal/tui.Bindings()`"))
	b.WriteString("The `--tui` screens' keybindings (ADR-002). A binding with no flag moves " +
		"the cursor, switches screen, or leaves the TUI, and changes no field of the request " +
		"being built.\n\n")
	b.WriteString("| Key | Action | Flag |\n")
	b.WriteString("|---|---|---|\n")
	for _, bind := range bindings {
		flag := "(navigation)"
		if !bind.Nav {
			flag = "`--" + bind.Flag + "`"
		}
		fmt.Fprintf(&b, "| `%s` | %s | %s |\n", mdEscape(bind.Key()), mdEscape(bind.Desc()), flag)
	}
	return b.String(), nil
}
