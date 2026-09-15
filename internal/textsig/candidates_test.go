// SPDX-License-Identifier: Apache-2.0

package textsig

import "testing"

// The 2026-09-15 red team, attack A2 and its A3 array variant: values written
// so that no validator fires, in columns no name rule reaches. Every string
// below reached the target verbatim under exit 0 before candidates.go existed,
// with the classifier's report saying "no name or value signal".
func TestObfuscatedValuesStillParse(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		in   string
		ok   func(string) bool
	}{
		{"email spelled with AT and DOT", "grace.hopper AT realcorp DOT example", ValidEmail},
		{"email spelled lower case", "grace.hopper at realcorp dot example", ValidEmail},
		{"email in brackets", "grace.hopper(at)realcorp(dot)example", ValidEmail},
		{"email in square brackets", "grace.hopper [at] realcorp [dot] example", ValidEmail},
		{"email with a Polish separator", "grace.hopper AT klinika kropka example", ValidEmail},
		{"phone dictated in words", "plus four four, two oh, seven nine four six, oh nine five eight", ValidPhone},
		{"phone dictated with double", "plus four four, two oh, seven nine four six, oh nine five eight", ValidPhone},
		{"phone with interleaved spaces", "+ 4 4 2 0 7 9 4 6 0 9 5 8", ValidPhone},
		{"card with dot separators", "4111.1111.1111.1111", ValidLuhn},
		{"card with slash separators", "4111/1111/1111/1111", ValidLuhn},
		{"card with a non-breaking space", "4111\u00a01111\u00a01111\u00a01111", ValidLuhn},
		{"address inside an RFC 5322 display name", "Grace Hopper <grace.hopper1@realcorp.example>", ValidEmail},
		{"address inside a display name with a trailing number", "Grace Hopper <grace.hopper1@realcorp.example> 078-05-1120", ValidEmail},
		{"iban with interleaved spaces", "G B 3 3 B U K B 2 0 2 0 1 5 5 5 5 5 5 5 5 5", ValidIBAN},
		// The A2 `ident` value itself, which the first version of candidates.go
		// did not close: spelledDigits wanted every field to be a digit word,
		// so the two letters and the suffix ended the parse and the number
		// validated as nothing. Only its all-digit spelling, one line down, was
		// tested — a case the brief named, covered by a test that could not
		// fail on it (the T-REDFIX review's third finding).
		{"national insurance number with spelled digits", "AB nine eight seven six five four D", ValidNationalID},
		{"national insurance number with interleaved spaces", "A B 9 8 7 6 5 4 D", ValidNationalID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if !tc.ok(tc.in) {
				t.Errorf("%q did not validate; the red team's A2 dodge is open", tc.in)
			}
		})
	}
}

// The precision half. A candidate is a new spelling offered to the same
// parser, so ordinary prose must not become a personal value by passing
// through it: spelledDigits requires the *whole* value to be digit words, and
// the at/dot substitution still has to produce something net/mail accepts.
func TestCandidatesDoNotInventValues(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		in   string
		ok   func(string) bool
	}{
		{"a sentence with at and dot in it", "Meet me at the dot com office on Tuesday", ValidEmail},
		{"a sentence with one digit word", "there is only one way to do this", ValidLuhn},
		{"counting to three", "one two three", ValidPhone},
		{"a room number", "Room 4 at the end of the hall", ValidEmail},
		{"a trailing multiplier", "four four two oh seven nine four six oh nine five eight double", ValidPhone},
		{"an ordinary title", "CHARIOTS CONSPIRACY PART TWO", ValidPhone},
		// The letter-group arm's own precision cases. A long number surrounded
		// by short words has no *spelled* digit in it, so nothing is rewritten;
		// two digit words among letter groups clear neither floor.
		{"a sentence around a long number", "we owe you 123456789012 now", ValidLuhn},
		{"two digit words among short groups", "abcd efgh one two", ValidNationalID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.ok(tc.in) {
				t.Errorf("%q validated; a candidate spelling invented a value", tc.in)
			}
		})
	}
}

// Candidates always offers the value as it stands first, so a caller that
// stops at the first match behaves exactly as it did before this existed.
func TestCandidatesLeadWithTheRawValue(t *testing.T) {
	t.Parallel()
	for _, in := range []string{"", "plain", "a b c", "grace.hopper AT x DOT test"} {
		got := Candidates(in)
		if len(got) == 0 || got[0] != in {
			t.Errorf("Candidates(%q)[0] = %q, want the input", in, got)
		}
	}
}

// A value with no space and no bracket takes the cheap path: one scan, one
// element, no allocation beyond the slice. The second net runs these over
// every value of every unmasked column of the target.
func TestCandidatesShortCircuitOnAPlainValue(t *testing.T) {
	t.Parallel()
	if got := Candidates("grace.hopper@realcorp.example"); len(got) != 1 {
		t.Errorf("Candidates on a plain value = %d spellings, want 1", len(got))
	}
}
