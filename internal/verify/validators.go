// SPDX-License-Identifier: Apache-2.0

package verify

import (
	"math"
	"net"
	"net/mail"
	"regexp"
	"strings"
	"unicode"

	"github.com/nyaruka/phonenumbers"

	"github.com/Liarea/lazyslice/internal/pipeline"
)

// The second net's validators (ARCHITECTURE.md section 6 item 4).
//
// They are a copy of the dictionary-free ones in internal/classify/validators.go,
// which is another stage package and therefore not importable from here
// (internal/CLAUDE.md). The copy is deliberate and it is recorded in
// internal/verify/CLAUDE.md: the alternative was a second *rule pack* — name
// patterns, dictionaries, scoring — which internal/transform already refused for
// the same reason, and this is the smaller half. What is copied is the
// value-only half: a validator is a pure function over one string and nothing
// here reads a name or a neighbour.
//
// Three consequences follow and are stated rather than hidden. The net can only
// find what a validator recognises, which is what THREAT_MODEL.md T1 already
// says about the classifier. It is *narrower* than the classifier by exactly the
// two dictionary-backed validators — see the note on `validators` below, which
// is the coverage this net actually has. And a change to
// internal/classify/validators.go has to be made here too, or the net stops
// agreeing with the classifier it is a second look at; the fix is a shared home
// for both, not a third copy.

// validatorThreshold is section 4's "at least 80% of non-null samples
// validating", applied here over the whole column rather than over 200 samples:
// the target is small, so this is a scan (section 6 item 4).
const validatorThreshold = 0.8

// minValues is how many non-NULL values a column needs before a ratio over it
// means anything. One row that parses as an address is evidence about a row.
const minValues = 3

// validator is one category's value signal.
type validator struct {
	category pipeline.Category
	// name is what the report says the column validated as. It is a fixed
	// identifier, never a value.
	name string
	// text is true when the validator runs over character columns.
	text bool
	// digits is true when it runs over integer, bigint and numeric columns.
	digits bool
	ok     func(string) bool
}

// validators is the set: eight of internal/classify's ten value validators, in
// its own order, folded into six entries (its two financial validators share
// one here, and so do its two network ones).
//
// The two that are missing, and why. `person_name` (looksLikeName) and
// `free_text` (prose) are *not* missing because this net never sees a column
// name — both are value validators over a sampled string, exactly like the six
// here. They are missing because both read internal/classify's embedded name
// dictionary, which is a rule pack: this package may not import
// internal/classify (internal/CLAUDE.md) and will not carry a second copy of a
// dictionary, for the reason internal/transform refused a second rule pack.
// That is a real hole in the largest personal-data category — an unmasked
// column of real person names, or a notes column carrying other rows' names
// (testdata/README.md trap 17), passes this net — and it halves one of
// THREAT_MODEL.md T1's two controls. It is recorded in
// internal/verify/CLAUDE.md under "Decisions made during implementation" and
// returned to the orchestrator by this task. There is no tracker task for it
// yet and this comment does not claim one: the fix is the shared home the same
// file already owes for everything else in this file, with the dictionary in
// it, and until that task is opened and lands the hole stands as written here.
var validators = []validator{
	{category: pipeline.CatEmail, name: "email", text: true, ok: validEmail},
	{category: pipeline.CatPhone, name: "phone", text: true, ok: validPhone},
	{category: pipeline.CatNetworkID, name: "network_id", text: true, ok: func(s string) bool {
		return validIP(s) || validMAC(s)
	}},
	{category: pipeline.CatFinancial, name: "financial_account", text: true, digits: true, ok: func(s string) bool {
		return validLuhn(s) || validIBAN(s)
	}},
	{category: pipeline.CatCredential, name: "credential", text: true, ok: looksSecret},
	{category: pipeline.CatAddress, name: "address", text: true, ok: addressShape},
}

var (
	uuidRE = regexp.MustCompile(`\A[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\z`)
	hexRE  = regexp.MustCompile(`\A[0-9a-fA-F]+\z`)
)

// validEmail is net/mail.ParseAddress, tightened: the domain must carry a dot,
// or every hostname-shaped identifier in a database satisfies it.
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

// phoneRegionHint is the unknown region, as internal/classify uses: only a
// number written in international form validates.
const phoneRegionHint = "ZZ"

func validPhone(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 40 {
		return false
	}
	num, err := phonenumbers.Parse(s, phoneRegionHint)
	if err != nil {
		return false
	}
	return phonenumbers.IsValidNumber(num)
}

func validIP(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	if i := strings.IndexByte(s, '/'); i >= 0 {
		s = s[:i]
	}
	return net.ParseIP(s) != nil
}

func validMAC(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	_, err := net.ParseMAC(s)
	return err == nil
}

func validUUID(s string) bool { return uuidRE.MatchString(strings.TrimSpace(s)) }

// validLuhn is the payment-card check digit, bounded to the twelve to nineteen
// digits ISO/IEC 7812 allows.
func validLuhn(s string) bool {
	digits := make([]int, 0, 20)
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			digits = append(digits, int(r-'0'))
		case r == ' ' || r == '-':
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

// validIBAN is the mod-97 check.
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
	rearranged := s[4:] + s[:4]
	rem := 0
	for _, r := range rearranged {
		if r >= '0' && r <= '9' {
			rem = rem*10 + int(r-'0')
		} else {
			rem = rem*100 + int(r-'A') + 10
		}
		rem %= 97
	}
	return rem == 1
}

// looksSecret is the Shannon-entropy check, run after the UUID check exactly as
// section 4 orders them.
func looksSecret(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 16 || len(s) > 512 {
		return false
	}
	if validUUID(s) || strings.ContainsAny(s, " \t\n@") {
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

// addressShape is "mixed digits and words": a street line carries a number and
// at least two words.
func addressShape(s string) bool {
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
