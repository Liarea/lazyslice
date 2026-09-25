// SPDX-License-Identifier: Apache-2.0

package core

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/Liarea/lazyslice/internal/classify"
	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// DefaultMaskCategory is the category --mask TABLE.COL records when no
// =CATEGORY is given (T-0319). free_text because it is the category whose
// masker replaces the whole value, which is the right answer for a column the
// operator knows to be personal and cannot name more precisely; a column whose
// type free_text does not accept is refused by checkMasks, naming the flag's
// =CATEGORY form, rather than copied.
const DefaultMaskCategory = string(pipeline.CatFreeText)

// maskCategories is every category --mask may name: the v1 list
// (ARCHITECTURE.md section 4) less none, which would be a mask that masks
// nothing.
var maskCategories = []pipeline.Category{
	pipeline.CatPersonName, pipeline.CatEmail, pipeline.CatPhone, pipeline.CatAddress,
	pipeline.CatGeo, pipeline.CatPersonDate, pipeline.CatNationalID, pipeline.CatFinancial,
	pipeline.CatNetworkID, pipeline.CatOnlineID, pipeline.CatCredential, pipeline.CatFreeText,
	pipeline.CatSpecial, pipeline.CatBinary, pipeline.CatSemiStruct, pipeline.CatDerivedText,
}

// CheckMaskCategory refuses a --mask category that is not one of the v1
// categories. It is exported so that cmd/lazyslice can refuse a misspelling at
// the flag surface, before anything connects, the way ParseMemoryBudget is.
// The empty string is the bare form and is accepted.
func CheckMaskCategory(category string) error {
	if category == "" || slices.Contains(maskCategories, pipeline.Category(category)) {
		return nil
	}
	names := make([]string, 0, len(maskCategories))
	for _, c := range maskCategories {
		names = append(names, string(c))
	}
	return fmt.Errorf("%q is not a category; want one of %s", category, strings.Join(names, ", "))
}

// maskRequest is one column the run was asked to mask, and who asked: the
// --mask flag, or a `mask:` block in the committed yml.
type maskRequest struct {
	category pipeline.Category
	flag     bool
}

// priorInputs is what buildPrior folded into the classifier's prior: the
// --unmask opt-outs it resolved and every mask it applied.
type priorInputs struct {
	unmask map[ref.ColumnRef]string
	masks  map[ref.ColumnRef]maskRequest
}

// buildPrior is the committed file with this run's flags folded in: the
// --phone-region, the --mask requests and, when withUnmaskFlags, the --unmask
// opt-outs and, when withMaskFlags, the --mask requests. classifyPrior asks
// for all of it; classifierFingerprint asks for everything but the --unmask
// flags, which the reasons screen writes and the review pin therefore leaves
// out; maskBaseline asks for everything but the --mask flags, to see what the
// classifier would have done without them.
//
// A mask only ever tightens (ADR-004). Each one, from the flag or from a
// `mask:` block in the file, becomes a raise to certain under its category —
// the one route a caller's file already has to move a decision up
// (internal/classify's raiseFromConfig) — and drops any `unmask:` the file
// holds for the same column, because a column cannot be both and masking is
// the side to fail on. The --unmask flag is the one thing that beats a mask,
// and only the file's: it is this run's explicit, reasoned opt-out, where the
// file's mask is a record of an earlier run's. The --mask flag and the
// --unmask flag on one column is a contradiction and exit 2.
func (r *run) buildPrior(withUnmaskFlags, withMaskFlags bool) (*pipeline.Config, priorInputs, error) {
	in := priorInputs{
		unmask: map[ref.ColumnRef]string{},
		masks:  map[ref.ColumnRef]maskRequest{},
	}
	fileMasks := r.prior != nil && slices.ContainsFunc(
		slices.Collect(maps.Values(r.prior.Columns)),
		func(cc pipeline.ColumnConfig) bool { return cc.Mask != nil })
	unmaskFlags := withUnmaskFlags && len(r.req.Unmask) > 0
	maskFlags := withMaskFlags && len(r.req.Mask) > 0
	if !unmaskFlags && !maskFlags && r.req.PhoneRegion == "" && !fileMasks {
		return r.prior, in, nil
	}

	prior := &pipeline.Config{Columns: map[ref.ColumnRef]pipeline.ColumnConfig{}}
	if r.prior != nil {
		copied := *r.prior
		prior = &copied
		prior.Columns = make(map[ref.ColumnRef]pipeline.ColumnConfig, len(r.prior.Columns))
		for k, v := range r.prior.Columns {
			prior.Columns[k] = v
		}
	}
	if r.req.PhoneRegion != "" {
		prior.PhoneRegion = r.req.PhoneRegion
	}

	flagMasks := map[ref.ColumnRef]pipeline.Category{}
	var maskNames []string
	if withMaskFlags {
		maskNames = slices.Sorted(maps.Keys(r.req.Mask))
	}
	for _, name := range maskNames {
		category := r.req.Mask[name]
		if category == "" {
			category = DefaultMaskCategory
		}
		if err := CheckMaskCategory(category); err != nil {
			return nil, in, wrap(CodeUsage, exitUsage, err, "--mask %s", name)
		}
		col, err := resolveColumn(name, r.schema)
		if err != nil {
			return nil, in, wrap(CodeUsage, exitUsage, err, "--mask %s", name)
		}
		if previous, twice := flagMasks[col]; twice && previous != pipeline.Category(category) {
			return nil, in, wrap(CodeUsage, exitUsage, nil,
				"--mask names %s twice, as %s and %s; one column has one category", col, previous, category)
		}
		flagMasks[col] = pipeline.Category(category)
	}
	if withUnmaskFlags {
		for _, name := range slices.Sorted(maps.Keys(r.req.Unmask)) {
			col, err := resolveColumn(name, r.schema)
			if err != nil {
				return nil, in, wrap(CodeUsage, exitUsage, err, "--unmask %s", name)
			}
			if _, masked := flagMasks[col]; masked {
				return nil, in, wrap(CodeUsage, exitUsage, nil,
					"--mask and --unmask both name %s; a column is masked or opted out, not both", col)
			}
			in.unmask[col] = r.req.Unmask[name]
		}
	}

	for col, cc := range prior.Columns {
		if cc.Mask == nil {
			continue
		}
		if _, optedOut := in.unmask[col]; optedOut {
			// This run's --unmask beats the file's mask; the record is dropped
			// from the file this run writes, because internal/emit writes a
			// `mask:` block only beside a masked decision.
			cc.Mask = nil
			prior.Columns[col] = cc
			continue
		}
		category := cc.Mask.Category
		if category == "" {
			category = pipeline.Category(DefaultMaskCategory)
		}
		prior.Columns[col] = raisedForMask(cc, category)
		in.masks[col] = maskRequest{category: category}
	}
	for col, reason := range in.unmask {
		cc := prior.Columns[col]
		cc.Unmask = &pipeline.Unmask{Reason: reason, By: "flag"}
		prior.Columns[col] = cc
	}
	for col, category := range flagMasks {
		cc := raisedForMask(prior.Columns[col], category)
		cc.Mask = &pipeline.Mask{Category: category, By: "flag"}
		prior.Columns[col] = cc
		in.masks[col] = maskRequest{category: category, flag: true}
	}
	return prior, in, nil
}

