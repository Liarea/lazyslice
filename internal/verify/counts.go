// SPDX-License-Identifier: Apache-2.0

package verify

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// Row counts and sequences (ARCHITECTURE.md section 6 item 5).
//
// Every step with mode ChildOK or ParentOnly holds exactly Keys.Len() rows;
// every Lookup step holds its bounded count; schema-only steps and lookups with
// no source rows are reported, not failed. Sequences are reset in the
// strict-NULL form, which is what stops the application's first INSERT from
// colliding with a copied row (THREAT_MODEL.md T8).
//
// ADR-005's exit table has no code of its own for either: a count or a sequence
// that disagrees with the plan is a failure of the movement of rows, so both
// take "7 extract or load". internal/verify/CLAUDE.md records the reading.

// rowCounts is item 5's count check. It also fills Report.Rows, which is what
// the run prints.
func (s *state) rowCounts(ctx context.Context) error {
	checked := int64(0)
	for _, step := range s.plan.Steps {
		t, ok := s.tables[step.Table]
		if !ok || t.Parent != nil {
			continue
		}
		got, err := s.countRows(ctx, step.Table)
		if err != nil {
			return err
		}
		s.rows[step.Table] = got

		want, ok := s.expected(step)
		if !ok {
			s.report(checkRowCount, CodeRowCountReported, step.Table, "", got)
			continue
		}
		checked++
		if got == want {
			continue
		}
		s.fail(&Refusal{
			Code: CodeRefusedRowCount, Exit: exitLoad, Check: checkRowCount,
			Table: step.Table, Count: got, Reason: "the plan copies " + strconv.FormatInt(want, 10),
		})
	}
	if !s.failed(checkRowCount) {
		s.pass(checkRowCount, CodeRowCountPassed, checked)
	}
	return nil
}

// expected is the row count one step must have in the target, and whether the
// step has one at all.
//
// A keyed step's count is its key set's size, which is the plan's own statement
// about what will be copied. A lookup's is the loader's committed count: the
// planner proved the table under section 3's lookup ceiling and copies it
// whole, but it records no key set for it (pipeline.Step: Keys is nil for
// Lookup), so the plan carries no number for this check to use. A lookup that
// copied nothing is reported rather than failed, which is what item 5 says
// about a lookup with no source rows.
func (s *state) expected(step pipeline.Step) (int64, bool) {
	switch step.Mode {
	case pipeline.ChildOK, pipeline.ParentOnly:
		if step.Keys == nil {
			return 0, true
		}
		return int64(step.Keys.Len()), true
	case pipeline.Lookup:
		n := s.lr.Rows[step.Table]
		if n == 0 {
			return 0, false
		}
		return n, true
	case pipeline.SchemaOnly:
		return 0, false
	}
	return 0, false
}

// sequences is item 5's "sequences reset in the strict-NULL form": last_value is
// the column's maximum and is_called is true, or, over a column with no rows,
// last_value is 1 and is_called is false — which is what
// setval(seq, coalesce(max(c), 1), max(c) IS NOT NULL) leaves behind.
func (s *state) sequences(ctx context.Context) error {
	checked := int64(0)
	for _, step := range s.steps {
		t := s.tables[step.Table]
		for _, seq := range t.Sequences {
			column := sequenceColumn(t, seq)
			if column == "" {
				s.report(checkSequences, CodeSequenceUnowned, step.Table, seq.Name, 0)
				continue
			}
			checked++
			if err := s.sequence(ctx, step.Table, seq, column); err != nil {
				return err
			}
		}
	}
	if !s.failed(checkSequences) {
		s.pass(checkSequences, CodeSequencesPassed, checked)
	}
	return nil
}

func (s *state) sequence(ctx context.Context, t ref.TableRef, seq pipeline.SequenceDef, column string) error {
	// Which relation to read is a question for the target, not an assumption
	// from the source's name (sequenceNameSQL).
	var name string
	if err := s.one(ctx, s.target, sequenceNameSQL(t, column, seq.Name), &name); err != nil {
		return fmt.Errorf("verify: resolving the sequence behind %s.%s in the target: %w", t, column, err)
	}
	if name == "" {
		// Neither the target's own ownership nor the source's name names a
		// relation the target has. Before sequenceNameSQL existed this branch
		// was a 42P01 out of sequenceSQL and it failed the run; reporting it
		// instead would turn "§11.1 did not create this sequence" into a green
		// note, which is exactly the target THREAT_MODEL.md T8 describes as
		// looking complete and not being.
		s.fail(&Refusal{
			Code: CodeRefusedSequence, Exit: exitLoad, Check: checkSequences,
			Table: t, Column: column, Reason: reasonNoSequence,
		})
		return nil
	}
	var last int64
	var called bool
	if err := s.one(ctx, s.target, sequenceSQL(name), &last, &called); err != nil {
		return fmt.Errorf("verify: reading the sequence %s in the target: %w", name, err)
	}
	var maxValue *int64
	if err := s.one(ctx, s.target, maxSQL(t, column), &maxValue); err != nil {
		return fmt.Errorf("verify: reading the maximum of %s.%s in the target: %w", t, column, err)
	}

	if maxValue == nil {
		if called {
			s.fail(&Refusal{
				Code: CodeRefusedSequence, Exit: exitLoad, Check: checkSequences,
				Table: t, Column: column, Reason: reasonNotCalled,
			})
		}
		return nil
	}
	switch {
	case !called:
		s.fail(&Refusal{
			Code: CodeRefusedSequence, Exit: exitLoad, Check: checkSequences,
			Table: t, Column: column, Reason: reasonNotCalled,
		})
	case last != *maxValue:
		s.fail(&Refusal{
			Code: CodeRefusedSequence, Exit: exitLoad, Check: checkSequences,
			Table: t, Column: column, Count: last, Reason: reasonLastValue,
		})
	}
	return nil
}

// sequenceColumn is the column a sequence belongs to: the catalogue's ownership
// when it records one, and otherwise the column whose default calls it.
//
// It is internal/load/ddl's own resolution, and it has to be: a sequence the
// loader could not attribute to a column is a sequence the loader did not reset,
// and this check would fail a target that is exactly what the loader promised.
// pagila records no ownership at all for any of its twelve sequences.
func sequenceColumn(t *pipeline.Table, s pipeline.SequenceDef) string {
	if s.Column != "" {
		return s.Column
	}
	needles := []string{"'" + strings.ReplaceAll(s.Name, "'", "''") + "'"}
	if i := strings.Index(s.Name, "."); i >= 0 {
		needles = append(needles, "'"+strings.ReplaceAll(s.Name[i+1:], "'", "''")+"'")
	}
	for _, c := range t.Columns {
		if c.Default == "" {
			continue
		}
		for _, needle := range needles {
			if strings.Contains(c.Default, needle) {
				return c.Name
			}
		}
	}
	return ""
}
