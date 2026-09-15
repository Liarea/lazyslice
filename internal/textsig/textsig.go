// SPDX-License-Identifier: Apache-2.0

// Package textsig holds the value-only half of ARCHITECTURE.md §4's validators:
// a pure function over one string, and the embedded English name dictionary the
// two dictionary-backed ones read.
//
// It exists because there were two copies. internal/classify has the validators
// because §4's signals are its job, and internal/verify's second net (§6 item 4)
// re-runs them over the loaded target — and a stage package may not import
// another stage package (internal/CLAUDE.md), so verify carried a hand copy of
// eight of the ten, which is how it came to be missing the two that need the
// dictionary (tracker T-0055). A copy that must be kept in step and is not is a
// second net that quietly stops agreeing with the classifier it is a second look
// at, so both packages now import this one and neither owns a validator.
//
// What is here is only ever a value shape. There is no rule pack here: no name
// pattern, no category, no confidence, no scoring, no threshold. A caller
// decides what a `true` means — internal/classify scores it into a category at
// a confidence, internal/verify decides whether a loaded column may keep it —
// and the two decide differently on purpose (see internal/verify/CLAUDE.md, "the
// dictionary rule"). Nothing here reads a column name, a neighbour, a schema or
// a database.
package textsig

import (
	"math"
	"net"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"unicode"

	"github.com/nyaruka/phonenumbers"
)

var (
	// uuidRE is the UUID check ARCHITECTURE.md §4 puts *before* the entropy
	// check for secrets: a column of UUIDs is high-entropy and is not a
	// credential.
	uuidRE = regexp.MustCompile(`\A[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\z`)
	hexRE  = regexp.MustCompile(`\A[0-9a-fA-F]+\z`)
)

// PhoneRegionHint is the libphonenumber region ValidPhone parses under: "ZZ",
// the unknown region, so only a number written in international form validates.
// See internal/classify/CLAUDE.md, "Decisions made during implementation": v1
// does not derive a region from a sibling country column.
const PhoneRegionHint = "ZZ"

// ValidEmail is net/mail.ParseAddress, tightened. ParseAddress accepts "a@b",
// which every hostname-shaped identifier in a database would satisfy, so the
// domain must also carry a dot.
//
// It reads every spelling Candidates yields, so "grace.hopper AT realcorp DOT
// example" is the address it is written as (candidates.go, the 2026-09-15 red
// team's A2). The parse itself is unchanged: a candidate is a new string
// offered to the same net/mail, never a loosening of it.
func ValidEmail(s string) bool { return anyCandidate(s, validEmail) }

func validEmail(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 320 || strings.ContainsAny(s, "<>") {
		return false
	}
	addr, err := mail.ParseAddress(s)
	if err != nil {
		return false
	}
	at := strings.LastIndex(addr.Address, "@")
	if at < 1 {
		return false
	}
	domain := addr.Address[at+1:]
	return strings.Contains(domain, ".") && !strings.HasSuffix(domain, ".")
}

// ValidPhone is libphonenumber's IsValidNumber under PhoneRegionHint. That is
// the conservative half of the rule: a national-format column reaches the
// classifier through its name, at `possible`, and is masked anyway.
//
// It reads every spelling Candidates yields, so a number dictated in words —
// "plus four four, seven seven oh oh, nine one one, nine one one" — is the
// number it spells (candidates.go).
func ValidPhone(s string) bool { return anyCandidate(s, validPhone) }

func validPhone(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 40 {
		return false
	}
	num, err := phonenumbers.Parse(s, PhoneRegionHint)
	if err != nil {
		return false
	}
	return phonenumbers.IsValidNumber(num)
}

// ValidIP reports whether a value is an IP address, with or without the prefix
// length an inet renders.
func ValidIP(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	if i := strings.IndexByte(s, '/'); i >= 0 {
		s = s[:i]
	}
	return net.ParseIP(s) != nil
}

// ValidMAC reports whether a value is a hardware address.
func ValidMAC(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	_, err := net.ParseMAC(s)
	return err == nil
}

// ValidUUID reports whether a value is a UUID in the canonical form.
func ValidUUID(s string) bool { return uuidRE.MatchString(strings.TrimSpace(s)) }

// ValidLuhn is the payment-card check digit. It runs only on a value that is
// twelve to nineteen digits after separators are removed, which is the range
// ISO/IEC 7812 allows; without that bound every even-length numeric identifier
// passes it about half the time.
//
// It reads every spelling Candidates yields, so a card dictated in words is
// still a card (candidates.go).
func ValidLuhn(s string) bool { return anyCandidate(s, validLuhn) }

