// SPDX-License-Identifier: Apache-2.0

package plan

import (
	"fmt"
	"strings"

	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/mask"
)

// The plan-time write-back check (T-0054 part 3).
//
// A masked column whose generator emits a value the column's type cannot hold
// is a run that dies in the middle. internal/transform discovers it per value:
// it masks, tries to parse the masker's text back into the Go kind the column
// arrived as, fails, and refuses at exit 7 — after extract has moved rows, and
// once per run for whichever column happens to come first. That is the shape of
// the bug T-0054 was opened for: on pagila the classifier decided `credential`
// on every `last_update timestamptz`, and "$lazyslice$invalid" is in none of
// the timestamp layouts, so every whole-pipeline run over pagila died mid-run.
//
// So the question is asked here instead, before a key is fetched and before a
// row moves, and transform's refusal stays as a backstop that a plan which
// passed this check cannot reach (internal/transform's
// TestTransformNeverRefusesWhatThePlanCheckAdmits).
//
// Why here and not only in internal/classify, which now gates a category by the
// column's type on its way in: because the category on a Decision at the end of
// classification is not always the one that gate saw. FK propagation, the
// same-column-name rule and a committed lazyslice.yml all move a category onto
// a column after the fact, and a raise can arrive from a column of another
// type. The classifier checks each of those against the rule pack today; this
// check is the one that runs over the answer rather than over each step to it,
// and it reads mask's own declaration rather than the rule pack, so the two
// have to agree for a plan to pass (mask/writable.go).

// checkWriteBack refuses a plan that would hand a masker a column it cannot
// write into. It runs over the tables still in scope — after --skip-table and
// the privilege pass, so a column in a table this run will never read is not a
// refusal — and in the order build() sorted them, so two runs over one snapshot
// refuse on the same column.
func (p *run) checkWriteBack() error {
	if p.cls == nil {
		return nil
	}
	for i := range p.tables {
		t := &p.tables[i]
		if !p.inScope[t.Ref] {
			continue
		}
		for _, col := range t.Columns {
			d, ok := p.cls.Decisions[ref.ColumnRef{Table: t.Ref, Column: col.Name}]
			if !ok || !d.Masked {
				continue
			}
			if d.Category == "" || d.Category == pipeline.CatNone {
				// A masked column with no category has no generator either, and
				// naming it here would be naming the wrong defect: transform
				// refuses it as "the decision names no masker", which is a
				// classification bug and not a type mismatch. §4's finalise
				// cannot produce one; nothing here should invent a second
				// spelling for it.
				continue
			}
			if base, ok := p.compositeType(col); ok {
				// A composite is the one type this check refuses on the type
				// alone (tracker T-0094, THREAT_MODEL.md T1). Everything else
				// it declines to judge is a type mask has no tag for and might
				// still load -- an ltree, a PostGIS geometry -- but a record
				// can hold no value any generator emits, and internal/classify
				// only reaches `possible` on one when a name or a field of a
				// sample said personal data. Before this, such a column was
				// copied verbatim under exit 0 with the personal data inside
				// it, or was masked as a scalar and died at load with the rows
				// already moving. The refusal is here, before a key is fetched,
				// and it prints the two escapes an operator has: drop the
				// table, or say in writing that the record is not personal.
				r := refuse(CodeUnwritable, exitPlan, t.Ref,
					fmt.Sprintf("%s.%s is masked as %s and its type %s is a composite, which no masker can write into",
						t.Ref, col.Name, d.Category, base),
					event.Args{
						event.ArgTable:  t.Ref.String(),
						event.ArgColumn: col.Name,
						event.ArgReason: "the type " + base + " is a composite and no masker can write a record: " +
							"--skip-table " + t.Ref.String() + ", or --unmask " + t.Ref.String() + "." + col.Name + "=REASON",
					})
				r.Column = col.Name
				return r
			}
			c, judged := p.constraintsOf(col)
			if !judged || mask.Writable(mask.Category(d.Category), d.Masker, c) {
				continue
			}
			r := refuse(CodeUnwritable, exitPlan, t.Ref,
				fmt.Sprintf("%s.%s is masked as %s and its type %s cannot hold what that category's masker emits",
					t.Ref, col.Name, d.Category, c.TypeTag),
				event.Args{
					event.ArgTable:  t.Ref.String(),
					event.ArgColumn: col.Name,
					event.ArgReason: "the category " + string(d.Category) + " does not fit the type " + c.TypeTag,
				})
			r.Column = col.Name
			return r
		}
	}
	return nil
}

