// SPDX-License-Identifier: Apache-2.0

package classify

import (
	"regexp"
	"strings"

	"github.com/Liarea/lazyslice/internal/textsig"
)

// What spares a signal-less column from the neighbouring-column sweep
// (tracker T-0311, dogfood session 1).
//
// unknownColumnsBesideCertain masks a character column with no name, value or
// type signal as free_text when its table holds a column at `certain` under a
// person-identifying category. On a production Rails schema of 143 tables that
// swept every such column of a users, devices or ads table — `role`,
// `ui_mode`, `os_type`, `state`, `log_level`, `timezone`, a text uuid, colours,
// asset paths — into 255-character word salad where the application expects
// 'admin' or 'linux', and the copy did not boot. Those columns are not
// signal-less in the sense the sweep was written for (a free-text column
// nobody could look inside): their samples say plainly what they are, and it
// is not a person.
//
// So a column is spared, and its reason line says why, when every one of its
// non-NULL samples is one identifier shape, or when the samples are an
// enumeration. Both are claims about *all* the samples, never a ratio, and
// both give way to the dictionary: a value that carries a dictionary name, a
// special-category term or a gender term is never spared on either shape (and
// an enumeration also refuses blood groups, marital status, dates and runs of
// four digits or more, separators allowed, the T-0311 review's guards),
// because "when in doubt, mask it" (CLAUDE.md) and the sweep is exactly the
// doubt. The third skip the task names, a unique index, is the one
// raisableUnknown already made silently; unknownColumnsBesideCertain now
// prints it.
//
// This narrows recall on a real shape and is THREAT_MODEL.md T1's to answer
// for; that row's T-0311 amendment carries the measurement and the residual.

// enumMinSamples is how many non-NULL samples an enumeration needs before
// the samples can call it one. Below it, "each value seen at least twice" is
// two rows of the same value, which is a coincidence and not a domain.
const enumMinSamples = 10

// enumMaxDistinct is the most distinct values an enumeration may have. A
// role, a state machine, a log level, an OS family, a UI mode and an
// orientation are each under ten; a timezone column over a real user base is
// under twenty in a 200-row sample.
const enumMaxDistinct = 20

// enumTokenMaxLen is the longest value an enumeration member may be.
const enumTokenMaxLen = 64

// enumTokenRE is the shape of an enumeration member: ASCII, no whitespace, the
// characters an application's own constants are written in. A whitespace
// excludes every "Given Surname" value, and ASCII excludes every native-script
// name, which are the two shapes the round-4 and round-5 red teams used to get
// a name past the dictionary; a repeated one of either is still swept.
var enumTokenRE = regexp.MustCompile(`\A[A-Za-z0-9#][A-Za-z0-9_.:/#+-]*\z`)

// genderTerms are values that describe a person's sex, gender or title. They
// are enumerations by every measure above and they are personal data under
// any column name, so a column holding one is never spared as an enum.
var genderTerms = map[string]bool{
	"m": true, "f": true, "x": true, "male": true, "female": true, "man": true,
	"woman": true, "men": true, "women": true, "nonbinary": true, "non-binary": true,
	"mr": true, "mrs": true, "ms": true, "miss": true, "mx": true,
}

// attributeTerms are low-cardinality personal attributes an enumeration would
// otherwise spare under a neutral column name (the T-0311 review): ABO/Rh
// blood groups, which are health data and a special category, and marital
// status. Like genderTerms a column holding one is never spared as an enum.
// The list is closed and short on purpose; an attribute outside it is
// THREAT_MODEL.md T1's T-0311 residual.
var attributeTerms = map[string]bool{
	"a+": true, "a-": true, "b+": true, "b-": true, "ab+": true, "ab-": true,
	"o+": true, "o-": true, "0+": true, "0-": true, "apos": true, "aneg": true,
	"bpos": true, "bneg": true, "abpos": true, "abneg": true, "opos": true, "oneg": true,
	"married": true, "single": true, "divorced": true, "widowed": true, "separated": true,
	"engaged": true, "unmarried": true, "partnered": true, "cohabiting": true,
	"civil_partnership": true, "domestic_partner": true,
}

