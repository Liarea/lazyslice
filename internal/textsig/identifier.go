// SPDX-License-Identifier: Apache-2.0

package textsig

import (
	"regexp"
	"strings"
)

// The identifier shapes (tracker T-0311). Each answers whether one value is a
// machine identifier an application reads back verbatim — a uuid, a hex
// digest, a filesystem or route path, a hostname, a semantic version — and
// none of them is a validator for a personal-data category: nothing masks a
// column because a value matches one. internal/classify reads them for the
// opposite question, whether a signal-less column beside a certain personal
// column may be *spared* the neighbouring-column sweep into free_text, and
// that caller owns the ratio, the minimum count and the dictionary guard over
// the two shapes (path, hostname) that are made of words. internal/verify does
// not call them, so nothing here narrows its second net.
//
// ValidUUID, the fifth shape, is in textsig.go: LooksSecret already reads it.

var (
	// semverRE is semver.org 2.0.0's own grammar for a version core with an
	// optional pre-release and build suffix, plus the leading "v" git tags and
	// most package managers write.
	semverRE = regexp.MustCompile(`\Av?(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)` +
		`(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?\z`)

	// hostLabelRE is one RFC 1123 label: letters, digits and interior hyphens,
	// at most 63 characters.
	hostLabelRE = regexp.MustCompile(`\A[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?\z`)

	// pathRE is the character set a path or an application route is written
	// in, with no whitespace, no "@" and no quote: a value outside it is not
	// being spared as a path whatever else it looks like.
	pathRE = regexp.MustCompile(`\A[A-Za-z0-9._~%+=,:/-]+\z`)
)

// hostSuffixes are the final labels a two-label hostname is accepted with.
// Two dot-separated words are also what a "first.last" username looks like,
// so a bare two-label value is a hostname only when its last label is one of
// these; three labels or more is a hostname on its shape alone.
var hostSuffixes = map[string]bool{
	"com": true, "net": true, "org": true, "io": true, "dev": true, "app": true,
	"cloud": true, "local": true, "localhost": true, "internal": true, "lan": true,
	"corp": true, "home": true, "test": true, "example": true, "invalid": true,
}

// SemanticVersion reports whether s is a semantic version: "1.4.2",
// "v2.0.0-rc.1", "3.1.0+build.7". A version core that also reads as a dotted
// calendar date with a four-digit year ("5.3.1985", "12.11.1979",
// "1985.3.5") is refused: semver's grammar accepts an unpadded date of birth,
// and a column of them was spared as versions and copied (the T-0311 review).
// A two-digit year ("5.3.85") is indistinguishable from a version and is
// THREAT_MODEL.md T1's T-0311 residual.
func SemanticVersion(s string) bool {
	s = strings.TrimSpace(s)
	if !semverRE.MatchString(s) {
		return false
	}
	core := strings.TrimPrefix(s, "v")
	if i := strings.IndexAny(core, "-+"); i >= 0 {
		core = core[:i]
	}
	return !DottedDate(core)
}

// DottedDate reports whether s is three all-digit parts joined by one
// separator ('.', '-' or '/') that read as a calendar date with a four-digit
// year between 1000 and 2999: day-month-year, month-day-year or
// year-month-day. It is a guard, not a validator: nothing masks a column
// because a value matches it, and internal/classify reads it only to refuse a
// spare.
func DottedDate(s string) bool {
	s = strings.TrimSpace(s)
	var sep string
	for _, c := range []string{".", "-", "/"} {
		if strings.Count(s, c) == 2 {
			sep = c
			break
		}
	}
	if sep == "" {
		return false
	}
	parts := strings.Split(s, sep)
	n := make([]int, 3)
	for i, p := range parts {
		if p == "" || len(p) > 4 {
			return false
		}
		for j := 0; j < len(p); j++ {
			if p[j] < '0' || p[j] > '9' {
				return false
			}
			n[i] = n[i]*10 + int(p[j]-'0')
		}
	}
	year := func(v int) bool { return v >= 1000 && v <= 2999 }
	day := func(v int) bool { return v >= 1 && v <= 31 }
	month := func(v int) bool { return v >= 1 && v <= 12 }
	switch {
	case year(n[2]) && day(n[0]) && day(n[1]) && (month(n[0]) || month(n[1])):
		return true // day-month-year or month-day-year
	case year(n[0]) && month(n[1]) && day(n[2]):
		return true // year-month-day
	}
	return false
}

