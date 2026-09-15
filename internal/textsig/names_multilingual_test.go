// SPDX-License-Identifier: Apache-2.0

package textsig

import "testing"

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
func TestDictionaryKeepsItsPrecisionRules(t *testing.T) {
	t.Parallel()
	d := Dictionary()
	for word := range d.all {
		if len(word) < 3 {
			t.Errorf("names.txt carries %q, shorter than three characters", word)
		}
	}
	for _, notAName := range []string{"Park Lane", "Long Acre", "Can Opener"} {
		if d.NameShape(notAName) {
			t.Errorf("NameShape(%q) = true; an ordinary English pair became a person", notAName)
		}
	}
}
