// SPDX-License-Identifier: Apache-2.0

package mask

import (
	"encoding/json"
	"net/netip"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	pn "github.com/nyaruka/phonenumbers"
)

// Every masked value must still be a value the column and the application
// accept: ARCHITECTURE.md §5 "preserve what the application checks". These are
// the shape assertions, one per generator, over several keys so that a shape
// that only holds for one digest fails here rather than at load.
func TestFormatPreservationPerCategory(t *testing.T) {
	keys := []Key{testKey(t), otherKey(t)}
	checks := map[string]func(*testing.T, string){
		"email varchar":      wantMatch(`^[a-z]+\.[a-z]+@example\.(com|net|org)$`),
		"email unique":       wantMatch(`^[a-z]+\.[a-z]+\.[a-z2-7]{13}@example\.(com|net|org)$`),
		"email text":         wantMatch(`^[a-z]+\.[a-z]+@example\.(com|net|org)$`),
		"name varchar":       wantMatch(`^[A-Z][a-z]+ [A-Z][a-z]+$`),
		"name narrow":        wantMatch(`^[A-Z][a-z]{0,5}$`),
		"phone text":         wantValidNANPFiction,
		"phone bigint":       wantMatch(`^1[0-9]{10}$`),
		"phone unique":       wantMatch(`^\+1[0-9]{12}$`),
		"address street":     wantMatch(`^[1-9][0-9]{3} [A-Z][a-z]+ [A-Z][a-z]+$`),
		"address postcode":   wantMatch(`^[0-9A-Z]{7}$`),
		"geo numeric":        wantCoordinate,
		"geo text":           wantCoordinate,
		"date":               wantTime(dateLayout),
		"timestamp":          wantTime(timestampLayout),
		"national id":        wantMatch(`^[1-9][0-9]{8}$`),
		"national id bigint": wantMatch(`^[1-9][0-9]{8}$`),
		"card":               wantLuhn,
		"ipv4":               wantIPIn("192.0.2.0/24", "198.51.100.0/24", "203.0.113.0/24"),
		"ipv6":               wantIPIn("2001:db8::/32"),
		"mac":                wantMatch(`^00:00:5e:00:53:[0-9a-f]{2}$`),
		"cidr":               wantPrefixIn("192.0.2.0/24", "198.51.100.0/24", "203.0.113.0/24"),
		"ip unique":          wantIPIn("2001:db8::/32"),
		"url":                wantMatch(`^https://example\.invalid/[a-z2-7]{13}$`),
		// The bounded column's path is shorter, and it is the branch whose
		// domain the planner refuses on (TestDomainMatchesWhatTheGeneratorEmits).
		"url varchar": wantMatch(`^https://example\.invalid/[a-z2-7]{12}$`),
		"handle":      wantMatch(`^[a-z]+_[a-z2-7]+$`),
		"uuid":        wantUUIDv4,
		"credential":  wantExactly(CredentialLiteral),
		// Still not a plausible credential: the prefix is fixed and readable and
		// only the 13-symbol suffix varies (T-0098).
		"credential unique": wantMatch(`^lazyslice-invalid-[a-z2-7]{13}$`),
		"free text":         wantMatch(`^[a-z ]{1,200}$`),
		"special enum":      wantOneOf("single", "married", "widowed"),
		"special text":      wantMatch(`^[a-z ]+$`),
		// '' is the empty tsvector, and it is the whole of what this generator
		// emits.
		"tsvector": wantExactly(""),
	}
	for _, tc := range corpus() {
		check, ok := checks[tc.name]
		if !ok {
			continue // shape-free cases are covered by their own tests below
		}
		for i, k := range keys {
			r := applyOrFail(t, k, tc)
			t.Run(tc.name+"/"+strconv.Itoa(i), func(t *testing.T) {
				if !fits(r.Out.Text, tc.c) {
					t.Fatalf("%q is longer than the column's %d characters", r.Out.Text, tc.c.MaxLen)
				}
				check(t, r.Out.Text)
			})
		}
	}
}

func wantMatch(pattern string) func(*testing.T, string) {
	re := regexp.MustCompile(pattern)
	return func(t *testing.T, got string) {
		t.Helper()
		if !re.MatchString(got) {
			t.Fatalf("%q does not match %s", got, pattern)
		}
	}
}

func wantExactly(want string) func(*testing.T, string) {
	return func(t *testing.T, got string) {
		t.Helper()
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	}
}

