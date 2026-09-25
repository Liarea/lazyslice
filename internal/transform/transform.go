// SPDX-License-Identifier: Apache-2.0

// Package transform applies the classification to every batch on its way from
// extract to load, and records what it masked in the residual filter.
//
// Transform is pure apart from Residual.Add and Residual.AddEmitted: same key,
// same classification, same input, same output, which is what invariant I3
// (two runs, byte-identical targets) rests on. Nothing here reads a clock, a random source or an
// environment; the only variation a generator gets is h, and h comes from the
// run key, the category and the canonical value (ARCHITECTURE.md §5).
//
// It masks in place. There is no pass-through mode and no flag that turns
// masking off wholesale; CONCEPT.md refuses one and CLAUDE.md forbids adding
// one. A value the masker refuses is a refusal and never a copy: shipping
// cleartext under a column the report calls masked is THREAT_MODEL.md T12.
package transform

import (
	"errors"
	"fmt"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/mask"
)

type transformer struct {
	schema *pipeline.Schema
	tables map[ref.TableRef]*pipeline.Table
}

// New returns the masking transformer over the schema introspect read.
//
// ARCHITECTURE.md §2's Transformer.Transform takes the classification and not
// the schema, and a Decision carries a category and a masker id but not the
// column's type, length, enum labels or CHECK constraints — which are exactly
// what mask.Constraints is and exactly what keeps a masked value inside the
// shape the application checks (§5). Verifier.Verify is given the schema for
// the same reason ("schema supplies the column types the second net
// canonicalises by"), so it arrives here through the constructor rather than
// through an interface §2 fixes.
func New(schema *pipeline.Schema) pipeline.Transformer {
	t := transformer{schema: schema}
	if schema != nil {
		t.tables = make(map[ref.TableRef]*pipeline.Table, len(schema.Tables))
		for i := range schema.Tables {
			t.tables[schema.Tables[i].Ref] = &schema.Tables[i]
		}
	}
	return t
}

var _ pipeline.Transformer = transformer{}

// colPlan is what one column of one batch needs, worked out once per batch
// rather than once per row.
type colPlan struct {
	col   ref.ColumnRef
	mask  bool
	cat   pipeline.Category
	id    mask.ID
	shape columnShape
	// document is true for a json, jsonb or hstore column, which §4 masks whole
	// rather than as a scalar.
	document bool
	// emits is true for a column whose masker has a vocabulary under this
	// column's constraints (mask.Emitting, ADR-015): every masked scalar and
	// array element of it is counted by its output (Residual.AddEmitted), so
	// that verify can tell a masked name that equals some other row's real
	// name from one the masker never produced. Never for a document: a JSON
	// leaf is masked under its own category and is never explained.
	emits bool
	// leaves is what a document column's leaves are decided under: the
	// decision's per-leaf map through pipeline.Decision.LeafMap (T-0272), nil
	// for any column whose own decision is not the classifier's plain
	// semi_structured one, which masks every leaf, and the run's phone region.
	// Read by json.go's leafRule.
	leaves leafPolicy
}

// Transform masks the batch in place and returns it.
//
// A column with no decision, or a decision the classifier did not mark masked,
// is copied through: the threshold is the classifier's (§4 "possible and above
// is masked"), and re-deciding it here would be a second, quieter classifier.
func (t transformer) Transform(
	b pipeline.RowBatch,
	cls *pipeline.Classification,
	key *mask.Key,
	res pipeline.Residual,
) (pipeline.RowBatch, error) {
	if t.schema == nil {
		return b, errors.New("transform: no schema")
	}
	if cls == nil {
		return b, errors.New("transform: no classification: a batch with no decisions would be a batch copied through unmasked")
	}
	if key == nil {
		return b, errors.New("transform: no key")
	}
	if res == nil {
		// Masking without recording defeats the only control THREAT_MODEL.md
		// T12 has, so it is a refusal and not a warning.
		return b, errors.New("transform: no residual filter")
	}
	if len(b.Rows) == 0 {
		return b, nil
	}
	table, ok := t.tables[b.Table]
	if !ok {
		return b, fmt.Errorf("transform: %s is in the batch and not in the schema", b.Table)
	}

	plans, err := t.plan(table, b.Cols, cls)
	if err != nil {
		return b, err
	}
	for _, row := range b.Rows {
		if len(row) != len(plans) {
			return b, fmt.Errorf("transform: a row of %s has %d values for %d columns", b.Table, len(row), len(plans))
		}
		for i := range plans {
			if !plans[i].mask || row[i] == nil {
				continue
			}
			out, err := t.cell(plans[i], row[i], *key, res)
			if err != nil {
				return b, err
			}
			row[i] = out
		}
	}
	return b, nil
}

