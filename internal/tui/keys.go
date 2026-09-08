// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"slices"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/table"
)

// screen is one of the two screens ADR-002 admits: the classification table
// with its reasons, and the plan table. There is no third one, and a capability
// that would need one is a flag first.
type screen int

// The two screens.
const (
	// screenReasons is the classifier's decisions, one row per column.
	screenReasons screen = iota
	// screenPlan is the plan: the tables, the caps in force and the estimate.
	screenPlan
)

// String names the screen, for the header and for a test failure.
func (s screen) String() string {
	switch s {
	case screenReasons:
		return "reasons"
	case screenPlan:
		return "plan"
	default:
		return "unknown"
	}
}

// other is the screen tab switches to.
func (s screen) other() screen {
	if s == screenReasons {
		return screenPlan
	}
	return screenReasons
}

// Binding is one keybinding together with the CLI flag that reaches the same
// action.
//
// Flag is the enforcement point for root CLAUDE.md's hardest rule for this
// package: every TUI action must be reachable by a CLI flag first.
// TestEveryTUIActionHasFlag in cmd/lazyslice walks Bindings() and fails on any
// binding that is not Nav and whose Flag is not a flag registered on the real
// command tree (ADR-002, enforcement mechanism 1). A screen therefore cannot
// grow a choice the command line cannot make: the flag has to exist in
// ARCHITECTURE.md section 8 and in cmd/lazyslice first, and the binding second.
type Binding struct {
	// Bind is the framework's own binding, so that key.Matches decides what was
	// pressed and the help text has one home.
	Bind key.Binding
	// Flag is the long flag name without its dashes. It is empty exactly when
	// Nav is true.
	Flag string
	// Nav marks a binding that moves the cursor, switches screen or leaves the
	// TUI. It changes no field of the core.Request being built, which is why it
	// needs no flag; TestNavigationBuildsNoRequest holds the other half of that
	// claim by pressing every one of them and comparing the request.
	Nav bool
	// screens are the screens whose footer prints it.
	screens []screen
}

// Key is what the operator presses, as the footer prints it.
func (b Binding) Key() string { return b.Bind.Help().Key }

// Desc is what the binding does, as the footer prints it.
func (b Binding) Desc() string { return b.Bind.Help().Desc }

// shows reports whether screen s prints this binding in its footer.
func (b Binding) shows(s screen) bool { return slices.Contains(b.screens, s) }

// keyMap is every binding the two screens have.
//
// It is a struct of named fields rather than a slice so that Update matches on
// the binding it means, and All() is the slice the footer and the flag test
// walk. Nothing rebinds a key in v1: there is no key configuration file, and
// ADR-002 records that Cancel never gains one.
type keyMap struct {
	// Navigation, on both screens.
	Up, Down, PageUp, PageDown, Top, Bottom Binding
	Switch, Help, Accept, Cancel, Quit      Binding

	// The reasons screen's actions.
	Unmask, Strict Binding

	// The plan screen's actions.
	Root, Take, Cap, Depth, Skip Binding
}

// defaultKeyMap returns the v1 bindings.
func defaultKeyMap() keyMap {
	both := []screen{screenReasons, screenPlan}
	reasons := []screen{screenReasons}
	plan := []screen{screenPlan}

	nav := func(keys []string, shown, desc string, screens []screen) Binding {
		return Binding{
			Bind:    key.NewBinding(key.WithKeys(keys...), key.WithHelp(shown, desc)),
			Nav:     true,
			screens: screens,
		}
	}
	act := func(keys []string, shown, desc, flag string, screens []screen) Binding {
		return Binding{
			Bind:    key.NewBinding(key.WithKeys(keys...), key.WithHelp(shown, desc)),
			Flag:    flag,
			screens: screens,
		}
	}

	return keyMap{
		Up:       nav([]string{"up", "k"}, "↑/k", "up a row", both),
		Down:     nav([]string{"down", "j"}, "↓/j", "down a row", both),
		PageUp:   nav([]string{"pgup", "b"}, "b/pgup", "up a page", both),
		PageDown: nav([]string{"pgdown", "f"}, "f/pgdn", "down a page", both),
		Top:      nav([]string{"home", "g"}, "g", "first row", both),
		Bottom:   nav([]string{"end", "G"}, "G", "last row", both),
		Switch:   nav([]string{"tab"}, "tab", "the other screen", both),
		Help:     nav([]string{"?"}, "?", "every key and the flag it is", both),
		Accept:   nav([]string{"enter"}, "enter", "leave and run", both),
		// Cancel is not rebindable (ADR-002). Its keys are the two a terminal
		// user presses to get out of anything, and they never reach a text
		// input: leaving must not depend on the screen being in a state that
		// can answer. Quit is separate because "q" is a character a reason
		// needs while the reason prompt is open, and Cancel is not.
		Cancel: nav([]string{"esc", "ctrl+c"}, "esc", "leave, or discard this answer", both),
		Quit:   nav([]string{"q"}, "q", "leave without running", both),

		Unmask: act([]string{"u"}, "u", "opt out of masking this column, with a reason",
			"unmask", reasons),
		Strict: act([]string{"S"}, "S", "refuse a column the config has never seen",
			"strict-schema", reasons),

		Root:  act([]string{"R"}, "R", "make this table the root", "root", plan),
		Take:  act([]string{"n"}, "n", "root rows", "take", plan),
		Cap:   act([]string{"c"}, "c", "children per parent key on this table", "cap", plan),
		Depth: act([]string{"d"}, "d", "child depth", "depth", plan),
		Skip:  act([]string{"x"}, "x", "drop this table to schema only", "skip-table", plan),
	}
}

// All returns every binding, actions before navigation, which is the order the
// footer prints them in: the thing a screen can change comes before the thing
// that only moves the cursor.
func (k keyMap) All() []Binding {
	return []Binding{
		k.Unmask, k.Strict,
		k.Root, k.Take, k.Cap, k.Depth, k.Skip,
		k.Up, k.Down, k.PageUp, k.PageDown, k.Top, k.Bottom,
		k.Switch, k.Help, k.Accept, k.Cancel, k.Quit,
	}
}

// actions returns the bindings that change the core.Request, in All order.
func (k keyMap) actions() []Binding {
	var out []Binding
	for _, b := range k.All() {
		if !b.Nav {
			out = append(out, b)
		}
	}
	return out
}

// Bindings is every keybinding the TUI has, for the test in cmd/lazyslice that
// fails on one whose action has no CLI flag (ADR-002).
func Bindings() []Binding { return defaultKeyMap().All() }

// tableKeyMap is the bubbles table's own navigation, narrowed to the keys this
// package's footer advertises.
//
// The half-page bindings are dropped rather than left at their defaults because
// they are "u" and "d", which are this package's opt-out and depth actions: a
// key that both opted a column out of masking and scrolled half a page would be
// two actions on one press, and the one that matters is the masking one.
func tableKeyMap(k keyMap) table.KeyMap {
	return table.KeyMap{
		LineUp:       k.Up.Bind,
		LineDown:     k.Down.Bind,
		PageUp:       k.PageUp.Bind,
		PageDown:     k.PageDown.Bind,
		HalfPageUp:   key.NewBinding(key.WithDisabled()),
		HalfPageDown: key.NewBinding(key.WithDisabled()),
		GotoTop:      k.Top.Bind,
		GotoBottom:   k.Bottom.Bind,
	}
}
