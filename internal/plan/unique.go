// SPDX-License-Identifier: Apache-2.0

package plan

import (
	"errors"
	"fmt"

	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pipeline"
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
func (p *run) checkUniqueDomain() error {
	if p.cls == nil {
		return nil
	}
	for i := range p.tables {
		t := &p.tables[i]
		rows := p.plannedRows(t.Ref)
		if rows <= 0 {
			continue
		}
		for _, col := range t.Columns {
			cref := ref.ColumnRef{Table: t.Ref, Column: col.Name}
			d, ok := p.cls.Decisions[cref]
			if !ok || !d.Masked || !d.UniqueIndex {
				continue
			}
			if d.Category == "" || d.Category == pipeline.CatNone {
				// A masked column with no category has no generator to choose
				// between. checkWriteBack leaves it alone for the same reason
				// and internal/transform refuses it as "the decision names no
				// masker", which is the defect this would otherwise rename.
				continue
			}
			c, judged := p.constraintsOf(col)
			if !judged {
				// An enum, or a type mask has no family for. Every generator
				// answers a labelled column with one of its own labels (§5), so
				// the question this check asks is not the one that decides such
				// a column, and constraintsOf carries no labels to ask it with.
				// checkWriteBack skips the same set.
				continue
			}
			c.Unique = true
			c.Rows = rows

			id, err := mask.Pick(mask.Category(d.Category), c)
			switch {
			case err == nil:
			case errors.Is(err, mask.ErrNoRoom):
				// No masked value fits the column at all, whatever the row
				// count. checkWriteBack has already refused it as
				// plan.refused.unwritable, which names the type; saying it
				// again here in terms of d_required would name the wrong cause.
				continue
			case errors.Is(err, mask.ErrNoCategory):
				// No generator is registered for the category. transform's own
				// refusal names that, and it is a classification defect rather
				// than a domain one.
				continue
			default:
				var de *mask.DomainError
				if !errors.As(err, &de) {
					return refuse(CodeUniqueDomain, exitPlan, t.Ref,
						fmt.Sprintf("%s.%s is under a unique index and no masker for %s could be chosen: %v",
							t.Ref, col.Name, d.Category, err),
						event.Args{
							event.ArgTable:  t.Ref.String(),
							event.ArgColumn: col.Name,
							event.ArgReason: err.Error(),
						})
				}
				return refuse(CodeUniqueDomain, exitPlan, t.Ref,
					fmt.Sprintf("%s.%s is under a unique index: %s", t.Ref, col.Name, uniqueDomainReason(t.Ref, col.Name, de)),
					event.Args{
						event.ArgTable:  t.Ref.String(),
						event.ArgColumn: col.Name,
						event.ArgCount:  fmt.Sprintf("%d", de.Rows),
						event.ArgReason: uniqueDomainReason(t.Ref, col.Name, de),
					})
			}

			if id != d.Masker {
				d.Masker = id
				p.cls.Decisions[cref] = d
			}
		}
	}
	return nil
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