// plan works out, once per batch, what happens to each column.
func (t transformer) plan(
	table *pipeline.Table,
	cols []string,
	cls *pipeline.Classification,
) ([]colPlan, error) {
	plans := make([]colPlan, len(cols))
	for i, name := range cols {
		col := ref.ColumnRef{Table: table.Ref, Column: name}
		plans[i].col = col

		column, ok := columnOf(table, name)
		if !ok {
			return nil, fmt.Errorf("transform: %s has no column %q", table.Ref, name)
		}
		plans[i].shape = t.shapeOf(table, column)
		plans[i].document = isDocument(plans[i].shape.family)

		d, ok := cls.Decisions[col]
		if !ok || !d.Masked {
			continue
		}
		plans[i].mask = true
		plans[i].cat = d.Category
		plans[i].id = d.Masker
		plans[i].leaves = leafPolicy{keys: d.LeafMap(), region: cls.PhoneRegion}
		if d.UniqueIndex {
			plans[i].shape.constraints.Unique = true
		}
		// Role reaches the masker the way Unique above does: a Decision field
		// mask.Constraints has its own field for (T-0287). d.Role is mask.RoleFull
		// for every category but person_name, which is Constraints.Role's own
		// zero value, so this is unconditional rather than gated the way Unique
		// is above.
		plans[i].shape.constraints.Role = d.Role
		// Decided once per column and per batch, after every constraint the
		// masker reads is in place: a closed column (an enum, a CHECK list)
		// has no vocabulary, whatever its masker.
		plans[i].emits = !plans[i].document && mask.Emitting(plans[i].id, plans[i].shape.constraints)
		if plans[i].id == "" {
			return nil, &Refusal{
				Code: CodeMasker, Exit: exitTransform, Col: col,
				Reason: errors.New("the decision names no masker"),
			}
		}
	}
	return plans, nil
}

// cell masks one value.
func (t transformer) cell(p colPlan, v any, key mask.Key, res pipeline.Residual) (any, error) {
	if p.document {
		return t.maskDocument(p.col, p.shape, p.leaves, v, key, res)
	}
	if elems, ok := v.([]any); ok && p.shape.array {
		return t.maskArray(p, elems, key, res)
	}
	if p.shape.array {
		switch text := v.(type) {
		case string:
			return t.maskArrayLiteral(p, text, key, res)
		case []byte:
			out, err := t.maskArrayLiteral(p, string(text), key, res)
			if err != nil {
				return nil, err
			}
			return []byte(out), nil
		}
	}
	return t.maskScalar(p, v, key, res)
}

// maskArray masks element-wise (§5 "Arrays mask element-wise"): each non-NULL
// element goes through the element masker, length is preserved, a NULL element
// stays NULL and an empty array stays empty. h is computed per element, so
// equal elements mask alike across rows.
//
// Every element enters the residual filter under the column's empty path, keyed
// by its own canonical bytes, so a surviving element is found whichever
// position it is in.
func (t transformer) maskArray(p colPlan, elems []any, key mask.Key, res pipeline.Residual) (any, error) {
	out := make([]any, len(elems))
	for i, e := range elems {
		if e == nil {
			continue
		}
		masked, err := t.maskScalar(p, e, key, res)
		if err != nil {
			return nil, err
		}
		out[i] = masked
	}
	return out, nil
}