func wantOneOf(labels ...string) func(*testing.T, string) {
	return func(t *testing.T, got string) {
		t.Helper()
		for _, l := range labels {
			if got == l {
				return
			}
		}
		t.Fatalf("%q is not one of %v: a masked enum must emit a member label", got, labels)
	}
}

// A masked phone must still satisfy libphonenumber and must sit in the range
// reserved for fiction, or the fixture it was made for stops validating.
func wantValidNANPFiction(t *testing.T, got string) {
	t.Helper()
	num, err := pn.Parse(got, "")
	if err != nil {
		t.Fatalf("%q does not parse: %v", got, err)
	}
	if !pn.IsValidNumber(num) {
		t.Fatalf("%q is not a valid number", got)
	}
	if !regexp.MustCompile(`^\+1[0-9]{3}555010?[0-9]{1,2}$`).MatchString(got) {
		t.Fatalf("%q is outside the 555-01XX range reserved for fiction", got)
	}
}

func wantCoordinate(t *testing.T, got string) {
	t.Helper()
	f, err := strconv.ParseFloat(got, 64)
	if err != nil {
		t.Fatalf("%q is not a number: %v", got, err)
	}
	if f < -90 || f > 90 {
		t.Fatalf("%q is outside ±90", got)
	}
}

func wantTime(layout string) func(*testing.T, string) {
	return func(t *testing.T, got string) {
		t.Helper()
		parsed, err := time.Parse(layout, got)
		if err != nil {
			t.Fatalf("%q is not a %s: %v", got, layout, err)
		}
		if parsed.Year() < 1940 || parsed.Year() > 2010 {
			t.Fatalf("%q is outside the window the generator draws from", got)
		}
	}
}

func wantLuhn(t *testing.T, got string) {
	t.Helper()
	if !regexp.MustCompile(`^[1-9][0-9]{15}$`).MatchString(got) {
		t.Fatalf("%q is not sixteen digits", got)
	}
	if luhnCheckDigit(got[:len(got)-1]) != got[len(got)-1:] {
		t.Fatalf("%q fails the Luhn check the application makes", got)
	}
}

func wantIPIn(blocks ...string) func(*testing.T, string) {
	return func(t *testing.T, got string) {
		t.Helper()
		addr, err := netip.ParseAddr(got)
		if err != nil {
			t.Fatalf("%q is not an address: %v", got, err)
		}
		for _, b := range blocks {
			if netip.MustParsePrefix(b).Contains(addr) {
				return
			}
		}
		t.Fatalf("%q is outside the documentation ranges %v", got, blocks)
	}
}

func wantPrefixIn(blocks ...string) func(*testing.T, string) {
	return func(t *testing.T, got string) {
		t.Helper()
		p, err := netip.ParsePrefix(got)
		if err != nil {
			t.Fatalf("%q is not a prefix: %v", got, err)
		}
		for _, b := range blocks {
			if netip.MustParsePrefix(b).Contains(p.Addr()) {
				return
			}
		}
		t.Fatalf("%q is outside the documentation ranges %v", got, blocks)
	}
}

func wantUUIDv4(t *testing.T, got string) {
	t.Helper()
	if !reUUID.MatchString(got) || len(got) != uuidLen {
		t.Fatalf("%q is not a UUID", got)
	}
	if got[14] != '4' {
		t.Fatalf("%q is not version 4", got)
	}
}

// A photograph has no fake worth generating: binary_personal is emptied.
func TestBinaryPersonalIsEmptied(t *testing.T) {
	k := testKey(t)
	nullable, err := Apply(k, CatBinary, MaskerNull, Value{Bytes: []byte("PNGDATA")},
		Constraints{TypeTag: famBytea, Nullable: true})
	if err != nil {
		t.Fatal(err)
	}
	if !nullable.Out.Null {
		t.Fatalf("a nullable bytea column should be NULLed, got %v", nullable.Out)
	}
	notNull, err := Apply(k, CatBinary, MaskerNull, Value{Bytes: []byte("PNGDATA")},
		Constraints{TypeTag: famBytea})
	if err != nil {
		t.Fatal(err)
	}
	if notNull.Out.Null || len(notNull.Out.Bytes) != 0 {
		t.Fatalf("a NOT NULL bytea column should get empty bytes, got %v", notNull.Out)
	}
}

