// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// inputLimit bounds a typed answer. A reason is a sentence and a count is three
// digits; a field with no bound is a field a paste can fill with a screen of
// text that then has to be rendered.
const inputLimit = 200

// lineInput is the one-line field the prompt asks a reason or a count with.
//
// It is written here rather than taken from charm.land/bubbles/v2/textinput
// because that component imports github.com/atotto/clipboard, a module this
// repository does not carry: a text field is not a reason to add a dependency
// that reads and writes the system clipboard while the process holds two
// database credentials and a masking key (THREAT_MODEL.md T5). What the two
// prompts need is a rune buffer, a cursor, and backspace.
type lineInput struct {
	value []rune
	pos   int
}

// newLineInput returns a field holding initial, with the cursor at the end.
func newLineInput(initial string) lineInput {
	in := lineInput{}
	in.SetValue(initial)
	return in
}

// Value is what has been typed.
func (in lineInput) Value() string { return string(in.value) }

// SetValue replaces the contents and puts the cursor at the end.
func (in *lineInput) SetValue(s string) {
	runes := []rune(s)
	if len(runes) > inputLimit {
		runes = runes[:inputLimit]
	}
	in.value = runes
	in.pos = len(runes)
}

// Update applies one key press to the field.
func (in lineInput) Update(msg tea.KeyPressMsg) lineInput {
	switch msg.Code {
	case tea.KeyBackspace:
		if in.pos > 0 {
			in.value = deleteRune(in.value, in.pos-1)
			in.pos--
		}
		return in
	case tea.KeyDelete:
		if in.pos < len(in.value) {
			in.value = deleteRune(in.value, in.pos)
		}
		return in
	case tea.KeyLeft:
		in.pos = max(in.pos-1, 0)
		return in
	case tea.KeyRight:
		in.pos = min(in.pos+1, len(in.value))
		return in
	case tea.KeyHome:
		in.pos = 0
		return in
	case tea.KeyEnd:
		in.pos = len(in.value)
		return in
	default:
	}

	// Only a key that produced printable text types, and only when no modifier
	// beyond the ones that produce a character is held: that is what keeps
	// ctrl+a from inserting an "a" while leaving shift and caps lock free to
	// type a capital.
	const typing = tea.ModShift | tea.ModCapsLock | tea.ModNumLock
	if msg.Text == "" || msg.Mod&^typing != 0 {
		return in
	}
	for _, r := range msg.Text {
		if len(in.value) >= inputLimit {
			break
		}
		in.value = insertRune(in.value, in.pos, r)
		in.pos++
	}
	return in
}

// View renders the field with a caret at the cursor.
func (in lineInput) View() string {
	var b strings.Builder
	b.WriteString("> ")
	b.WriteString(string(in.value[:in.pos]))
	b.WriteString("▏")
	b.WriteString(string(in.value[in.pos:]))
	return b.String()
}

func insertRune(in []rune, at int, r rune) []rune {
	out := make([]rune, 0, len(in)+1)
	out = append(out, in[:at]...)
	out = append(out, r)
	return append(out, in[at:]...)
}

func deleteRune(in []rune, at int) []rune {
	out := make([]rune, 0, len(in)-1)
	out = append(out, in[:at]...)
	return append(out, in[at+1:]...)
}