// luhnSeparator is the set of characters a written card number may be grouped
// with. It was {' ', '-'} until the 2026-09-15 red team wrote a card with "."
// between the groups and watched this function reject it outright — the value
// then matched ValidMAC instead (a dotted sixteen-digit run parses as a
// hardware address) and was masked to an IP address, which is the right
// direction under the wrong category and would have been no masking at all on
// a column of a family network_id cannot hold. The slash and the non-breaking
// space are the two other separators a card is printed with.
func luhnSeparator(r rune) bool {
	switch r {
	case ' ', '-', '.', '/', '\u00a0':
		return true
	}
	return false
}

func validLuhn(s string) bool {
	digits := make([]int, 0, 20)
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			digits = append(digits, int(r-'0'))
		case luhnSeparator(r):
		default:
			return false
		}
	}
	if len(digits) < 12 || len(digits) > 19 {
		return false
	}
	sum, double := 0, false
	for i := len(digits) - 1; i >= 0; i-- {
		d := digits[i]
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

// ValidIBAN is the mod-97 check. It reads every spelling Candidates yields
// (candidates.go).
func ValidIBAN(s string) bool { return anyCandidate(s, validIBAN) }

func validIBAN(s string) bool {
	s = strings.ToUpper(strings.NewReplacer(" ", "", "-", "").Replace(strings.TrimSpace(s)))
	if len(s) < 15 || len(s) > 34 {
		return false
	}
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'A' || r > 'Z') {
			return false
		}
	}
	if !unicode.IsLetter(rune(s[0])) || !unicode.IsLetter(rune(s[1])) {
		return false
	}
	// ISO 13616: characters three and four are the check digits and are always
	// numeric. Without this, any fifteen-to-thirty-four-character run of
	// letters clears the mod-97 check about one time in ninety-seven — five of
	// pagila's own film titles do ("CHARIOTS CONSPIRACY" among them), which is
	// why internal/verify's second net has to score IBAN on a ratio rather than
	// as a parse. Requiring the two check digits is the spec, it costs no real
	// IBAN anything, and it is what makes this validator precise enough for
	// internal/plan's and internal/verify's DDL-literal passes to refuse a run
	// on one occurrence (the 2026-09-15 red team's A20).
	if s[2] < '0' || s[2] > '9' || s[3] < '0' || s[3] > '9' {
		return false
	}
	rearranged := s[4:] + s[:4]
	rem := 0
	for _, r := range rearranged {
		switch {
		case r >= '0' && r <= '9':
			rem = rem*10 + int(r-'0')
		default:
			rem = rem*100 + int(r-'A') + 10
		}
		rem %= 97
	}
	return rem == 1
}

// ValidURL reports whether a value is a URL with both a scheme and a host.
//
// Both halves are required, and that is the whole of the shape: "mailto:a@b",
// "a:b" and "//host/path" are not URLs to this function, because an opaque
// scheme and a scheme-relative reference are not the thing a person's profile
// link is. A timestamp ("2017-02-15T09:34:33Z") cannot reach it either, because
// a URL scheme must begin with a letter.
//
// It exists because LooksSecret used to answer for these values (tracker
// T-0100): a URL is 16 to 512 characters, has no space and no "@", mixes
// character classes and clears the entropy floor, so mastodon's accounts.uri
// (https://home.social.test/users/bea_donnelly1) was `credential` on every row
// and was masked to the fixed literal — safe, and wrong, and a plan refusal
// under the unique index it usually carries. A URL that names a person is an
// online_id: internal/classify runs this validator ahead of the secrets one, so
// the column is masked under a category whose generator emits a URL-shaped
// value out of a domain large enough for a unique column.
func ValidURL(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 2048 || strings.ContainsAny(s, " \t\n\r") {
		return false
	}
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	return u.Scheme != "" && u.Host != ""
}

// LooksSecret is the Shannon-entropy check, run after the UUID check exactly as
// ARCHITECTURE.md §4 orders them. The guards before the entropy are what stop
// an email address or a sentence from reading as a secret: a credential has no
// whitespace, no "@", and mixes character classes or is long hex.
//
// A URL is excluded for the same reason a UUID is: it clears every one of those
// guards and is not a secret (tracker T-0100). Excluding it here would be a
// fail-open on its own — a URL that carries a username would drop to `none` and
// be copied verbatim (THREAT_MODEL.md T1) — so it is only half of the change,
// and ValidURL above is the other half.
func LooksSecret(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 16 || len(s) > 512 {
		return false
	}
	if ValidUUID(s) || ValidURL(s) || strings.ContainsAny(s, " \t\n@") {
		return false
	}
	if hexRE.MatchString(s) {
		return len(s) >= 32 && shannon(s) >= 3.0
	}
	classes := 0
	for _, in := range []func(rune) bool{
		func(r rune) bool { return r >= 'a' && r <= 'z' },
		func(r rune) bool { return r >= 'A' && r <= 'Z' },
		func(r rune) bool { return r >= '0' && r <= '9' },
	} {
		for _, r := range s {
			if in(r) {
				classes++
				break
			}
		}
	}
	return classes >= 2 && shannon(s) >= 3.2
}

