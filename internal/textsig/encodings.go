// SPDX-License-Identifier: Apache-2.0

package textsig

import (
	"encoding/base64"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// Code-point spellings and reversible encodings (the 2026-09-25 JSON red team,
// docs/reviews/2026-09-25-redteam-json/round1.json entries 16 and 21; tracker
// T-0403).
//
// candidates.go's rules read a value written the way a person dictates it. The
// JSON red team wrote the same values the way a program or a keyboard in
// another locale writes them, and every one crossed under exit 0 in a text
// column and under a neutral JSON key alike: an address with a fullwidth at
// sign (U+FF20), an international phone number in fullwidth digits and a
// fullwidth plus, a card with en-dash or underscore separators, an address
// whose domain ends in the DNS root's trailing dot, an address behind a
// mailto: scheme, and a phone number percent-encoded ("%2B44..."). None of
// those is a shape the validators do not know; each is a known shape in
// another code point or a reversible encoding, which THREAT_MODEL.md T1's
// A2 amendment ("obfuscation is not encryption") already promised to read.
//
// The rules are the same as candidates.go's: every spelling here is a new
// string offered to the same parsers, never a loosening of one, and the raw
// value is still offered first. Each decoding is applied at most once and
// nothing decoded is decoded again, so a value costs a bounded number of
// passes over at most maxCandidateLen bytes whatever it holds.

// canonicalSpelling is the value with its code-point and transport spellings
// undone, in the order a value is wrapped: a backslash-u escape and a percent
// escape are undone first, because either can spell any character below;
// then NFKC (fullwidth and other compatibility forms to their plain
// equivalents, so U+FF20 is "@", U+FF0B is "+" and U+FF14 is "4"); then every
// format character (Unicode Cf: the zero-width space, joiner and non-joiner,
// the word joiner, the byte-order mark, the soft hyphen and the bidi
// controls) is removed and every dash (Unicode Pd, and U+2212 MINUS SIGN,
// which is a symbol and looks the same) becomes the ASCII hyphen every
// parser here already reads as a group separator; then a leading mailto:,
// tel: or sms: scheme is removed, with the hfields or parameters after it;
// and last the trailing dot a fully qualified domain name may carry.
//
// It reports false when the value has none of these, which is what most
// values are: the gate is one pass over the bytes.
func canonicalSpelling(s string) (string, bool) {
	if !mayBeEncoded(s) {
		return "", false
	}
	n := s
	if strings.Contains(n, `\u`) {
		n = unescapeU(n)
	}
	if hasPercentEscape(n) {
		// A percent escape that decodes to bytes that are not UTF-8 was not
		// text, and the value is left as it was written.
		if d := percentDecode(n); utf8.ValidString(d) {
			n = d
		}
	}
	if !isASCII(n) {
		n = norm.NFKC.String(n)
		n = strings.Map(func(r rune) rune {
			switch {
			case unicode.Is(unicode.Cf, r):
				return -1
			case r == '\u2212', r != '-' && unicode.Is(unicode.Pd, r):
				return '-'
			}
			return r
		}, n)
	}
	n = stripScheme(n)
	n = strings.TrimSpace(strings.TrimRight(strings.TrimSpace(n), "."))
	if n == "" || n == s {
		return "", false
	}
	return n, true
}

// mayBeEncoded is canonicalSpelling's gate: a byte outside ASCII (a
// compatibility form, a format character or a Unicode dash), a percent or a
// backslash (an escape), a trailing dot, or one of the three schemes.
func mayBeEncoded(s string) bool {
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c >= utf8.RuneSelf, c == '%', c == '\\':
			return true
		}
	}
	t := strings.TrimSpace(s)
	return strings.HasSuffix(t, ".") || schemeOf(t) != ""
}

// candidateSchemes are the URI schemes whose whole content is a personal value
// a validator here parses: an address (RFC 6068), a telephone number (RFC
// 3966) and a number to text (RFC 5724). ValidURL does not claim any of them,
// because an opaque scheme has no host (textsig.go).
var candidateSchemes = []string{"mailto:", "tel:", "sms:"}

// schemeOf returns the scheme t begins with, in any case, or "".
func schemeOf(t string) string {
	for _, sch := range candidateSchemes {
		if len(t) > len(sch) && strings.EqualFold(t[:len(sch)], sch) {
			return sch
		}
	}
	return ""
}

// stripScheme removes a leading mailto:, tel: or sms: and what follows the
// value inside the URI: a mailto's "?subject=" hfields, a tel's ";ext="
// parameters, an sms's "?body=". A value with no such scheme is returned as
// it is.
func stripScheme(s string) string {
	t := strings.TrimSpace(s)
	sch := schemeOf(t)
	if sch == "" {
		return s
	}
	rest := strings.TrimPrefix(t[len(sch):], "//")
	cut := "?;"
	if sch == "mailto:" {
		cut = "?"
	}
	if i := strings.IndexAny(rest, cut); i >= 0 {
		rest = rest[:i]
	}
	return rest
}

