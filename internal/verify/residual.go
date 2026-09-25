// SPDX-License-Identifier: Apache-2.0

package verify

import (
	"context"
	"errors"
	"strings"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/internal/textsig"
	"github.com/Liarea/lazyslice/mask"
)

// The residual scan (ARCHITECTURE.md section 6 items 1 to 3, THREAT_MODEL.md
// T12). It is the only check that tests the *masker* rather than the
// classifier: it answers "did a value we meant to mask reach the target
// unchanged?".
//
// Every masked column of the target is streamed, every value canonicalised the
// way internal/transform canonicalised the source's (value.go), and the filter
// tested. A hit is confirmed against the source in the short second
// transaction, indexable probe first. A confirmed hit is exit 9 naming table and
// column; a hit the source says is absent is counted and printed; a hit that
// cannot be tested at all is exit 9 with the reason, never a printed note.

// outcome is what confirmation said about one hit.
type outcome int

const (
	// confirmed: the source still holds this value in this column.
	confirmed outcome = iota
	// absent: the source says no. A filter false positive, or the source
	// changed since the snapshot.
	absent
	// untestable: no probe could answer. Section 6 item 3 makes this exit 9.
	untestable
)

// errStop ends a column scan early. It never leaves this package.
var errStop = errors.New("verify: stop scanning")

// residualScan is section 6 item 3.
func (s *state) residualScan(ctx context.Context) error {
	tested := int64(0)
	for _, step := range s.steps {
		table := s.tables[step.Table]
		for _, col := range maskedColumns(s.cls, step.Table) {
			column, ok := columnOf(table, col.Column)
			if !ok {
				// A decision about a column the schema does not have is a
				// classification and a schema that disagree; the row-count and
				// FK checks are the ones that would notice, and there is
				// nothing here to scan.
				continue
			}
			d, _ := s.decision(col)
			n, stop, err := s.scanMasked(ctx, step, col, column, d)
			tested += n
			if err != nil {
				return err
			}
			if stop {
				return nil
			}
		}
	}
	// One rule for all seven checks: a pass is never appended beside a failure
	// of the same name, or the report carries a green tick for the check that
	// produced the exit code (fk.go, counts.go, secondnet.go, sample.go).
	if !s.failed(checkResidual) {
		s.pass(checkResidual, CodeResidualPassed, tested)
	}
	if !s.failed(checkUnconfirmable) {
		s.pass(checkUnconfirmable, CodeResidualProbes, s.probes)
	}
	return nil
}

// scanMasked streams one masked column and tests every value against the
// filter. It reports how many values it tested and whether the scan stopped on
// a failing hit.
//
// A column whose masker has a vocabulary (mask.Emitting, ADR-015) is scanned
// the same way, and a hit inside that vocabulary is tallied rather than
// probed; explain.go decides at the end of the column what the tally proves.
// Every other hit — any hit in any other column, and a hit mask.Emits rejects
// in this one — takes section 6 item 3's column probe at once, as it always
// has.
func (s *state) scanMasked(
	ctx context.Context,
	step pipeline.Step,
	col ref.ColumnRef,
	column pipeline.Column,
	d pipeline.Decision,
) (int64, bool, error) {
	family, array := s.shapeOf(column)
	tested := int64(0)
	stopped := false
	before := s.unconfirmed
	ex := s.newExplainer(step, col, column, d, family, array)

	visit := func(v any, ids []any) error {
		if v == nil {
			return nil
		}
		var hits []hit
		switch {
		case document(family):
			hits = s.documentHits(col, v)
		case array:
			hs, err := s.arrayHits(col, d.Category, v)
			if err != nil {
				// A masked array column this stage cannot split is a column it
				// cannot scan at all: every filter entry for it is per element
				// and nothing here can produce an element to test. Item 3's
				// rule for a finding that cannot be tested is exit 9 with the
				// reason, and it applies to a value that cannot be *reached*
				// for the same argument — the alternative is a green tick over
				// the one column class ARCHITECTURE.md section 6 item 1 is
				// blind to (tracker T-0129, THREAT_MODEL.md T12).
				s.fail(&Refusal{
					Code: CodeRefusedUnconfirmable, Exit: exitResidual, Check: checkUnconfirmable,
					Table: col.Table, Column: col.Column, Count: 1,
					Reason: reasonArrayLiteral, err: err,
				})
				stopped = true
				return errStop
			}
			hits = hs
		default:
			hits = s.scalarHits(col, d.Category, v)
		}
		tested++
		for _, h := range hits {
			if ex.tally(h, ids) {
				if ex.flushFull(ctx) {
					stopped = true
					return errStop
				}
				continue
			}
			done, err := s.handle(ctx, col, h)
			if err != nil {
				return err
			}
			if done {
				stopped = true
				return errStop
			}
		}
		return nil
	}

	var err error
	if ex.identified() {
		n := len(ex.idCols)
		err = s.scanRows(ctx, col.Table, append(append([]string{}, ex.idCols...), col.Column),
			func(row []any) error { return visit(row[n], row[:n]) })
	} else {
		err = s.scanColumn(ctx, col.Table, col.Column, func(v any) error { return visit(v, nil) })
	}
	if err != nil && !errors.Is(err, errStop) {
		return tested, stopped, err
	}
	if !stopped {
		done, err := ex.finish(ctx)
		if err != nil {
			return tested, stopped, err
		}
		stopped = done
	}
	// One line per column, not one per hit: a false-positive rate of 10^-6 over
	// a large column is still a handful of hits, and a report is read by a
	// person (ARCHITECTURE.md section 6 item 3).
	if n := s.unconfirmed - before; n > 0 {
		s.report(checkResidual, CodeUnconfirmed, col.Table, col.Column, n)
	}
	return tested, stopped, nil
}

