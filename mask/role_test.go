// SPDX-License-Identifier: Apache-2.0

package mask

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// inWordList reports whether s, folded to lower case, is one of w's words.
func inWordList(t *testing.T, w *wordList, s string) bool {
	t.Helper()
	lower := strings.ToLower(s)
	for _, word := range w.words {
		if word == lower {
			return true
		}
	}
	return false
}

// TestPersonNameRoleShape is ARCHITECTURE.md §5's role rule pinned directly:
// RoleGiven emits one word off the given-name list, RoleFamily one word off
// the surname list, and RoleFull -- the zero value -- keeps the original
// "Given Family" pair. mask/CLAUDE.md's person_name masker previously read
// only Constraints.Domain()/Mask() budgets; T-0287 is the first thing that
// branches on Constraints.Role, so this is the guard that a masked
// first_name column never carries a second word home.
func TestPersonNameRoleShape(t *testing.T) {
	m, ok := Get(MaskerPersonName)
	if !ok {
		t.Fatal("no person_name masker registered")
	}
	in := Value{Text: "x"}
	cases := []struct {
		name string
		role Role
		want *wordList
	}{
		{"given", RoleGiven, givenNames},
		{"family", RoleFamily, surnames},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := Constraints{TypeTag: famText, Role: tc.role}
			for i := 0; i < 200; i++ {
				out, err := m.Mask(digestOf(i), in, c)
				if err != nil {
					t.Fatalf("digest %d: %v", i, err)
				}
				if strings.Contains(out.Text, " ") {
					t.Fatalf("digest %d: %q is more than one word", i, out.Text)
				}
				if !inWordList(t, tc.want, out.Text) {
					t.Fatalf("digest %d: %q is not one of the %s list", i, out.Text, tc.name)
				}
			}
		})
	}
}

// TestPersonNameRoleFullIsStillAPair is the regression this task exists to
// close: a first_name and a last_name column used to get the same "Given
// Family" pair a full_name column does. RoleFull -- Constraints.Role's zero
// value -- has to keep doing exactly that, or every caller of mask.Apply built
// before this field existed changes behaviour under it for free.
func TestPersonNameRoleFullIsStillAPair(t *testing.T) {
	m, _ := Get(MaskerPersonName)
	c := Constraints{TypeTag: famText}
	for i := 0; i < 20; i++ {
		out, err := m.Mask(digestOf(i), Value{Text: "x"}, c)
		if err != nil {
			t.Fatalf("digest %d: %v", i, err)
		}
		words := strings.Fields(out.Text)
		if len(words) != 2 {
			t.Fatalf("digest %d: %q is not a two-word name", i, out.Text)
		}
		if !inWordList(t, givenNames, words[0]) || !inWordList(t, surnames, words[1]) {
			t.Fatalf("digest %d: %q is not given-then-surname", i, out.Text)
		}
	}
}

// TestPersonNameRoleDeterministic is §5's determinism scope restated for the
// new field: the same input under the same key and the same role masks the
// same way, run after run.
func TestPersonNameRoleDeterministic(t *testing.T) {
	k := testKey(t)
	in := Value{Text: "Ada Lovelace"}
	for _, role := range []Role{RoleFull, RoleGiven, RoleFamily} {
		c := Constraints{TypeTag: famText, Role: role}
		a, err := Apply(k, CatPersonName, MaskerPersonName, in, c)
		if err != nil {
			t.Fatalf("role %q: %v", role, err)
		}
		b, err := Apply(k, CatPersonName, MaskerPersonName, in, c)
		if err != nil {
			t.Fatalf("role %q: %v", role, err)
		}
		if a.Out.Text != b.Out.Text {
			t.Fatalf("role %q: %q then %q for the same input, key and role", role, a.Out.Text, b.Out.Text)
		}
	}
}

