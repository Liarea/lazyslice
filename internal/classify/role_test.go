// SPDX-License-Identifier: Apache-2.0

package classify

import (
	"testing"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/internal/textsig"
	"github.com/Liarea/lazyslice/mask"
)

// TestPersonNameRoleFromColumnName is T-0287's own regression: a person_name
// column carries a role read from its own name, so that a first-name column
// masks to a given name and a last-name column to a surname instead of both
// getting the "Given Family" pair a full-name column does. One column per
// case, the way TestNameMatchedPhoneColumnMasksOnNameAlone reads.
func TestPersonNameRoleFromColumnName(t *testing.T) {
	t.Parallel()
	cases := []struct {
		column string
		want   mask.Role
	}{
		{"first_name", mask.RoleGiven},
		{"given_name", mask.RoleGiven},
		{"forename", mask.RoleGiven},
		{"fname", mask.RoleGiven},
		{"last_name", mask.RoleFamily},
		{"family_name", mask.RoleFamily},
		{"surname", mask.RoleFamily},
		{"lname", mask.RoleFamily},
		{"name", mask.RoleFull},
		{"full_name", mask.RoleFull},
		// middle_name is neither a given- nor a family-name shape by this
		// task's own mapping, so it stays the unchanged default.
		{"middle_name", mask.RoleFull},
	}
	for _, rt := range cases {
		t.Run(rt.column, func(t *testing.T) {
			t.Parallel()
			tbl := ref.TableRef{Schema: "public", Name: "reg_role_people"}
			schema := &pipeline.Schema{
				Tables: []pipeline.Table{
					tt("public", "reg_role_people", nil, tc(rt.column, "text")),
				},
			}
			cls, err := New().Classify(schema, mapSampler{}, nil)
			if err != nil {
				t.Fatalf("Classify: %v", err)
			}
			col := ref.ColumnRef{Table: tbl, Column: rt.column}
			d := decision(t, cls, col)
			if d.Category != pipeline.CatPersonName {
				t.Fatalf("%s decided category %q, want person_name", rt.column, d.Category)
			}
			if d.Role != rt.want {
				t.Errorf("%s decided role %q, want %q", rt.column, d.Role, rt.want)
			}
		})
	}
}

// TestRoleWordsExcludeCurrentNameDictionary is the fuller half of the
// fix-round review's finding 1 (T-0287, high): mask/role_test.go's own
// TestRoleWordsExcludeKnownRealNames and TestRoleWordsDisjointFromSharedNameLists
// pin the exact tokens the review found and the mask module's own two
// shared lists, but mask may not import internal/textsig at all
// (mask/CLAUDE.md's "Never" list: "Import anything under internal/"), so
// neither test there can check roleGivenWords/roleFamilyWords against the
// project's actual, current, multilingual name dictionary
// (internal/textsig/names.txt) -- the corpus the review's finding named
// first and the one most likely to gain an entry that collides with a role
// word in the future. This package already imports both mask and textsig
// (classify.go, validators.go), so the cross-check runs here: every word in
// mask.RoleWords(mask.RoleGiven) and mask.RoleWords(mask.RoleFamily) is
// checked with textsig.Dictionary().LooksLikeName, the same one-word name
// test internal/classify's own signals use to decide a first_name column in
// the first place. A hit here means a name added to names.txt (or a
// multilingual population added to it, T-0187's own kind of change) has
// collided with a synthetic role token, which is exactly the residual risk
// mask/words.go's own comment on roleGivenWords/roleFamilyWords states
// rather than hides.
func TestRoleWordsExcludeCurrentNameDictionary(t *testing.T) {
	dict := textsig.Dictionary()
	cases := []struct {
		role  mask.Role
		words []string
	}{
		{mask.RoleGiven, mask.RoleWords(mask.RoleGiven)},
		{mask.RoleFamily, mask.RoleWords(mask.RoleFamily)},
	}
	for _, tc := range cases {
		if len(tc.words) == 0 {
			t.Fatalf("mask.RoleWords(%q) returned no words", tc.role)
		}
		for _, w := range tc.words {
			if dict.LooksLikeName(w) {
				t.Errorf("role %q word %q is in internal/textsig's current name dictionary", tc.role, w)
			}
		}
	}
}
