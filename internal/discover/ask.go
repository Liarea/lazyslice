// SPDX-License-Identifier: Apache-2.0

package discover

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// ErrNoTerminal is "there is no controlling terminal to put a question to".
//
// ADR-008 §7 makes that state and --yes one code path: the trigger for the
// headless path is the absence of a controlling terminal — /dev/tty cannot be
// opened, or on Windows the console cannot be — and nothing else. os.Stdin is
// never read, never checked to decide whether to ask, and never consulted at
// all, so `echo y | lazyslice` from a terminal asks the human at the terminal
// and the piped y is still sitting unread in the pipe, while the same command
// from cron takes the headless path and creates nothing.
var ErrNoTerminal = errors.New("discover: no controlling terminal")

// Prompter asks the one blocking question a run may ask.
//
// It is an interface so that a test can answer without a terminal; the only
// implementation outside a test is the controlling-terminal one below.
type Prompter interface {
	// Confirm writes question and reads a yes or no. def is the bracketed
	// letter in the question, which is what a bare Enter selects. Anything
	// other than an empty line, y/yes or n/no (case-insensitively) re-prompts
	// once and then takes def (ADR-008 §7).
	Confirm(question string, def bool) (bool, error)
	// Ask writes question and reads one line of free-form text, trimmed. An
	// empty line (a bare Enter) returns def unchanged — ADR-008 §7's "the
	// bracketed letter is the default and the only thing a bare Enter
	// selects", read for a question whose default is a name rather than a
	// yes or no (ADR-008 §6's Q2, "root table? [customers]"). It does not
	// validate, retry, or interpret the answer beyond that: what a non-empty
	// answer means — a table name, "?" — is entirely the caller's question to
	// ask, the same division Confirm already draws for a yes/no one. The
	// terminal going away mid-question is reported the same way Confirm
	// reports it: def and ErrNoTerminal, the headless answer arriving late.
	Ask(question string, def string) (string, error)
	// Close releases the controlling terminal.
	Close() error
}

// prompt is the controlling-terminal Prompter.
//
// out is where the question is written — os.Stderr, per ADR-008 §7 — and tty is
// where the answer is read from. They are two fields and not one because that
// separation is the rule: a prompt read from os.Stdin would satisfy "ask on a
// terminal" whenever stdin happened to be one, which is the pattern the rule
// exists to reject.
type prompt struct {
	out io.Writer
	tty io.Closer
	in  *bufio.Reader
}

// openPrompter opens the controlling terminal, or reports ErrNoTerminal.
func openPrompter() (Prompter, error) {
	tty, err := openControllingTerminal()
	if err != nil {
		return nil, ErrNoTerminal
	}
	return &prompt{out: os.Stderr, tty: tty, in: bufio.NewReader(tty)}, nil
}

// Confirm implements Prompter.
func (p *prompt) Confirm(question string, def bool) (bool, error) {
	for attempt := range 2 {
		if _, err := fmt.Fprint(p.out, question+" "); err != nil {
			return def, err
		}
		line, err := p.in.ReadString('\n')
		if err != nil && line == "" {
			// The terminal went away mid-question. That is the headless state
			// arriving late, and it takes the headless answer rather than
			// looping on an EOF that will not change.
			return def, ErrNoTerminal
		}
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "":
			return def, nil
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		}
		if attempt == 0 {
			// One re-prompt, then the default: a question that repeats until it
			// is understood is a question that hangs a run (ADR-008 §7).
			continue
		}
	}
	return def, nil
}

// Ask implements Prompter.
func (p *prompt) Ask(question string, def string) (string, error) {
	if _, err := fmt.Fprint(p.out, question+" "); err != nil {
		return def, err
	}
	line, err := p.in.ReadString('\n')
	if err != nil && line == "" {
		// The terminal went away mid-question, exactly Confirm's own case: the
		// headless state arriving late takes the headless answer rather than
		// looping on an EOF that will not change.
		return def, ErrNoTerminal
	}
	if answer := strings.TrimSpace(line); answer != "" {
		return answer, nil
	}
	return def, nil
}

// Close implements Prompter.
func (p *prompt) Close() error { return p.tty.Close() }
