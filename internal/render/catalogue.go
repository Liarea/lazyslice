// SPDX-License-Identifier: Apache-2.0

package render

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/goccy/go-yaml"

	"github.com/Liarea/lazyslice/internal/event"
)

// The code catalogue (ARCHITECTURE.md §7) is the one place a message a user
// reads is written, and the source docs/ERRORS.md is generated from. It lives
// at internal/event/catalogue.yml, which ARCHITECTURE.md §12 names as its home
// and which is where a new code is added.
//
// This package used to embed a byte-for-byte copy of that file, because Go's
// embed directive cannot reach outside its own package directory and
// internal/event held no Go file to put the embed in. It has one now:
// event.Catalogue() returns the bytes, the copy under internal/render is gone,
// and so is the test that compared the two (tracker T-0058). A row added to the
// catalogue is rendered by this package with nothing to copy.

// row is one catalogue entry.
type row struct {
	Code    event.Code `yaml:"code"`
	Kind    string     `yaml:"kind"`
	Stage   string     `yaml:"stage"`
	Exit    int        `yaml:"exit"`
	Message string     `yaml:"message"`
	Args    []string   `yaml:"args"`
}

// catalogue is the parsed file, keyed by code.
type catalogue map[event.Code]row

var (
	loadOnce sync.Once
	loaded   catalogue
	loadErr  error
)

// codes returns the parsed catalogue, parsing it once.
//
// A malformed catalogue is a build-time mistake that only shows up at runtime,
// so it is not swallowed: every renderer that cannot find a code says so on the
// line where the message would have been, and a catalogue that would not parse
// at all makes every line say it.
func codes() (catalogue, error) {
	loadOnce.Do(func() {
		var rows []row
		if err := yaml.Unmarshal(event.Catalogue(), &rows); err != nil {
			loadErr = fmt.Errorf("render: parsing the code catalogue: %w", err)
			return
		}
		c := make(catalogue, len(rows))
		for _, r := range rows {
			c[r.Code] = r
		}
		loaded = c
	})
	return loaded, loadErr
}

// text renders one event's line from the catalogue.
//
// Placeholders are {argkey}, drawn only from event.ArgKey. A placeholder with
// no argument is left as it is rather than blanked, because a message with a
// hole in it names the missing argument and a message with a gap hides it.
func text(e event.Event) string {
	c, err := codes()
	if err != nil {
		return err.Error()
	}
	r, ok := c[e.Code]
	if !ok {
		// A code with no row is a bug in the tree, not something a user did.
		// ARCHITECTURE.md §7 says CI fails on it; until that job exists, the
		// line says which code is missing rather than printing nothing.
		return fmt.Sprintf("%s: no row in internal/event/catalogue.yml", e.Code)
	}
	return substitute(r.Message, args(e))
}

// args is the substitution table: the event's own Args, with the fields Event
// carries in its own right filled in where Args does not name them.
//
// The fallback matters because Event.Table, Event.Column, Event.Done and
// Event.Total are typed fields and Args is a map of strings: a stage that set
// the field and not the key would otherwise render "{table}" verbatim.
func args(e event.Event) map[event.ArgKey]string {
	out := make(map[event.ArgKey]string, len(e.Args)+4)
	for k, v := range e.Args {
		out[k] = v
	}
	if _, ok := out[event.ArgTable]; !ok && e.Table != (event.Event{}).Table {
		out[event.ArgTable] = e.Table.String()
	}
	if _, ok := out[event.ArgColumn]; !ok && e.Column != "" {
		out[event.ArgColumn] = e.Column
	}
	if _, ok := out[event.ArgCount]; !ok && e.Done != 0 {
		out[event.ArgCount] = strconv.FormatInt(e.Done, 10)
	}
	if _, ok := out[event.ArgStage]; !ok {
		out[event.ArgStage] = e.Stage.String()
	}
	return out
}

// substitute replaces every {key} the table names.
func substitute(message string, table map[event.ArgKey]string) string {
	if !strings.ContainsRune(message, '{') {
		return message
	}
	var b strings.Builder
	for {
		open := strings.IndexByte(message, '{')
		if open < 0 {
			b.WriteString(message)
			return b.String()
		}
		shut := strings.IndexByte(message[open:], '}')
		if shut < 0 {
			b.WriteString(message)
			return b.String()
		}
		shut += open
		b.WriteString(message[:open])
		key := event.ArgKey(message[open+1 : shut])
		if v, ok := table[key]; ok {
			b.WriteString(v)
		} else {
			b.WriteString(message[open : shut+1])
		}
		message = message[shut+1:]
	}
}
