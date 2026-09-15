// SPDX-License-Identifier: Apache-2.0

package plan

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/mask"
)

// Equality groups: the masker is chosen per foreign-key-connected set of
// columns, never per column (T-0132, ARCHITECTURE.md §5's 2026-09-14
// amendment).
//
// The defect this exists for is finding 3 of docs/reviews/2026-09-09/REVIEW.md.
// internal/classify propagates a *category* along a foreign key
// (classify.go's propagateKeys), so both ends of a key agree about what they
// hold. Nothing propagated the *masker*, and checkUniqueDomain then rewrote one
// column's masker at a time. On
//
//	tokens(id PRIMARY KEY, token TEXT UNIQUE)
//	items(id PRIMARY KEY, token TEXT REFERENCES tokens(token))
//
// the parent is under a unique index, so §5 escalated it to
// `credential_unique`; the child is not, so it kept the category default, the
// fixed literal. One value, two outputs: the parent became
// `lazyslice-invalid-…`, the child `$lazyslice$invalid`, and the load ended at
// exit 8 with the foreign key unvalidatable
// (docs/reviews/2026-09-09/evidence/fk_masker.log).
//
// Equality of masked values is what a foreign key is: `h` is a function of the
// category and the canonical value (mask.Apply), so two columns holding one
// value mask alike exactly when they agree on the category *and* the generator.
// The category was already unified. This unifies the generator:
//
//   - The group is the transitive closure of "appears at either end of a
//     declared foreign key", intersected with the masked columns this run will
//     load, and split by category — classify has already unified the categories
//     it could, and where it could not (propagateKeys' `type_conflict`) two
//     members cannot share one mapping whatever masker they are given.
//   - One masker is chosen for the whole group: the widest generator any member
//     needs, which is each member's own mask.Pick answer taken at its maximum.
//     A group with no unique member therefore keeps the category default, which
//     is what every non-unique masked column had before this landed.
//   - That choice is then checked against every member — writable there, wide
//     enough for every unique member's d_required, drawing on the same declared
//     value list where the members are closed columns, and emitting the same
//     number of values everywhere so a length-fitted generator does not produce
//     two shapes. No single generator that fits every member is exit 12
//     (plan.refused.equality_group) naming the group and every column in it, at
//     plan, before a key is fetched.
//
// internal/transform needs no change: it masks with `Decision.Masker`
// (transform.go's plan), so once the decisions agree the values do.

// groupMember is one masked column this run will load, reduced to what the
// choice reads. It is deliberately the same reduction checkWriteBack makes
// (constraintsOf), plus the two fields mask.Pick needs and constraintsOf does
// not carry: the unique-index flag and the planned row count.
type groupMember struct {
	cref ref.ColumnRef
	tbl  ref.TableRef
	col  string
	cat  mask.Category
	cur  mask.ID
	cons mask.Constraints
}

// maskedMembers lists every masked column with a category and a judgeable type,
// in a table this run will put rows in, in (schema, name) then column order.
//
// The filters are checkUniqueDomain's own, and each one is a decision recorded
// there: a table with no planned rows has nothing in the target to collide or
// to break a key; a masked column with no category has no generator to choose
// between; a type mask has no family for is one this package declines to judge
// rather than refuse.
func (p *run) maskedMembers() []groupMember {
	var out []groupMember
	for i := range p.tables {
		t := &p.tables[i]
		rows := p.plannedRows(t.Ref)
		if rows <= 0 {
			continue
		}
		for _, col := range t.Columns {
			cref := ref.ColumnRef{Table: t.Ref, Column: col.Name}
			d, ok := p.cls.Decisions[cref]
			if !ok || !d.Masked {
				continue
			}
			if d.Category == "" || d.Category == pipeline.CatNone {
				continue
			}
			c, judged := p.constraintsOf(col)
			if !judged {
				continue
			}
			c.Unique = d.UniqueIndex
			c.Rows = rows
			out = append(out, groupMember{
				cref: cref,
				tbl:  t.Ref,
				col:  col.Name,
				cat:  mask.Category(d.Category),
				cur:  d.Masker,
				cons: c,
			})
		}
	}
	return out
}

