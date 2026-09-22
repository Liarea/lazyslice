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

func validPhone(s string) bool { return validPhoneRegion(s, PhoneRegionHint) }

// ValidPhoneRegion is ValidPhone under a caller-supplied libphonenumber
// region instead of the fixed PhoneRegionHint ("ZZ", international only).
//
// An empty region behaves exactly like ValidPhone: PhoneRegionHint is
// substituted, so a caller need not special-case "no region configured"
// itself. A value already written in international form (a leading "+")
// parses the same way whatever region is passed -- libphonenumber reads the
// country code from the number itself -- so this only ever *widens* what
// parses over ValidPhone: a national-format number such as "07911 123456" or
// "020 7946 0958" needs a region to be read at all, which PhoneRegionHint can
// never supply (T-0221, the 2026-09-15 red team round 3's kontaktnr/contact
// finding: docs/reviews/2026-09-15-redteam/round3-still-leaking.json).
//
// It reads every spelling Candidates yields, exactly as ValidPhone does, so a
// number dictated in words under the given region is the number it spells
// (candidates.go).
//
// Which region to trust, and what a hit under it means, is the caller's
// question and not this package's own rule (internal/textsig/CLAUDE.md): a
// region an operator configured is different evidence from one a caller only
// guesses, and internal/classify and internal/verify are where that
// distinction is drawn, never here.
func ValidPhoneRegion(s, region string) bool {
	if region == "" {
		region = PhoneRegionHint
	}
	return anyCandidate(s, func(c string) bool { return validPhoneRegion(c, region) })
}

// SupportedPhoneRegion reports whether region is exactly one of the region
// codes libphonenumber's own table serves: upper case, two ASCII letters, no
// synonym table of its own -- "gb" and "UK" both answer false, only "GB"
// answers true for the United Kingdom. It answers a different question from
// ValidPhoneRegion above: whether the *region string* is one libphonenumber
// recognises at all, never whether a *number* parses under it. A caller that
// skips this and hands ValidPhoneRegion a region it does not recognise gets a
// silent "never parses" rather than a caller-visible refusal, which is what
// this function exists to give internal/core's --phone-region validation
// (T-0221 review round, finding 1: an unrecognised region used to reach the
// classifier and the second net unchecked and silently disable controls
// rather than refuse the flag).
func SupportedPhoneRegion(region string) bool {
	return phonenumbers.GetSupportedRegions()[region]
}

