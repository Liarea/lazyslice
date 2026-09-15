// SPDX-License-Identifier: Apache-2.0

package textsig

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// De-obfuscating normalisation (the 2026-09-15 red team, attack A2).
//
// Every validator in this package is a parse, and a parse is defeated by
// writing the same value a different way. The red team's A2 wrote a phone
// number as "plus four four, seven seven oh oh, nine one one, nine one one",
// an email address as "grace.hopper AT realcorp DOT example", a national
// insurance number with interleaved spaces and a payment card with "."
// separators, and every one of them reached the target verbatim under exit 0:
// no name rule matched the column, and no validator matched the value, so the
// report affirmatively said "no name or value signal".
//
// Obfuscation is not encryption. A human reading "seven seven oh oh" reads a
// phone number, so the validators have to as well, and the cheapest way to say
// that once for both nets is here: internal/classify's signals and
// internal/verify's second net both read this package, which is the whole
// reason it exists (internal/textsig/CLAUDE.md).
//
// The shape of the fix is a *candidate list* rather than a rewrite of each
// validator. Candidates(s) yields the value as it stands plus the canonical
// spellings of the de-obfuscations below, and the four validators that parse a
// written value — email, phone, Luhn and IBAN — accept the value if any
// candidate parses. The other validators are untouched: an IP address, a MAC,
// a UUID and a URL have no folk spelling, and the dictionary-backed and
// entropy-backed ones are shape guesses whose false-positive rate a candidate
// list would multiply for nothing.
//
// What it costs. A candidate is a *new string offered to the same parser*, so
// this widens recall and never loosens a parse: nothing that failed
// ValidEmail before can pass it now except by way of a spelling this file
// produced on purpose. What it does risk is a false positive on prose — "meet
// me at the dot com office" is one substitution away from something with an @
// in it — so each rule below is written to require the whole value to look
// like the thing, and net/mail, libphonenumber and the two checksums still
// have to agree afterwards.

// maxCandidateLen bounds the work. Every validator that reads a candidate has
// its own length cap far below this (320 for an email address, 40 for a phone
// number, 34 for an IBAN), so a longer value can only ever be rejected — and
// the second net runs these over every value of every unmasked column, so a
// free-text column of paragraphs must not pay for a regexp per row.
const maxCandidateLen = 512

var (
	// atRE is " at ", "(at)", "[at]" and "{at}", in any case, with the
	// surrounding whitespace consumed: "grace.hopper AT realcorp" is one
	// token either side of an @ and not three words.
	atRE = regexp.MustCompile(`(?i)\s*[\(\[\{]\s*(?:at|@)\s*[\)\]\}]\s*|\s+at\s+`)
	// dotRE is the same for the separator, in the four languages the red
	// team's fixtures used. "kropka" is Polish, "punkt" German and
	// Scandinavian, "punto" Italian and Spanish.
	dotRE = regexp.MustCompile(`(?i)\s*[\(\[\{]\s*(?:dot|kropka|punkt|punto)\s*[\)\]\}]\s*|\s+(?:dot|kropka|punkt|punto)\s+`)
	// angleRE is RFC 5322's name-addr: the address inside the angle brackets
	// of "Grace Hopper <grace.hopper@realcorp.example>". ValidEmail rejects a
	// value carrying "<" or ">" outright — it has to, or every display name in
	// the world would be an address — so before this the whole shape was
	// invisible to both nets, which is the form the 2026-09-15 red team's A4a
	// wrote its bytea payload in. The address is offered as a candidate; the
	// display name and anything after the brackets are dropped, because it is
	// the address that is the personal value and net/mail still has to accept
	// what comes out.
	angleRE = regexp.MustCompile(`<([^<>\s@]+@[^<>\s@]+)>`)
)

// Candidates returns the value as it stands followed by the canonical
// spellings of the de-obfuscations this file recognises, without duplicates.
// The first element is always the input, so a caller that stops at the first
// match behaves exactly as it did before this existed.
//
// It is exported because internal/classify and internal/verify both need the
// same list — one to decide a column, one to refuse a loaded target — and a
// second hand copy of it is the defect internal/textsig was created to end
// (tracker T-0055).
func Candidates(s string) []string {
	out := []string{s}
	if len(s) == 0 || len(s) > maxCandidateLen || !mayBeObfuscated(s) {
		return out
	}
	add := func(c string) {
		if c == "" || len(c) > maxCandidateLen {
			return
		}
		for _, existing := range out {
			if existing == c {
				return
			}
		}
		out = append(out, c)
	}
	spelled := s
	if at := atRE.ReplaceAllString(s, "@"); at != s {
		spelled = at
	}
	if dot := dotRE.ReplaceAllString(spelled, "."); dot != spelled {
		spelled = dot
	}
	if m := angleRE.FindStringSubmatch(spelled); m != nil {
		add(m[1])
	}
	add(spelled)
	// The separator-only dodge: a passport number, a card number or an IBAN
	// written with spaces between the characters or between the groups.
	if collapsed, ok := collapseGroups(spelled); ok {
		add(collapsed)
	}
	if digits, ok := spelledDigits(s); ok {
		add(digits)
	}
	return out
}

