// SPDX-License-Identifier: Apache-2.0

package mask

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// The vocabulary gate and the redraw of ADR-015 (vocab.go, mask.go's maskCell).

var updateRedraw = flag.Bool("update-redraw", false,
	"rewrite testdata/person_name_redraw.golden from the current redraw")

// roleCase is one shape of person_name column: a role and a width.
type roleCase struct {
	name string
	c    Constraints
}

// roleCases are the four shapes personNameMasker draws in: a given-name column,
// a family-name column, a full-name column wide enough for a pair, and a
// full-name column too narrow for any pair, which draws one given name.
func roleCases() []roleCase {
	return []roleCase{
		{"given", Constraints{TypeTag: "text", Role: RoleGiven}},
		{"family", Constraints{TypeTag: "text", Role: RoleFamily}},
		{"full", Constraints{TypeTag: "text", Role: RoleFull}},
		{"full-narrow", Constraints{TypeTag: "varchar", MaxLen: 7, Role: RoleFull}},
	}
}

// everyListWord is every word of every list personNameMasker draws from.
func everyListWord() []string {
	var out []string
	for _, w := range []*wordList{roleGivenNames, roleFamilyNames, givenNames, surnames} {
		out = append(out, w.words...)
	}
	return out
}

// Every value a vocabulary masker can emit must be fold-faithful: canonical
// equality with any input implies letters-and-digits fold equality with that
// input's canonical form. Since canonical equality means fold(o) == fold(x),
// that is foldEqual(o, fold(o)) for every emitted spelling o — which is what
// lets the redraw (on the fold) cover every value the residual scan would call
// equal (on the canonical form). A list word that fold() changes beyond case
// and spacing breaks it, and this test is what says so.
func TestVocabularyIsFoldFaithful(t *testing.T) {
	check := func(o string) {
		t.Helper()
		if !FoldEqual(o, fold(o)) {
			t.Errorf("%q folds to %q and the two are not letters-and-digits equal", o, fold(o))
		}
	}
	for _, w := range everyListWord() {
		check(titleASCII(w))
	}
	for _, g := range givenNames.words {
		for _, s := range surnames.words {
			check(titleASCII(g) + " " + titleASCII(s))
		}
	}
}

// Only person_name answers the vocabulary question. Adding a second masker to
// this set is an ADR-015 decision (it has to pass the fold-faithful criterion,
// and its column loses the residual scan's column probe), not a tidy-up.
func TestOnlyTheseMaskersEmit(t *testing.T) {
	var got []string
	for _, id := range IDs() {
		m, ok := Get(id)
		if !ok {
			t.Fatalf("IDs() lists %q and Get does not know it", id)
		}
		if _, ok := m.(vocabularyMasker); ok {
			got = append(got, string(id))
		}
		if Emitting(id, Constraints{TypeTag: "text"}) != (id == MaskerPersonName) {
			t.Errorf("Emitting(%q) = %v", id, id != MaskerPersonName)
		}
	}
	sort.Strings(got)
	if strings.Join(got, ",") != string(MaskerPersonName) {
		t.Fatalf("the maskers with a vocabulary are %v, want exactly [%s]", got, MaskerPersonName)
	}
}

