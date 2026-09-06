// SPDX-License-Identifier: Apache-2.0

package plan

import (
	"sort"
	"strings"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// Polymorphic pairs (ARCHITECTURE.md §3.2, testdata/README.md trap 6).
//
// PostgreSQL knows nothing about a `<x>_type` / `<x>_id` pair, so lazyslice
// must not follow it — and must not be quiet about not following it. v1
// detects the pair and reports it; no row is selected because of it, and
// Plan.Virtual stays empty.
//
// Empty is a statement the walk keeps: followsAsParent and followsAsChild
// (plan.go) refuse a Virtual edge in both directions, so a Virtual edge
// introspect supplies selects no row either. A field that says "no inferred
// edge was followed" and a walk that quietly follows one would put rows in the
// slice for a reason the plan does not print (§3.5).
//
// The mapping half of §3.2 — sampling the distinct `_type` values and mapping
// them to tables the Rails or Django way, each mapping becoming a Virtual
// foreign key followed in the parent direction — is not implemented here. The
// deviation is recorded in this package's CLAUDE.md.

// polymorphicPair is one detected pair.
type polymorphicPair struct {
	Table     ref.TableRef
	TypeCol   string
	IDCol     string
	ObjectCol bool // the content_type_id / object_id spelling
}

// String names the pair the way the plan reports it: the table and both
// columns, and nothing about the values in them.
func (p polymorphicPair) String() string {
	return p.Table.String() + " (" + p.TypeCol + ", " + p.IDCol + ")"
}

// polymorphicPairs finds the pairs in a table set. A pair whose id column is
// already part of a declared foreign key is not one: PostgreSQL does know about
// that edge, and the planner follows it.
func polymorphicPairs(tables []pipeline.Table, outgoing map[ref.TableRef][]pipeline.ForeignKey) []polymorphicPair {
	var out []polymorphicPair
	for _, t := range tables {
		declared := map[string]bool{}
		for _, fk := range outgoing[t.Ref] {
			for _, c := range fk.ChildCols {
				declared[c] = true
			}
		}
		names := map[string]bool{}
		for _, c := range t.Columns {
			names[c.Name] = true
		}
		for _, c := range t.Columns {
			typeCol := c.Name
			var idCol string
			object := false
			switch {
			case strings.HasSuffix(typeCol, "_type") && len(typeCol) > len("_type"):
				idCol = strings.TrimSuffix(typeCol, "_type") + "_id"
			case typeCol == "content_type_id" && names["object_id"]:
				idCol = "object_id"
				object = true
			default:
				continue
			}
			if !names[idCol] || declared[idCol] {
				continue
			}
			out = append(out, polymorphicPair{Table: t.Ref, TypeCol: typeCol, IDCol: idCol, ObjectCol: object})
		}
	}
	sort.Slice(out, func(a, b int) bool {
		if out[a].Table != out[b].Table {
			return tableRefLess(out[a].Table, out[b].Table)
		}
		return out[a].TypeCol < out[b].TypeCol
	})
	return out
}