// shannon is the entropy of a string in bits per byte.
func shannon(s string) float64 {
	if s == "" {
		return 0
	}
	var counts [256]int
	for i := 0; i < len(s); i++ {
		counts[s[i]]++
	}
	n := float64(len(s))
	h := 0.0
	for _, c := range counts {
		if c == 0 {
			continue
		}
		p := float64(c) / n
		h -= p * math.Log2(p)
	}
	return h
}

// AddressShape is ARCHITECTURE.md §10's "mixed digits and words": a street line
// carries a number and at least two words.
func AddressShape(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 200 {
		return false
	}
	digits, words := false, 0
	for _, f := range strings.Fields(s) {
		hasLetter := false
		for _, r := range f {
			switch {
			case r >= '0' && r <= '9':
				digits = true
			case unicode.IsLetter(r):
				hasLetter = true
			}
		}
		if hasLetter {
			words++
		}
	}
	return digits && words >= 2
}

// TwoLetterCode is the low-confidence value shape ARCHITECTURE.md §10 records
// for a column of ISO codes: it explains a column rather than masking one.
func TwoLetterCode(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) != 2 {
		return false
	}
	for _, r := range s {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

// ssnRE and ninoRE are the two national identifier formats with a pattern
// precise enough to decide a value rather than guess at one.
//
// They exist for internal/plan's and internal/verify's DDL-literal passes (the
// 2026-09-15 red team's A20). Those passes run only the validators that are a
// parse, because the text they read is SQL and full of English words — and
// THREAT_MODEL.md T1 stated the exclusion of national_id in the same breath as
// person_name and address, which is where the red team put
// "Alice Anderson, 42 Elm St, SSN 123-45-6789, IBAN GB33BUKB20201555555555"
// into a CHECK and watched it cross into the target under exit 0. A name and a
// street are dictionary heuristics and stay out; a US Social Security number
// and a UK National Insurance number are neither.
var (
	// ssnRE is the US Social Security number's shape. RE2 has no negative
	// lookahead, so the excluded ranges are checked in validNationalID below.
	ssnRE = regexp.MustCompile(`\A(\d{3})-(\d{2})-(\d{4})\z`)
	// ninoRE is the UK National Insurance number: two prefix letters, six
	// digits, one suffix letter from A-D. The excluded prefix letters (D, F, I,
	// Q, U, V in first position; D, F, I, O, Q, U, V in second) and the
	// disallowed pairs are HMRC's.
	ninoRE = regexp.MustCompile(`\A[ABCEGHJKLMNOPRSTWXYZ][ABCEGHJKLMNPRSTWXYZ]\d{6}[A-D]\z`)
)

// ninoDisallowed are the prefix pairs HMRC never issues.
var ninoDisallowed = map[string]bool{"BG": true, "GB": true, "NK": true, "KN": true, "TN": true, "NT": true, "ZZ": true}

// ValidNationalID reports whether a value is a national identifier in one of
// the two formats above, in any spelling Candidates yields — so a number
// written with interleaved spaces is still the number it spells.
//
// It is deliberately two formats and not a family of them. A validator here
// answers about one value with a parse; a loose "looks like an identifier"
// rule over SQL text would refuse ordinary schemas, which is the failure mode
// the DDL passes' narrow validator set exists to avoid.
func ValidNationalID(s string) bool { return anyCandidate(s, validNationalID) }

func validNationalID(s string) bool {
	s = strings.TrimSpace(s)
	if m := ssnRE.FindStringSubmatch(s); m != nil {
		// The Social Security Administration's own exclusions: no area 000,
		// 666 or 900-999, no group 00, no serial 0000. Without them every
		// three-two-four digit grouping in a schema — a version triple, a date
		// range, a part number — would be a national identifier.
		area, group, serial := m[1], m[2], m[3]
		switch {
		case area == "000" || area == "666" || area[0] == '9':
		case group == "00":
		case serial == "0000":
		default:
			return true
		}
	}
	up := strings.ToUpper(strings.ReplaceAll(s, " ", ""))
	return ninoRE.MatchString(up) && !ninoDisallowed[up[:2]]
}
