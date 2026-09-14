// SPDX-License-Identifier: Apache-2.0

package plan

import (
	"errors"
	"fmt"

	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/mask"
)

// The unique-index domain rule (ARCHITECTURE.md §5).
//
// "Unique indexes (including expression indexes such as lower(email)):
// d_required = n² / 2ε at ε = 10⁻⁶, with n the planned row count of the table.
// The plan picks, within the column's category, the registered generator with
// the largest Domain() that fits the column; the explanation says uniqueness
// chose it ... There is never a retry counter. When even the largest generator
// has d < d_required, the column is refused by name at plan with exit 12,
// printing d, d_required, and the three escapes: lower n ..., --unmask, or
// mapping_file:."
//
// Every part of that existed except the caller. `mask.Pick` implements the
// choice and both refusals; `pipeline.Decision` has carried `UniqueIndex` since
// the classifier landed; `internal/transform` sets `Constraints.Unique` from the
// schema. What nothing did was *call* Pick — internal/transform used the rule
// pack's default masker for the category whatever the column's constraints
// were, and internal/plan/writeback.go said so in as many words: "the
// unique-index domain rule of ARCHITECTURE.md §5 ... is a separate plan-time
// check that has not landed".
//
// So the first anyone heard of a collision was the loader. On the ten schemas
// in testdata/torture/ that was three of ten runs, in three different shapes:
//
//   - django, `auth_group.name varchar(150) UNIQUE`, decided person_name: the
//     insert died on `auth_group_name_key` with SQLSTATE 23505, after the
//     extract had moved every row of every table.
//   - rails-activestorage, `active_storage_blobs.key UNIQUE`, decided
//     credential: the rows went in and the *index* could not be recreated over
//     them, which is the same collision one statement later and a worse message.
//   - supabase-auth, `refresh_tokens.token UNIQUE`, decided credential.
//
// testdata/regressions/001-unique-index-masking-collision.sql is the reduction.
//
// This check is where n first exists: it runs after walk, so `p.selected` holds
// the key set of every step and the row count is the planned one rather than the
// source's. That is also why it is not in internal/classify, which has the
// samples and the unique indexes but no idea how many rows the run will take.

// checkUniqueDomain applies §5's unique-index rule to every masked column under
// a unique index, in a table this run will load.
//
// Two outcomes. When some registered generator for the category clears
// d_required, the plan records it on the decision — which is what
// internal/transform then masks with, and what internal/emit writes into the
// yml — so `phone` on a unique column becomes `phone_unique` and `network_id`
// becomes `ip_unique` rather than colliding. When none does, the run stops here,
// before a key is fetched.
//
// The tables walked are the loaded ones only. A `SchemaOnly` step has no rows in
// the target, so nothing can collide in it, and refusing on a table this run
// will never write would be refusing on the source's shape rather than on the
// plan's — the same reason checkWriteBack runs after the skip and privilege pass.
//
// Since T-0132 the unit of the choice is the equality group and not the column:
// a masker is picked for every foreign-key-connected set of masked columns at
// once (equality.go), because changing one column's masker and not its
// neighbour's is how equal inputs came to mask to different outputs and the load
// died at exit 8 with the foreign key unvalidatable (finding 3 of
// docs/reviews/2026-09-09/REVIEW.md). A column no foreign key touches is a group
// of one and this is exactly the check it always was.
func (p *run) checkUniqueDomain() error {
	if p.cls == nil {
		return nil
	}
	for _, g := range p.equalityGroups(p.maskedMembers()) {
		if err := p.chooseGroupMasker(g); err != nil {
			return err
		}
	}
	return nil
}

