// SPDX-License-Identifier: Apache-2.0

package mask

import (
	"errors"
	"slices"
	"strings"
	"testing"

	pn "github.com/nyaruka/phonenumbers"
)

// Since T-0328 a free-text value is masked to filler about as long as the
// input, not to a length drawn from the column: in a column that holds the
// longest filler word it is whole words, between four characters shorter than
// the input and max(input length, nine); in a narrower column it is exactly
// the input's length. Every character is still drawn from h. The length is
// the rune count of the input's canonical form.
func TestFreeTextLengthFollowsTheInput(t *testing.T) {
	k := testKey(t)
	wide := []Constraints{
		{TypeTag: famText},
		{TypeTag: famVarchar, MaxLen: 255},
		{TypeTag: famVarchar, MaxLen: 9},
	}
	for _, c := range wide {
		for i := 1; i <= 240; i++ {
			in := strings.Repeat("Prose, ", i)[:min(i*7, freeTextMax(c))]
			r, err := Apply(k, CatFreeText, MaskerFreeText, Value{Text: in}, c)
			if err != nil {
				t.Fatalf("maxlen %d, input %d: %v", c.MaxLen, i, err)
			}
			n, got := freeTextInputLen(Value{Text: in}), len(r.Out.Text)
			if got < n-4 || got > max(n, fittedFillers.longest()) {
				t.Fatalf("maxlen %d: a %d-character input masked to %d characters; want %d to %d",
					c.MaxLen, n, got, n-4, max(n, fittedFillers.longest()))
			}
			for _, w := range strings.Split(r.Out.Text, " ") {
				if !isFillerWord(w) {
					t.Fatalf("maxlen %d: %q is not a run of whole filler words", c.MaxLen, r.Out.Text)
				}
			}
		}
	}
	narrow := Constraints{TypeTag: famVarchar, MaxLen: 8}
	for i := 1; i <= 8; i++ {
		in := strings.Repeat("Z", i)
		r, err := Apply(k, CatFreeText, MaskerFreeText, Value{Text: in}, narrow)
		if err != nil {
			t.Fatal(err)
		}
		if len(r.Out.Text) != i {
			t.Fatalf("varchar(8): a %d-character input masked to %q", i, r.Out.Text)
		}
	}
}

func isFillerWord(w string) bool {
	for _, x := range fittedFillers.words {
		if w == x {
			return true
		}
	}
	return false
}

// The dogfood case T-0323 decided (docs/DOGFOOD_LOG.md, session 1): a users
// role column with three short values masked to three distinct 255-character
// paragraphs, and the copy could not boot. Pinned here as a three-value
// enum-like column: each value masks to one or two filler words, deterministic
// under the key, none equal to its input, and the same in a varchar(20), a
// varchar(255) and a text column — the length comes from the input, not the
// column, so two foreign-key-linked columns holding one role mask it alike and
// report the same Domain() to the plan's equality-group check. The admissible
// domain is the one-word count, so the column is not a small domain at three
// distinct values, and the same column under a unique index is refused at plan
// with d = 80 (it was refused at every row count before as well).
func TestFreeTextEnumLikeColumnMasksToShortValues(t *testing.T) {
	k := testKey(t)
	cols := []Constraints{
		{TypeTag: famVarchar, MaxLen: 255},
		{TypeTag: famVarchar, MaxLen: 20},
		{TypeTag: famText},
	}
	roles := []string{"admin", "user", "guest"}
	seen := map[string]string{}
	for _, role := range roles {
		var first string
		for i, c := range cols {
			r, err := Apply(k, CatFreeText, MaskerFreeText, Value{Text: role}, c)
			if err != nil {
				t.Fatalf("%s: %v", role, err)
			}
			again, err := Apply(k, CatFreeText, MaskerFreeText, Value{Text: role}, c)
			if err != nil {
				t.Fatal(err)
			}
			if again.Out.Text != r.Out.Text {
				t.Fatalf("%s: one key, one input, two outputs: %q and %q", role, r.Out.Text, again.Out.Text)
			}
			if !r.Masked || r.Out.Text == role || len(r.Out.Text) > fittedFillers.longest() {
				t.Fatalf("%s masked to %q; want a masked value of at most %d characters",
					role, r.Out.Text, fittedFillers.longest())
			}
			if i == 0 {
				first = r.Out.Text
			} else if r.Out.Text != first {
				t.Fatalf("%s masks to %q in one column and %q in another", role, first, r.Out.Text)
			}
		}
		if other, ok := seen[first]; ok {
			t.Fatalf("%s and %s both mask to %q under the test key", role, other, first)
		}
		seen[first] = role
	}
	m, _ := Get(MaskerFreeText)
	for _, c := range cols {
		if d := m.Domain(c); d != int64(len(fittedFillers.words)) {
			t.Fatalf("maxlen %d: Domain = %d, want the %d one-word values", c.MaxLen, d, len(fittedFillers.words))
		}
		c.Distinct = int64(len(roles))
		if Small(MaskerFreeText, c) {
			t.Fatalf("maxlen %d: three distinct values against %d is not a small domain", c.MaxLen, m.Domain(c))
		}
	}
	u := Constraints{TypeTag: famVarchar, MaxLen: 255, Unique: true, Rows: 3}
	var de *DomainError
	if _, err := Pick(CatFreeText, u); !errors.As(err, &de) || de.Domain != int64(len(fittedFillers.words)) {
		t.Fatalf("a unique free-text column of three rows: %v, want a *DomainError with d = %d", err, len(fittedFillers.words))
	}
}

