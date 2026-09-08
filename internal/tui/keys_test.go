// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// keyMsg builds the tea.KeyPressMsg for a key named the way a key.Binding names
// it. It is the tests' whole vocabulary for driving the model, and
// TestKeyNamesMatchTheFramework holds it against the framework's own String().
func keyMsg(name string) tea.KeyPressMsg {
	special := map[string]tea.KeyPressMsg{
		"up":     {Code: tea.KeyUp},
		"down":   {Code: tea.KeyDown},
		"left":   {Code: tea.KeyLeft},
		"right":  {Code: tea.KeyRight},
		"pgup":   {Code: tea.KeyPgUp},
		"pgdown": {Code: tea.KeyPgDown},
		"home":   {Code: tea.KeyHome},
		"end":    {Code: tea.KeyEnd},
		"tab":    {Code: tea.KeyTab},
		"enter":  {Code: tea.KeyEnter},
		"esc":    {Code: tea.KeyEsc},
		"space":  {Code: tea.KeySpace, Text: " "},
		"ctrl+c": {Code: 'c', Mod: tea.ModCtrl},
	}
	if msg, ok := special[name]; ok {
		return msg
	}
	return tea.KeyPressMsg{Code: []rune(name)[0], Text: name}
}

// TestKeyNamesMatchTheFramework is the test that keeps every other test honest:
// key.Matches compares a binding's key strings against Key.String(), so a
// binding whose key is spelled in a way the framework never produces would
// simply never fire, and every model test would pass by pressing nothing.
func TestKeyNamesMatchTheFramework(t *testing.T) {
	for _, b := range Bindings() {
		for _, name := range b.Bind.Keys() {
			if got := keyMsg(name).String(); got != name {
				t.Errorf("binding key %q is spelled %q by the framework", name, got)
			}
		}
	}
}

// TestEveryBindingShowsItsKeyAndWhatItDoes is ADR-002's "every action shows its
// keybinding": a binding with no help text has nothing to print in the footer,
// so the action would be reachable and invisible.
func TestEveryBindingShowsItsKeyAndWhatItDoes(t *testing.T) {
	for _, b := range Bindings() {
		switch {
		case len(b.Bind.Keys()) == 0:
			t.Errorf("binding %q has no key", b.Desc())
		case b.Key() == "":
			t.Errorf("binding for %v has no key text for the footer", b.Bind.Keys())
		case b.Desc() == "":
			t.Errorf("binding %v has no description", b.Bind.Keys())
		case len(b.screens) == 0:
			t.Errorf("binding %q is on no screen, so no footer prints it", b.Desc())
		}
	}
}

// TestNavigationBindingsNameNoFlag is the other half of the flag rule. A
// navigation binding must not carry a flag, because a flag on it would make
// TestEveryTUIActionHasFlag pass for something that changes nothing, and an
// action must carry one.
func TestNavigationBindingsNameNoFlag(t *testing.T) {
	for _, b := range Bindings() {
		if b.Nav && b.Flag != "" {
			t.Errorf("navigation binding %q names the flag --%s", b.Desc(), b.Flag)
		}
		if !b.Nav && b.Flag == "" {
			t.Errorf("action %q names no flag", b.Desc())
		}
	}
}

// TestTheTableDoesNotStealAnActionKey holds the one collision the bubbles table
// brings with it: its default half-page bindings are "u" and "d", which are this
// package's opt-out and depth actions.
func TestTheTableDoesNotStealAnActionKey(t *testing.T) {
	k := defaultKeyMap()
	tk := tableKeyMap(k)

	taken := map[string]string{}
	for _, b := range k.actions() {
		for _, name := range b.Bind.Keys() {
			taken[name] = b.Desc()
		}
	}
	for _, nav := range []struct {
		what string
		keys []string
	}{
		{"line up", tk.LineUp.Keys()},
		{"line down", tk.LineDown.Keys()},
		{"page up", tk.PageUp.Keys()},
		{"page down", tk.PageDown.Keys()},
		{"half page up", tk.HalfPageUp.Keys()},
		{"half page down", tk.HalfPageDown.Keys()},
		{"top", tk.GotoTop.Keys()},
		{"bottom", tk.GotoBottom.Keys()},
	} {
		for _, name := range nav.keys {
			if action, clash := taken[name]; clash {
				t.Errorf("the table's %q binding takes %q, which is the action %q",
					nav.what, name, action)
			}
		}
	}
}
