// SPDX-License-Identifier: Apache-2.0

package verify

import (
	"context"
	"sort"
	"strings"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/mask"
)

// ADR-015 (proposed, 2026-09-22): a residual hit inside the name masker's own
// vocabulary is explained by count and row identity, not confirmed by a column
// probe.
//
// Section 6 item 3's column probe asks whether the source *column* holds a hit
// value anywhere. For a masker that draws real words from a list, that is true
// of a correct run: a masked "Mary" is some other customer's real "Mary". So
// for a column whose masker has a vocabulary (mask.Emitting — today only
// person_name), a hit whose value that vocabulary contains (mask.Emits) is
// tallied instead of probed, and at the end of the column:
//
//  1. every tallied value the target holds more often than internal/transform
//     emitted it (Residual.Emitted) goes back through the column probe, on one
//     retained target value, and a confirmed probe is exit 9 with
//     reasonOverCount: a copy the masker did not make came from somewhere;
//  2. where the table has a usable row identity, the rows holding a tallied
//     value are looked up in the source by that identity, in batches of at
//     most rowCheckBatch sent as each batch fills during the scan (so the
//     stage holds one batch of identities per column, never the column's),
//     and any row whose source value is canonical-equal or fold-equal to its
//     own target value is exit 9 with reasonSameRow. mask.maskCell redraws
//     the masker until its output never reads as its own input, so only a
//     bypass of the masker puts it there;
//  3. what survives both is reported once, as a count
//     (CodeResidualExplained), never as a value.
//
// Every other hit keeps the column probe at once: any hit in any other column,
// and in this one a hit mask.Emits rejects (a name off the list, a full name
// in a given-name column), a hit in a closed column, a hit from a custom,
// unknown or fixed: masker, and every JSON leaf.
//
// A usable row identity is one the plan knows to be unique: a primary key or a
// unique rung (pipeline.IdentityPK, IdentityUnique — an explicit --key the
// plan probed over the whole table is the latter), or a lookup step's primary
// key. A pseudo-key, or an explicit key probed only on a sample, is
// IdentityPseudo and is no usable identity: two rows sharing it would compare
// one row's source value against another row's target value, which passes a
// same-row leak or refuses a correct run. Such a table is explained by the
// count check alone, as one without a key is.
//
// A row-check statement does not spend --residual-probe-cap. The cap bounds
// statements that bind a candidate value (which lands in the source's log) and
// that may be a sequential scan; a row check binds only identifiers the
// target already holds and joins on a unique, indexed identity, and there is
// one statement per rowCheckBatch rows the column holds a tallied value in,
// which the target's row count already bounds. Counting it against the cap
// would refuse a large correct run (500,000 people with two name columns is
// 1,000 statements).
//
// A row check that cannot run is never read as a coincidence: a statement
// error or a source that will not open is exit 9 as an unconfirmable hit is.
// A row the source no longer holds, and an identity with a NULL part, learn
// nothing — the count check already stands for them — and never fall back to
// the column probe, because that fallback is what would refuse a correct run.

// rowCheckBatch is the most identity tuples one row-check statement carries.
const rowCheckBatch = 1000

// explainer is one masked column's ADR-015 state. A nil-safe zero value is a
// column nothing is explained in: tally answers false and finish does nothing.
type explainer struct {
	s    *state
	step pipeline.Step
	col  ref.ColumnRef
	cat  pipeline.Category
	id   mask.ID
	cons mask.Constraints

	// on is true for a column whose masker has a vocabulary.
	on bool
	// idCols and casts are the row identity the row check joins on, empty
	// when the column is an array or the table has no usable identity.
	idCols []string
	casts  []string
	types  []pipeline.Column

	// byValue is the tally, keyed by the canonical form (string(hit.canon)).
	// It holds masked target values the vocabulary contains, which is what a
	// person_name masker emits, for as long as one column's scan lasts. It is
	// bounded by the vocabulary, not by the column's row count.
	byValue map[string]*tallied
	// pending is the rows holding a tallied value that the row check has not
	// yet sent: at most rowCheckBatch of them, flushed as the batch fills.
	pending []heldRow
}

// tallied is one explained candidate: T_v, and one target value to probe with
// if the count check fails.
type tallied struct {
	count int64
	value any
}

// heldRow is one target row holding a tallied value: its identity tuple and
// the value as that row holds it.
type heldRow struct {
	ids   []any
	value any
}