// maskArrayLiteral masks an array that arrived as the server's own text form
// rather than as a slice, which is what pgx hands back for an array whose
// element type its map does not know — a citext[], and every other array of an
// extension's base type (tracker T-0118, array.go).
//
// It is maskArray's rule over a different carrier and not a second one: the
// literal is parsed, every non-NULL element goes through the same element
// masker with h computed per element, a NULL element stays NULL, an empty array
// stays empty, the dimensions are unchanged, and the result is written back as
// a literal array_in accepts. A literal this package cannot parse is a refusal
// and never a copy — masking it as one string is what handed CopyFrom a scalar
// for an _citext column, and copying it through is the cleartext
// THREAT_MODEL.md T12 is about.
func (t transformer) maskArrayLiteral(p colPlan, text string, key mask.Key, res pipeline.Residual) (string, error) {
	prefix, root, err := parseArrayLiteral(text)
	if err != nil {
		return "", &Refusal{
			Code: CodeMasker, Exit: exitTransform, Col: p.col,
			Masker: string(p.id), Reason: err,
		}
	}
	if err := t.maskArrayNode(p, &root, key, res); err != nil {
		return "", err
	}
	return renderArrayLiteral(prefix, root), nil
}

// maskArrayNode masks one node of a parsed literal, recursing into a
// multidimensional array's inner arrays.
func (t transformer) maskArrayNode(p colPlan, n *arrayNode, key mask.Key, res pipeline.Residual) error {
	if n.nested {
		for i := range n.elems {
			if err := t.maskArrayNode(p, &n.elems[i], key, res); err != nil {
				return err
			}
		}
		return nil
	}
	if n.null {
		return nil
	}
	out, err := t.maskScalar(p, n.text, key, res)
	if err != nil {
		return err
	}
	if out == nil {
		// A generator that answers NULL for an element writes the NULL marker,
		// not the four-letter string.
		n.null, n.text = true, ""
		return nil
	}
	n.text = textOf(out)
	return nil
}

// maskScalar is one value through mask.Apply: the NULL and empty rules,
// canonicalisation, the key schedule and the generator, in that order, and then
// the canonical source bytes into the filter.
func (t transformer) maskScalar(p colPlan, v any, key mask.Key, res pipeline.Residual) (any, error) {
	in := valueOf(v)
	r, err := mask.Apply(key, mask.Category(p.cat), p.id, in, p.shape.constraints)
	if err != nil {
		return nil, &Refusal{
			Code: CodeMasker, Exit: exitTransform, Col: p.col,
			Masker: string(p.id), Reason: err,
		}
	}
	if r.Masked {
		// Every masked cell adds its canonical source bytes to the filter. A
		// path here through mask.Apply that skipped this would be a masked cell
		// verify can never test (§6 item 1).
		res.Add(p.col, "", r.Canonical)
		if p.emits {
			t.addEmitted(p, r.Out, res)
		}
	}
	out, err := coerce(r.Out, v)
	if err != nil {
		return nil, &Refusal{
			Code: CodeMasker, Exit: exitTransform, Col: p.col,
			Masker: string(p.id), Reason: err,
		}
	}
	return out, nil
}

// addEmitted counts one masked cell of an emitting column by its output, in the
// canonical form verify reads the target's value back into (ADR-015). The
// canonical form is the category's own — for person_name the fold — so "Mary"
// in the target is counted against the "mary" transform recorded here.
//
// An output with no canonical bytes is not counted: verify tests nothing for a
// value it cannot canonicalise either (internal/verify/value.go).
func (t transformer) addEmitted(p colPlan, out mask.Value, res pipeline.Residual) {
	if out.Null || out.Empty() {
		return
	}
	canon, _, err := mask.Canonical(mask.Category(p.cat), out, mask.Constraints{})
	if err != nil {
		return
	}
	b := []byte(canon.Text)
	if len(canon.Bytes) > 0 {
		b = canon.Bytes
	}
	if len(b) == 0 {
		return
	}
	res.AddEmitted(p.col, "", b)
}

// isDocument reports the families §4 masks whole rather than as a scalar.
func isDocument(family string) bool {
	switch family {
	case famJSON, famJSONB, famHstore:
		return true
	}
	return false
}

func columnOf(t *pipeline.Table, name string) (pipeline.Column, bool) {
	for _, c := range t.Columns {
		if c.Name == name {
			return c, true
		}
	}
	return pipeline.Column{}, false
}
