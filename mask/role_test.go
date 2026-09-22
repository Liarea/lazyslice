// SPDX-License-Identifier: Apache-2.0

package mask

import (
	"bytes"
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
		{"given", RoleGiven, roleGivenNames},
		{"family", RoleFamily, roleFamilyNames},
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

// TestRoleWordsExcludeKnownRealNames is the fix-round review's own regression
// (T-0287, high finding 1): a review measured that roleGivenWords' and
// roleFamilyWords' original 900 entries each collided with real names on two
// counts -- 34 were literal entries of internal/textsig/names.txt, and
// materially more again were ordinary common given names, surnames or
// English words the dictionary does not carry -- while the comment above the
// two lists claimed them disjoint from real names "by construction". This
// pins the exact tokens the review's own finding named as evidence, on both
// lists, as a permanent guard: any of them reappearing means the filtering
// mask/words.go's comment now describes has regressed. It cannot re-run the
// filtering itself (that needs internal/textsig's dictionary and a corpus
// outside this module, which mask may import neither of), so
// internal/classify/role_test.go's TestRoleWordsExcludeCurrentNameDictionary
// carries the fuller, live check.
func TestRoleWordsExcludeKnownRealNames(t *testing.T) {
	// Exactly the tokens the review's finding cited as evidence (mask/CLAUDE.md's
	// T-0287 section and words.go's own comment have the fuller account), given
	// names and surnames mixed without regard to which role list they came from
	// in real life: the point is that neither role list may contain any of them,
	// whichever position a real person carries it in.
	knownRealNames := []string{
		// from internal/textsig/names.txt
		"siri", "nero", "leni", "mena", "nela", "risa", "sona", "sosa", "rumi",
		"sibel", "geri", "veli", "vesa", "rola", "tani", "sama", "nuno", "runo",
		"gema", "tunes",
		// common real given names and surnames the dictionary does not carry
		"gale", "sage", "bela", "mari", "manu", "mano", "rani", "nori", "sade",
		"rima", "levon", "deron", "lorin", "daven", "boris", "titus", "juli",
		"kota", "mati", "mako", "koda",
	}
	for _, name := range knownRealNames {
		if inWordList(t, roleGivenNames, name) {
			t.Errorf("roleGivenNames still contains the known real name %q", name)
		}
		if inWordList(t, roleFamilyNames, name) {
			t.Errorf("roleFamilyNames still contains the known real name %q", name)
		}
	}
}

// TestRoleWordsDisjointFromSharedNameLists checks, rather than merely
// comments, the one claim words.go's "by construction" language happened to
// get right: roleGivenWords and roleFamilyWords never share a token with
// givenWords or surnameWords, the lists RoleFull and emailMasker read. This
// is the narrow, self-contained half of the review's ask that this module can
// verify on its own, with no corpus outside it.
func TestRoleWordsDisjointFromSharedNameLists(t *testing.T) {
	shared := make(map[string]bool, len(givenNames.words)+len(surnames.words))
	for _, w := range givenNames.words {
		shared[w] = true
	}
	for _, w := range surnames.words {
		shared[w] = true
	}
	for _, w := range roleGivenNames.words {
		if shared[w] {
			t.Errorf("roleGivenNames contains %q, which is also in givenWords/surnameWords", w)
		}
	}
	for _, w := range roleFamilyNames.words {
		if shared[w] {
			t.Errorf("roleFamilyNames contains %q, which is also in givenWords/surnameWords", w)
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