// newExplainer decides, once per column, whether ADR-015 applies to it and
// with which row identity.
func (s *state) newExplainer(
	step pipeline.Step, col ref.ColumnRef, column pipeline.Column, d pipeline.Decision,
	family string, array bool,
) *explainer {
	ex := &explainer{s: s, step: step, col: col, cat: d.Category, id: d.Masker}
	if document(family) {
		// A document's leaves are masked under their own category and are
		// never explained.
		return ex
	}
	cons, ok := emitConstraints(column, d, family)
	if !ok {
		return ex
	}
	ex.on, ex.cons = true, cons
	ex.byValue = map[string]*tallied{}
	if array {
		// Scalar columns only: an array element has no row of its own to
		// check, and its count check stands alone.
		return ex
	}
	if idCols, casts, types, ok := s.rowIdentity(step); ok {
		ex.idCols, ex.casts, ex.types = idCols, casts, types
	}
	return ex
}

// emitConstraints is the part of mask.Constraints a vocabulary answer reads —
// the family, the declared length, the CHECK expressions and the role — and
// whether the column's masker has a vocabulary under them. Only a character
// column can hold a person name; every other family is left to the column
// probe, and so is an enum (famOther here), whose labels are not a vocabulary.
//
// It need not be internal/transform's own Constraints to the byte. A
// difference can only make this answer true where transform's was false, and
// then transform counted nothing for the column, so every hit fails the count
// check and takes the column probe: the disagreement fails closed.
func emitConstraints(column pipeline.Column, d pipeline.Decision, family string) (mask.Constraints, bool) {
	switch family {
	case famText, famVarchar, famBpchar, famCitext:
	default:
		return mask.Constraints{}, false
	}
	c := mask.Constraints{
		TypeTag: family,
		MaxLen:  mask.MaxLen(family, column.TypMod),
		Checks:  column.Checks,
		Role:    d.Role,
	}
	return c, mask.Emitting(d.Masker, c)
}

// rowIdentity is the identity ADR-015's row check joins on: the step's
// identity columns when the plan knows them unique (uniqueIdentity), or its
// table's primary key for a lookup step, each unmasked, not generated, and
// joinable (joinCasts). It returns the columns as well so the chunk can be
// encoded by type.
func (s *state) rowIdentity(step pipeline.Step) ([]string, []string, []pipeline.Column, bool) {
	table := s.tables[step.Table]
	if table == nil {
		return nil, nil, nil, false
	}
	var idCols []string
	switch {
	case step.Mode == pipeline.Lookup:
		idCols = table.PK
	case uniqueIdentity(step.Identity):
		idCols = step.Identity.Columns
	}
	if len(idCols) == 0 {
		return nil, nil, nil, false
	}
	if s.identityMasked(pipeline.Step{Table: step.Table, Identity: pipeline.Identity{Columns: idCols}}) {
		return nil, nil, nil, false
	}
	types := make([]pipeline.Column, len(idCols))
	for i, name := range idCols {
		c, ok := columnOf(table, name)
		if !ok || c.Generated != "" {
			return nil, nil, nil, false
		}
		types[i] = c
	}
	casts, ok := s.joinCasts(table, idCols)
	if !ok {
		return nil, nil, nil, false
	}
	return idCols, casts, types, true
}

// uniqueIdentity reports whether a step's identity is one the plan knows to
// be unique over the whole table — a primary key or a unique rung — and so one
// the row check can match a target row to its own source row by. A pseudo-key
// was only ever probed on a sample (ARCHITECTURE.md section 3.4).
func uniqueIdentity(id pipeline.Identity) bool {
	return len(id.Columns) > 0 && (id.Kind == pipeline.IdentityPK || id.Kind == pipeline.IdentityUnique)
}

// identified reports whether the column is scanned with its row identity.
func (ex *explainer) identified() bool { return ex.on && len(ex.idCols) > 0 }

// tally takes a hit the vocabulary contains and reports true; any other hit is
// the caller's to probe. ids is the row's identity tuple, nil when the column
// is scanned without one. A row it keeps for the row check waits in pending;
// the caller sends a full batch with flushFull.
func (ex *explainer) tally(h hit, ids []any) bool {
	if !ex.on || h.leaf || len(h.canon) == 0 {
		return false
	}
	if !mask.Emits(ex.id, valueOf(h.value), ex.cons) {
		return false
	}
	k := string(h.canon)
	t := ex.byValue[k]
	if t == nil {
		t = &tallied{value: h.value}
		ex.byValue[k] = t
	}
	t.count++
	if ex.identified() && ids != nil && !hasNull(ids) {
		ex.pending = append(ex.pending, heldRow{ids: append([]any(nil), ids...), value: h.value})
	}
	return true
}

