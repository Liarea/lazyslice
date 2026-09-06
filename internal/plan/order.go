// SPDX-License-Identifier: Apache-2.0

package plan

import (
	"sort"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// Load order (ARCHITECTURE.md §3): Tarjan's strongly connected components,
// condensed, topologically sorted, with (schema, name) breaking every tie —
// inside a component and between components alike.
//
// The order is a courtesy and not a correctness requirement: foreign keys are
// created after the data (§11.1 item 6), so a cycle needs no deferral and no
// special case. testdata/nasty.sql's three-table cycle is the fixture that says
// so.

// tarjan returns the strongly connected components of the table graph, each
// component sorted by (schema, name), with the components themselves in a
// deterministic order.
func tarjan(tables []ref.TableRef, edges map[ref.TableRef][]ref.TableRef) [][]ref.TableRef {
	index := map[ref.TableRef]int{}
	low := map[ref.TableRef]int{}
	onStack := map[ref.TableRef]bool{}
	var stack []ref.TableRef
	var out [][]ref.TableRef
	next := 0

	var strongconnect func(v ref.TableRef)
	strongconnect = func(v ref.TableRef) {
		index[v] = next
		low[v] = next
		next++
		stack = append(stack, v)
		onStack[v] = true

		for _, w := range edges[v] {
			if _, ok := index[w]; !ok {
				strongconnect(w)
				if low[w] < low[v] {
					low[v] = low[w]
				}
				continue
			}
			if onStack[w] && index[w] < low[v] {
				low[v] = index[w]
			}
		}

		if low[v] == index[v] {
			var comp []ref.TableRef
			for {
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				onStack[w] = false
				comp = append(comp, w)
				if w == v {
					break
				}
			}
			sort.Slice(comp, func(a, b int) bool { return tableRefLess(comp[a], comp[b]) })
			out = append(out, comp)
		}
	}

	for _, t := range tables {
		if _, ok := index[t]; !ok {
			strongconnect(t)
		}
	}
	return out
}

// loadOrder condenses the components and topologically sorts them so that a
// table's parents come before it, with (schema, name) breaking every tie.
// selfCycles names the tables that reference themselves, which §3 reports as
// components of their own.
func loadOrder(tables []ref.TableRef, fks []pipeline.ForeignKey) (order []ref.TableRef, components [][]ref.TableRef) {
	in := map[ref.TableRef]bool{}
	for _, t := range tables {
		in[t] = true
	}
	sorted := append([]ref.TableRef(nil), tables...)
	sort.Slice(sorted, func(a, b int) bool { return tableRefLess(sorted[a], sorted[b]) })

	// Child -> parent for the component search; the direction does not change
	// which tables are in a cycle together.
	edges := map[ref.TableRef][]ref.TableRef{}
	selfCycle := map[ref.TableRef]bool{}
	for _, fk := range fks {
		if fk.Virtual || !in[fk.Child] || !in[fk.Parent] {
			continue
		}
		if fk.Child == fk.Parent {
			selfCycle[fk.Child] = true
			continue
		}
		edges[fk.Child] = append(edges[fk.Child], fk.Parent)
	}
	for k := range edges {
		sort.Slice(edges[k], func(a, b int) bool { return tableRefLess(edges[k][a], edges[k][b]) })
	}

	comps := tarjan(sorted, edges)
	compOf := map[ref.TableRef]int{}
	for i, c := range comps {
		for _, t := range c {
			compOf[t] = i
		}
	}
	// Components in a deterministic order of their own, by their first table.
	idx := make([]int, len(comps))
	for i := range idx {
		idx[i] = i
	}
	sort.Slice(idx, func(a, b int) bool { return tableRefLess(comps[idx[a]][0], comps[idx[b]][0]) })
	rank := make([]int, len(comps))
	for r, i := range idx {
		rank[i] = r
	}

	// Condensation edges point parent-component -> child-component, because a
	// parent is loaded first.
	succ := make([][]int, len(comps))
	indeg := make([]int, len(comps))
	seen := map[[2]int]bool{}
	for _, fk := range fks {
		if fk.Virtual || !in[fk.Child] || !in[fk.Parent] {
			continue
		}
		from, to := compOf[fk.Parent], compOf[fk.Child]
		if from == to || seen[[2]int{from, to}] {
			continue
		}
		seen[[2]int{from, to}] = true
		succ[from] = append(succ[from], to)
		indeg[to]++
	}

	ready := make([]int, 0, len(comps))
	for i := range comps {
		if indeg[i] == 0 {
			ready = append(ready, i)
		}
	}
	byRank := func(s []int) {
		sort.Slice(s, func(a, b int) bool { return rank[s[a]] < rank[s[b]] })
	}
	byRank(ready)

	for len(ready) > 0 {
		i := ready[0]
		ready = ready[1:]
		order = append(order, comps[i]...)
		next := append([]int(nil), succ[i]...)
		byRank(next)
		for _, j := range next {
			indeg[j]--
			if indeg[j] == 0 {
				ready = append(ready, j)
			}
		}
		byRank(ready)
	}
	// A cycle in the condensation is impossible, but a defensive tail keeps
	// every table in the order rather than silently dropping one.
	if len(order) < len(sorted) {
		placed := map[ref.TableRef]bool{}
		for _, t := range order {
			placed[t] = true
		}
		for _, t := range sorted {
			if !placed[t] {
				order = append(order, t)
			}
		}
	}

	for _, i := range idx {
		c := comps[i]
		if len(c) > 1 || selfCycle[c[0]] {
			components = append(components, c)
		}
	}
	return order, components
}