// raisedForMask is one column's prior entry with a mask applied: the category,
// the top confidence, and no opt-out.
func raisedForMask(cc pipeline.ColumnConfig, category pipeline.Category) pipeline.ColumnConfig {
	cc.Category = category
	cc.Confidence = pipeline.ConfCertain
	cc.Unmask = nil
	return cc
}

// flagMasks is the --mask half of r.masks, which internal/emit records under
// each column's `mask:` block with `by: flag`.
func (r *run) flagMasks() map[ref.ColumnRef]pipeline.Category {
	out := map[ref.ColumnRef]pipeline.Category{}
	for col, m := range r.masks {
		if m.flag {
			out[col] = m.category
		}
	}
	return out
}

// checkMasks refuses a run in which a mask the operator asked for did not do
// what the request and its `mask:` record say (T-0319). Four cases, each exit
// 2, each named as its own Error event, all of them before the plan and before
// any write:
//
//   - The column came out unmasked. A mask is folded in as a raise, and
//     internal/classify declines a raise for a column it never masks (a
//     surrogate key, a copy of an unmasked key, a generated column) and for a
//     type the category does not accept (rules.yml's accepts: list, so
//     free_text on a jsonb column). A mask that silently did not apply looks
//     exactly like one that did.
//
//   - --mask named a column the classifier already masks under another
//     category. The flag is for a column the classifier left unmasked; a raise
//     would quietly swap the existing masker (credential for online_id, say),
//     and a mask only ever tightens (ADR-004).
//
//   - --mask's column came out masked under a category other than the one it
//     named (its type refused the named one and accepted the one it already
//     had), so the `mask:` record would contradict the decision beside it.
//
//   - A column that references a masked one across a foreign key is still
//     unmasked. Since T-0364 internal/classify runs FK propagation again after
//     the raise a mask becomes, so a masked natural key normally takes every
//     child with it under the same category and masker, and this never fires.
//     It stays as the fail-closed backstop for the children propagation
//     leaves behind: a child whose type does not accept the parent's category,
//     an edge whose parent column is not a key or unique. Such a child would carry the operator's own values in
//     clear (THREAT_MODEL.md T1) across a broken join (T8), so each is named
//     for a --mask of its own. A child with its own reasoned --unmask or
//     `unmask:` is left alone, the same opt-out that beats a file's mask, and
//     a generated child is not copied at all.
func (r *run) checkMasks(cls *pipeline.Classification) error {
	base, err := r.maskBaseline()
	if err != nil {
		return err
	}
	var first *Stop
	refuse := func(col ref.ColumnRef, from, reason string) {
		args := event.Args{
			event.ArgFlag:   from,
			event.ArgTable:  col.Table.String(),
			event.ArgColumn: col.Column,
			event.ArgReason: reason,
		}
		r.sink.Send(event.Event{
			At: time.Now(), Stage: event.Classify, Kind: event.Error, Code: CodeMaskNotApplied,
			Exit: exitUsage, Table: col.Table, Column: col.Column, Args: args,
		})
		if first == nil {
			first = &Stop{
				Code: CodeMaskNotApplied, Exit: exitUsage, Table: col.Table, Column: col.Column,
				Args:    args,
				Message: fmt.Sprintf("%s asks for %s to be masked, and it cannot be: %s", from, col, reason),
				sent:    true,
			}
		}
	}
	for _, col := range slices.SortedFunc(maps.Keys(r.masks), compareColumns) {
		m := r.masks[col]
		d, ok := cls.Decisions[col]
		if !ok {
			continue
		}
		from := "mask: in " + r.req.ConfigPath
		if m.flag {
			from = "--mask"
		}
		var was pipeline.Decision
		if base != nil {
			was = base.Decisions[col]
		}
		switch {
		case !d.Masked && d.NeverMasked:
			refuse(col, from, "it is a key, a copy of an unmasked key or a generated column, which are never masked")
			continue
		case !d.Masked:
			refuse(col, from, fmt.Sprintf(
				"its type does not accept the category %s; name one it does as --mask %s=CATEGORY",
				m.category, col))
			continue
		case m.flag && was.Masked && was.Category != m.category:
			refuse(col, from, fmt.Sprintf(
				"it is already masked as %s, and --mask never changes a column's masker; drop the flag, or name --mask %s=%s",
				was.Category, col, was.Category))
			continue
		case m.flag && d.Category != m.category:
			refuse(col, from, fmt.Sprintf(
				"it came out masked as %s, not %s; name --mask %s=%s",
				d.Category, m.category, col, d.Category))
			continue
		}
		for _, child := range r.unmaskedChildren(cls, col) {
			refuse(col, from, fmt.Sprintf(
				"%s references it and would be copied in clear; mask it too, as --mask %s=%s",
				child, child, d.Category))
		}
	}
	if first == nil {
		return nil
	}
	return first
}