// equalityGroups partitions members into the sets whose masked values must be
// equal: connected by declared foreign keys, transitively, and agreeing on the
// category.
//
// The edges are p.fks — the schema's own keys, in constraint-name order, with
// partition leaves already folded into the table that stands for them. The
// inferred edges of §3.2 (`virtual_fks:`, the polymorphic pairs) are
// deliberately not here: the target never carries them as constraints, nothing
// validates them at load, and classify does not propagate a category along one
// either, so a group built over them would unify maskers on a join no stage
// downstream agrees exists. Tracker T-0154 owes that question an answer.
//
// The returned order, and the order within each group, is the order
// maskedMembers produced — tables in (schema, name) order, columns in schema
// order — so two runs over one snapshot refuse on the same group and rewrite
// the same decisions.
func (p *run) equalityGroups(members []groupMember) [][]groupMember {
	parent := map[ref.ColumnRef]ref.ColumnRef{}
	var find func(ref.ColumnRef) ref.ColumnRef
	find = func(c ref.ColumnRef) ref.ColumnRef {
		up, ok := parent[c]
		if !ok || up == c {
			return c
		}
		root := find(up)
		parent[c] = root
		return root
	}
	union := func(a, b ref.ColumnRef) {
		ra, rb := find(a), find(b)
		if ra == rb {
			return
		}
		// The smaller reference is always the root, so the partition does not
		// depend on the order the edges arrived in.
		if rb.Less(ra) {
			ra, rb = rb, ra
		}
		parent[rb] = ra
	}
	for _, fk := range p.fks {
		for i, child := range fk.ChildCols {
			if i >= len(fk.ParentCols) {
				break
			}
			union(
				ref.ColumnRef{Table: fk.Child, Column: child},
				ref.ColumnRef{Table: fk.Parent, Column: fk.ParentCols[i]},
			)
		}
	}

	type bucket struct {
		root ref.ColumnRef
		cat  mask.Category
	}
	at := map[bucket]int{}
	var groups [][]groupMember
	for _, m := range members {
		b := bucket{root: find(m.cref), cat: m.cat}
		if i, ok := at[b]; ok {
			groups[i] = append(groups[i], m)
			continue
		}
		at[b] = len(groups)
		groups = append(groups, []groupMember{m})
	}
	return groups
}

// groupWidth is the admissible domain of one generator over a whole group: the
// narrowest member's, because a generator is only as wide as the tightest
// column it has to write into.
func groupWidth(id mask.ID, g []groupMember) int64 {
	w := int64(math.MaxInt64)
	for _, m := range g {
		if d := mask.Admissible(id, m.cons); d < w {
			w = d
		}
	}
	return w
}

// widest is "the widest generator any member needs": each member's own
// mask.Pick answer, taken at its maximum over the group.
//
// It is not "the widest generator the category has". A group whose members are
// all outside a unique index needs nothing but the category default, and
// handing it the widest registered alternate would move every non-unique masked
// column in the schema onto a generator nobody asked for.
//
// Ties break on the id, so the choice does not depend on map order or on which
// member happened to be first.
func widest(g []groupMember, needed []mask.ID) mask.ID {
	var best mask.ID
	bestW := int64(-1)
	seen := map[mask.ID]bool{}
	for _, id := range needed {
		if seen[id] {
			continue
		}
		seen[id] = true
		switch w := groupWidth(id, g); {
		case w > bestW, w == bestW && id < best:
			best, bestW = id, w
		}
	}
	return best
}

// groupColumns renders the group for a refusal: every column in it, in order,
// identifiers only (THREAT_MODEL.md T4).
func groupColumns(g []groupMember) string {
	names := make([]string, 0, len(g))
	for _, m := range g {
		names = append(names, m.cref.String())
	}
	return strings.Join(names, ", ")
}