// constraintsOf reduces one column to the constraints mask.Writable reads: the
// type tag, the length, and the labels that make a column writable whatever its
// type. It is deliberately the same reduction internal/transform makes
// (constraints.go) through the same two functions in mask, because a check that
// judged a different column than transform masks would be no check at all.
//
// What it leaves out is what Writable does not read: Unique, Rows and Distinct.
// The unique-index domain rule of ARCHITECTURE.md §5 — d_required = n²/2ε, and
// the refusal that names the largest --take a column can carry — is a separate
// plan-time check that has not landed; this one answers only whether there is a
// value to write and whether the type can hold it.
//
// The second return is false for a column this check does not judge. Two cases
// reach it, and both are deliberately not refusals:
//
//   - An enum, or any type mask has no family for — an extension type, a
//     domain whose base introspect could not render. Every generator answers a
//     labelled column with one of its labels, so an enum is writable under any
//     category; an unknown type is one nothing here knows enough about to
//     refuse, and refusing on ignorance would turn a working run into a plan
//     refusal. A **composite** used to be in this list and is not any more: it
//     is refused by type above (compositeType, T-0094), because it is the one
//     case where the ignorance is not real — a record can hold nothing any
//     generator emits.
//   - An array whose element type is unknown, for the same reason. An array is
//     masked element-wise under its element type (ARCHITECTURE.md §5), so the
//     element is what the tag is taken from.
func (p *run) constraintsOf(col pipeline.Column) (mask.Constraints, bool) {
	name := strings.TrimSpace(col.TypeName)
	for strings.HasSuffix(name, "[]") {
		name = strings.TrimSuffix(name, "[]")
	}
	if col.Domain != "" {
		if base, ok := p.domainBase(col.Domain); ok {
			name = base
			for strings.HasSuffix(name, "[]") {
				name = strings.TrimSuffix(name, "[]")
			}
		}
	}
	tag, known := mask.TypeTag(mask.BareTypeName(mask.StripTypmod(name)))
	if !known {
		return mask.Constraints{}, false
	}
	return mask.Constraints{
		TypeTag:  tag,
		MaxLen:   mask.MaxLen(tag, col.TypMod),
		Checks:   col.Checks,
		Nullable: col.Nullable,
	}, true
}

// compositeType reports the composite type a column is declared over, after an
// array suffix and a domain have been resolved, and whether it is one at all.
// Schema.Composites is the catalog's own list of typtype 'c' with relkind 'c'
// (internal/introspect/sql.go), so a view's row type is not in it.
//
// An array of a composite is a composite for this purpose: ARCHITECTURE.md §5
// masks an array element-wise under its element type, and the element is the
// record nothing can write.
func (p *run) compositeType(col pipeline.Column) (string, bool) {
	name := strings.TrimSpace(col.TypeName)
	for strings.HasSuffix(name, "[]") {
		name = strings.TrimSuffix(name, "[]")
	}
	if col.Domain != "" {
		if base, ok := p.domainBase(col.Domain); ok {
			name = base
			for strings.HasSuffix(name, "[]") {
				name = strings.TrimSuffix(name, "[]")
			}
		}
	}
	name = mask.UnquoteType(mask.StripTypmod(name))
	bare := mask.BareTypeName(name)
	for _, c := range p.schema.Composites {
		if mask.UnquoteType(c.Name) == name || mask.BareTypeName(c.Name) == bare {
			return c.Name, true
		}
	}
	return "", false
}

// domainBase returns the base type a domain is declared over, read out of the
// CREATE DOMAIN text introspect renders from the catalog's own deparser, so
// "AS <base type>" is always present and always the catalog's spelling.
func (p *run) domainBase(domain string) (string, bool) {
	want := mask.UnquoteType(domain)
	for _, d := range p.schema.Domains {
		if mask.UnquoteType(d.Name) != want && mask.BareTypeName(d.Name) != mask.BareTypeName(domain) {
			continue
		}
		const as = " AS "
		i := strings.Index(d.Def, as)
		if i < 0 {
			return "", false
		}
		rest := d.Def[i+len(as):]
		for _, clause := range []string{" COLLATE ", " DEFAULT ", " NOT NULL", " CONSTRAINT "} {
			if j := strings.Index(rest, clause); j >= 0 {
				rest = rest[:j]
			}
		}
		base := strings.TrimSpace(rest)
		return base, base != ""
	}
	return "", false
}
