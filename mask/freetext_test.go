// SPDX-License-Identifier: Apache-2.0

package mask

import (
	"math"
	"strings"
	"testing"

	pn "github.com/nyaruka/phonenumbers"
)

// A 1,247-character bio and a two-word note must be indistinguishable after
// masking. The length of the filler comes from h, never from the input, and
// this is the assertion that says so for a fixed key: over a corpus whose
// input lengths span three orders of magnitude, the correlation between input
// length and output length is nil.
func TestFreeTextLengthUncorrelated(t *testing.T) {
	k := testKey(t)
	c := Constraints{TypeTag: famText}
	const n = 600
	xs := make([]float64, 0, n)
	ys := make([]float64, 0, n)
	for i := 1; i <= n; i++ {
		in := strings.Repeat("a sentence of prose ", i)
		r, err := Apply(k, CatFreeText, MaskerFreeText, Value{Text: in}, c)
		if err != nil {
			t.Fatal(err)
		}
		xs = append(xs, float64(len(in)))
		ys = append(ys, float64(len(r.Out.Text)))
	}
	if r := pearson(xs, ys); math.Abs(r) > 0.15 {
		t.Fatalf("output length correlates with input length: r = %.3f", r)
	}
}

// The same claim in the direction a reader cares about: two inputs of wildly
// different lengths can produce outputs of any lengths at all, and the shortest
// input is not the shortest output.
func TestFreeTextLengthDoesNotSurvive(t *testing.T) {
	k := testKey(t)
	c := Constraints{TypeTag: famText}
	short, err := Apply(k, CatFreeText, MaskerFreeText, Value{Text: "ok"}, c)
	if err != nil {
		t.Fatal(err)
	}
	long, err := Apply(k, CatFreeText, MaskerFreeText,
		Value{Text: strings.Repeat("x", 1247)}, c)
	if err != nil {
		t.Fatal(err)
	}
	if len(short.Out.Text) == 2 || len(long.Out.Text) == 1247 {
		t.Fatalf("a length survived: %d and %d", len(short.Out.Text), len(long.Out.Text))
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

func pearson(xs, ys []float64) float64 {
	n := float64(len(xs))
	var sx, sy float64
	for i := range xs {
		sx += xs[i]
		sy += ys[i]
	}
	mx, my := sx/n, sy/n
	var num, dx, dy float64
	for i := range xs {
		a, b := xs[i]-mx, ys[i]-my
		num += a * b
		dx += a * a
		dy += b * b
	}
	if dx == 0 || dy == 0 {
		return 0
	}
	return num / math.Sqrt(dx*dy)
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