// monthAbbrevs are the lower-case English month names an enumeration member
// written as a date carries ("05-Mar-1985").
var monthAbbrevs = map[string]bool{
	"jan": true, "feb": true, "mar": true, "apr": true, "may": true, "jun": true,
	"jul": true, "aug": true, "sep": true, "sept": true, "oct": true, "nov": true, "dec": true,
	"january": true, "february": true, "march": true, "april": true, "june": true,
	"july": true, "august": true, "september": true, "october": true, "november": true,
	"december": true,
}

// datelike reports whether an enumeration member reads as a date: a dotted,
// dashed or slashed date with a four-digit year (textsig.DottedDate), an ISO
// date or timestamp ("1985-03-05", "1985-03-05T10:00:00"), a dashed or
// slashed date with a two-digit year ("05/03/85"), or one written with a
// month name ("05-Mar-1985"). A dotted "1.2.3" is left to be a version.
func datelike(v string) bool {
	if textsig.DottedDate(v) || isoDateRE.MatchString(v) {
		return true
	}
	for _, sep := range []string{"-", "/"} {
		parts := strings.Split(v, sep)
		if len(parts) != 3 {
			continue
		}
		digits, month := 0, 0
		for _, p := range parts {
			switch {
			case p != "" && len(p) <= 4 && strings.Trim(p, "0123456789") == "":
				digits++
			case monthAbbrevs[strings.ToLower(p)]:
				month++
			}
		}
		if digits == 3 || (digits == 2 && month == 1) {
			return true
		}
	}
	return false
}

// digitRun reports whether an enumeration member is only digits and the
// separators '-', '.', '/' and '+', with four digits or more in all: a bare
// postcode or account ("94105"), a ZIP+4 ("94105-1234"), a local phone number
// ("555-1234") or a dialled one ("+44-20-7946-0000"). datelike only answers
// for three parts, so a two-part digit run needs its own guard (the T-0311
// review's second round). A dotted "1.2.3" has three digits and is left to be
// a version.
func digitRun(v string) bool {
	if v == "" || strings.Trim(v, "0123456789-./+") != "" {
		return false
	}
	n := 0
	for i := 0; i < len(v); i++ {
		if v[i] >= '0' && v[i] <= '9' {
			n++
		}
	}
	return n >= 4
}

// isoDateRE is an ISO 8601 calendar date at the start of a value.
var isoDateRE = regexp.MustCompile(`\A[0-9]{4}-[0-9]{2}-[0-9]{2}(?:\z|T)`)

// Identifier-shape phrases, the closed vocabulary the spared_identifier
// fragment draws on (reasons.go), in the order a column is tested against
// them.
const (
	shapeUUID    = "uuids"
	shapeHex     = "hex digests"
	shapeSemver  = "semantic versions"
	shapeHost    = "hostnames"
	shapePath    = "paths"
	shapeNothing = ""
)

// identifierShapes is every identifier shape a column can be spared on.
// named is set on the two shapes made of words, which are the two a person's
// name can hide inside ("gareths-laptop.local", "/home/gareth").
var identifierShapes = []struct {
	phrase string
	ok     func(string) bool
	named  bool
}{
	{shapeUUID, textsig.ValidUUID, false},
	{shapeHex, textsig.HexDigest, false},
	{shapeSemver, textsig.SemanticVersion, false},
	{shapeHost, textsig.HostnameShape, true},
	{shapePath, textsig.PathShape, true},
}

// identifierPhrases is the alternation reasons.go accepts.
var identifierPhrases = []string{shapeUUID, shapeHex, shapeSemver, shapeHost, shapePath}