// TestRoleWordsExcludeKnownRealNames and TestRoleWordsDisjointFromSharedNameLists
// are retired (T-0304). They pinned T-0287's synthetic RoleGiven/RoleFamily
// lists as containing no real name and sharing no word with the hand-curated
// RoleFull lists, because a real name in a masked first_name column was a
// residual hit the scan refused at exit 9. ADR-015 inverted that premise: the
// residual scan now explains a masked name that equals some other row's real
// name (the list contains it, transform emitted every copy, no row kept its
// own), so the lists ARE real names -- the 2020 Census given names and
// surnames, one pair for every role and for email local parts. What replaces
// the two tests is below: every list word is in Emits for its role, no list
// holds two words that fold alike, Domain reports the Census counts, and the
// small_domain rule stays quiet over an ordinary name sample. The redraw's
// half -- Apply over every list word, in every role and three spellings,
// never reads as its input -- is vocab_test.go's
// TestListWordsNeverMaskToThemselves, which walks these same lists.

// spellings is a list word as a column might hold it: lower case, title case
// and upper case.
func spellings(w string) []string { return []string{w, titleASCII(w), strings.ToUpper(w)} }

// TestEveryListWordIsEmittedForItsRole is ADR-015's vocabulary gate over the
// whole of both lists: every word Mask can draw for a role is a word Emits
// accepts for that role, in each of three spellings. A word Emits missed would
// send a correct run's coincidence to the column probe and refuse it at exit 9.
func TestEveryListWordIsEmittedForItsRole(t *testing.T) {
	text := Constraints{TypeTag: famText}
	with := func(r Role) Constraints { c := text; c.Role = r; return c }
	for _, w := range givenNames.words {
		for _, in := range spellings(w) {
			if !Emits(MaskerPersonName, Value{Text: in}, with(RoleGiven)) {
				t.Errorf("given name %q is not in Emits for RoleGiven", in)
			}
			// RoleFull's narrow form draws one given name.
			if !Emits(MaskerPersonName, Value{Text: in}, with(RoleFull)) {
				t.Errorf("given name %q is not in Emits for RoleFull", in)
			}
		}
	}
	for _, w := range surnames.words {
		for _, in := range spellings(w) {
			if !Emits(MaskerPersonName, Value{Text: in}, with(RoleFamily)) {
				t.Errorf("surname %q is not in Emits for RoleFamily", in)
			}
		}
	}
	// RoleFull's wide form: every given name once and every surname once, each
	// paired with a word of the other list, in three spellings.
	n := max(len(givenNames.words), len(surnames.words))
	for i := 0; i < n; i++ {
		g := givenNames.words[i%len(givenNames.words)]
		sn := surnames.words[i%len(surnames.words)]
		for _, in := range []string{g + " " + sn, titleASCII(g) + " " + titleASCII(sn), strings.ToUpper(g + " " + sn)} {
			if !Emits(MaskerPersonName, Value{Text: in}, with(RoleFull)) {
				t.Errorf("pair %q is not in Emits for RoleFull", in)
			}
		}
	}
}

// TestNameListsHaveNoFoldDuplicates: two words of one list that fold alike
// would be one output drawn twice as often, and Domain would count it twice --
// a Domain above what the generator emits, which mask/CLAUDE.md forbids.
func TestNameListsHaveNoFoldDuplicates(t *testing.T) {
	for _, l := range []struct {
		name string
		w    *wordList
	}{{"givenNames", givenNames}, {"surnames", surnames}} {
		seen := make(map[string]string, len(l.w.words))
		for _, w := range l.w.words {
			k := fold(titleASCII(w))
			if prev, ok := seen[k]; ok {
				t.Errorf("%s: %q and %q fold to the same word", l.name, prev, w)
			}
			seen[k] = w
		}
	}
}