// chooseGroupMasker applies §5's rule to one equality group: ask mask.Pick what
// each member needs, take the widest of those answers, check it against every
// member, and write it onto every member's decision.
//
// internal/transform masks with Decision.Masker, so the decisions are where the
// choice has to land; writing it onto every member is the whole of "transform
// masks every member identically".
func (p *run) chooseGroupMasker(g []groupMember) error {
	cat := g[0].cat
	needed := make([]mask.ID, 0, len(g))
	for _, m := range g {
		id, err := mask.Pick(cat, m.cons)
		switch {
		case err == nil:
			needed = append(needed, id)
		case errors.Is(err, mask.ErrNoRoom), errors.Is(err, mask.ErrNoCategory):
			// No masked value fits one of the members at all, whatever the row
			// count, or the category has no generator. checkWriteBack has
			// already refused both — plan.refused.unwritable, which names the
			// type — so this is unreachable after that pass; saying it again
			// here in terms of d_required would name the wrong cause. The whole
			// group is left alone rather than the member skipped, because a
			// group missing a member is a masker chosen for a set narrower than
			// the one that has to agree.
			return nil
		default:
			var de *mask.DomainError
			if !errors.As(err, &de) {
				return refuse(CodeUniqueDomain, exitPlan, m.tbl,
					fmt.Sprintf("%s.%s is under a unique index and no masker for %s could be chosen: %v",
						m.tbl, m.col, cat, err),
					event.Args{
						event.ArgTable:  m.tbl.String(),
						event.ArgColumn: m.col,
						event.ArgReason: err.Error(),
					})
			}
			reason := uniqueDomainReason(m.tbl, m.col, de)
			if len(g) > 1 {
				reason += equalityNote(g)
			}
			return refuse(CodeUniqueDomain, exitPlan, m.tbl,
				fmt.Sprintf("%s.%s is under a unique index: %s", m.tbl, m.col, reason),
				event.Args{
					event.ArgTable:  m.tbl.String(),
					event.ArgColumn: m.col,
					event.ArgCount:  fmt.Sprintf("%d", de.Rows),
					event.ArgReason: reason,
				})
		}
	}

	best := widest(g, needed)
	if best == "" {
		return nil
	}
	if err := fitsGroup(g, best); err != nil {
		return err
	}
	for _, m := range g {
		if m.cur == best {
			continue
		}
		d := p.cls.Decisions[m.cref]
		d.Masker = best
		p.cls.Decisions[m.cref] = d
	}
	return nil
}

// equalityNote is what a per-column refusal has to add when the column is not
// alone: the escape the operator is offered has to be applied across the group,
// and naming one end of a foreign key is naming half the problem.
//
// It corrects uniqueDomainReason's --unmask escape rather than only widening the
// refusal, because that sentence names a single column and applying it to a
// single member of a group is the failure this whole file exists to prevent:
// maskedMembers skips a column whose decision is not Masked, so the unmasked end
// keeps its production values while the other end is still masked, the key does
// not validate, and the real values are in the target as well (T-0132 review,
// finding 2).
func equalityNote(g []groupMember) string {
	return fmt.Sprintf(
		"; %s are joined by foreign keys and mask alike, so this refusal covers all of them and the "+
			"--unmask above has to name every one of them, each with its own =REASON — unmasking one end "+
			"of a foreign key copies that end's real values into the target and still leaves the key "+
			"unvalidatable", groupColumns(g))
}

// uniqueDomainReason is §5's refusal sentence: d, d_required and the three
// escapes, with the largest usable row count worked out rather than left as a
// formula.
//
// The row-count escape is printed only when there is one. `MaxRows` inverts
// d_required, and for a generator with a domain of 1 — which is what the
// `credential` category's fixed literal has — it is zero: no --take makes a
// constant unique, and printing "take at most 0 rows" as advice would be
// telling the operator to do something that is not a run.
func uniqueDomainReason(t ref.TableRef, col string, de *mask.DomainError) string {
	s := fmt.Sprintf(
		"the widest masker for %s emits %d distinct values and %d row(s) of %s need %d at one-in-a-million collision odds",
		de.Category, de.Domain, de.Rows, t, de.Required)
	if de.MaxRows > 0 {
		s += fmt.Sprintf("; take at most %d row(s) of %s", de.MaxRows, t)
	} else {
		s += "; no row count is small enough"
	}
	return s + fmt.Sprintf(", or --unmask %s.%s=REASON, or give the column a mapping_file", t, col)
}

// plannedRows is n for §5's rule: how many rows of this table the run will put
// in the target.
//
// It is the plan's number and never the source's. A lookup table is copied
// whole, so its n is the row count findLookups measured; a selected table's n is
// the size of its key set after the walk, which is what --take, --cap and
// --depth between them decided; and a table that is neither is SchemaOnly, has
// no rows in the target, and returns 0 so the caller skips it.
func (p *run) plannedRows(t ref.TableRef) int64 {
	if p.lookups[t] {
		return p.lookupRows[t]
	}
	if ks := p.selected[t]; ks != nil {
		return int64(ks.Len())
	}
	return 0
}