// hit is one value of the target that the filter says may have been masked.
type hit struct {
	// value is the value as the target holds it, bound as the confirmation
	// probe's parameter. It never reaches an event, a report or an error.
	value any
	// canon is value's canonical bytes under the column's category, the bytes
	// the filter was tested with. It is empty for a document hit, which ADR-015
	// never explains, and is what explain.go tallies a scalar or an element by.
	canon []byte
	// leaf is true when the value came from inside a document, in which case
	// neither probe of section 6 can express the question.
	leaf bool
}

// scalarHits tests one scalar value against the filter.
func (s *state) scalarHits(col ref.ColumnRef, cat pipeline.Category, v any) []hit {
	canon, ok, err := canonicalOf(mask.Category(cat), v)
	if err != nil || !ok {
		// A value the canonicaliser refuses is a value transform could not have
		// put in the filter either: mask.Apply canonicalises before it masks, so
		// a refusal there was a refusal of the whole cell and the run stopped.
		return nil
	}
	if !s.res.MayContain(col, "", canon) {
		return nil
	}
	return []hit{{value: v, canon: canon}}
}

// arrayHits tests each element of an array column, which internal/transform
// masked element-wise and recorded under the column's empty path.
//
// The carriers are internal/transform's own, in its order (its cell): a []any
// for an array type the pool's map knows, and the server's text output form for
// one it does not — a citext[], and every other array of an extension's base
// type, which arrives as the single string "{a@b.test,c@d.test}" (T-0118).
// Transform parses that literal and records one filter entry per element, so a
// scan that canonicalised the whole literal would test bytes nothing ever added
// and pass green over exactly the column class T-0118 enables (T-0129).
//
// A literal that will not parse is an error and never a fall back to the scalar
// path: the scalar path would test the whole value against per-element entries,
// which is the same green tick by a shorter route. The caller makes it exit 9
// naming the column.
//
// Any other carrier falls through to the scalar path, because that is what
// transform's cell does with it: it masks such a value as one scalar and
// records one entry for the whole value, so one entry for the whole value is
// what there is to test.
func (s *state) arrayHits(col ref.ColumnRef, cat pipeline.Category, v any) ([]hit, error) {
	switch t := v.(type) {
	case []any:
		var out []hit
		for _, e := range t {
			if e == nil {
				continue
			}
			out = append(out, s.scalarHits(col, cat, e)...)
		}
		return out, nil
	case string:
		return s.literalHits(col, cat, t)
	case []byte:
		return s.literalHits(col, cat, string(t))
	}
	return s.scalarHits(col, cat, v), nil
}