// collapseGroups joins a value written as short space-separated groups —
// "4111 1111 1111 1111", "G B 3 3 B U K B", "AB 98 76 54 D" — into one token.
//
// The group-length bound is what keeps this from reading a sentence. Without
// it, "Meet me at the dot com office on Tuesday" normalises to
// "Meet me@the.com office on Tuesday" and then collapses to
// "Meetme@the.comofficeonTuesday", whose domain carries a dot and which
// net/mail therefore accepts: prose becomes an email address by a route
// nobody wrote. A value whose every whitespace-separated field is four
// characters or fewer is a number or an identifier somebody spaced out, and
// prose is not (no sentence is made entirely of words of four letters or
// fewer for long enough to matter here, and anything this misses is still
// offered to the parser as it stands).
func collapseGroups(s string) (string, bool) {
	fields := strings.Fields(s)
	if len(fields) < 2 {
		return "", false
	}
	for _, f := range fields {
		if len(f) > maxGroupLen {
			return "", false
		}
	}
	return strings.Join(fields, ""), true
}

// maxGroupLen is collapseGroups' bound: four characters is a card's printed
// group and an IBAN's, and one character is the interleaved-space dodge.
const maxGroupLen = 4

// mayBeObfuscated is the cheap gate in front of the regexps: every rule below
// needs at least one space or one bracket, so a value with neither — which is
// what most columns hold — costs one scan and no allocation.
func mayBeObfuscated(s string) bool {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case ' ', '\t', '(', '[', '{', '<':
			return true
		}
	}
	return strings.ContainsRune(s, '\u00a0')
}

// anyCandidate reports whether ok accepts any spelling of s. It is how the four
// parse-backed validators read Candidates, and it is deliberately not a method
// on anything: a validator stays func(string) bool from the outside, which is
// this package's contract (internal/textsig/CLAUDE.md).
func anyCandidate(s string, ok func(string) bool) bool {
	if ok(s) {
		return true
	}
	cands := Candidates(s)
	for _, c := range cands[1:] {
		if ok(c) {
			return true
		}
	}
	return false
}

// digitWords is the spelled-out digit alphabet. "oh" and "o" are how a phone
// number is read aloud in English and are the spelling the red team used;
// "nought" and "naught" are the same digit again.
var digitWords = map[string]string{
	"zero": "0", "oh": "0", "o": "0", "nought": "0", "naught": "0",
	"one": "1", "two": "2", "three": "3", "four": "4", "five": "5",
	"six": "6", "seven": "7", "eight": "8", "nine": "9",
}

// minSpelledDigits is the floor under spelledDigits. Below it the candidate
// could not be a phone number, a card or an IBAN anyway (libphonenumber's
// shortest valid international number is seven digits, ISO/IEC 7812's shortest
// card is twelve), and the floor is what keeps an ordinary phrase — "one or
// two" — from becoming a number at all. On a value that also carries letter
// groups it is applied to the candidate's whole alphanumeric length, because a
// national identifier's digits are only part of it: a UK National Insurance
// number is two letters, six digits and a letter.
const minSpelledDigits = 7

// minMixedDigits is the second floor, and it applies only to a value carrying
// letter groups: at least four of the characters have to be digits that were
// spelled out or written. Without it "abcd efgh one two" would be a candidate
// on the strength of two digit words, which is a phrase and not an identifier.
const minMixedDigits = 4

