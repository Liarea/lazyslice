// SPDX-License-Identifier: Apache-2.0

package textsig

import (
	"strings"
	"testing"
	"unicode"
)

// The 2026-09-15 red team, attacks A1 and A2b: non-English names in columns no
// name rule reaches. Every value below reached the target verbatim under exit
// 0, with the classifier's report affirmatively printing "no name or value
// signal", because names.txt was English only.
//
// LooksLikeName is the classifier's side — one dictionary word is enough, so a
// `nazwisko` column of one surname per row is caught — and NameShape is the
// second net's, which needs the given-then-surname pair.
func TestNonEnglishNamesAreInTheDictionary(t *testing.T) {
	t.Parallel()
	d := Dictionary()
	for _, full := range []string{
		"Bogusław Szczepański",
		"Nkechi Okonkwo",
		"Þórunn Jónsdóttir",
		"Wanjiru Kamau",
		"Ayşe Yıldırım",
	} {
		if !d.LooksLikeName(full) {
			t.Errorf("LooksLikeName(%q) = false; the classifier's name signal misses it", full)
		}
		if !d.NameShape(full) {
			t.Errorf("NameShape(%q) = false; the second net misses it", full)
		}
	}
	// One word per row is the shape a `nazwisko`/`cognome`/`achternaam` column
	// actually holds, and it is the classifier's side alone.
	for _, one := range []string{"Kowalski", "Wanjiru", "Yıldırım", "Szczepański", "Rossi"} {
		if !d.LooksLikeName(one) {
			t.Errorf("LooksLikeName(%q) = false; a one-word surname column is still copied verbatim", one)
		}
	}
}

// ContainsName splits on unicode.IsLetter, not [a-zA-Z]: an ASCII-only
// splitter cut "Bogusław" into "bogus" and "aw", neither of which is in any
// section of names.txt, so prose naming that person read as prose with no name
// in it.
func TestProseFindsANonASCIIName(t *testing.T) {
	t.Parallel()
	d := Dictionary()
	const note = "The claim was confirmed by Bogusław Szczepański on the fourteenth of March."
	if !d.ContainsName(note) {
		t.Error("ContainsName missed a non-ASCII name; the letter splitter is still ASCII-only")
	}
	if !d.Prose(note) {
		t.Error("Prose missed a note naming a person in Polish")
	}
	if !d.ProseName(note) {
		t.Error("ProseName missed a given-then-surname pair in Polish")
	}
}

// The precision rules the header of names.txt states, held to by a test rather
// than by good intentions: nothing shorter than three characters (a two-letter
// value is an ISO code far more often than a person), and the dictionary still
// does not read an ordinary English noun column as a table of people beyond
// the surnames it already carried.
//
// The T-0188 review round (finding 1) caught a pull that promoted 23
// surname-section English words into the given section too -- long, sun,
// berry among them -- and added "has" as a new surname outright. Either one
// breaks givenThenSurname's whole narrowing: a promoted word paired with any
// of the ~200 ordinary-English surnames already in the file reads as a
// person, and "has" is common enough to turn ordinary prose into ProseName.
// These cases pin the fix rather than leaving it to the pull script.
func TestDictionaryKeepsItsPrecisionRules(t *testing.T) {
	t.Parallel()
	d := Dictionary()
	for word := range d.all {
		if len(word) < 3 {
			t.Errorf("names.txt carries %q, shorter than three characters", word)
		}
	}
	for _, notAName := range []string{
		"Park Lane", "Long Acre", "Can Opener",
		// T-0188 review finding 1: these ordinary English words must stay
		// surname-only, or every one of these real street/place names reads
		// as a person under the second net.
		"Long Lane", "Sun Hill", "Berry Hill",
		"Mitchell Lane", "Morris Lane", "Duncan Hill", "Lin Wood",
	} {
		if d.NameShape(notAName) {
			t.Errorf("NameShape(%q) = true; an ordinary English pair became a person", notAName)
		}
	}
	for _, notProse := range []string{
		"The sun has set over the harbour tonight.",
		"This long has been the standard pricing for the region.",
	} {
		if d.ProseName(notProse) {
			t.Errorf("ProseName(%q) = true; an ordinary sentence with no person in it became one", notProse)
		}
	}
}

// TestT0188NamesAreDetected is the T-0188 review round's finding 4: no test in
// this package asserted anything the task actually added, so a stub that
// added zero new names -- or one that added 29,000 dead unreachable lines --
// would have passed this package's suite identically. These are the ten
// full names testdata/regressions/024 carries (T-0188, red team round two
// A10's shape, German, French, Spanish, Portuguese, Turkish, Hindi, Arabic,
// Japanese, Korean and Chinese, each romanised where the source language
// does not use Latin script) plus an İ-initial Turkish given name -- the
// U+0130 capital dotted I that finding 2's normalisation fix is about --
// paired with a Turkish surname.
func TestT0188NamesAreDetected(t *testing.T) {
	t.Parallel()
	d := Dictionary()
	for _, full := range []string{
		"Adolf Strauss",
		"Mia Trudeau",
		"Aida Hernández",
		"Zita Araújo",
		"Alina Kaşıkçı",
		"Mallika Thapa",
		"Abdullah Bushnak",
		"Daisuke Satō",
		"Jeong Hwang",
		"Tíngtíng Gao",
		"İbrahim Kaşıkçı",
	} {
		if !d.LooksLikeName(full) {
			t.Errorf("LooksLikeName(%q) = false; T-0188's dictionary should carry this name", full)
		}
		if !d.NameShape(full) {
			t.Errorf("NameShape(%q) = false; the second net should carry this given-then-surname pair", full)
		}
	}

	// The same review round's finding 1: the ordinary-English pairs the
	// promoted words made possible must stay false, so the new recall above
	// and this precision boundary both fail together in one test run rather
	// than only under a container-based torture run.
	for _, notAName := range []string{"Long Lane", "Sun Hill", "Berry Hill"} {
		if d.NameShape(notAName) {
			t.Errorf("NameShape(%q) = true; an ordinary English pair became a person", notAName)
		}
	}
}

// TestNamesFileHasNoUnreachableEntries guards against the whole family of dead
// entries the T-0188 review round found (finding 2): 70 Turkish lines began
// with U+0069 U+0307 ("i" + COMBINING DOT ABOVE), the signature of Python
// case-folding Turkish "İ" (U+0130). Go's strings.ToLower does not fold that
// way, and Dict.nameWords / Dict.ContainsName split on unicode.IsLetter, which
// treats U+0307 as a separator -- so no sampled value could ever produce such a
// key and the entries, and the names they carried, were silent dead weight.
// Any line containing a rune unicode.IsLetter rejects (the apostrophe aside,
// which the tokenizer keeps as part of a word) is unreachable the same way and
// should never be committed again.
func TestNamesFileHasNoUnreachableEntries(t *testing.T) {
	t.Parallel()
	section := ""
	for i, raw := range strings.Split(namesTXT, "\n") {
		line := strings.TrimSpace(raw)
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
		for _, r := range line {
			if r == '\'' {
				continue
			}
			if !unicode.IsLetter(r) {
				t.Errorf("names.txt:%d: %q contains %q (U+%04X), which is not a letter -- this entry's key can never be produced by the tokenizer that reads sampled values", i+1, line, r, r)
				break
			}
		}
	}
}