// An input longer than the column allows keeps the column-width behaviour: a
// length drawn from h inside [1, min(column, 4096)]. So does a value whose
// canonical form NFKC lengthens (a ligature) past the column that held it, so
// the masked value still fits that column.
func TestFreeTextLongerThanTheColumnAllows(t *testing.T) {
	k := testKey(t)
	c := Constraints{TypeTag: famText}
	lengths := map[int]bool{}
	for i := 0; i < 20; i++ {
		in := strings.Repeat("x", freeTextCap+1+i)
		r, err := Apply(k, CatFreeText, MaskerFreeText, Value{Text: in}, c)
		if err != nil {
			t.Fatal(err)
		}
		if n := len(r.Out.Text); n < 1 || n > freeTextCap {
			t.Fatalf("a %d-character input masked to %d characters; want 1 to %d", len(in), n, freeTextCap)
		}
		lengths[len(r.Out.Text)] = true
	}
	if len(lengths) < 10 {
		t.Fatalf("20 over-long inputs masked to only %d lengths; the length should come from h", len(lengths))
	}
	narrow := Constraints{TypeTag: famVarchar, MaxLen: 5}
	r, err := Apply(k, CatFreeText, MaskerFreeText, Value{Text: "\ufb01\ufb01\ufb01\ufb01\ufb01"}, narrow)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(r.Out.Text); n < 1 || n > narrow.MaxLen {
		t.Fatalf("five ligatures in a varchar(5) masked to %q; want 1 to 5 characters", r.Out.Text)
	}
}

// fold promises that two values differing only in case, compatibility form
// or whitespace mask alike, and T-0328's length must not break it: the length
// is the canonical form's, so spellings of different raw length that hash
// alike (the review of T-0328) are fitted alike, in a narrow column where the
// output is exactly that length and in a wide one where the length decides
// which words fit.
func TestFreeTextCanonicalSpellingsMaskAlike(t *testing.T) {
	k := testKey(t)
	pairs := [][2]string{
		{"STRASSE", "stra\u00dfe"},
		{"file", "\ufb01le"},
		{"Admin ", "admin"},
	}
	cols := []Constraints{
		{TypeTag: famVarchar, MaxLen: 8},
		{TypeTag: famText},
	}
	for _, c := range cols {
		for _, p := range pairs {
			a, err := Apply(k, CatFreeText, MaskerFreeText, Value{Text: p[0]}, c)
			if err != nil {
				t.Fatal(err)
			}
			b, err := Apply(k, CatFreeText, MaskerFreeText, Value{Text: p[1]}, c)
			if err != nil {
				t.Fatal(err)
			}
			if a.Out.Text != b.Out.Text {
				t.Fatalf("maxlen %d: %q masks to %q and %q to %q; one canonical value, two outputs",
					c.MaxLen, p[0], a.Out.Text, p[1], b.Out.Text)
			}
		}
	}
}

