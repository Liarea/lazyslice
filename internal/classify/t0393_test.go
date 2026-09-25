// SPDX-License-Identifier: Apache-2.0

package classify

import (
	"strings"
	"testing"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// T-0393 (the 2026-09-25 JSON red team, round 1, attacker 1's A12, A13 and
// A11): a jsonb column whose own name matches a personal rule that does not
// accept jsonb -- only special_category and semi_structured do -- is decided
// semi_structured by its type (testdata/regressions/008's fix), and before
// this task that decision was indistinguishable from a jsonb nothing names:
// pipeline.Decision.LeafMap handed transform the per-leaf map and every leaf
// with no signal of its own was copied, names, streets, passwords and birth
// dates at exit 0. The decision now carries the name's category (NameHit), so
// LeafMap is nil and LeafNameCategory names what every leaf is masked under.
//
// The documents are the red team's own, and deliberately signal-free under
// the name rules and the validators: a map that calls every key `none` is
// what made the leaves copyable.
func TestADocumentWhoseOwnNameIsPersonalCarriesTheNameHit(t *testing.T) {
	t.Parallel()
	members := ref.TableRef{Schema: "public", Name: "t393_members"}
	// The documents are testdata/regressions/046's, byte for byte.
	cases := []struct {
		column string
		want   pipeline.Category
		doc    string
	}{
		{"full_name", pipeline.CatPersonName, `{"given": "Wren", "family": "Calloway"}`},
		{"home_address", pipeline.CatAddress, `{"a": "Larkspur Row", "b": "Upper Dunmore", "c": "DN7"}`},
		{"passwords", pipeline.CatCredential, `{"current": "hunter2x", "previous": "tr0ub4dor"}`},
		{"date_of_birth", pipeline.CatPersonDate, `{"d": "1987-03-14"}`},
		{"national_id", pipeline.CatNationalID, `{"scheme": "UK-NINO", "holder": "Wren Calloway"}`},
		{"emails", pipeline.CatEmail, `{"owner": "Wren Calloway"}`},
		{"notes", pipeline.CatFreeText, `{"entry": "Wren Calloway seen for follow-up"}`},
		{"by_phone", pipeline.CatPhone, `{"label": "Ysolde Brackenridge"}`},
	}
	cols := []pipeline.Column{tc("id", "integer"), tc("prefs", "jsonb"), tc("medical_history", "jsonb")}
	samples := mapSampler{
		col(members, "id"):              anyOf(int64(1), int64(2), int64(3), int64(4)),
		col(members, "prefs"):           anyOf(`{"theme": "dark"}`, `{"theme": "light"}`, `{"theme": "dark"}`, `{"theme": "auto"}`),
		col(members, "medical_history"): anyOf(`{"entry": "seen"}`, `{"entry": "seen"}`, `{"entry": "seen"}`, `{"entry": "seen"}`),
	}
	for _, c := range cases {
		cols = append(cols, tc(c.column, "jsonb"))
		samples[col(members, c.column)] = anyOf(c.doc, c.doc, c.doc, c.doc)
	}
	schema := &pipeline.Schema{Tables: []pipeline.Table{tt("public", "t393_members", []string{"id"}, cols...)}}

	cls, err := New().Classify(schema, samples, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}

	for _, c := range cases {
		t.Run(c.column, func(t *testing.T) {
			d := decision(t, cls, col(members, c.column))
			if !d.Masked || d.Category != pipeline.CatSemiStruct {
				t.Fatalf("%s = %+v, want a masked semi_structured column", c.column, d)
			}
			if !strings.Contains(d.Reason, "jsonb is not an accepted type for "+string(c.want)) {
				t.Errorf("%s reason = %q, want the rejected %s name hit named", c.column, d.Reason, c.want)
			}
			if d.NameHit != c.want {
				t.Errorf("%s NameHit = %q, want %q", c.column, d.NameHit, c.want)
			}
			if d.LeafKeys == nil {
				t.Fatalf("%s carries no LeafKeys, so this test does not reach the path that copied", c.column)
			}
			// Every key none is the map that copied every leaf before the
			// fix, which is what makes 046's not-copied check see the bug.
			for k, cat := range d.LeafKeys {
				if cat != pipeline.CatNone {
					t.Errorf("%s LeafKeys[%q] = %q, want none: the fixture's document must be signal-free", c.column, k, cat)
				}
			}
			if m := d.LeafMap(); m != nil {
				t.Errorf("%s LeafMap() = %v, want nil: every leaf's key is none, so a map copies every leaf", c.column, m)
			}
			if got := d.LeafNameCategory(); got != c.want {
				t.Errorf("%s LeafNameCategory() = %q, want %q", c.column, got, c.want)
			}
		})
	}

	// The controls. A jsonb nothing names keeps its map, so a configuration
	// document is still copied leaf by leaf; a special_category name accepts
	// jsonb, so its decision is special_category and no name hit is carried.
	t.Run("prefs", func(t *testing.T) {
		d := decision(t, cls, col(members, "prefs"))
		if d.NameHit != "" || d.LeafNameCategory() != "" || d.LeafMap() == nil {
			t.Errorf("prefs = NameHit %q, LeafMap %v: a document nothing names keeps its per-leaf map", d.NameHit, d.LeafMap())
		}
	})
	t.Run("medical_history", func(t *testing.T) {
		d := decision(t, cls, col(members, "medical_history"))
		if d.Category != pipeline.CatSpecial || d.NameHit != "" || d.LeafMap() != nil {
			t.Errorf("medical_history = %+v, want special_category with no name hit and no map", d)
		}
	})
}
