// SPDX-License-Identifier: Apache-2.0

package classify

import (
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/mail"
	"reflect"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/nyaruka/phonenumbers"

	"github.com/Liarea/lazyslice/internal/pipeline"
)

// The validators ARCHITECTURE.md §4 names, each over one sampled value. They
// are pure and they never see a database: samples arrive through
// pipeline.Sampler, taken at introspect.
//
// A validator returns a verdict on one value. The scorer counts verdicts over
// the non-NULL samples and compares the ratio against validatorThreshold; no
// validator ever reaches a reason string, only the count and the fixed phrase
// that names it.

// validatorThreshold is ARCHITECTURE.md §4's "≥80% of non-null samples
// validating". It is the same number on both branches of the scoring rule: a
// name hit plus this ratio is `certain`, and this ratio with no name hit is
// `likely`.
const validatorThreshold = 0.8

// weakThreshold is where a value signal stops being noise and starts being a
// reason to look at the column's neighbours. ARCHITECTURE.md §4 does not name
// it; see internal/classify/CLAUDE.md, "Decisions made during implementation".
const weakThreshold = 0.5

// minSamples is how many non-NULL samples a value signal needs before it can
// decide anything on its own. One row that happens to parse as an address is
// not evidence about a column; it is evidence about a row.
const minSamples = 3

// uuidRE is the UUID check ARCHITECTURE.md §4 puts *before* the entropy check
// for secrets: a column of UUIDs is high-entropy and is not a credential.
var uuidRE = regexp.MustCompile(`\A[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\z`)

var hexRE = regexp.MustCompile(`\A[0-9a-fA-F]+\z`)

// validEmail is net/mail.ParseAddress, tightened. ParseAddress accepts "a@b",
// which every hostname-shaped identifier in a database would satisfy, so the
// domain must also carry a dot.
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

// validPhone is libphonenumber's IsValidNumber. The region hint is "ZZ", the
// unknown region, so only a number written in international form validates.
// That is the conservative half of the rule: a national-format column reaches
// the classifier through its name, at `possible`, and is masked anyway.
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

// phoneRegionHint is the libphonenumber region passed to Parse. See
// internal/classify/CLAUDE.md, "Decisions made during implementation": v1 uses
// the unknown region rather than deriving one from a sibling country column.
const phoneRegionHint = "ZZ"

func validIP(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	if i := strings.IndexByte(s, '/'); i >= 0 { // inet renders a prefix length
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

// validLuhn is the payment-card check digit. It runs only on a value that is
// twelve to nineteen digits after separators are removed, which is the range
// ISO/IEC 7812 allows; without that bound every even-length numeric identifier
// passes it about half the time.
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

// looksSecret is the Shannon-entropy check, run after the UUID check exactly as
// ARCHITECTURE.md §4 orders them. The guards before the entropy are what stop
// an email address or a sentence from reading as a secret: a credential has no
// whitespace, no "@", and mixes character classes or is long hex.
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

// addressShape is ARCHITECTURE.md §10's "mixed digits and words": a street line
// carries a number and at least two words.
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

// twoLetterCode is the low-confidence value shape ARCHITECTURE.md §10 records
// for a column of ISO codes: it explains a column rather than masking one.
func twoLetterCode(s string) bool {
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

// ---------- turning a sampled value into strings ----------

// scalars reduces one sampled value to the strings the validators run over. An
// array yields its elements, because ARCHITECTURE.md §4 classifies an array on
// its element type; a NULL yields nothing at all, because the ratio is over the
// non-NULL samples.
func scalars(v any) []string {
	if v == nil {
		return nil
	}
	if elems, ok := arrayElements(v); ok {
		out := make([]string, 0, len(elems))
		for _, e := range elems {
			out = append(out, scalars(e)...)
		}
		return out
	}
	if s, ok := asText(v); ok {
		return []string{s}
	}
	return nil
}

// arrayElements reports the elements of an array value. A []byte is bytea, not
// an array, and a string is not a sequence for this purpose.
func arrayElements(v any) ([]any, bool) {
	switch t := v.(type) {
	case []byte, string:
		return nil, false
	case []any:
		return t, true
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return nil, false
	}
	out := make([]any, 0, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		out = append(out, rv.Index(i).Interface())
	}
	return out, true
}

// asText renders one scalar. It is deliberately narrow: a type it does not know
// yields nothing rather than a Go rendering of a struct, which no validator
// could read and which could carry a production value into a comparison it was
// never meant for.
func asText(v any) (string, bool) {
	switch t := v.(type) {
	case nil:
		return "", false
	case string:
		return t, true
	case []byte:
		return string(t), true
	case bool:
		return fmt.Sprintf("%t", t), true
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", t), true
	case float32, float64:
		return fmt.Sprintf("%v", t), true
	case time.Time:
		return t.Format(time.RFC3339), true
	case fmt.Stringer:
		return t.String(), true
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return "", false
		}
		return asText(rv.Elem().Interface())
	}
	return "", false
}

// ---------- JSON ----------

// jsonLeaf is one scalar leaf of a sampled JSON document, with the key that
// names it. ARCHITECTURE.md §4 masks a document leaf by leaf, choosing each
// string leaf's masker by running its key through the name rules; the
// classifier's job here is narrower — deciding whether the document carries
// personal data at all.
type jsonLeaf struct {
	Key   string
	Value string
}

// jsonLeaves walks a sampled JSON value. A document that does not parse yields
// nothing, which leaves the column classified on its type alone: json and jsonb
// are type signals in their own right, so an unparseable document is still
// masked.
func jsonLeaves(v any) []jsonLeaf {
	var doc any
	switch t := v.(type) {
	case nil:
		return nil
	case []byte:
		if json.Unmarshal(t, &doc) != nil {
			return nil
		}
	case string:
		if json.Unmarshal([]byte(t), &doc) != nil {
			return nil
		}
	default:
		doc = v
	}
	var out []jsonLeaf
	walkJSON("", doc, &out, 0)
	return out
}

const jsonMaxDepth = 16

func walkJSON(key string, v any, out *[]jsonLeaf, depth int) {
	if depth > jsonMaxDepth || len(*out) > 4096 {
		return
	}
	switch t := v.(type) {
	case map[string]any:
		for k, child := range t {
			walkJSON(k, child, out, depth+1)
		}
	case []any:
		for _, child := range t {
			walkJSON(key, child, out, depth+1)
		}
	case string:
		*out = append(*out, jsonLeaf{Key: key, Value: t})
	case nil:
	default:
		if s, ok := asText(v); ok {
			*out = append(*out, jsonLeaf{Key: key, Value: s})
		}
	}
}

// jsonLeafIsPersonal reports whether a leaf carries personal data, by the same
// rules the classifier applies to a column: the leaf's key through the name
// rules, or the leaf's value through the validators.
func jsonLeafIsPersonal(p *compiledPack, leaf jsonLeaf) bool {
	if leaf.Key != "" {
		if pat, ok := p.match(normaliseName(leaf.Key)); ok && pat.Category != pipeline.CatFreeText {
			return true
		}
	}
	return validEmail(leaf.Value) || validPhone(leaf.Value) || validIP(leaf.Value) ||
		validIBAN(leaf.Value) || validLuhn(leaf.Value)
}