// spareShape is what base() found about a signal-less character column's
// samples, for unknownColumnsBesideCertain to read. Its zero value spares
// nothing.
type spareShape struct {
	// identifier is the phrase of the one identifier shape every sample
	// matched, or "".
	identifier string
	// enum is set when the samples are an enumeration.
	enum bool
	// distinct and total are the counts the reason line names.
	distinct, total int
}

// sparedBy reports what, if anything, spares a column with these non-NULL
// samples from the sweep. An identifier shape is tried first: it is the more
// specific claim, and a column of three repeated uuids is better described as
// uuids than as an enumeration of them.
func sparedBy(dict *textsig.Dict, values []string) spareShape {
	sp := spareShape{total: len(values)}
	if len(values) < minSamples {
		return sp
	}
	if phrase := identifierShape(dict, values); phrase != shapeNothing {
		sp.identifier = phrase
		return sp
	}
	if n, ok := enumeration(dict, values); ok {
		sp.enum, sp.distinct = true, n
	}
	return sp
}

// identifierShape returns the phrase of the first identifier shape every
// value matches, or shapeNothing. minSamples is the floor, as for any value
// signal here: two uuids are two rows, not a column.
func identifierShape(dict *textsig.Dict, values []string) string {
	for _, shape := range identifierShapes {
		all := true
		for _, v := range values {
			if !shape.ok(v) || (shape.named && carriesAPerson(dict, v)) {
				all = false
				break
			}
		}
		if all {
			return shape.phrase
		}
	}
	return shapeNothing
}

// enumeration reports whether the values are an enumeration and how many
// distinct values it has: at least enumMinSamples of them, at most
// enumMaxDistinct distinct, every distinct value seen at least twice, and
// every value an enumTokenRE token carrying no person, no gender or
// attribute term, not a run of four digits or more (a postcode, a ZIP+4, a
// phone number, an account or a year; see digitRun) and not a date. Sampling is TABLESAMPLE SYSTEM, whole pages, so
// rows inserted together share values and "each seen twice" is easier for a
// person-valued column to meet than a uniform sample would make it; the
// guards are what answer for that, and THREAT_MODEL.md T1 states what they
// miss.
func enumeration(dict *textsig.Dict, values []string) (int, bool) {
	if len(values) < enumMinSamples {
		return 0, false
	}
	counts := map[string]int{}
	for _, v := range values {
		v = strings.TrimSpace(v)
		if len(v) > enumTokenMaxLen || !enumTokenRE.MatchString(v) || carriesAPerson(dict, v) ||
			genderTerms[strings.ToLower(v)] || attributeTerms[strings.ToLower(v)] ||
			digitRun(v) || datelike(v) {
			return 0, false
		}
		counts[v]++
		if len(counts) > enumMaxDistinct {
			return 0, false
		}
	}
	for _, n := range counts {
		if n < 2 {
			return 0, false
		}
	}
	return len(counts), true
}

// carriesAPerson is the guard every spare gives way to: a dictionary name as
// a word inside the value, or a special-category term. A CamelCase value is
// split at each lower-to-upper boundary first, so "JohnSmith" is read as
// "John Smith" and not as one unknown word (the T-0311 review).
func carriesAPerson(dict *textsig.Dict, v string) bool {
	if split := splitCamel(v); split != v && dict.ContainsName(split) {
		return true
	}
	return dict.ContainsName(v) || textsig.SpecialCategoryVocabulary(v)
}

// splitCamel inserts a space at every ASCII lower-to-upper case boundary.
func splitCamel(v string) string {
	var b strings.Builder
	for i := 0; i < len(v); i++ {
		c := v[i]
		if i > 0 && c >= 'A' && c <= 'Z' && v[i-1] >= 'a' && v[i-1] <= 'z' {
			b.WriteByte(' ')
		}
		b.WriteByte(c)
	}
	return b.String()
}