// literalHits splits one array literal with internal/transform's own grammar
// (arrayliteral.go) and tests each element.
func (s *state) literalHits(col ref.ColumnRef, cat pipeline.Category, text string) ([]hit, error) {
	elems, err := arrayLiteralElements(text)
	if err != nil {
		return nil, err
	}
	var out []hit
	for _, e := range elems {
		out = append(out, s.scalarHits(col, cat, e)...)
	}
	return out, nil
}

// documentHits tests a json, jsonb or hstore column both ways internal/transform
// may have recorded it: the whole document, under semi_structured at the empty
// path, which is what a collapse records; and every string leaf under free_text
// at its own path, which is what a walk records
// (internal/transform/CLAUDE.md, "the residual-filter contract").
//
// That is unchanged by per-leaf categories (T-0272): transform records a
// masked string leaf under free_text's canonical form whichever category's
// masker replaced it, and records nothing for a leaf it copied, so every
// string leaf is tested here the same way and a copied one is not in the
// filter to be found (jsonleaf.go).
//
// A *number* leaf is recorded in the filter too and is deliberately not tested
// here. internal/transform redraws a number leaf over a domain of 10^6 values
// (10^5 for a fractional one), so the target's number at that path matches the
// source's canonical bytes about N/10^6 of the time on a run that masked
// correctly — and every leaf hit is untestable, and therefore an unconditional
// exit 9 with a reason that names no action. That is a run refused for a leak
// that did not happen, at a rate that grows with the row count. It is the same
// argument internal/transform made when it kept a boolean leaf out of the
// filter ("a two-valued domain"), carried to the domain one size up: a value
// redrawn over a small domain carries no residual signal, so a match on it is
// evidence about the domain and not about the masker. A string leaf keeps its
// entry, because the free_text masker's domain is not small.
func (s *state) documentHits(col ref.ColumnRef, v any) []hit {
	var out []hit
	if canon, ok, err := canonicalOf(mask.Category(pipeline.CatSemiStruct), v); err == nil && ok {
		if s.res.MayContain(col, "", canon) {
			out = append(out, hit{value: v})
		}
	}
	for _, l := range leaves(v) {
		if !l.str {
			continue
		}
		canon, ok, err := canonicalOf(mask.Category(pipeline.CatFreeText), l.text)
		if err != nil || !ok {
			continue
		}
		if s.res.MayContain(col, l.path, canon) {
			out = append(out, hit{value: l.text, leaf: true})
		}
	}
	out = append(out, s.keyHits(col, v)...)
	return out
}

// keyHits tests every object key of a document against the filter, under
// whichever of the three strong validators the key matches (T-0137,
// docs/reviews/2026-09-09/REVIEW.md finding 8).
//
// internal/transform's maskKey (through walk) now records a masked key's
// entry at the key's own path in the *target* document — the position the
// masked key itself occupies, not the source one (T-0137 review round,
// finding 1; json.go). documentKeys walks the target the same way and hands
// back that same path per key, so the two sides agree on the spelling. Every
// key hit is still a document-leaf hit like the whole-document one above it:
// it lives inside the document and neither of section 6 item 3's probes can
// ask a column "does this key appear inside you", so it is untestable and
// exit 9 on any match (`confirm`'s `h.leaf` branch).
func (s *state) keyHits(col ref.ColumnRef, v any) []hit {
	var out []hit
	for _, occ := range documentKeys(v) {
		cat, ok := strongKeyCategory(occ.name)
		if !ok {
			continue
		}
		canon, ok, err := canonicalOf(mask.Category(cat), occ.name)
		if err != nil || !ok {
			continue
		}
		if s.res.MayContain(col, occ.path, canon) {
			out = append(out, hit{value: occ.name, leaf: true})
		}
	}
	return out
}

// strongKeyCategory names the category a JSON object key matches under one of
// the three strong validators, or "" for anything else. It is
// internal/transform's own `keyCategory` (`json.go`) restated here — the two
// packages may not import each other, and it is the same "second copy" this
// package's `catalog.go` already keeps for a `CHECK` or a `DEFAULT` literal
// (`strongCatalogHit`): a shape guess (address, credential) is deliberately
// excluded, because a key that only *looks* like an identifier is not the
// precise parse this residual check requires — it tests only the columns
// internal/transform already masked, and a confirmed hit there has no
// `--unmask` escape (THREAT_MODEL.md T12), so a shape guess here would be a
// refusal on a loaded target with no way past it at all.
func strongKeyCategory(name string) (pipeline.Category, bool) {
	s := strings.TrimSpace(name)
	switch {
	case textsig.ValidEmail(s):
		return pipeline.CatEmail, true
	case textsig.ValidPhone(s):
		return pipeline.CatPhone, true
	case textsig.ValidLuhn(s):
		return pipeline.CatFinancial, true
	}
	return "", false
}