// flushFull sends the pending rows once they make a whole batch, and reports
// whether the column failed. Called after every tally during the scan, it is
// what keeps the row check to one batch of identities in memory.
func (ex *explainer) flushFull(ctx context.Context) bool {
	if len(ex.pending) < rowCheckBatch {
		return false
	}
	return ex.flush(ctx)
}

// flush sends whatever rows are pending and empties the batch.
func (ex *explainer) flush(ctx context.Context) bool {
	if len(ex.pending) == 0 {
		return false
	}
	batch := ex.pending
	ex.pending = ex.pending[:0]
	return ex.rowCheck(ctx, batch)
}

func hasNull(ids []any) bool {
	for _, v := range ids {
		if v == nil {
			return true
		}
	}
	return false
}

// finish is steps 1 to 3 above, at the end of the column. It reports whether
// the column failed, which stops the scan the way a confirmed hit does.
func (ex *explainer) finish(ctx context.Context) (bool, error) {
	if !ex.on || len(ex.byValue) == 0 {
		return false, nil
	}
	keys := make([]string, 0, len(ex.byValue))
	for k := range ex.byValue {
		keys = append(keys, k)
	}
	// Sorted, so two runs over one target probe the same value first and a
	// report names the same failure.
	sort.Strings(keys)

	var explained int64
	for _, k := range keys {
		t := ex.byValue[k]
		if t.count > ex.s.res.Emitted(ex.col, "", []byte(k)) {
			done, err := ex.s.handleAs(ctx, ex.col, hit{value: t.value, canon: []byte(k)}, reasonOverCount)
			if err != nil || done {
				return done, err
			}
			continue
		}
		explained += t.count
	}
	// The last, partial batch. A row holding a value the count check sent to
	// the column probe is checked too: a same-row match is a leak whichever
	// way its value is judged.
	if ex.flush(ctx) {
		return true, nil
	}
	if explained > 0 {
		ex.s.report(checkResidual, CodeResidualExplained, ex.col.Table, ex.col.Column, explained)
	}
	return false, nil
}

// rowCheck is step 2 for one batch of at most rowCheckBatch rows: looked up
// in the source by identity in one statement. It reports whether the column
// failed.
func (ex *explainer) rowCheck(ctx context.Context, batch []heldRow) bool {
	s := ex.s
	r, err := s.shortReader(ctx)
	if err != nil {
		ex.unconfirmable(reasonSourceClosed, len(batch))
		return true
	}
	cols := append(append([]string{}, ex.idCols...), ex.col.Column)
	n := len(ex.idCols)
	chunk, byKey := ex.chunkOf(batch)
	if chunk.Len() == 0 {
		// No tuple of this batch could be encoded in its column's type:
		// nothing to ask, and nothing learned.
		return false
	}
	source, err := rowValues(ctx, r, rowCheckSQL(ex.step.Table, cols, ex.idCols, ex.casts, chunk),
		len(cols), chunkArgs(chunk, n)...)
	if err != nil {
		ex.unconfirmable(reasonProbeFailed, len(batch))
		return true
	}
	for _, row := range source {
		// Every target row with this identity is compared: the identity is
		// unique in the source, so each of them was copied from this row.
		for _, target := range byKey[identityKey(row[:n])] {
			if ex.sameValue(row[n], target) {
				s.fail(&Refusal{
					Code: CodeRefusedResidual, Exit: exitResidual, Check: checkResidual,
					Table: ex.col.Table, Column: ex.col.Column, Count: 1,
					Reason: reasonSameRow,
				})
				return true
			}
		}
	}
	return false
}

// unconfirmable is a row check that could not run: exit 9 as an unconfirmable
// hit is, never read as a coincidence.
func (ex *explainer) unconfirmable(reason string, rows int) {
	ex.s.fail(&Refusal{
		Code: CodeRefusedUnconfirmable, Exit: exitResidual, Check: checkUnconfirmable,
		Table: ex.col.Table, Column: ex.col.Column, Count: int64(rows), Reason: reason,
	})
}

// sameValue is the row check's question: does the source row still hold this
// row's masked value, under the category's canonical form or over its letters
// and digits (mask.FoldEqual, the comparison mask.maskCell's redraw fires on)?
// NULL on either side is not a match.
func (ex *explainer) sameValue(source, target any) bool {
	if source == nil || target == nil {
		return false
	}
	a, okA, errA := canonicalOf(mask.Category(ex.cat), source)
	b, okB, errB := canonicalOf(mask.Category(ex.cat), target)
	if errA == nil && errB == nil && okA && okB && string(a) == string(b) {
		return true
	}
	return mask.FoldEqual(textOf(source), textOf(target))
}

