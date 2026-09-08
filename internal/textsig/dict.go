// SPDX-License-Identifier: Apache-2.0

package textsig

import (
	_ "embed"
	"strings"
	"sync"
)

// namesTXT is the English name dictionary (ARCHITECTURE.md §4 "Signals"). Like
// the rule pack it is embedded: there is no path to point at another copy. It
// lived beside the rule pack in internal/classify until tracker T-0055, which
// moved it here so that internal/verify's second net could read it without
// importing a stage package or carrying a second copy of it.
//
//go:embed names.txt
var namesTXT string

// Dict is the name dictionary. The two sections of names.txt (`# given` and
// `# surname`) are read as one set for the validators that only ask whether a
// word is a name at all, and are also kept apart, because the two-word shapes
// below ask the narrower question: is this a *given* name followed by a
// *surname*. Keeping the sections costs one map and is what tells "Grace
// Hopper" from "Green Lane" — green and lane are both surnames in this list, and
// so are stone, hill, west, long, marsh, black, berry and about two hundred
// other ordinary English words (tracker T-0055 review).
type Dict struct {
	all     map[string]bool
	given   map[string]bool
	surname map[string]bool
}

var dictionary = sync.OnceValue(loadDict)

// Dictionary returns the embedded name dictionary. It is parsed once and shared,
// and it is read-only from the outside: the set is unexported so that no caller
// can add a word to a dictionary every other caller reads.
func Dictionary() *Dict { return dictionary() }

func loadDict() *Dict {
	d := &Dict{all: map[string]bool{}, given: map[string]bool{}, surname: map[string]bool{}}
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
		if section == "" {
			continue
		}
		word := strings.ToLower(line)
		d.all[word] = true
		if section == "given" {
			d.given[word] = true
		} else {
			d.surname[word] = true
		}
	}
	return d
}

// nameWords returns the words a value is made of when every one of them is in
// the dictionary. A value of more than three words, or one carrying a word the
// dictionary does not have, is not a name and returns nil.
func (d *Dict) nameWords(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 64 {
		return nil
	}
	words := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return r == ' ' || r == '-' || r == '\'' || r == '.' || r == ','
	})
	if len(words) == 0 || len(words) > 3 {
		return nil
	}
	for _, w := range words {
		if !d.all[w] {
			return nil
		}
	}
	return words
}

// givenThenSurname reports whether the sequence carries a given name
// immediately followed by a surname. That adjacency is the shape this package
// can tell from an ordinary English word pair, and it is the whole of the
// narrowing tracker T-0055's review asked for: "green lane", "hunter green" and
// "stone hill" are surname-surname, because green, lane, hunter, stone and hill
// are all in the surname section and none is a given name, while "Grace Hopper"
// and "Katherine Johnson" are given-then-surname. A word in both sections (rose)
// counts as whichever side of the pair it is standing on, so "berry rose" is
// still surname-then-given and not a name.
//
// The order is the Western one and it is deliberate: accepting either order
// would accept "berry rose" and every other pair of ordinary nouns where one
// happens to be a first name. A value written "Hopper, Grace" is therefore not a
// name to this function. That is the direction both callers of this shape want
// to fail in -- see NameShape.
func (d *Dict) givenThenSurname(words []string) bool {
	for i := 0; i+1 < len(words); i++ {
		if d.given[words[i]] && d.surname[words[i+1]] {
			return true
		}
	}
	return false
}

// LooksLikeName reports whether a whole sampled value is a person's name: one to
// three dictionary words, nothing else. It is the validator behind
// ARCHITECTURE.md §10's "196/200 samples in name dictionary".
//
// One word is enough here, which is what makes it right for internal/classify
// and wrong for internal/verify: a `first_name` column holds one word per row,
// and so does a `colour` column of black, brown, hill, green and wood, every one
// of which is a surname. The classifier's answer to that is a masked lookup
// table, which costs a lookup; the second net's answer would be exit 9 on a
// loaded target with no green path short of --unmask, so verify uses NameShape
// instead (internal/verify/CLAUDE.md, "the dictionary rule").
func (d *Dict) LooksLikeName(s string) bool {
	return len(d.nameWords(s)) >= 1
}