// fitsGroup is the "checked to fit every member" half of the rule. It asks four
// questions of the chosen generator, and a group has to pass all four:
//
//  1. Can every member's type hold what it emits? mask.Writable, the same
//     question checkWriteBack asks of the category default — asked again,
//     because the group's choice is not that default.
//  2. Is it wide enough for every member under a unique index? mask.Pick
//     guarantees this for the member the generator came from and says nothing
//     about the others: two unique columns joined by a key can have different
//     planned row counts, so the larger d_required has to be checked here.
//  3. Do the members accept the same declared values? A closed column — an enum,
//     a CHECK with a value list — is answered by every generator with one of
//     *its own* labels (mask/domain.go's labelValue), so two members whose lists
//     differ mask one input to two different labels however wide the generator
//     is. This question is asked before the count question below and not folded
//     into it, because a count is not the answer: two CHECK lists of the same
//     length and different contents report the same Domain() and mask
//     differently, which is finding 3's failure mode surviving the check written
//     to remove it (T-0132 review, finding 1).
//  4. Does it emit the same number of values in every member? Several
//     generators are length-fitted — credential_unique shortens its suffix to
//     fit a narrow column (mask/gen_credential.go), free text draws its length
//     from the column — so one masker over a varchar(25) and a text column is
//     still two mappings, and the foreign key would break exactly the way
//     finding 3 describes.
//
// What the four do not cover is Constraints.TypeTag: two members of one group
// can be different type families (inet and cidr, date and timestamp) whose
// generators branch on the tag and emit different text from one h. That is the
// same class of defect as this file's and is filed as **T-0159** rather than
// guessed at here, because the obvious rule — refuse a group whose members
// disagree on the tag — would also refuse the ordinary text-against-varchar
// pair, which masks alike.
func fitsGroup(g []groupMember, best mask.ID) error {
	cat := g[0].cat
	for _, m := range g {
		if !mask.Writable(cat, best, m.cons) {
			return refuseEquality(g, best, fmt.Sprintf("cannot be written into %s", m.cref), 0, causeUnwritable)
		}
		if !m.cons.Unique {
			continue
		}
		if d, req := mask.Admissible(best, m.cons), mask.Required(m.cons.Rows); d < req {
			return refuseEquality(g, best,
				fmt.Sprintf("emits %d distinct value(s) in %s, and the %d row(s) of %s planned here need %d at one-in-a-million collision odds",
					d, m.cref, m.cons.Rows, m.tbl, req),
				m.cons.Rows, causeRowCount)
		}
	}
	if len(g) < 2 {
		return nil
	}
	for _, m := range g[1:] {
		if !closedColumn(g[0].cons) && !closedColumn(m.cons) {
			continue
		}
		if sameClosedSet(g[0].cons, m.cons) {
			continue
		}
		return refuseEquality(g, best,
			fmt.Sprintf("draws its value from the labels %s accepts, which are not the labels %s accepts, so equal values there would not mask alike",
				g[0].cref, m.cref),
			0, causeDisagree)
	}
	gen, ok := mask.Get(best)
	if !ok {
		// Unreachable: best came out of mask.Pick, which reads the registry.
		return nil
	}
	first := gen.Domain(g[0].cons)
	for _, m := range g[1:] {
		if d := gen.Domain(m.cons); d != first {
			return refuseEquality(g, best,
				fmt.Sprintf("emits %d distinct value(s) in %s but %d in %s, so equal values there would not mask alike",
					first, g[0].cref, d, m.cref),
				0, causeDisagree)
		}
	}
	return nil
}

// closedColumn reports whether the column's admissible domain is decided by
// what it is declared to accept — an enum's labels, a CHECK's value list, a
// CHECK's exact length — rather than by its type alone.
//
// It asks mask rather than parsing a CHECK here: mask.ColumnDomain returns the
// label count for a closed column and a type-shaped number for an open one, so
// comparing the column against a copy of itself with the labels and the CHECKs
// removed is mask's own answer to "do these declarations bound it", with no
// second copy of the CHECK grammar in this package (mask/domain.go's checkValues
// is unexported and mask is a separate module, outside this task's paths).
//
// It answers true for an exact-length CHECK as well as for a value list, which
// is wider than the question fitsGroup asks. That is deliberate: it costs a
// refusal on a group whose two ends declare different exact lengths, where the
// masked value has to violate one of the two CHECKs or be length-fitted and
// therefore already caught by the count question.
func closedColumn(c mask.Constraints) bool {
	open := c
	open.EnumLabels = nil
	open.Checks = nil
	return mask.ColumnDomain(c) != mask.ColumnDomain(open)
}

// sameClosedSet reports whether two members accept the same declared values.
//
// It compares the declarations verbatim — the enum labels element-wise, and the
// CHECK expressions as pg_get_constraintdef rendered them — rather than the
// label lists mask parses out of them, for the reason closedColumn gives: the
// parser is unexported and in another module. That makes this check *stricter*
// than "the label sets differ": two members whose CHECKs spell one list two ways
// (a different column name inside the expression, a different literal cast) are
// refused although they would in fact mask alike. Refusing a group that would
// have worked is the recoverable direction — the operator sees the columns named
// at plan, with an escape — and admitting one that does not is exit 8 in the
// loader with rows already moved. **T-0158** owes the exact comparison: export
// the label list from mask and compare that.
func sameClosedSet(a, b mask.Constraints) bool {
	return slices.Equal(a.EnumLabels, b.EnumLabels) && slices.Equal(a.Checks, b.Checks)
}