// TestPersonNameDomainIsTheCensusLists pins what Domain reports now that the
// three roles draw from the 2020 Census lists (958 given names, 1,000
// surnames), and that the unique-index rule is unchanged in effect.
//
// A unique single-name column was refused at plan before T-0304 -- RoleGiven
// had 780 words and RoleFamily 863, against d_required = n^2/2e = 500,000 at
// one row -- and it still is: 958 and 1,000 are as far below 500,000, and
// MaxRows is 0 either way, so the refusal names no --take that would work. A
// unique full-name column moves from 141 x 145 = 20,445 pairs (MaxRows 0) to
// 958,000 (MaxRows 1): still refused at any real row count.
func TestPersonNameDomainIsTheCensusLists(t *testing.T) {
	m, _ := Get(MaskerPersonName)
	cases := []struct {
		role Role
		want int64
	}{
		{RoleGiven, 958},
		{RoleFamily, 1000},
		{RoleFull, 958 * 1000},
	}
	for _, tc := range cases {
		c := Constraints{TypeTag: famText, Role: tc.role}
		if got := m.Domain(c); got != tc.want {
			t.Errorf("role %q: Domain = %d, want %d", tc.role, got, tc.want)
		}

		c.Unique = true
		c.Rows = 2
		_, err := Pick(CatPersonName, c)
		var de *DomainError
		if !errors.As(err, &de) {
			t.Fatalf("role %q: a unique name column of 2 rows: Pick err = %v, want a *DomainError", tc.role, err)
		}
		if de.Domain != tc.want || de.Required != Required(2) {
			t.Errorf("role %q: refusal says d=%d, d_required=%d; want %d and %d",
				tc.role, de.Domain, de.Required, tc.want, Required(2))
		}
	}
	for _, role := range []Role{RoleGiven, RoleFamily} {
		c := Constraints{TypeTag: famText, Role: role, Unique: true, Rows: 1}
		if _, err := Pick(CatPersonName, c); err == nil {
			t.Errorf("role %q: a unique single-name column of one row was accepted", role)
		}
		if got := MaxRows(m.Domain(c)); got != 0 {
			t.Errorf("role %q: MaxRows = %d, want 0", role, got)
		}
	}
}

// TestNameColumnIsNotSmallDomain is ADR-015's floor on the lists' size: at
// least 400 words each, so that an ordinary name column never trips section
// 5's small_domain rule (d < 2 x distinct samples) over a 200-row sample in
// which every sampled name is distinct. Listing it there would take it out of
// the residual filter and out of invariant I2 for a reason that is not true.
// Small is this module's statement of the rule over the generator's own
// domain; the parent's markSmallDomains (internal/core) applies only its
// catalog half today, which a text column never trips, so this pins the half
// that would list a name column if it did fire.
func TestNameColumnIsNotSmallDomain(t *testing.T) {
	for _, role := range []Role{RoleGiven, RoleFamily, RoleFull} {
		c := Constraints{TypeTag: famText, Role: role, Distinct: 200}
		if d := Admissible(MaskerPersonName, c); d < 400 {
			t.Errorf("role %q: admissible domain %d is below 400", role, d)
		}
		if Small(MaskerPersonName, c) {
			t.Errorf("role %q: a 200-row sample of distinct names reads as a small domain", role)
		}
	}
}

// TestPersonNameRoleDoesNotChangeTheDigest is the residual scan's own half of
// §5's amendment: a role changes which generator branch Mask takes, never the
// canonical bytes the residual filter hashes or the digest the branch is
// chosen from. Canonical is a function of the category and the value alone
// (ARCHITECTURE.md §5's Encode), and CatPersonName's canonical form does not
// read Constraints at all -- this pins that so a future canonicaliser change
// cannot fold Role into it by accident and quietly turn a role change into a
// value change for verify's residual scan.
func TestPersonNameRoleDoesNotChangeTheDigest(t *testing.T) {
	k := testKey(t)
	in := Value{Text: "Ada Lovelace"}
	full, err := Apply(k, CatPersonName, MaskerPersonName, in, Constraints{TypeTag: famText})
	if err != nil {
		t.Fatal(err)
	}
	given, err := Apply(k, CatPersonName, MaskerPersonName, in, Constraints{TypeTag: famText, Role: RoleGiven})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(full.Canonical, given.Canonical) {
		t.Fatalf("role changed the canonical bytes: %q vs %q", full.Canonical, given.Canonical)
	}
	if full.TypeTag != given.TypeTag {
		t.Fatalf("role changed the canonical type tag: %q vs %q", full.TypeTag, given.TypeTag)
	}
}