// NameShape is the whole-value shape internal/verify's second net scores
// person_name under: two or three dictionary words, nothing else, and a given
// name immediately followed by a surname among them.
//
// The first version of this was LooksLikeName with the single-word case removed
// ("two or three dictionary words"), and that was not enough. The dictionary's
// surname section is full of ordinary English nouns, so a column of two-word
// street names ("green lane", "west hill", "marsh lane") or of compound colours
// ("hunter green", "stone gray", "black cherry") was 100% "person_name" and
// exit 9 on a loaded target holding no personal data (tracker T-0055 review).
// Requiring the given-then-surname pair asks for evidence a street name and a
// colour cannot carry.
//
// It is narrower than LooksLikeName in both directions on purpose: a real name
// this misses is a column internal/classify still masks on LooksLikeName, and a
// name written surname-first is one this net lets through. Missing is the
// direction this net has to fail in, because its verdict is a refusal after the
// target is loaded (internal/verify/CLAUDE.md, "the dictionary rule").
func (d *Dict) NameShape(s string) bool {
	words := d.nameWords(s)
	return len(words) >= 2 && d.givenThenSurname(words)
}

// ContainsName reports whether a dictionary name appears as a word inside a
// longer string. This is the prose case: the free-text columns in
// testdata/README.md trap 17 carry the names of people on *other* rows, which is
// why a whole-value match is not enough.
func (d *Dict) ContainsName(s string) bool {
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
		if d.all[w] {
			return true
		}
	}
	return false
}

// Prose is internal/classify's free-text value signal: a sentence or more with a
// dictionary name in it. testdata/README.md trap 17's notes carry the names of
// people on other rows, which is what a whole-value name match would miss.
//
// The six-word floor excludes short values and nothing more. It does NOT keep a
// single dictionary word out of this validator -- an earlier version of this
// comment said it did, and that was wrong (tracker T-0055 review): "green" is
// not prose, but "The supplier may terminate this agreement on thirty days
// notice." is, because may is a surname, and so is any English sentence
// carrying black, brown, price, read, little, long, west, wood or one of the
// other ordinary words in the list. For internal/classify that width is the
// intended direction -- it masks the column, which costs a lookup -- and for
// internal/verify it is not, which is why the second net scores ProseName.
func (d *Dict) Prose(s string) bool {
	if len(strings.Fields(s)) < 6 {
		return false
	}
	return d.ContainsName(s)
}

// ProseName is the free-text shape internal/verify's second net scores: prose
// carrying a written name, rather than prose carrying a word that is also a
// surname. The word floor is Prose's; what is inside has to be a given name
// immediately followed by a surname, the same evidence NameShape asks a
// two-word value for, scanned across the whole string.
//
// This is the free-text half of the narrowing tracker T-0055's review asked
// for. Prose fires on one dictionary word, and ~200 of the surnames in
// names.txt are ordinary English words, so under Prose a column of contract
// clauses ("The supplier may terminate...") or of product descriptions
// (green, stone, black) was exit 9 on a target holding no personal data. It is
// still the trap-17 case that fails here: a note naming another row's person is
// prose with a name in it.
//
// A sentence boundary breaks the adjacency, so "...confirmed with Grace.
// Hopper Ltd was invoiced..." is not a name; a comma does not, so a list of
// names still reads as one.
func (d *Dict) ProseName(s string) bool {
	if len(s) > 1<<16 {
		s = s[:1<<16]
	}
	fields := strings.Fields(s)
	if len(fields) < 6 {
		return false
	}
	prevGiven := false
	for _, field := range fields {
		word, ends := trimWord(field)
		if prevGiven && d.surname[word] {
			return true
		}
		prevGiven = word != "" && d.given[word] && !ends
	}
	return false
}

// trimWord reduces one whitespace-separated field to its lower-cased letters and
// reports whether it ends a sentence or a clause, which breaks the adjacency
// ProseName looks for.
func trimWord(field string) (word string, ends bool) {
	last := field[len(field)-1]
	ends = last == '.' || last == '!' || last == '?' || last == ';' || last == ':'
	word = strings.Trim(strings.ToLower(field), "\"'`.,;:!?()[]{}<>-\u2014\u2013\u201c\u201d\u2018\u2019")
	return word, ends
}
