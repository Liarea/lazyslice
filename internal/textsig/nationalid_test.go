// SPDX-License-Identifier: Apache-2.0

package textsig

import (
	"fmt"
	"math/rand"
	"testing"
)

// TestValidNationalIDTwelveFormats is T-0187's coverage of the ten formats
// nationalid.go added to the two ValidNationalID already had (SSN, NINO
// already covered by candidates_test.go). Every positive vector here is
// constructed from its format's own check rule — never a real person's
// number — and every negative vector fails only the check digit or the
// exclusion rule, so a false accept here is a checksum bug and not a shape
// typo.
func TestValidNationalIDTwelveFormats(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		valid string
		// invalid is the same shape with the check digit/letter/rule broken.
		invalid string
	}{
		// US SSN: area/group/serial, no checksum. 078-05-1001 is an ordinary
		// area/group/serial; 000-05-1001 is the excluded area.
		{"US SSN", "078-05-1001", "000-05-1001"},
		// UK NINO: HMRC's excluded prefix pairs. AB123456D is ordinary; GB is
		// a disallowed prefix pair (used for the "GB" gazette shape, never
		// issued).
		{"UK NINO", "AB123456D", "GB123456D"},
		// Polish PESEL: weighted mod-10 checksum over the first ten digits.
		{"Polish PESEL", "44050612341", "44050612342"},
		// Italian codice fiscale: the CIN checksum over the first fifteen
		// characters, mod 26.
		{"Italian codice fiscale", "ABCDEF85M01H501I", "ABCDEF85M01H501A"},
		// Dutch BSN: the eleven-test (weighted sum, last weight -1, multiple
		// of eleven).
		{"Dutch BSN", "111111110", "111111111"},
		// Spanish DNI: eight digits mod 23 against the letter table.
		{"Spanish DNI", "12345678Z", "12345678A"},
		// Spanish NIE: X/Y/Z stands for 0/1/2, then the same mod-23 table.
		{"Spanish NIE", "X1234567L", "X1234567A"},
		// French NIR: the thirteen-digit number mod 97, key = 97 - remainder.
		{"French NIR", "185067511200101", "185067511200102"},
		// Brazilian CPF: two weighted check digits.
		{"Brazilian CPF", "12345678909", "12345678900"},
		// Canadian SIN: the Luhn check digit over nine digits.
		{"Canadian SIN", "123456782", "123456781"},
		// Indian Aadhaar: the Verhoeff checksum over all twelve digits.
		{"Indian Aadhaar", "123456789010", "123456789011"},
		// Australian TFN: the weighted mod-11 checksum.
		{"Australian TFN", "100000001", "100000002"},
	}
	for _, tc := range cases {
		if !ValidNationalID(tc.valid) {
			t.Errorf("%s: ValidNationalID(%q) = false, want true (a positive check-rule vector)", tc.name, tc.valid)
		}
		if ValidNationalID(tc.invalid) {
			t.Errorf("%s: ValidNationalID(%q) = true, want false (the check rule broken)", tc.name, tc.invalid)
		}
	}
}

