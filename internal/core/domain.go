// SPDX-License-Identifier: Apache-2.0

package core

import (
	"fmt"
	"strings"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/mask"
)

// The small-domain rule (ARCHITECTURE.md section 5, last bullet but one).
//
// "Every masked column's admissible domain is d = min(column domain, generator
// Domain()) ... when d < 2 × distinct(samples) the masking is a stable
// substitution over a small alphabet, recoverable by frequency ... and the tool
// says so: the column is listed under small_domain: in the yml, in the plan,
// and under what the green tick does not prove."
//
// pipeline.Decision carries Domain and SmallDomain for it and **nothing in the
// tree fills them**: internal/classify decides categories and internal/plan
// decides rows, and the domain check fell between the two. That is not a
// cosmetic gap. Two things downstream read it and both are wrong without it:
// internal/emit writes `small_domain:` from it, and
// internal/invariants/i2_masking_test.go exempts exactly the columns listed
// there from its "the target's values and the source's must not overlap" check
// — which a column with six admissible values cannot satisfy, whatever the
// masker does. testdata/README.md trap 24 (public.marital_status) is the
// fixture that says so.
//
// So it is computed here, in the wiring that holds both the schema and the
// classification, and it is deliberately the *column* half of d only: the
// generator's own Domain() needs mask.Constraints, which internal/transform
// builds and does not export. Taking the column domain alone can only make d
// larger, so a column called small here is small under the full rule too, and
// the columns this misses are the ones with a wide type and a narrow generator.
// Its proper home is internal/classify (which has the samples) or internal/plan
// (which has the row count the unique-index half of the same rule needs);
// internal/core/CLAUDE.md records the deviation.

// printableDomain is the domain of a one-character text column: the printable
// ASCII range, which is what internal/invariants counts with power(95, n).
const printableDomain = 95

// smallDomainChars bounds the exponent for a character column. Above two
// characters the count exceeds anything worth calling small, and 95^n overflows
// nothing this way.
const smallDomainChars = 2

// smallDomainCeiling is the absolute bound on a "small" domain, beside section
// 5's ratio.
//
// The ratio alone (d < 2 × distinct(samples)) has no upper bound on d, and the
// flag is not cosmetic: a column marked small-domain is left out of the residual
// filter (run.go's smallDomainAware) and out of invariant I2's value-overlap
// check, so nothing downstream verifies that its masker ran. An enum with three
// hundred labels, two hundred of which appear in a two-hundred-row sample,
// satisfies the ratio — and three hundred labels of clinic or city names is
// personal data that would then be checked by nothing.
//
// 128 is the bound because it covers every domain the ratio can reach for a
// reason: a boolean (2), an enum with a human-written label set, and a
// character(1) (95 printable values). Above it the column is not "a small
// alphabet recoverable by frequency" in section 5's sense, and it stays in the
// filter.
//
// The cost of the bound is stated rather than hidden: for a masked enum, section
// 5 requires the masked value to be a valid label, so every masked value equals
// some real source value and the residual filter hits on all of them. A masked
// enum with more than 128 labels therefore reaches exit 9 on a true statement
// about the domain rather than about the masker. No fixture has one, and a false
// exit 9 is a run that stops and says why, where the other direction is a column
// nothing checks (internal/core/CLAUDE.md).
const smallDomainCeiling = 128

// markSmallDomains fills Decision.Domain and Decision.SmallDomain on every
// masked column whose admissible domain the catalog knows.
//
// The domains it knows are the three internal/invariants also reads from the
// catalog: an enum's label count, a boolean's two values, and a char(n) or
// varchar(n) at n of at most two. Every other type is left at zero — unknown,
// not large — because a guess here would either exempt a column from the
// residual scan that should be in it or list a column in the yml that is not
// small.
//
// Domain is recorded whenever the catalog knows it; SmallDomain is set only when
// the domain is small by both of section 5's ratio and smallDomainCeiling, since
// SmallDomain is what takes a column out of the residual filter and out of I2.
func markSmallDomains(schema *pipeline.Schema, cls *pipeline.Classification) {
	if schema == nil || cls == nil {
		return
	}
	for i := range schema.Tables {
		t := &schema.Tables[i]
		for j := range t.Columns {
			col := ref.ColumnRef{Table: t.Ref, Column: t.Columns[j].Name}
			d, ok := cls.Decisions[col]
			if !ok || !d.Masked {
				continue
			}
			domain := columnDomain(schema, t.Columns[j])
			if domain <= 0 {
				continue
			}
			d.Domain = domain
			d.SmallDomain = domain <= smallDomainCeiling &&
				domain < 2*int64(distinctSamples(t, j))
			cls.Decisions[col] = d
		}
	}
}

