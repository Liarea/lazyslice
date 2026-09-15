// SPDX-License-Identifier: Apache-2.0

package textsig

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Ten more national identifier formats (T-0187, the 2026-09-15 round-2 red
// team's R2-01 through R2-04: docs/reviews/2026-09-15-redteam/round2-still-
// leaking.json, attempts A2, A6, A7, A9b).
//
// ValidNationalID was "exactly two formats" — a US Social Security number and
// a UK National Insurance number — and nothing on the row-scanning path ever
// called it at all: internal/classify's validators list had no national_id
// entry, and internal/verify's second net had none either, so a plain SSN in a
// column called ref_code or code, a text[] of NI numbers, and a JSON document
// whose only personal datum was an SSN all crossed into the target verbatim
// under exit 0, reported as "no name or value signal". rules.yml's own
// national_id name pattern already carries dni, cpf, aadhaar, pesel,
// codice_fiscale, bsn and nir among its abbreviations (the name half has
// always recognised them); this file is the value half those names were
// missing entirely.
//
// Each format below is a real check rule, not a length guess: a checksum, a
// weighted total, or the issuing authority's own exclusion ranges — but a
// checksum over an otherwise unconstrained digit run is not the same claim as
// a checksum over a value a regexp also constrains, and the T-0187 review
// round (finding 2) is what measured the gap: PESEL, BSN, SIN, TFN and
// Aadhaar's Verhoeff check (validPESEL, validBSN, validSIN, validTFN,
// validAadhaar, below) each clear a meaningful fraction of a random digit run
// of the right length — 9.1% of random 8-digit strings, 25.7% of 9-digit,
// 11.0% of 11-digit — which is a length guess wearing a checksum's clothes,
// not the precision a one-occurrence refusal needs. What actually keeps
// ValidNationalIDStructured (textsig.go) precise enough for
// internal/verify's DDL-literal pass and its own strong text-family entry is
// the other six — a US SSN, a UK NINO, an Italian codice fiscale, a Spanish
// DNI or NIE and a French NIR — every one of which also constrains the
// value's *shape*: a dash, a letter, or (NIR) a length no random digit run
// this package tests is. CPF's own two check digits (validCPF) land closer to
// those five's precision than to the shape-constrained six's, but it has no
// shape constraint either, so it is grouped with the checksum-only five and
// not the structured six — textsig.go's own comment says by how much closer.
// internal/plan/ddlliteral.go's DDL-literal pass calls
// ValidNationalIDStructured for the same reason (tracker T-0194).

// allSameDigit reports a string of one repeated digit: "000000000",
// "111111111". Every weighted-sum checksum below (PESEL, BSN, CPF's own
// version of this guard, SIN's Luhn, TFN) is linear in each digit, so a run of
// one repeated digit is either always a hit (all zeros sums to zero, which
// every modulus divides) or, for TFN and SIN, exercises a checksum an issuing
// authority also never assigns to a real number. Excluding the shape outright
// is what every one of those authorities' own validators does, and it is the
// difference between "a checksum" and "a checksum that also accepts every
// placeholder string a schema is likely to hold".
func allSameDigit(s string) bool {
	if s == "" {
		return false
	}
	for i := 1; i < len(s); i++ {
		if s[i] != s[0] {
			return false
		}
	}
	return true
}

// validSSN applies the US Social Security Administration's own exclusion
// ranges to a 9-digit number already split into its three fields: no area 000,
// 666 or 900-999, no group 00, no serial 0000. There is no checksum digit in an
// SSN, so the exclusions are the whole of the check rule — shared by the
// dashed text form (validNationalID below) and the zero-padded digits-only
// form a numeric column renders (ValidNationalIDDigits).
func validSSN(area, group, serial string) bool {
	switch {
	case area == "000" || area == "666" || area[0] == '9':
		return false
	case group == "00":
		return false
	case serial == "0000":
		return false
	}
	return true
}

// looksLikePlausibleDate reports an eight-digit string that also reads as a
// real YYYYMMDD calendar date (T-0187 second review round, finding 1).
// ValidNationalIDDigits's eight-digit branch zero-pads to nine and asks only
// validSSN's exclusion ranges of the result, and prepending a forced '0'
// means the padded "area" field can never fall in the 666 or 9xx excluded
// bands — the check degenerates to "is the year's own two-digit suffix not
// 00", which a date clears on every year but a century boundary. The review's
// own measurement is what this function exists to act on: 1000/1000 on
// YYYYMMDD dates across 2022-2024, against the 97% the entry's own comment
// had assumed from averaging over 1950-2050, where the century-boundary years
// are a much larger share of the range. A value that is also a real calendar
// date is evidence about a *date* column, not about a national identifier,
// whatever the degenerate padded check says about the same digits — so
// ValidNationalIDDigits skips that one check for it rather than trusting the
// ratio to notice, and falls back to validNationalID for the six
// checksum-only formats that do not share this failure mode (nationalid.go's
// own package comment has the measured rates for those).
func looksLikePlausibleDate(s string) bool {
	if len(s) != 8 {
		return false
	}
	year, err := strconv.Atoi(s[0:4])
	if err != nil || year < 1900 || year > 2099 {
		return false
	}
	month, err := strconv.Atoi(s[4:6])
	if err != nil || month < 1 || month > 12 {
		return false
	}
	day, err := strconv.Atoi(s[6:8])
	if err != nil || day < 1 || day > 31 {
		return false
	}
	t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	return t.Year() == year && int(t.Month()) == month && t.Day() == day
}

// peselRE is Poland's PESEL: eleven digits, no separator.
var peselRE = regexp.MustCompile(`\A\d{11}\z`)

// peselWeights is the weighted-sum check rule over the first ten digits.
var peselWeights = [10]int{1, 3, 7, 9, 1, 3, 7, 9, 1, 3}

func validPESEL(s string) bool {
	if !peselRE.MatchString(s) || allSameDigit(s) {
		return false
	}
	sum := 0
	for i := 0; i < 10; i++ {
		sum += int(s[i]-'0') * peselWeights[i]
	}
	check := (10 - sum%10) % 10
	return check == int(s[10]-'0')
}

// cfMonthLetters is the twelve month codes Italy's codice fiscale uses in
// place of a numeric month (Jan=A through Dec=T, skipping the five letters
// that were never assigned). A month letter outside this set fails the shape
// check before the checksum is even computed.
var cfMonthLetters = map[byte]bool{
	'A': true, 'B': true, 'C': true, 'D': true, 'E': true, 'H': true,
	'L': true, 'M': true, 'P': true, 'R': true, 'S': true, 'T': true,
}

// cfOdd and cfEven are the two conversion tables Italy's codice fiscale (CIN)
// checksum reads a character's value from, keyed by the character's position
// (odd or even, one-indexed) among the first fifteen. They are the published
// "valore dispari"/"valore pari" tables and nothing here approximates them.
var cfOdd = map[byte]int{
	'0': 1, '1': 0, '2': 5, '3': 7, '4': 9, '5': 13, '6': 15, '7': 17, '8': 19, '9': 21,
	'A': 1, 'B': 0, 'C': 5, 'D': 7, 'E': 9, 'F': 13, 'G': 15, 'H': 17, 'I': 19, 'J': 21,
	'K': 2, 'L': 4, 'M': 18, 'N': 20, 'O': 11, 'P': 3, 'Q': 6, 'R': 8, 'S': 12, 'T': 14,
	'U': 16, 'V': 10, 'W': 22, 'X': 25, 'Y': 24, 'Z': 23,
}
var cfEven = map[byte]int{
	'0': 0, '1': 1, '2': 2, '3': 3, '4': 4, '5': 5, '6': 6, '7': 7, '8': 8, '9': 9,
	'A': 0, 'B': 1, 'C': 2, 'D': 3, 'E': 4, 'F': 5, 'G': 6, 'H': 7, 'I': 8, 'J': 9,
	'K': 10, 'L': 11, 'M': 12, 'N': 13, 'O': 14, 'P': 15, 'Q': 16, 'R': 17, 'S': 18, 'T': 19,
	'U': 20, 'V': 21, 'W': 22, 'X': 23, 'Y': 24, 'Z': 25,
}

// validCodiceFiscale checks the sixteen-character shape (six letters, two
// digits, one month letter, two digits, one letter and three digits for the
// birthplace code, one check letter) and the check character itself: the sum
// of the first fifteen characters' table values, mod 26, read back as a
// letter.
func validCodiceFiscale(s string) bool {
	s = strings.ToUpper(strings.TrimSpace(s))
	if len(s) != 16 {
		return false
	}
	for i := 0; i < 16; i++ {
		c := s[i]
		switch {
		case i < 6, i == 8, i == 11, i == 15:
			if c < 'A' || c > 'Z' {
				return false
			}
		default:
			if c < '0' || c > '9' {
				return false
			}
		}
	}
	if !cfMonthLetters[s[8]] {
		return false
	}
	sum := 0
	for i := 0; i < 15; i++ {
		if (i+1)%2 == 1 { // one-indexed odd position
			sum += cfOdd[s[i]]
		} else {
			sum += cfEven[s[i]]
		}
	}
	return s[15] == byte('A'+sum%26)
}

// bsnRE is the Netherlands' Burgerservicenummer: nine digits, no separator.
var bsnRE = regexp.MustCompile(`\A\d{9}\z`)

// bsnWeights is the "11-proef" (eleven-test): the ninth digit's weight is
// negative one, and the weighted sum must be an exact multiple of eleven.
var bsnWeights = [9]int{9, 8, 7, 6, 5, 4, 3, 2, -1}

func validBSN(s string) bool {
	if !bsnRE.MatchString(s) || allSameDigit(s) {
		return false
	}
	sum := 0
	for i := 0; i < 9; i++ {
		sum += int(s[i]-'0') * bsnWeights[i]
	}
	return sum%11 == 0
}

// dniLetters is Spain's DNI/NIE check-letter table, indexed by the eight-digit
// number mod 23.
const dniLetters = "TRWAGMYFPDXBNJZSQVHLCKE"

var (
	dniRE = regexp.MustCompile(`\A(\d{8})([A-Za-z])\z`)
	nieRE = regexp.MustCompile(`\A([XYZxyz])(\d{7})([A-Za-z])\z`)
)

// nieLeadingDigit is the digit NIE's leading letter stands for before the
// check-letter table is read: X=0, Y=1, Z=2.
var nieLeadingDigit = map[byte]byte{'X': '0', 'Y': '1', 'Z': '2'}

func validDNI(s string) bool {
	m := dniRE.FindStringSubmatch(s)
	if m == nil {
		return false
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return false
	}
	return dniLetters[n%23] == strings.ToUpper(m[2])[0]
}

func validNIE(s string) bool {
	m := nieRE.FindStringSubmatch(s)
	if m == nil {
		return false
	}
	lead := nieLeadingDigit[strings.ToUpper(m[1])[0]]
	n, err := strconv.Atoi(string(lead) + m[2])
	if err != nil {
		return false
	}
	return dniLetters[n%23] == strings.ToUpper(m[3])[0]
}

// validNIR is France's numéro de sécurité sociale: a fixed fifteen characters
// — sex (1 or 2), year (2 digits), month (2 digits, or one of the four codes
// for an unusual case), department (2 digits, or 2A/2B for Corsica), commune
// (3 digits), order (3 digits), key (2 digits) — with the key computed as
// 97 minus the thirteen-digit number (Corsica's letters standing for 19/18)
// mod 97.
func validNIR(s string) bool {
	if len(s) != 15 {
		return false
	}
	s = strings.ToUpper(s)
	if s[0] != '1' && s[0] != '2' {
		return false
	}
	year, month, dept := s[1:3], s[3:5], s[5:7]
	commune, order, key := s[7:10], s[10:13], s[13:15]
	if !allDigits(year) || !allDigits(commune) || !allDigits(order) || !allDigits(key) {
		return false
	}
	switch month {
	case "01", "02", "03", "04", "05", "06", "07", "08", "09", "10", "11", "12",
		"20", "30", "40", "50":
	default:
		return false
	}
	var deptDigits string
	switch dept {
	case "2A":
		deptDigits = "19"
	case "2B":
		deptDigits = "18"
	default:
		if !allDigits(dept) {
			return false
		}
		deptDigits = dept
	}
	num, err := strconv.ParseInt(s[0:1]+year+month+deptDigits+commune+order, 10, 64)
	if err != nil {
		return false
	}
	k, err := strconv.Atoi(key)
	if err != nil {
		return false
	}
	return int64(k) == 97-num%97
}

// cpfWeighted is Brazil's CPF check-digit rule, applied twice: once over the
// first nine digits with weights 10 down to 2 to produce the tenth digit, and
// again over the first ten (the tenth included) with weights 11 down to 2 to
// produce the eleventh. A remainder under two maps to check digit 0.
func cpfWeighted(digits []int, startWeight int) int {
	sum := 0
	w := startWeight
	for _, d := range digits {
		sum += d * w
		w--
	}
	r := sum % 11
	if r < 2 {
		return 0
	}
	return 11 - r
}

func validCPF(s string) bool {
	if len(s) != 11 || !allDigits(s) {
		return false
	}
	if allSameDigit(s) {
		// Never issued: the CPF checksum passes every repeated-digit string,
		// which is exactly the false-positive shape a checksum-only rule
		// would otherwise accept on an ordinary column of "00000000000"-style
		// placeholder values.
		return false
	}
	d := make([]int, 11)
	for i := 0; i < 11; i++ {
		d[i] = int(s[i] - '0')
	}
	if cpfWeighted(d[:9], 10) != d[9] {
		return false
	}
	return cpfWeighted(d[:10], 11) == d[10]
}

// validSIN is Canada's Social Insurance Number: nine digits under the same
// Luhn check digit ValidLuhn uses, applied directly rather than through
// ValidLuhn because Luhn's own twelve-to-nineteen-digit floor (the ISO/IEC
// 7812 payment-card range) would reject a nine-digit SIN outright.
func validSIN(s string) bool {
	if len(s) != 9 || !allDigits(s) || allSameDigit(s) {
		return false
	}
	sum, double := 0, false
	for i := len(s) - 1; i >= 0; i-- {
		d := int(s[i] - '0')
		if double {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		double = !double
	}
	return sum%10 == 0
}

// verhoeffD is the multiplication table of the dihedral group D5, and
// verhoeffP the permutation applied to each digit before it is read against
// that table — together the Verhoeff checksum India's Aadhaar number uses,
// chosen over a simple weighted sum because it catches every single-digit
// substitution and every adjacent transposition, which a mod-11 check does
// not.
var verhoeffD = [10][10]int{
	{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
	{1, 2, 3, 4, 0, 6, 7, 8, 9, 5},
	{2, 3, 4, 0, 1, 7, 8, 9, 5, 6},
	{3, 4, 0, 1, 2, 8, 9, 5, 6, 7},
	{4, 0, 1, 2, 3, 9, 5, 6, 7, 8},
	{5, 9, 8, 7, 6, 0, 4, 3, 2, 1},
	{6, 5, 9, 8, 7, 1, 0, 4, 3, 2},
	{7, 6, 5, 9, 8, 2, 1, 0, 4, 3},
	{8, 7, 6, 5, 9, 3, 2, 1, 0, 4},
	{9, 8, 7, 6, 5, 4, 3, 2, 1, 0},
}
var verhoeffP = [8][10]int{
	{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
	{1, 5, 7, 6, 2, 8, 3, 0, 9, 4},
	{5, 8, 0, 3, 7, 9, 6, 1, 4, 2},
	{8, 9, 1, 6, 0, 4, 3, 5, 2, 7},
	{9, 4, 5, 3, 1, 2, 6, 8, 7, 0},
	{4, 2, 8, 6, 5, 7, 3, 9, 0, 1},
	{2, 7, 9, 3, 8, 0, 6, 4, 1, 5},
	{7, 0, 4, 6, 9, 1, 3, 2, 5, 8},
}

// validAadhaar checks India's twelve-digit Aadhaar number by running the
// Verhoeff algorithm over every digit, rightmost (the check digit itself)
// first: a valid number reduces to zero.
func validAadhaar(s string) bool {
	if len(s) != 12 || !allDigits(s) {
		return false
	}
	c := 0
	for i := 0; i < 12; i++ {
		d := int(s[len(s)-1-i] - '0')
		c = verhoeffD[c][verhoeffP[i%8][d]]
	}
	return c == 0
}

// tfnWeights is Australia's Tax File Number check rule: a weighted sum over
// the digits (the ninth applying only to the modern nine-digit TFN) that must
// be an exact multiple of eleven.
var tfnWeights = [9]int{1, 4, 3, 7, 5, 8, 6, 9, 10}

func validTFN(s string) bool {
	if (len(s) != 8 && len(s) != 9) || !allDigits(s) || allSameDigit(s) {
		return false
	}
	sum := 0
	for i := 0; i < len(s); i++ {
		sum += int(s[i]-'0') * tfnWeights[i]
	}
	return sum%11 == 0
}
