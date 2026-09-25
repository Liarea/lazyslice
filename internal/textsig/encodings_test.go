// SPDX-License-Identifier: Apache-2.0

package textsig

import (
	"encoding/base64"
	"strings"
	"testing"
)

// bs is a backslash, so a backslash-u escape can be written into a test value
// as the six characters it is rather than the character Go would decode it to.
const bs = "\\"

// The 2026-09-25 JSON red team, round 1 (docs/reviews/2026-09-25-redteam-json/
// round1.json entries 16 and 21; T-0403): a known shape in another code point
// or a reversible encoding. Every value below was copied into the target under
// exit 0, in a plain text column and under a neutral JSON key alike, and each
// case first shows the parser alone rejects it, so the candidate is what
// closes it.
func TestEncodedSpellingsStillParse(t *testing.T) {
	t.Parallel()
	email := "ana.fake@example.org"
	for _, tc := range []struct {
		name  string
		in    string
		ok    func(string) bool
		plain func(string) bool
	}{
		{"fullwidth at sign", "ana.fake\uff20example.org", ValidEmail, validEmail},
		{"small form at sign", "ana.fake\ufe6bexample.org", ValidEmail, validEmail},
		{"fullwidth plus and digits", "\uff0b\uff14\uff14 \uff12\uff10 \uff17\uff19\uff14\uff16 \uff10\uff19\uff15\uff18", ValidPhone, validPhone},
		{"card with en-dash separators", "4111\u20131111\u20131111\u20131111", ValidLuhn, validLuhn},
		{"card with em-dash separators", "4111\u20141111\u20141111\u20141111", ValidCard, func(s string) bool { return validCard(s, false) }},
		{"card with minus-sign separators", "4111\u22121111\u22121111\u22121111", ValidLuhn, validLuhn},
		{"card with underscore separators", "4111_1111_1111_1111", ValidLuhn, validLuhn},
		{"card with fullwidth digits", "\uff14\uff11\uff11\uff11 \uff11\uff11\uff11\uff11 \uff11\uff11\uff11\uff11 \uff11\uff11\uff11\uff11", ValidLuhn, validLuhn},
		{"iban with underscore separators", "GB33_BUKB_2020_1555_5555_55", ValidIBAN, validIBAN},
		{"ssn with en-dash separators", "078\u201305\u20131120", ValidNationalIDStructured, validNationalIDStructured},
		{"ssn with underscore separators", "078_05_1120", ValidNationalIDStructured, validNationalIDStructured},
		{"national insurance number with slashes", "AB/98/76/54/D", ValidNationalIDStructured, validNationalIDStructured},
		{"trailing root dot on the domain", "ANA.FAKE@EXAMPLE.ORG.", ValidEmail, validEmail},
		{"zero-width space inside a card", "4111\u200b1111\u200b1111\u200b1111", ValidLuhn, validLuhn},
		// A zero-width space inside an address needs no candidate: net/mail
		// accepts it, and the red team's own a7.sql value was masked for that reason.
		{"zero-width joiner inside a phone number", "+44 20\u200d 7946 0958", ValidPhone, validPhone},
		{"soft hyphen inside a card", "4111\u00ad1111\u00ad1111\u00ad1111", ValidLuhn, validLuhn},
		{"backslash-u escaped at sign", "ana.fake" + bs + "u0040example.org", ValidEmail, validEmail},
		{"backslash-u escaped plus", bs + "u002B442079460958", ValidPhone, validPhone},
		{"mailto scheme", "mailto:" + email, ValidEmail, validEmail},
		{"mailto scheme in capitals with a subject", "MAILTO:" + email + "?subject=hello", ValidEmail, validEmail},
		// A tel: URI needs no candidate: libphonenumber reads RFC 3966 itself.
		{"sms scheme with a body", "sms:+442079460958?body=hi", ValidPhone, validPhone},
		{"percent-encoded at sign", "ana.fake%40example.org", ValidEmail, validEmail},
		{"percent-encoded plus", "%2B442079460958", ValidPhone, validPhone},
		{"percent-encoded mailto", "mailto:ana.fake%40example.org", ValidEmail, validEmail},
		{"base64 of an address", base64.StdEncoding.EncodeToString([]byte(email)), ValidEmail, validEmail},
		{"unpadded base64 of an address", base64.RawStdEncoding.EncodeToString([]byte(email)), ValidEmail, validEmail},
		{"url-safe base64 of an address", base64.RawURLEncoding.EncodeToString([]byte("an~a?@example.org")), ValidEmail, validEmail},
		{"base64 of a phone number", base64.StdEncoding.EncodeToString([]byte("+442079460958")), ValidPhone, validPhone},
		{"base64 of an address spelled with AT", base64.StdEncoding.EncodeToString([]byte("ana.fake AT example DOT org")), ValidEmail, validEmail},
		// The shell's spelling: echo keeps the newline, and base64 encodes it.
		{"base64 of an address with a trailing newline", base64.StdEncoding.EncodeToString([]byte(email + "\n")), ValidEmail, validEmail},
		{"base64 of an address with a trailing CRLF", base64.StdEncoding.EncodeToString([]byte(email + "\r\n")), ValidEmail, validEmail},
		{"base64 of a phone number with a trailing newline", base64.StdEncoding.EncodeToString([]byte("+442079460958\n")), ValidPhone, validPhone},
		{"base64 of a card with a trailing newline", base64.StdEncoding.EncodeToString([]byte("4111111111111111\n")), ValidLuhn, validLuhn},
		// MIME (RFC 2045) and Postgres's encode(..., 'base64') break the
		// output every 76 characters.
		{"MIME-wrapped base64 of an address", mimeWrap(base64.StdEncoding.EncodeToString([]byte(longEmail + "\n"))), ValidEmail, validEmail},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.plain(tc.in) {
				t.Fatalf("%q parses without a candidate; the case proves nothing", tc.in)
			}
			if !tc.ok(tc.in) {
				t.Errorf("%q did not validate; the JSON red team's spelling is open", tc.in)
			}
		})
	}
}