// escapeCause is why a group was refused, which is the only thing that decides
// which escapes are printed. Offering an escape that does not apply to the cause
// is worse than offering none: it sends the operator to change a number that had
// nothing to do with it (T-0132 review, finding 2).
type escapeCause int

const (
	// causeUnwritable: the chosen generator's values cannot be written into one
	// member's type at all. No row count changes that.
	causeUnwritable escapeCause = iota
	// causeRowCount: a member under a unique index needs more distinct values
	// than the generator emits. This is the one cause a smaller --take, --cap or
	// --depth removes, and the only one "lower the row count" belongs on.
	causeRowCount
	// causeDisagree: the members do not mask alike under the chosen generator —
	// different output counts because it is length-fitted, or different closed
	// label sets. Rows have nothing to do with it.
	causeDisagree
)

// groupEscapes is what the operator can actually do about this refusal.
//
// For a group of more than one the --unmask escape is "every column of the group
// or none", never "one of them". Unmasking one end of a foreign key is exactly
// the failure this whole file exists to prevent: maskedMembers skips a column
// whose decision is not Masked, so the unmasked end keeps its production values
// while the other end is still masked, the key does not validate at load, and on
// a credential or free_text group the real values are in the target as well
// (T-0132 review, finding 2).
func groupEscapes(g []groupMember, cause escapeCause) string {
	cols := groupColumns(g)
	var s string
	if cause == causeRowCount {
		s = "lower the row count, or "
	}
	// mapping_file was the group escape's alternative; ADR-012 defers it past
	// v1 (docs/reviews/2026-09-09/REVIEW.md finding 10), so it is dropped here
	// too rather than pointing at an escape that exits 2 if taken.
	if len(g) > 1 {
		s += fmt.Sprintf(
			"--unmask every column of the group — %s, each with its own =REASON, all of them or none, "+
				"because unmasking one end of a foreign key copies that end's real values into the target "+
				"and still leaves the key unvalidatable", cols)
	} else {
		s += fmt.Sprintf("--unmask %s=REASON", cols)
	}
	return "; " + s
}

// refuseEquality is the exit-12 refusal §5's amendment asks for: it names the
// group and every column in it, not the one column the check happened to stop
// on, because the operator's escape has to be applied across the group.
//
// A group of more than one carries plan.refused.equality_group, a code of its
// own (internal/event/catalogue.yml). It used to borrow
// plan.refused.unique_domain, whose template opens "is under a unique index",
// and two of the three causes above reach this function with no member under a
// unique index at all — the length-mismatch branch fires on any foreign key
// between two masked text columns of different declared lengths — so the line
// an operator read asserted an index that was not in their schema (T-0132
// review, finding 3). A group of one is the per-column unique-index refusal it
// always was and keeps that code.
func refuseEquality(g []groupMember, best mask.ID, why string, rows int64, cause escapeCause) *Refusal {
	anchor := g[0]
	for _, m := range g {
		if m.cons.Unique {
			anchor = m
			break
		}
	}
	cols := groupColumns(g)
	var reason string
	code := CodeUniqueDomain
	if len(g) > 1 {
		code = CodeEqualityGroup
		reason = fmt.Sprintf(
			"%s are joined by foreign keys and must mask to the same value, but %s — the widest masker %s has for them — %s",
			cols, best, anchor.cat, why)
	} else {
		reason = fmt.Sprintf("%s cannot be masked: %s — the widest masker %s has for it — %s",
			cols, best, anchor.cat, why)
	}
	reason += groupEscapes(g, cause)

	args := event.Args{
		event.ArgTable:  anchor.tbl.String(),
		event.ArgColumn: anchor.col,
		event.ArgReason: reason,
	}
	if rows > 0 {
		args[event.ArgCount] = fmt.Sprintf("%d", rows)
	}
	r := refuse(code, exitPlan, anchor.tbl,
		fmt.Sprintf("%s is in a foreign-key equality group no masker fits: %s", anchor.cref, reason), args)
	r.Column = anchor.col
	return r
}