// A short input becomes one filler word (or a cut of one), so a filler word an
// application stores as a state or level of its own could come out equal to a
// real value of the column, and the residual scan would stop the run at exit
// 9 with nothing the user can change (T-0328's review: a log_level column
// holding trace). The input-fitted branches draw from fittedFillers, which
// leaves those words out, so none of them is ever emitted, as a whole value or
// as the cut a narrow column takes.
func TestFreeTextNeverEmitsAStateWord(t *testing.T) {
	state := strings.Fields(fillerStateWords)
	for _, w := range state {
		if !slices.Contains(strings.Fields(fillerWords), w) {
			t.Fatalf("%q is not a filler word; fillerStateWords names only words it removes", w)
		}
	}
	if len(fittedFillers.words) != len(fillers.words)-len(state) {
		t.Fatalf("fittedFillers holds %d words; want %d less %d", len(fittedFillers.words), len(fillers.words), len(state))
	}
	for _, x := range fittedFillers.words {
		for _, w := range state {
			if strings.HasPrefix(x, w) {
				t.Fatalf("%q begins with the state word %q, so a narrow column could cut it to that", x, w)
			}
		}
	}
	k := testKey(t)
	inputs := append([]string{"debug", "info", "warn", "error", "fatal", "active", "pending"}, state...)
	cols := []Constraints{
		{TypeTag: famVarchar, MaxLen: 8},
		{TypeTag: famVarchar, MaxLen: 20},
		{TypeTag: famText},
	}
	for _, c := range cols {
		for _, in := range inputs {
			for i := 0; i <= 26; i++ {
				v := in // and 26 more of at most seven characters, so every column holds them
				if i > 0 {
					v = in[:min(len(in), 6)] + string(rune('a'+i-1))
				}
				r, err := Apply(k, CatFreeText, MaskerFreeText, Value{Text: v}, c)
				if err != nil {
					t.Fatal(err)
				}
				for _, w := range state {
					if r.Out.Text == w {
						t.Fatalf("maxlen %d: %q masked to the state word %q", c.MaxLen, v, w)
					}
				}
			}
		}
	}
}

// A char_length CHECK wider than the column's own varchar(n) is a
// contradiction: no row could satisfy both, but freeTextExact must not read
// the CHECK's length as the target anyway, because Mask would then emit a
// value longer than the column holds. It falls back to the ranged filler
// bounded by MaxLen instead, exactly as if there had been no CHECK.
func TestFreeTextExactClampsToMaxLen(t *testing.T) {
	c := Constraints{
		TypeTag: famVarchar,
		MaxLen:  20,
		Checks:  []string{"CHECK ((char_length(bio) = 500))"},
	}
	if l := freeTextExact(c); l != 0 {
		t.Fatalf("freeTextExact = %d for a CHECK length wider than MaxLen; want 0", l)
	}
	k := testKey(t)
	for i := 0; i < 50; i++ {
		r, err := Apply(k, CatFreeText, MaskerFreeText,
			Value{Text: strings.Repeat("x", i+1)}, c)
		if err != nil {
			t.Fatalf("input %d: %v", i, err)
		}
		if len(r.Out.Text) > c.MaxLen {
			t.Fatalf("input %d: output %q is %d bytes, wider than MaxLen %d",
				i, r.Out.Text, len(r.Out.Text), c.MaxLen)
		}
	}

	// A CHECK length that does fit is still honoured exactly.
	fits := Constraints{
		TypeTag: famVarchar,
		MaxLen:  20,
		Checks:  []string{"CHECK ((char_length(bio) = 12))"},
	}
	if l := freeTextExact(fits); l != 12 {
		t.Fatalf("freeTextExact = %d for a CHECK length within MaxLen; want 12", l)
	}
}

// The embedded area codes are a fixed list rather than a probe of the
// phonenumbers metadata, because probing would make a masked phone number
// depend on the library version and a dependency bump would silently remap
// every phone column. This is the loud alternative: a bump that invalidates
// one of them fails here.
func TestNANPAreaCodesAreStillValid(t *testing.T) {
	if len(areaCodes) != 447 {
		t.Fatalf("the embedded list holds %d area codes, not 447; "+
			"changing it changes every masked phone number", len(areaCodes))
	}
	for _, a := range areaCodes {
		num, err := pn.Parse("+1"+a+"5550142", "")
		if err != nil {
			t.Errorf("area code %s no longer parses: %v", a, err)
			continue
		}
		if !pn.IsValidNumber(num) {
			t.Errorf("area code %s is no longer valid in the 555-01XX range", a)
		}
	}
}
