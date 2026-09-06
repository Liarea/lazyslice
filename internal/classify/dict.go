// SPDX-License-Identifier: Apache-2.0

package classify

import (
	_ "embed"
	"strings"
	"sync"
)

// namesTXT is the English name dictionary (ARCHITECTURE.md §4 "Signals"). Like
// the rule pack it is embedded: there is no path to point at another copy.
//
//go:embed names.txt
var namesTXT string

// nameDict is the dictionary in the two forms the validators use: the given and
// surname sets separately, and their union.
type nameDict struct {
	Given   map[string]bool
	Surname map[string]bool
	All     map[string]bool
}

var dictionary = sync.OnceValue(loadDict)

func loadDict() *nameDict {
	d := &nameDict{
		Given:   map[string]bool{},
		Surname: map[string]bool{},
		All:     map[string]bool{},
	}
	section := ""
	for _, line := range strings.Split(namesTXT, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			marker := strings.TrimSpace(strings.TrimPrefix(line, "#"))
			if marker == "given" || marker == "surname" {
				section = marker
			}
			continue
		}
		name := strings.ToLower(line)
		switch section {
		case "given":
			d.Given[name] = true
		case "surname":
			d.Surname[name] = true
		default:
			continue
		}
		d.All[name] = true
	}
	return d
}

// looksLikeName reports whether a whole sampled value is a person's name: one
// or two dictionary words, nothing else. It is the validator behind
// ARCHITECTURE.md §10's "196/200 samples in name dictionary".
func (d *nameDict) looksLikeName(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 64 {
		return false
	}
	words := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return r == ' ' || r == '-' || r == '\'' || r == '.' || r == ','
	})
	if len(words) == 0 || len(words) > 3 {
		return false
	}
	for _, w := range words {
		if !d.All[w] {
			return false
		}
	}
	return true
}

// containsName reports whether a dictionary name appears as a word inside a
// longer string. This is the prose case: the free-text columns in
// testdata/README.md trap 17 carry the names of people on *other* rows, which is
// why a whole-value match is not enough.
func (d *nameDict) containsName(s string) bool {
	if len(s) > 1<<16 {
		s = s[:1<<16]
	}
	words := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && r != '\''
	})
	for _, w := range words {
		if len(w) < 3 {
			continue
		}
		if d.All[w] {
			return true
		}
	}
	return false
}
