// SPDX-License-Identifier: Apache-2.0

package classify

import (
	"testing"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
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

// TestRoleWordsExcludeCurrentNameDictionary is retired (T-0304). It checked
// every word of mask.RoleWords(RoleGiven/RoleFamily) against
// internal/textsig's name dictionary, because a real name in a masked
// first_name or last_name column was a residual hit the scan confirmed and
// refused at exit 9. ADR-015 inverted that premise: the residual scan explains
// a masked name equal to some other row's real name (the list contains it,
// transform emitted every copy, no row kept its own), so the lists ARE real
// names now -- the 2020 Census given names and surnames -- and a dictionary
// hit on one of them is the intended state, not a defect. What replaces it
// lives in the mask module, beside the lists: mask/role_test.go's
// TestEveryListWordIsEmittedForItsRole, TestNameListsHaveNoFoldDuplicates,
// TestPersonNameDomainIsTheCensusLists and TestNameColumnIsNotSmallDomain,
// and mask/vocab_test.go's TestListWordsNeverMaskToThemselves.