// TestValidNationalIDRejectsOrdinaryShapes checks a set of hand-picked
// ordinary values against every one of the twelve formats. It is a set of
// deterministic vectors, not a precision claim: a run of identical digits is
// the one shape every mod-11/mod-10 checksum in this file is vulnerable to by
// accident (all the weighted sums scale linearly), so CPF excludes it by name;
// most of the rest are simply not shaped like any of the twelve formats at
// all (a letter, a dash, a length none of the twelve uses).
//
// Three vectors are deliberately shapes a real schema's own numeric columns
// hold — a sequential surrogate id, a YYYYMMDD booking date, an order code —
// added by the T-0187 review round (finding 4) because the version of this
// test that shipped with T-0187 asserted a *precision* claim
// ("ValidNationalID's doc comment") from a single hand-picked literal,
// "123456789", annotated "nine plain digits that happen to fail every
// checksum". It does happen to; 25.7% of random 9-digit strings do not
// (finding 2's own measurement), so a stub that rejected exactly that one
// string would have passed this test and did — a false accept over an
// ordinary schema is what finding 1's own probe fixture reproduced, and
// nothing here would have caught it, because no vector here was ever
// generated from the shape that broke. The *rate* claim now belongs to
// TestValidNationalIDStructuredPrecisionOverRandomDigitRuns, below, which
// tests the function that actually still promises it
// (ValidNationalIDStructured — ValidNationalID's own doc comment disclaims
// the property these three vectors used to stand in for). These three stay
// here as fixed regression vectors for the exact shapes finding 1 and
// finding 2's probe fixtures used, confirmed by direct computation (not
// hand-waving) to fail every one of BSN, SIN and TFN, the only checksum-only
// formats a bare 8- or 9-digit run can even reach.
func TestValidNationalIDRejectsOrdinaryShapes(t *testing.T) {
	t.Parallel()

	notIDs := []string{
		"",
		"1.2.3",
		"CHARIOTS CONSPIRACY", // pagila film title, per the IBAN precision note
		"2017-02-15T09:34:33Z",
		"00000000000", // eleven digits: PESEL- and CPF-shaped, and a repeated
		"11111111111", // digit run every checksum here excludes by name (allSameDigit)
		"000000000",   // nine digits: BSN/SIN/TFN-shaped, the same exclusion;
		// SSN's own regexp requires the dashes, so a bare digit run never
		// reaches it at all
		"123456789",     // nine plain digits that happen to fail every checksum
		"ORD-2024-0001", // an order number, not any of the twelve
		// A nine-digit sequential surrogate id (finding 1's probe fixture:
		// public.probe_orders.id, 100234567..100234576). Fails BSN's
		// eleven-test and SIN's Luhn digit by construction; TFN does not
		// apply at nine digits without also being a checksum coincidence,
		// verified by direct computation.
		"100000042",
		// A YYYYMMDD-shaped integer date (finding 1's probe fixture:
		// public.probe_orders.booked_on). Fails TFN's eight-digit weighted
		// sum; no other checksum-only format matches an eight-digit run.
		"20240115",
		// An eight-digit order/reference code with no calendar structure at
		// all, the other shape finding 1's fix comment names.
		"10023456",
	}
	for _, s := range notIDs {
		if ValidNationalID(s) {
			t.Errorf("ValidNationalID(%q) = true, want false: not shaped like any of the twelve formats", s)
		}
	}
}

// randomDigits returns a string of n pseudo-random decimal digits, source rng.
func randomDigits(rng *rand.Rand, n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte('0' + rng.Intn(10))
	}
	return string(b)
}

// TestValidNationalIDStructuredPrecisionOverRandomDigitRuns is the rate
// assertion the T-0187 review round (finding 4) asked for, over the function
// that actually still carries a precision claim: internal/verify/catalog.go's
// strongCatalogHit and internal/verify/validators.go's strong text-family
// entry both call ValidNationalIDStructured, not ValidNationalID, precisely
// because the six checksum-only formats made the union's own false-accept
// rate 9-25% over a random digit run of the right length (finding 2,
// measured). ValidNationalIDStructured's own doc comment claims none of its
// six formats matches a bare, unpunctuated, unlettered digit run *at all*
// except French NIR, whose fifteen-digit shape reduces to a single mod-97
// check once the month and department gates pass — about a 1-in-97 accept
// rate on a random candidate. This test measures both halves of that claim
// over a generated population rather than trusting the doc comment: a
// generated population is what would have caught finding 1 and finding 2, and
// a single hand-picked literal is what did not (this file's own history,
// above).
func TestValidNationalIDStructuredPrecisionOverRandomDigitRuns(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewSource(1))
	const trials = 10000

	t.Run("fifteen-digit runs clear near NIR's own 1-in-97 rate", func(t *testing.T) {
		hits := 0
		for i := 0; i < trials; i++ {
			if ValidNationalIDStructured(randomDigits(rng, 15)) {
				hits++
			}
		}
		// 1/97 is ~1.03%; the month/department shape gates only ever lower
		// the observed rate below the raw mod-97 one, never raise it, so 3%
		// is headroom over the true rate and still an order of magnitude
		// under the checksum-only formats' 9-25% (finding 2) — a regression
		// that widened NIR back toward that range, or dropped its shape
		// gates, fails this well before a one-occurrence refusal would ever
		// see it in production.
		if rate := float64(hits) / trials; rate > 0.03 {
			t.Errorf("ValidNationalIDStructured accepted %d/%d (%.2f%%) random 15-digit runs, want at or near NIR's own 1-in-97 mod-97 rate (~1.03%%), well under the 3%% ceiling a single-occurrence refusal needs", hits, trials, rate*100)
		}
	})

	for _, n := range []int{8, 9, 11, 12} {
		t.Run(fmt.Sprintf("%d-digit runs never match", n), func(t *testing.T) {
			hits := 0
			for i := 0; i < trials; i++ {
				if ValidNationalIDStructured(randomDigits(rng, n)) {
					hits++
				}
			}
			if hits != 0 {
				t.Errorf("ValidNationalIDStructured accepted %d/%d random %d-digit runs with no dash or letter, want 0: none of its six formats matches a bare digit run at that length", hits, trials, n)
			}
		})
	}
}