// handle confirms one hit and records what came of it. It reports whether the
// scan should stop, which it does on any outcome that fails the run: there is
// nothing more to learn, and every further probe is another candidate value in
// the source's log (THREAT_MODEL.md T4).
func (s *state) handle(ctx context.Context, col ref.ColumnRef, h hit) (bool, error) {
	return s.handleAs(ctx, col, h, reasonStillHolds)
}

// handleAs is handle with the fixed phrase a confirmed hit is refused under:
// reasonStillHolds for an ordinary hit, reasonOverCount for a value ADR-015's
// count check sent back to the column probe (explain.go).
func (s *state) handleAs(ctx context.Context, col ref.ColumnRef, h hit, whenConfirmed string) (bool, error) {
	verdict, reason := s.confirm(ctx, col, h)
	switch verdict {
	case confirmed:
		s.fail(&Refusal{
			Code: CodeRefusedResidual, Exit: exitResidual, Check: checkResidual,
			Table: col.Table, Column: col.Column, Count: 1,
			Reason: whenConfirmed,
		})
		return true, nil
	case untestable:
		s.fail(&Refusal{
			Code: CodeRefusedUnconfirmable, Exit: exitResidual, Check: checkUnconfirmable,
			Table: col.Table, Column: col.Column, Count: 1, Reason: reason,
		})
		return true, nil
	case absent:
		s.unconfirmed++
		return false, nil
	}
	return false, nil
}

// confirm is section 6 item 3's two probes, indexable form first, against the
// short second transaction.
//
// A leaf is never probed. Both probes ask whether the *column* holds the value,
// and a leaf's value lives inside a document, so neither can answer; item 3
// makes a hit that cannot be tested exit 9, and spending two probes to reach
// the same answer would put the candidate in the source's log for nothing.
func (s *state) confirm(ctx context.Context, col ref.ColumnRef, h hit) (outcome, string) {
	if h.leaf {
		return untestable, reasonNoProbe
	}
	r, err := s.shortReader(ctx)
	if err != nil {
		return untestable, reasonSourceClosed
	}
	exists, ok := s.probe(ctx, r, probeSQL(col.Table, col.Column), h.value)
	switch {
	case !ok:
		return untestable, s.probeFailure()
	case exists:
		return confirmed, ""
	}
	exists, ok = s.probe(ctx, r, foldedProbeSQL(col.Table, col.Column), h.value)
	switch {
	case !ok:
		return untestable, s.probeFailure()
	case exists:
		return confirmed, ""
	}
	return absent, ""
}

// probeFailure names which of section 6 item 3's untestable cases stopped the
// last probe.
func (s *state) probeFailure() string {
	if s.probes >= s.probeCap() {
		return reasonProbeCap
	}
	return reasonProbeFailed
}

// probe runs one confirmation probe, counting it against the cap. It reports
// the answer and whether there was one at all.
func (s *state) probe(ctx context.Context, r pipeline.Reader, sql string, value any) (bool, bool) {
	if s.probes >= s.probeCap() {
		return false, false
	}
	s.probes++
	var exists bool
	rows, err := r.Query(ctx, sql, value)
	if err != nil {
		return false, false
	}
	defer rows.Close()
	if !rows.Next() {
		return false, false
	}
	if err := rows.Scan(&exists); err != nil {
		return false, false
	}
	if err := rows.Err(); err != nil {
		return false, false
	}
	return exists, true
}

// probeCap is --residual-probe-cap.
func (s *state) probeCap() int64 {
	switch {
	case s.opts.ProbeCap == 0:
		return DefaultProbeCap
	case s.opts.ProbeCap < 0:
		return 0
	}
	return int64(s.opts.ProbeCap)
}