// longEmail encodes to more than one 76-character MIME line.
const longEmail = "ana.fake.with.a.considerably.longer.local.part@example.org"

// mimeWrap breaks s into lines of 76 characters, as RFC 2045 and Postgres's
// encode(..., 'base64') do.
func mimeWrap(s string) string {
	var b strings.Builder
	for len(s) > 76 {
		b.WriteString(s[:76])
		b.WriteString("\n")
		s = s[76:]
	}
	b.WriteString(s)
	return b.String()
}

// The precision half: the new spellings must not turn an ordinary value into a
// personal one. A date written with separators is the case that matters most,
// because eight bare digits is a length two checksum-only national identifier
// formats read, and TestLeafValueCategoryIsPinned holds "2026-09-24" copied in
// both internal/transform and internal/verify.
func TestEncodedSpellingsDoNotInventValues(t *testing.T) {
	t.Parallel()
	anything := func(s string) bool {
		return ValidEmail(s) || ValidPhone(s) || ValidLuhn(s) || ValidCard(s) || ValidIBAN(s) ||
			ValidNationalIDStructured(s) || ValidNationalIDChecksumOnly(s)
	}
	for _, in := range []string{
		"2026-09-24", "2026/09/24", "24/09/2026", "09/24/2026", "2026_09_24", "2026\u201309\u201324", "1999-12-31",
		"en-GB", "gpt-4o", "us-east-1", "Europe/London", "ad-slot-728x90", "snake_case_id_2",
		"Tuesdays", "username", "Qx7vR2mK9pL4tZ8wN3bH6cJ1yF5dS0aE", "d41d8cd98f00b204e9800998ecf8427e",
		"mailto:", "tel:", "sms:", "mailto:support", "100%", "50%25 off", "%zz%", bs + "u00zz", bs + "ud800",
		"Meet me at the dot com office on Tuesday.", "The end.", "caf\u00e9 cr\u00e8me", "\uff21\uff22\uff23",
		base64.StdEncoding.EncodeToString([]byte("just some words")),
		base64.StdEncoding.EncodeToString([]byte{0x00, 0x01, 0xfe, 0xff, 0x80, 0x7f, 0x10, 0x20}),
	} {
		if anything(in) {
			t.Errorf("%q validated through %q; a candidate spelling invented a value", in, Candidates(in))
		}
	}
}

// Each decoding is applied once, and never to its own output, which is the
// cost bound the red team's fix asked for: base64 of base64 of an address is
// not decoded twice, and a value longer than an encoder could write for the
// longest value any parser here reads is not decoded at all.
func TestEncodedSpellingsAreBounded(t *testing.T) {
	t.Parallel()
	email := "ana.fake@example.org"
	twice := base64.StdEncoding.EncodeToString([]byte(base64.StdEncoding.EncodeToString([]byte(email))))
	if ValidEmail(twice) {
		t.Errorf("base64 of base64 of an address validated; a decode was applied to its own output")
	}
	long := base64.StdEncoding.EncodeToString([]byte(email + strings.Repeat(" ", maxBase64Text)))
	if _, ok := base64Text(long); ok {
		t.Errorf("a %d-character base64 value was decoded; the bound is %d", len(long), maxBase64Len)
	}
	if _, ok := base64Text(base64.StdEncoding.EncodeToString([]byte(email))); !ok {
		t.Errorf("base64 of an address was not decoded at all")
	}
	// A plain address still takes the cheap path (TestCandidatesShortCircuitOnAPlainValue):
	// '@' and '.' are outside the base64 alphabet, and nothing in it is an escape.
	if got := Candidates(email); len(got) != 1 {
		t.Errorf("Candidates(%q) = %q, want the value alone", email, got)
	}
}