// chunkOf encodes one batch of identity tuples as the typed arrays the chunk
// join binds, in the encoding internal/plan's key sets use (int8[], text[],
// uuid[], and text[] cast back for anything else), and maps each tuple's
// identityKey to the value its target row holds. A tuple a value of which
// cannot be encoded in its column's type is left out: it learns nothing. A
// key maps to every target value that holds it, never the last one only, so a
// duplicated identity cannot hide one row's value behind another's.
func (ex *explainer) chunkOf(batch []heldRow) (tupleChunk, map[string][]any) {
	n := len(ex.idCols)
	ch := tupleChunk{casts: make([]string, n), cols: make([]any, n)}
	for i, c := range ex.types {
		ch.casts[i] = arrayCastOf(c)
	}
	ints := make([][]int64, n)
	texts := make([][]string, n)
	uuids := make([][][16]byte, n)
	byKey := make(map[string][]any, len(batch))
	for _, row := range batch {
		enc := make([]any, n)
		ok := true
		for i, c := range ex.types {
			v, good := encodeKey(c, row.ids[i])
			if !good {
				ok = false
				break
			}
			enc[i] = v
		}
		if !ok {
			continue
		}
		k := identityKey(row.ids)
		if _, seen := byKey[k]; !seen {
			// A tuple the join is sent once; the second copy of a duplicated
			// identity would only return its source row twice.
			for i := range enc {
				switch v := enc[i].(type) {
				case int64:
					ints[i] = append(ints[i], v)
				case string:
					texts[i] = append(texts[i], v)
				case [16]byte:
					uuids[i] = append(uuids[i], v)
				}
			}
			ch.n++
		}
		byKey[k] = append(byKey[k], row.value)
	}
	for i, c := range ex.types {
		switch arrayCastOf(c) {
		case "::int8[]":
			ch.cols[i] = ints[i]
		case "::uuid[]":
			ch.cols[i] = uuids[i]
		default:
			ch.cols[i] = texts[i]
		}
	}
	return ch, byKey
}

// tupleChunk is a pipeline.Chunk over identity tuples read from the target,
// so that sample.go's unnestFrom, joinOn and chunkArgs build the row check
// exactly as they build the sample read.
type tupleChunk struct {
	casts []string
	cols  []any
	n     int
}

func (c tupleChunk) Len() int          { return c.n }
func (c tupleChunk) Column(i int) any  { return c.cols[i] }
func (c tupleChunk) Cast(i int) string { return c.casts[i] }

var _ pipeline.Chunk = tupleChunk{}

// arrayCastOf is the cast one identity column's array parameter carries,
// internal/plan's keyset.go encoding: an integer as int8, a text-like key as
// text (bpchar too, trimmed, because its comparison against text strips the
// padding), a uuid as itself, and anything else as text cast back in the join
// (joinCast).
func arrayCastOf(c pipeline.Column) string {
	switch c.TypeOID {
	case oidInt2, oidInt4, oidInt8:
		return "::int8[]"
	case oidUUID:
		return "::uuid[]"
	}
	return "::text[]"
}

// encodeKey is one identity value in its column's array encoding, and whether
// it could be encoded at all.
func encodeKey(c pipeline.Column, v any) (any, bool) {
	switch arrayCastOf(c) {
	case "::int8[]":
		switch t := v.(type) {
		case int64:
			return t, true
		case int32:
			return int64(t), true
		case int16:
			return int64(t), true
		case int:
			return int64(t), true
		case int8:
			return int64(t), true
		}
		return nil, false
	case "::uuid[]":
		switch t := v.(type) {
		case [16]byte:
			return t, true
		case string:
			return parseUUID(t)
		}
		return nil, false
	}
	s := textOf(v)
	if c.TypeOID == oidBpchar {
		s = strings.TrimRight(s, " ")
	}
	return s, true
}

// parseUUID reads a uuid's text form into the 16 bytes pgx encodes as one.
func parseUUID(s string) (any, bool) {
	clean := strings.ReplaceAll(s, "-", "")
	if len(clean) != 32 {
		return nil, false
	}
	var out [16]byte
	for i := range out {
		hi, ok1 := hexNibble(clean[2*i])
		lo, ok2 := hexNibble(clean[2*i+1])
		if !ok1 || !ok2 {
			return nil, false
		}
		out[i] = hi<<4 | lo
	}
	return out, true
}

func hexNibble(c byte) (byte, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}