// maskBaseline is this run's classification without its --mask flags, which
// checkMasks reads to tell a column the classifier left unmasked from one it
// already masked under another category. nil when no --mask flag was given:
// there is nothing to compare, and no second classification to pay for.
func (r *run) maskBaseline() (*pipeline.Classification, error) {
	if len(r.req.Mask) == 0 {
		return nil, nil
	}
	prior, _, err := r.buildPrior(true, false)
	if err != nil {
		return nil, err
	}
	cls, err := classify.New().Classify(r.schema, schemaSampler{schema: r.schema}, prior)
	if err != nil {
		return nil, wrap(CodeInternal, exitInternal, err, "the columns could not be classified")
	}
	return cls, nil
}

// unmaskedChildren is every column that references parent across a foreign
// key (any edge, a superset of what internal/classify's propagation walks,
// because this only ever asks for more masking) and that this run would still
// copy: not masked, not opted out by its own --unmask or `unmask:`, and not a
// generated column, which the target recomputes rather than receives. Sorted,
// each once. After T-0364's second propagation sweep it is empty for every
// child propagation can mask; what is left is what it cannot.
func (r *run) unmaskedChildren(cls *pipeline.Classification, parent ref.ColumnRef) []ref.ColumnRef {
	seen := map[ref.ColumnRef]bool{}
	for _, fk := range r.schema.FKs {
		if fk.Parent != parent.Table {
			continue
		}
		for i, name := range fk.ParentCols {
			if name != parent.Column || i >= len(fk.ChildCols) {
				continue
			}
			child := ref.ColumnRef{Table: fk.Child, Column: fk.ChildCols[i]}
			if child == parent || generatedColumn(r.schema, child) {
				continue
			}
			d, ok := cls.Decisions[child]
			if !ok || d.Masked || d.Source == pipeline.ByFlagUnmask || d.Source == pipeline.ByYmlUnmask {
				continue
			}
			seen[child] = true
		}
	}
	return slices.SortedFunc(maps.Keys(seen), compareColumns)
}

// generatedColumn reports whether col is a generated column of schema.
func generatedColumn(schema *pipeline.Schema, col ref.ColumnRef) bool {
	for _, t := range schema.Tables {
		if t.Ref != col.Table {
			continue
		}
		for _, c := range t.Columns {
			if c.Name == col.Column {
				return c.Generated != ""
			}
		}
	}
	return false
}

// compareColumns orders column references for slices.SortedFunc.
func compareColumns(a, b ref.ColumnRef) int {
	switch {
	case a.Less(b):
		return -1
	case b.Less(a):
		return 1
	}
	return 0
}