func validPhoneRegion(s, region string) bool {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 40 {
		return false
	}
	num, err := phonenumbers.Parse(s, region)
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
//
// net.ParseMAC alone accepts a bare run of hex digits with no separator at
// all, which admits 10-to-20-digit runs of plain 0-9 that were never written
// as a MAC by anyone: Pagila's address.phone (10-to-12 plain digits) is one
// (tracker T-0297), the same failure mode luhnSeparator's own comment records
// for a card written with "." groups. A MAC is written either with colon,
// dash or dot separators, or (unseparated) with at least one hex letter
// a-f/A-F; a value with neither is a plain digit run under a different
// category, so ValidMAC requires one of the two before it asks net.ParseMAC
// at all.
//
// This narrows what network_id catches, on purpose, and it is a
// THREAT_MODEL.md T1 question: a column with no name match that holds
// 12-digit phone numbers written without "+" (e.g. 447911123456) used to be
// masked as network_id on the MAC hit alone -- the wrong category, but
// masked. Now only a chance Luhn hit (roughly one in ten 12-to-19-digit
// values, internal/classify/classify.go's phraseLuhn / internal/verify's
// financial entry) keeps such a column masked; a mixed-length column with
// few 12-to-19-digit values can clear neither and land Category: none,
// Masked: false. The callers that lose this coverage are
// internal/classify's CatNetworkID value signal,
// internal/verify/validators.go's CatNetworkID digits/text entries (T1's
// second-net refusal), internal/verify/catalog.go's strongCatalogHitOverText
// and internal/plan/ddlliteral.go's strongValidators (both DDL-literal
// passes). Tracker T-0298 is the owed fix: a digit-run/phone-without-plus
// signal of its own, so a plain 10-to-12-digit column is not left to Luhn
// chance. See internal/textsig/CLAUDE.md's T-0297 section for the full
// account.
func ValidMAC(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	if !strings.ContainsAny(s, ":-.") && !containsHexLetter(s) {
		return false
	}
	_, err := net.ParseMAC(s)
	return err == nil
}

// containsHexLetter reports whether s has at least one a-f/A-F character.
func containsHexLetter(s string) bool {
	for _, r := range s {
		switch r {
		case 'a', 'b', 'c', 'd', 'e', 'f', 'A', 'B', 'C', 'D', 'E', 'F':
			return true
		}
	}
	return false
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

// ssnRE and ninoRE are two of the twelve national identifier formats with a
// pattern precise enough to decide a value rather than guess at one; the other
// ten are nationalid.go's (T-0187).
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
	// lookahead, so the excluded ranges are checked by validSSN (nationalid.go).
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
// twelve formats, in any spelling Candidates yields — so a number written with
// interleaved spaces is still the number it spells.
//
// Twelve, not the two this function held until T-0187 (the 2026-09-15 round-2
// red team, R2-01 through R2-04): a US Social Security number, a UK National
// Insurance number, a Polish PESEL, an Italian codice fiscale, a Dutch BSN, a
// Spanish DNI or NIE, a French NIR, a Brazilian CPF, a Canadian SIN, an Indian
// Aadhaar number and an Australian TFN — every one of them nationalid.go's.
//
// It is the union of ValidNationalIDStructured and ValidNationalIDChecksumOnly
// and nothing else, and it is deliberately **not** precise enough to decide a
// single occurrence (the T-0187 review round, finding 2, docs/reviews/): six of
// the twelve also constrain the value's shape and are what
// ValidNationalIDStructured alone answers for; the other six (PESEL, BSN, SIN,
// TFN, Aadhaar and CPF) are a mod-N sum, or two of them for CPF, over an
// otherwise unconstrained digit run, and a mod-N sum answers "yes" to a
// meaningful fraction of a random string of the right length regardless of
// what it means (measured: 9.1% of random 8-digit strings, 25.7% of 9-digit,
// 11.0% of 11-digit). A caller that refuses a whole run, or fails a whole
// column, on any single hit from this union — internal/plan's and
// internal/verify's DDL-literal passes did until this review, and
// internal/verify's own text-family net entry did too — refuses an ordinary
// schema roughly one time in four whenever it holds a nine-digit column of
// reference codes, order numbers or dates. Call ValidNationalIDStructured for
// that use instead; this union stays what it always was, for
// ValidNationalIDDigits's own fallback (below) and for the twelve-format
// positive coverage in nationalid_test.go, where every value is a *real*
// check-rule vector for its own format and a false accept is a checksum bug,
// not a shape false positive.
func ValidNationalID(s string) bool { return anyCandidate(s, validNationalID) }

func validNationalID(s string) bool {
	return validNationalIDStructured(s) || validNationalIDChecksumOnly(s)
}

// ValidNationalIDStructured is the six of the twelve formats whose check rule
// also constrains the value's *shape*, not only its checksum: a US SSN (the
// dashes are required), a UK NINO (a letter prefix and suffix from a fixed
// set), an Italian codice fiscale (six letters, a month letter, sixteen fixed
// positions), a Spanish DNI or NIE (a trailing check letter against a fixed
// table) and a French NIR (a fixed fifteen-character layout with a sex digit
// and a month code, itself checked against a mod-97 sum). The other six —
// PESEL, BSN, SIN, TFN, Aadhaar's Verhoeff check and CPF's own two check
// digits — are excluded here for the reason ValidNationalID's own comment
// gives: a mod-N sum over an unconstrained digit run is not a shape, so it
// clears at a rate a length guess would, not at the rate a checksum implies.
//
// This is the narrow function precise enough for a single-occurrence refusal
// (the T-0187 review round, finding 2): none of its six formats matches a bare
// run of digits at all — every one needs a dash, a letter, or (NIR) fifteen
// characters under its own mod-97 check, which a random string of that length
// clears about once in ninety-seven — so internal/verify/catalog.go's
// strongCatalogHit calls this instead of the twelve-format union,
// internal/verify/validators.go's own strong text-family entry does too, and
// internal/plan/ddlliteral.go's strongHit does as well (tracker T-0194): the
// review's own measurement (25.7% of random 9-digit strings, 11.0% of
// 11-digit clearing a checksum-only format) is why the union was not
// precise enough for a one-occurrence refusal on any of the three passes.
func ValidNationalIDStructured(s string) bool { return anyCandidate(s, validNationalIDStructured) }

func validNationalIDStructured(s string) bool {
	s = strings.TrimSpace(s)
	if m := ssnRE.FindStringSubmatch(s); m != nil && validSSN(m[1], m[2], m[3]) {
		return true
	}
	up := strings.ToUpper(strings.ReplaceAll(s, " ", ""))
	if ninoRE.MatchString(up) && !ninoDisallowed[up[:2]] {
		return true
	}
	switch {
	case validCodiceFiscale(s):
		return true
	case validDNI(s):
		return true
	case validNIE(s):
		return true
	case validNIR(up):
		return true
	}
	return false
}

// ValidNationalIDChecksumOnly is the other six of the twelve: PESEL, BSN, SIN,
// TFN, Aadhaar's Verhoeff check and CPF's own two check digits, none of which
// constrains anything about the value beyond its length and a weighted sum
// (CPF's is two weighted sums, which is why its own random-string acceptance
// is far below the other five's — see nationalid.go's validCPF — but it is
// grouped here rather than with ValidNationalIDStructured because it still has
// no shape constraint at all, only a digit run and a checksum, the same
// category of evidence as the other five and not the category a dash or a
// letter table is). It is what a *ratio* over a whole column may still decide
// — internal/verify/validators.go's own non-strong text-family entry — and
// what nothing may decide on one occurrence: the T-0187 review round's own
// measurement is 9.1% of random 8-digit strings, 25.7% of 9-digit and 11.0% of
// 11-digit, none of which a one-hit refusal can tell apart from a real leak.
func ValidNationalIDChecksumOnly(s string) bool { return anyCandidate(s, validNationalIDChecksumOnly) }

func validNationalIDChecksumOnly(s string) bool {
	s = strings.TrimSpace(s)
	switch {
	case validPESEL(s):
		return true
	case validBSN(s):
		return true
	case validCPF(s):
		return true
	case validSIN(s):
		return true
	case validAadhaar(s):
		return true
	case validTFN(s):
		return true
	}
	return false
}

// ValidNationalIDDigits reports whether a value is a national identifier
// rendered without its format's usual separators — the shape a numeric
// (bigint/integer/numeric) column produces, which cannot hold a hyphen and
// silently drops a leading zero (T-0187, red team round 2's A9b).
// "078-05-1001" stored as a bigint renders as the eight-digit "78051001", and
// ValidNationalID's SSN branch requires the dashes, so it never sees the same
// number twice.
//
// It is a separate function from ValidNationalID and is never read by
// Candidates, by design: internal/verify/catalog.go's and
// internal/plan/ddlliteral.go's DDL-literal passes both call
// ValidNationalIDStructured directly on SQL text (the T-0187 review round,
// finding 2, and tracker T-0194) and must never gain this recall either. An
// SSN has no check digit at all, so a bare nine-digit number is "SSN-shaped"
// about as often as a random nine-digit number clears the SSA's exclusion
// ranges — precise enough for a
// *ratio* over a whole numeric column (internal/classify's and
// internal/verify's digits: true entries, mirroring Luhn's own text/digits
// split, T-0136) and far too wide for a one-occurrence refusal, which is why
// the DDL-literal passes keep calling a Candidates-free function and not this
// one.
//
// A digit string of another length still reaches ValidNationalID unchanged —
// PESEL, BSN, CPF, SIN, Aadhaar and TFN carry their own checksum and need no
// separator in the first place, so the zero-pad recovery below is SSN's alone.
//
// The eight-digit branch skips its own zero-padded SSA check for a value that
// is also a real YYYYMMDD calendar date (looksLikePlausibleDate,
// nationalid.go, T-0187 second review round finding 1): the forced leading
// zero the padding writes means that check clears an ordinary date at a rate
// close to 1.0 over any realistic range, which is not a meaningful signal.
// The other checksum-only formats reachable through the fallback are
// unaffected — none of them shares SSN's padding failure.
func ValidNationalIDDigits(s string) bool {
	s = strings.TrimSpace(s)
	if !allDigits(s) {
		return validNationalID(s)
	}
	switch len(s) {
	case 8:
		if !looksLikePlausibleDate(s) && validSSN("0"+s[:2], s[2:4], s[4:8]) {
			return true
		}
	case 9:
		if validSSN(s[:3], s[3:5], s[5:9]) {
			return true
		}
	}
	return validNationalID(s)
}