// TestValidNationalIDDigitsRecoversTheDroppedLeadingZero is the A9b fix
// (T-0187): a bigint or numeric column cannot hold a hyphen and silently
// drops a leading zero, so "078-05-1001" stored that way renders as the
// eight-digit "78051001" — a shape ValidNationalID's dashed regex never
// matches. ValidNationalIDDigits is the function the digits-family ratio
// entry in internal/classify's and internal/verify's second net reads
// instead.
func TestValidNationalIDDigitsRecoversTheDroppedLeadingZero(t *testing.T) {
	t.Parallel()

	// The eight-digit rendering of 001-01-0001 (area 001, group 01, serial
	// 0001 -- an ordinary SSN shape, chosen to collide with no other
	// format's own checksum at eight digits), and the excluded-area rewrite
	// that must still fail once zero-padded to 000-10-0001.
	if !ValidNationalIDDigits("01010001") {
		t.Errorf("ValidNationalIDDigits(%q) = false, want true (recovers the leading zero SSN 001-01-0001 lost)", "01010001")
	}
	if ValidNationalIDDigits("00100001") {
		t.Errorf("ValidNationalIDDigits(%q) = true, want false: zero-pads to the excluded area 000", "00100001")
	}
	// A nine-digit value needs no padding at all (area 100, group 01, serial
	// 0001).
	if !ValidNationalIDDigits("100010001") {
		t.Errorf("ValidNationalIDDigits(%q) = false, want true (a nine-digit SSN with no leading zero to lose)", "100010001")
	}
	// A digit-only format that never had a separator to lose still validates
	// through the same function.
	if !ValidNationalIDDigits("123456782") { // Canadian SIN, from the table above
		t.Errorf("ValidNationalIDDigits(%q) = false, want true (a SIN carries its own checksum, no padding needed)", "123456782")
	}
	// ValidNationalID itself must NOT gain this recall: internal/plan's and
	// internal/verify's DDL-literal passes call ValidNationalID directly on
	// SQL text, and a bare eight-digit number is a version triple or a part
	// number about as often as it is a Social Security number missing its
	// leading zero -- an ordinary schema would refuse under that widening.
	if ValidNationalID("01010001") {
		t.Errorf("ValidNationalID(%q) = true, want false: the digits-only SSN recall must stay out of the DDL-literal validator (T-0187)", "01010001")
	}
}

// TestValidNationalIDDigitsExcludesPlausibleDates is the T-0187 second review
// round's finding 1: the eight-digit branch's zero-padded SSA check forces the
// padded "area" field to start with '0', which can never fall in the
// excluded 666/9xx bands, so the check degenerates to "the year's two-digit
// suffix is not 00" — a rate close to 1.0 over any realistic booking range,
// not the ~97% the entry's own comment had assumed from averaging over
// 1950-2050. A value that is also a real YYYYMMDD calendar date must not
// clear the digits-family entry through that degenerate check.
func TestValidNationalIDDigitsExcludesPlausibleDates(t *testing.T) {
	t.Parallel()

	dates := []string{
		"20240115", // an ordinary 2024 booking date
		"20240229", // 2024 is a leap year: Feb 29 is real
		"19991231", // an ordinary date in a different century
		"20000615", // the century-boundary year the old ~97% figure relied on
	}
	for _, s := range dates {
		if ValidNationalIDDigits(s) {
			t.Errorf("ValidNationalIDDigits(%q) = true, want false: it is a real calendar date", s)
		}
	}

	// A non-date eight-digit shape must be unaffected: this is 001-01-0001's
	// leading zero recovered, the same value TestValidNationalIDDigitsRecoversTheDroppedLeadingZero
	// pins, restated here so the two guards cannot silently converge.
	if !ValidNationalIDDigits("01010001") {
		t.Errorf("ValidNationalIDDigits(%q) = false, want true: not a calendar date (month 01, day 01 of year 0001, out of the plausible range)", "01010001")
	}
	// An invalid calendar date (month 13) is not excluded by the date check,
	// so it still reaches the padded SSA check on its own merits.
	if !ValidNationalIDDigits("20241301") {
		t.Errorf("ValidNationalIDDigits(%q) = false, want true: month 13 is not a real date, so the SSA check still applies", "20241301")
	}
	// Nine-digit values are untouched by the eight-digit date exclusion.
	if !ValidNationalIDDigits("100010001") {
		t.Errorf("ValidNationalIDDigits(%q) = false, want true: the date exclusion is eight-digit only", "100010001")
	}
}
