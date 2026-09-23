// SPDX-License-Identifier: Apache-2.0

package transform

import (
	"testing"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/mask"
)

// ADR-015's count: every masked cell of a column whose masker has a vocabulary
// is counted by its output (Residual.AddEmitted), per element for an array,
// and nothing is counted for any other column. internal/verify explains a
// masked name that equals another row's real name only when the target holds
// it no more often than this count says the masker produced it, so a count
// that missed a cell would refuse a correct run and a count that took one from
// another column would explain a leak.
func TestEmittedCountsEveryMaskedCellOfAnEmittingColumn(t *testing.T) {
	names := pipeline.Table{
		Ref: tbl("names"),
		Columns: []pipeline.Column{
			{Name: "name_id", TypeName: "bigint", TypeOID: 20},
			{Name: "first_name", TypeName: "text", TypeOID: 25, Nullable: true},
			{Name: "aliases", TypeName: "text[]", TypeOID: 1009, Nullable: true},
			{Name: "email", TypeName: "text", TypeOID: 25, Nullable: true},
			{Name: "grade", TypeName: "text", TypeOID: 25, Nullable: true,
				Checks: []string{"CHECK ((grade = ANY (ARRAY['Bado'::text, 'Balo'::text])))"}},
		},
		PK: []string{"name_id"},
	}
	schema := &pipeline.Schema{Tables: []pipeline.Table{names}}
	c := func(name string) ref.ColumnRef { return ref.ColumnRef{Table: names.Ref, Column: name} }
	decide := func(name string, cat pipeline.Category, id mask.ID, role mask.Role) pipeline.Decision {
		return pipeline.Decision{Col: c(name), Category: cat, Confidence: pipeline.ConfCertain,
			Masker: id, Masked: true, Role: role}
	}
	cls := &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{
		c("first_name"): decide("first_name", pipeline.CatPersonName, mask.MaskerPersonName, mask.RoleGiven),
		c("aliases"):    decide("aliases", pipeline.CatPersonName, mask.MaskerPersonName, mask.RoleGiven),
		c("email"):      decide("email", pipeline.CatEmail, mask.MaskerEmail, mask.RoleFull),
		// A closed column: the masker has a vocabulary, the column does not.
		c("grade"):   decide("grade", pipeline.CatPersonName, mask.MaskerPersonName, mask.RoleGiven),
		c("name_id"): {Col: c("name_id"), Category: pipeline.CatNone},
	}}
	b := pipeline.RowBatch{
		Table: names.Ref,
		Cols:  []string{"name_id", "first_name", "aliases", "email", "grade"},
		Rows: [][]any{
			{int64(1), "Ada", []any{"Ada", nil, "Grace"}, "ada@example.org", "Bado"},
			{int64(2), "Grace", []any{}, "grace@example.org", "Balo"},
			{int64(3), nil, nil, nil, nil},
			{int64(4), "", []any{"", "Mary"}, "", ""},
			{int64(5), "Ada", "{Ada,NULL,\"Mary Ann\"}", "x@example.org", "Bado"},
		},
	}
	k := key(t, 0xE1)
	res := &recorder{inner: NewResidual(1000)}
	out, err := New(schema).Transform(b, cls, &k, res)
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}

	perColumn := map[string]int64{}
	for _, e := range res.emitted {
		perColumn[e.col.Column]++
		if e.path != "" {
			t.Errorf("an emitted count for %s carries the path %q; scalars and array elements count at the empty path",
				e.col.Column, e.path)
		}
	}
	// first_name: Ada, Grace, Ada; NULL and '' pass through unmasked.
	// aliases: Ada and Grace from row 1, Mary from row 4 ('' passes through),
	// and Ada and "Mary Ann" from row 5's literal.
	want := map[string]int64{"first_name": 3, "aliases": 5}
	for col, n := range want {
		if perColumn[col] != n {
			t.Errorf("%s: %d emitted counts, want %d (one per non-null non-empty masked cell or element)",
				col, perColumn[col], n)
		}
	}
	for _, col := range []string{"email", "grade", "name_id"} {
		if perColumn[col] != 0 {
			t.Errorf("%s is not an emitting column and was counted %d times", col, perColumn[col])
		}
	}

	// Every count answers for the value the target holds: Emitted over each
	// distinct masked first_name sums to the cells masked.
	seen := map[string]bool{}
	var total int64
	for _, row := range out.Rows {
		v, ok := row[1].(string)
		if !ok || v == "" || seen[v] {
			continue
		}
		seen[v] = true
		canon, _, err := mask.Canonical(mask.CatPersonName, mask.Value{Text: v}, mask.Constraints{})
		if err != nil {
			t.Fatal(err)
		}
		total += res.Emitted(c("first_name"), "", []byte(canon.Text))
	}
	if total != 3 {
		t.Errorf("Emitted over the target's first_name values sums to %d, want 3", total)
	}
	if n := res.Emitted(c("first_name"), "", []byte("not a name anybody drew")); n != 0 {
		t.Errorf("Emitted answers %d for a value the masker never produced", n)
	}
}