// HexDigest reports whether s is a run of hexadecimal digits of the length a
// digest or an abbreviated commit hash takes: eight to 128 characters, with at
// least one decimal digit and at least one letter a to f. The mixture is what
// keeps an all-digit number (an account, a national identifier) and an
// all-letter word ("facade", "deface") out of it.
func HexDigest(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 8 || len(s) > 128 {
		return false
	}
	digit, letter := false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
			digit = true
		case (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F'):
			letter = true
		default:
			return false
		}
	}
	return digit && letter
}

// HostnameShape reports whether s is a DNS hostname: RFC 1123 labels joined by
// dots, a final label of letters only, and either three labels or more or a
// final label from hostSuffixes. It says nothing about whether any label is a
// person's name ("gareths-laptop.local"); that guard is the caller's, because
// this package's dictionary signal is the caller's to score.
func HostnameShape(s string) bool {
	s = strings.TrimSuffix(strings.TrimSpace(s), ".")
	if s == "" || len(s) > 253 {
		return false
	}
	labels := strings.Split(s, ".")
	if len(labels) < 2 {
		return false
	}
	for _, l := range labels {
		if !hostLabelRE.MatchString(l) {
			return false
		}
	}
	last := labels[len(labels)-1]
	for i := 0; i < len(last); i++ {
		c := last[i]
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') {
			return false
		}
	}
	if len(last) < 2 {
		return false
	}
	return len(labels) >= 3 || hostSuffixes[strings.ToLower(last)]
}

// tzAreas are the IANA time zone database's top-level areas, the first
// segment of an "Area/Location" zone name.
var tzAreas = map[string]bool{
	"Africa": true, "America": true, "Antarctica": true, "Arctic": true, "Asia": true,
	"Atlantic": true, "Australia": true, "Europe": true, "Indian": true, "Pacific": true,
	"Etc": true,
}

// monthNames are the English month names and their three-letter
// abbreviations, lower case: a slashed date written with one ("05/Mar/1985")
// is not a path.
var monthNames = map[string]bool{
	"jan": true, "feb": true, "mar": true, "apr": true, "may": true, "jun": true,
	"jul": true, "aug": true, "sep": true, "sept": true, "oct": true, "nov": true, "dec": true,
	"january": true, "february": true, "march": true, "april": true, "june": true,
	"july": true, "august": true, "september": true, "october": true, "november": true,
	"december": true,
}

// fileExtRE is a final segment ending in a file extension: a name, a dot,
// and one to five alphanumerics, of which hasFileExt also wants one a letter.
var fileExtRE = regexp.MustCompile(`[^./][.]([0-9A-Za-z]{1,5})\z`)

// hasFileExt reports whether seg ends in a file extension ("banner.png",
// "app.min.js", "clip.mp4"), and not in an all-digit suffix ("1.2").
func hasFileExt(seg string) bool {
	m := fileExtRE.FindStringSubmatch(seg)
	if m == nil {
		return false
	}
	for i := 0; i < len(m[1]); i++ {
		c := m[1][i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			return true
		}
	}
	return false
}

// PathShape reports whether s is a filesystem path or an application route:
// "/images/banner.png", "assets/css/app.css", "/api/v1/users",
// "America/New_York". It needs no "://" (a URL is ValidURL's, under
// online_id), no whitespace and no "@", at least one segment with a letter in
// it (so an all-digit slashed date or a fraction is never a path), no segment
// that is a month name beside all-digit ones ("05/Mar/1985"), and one of three
// application-path markers: a final segment with a file extension, a leading
// "/" with three segments or more, or an IANA "Area/Location" zone name. A
// bare "/home/jsmith" or "admin/users" has none of them and is not a path
// here (the T-0311 review). Like HostnameShape it does not ask whether a
// segment names a person ("/home/gareth/src"); the caller does.
func PathShape(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 1024 || !strings.Contains(s, "/") || strings.Contains(s, "://") {
		return false
	}
	if !pathRE.MatchString(s) {
		return false
	}
	segs := strings.Split(s, "/")
	letter, month, other := false, false, false
	var nonEmpty []string
	for _, seg := range segs {
		if seg == "" {
			continue
		}
		nonEmpty = append(nonEmpty, seg)
		hasLetter := false
		for i := 0; i < len(seg); i++ {
			c := seg[i]
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
				hasLetter = true
				break
			}
		}
		switch {
		case monthNames[strings.ToLower(seg)]:
			month = true
		case hasLetter:
			other = true
		}
		letter = letter || hasLetter
	}
	if !letter || (month && !other) || len(nonEmpty) == 0 {
		return false
	}
	switch {
	case hasFileExt(nonEmpty[len(nonEmpty)-1]):
		return true
	case strings.HasPrefix(s, "/") && len(nonEmpty) >= 3:
		return true
	case len(nonEmpty) >= 2 && tzAreas[nonEmpty[0]]:
		return true
	}
	return false
}