// A JSON document keeps its structure and its key names — ARCHITECTURE.md §6
// item 6 lists key names as a stated false negative — and loses every scalar
// leaf.
func TestJSONKeepsShapeAndLosesEveryLeaf(t *testing.T) {
	const src = `{"b":{"name":"Ada","age":36},"a":[1,true,null,"x"]}`
	r, err := Apply(testKey(t), CatSemiStruct, MaskerSemiStruct, Value{Text: src},
		Constraints{TypeTag: famJSONB})
	if err != nil {
		t.Fatal(err)
	}
	var in, out any
	if err := json.Unmarshal([]byte(src), &in); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(r.Out.Text), &out); err != nil {
		t.Fatalf("the masked document is not JSON: %v (%s)", err, r.Out.Text)
	}
	if !reflect.DeepEqual(shapeOf(in), shapeOf(out)) {
		t.Fatalf("shape changed:\n%v\n%v", shapeOf(in), shapeOf(out))
	}
	for _, leaf := range []string{"Ada", `"x"`, "36"} {
		if strings.Contains(r.Out.Text, leaf) {
			t.Errorf("leaf %q survived masking: %s", leaf, r.Out.Text)
		}
	}
}

// shapeOf reduces a document to its keys, its nesting and its leaf kinds.
func shapeOf(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, x := range t {
			out[k] = shapeOf(x)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, x := range t {
			out[i] = shapeOf(x)
		}
		return out
	case nil:
		return "null"
	default:
		return reflect.TypeOf(v).Kind().String()
	}
}

// NULL stays NULL and ” stays ”. They are the two stated exceptions and
// there is no third: a value that is merely whitespace is masked like any
// other, because a canonicaliser that trims to nothing must not fall into the
// empty rule (THREAT_MODEL.md T12).
func TestNullAndEmptyPassThroughAndNothingElseDoes(t *testing.T) {
	k := testKey(t)
	c := Constraints{TypeTag: famText, Nullable: true}
	for _, tc := range corpus() {
		null, err := Apply(k, tc.cat, tc.id, Value{Null: true}, tc.c)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if !null.Out.Null || null.Masked {
			t.Errorf("%s: NULL did not pass through: %+v", tc.name, null)
		}
		empty, err := Apply(k, tc.cat, tc.id, Value{Text: ""}, tc.c)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if empty.Out.Text != "" || empty.Masked {
			t.Errorf("%s: '' did not pass through: %+v", tc.name, empty)
		}
	}
	blank, err := Apply(k, CatFreeText, MaskerFreeText, Value{Text: "   "}, c)
	if err != nil {
		t.Fatal(err)
	}
	if !blank.Masked || blank.Out.Text == "   " {
		t.Fatalf("a whitespace-only value was passed through: %+v", blank)
	}
}

// THREAT_MODEL.md T12's first failure mode: a generator that returns its input
// on a type it did not expect. Every generator whose output space is wider
// than a handful of values must move every value in the corpus.
func TestNoGeneratorReturnsItsInput(t *testing.T) {
	k := testKey(t)
	for _, tc := range corpus() {
		if tc.id == CredentialMasker || tc.id == MaskerNull {
			continue // a fixed literal and a NULL are the intended constants
		}
		r := applyOrFail(t, k, tc)
		if rendered(r.Out) == tc.in {
			t.Errorf("%s: the masker returned its input, %q", tc.name, tc.in)
		}
	}
}

// A generator handed a type its category does not accept must still produce a
// value of that type rather than pass the input through.
func TestUnexpectedTypesDoNotPassThrough(t *testing.T) {
	k := testKey(t)
	odd := []Constraints{
		{TypeTag: famBoolean, Nullable: true},
		{TypeTag: famInteger},
		{TypeTag: famUUID},
		{TypeTag: "other"},
		{TypeTag: ""},
	}
	for _, id := range IDs() {
		if id == CredentialMasker || id == MaskerNull {
			continue
		}
		for _, c := range odd {
			cat := categoryOf(id)
			r, err := Apply(k, cat, id, Value{Text: "original"}, c)
			if err != nil {
				continue // a refusal is a fine answer; a pass-through is not
			}
			if r.Out.Text == "original" {
				t.Errorf("%s on %s returned its input", id, c.TypeTag)
			}
		}
	}
}

func categoryOf(id ID) Category {
	mu.RLock()
	defer mu.RUnlock()
	return registry[id].cat
}