// spelledDigits reads a value whose digits are written as words back into
// digits, or reports that it is not one.
//
// Precision is the whole of this function's design, and it is why this is a
// separate pass rather than a word-by-word substitution inside the
// normalisation above. Substituting digit words wherever they appear would
// turn every English sentence containing "one" into a string with a 1 in it,
// and with a twelve-digit floor and a Luhn check roughly one such sentence in
// ten would then be a payment card.
//
// So every field of the value has to be one of four things: a digit word, a
// literal digit run, one of the three multipliers, or — since the T-REDFIX
// review's third finding — a **short alphanumeric group**, bounded by the same
// maxGroupLen collapseGroups uses. The last of those is what closes the brief's
// own A2 case, which the first version of this file did not: the red team's
// `ident` column held "AB nine eight seven six five four D", a UK National
// Insurance number with interleaved spaces and spelled digits, and it reached
// the target verbatim because "ab" is not a digit word and this returned early
// on it. The digit-only spellings ("A B 9 8 7 6 5 4 D") worked, the tests
// covered those, and the docstring claimed the case was closed.
//
// Three things keep the letter groups from reopening the prose hole:
//
//   - the maxGroupLen bound, which is collapseGroups' own argument — a value
//     whose non-digit fields are all four characters or fewer is an identifier
//     somebody spaced out, and "Meet me at the dot com office" is not (office
//     is six);
//   - at least one digit actually spelled as a word, so a sentence of short
//     words around a literal number ("we owe you 123456789012 now") is not
//     rewritten into anything — it is offered to the parsers as it stands, as
//     it always was;
//   - the two floors: minMixedDigits spelled-or-written digits, and
//     minSpelledDigits alphanumeric characters in the candidate overall.
//
// "double" and "treble"/"triple" repeat the digit that follows them, which is
// how a British number is dictated: "nine double-oh" is 900. A leading "plus"
// is the international prefix, so "plus four four" is +44 and reaches
// libphonenumber as the international form ValidPhone requires.
//
// The letter groups are written out in the case the value had them in, because
// the candidate goes to the same parsers the raw value does and an identifier's
// letters are part of what they match.
func spelledDigits(s string) (string, bool) {
	if len(s) > 200 {
		return "", false
	}
	split := func(r rune) bool {
		switch r {
		case ' ', '\t', '\n', ',', '-', '.', '(', ')', '\u00a0', '\u2013', '\u2014':
			return true
		}
		return false
	}
	fields := strings.FieldsFunc(s, split)
	if len(fields) == 0 {
		return "", false
	}
	var b strings.Builder
	repeat := 1
	digits := 0
	letters := 0
	spelled := 0
	for i, f := range fields {
		lower := strings.ToLower(f)
		switch {
		case i == 0 && (lower == "plus" || lower == "+"):
			b.WriteByte('+')
			continue
		case lower == "double":
			repeat = 2
			continue
		case lower == "treble", lower == "triple":
			repeat = 3
			continue
		}
		if d, ok := digitWords[lower]; ok {
			b.WriteString(strings.Repeat(d, repeat))
			digits += repeat
			spelled += repeat
			repeat = 1
			continue
		}
		if allDigits(f) {
			b.WriteString(strings.Repeat(f, repeat))
			digits += len(f) * repeat
			repeat = 1
			continue
		}
		// A short alphanumeric group: the letters of an identifier whose
		// digits are spelled out. A pending multiplier has nothing to
		// multiply here — "double AB" is not a number — so it is refused
		// rather than applied to letters.
		if repeat == 1 && shortAlnumGroup(f) {
			b.WriteString(f)
			letters += len(f)
			continue
		}
		return "", false
	}
	// A trailing multiplier with nothing to multiply is not a number either.
	if repeat != 1 {
		return "", false
	}
	if letters > 0 {
		if spelled == 0 || digits < minMixedDigits || digits+letters < minSpelledDigits {
			return "", false
		}
		return b.String(), true
	}
	if digits < minSpelledDigits {
		return "", false
	}
	return b.String(), true
}

// shortAlnumGroup reports a field that is letters and digits only and no longer
// than maxGroupLen: the "AB" and the "D" of a National Insurance number, the
// "GB" of an IBAN. Anything with punctuation in it, and anything longer, is a
// word rather than a group and ends the parse.
func shortAlnumGroup(f string) bool {
	if f == "" || len(f) > maxGroupLen {
		return false
	}
	for _, r := range f {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// PrintableText reports whether a rendered value is text a person wrote rather
// than bytes a program wrote: valid UTF-8, non-empty, and at least
// printableRatio printable runes. Tab, newline and carriage return count as
// printable, because a document holds them.
//
// It is here, with the validators, because both nets need the same answer to
// the same question about the same value (the 2026-09-15 red team's A4a):
// internal/classify asks it before running the validators over a bytea
// column's samples, and internal/verify's second net asks it before running
// them over a bytea column of the loaded target. A second hand copy of it
// would be the drift internal/textsig exists to end.
//
// It is not a validator — it says nothing about whether the value is personal
// data — so it is named for what it answers.
func PrintableText(s string) bool {
	if s == "" || !utf8.ValidString(s) {
		return false
	}
	total, printable := 0, 0
	for _, r := range s {
		total++
		if unicode.IsPrint(r) || r == '\t' || r == '\n' || r == '\r' {
			printable++
		}
	}
	return total > 0 && float64(printable)/float64(total) >= printableRatio
}

// printableRatio is PrintableText's floor. It is deliberately high: a
// compressed blob, an image or an encrypted value fails it on the first byte
// that is not a character.
const printableRatio = 0.95
