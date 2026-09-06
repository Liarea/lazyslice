// SPDX-License-Identifier: Apache-2.0

package verify

import (
	"context"
	"fmt"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// Foreign-key validation (ARCHITECTURE.md section 6 item 5, ADR-005 "Verify"
// item 1: exit 8).
//
// It counts orphan rows rather than reading pg_constraint.convalidated. The
// loader adds every edge NOT VALID and then validates it, so trusting the
// catalogue here would be trusting the statement whose result this check
// exists to confirm — and a constraint someone dropped by hand afterwards
// would read as a target with nothing wrong with it.

// foreignKeys is item 5's "every FK validates".
func (s *state) foreignKeys(ctx context.Context) error {
	loaded := map[ref.TableRef]bool{}
	for _, step := range s.steps {
		loaded[step.Table] = true
	}
	checked := int64(0)
	for _, fk := range s.schema.FKs {
		if !s.recreatedEdge(fk, loaded) {
			continue
		}
		checked++
		var orphans int64
		if err := s.one(ctx, s.target, orphansSQL(fk), &orphans); err != nil {
			return fmt.Errorf("verify: checking the foreign key %s on %s: %w", fk.Name, fk.Child, err)
		}
		if orphans == 0 {
			continue
		}
		s.fail(&Refusal{
			Code: CodeRefusedFK, Exit: exitFK, Check: checkFK,
			Table: fk.Child, Column: fk.Name, Count: orphans,
			Reason: "rows reference a parent row the target does not hold",
		})
	}
	if !s.failed(checkFK) {
		s.pass(checkFK, CodeFKPassed, checked)
	}
	return nil
}

// recreatedEdge reports the edges this check applies to: the ones the loader
// recreates between two tables the target holds (internal/load/ddl's own
// filter). A virtual edge is inferred and has no constraint; an edge onto a
// partition leaf has no key in the target to reference (section 11.1); an edge
// whose child or parent is not in the plan is not in the target either.
func (s *state) recreatedEdge(fk pipeline.ForeignKey, loaded map[ref.TableRef]bool) bool {
	if fk.Virtual || fk.NotRecreatable {
		return false
	}
	if len(fk.ChildCols) == 0 || len(fk.ChildCols) != len(fk.ParentCols) {
		return false
	}
	if !loaded[fk.Child] || !loaded[fk.Parent] {
		return false
	}
	child, ok := s.tables[fk.Child]
	if !ok || child.Parent != nil {
		return false
	}
	parent, ok := s.tables[fk.Parent]
	return ok && parent.Parent == nil
}

// failed reports whether a check has already recorded a failure, so that a pass
// is not appended beside one.
func (s *state) failed(name string) bool {
	for _, r := range s.failures {
		if r.Check == name {
			return true
		}
	}
	return false
}
