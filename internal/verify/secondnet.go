// SPDX-License-Identifier: Apache-2.0

package verify

import (
	"context"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// The second net (ARCHITECTURE.md section 6 item 4).
//
// Eight of the classifier's ten value validators, folded into the six entries
// in validators.go (its two financial validators share one entry here, and so
// do its two network ones), run over the full contents of
// every column of the loaded target that is not fully masked by a category
// masker: every unmasked, non-opted-out column of a family this package can
// name and render, and the string leaves of every JSON column, masked or not. A
// column reaching the strong ratio is exit 9. The target is small, so this is a
// scan and not a sample, which is what makes it catch a column the 200-row
// sample under-represented.
//
// Three things it does not catch, stated here and in internal/verify/CLAUDE.md
// rather than implied:
//   - a category outside the rule pack (THREAT_MODEL.md T1 says so);
//   - person_name and free_text, the two validators backed by
//     internal/classify's embedded name dictionary, which this package cannot
//     import and will not copy (validators.go);
//   - a column of a family this package cannot name — famOther, and so a
//     tsvector or an enum (columns.go, netText).
//
// It also implements only the strong branch of ARCHITECTURE.md section 4's
// scoring: a weak ratio raised to `possible` by the neighbouring-column rule is
// not reached here (see CLAUDE.md).
//
// A column carrying an --unmask opt-out is deliberately outside this net: that
// is why the opt-out requires a reason and expires when the column's type
// changes (ARCHITECTURE.md section 8), rather than being a scan it would fail
// on every run.

// secondNet is item 4.
func (s *state) secondNet(ctx context.Context) error {
	scanned := int64(0)
	for _, step := range s.steps {
		table := s.tables[step.Table]
		for _, c := range table.Columns {
			col := ref.ColumnRef{Table: step.Table, Column: c.Name}
			mode, ok := s.netMode(col, c)
			if !ok {
				continue
			}
			scanned++
			if err := s.netColumn(ctx, col, mode); err != nil {
				return err
			}
		}
	}
	// Never beside a failure of the same name: Report.Checks is what the run
	// prints, and a passing "second_net" line under a failing one is a green
	// tick on the very check that produced exit 9. fk.go, counts.go,
	// residual.go and sample.go all guard the same way.
	if !s.failed(checkSecondNet) {
		s.pass(checkSecondNet, CodeSecondNetPassed, scanned)
	}
	return nil
}

// netMode is how one column is read by the net, and whether it is read at all.
type netMode struct {
	// leaves is true for a masked JSON column, whose string leaves are the net's
	// subject rather than the column's own value.
	leaves bool
	// text and digits say which validators apply, from the column's family.
	text   bool
	digits bool
}

func (s *state) netMode(col ref.ColumnRef, c pipeline.Column) (netMode, bool) {
	family, _ := s.shapeOf(c)
	d, has := s.decision(col)
	switch {
	case has && optedOut(d):
		// The one column deliberately outside this net (ARCHITECTURE.md section
		// 8): the opt-out carries a reason and expires on a type change instead.
		return netMode{}, false
	case document(family):
		// A masked document's masker was chosen per key by name, so the net
		// checks the leaves; an unmasked one had no masker at all, which is a
		// stronger reason to read its leaves and not a reason to skip it.
		return netMode{leaves: true, text: true}, true
	case has && d.Masked:
		return netMode{}, false
	case netText(family):
		return netMode{text: true}, true
	case numeric(family):
		return netMode{digits: true}, true
	}
	return netMode{}, false
}

// netColumn runs every applicable validator over one column's whole contents.
func (s *state) netColumn(ctx context.Context, col ref.ColumnRef, mode netMode) error {
	nonNull := int64(0)
	hits := make([]int64, len(validators))

	err := s.scanColumn(ctx, col.Table, col.Column, func(v any) error {
		for _, text := range s.netValues(v, mode) {
			nonNull++
			for i, val := range validators {
				if !applies(val, mode) {
					continue
				}
				if val.ok(text) {
					hits[i]++
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if nonNull < minValues {
		return nil
	}
	for i, val := range validators {
		if !applies(val, mode) || hits[i] == 0 {
			continue
		}
		if float64(hits[i])/float64(nonNull) < validatorThreshold {
			continue
		}
		s.fail(&Refusal{
			Code: CodeRefusedSecondNet, Exit: exitResidual, Check: checkSecondNet,
			Table: col.Table, Column: col.Column, Count: hits[i], Reason: val.name,
		})
		// One category per column: the column is already exit 9, and a second
		// line naming a second validator over the same values says nothing more
		// about what to do next.
		return nil
	}
	return nil
}

func applies(v validator, mode netMode) bool {
	return (mode.text && v.text) || (mode.digits && v.digits)
}

// netValues reduces one scanned value to the strings the validators run over:
// an array yields its elements, because section 4 classifies an array on its
// element type; a masked document yields its string leaves; a NULL yields
// nothing, because the ratio is over the non-NULL values.
func (s *state) netValues(v any, mode netMode) []string {
	if v == nil {
		return nil
	}
	if mode.leaves {
		ls := leaves(v)
		out := make([]string, 0, len(ls))
		for _, l := range ls {
			if l.str && l.text != "" {
				out = append(out, l.text)
			}
		}
		return out
	}
	if elems, ok := v.([]any); ok {
		out := make([]string, 0, len(elems))
		for _, e := range elems {
			if e == nil {
				continue
			}
			out = append(out, textOf(e))
		}
		return out
	}
	return []string{textOf(v)}
}