// columnDomain is how many distinct values the column's own type admits, or 0
// where the catalog does not say.
func columnDomain(schema *pipeline.Schema, c pipeline.Column) int64 {
	name := c.TypeName
	if c.Domain != "" {
		if base, ok := domainBase(schema, c.Domain); ok {
			name = base
		}
	}
	if labels, ok := enumLabels(schema, name); ok {
		return int64(len(labels))
	}
	switch mask.BareTypeName(mask.UnquoteType(name)) {
	case "boolean", "bool":
		return 2
	}
	if n, ok := charLength(name); ok && n >= 1 && n <= smallDomainChars {
		d := int64(1)
		for range n {
			d *= printableDomain
		}
		return d
	}
	return 0
}

// charLength is the n of character(n) or character varying(n), from the type
// name format_type produced.
func charLength(name string) (int, bool) {
	open := strings.IndexByte(name, '(')
	if open < 0 || !strings.HasSuffix(name, ")") {
		return 0, false
	}
	switch mask.BareTypeName(mask.UnquoteType(name[:open])) {
	case "character", "character varying", "bpchar", "varchar":
	default:
		return 0, false
	}
	var n int
	if _, err := fmt.Sscanf(name[open+1:len(name)-1], "%d", &n); err != nil {
		return 0, false
	}
	return n, true
}

// enumLabels resolves a type name against Schema.Enums by both spellings.
// Schema.Enums is keyed "nspname.typname", while format_type writes a type
// visible in the search_path unqualified, so a lookup by the qualified key
// alone can never match one (the same reasoning as internal/transform's).
func enumLabels(schema *pipeline.Schema, name string) ([]string, bool) {
	if labels, ok := schema.Enums[mask.UnquoteType(name)]; ok {
		return labels, true
	}
	bare := mask.BareTypeName(name)
	for key, labels := range schema.Enums {
		if mask.BareTypeName(key) == bare {
			return labels, true
		}
	}
	return nil, false
}

// domainBase returns the base type named by a domain's CREATE DOMAIN text,
// which introspect renders from the catalog's own deparser, so "AS <base type>"
// is always present.
func domainBase(schema *pipeline.Schema, domain string) (string, bool) {
	want := mask.UnquoteType(domain)
	for _, d := range schema.Domains {
		if mask.UnquoteType(d.Name) != want && mask.BareTypeName(d.Name) != mask.BareTypeName(domain) {
			continue
		}
		const as = " AS "
		i := strings.Index(d.Def, as)
		if i < 0 {
			return "", false
		}
		rest := strings.TrimSpace(d.Def[i+len(as):])
		if end := strings.IndexAny(rest, " \n\t"); end > 0 {
			rest = rest[:end]
		}
		return strings.TrimSuffix(rest, ";"), true
	}
	return "", false
}

// distinctSamples counts the distinct non-NULL sampled values of one column.
// The samples are the ones introspect already took (200 rows by TABLESAMPLE),
// which is the same set the classifier scored the column on.
func distinctSamples(t *pipeline.Table, idx int) int {
	seen := map[string]bool{}
	for _, row := range t.Samples {
		if idx >= len(row) || row[idx] == nil {
			continue
		}
		// The values never leave this function: only their count does, and a
		// count is not a value (THREAT_MODEL.md T4).
		seen[fmt.Sprint(row[idx])] = true
	}
	return len(seen)
}