// unescapeU undoes JSON's and JavaScript's \uXXXX escape, surrogate pairs
// included, so "ana.fake\u0040example.org" is the address it spells. A
// malformed escape or a lone surrogate is left as written.
func unescapeU(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		r, n := uEscape(s[i:])
		if n == 0 {
			b.WriteByte(s[i])
			i++
			continue
		}
		if utf16.IsSurrogate(r) {
			if r2, n2 := uEscape(s[i+n:]); n2 > 0 {
				if d := utf16.DecodeRune(r, r2); d != unicode.ReplacementChar {
					b.WriteRune(d)
					i += n + n2
					continue
				}
			}
			b.WriteString(s[i : i+n])
			i += n
			continue
		}
		b.WriteRune(r)
		i += n
	}
	return b.String()
}

// uEscape reads one \uXXXX escape at the start of s, reporting its length (six)
// or zero when s does not start with one.
func uEscape(s string) (rune, int) {
	if len(s) < 6 || s[0] != '\\' || s[1] != 'u' {
		return 0, 0
	}
	var r rune
	for i := 2; i < 6; i++ {
		if !isHex(s[i]) {
			return 0, 0
		}
		r = r<<4 | rune(unhex(s[i]))
	}
	return r, 6
}

// hasPercentEscape reports a "%" followed by two hex digits.
func hasPercentEscape(s string) bool {
	for i := 0; i+2 < len(s); i++ {
		if s[i] == '%' && isHex(s[i+1]) && isHex(s[i+2]) {
			return true
		}
	}
	return false
}

// percentDecode undoes every %XX escape and leaves everything else, a "+"
// included, as written: a "+" in a phone number is the international prefix,
// not a form-encoded space.
func percentDecode(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) && isHex(s[i+1]) && isHex(s[i+2]) {
			b.WriteByte(unhex(s[i+1])<<4 | unhex(s[i+2]))
			i += 2
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func isHex(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func unhex(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	default:
		return c - 'A' + 10
	}
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

// Base64. A value that is base64 of an address held until now only by the
// accident of its entropy: LooksSecret read it as a credential, which masks
// it, and a longer encoding of low-entropy text could fall under that floor
// and be copied (round1.json entry 21). base64Text offers the decoded text as
// a candidate when it is plausibly text somebody encoded:
//
//   - the value, with its ASCII whitespace removed (a MIME or PEM encoder,
//     and Postgres's own encode(..., 'base64'), break the output into lines
//     of 76 or 64 characters), is 8 to maxBase64Len characters of one base64
//     alphabet (standard or URL-safe, never both), with at most two trailing
//     "=" and a length an encoder can produce;
//   - the decode, less the line break a line-oriented encoder keeps from its
//     input ("echo addr | base64" encodes "addr\n"), is valid UTF-8 of
//     minBase64Text to maxBase64Text bytes and every rune in it is printable
//     (unicode.IsPrint, which admits the ASCII space and nothing else that is
//     not a graphic character).
//
// A random token of that alphabet decodes to bytes, not text, and fails the
// second rule. Measured over 100,000 random alphanumeric tokens at each
// length (the T-0403 landing): 0.8% of eight-character tokens decode to
// printable text, 0.07% of twelve-character ones, 0.016% of sixteen and none
// from twenty-four up — and of all 600,000, one then parsed as anything a
// validator here reads. The bounds are the cost bound: one decode of at most
// maxBase64Len bytes, never repeated on its own output.
const (
	minBase64Text = 6   // "a@b.co", the shortest address ValidEmail accepts
	maxBase64Text = 320 // validEmail's own cap, the longest value any parser here reads
	maxBase64Len  = (maxBase64Text+2)/3*4 + 2
)

// base64Text returns v decoded from base64 when the decode is plausible text,
// by the rules above.
func base64Text(v string) (string, bool) {
	// The raw bound first, so the whitespace pass is bounded too. No encoder
	// writes more than one two-byte line break per 64 characters, so twice
	// the encoded bound is generous.
	if len(v) > 2*maxBase64Len {
		return "", false
	}
	v = strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '\r', '\n':
			return -1
		}
		return r
	}, v)
	if len(v) < 8 || len(v) > maxBase64Len {
		return "", false
	}
	std, urlSafe, pad := false, false, 0
	for i := 0; i < len(v); i++ {
		c := v[i]
		switch {
		case c == '=':
			pad++
			continue
		case (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9'):
		case c == '+' || c == '/':
			std = true
		case c == '-' || c == '_':
			urlSafe = true
		default:
			return "", false
		}
		if pad > 0 {
			// Padding is only ever at the end.
			return "", false
		}
	}
	if (std && urlSafe) || pad > 2 {
		return "", false
	}
	body := v[:len(v)-pad]
	if len(body)%4 == 1 || (pad > 0 && len(v)%4 != 0) {
		return "", false
	}
	enc := base64.RawStdEncoding
	if urlSafe {
		enc = base64.RawURLEncoding
	}
	b, err := enc.DecodeString(body)
	if err != nil || len(b) < minBase64Text || len(b) > maxBase64Text || !utf8.Valid(b) {
		return "", false
	}
	text := strings.TrimRight(string(b), "\r\n")
	if len(text) < minBase64Text {
		return "", false
	}
	for _, r := range text {
		if !unicode.IsPrint(r) {
			return "", false
		}
	}
	return text, true
}