func TestEmits(t *testing.T) {
	given := titleASCII(roleGivenNames.words[0])
	family := titleASCII(roleFamilyNames.words[0])
	pair := titleASCII(givenNames.words[3]) + " " + titleASCII(surnames.words[5])
	single := titleASCII(givenNames.words[0])
	text := Constraints{TypeTag: "text"}
	with := func(r Role) Constraints { c := text; c.Role = r; return c }

	cases := []struct {
		name string
		id   ID
		v    Value
		c    Constraints
		want bool
	}{
		{"a given word in a given column", MaskerPersonName, Value{Text: given}, with(RoleGiven), true},
		{"the same word folded", MaskerPersonName, Value{Text: "  " + strings.ToUpper(given) + " "}, with(RoleGiven), true},
		{"a family word in a family column", MaskerPersonName, Value{Text: family}, with(RoleFamily), true},
		{"a family word in a given column", MaskerPersonName, Value{Text: family}, with(RoleGiven), false},
		{"a full name in a given column", MaskerPersonName, Value{Text: pair}, with(RoleGiven), false},
		{"a pair in a full column", MaskerPersonName, Value{Text: pair}, with(RoleFull), true},
		{"a pair spaced oddly", MaskerPersonName, Value{Text: strings.ReplaceAll(pair, " ", "   ")}, with(RoleFull), true},
		{"a single given name in a full column", MaskerPersonName, Value{Text: single}, with(RoleFull), true},
		{"a pair too wide for the column", MaskerPersonName, Value{Text: pair},
			Constraints{TypeTag: "varchar", MaxLen: len(pair) - 1}, false},
		{"a name off every list", MaskerPersonName, Value{Text: "Wolfgangina"}, with(RoleGiven), false},
		{"a shared-list word in a role column", MaskerPersonName, Value{Text: single}, with(RoleGiven), false},
		{"NULL", MaskerPersonName, Value{Null: true}, with(RoleGiven), false},
		{"empty", MaskerPersonName, Value{}, with(RoleGiven), false},
		{"bytes", MaskerPersonName, Value{Bytes: []byte(given)}, with(RoleGiven), false},
		{"a closed column", MaskerPersonName, Value{Text: given},
			Constraints{TypeTag: "text", Role: RoleGiven, EnumLabels: []string{given, "x"}}, false},
		{"a CHECK value list", MaskerPersonName, Value{Text: given},
			Constraints{TypeTag: "text", Role: RoleGiven,
				Checks: []string{"CHECK ((first_name = ANY (ARRAY['" + given + "'::text, 'x'::text])))"}}, false},
		{"an unknown id", "no_such_masker", Value{Text: given}, with(RoleGiven), false},
		{"a fixed: id", ID(FixedPrefix + given), Value{Text: given}, with(RoleGiven), false},
		{"another category's masker", MaskerEmail, Value{Text: given}, with(RoleGiven), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Emits(tc.id, tc.v, tc.c); got != tc.want {
				t.Errorf("Emits = %v, want %v", got, tc.want)
			}
		})
	}
}

// A masker registered from outside this package cannot claim a vocabulary,
// even when it is registered under the person_name category and draws from
// the very lists the built-in one does: its column keeps the column probe.
func TestACustomNameMaskerDoesNotEmit(t *testing.T) {
	m := funcMasker{
		mask: func(h [32]byte, _ Value, _ Constraints) (Value, error) {
			return Value{Text: titleASCII(roleGivenNames.words[int(h[0])%len(roleGivenNames.words)])}, nil
		},
		domain: int64(len(roleGivenNames.words)),
	}
	if _, ok := Masker(m).(vocabularyMasker); ok {
		t.Fatal("a masker written outside personNameMasker answers the vocabulary question")
	}
}

// ADR-015 decision 3: after the redraw, no list word in any spelling, in any
// role, masks to a value that reads as itself.
func TestListWordsNeverMaskToThemselves(t *testing.T) {
	k := testKey(t)
	spellings := func(w string) []string { return []string{w, titleASCII(w), strings.ToUpper(w)} }
	for _, rc := range roleCases() {
		for _, w := range everyListWord() {
			for _, in := range spellings(w) {
				r, err := Apply(k, CatPersonName, MaskerPersonName, Value{Text: in}, rc.c)
				if errors.Is(err, ErrNoRoom) {
					continue
				}
				if err != nil {
					t.Fatalf("%s: Apply: %v", rc.name, err)
				}
				if FoldEqual(r.Out.Text, in) {
					t.Errorf("%s: a list word masked to itself (%d letters)", rc.name, len(in))
				}
			}
		}
	}
	// A full name as the input of a full-name column, every pair.
	full := roleCases()[2].c
	for _, g := range givenNames.words {
		for _, s := range surnames.words {
			in := titleASCII(g) + " " + titleASCII(s)
			r, err := Apply(k, CatPersonName, MaskerPersonName, Value{Text: in}, full)
			if err != nil {
				t.Fatalf("Apply: %v", err)
			}
			if FoldEqual(r.Out.Text, in) {
				t.Errorf("a full name masked to itself (%d letters)", len(in))
			}
		}
	}
}

// before is what personNameMasker drew for in before the redraw existed: the
// generator called once under h, which is what maskCell returned.
func before(t *testing.T, k Key, in string, c Constraints) string {
	t.Helper()
	canon, tag, err := Canonical(CatPersonName, Value{Text: in}, c)
	if err != nil {
		t.Fatalf("Canonical: %v", err)
	}
	h, err := Digest(k, CatPersonName, tag, canon.bytes())
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	out, err := personNameMasker{}.Mask(h, Value{Text: in}, c)
	if err != nil {
		t.Fatalf("Mask: %v", err)
	}
	return out.Text
}