// Punctuation between the groups. collapseGroups (candidates.go) joins a value
// grouped with spaces; the red team grouped a card with en dashes
// ("4111–1111–1111–1111") and with underscores, and ValidLuhn's separator set
// knows neither. collapseSeparatedGroups joins a value grouped with any mix of
// whitespace, a dash (Unicode Pd, the ASCII hyphen included, and U+2212), an
// underscore or a slash, under collapseGroups' own four-character bound; and
// hyphenatedGroups writes the same separators as the ASCII hyphen, because a
// US Social Security number is a shape only with its hyphens ("078–05–1120"
// joined is nine bare digits, which ValidNationalIDStructured does not read).

// isGroupSeparator is the punctuation a written identifier is grouped with.
func isGroupSeparator(r rune) bool {
	switch r {
	case '-', '_', '/', '\u2212':
		return true
	}
	return r >= utf8.RuneSelf && unicode.Is(unicode.Pd, r)
}

// hasGroupSeparator is the gate in front of the two functions below: a value
// carrying a group separator and a digit. Every shape they are for — a card,
// an IBAN, a national identifier, a phone number — has digits.
func hasGroupSeparator(s string) bool {
	sep, digit := false, false
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			digit = true
		case isGroupSeparator(r):
			sep = true
		}
	}
	return sep && digit
}

// collapseSeparatedGroups is collapseGroups over punctuation as well as
// whitespace. A value that is a calendar date written with separators
// ("2026-09-24", "24/09/2026") is not collapsed: eight bare digits is a length
// two checksum-only national identifier formats read (nationalid.go's TFN and
// BSN), and a date column would clear them by chance about one time in ten,
// which is the shape looksLikePlausibleDate already refuses for the digits
// reading. A date grouped with spaces is collapsed by collapseGroups as it
// always was.
func collapseSeparatedGroups(s string) (string, bool) {
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return unicode.IsSpace(r) || isGroupSeparator(r)
	})
	if len(fields) < 2 || separatedDate(fields) {
		return "", false
	}
	for _, f := range fields {
		if len(f) > maxGroupLen {
			return "", false
		}
	}
	return strings.Join(fields, ""), true
}

// separatedDate reports three digit groups that read as a calendar date, year
// first or year last: a four-digit year from 1800 to 2199 and two groups of
// one or two digits, one of which is a month.
func separatedDate(fields []string) bool {
	if len(fields) != 3 {
		return false
	}
	for _, f := range fields {
		if !allDigits(f) {
			return false
		}
	}
	var year string
	var a, b string
	switch {
	case len(fields[0]) == 4 && len(fields[1]) <= 2 && len(fields[2]) <= 2:
		year, a, b = fields[0], fields[1], fields[2]
	case len(fields[2]) == 4 && len(fields[0]) <= 2 && len(fields[1]) <= 2:
		year, a, b = fields[2], fields[0], fields[1]
	default:
		return false
	}
	y, m, d := digitValue(year), digitValue(a), digitValue(b)
	if y < 1800 || y > 2199 || m < 1 || d < 1 || m > 31 || d > 31 {
		return false
	}
	return m <= 12 || d <= 12
}

// digitValue is the value of a run of ASCII digits separatedDate has already
// checked with allDigits; it is at most four long, so it cannot overflow.
func digitValue(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		n = n*10 + int(s[i]-'0')
	}
	return n
}

// hyphenatedGroups writes every underscore, slash and non-ASCII dash between
// two groups as the ASCII hyphen. It is offered only when it changes the
// value.
func hyphenatedGroups(s string) (string, bool) {
	h := strings.Map(func(r rune) rune {
		if r != '-' && isGroupSeparator(r) {
			return '-'
		}
		return r
	}, s)
	if h == s {
		return "", false
	}
	return h, true
}