// The redraw changes a value only where the value used to read as its own
// input, and the golden file lists exactly those, under the fixed test key. A
// change to this file is a change to masked values: every line in it is a
// person name that masked differently before ADR-015, and every line removed or
// added is a name that masks differently after the change being reviewed.
func TestRedrawChangesOnlyValuesThatUsedToSelfMap(t *testing.T) {
	k := testKey(t)
	var changed []string
	for _, rc := range roleCases() {
		for _, w := range everyListWord() {
			in := titleASCII(w)
			r, err := Apply(k, CatPersonName, MaskerPersonName, Value{Text: in}, rc.c)
			if errors.Is(err, ErrNoRoom) {
				continue
			}
			if err != nil {
				t.Fatalf("%s: Apply: %v", rc.name, err)
			}
			was := before(t, k, in, rc.c)
			if r.Out.Text == was {
				continue
			}
			if !FoldEqual(was, in) {
				t.Errorf("%s: the redraw changed a value that did not read as its input", rc.name)
			}
			changed = append(changed, rc.name+"\t"+in+"\t"+was+"\t"+r.Out.Text)
		}
	}
	sort.Strings(changed)
	got := strings.Join(changed, "\n") + "\n"

	path := filepath.Join("testdata", "person_name_redraw.golden")
	if *updateRedraw {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s (run with -update-redraw to write it): %v", path, err)
	}
	// A Windows checkout may hand the golden back with CRLF line endings
	// (.gitattributes now marks mask/testdata/** -text, and this keeps the
	// comparison honest either way); the test compares lines, not bytes.
	want = []byte(strings.ReplaceAll(string(want), "\r\n", "\n"))
	if string(want) != got {
		t.Errorf("the redraw changed a different set of values than %s records:\ngot:\n%swant:\n%s",
			path, got, want)
	}
	if len(changed) == 0 {
		t.Log("no list word self-mapped under the test key; the redraw changed nothing")
	}
}

// passName is a vocabulary masker that hands its input back: the whole of it,
// or only under some h. It embeds personNameMasker, so it has the vocabulary
// method and the redraw would reach it — which is why the redraw has to sit
// after tracksItsInput, and why this test exists.
type passName struct {
	personNameMasker
	partial bool
}

func (p passName) Mask(h [32]byte, in Value, c Constraints) (Value, error) {
	if !p.partial || h[0]%2 == 0 {
		return in, nil
	}
	return p.personNameMasker.Mask(h, in, c)
}

func TestTheRedrawNeverLaundersAPassthrough(t *testing.T) {
	c := Constraints{TypeTag: "text", Role: RoleGiven}
	whole := passName{}
	if _, err := guarded(t, whole, CatPersonName, Value{Text: "Bado"}, c); !errors.Is(err, ErrPassthrough) {
		t.Errorf("a vocabulary masker that returns its input: err = %v, want ErrPassthrough", err)
	}

	// The partial one follows its input for about half of all h. Find an input
	// under the test key whose h it follows, and it must be refused there.
	partial := passName{partial: true}
	k := testKey(t)
	found := false
	for _, w := range roleGivenNames.words {
		in := Value{Text: titleASCII(w)}
		canon, tag, err := Canonical(CatPersonName, in, c)
		if err != nil {
			t.Fatal(err)
		}
		h, err := Digest(k, CatPersonName, tag, canon.bytes())
		if err != nil {
			t.Fatal(err)
		}
		if h[0]%2 != 0 {
			continue
		}
		found = true
		if _, err := guarded(t, partial, CatPersonName, in, c); !errors.Is(err, ErrPassthrough) {
			t.Errorf("a partial passthrough: err = %v, want ErrPassthrough", err)
		}
		break
	}
	if !found {
		t.Fatal("no list word's digest selects the passthrough branch, so this test proves nothing")
	}
}

func TestRedrawDigestIsAFunctionOfHAndI(t *testing.T) {
	var h [32]byte
	h[0] = 7
	a, b := redrawDigest(h, 1), redrawDigest(h, 2)
	if a == b || a == h {
		t.Fatal("two redraws share a digest, or a redraw reuses h")
	}
	if redrawDigest(h, 1) != a {
		t.Fatal("the redraw digest is not deterministic")
	}
}
