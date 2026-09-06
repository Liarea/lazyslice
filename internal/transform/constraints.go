// SPDX-License-Identifier: Apache-2.0

package transform

import (
	"strings"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/mask"
)

// What the column will accept, gathered from the schema introspect read and
// handed to the masker so that it stays inside the shape the application checks
// (ARCHITECTURE.md §5 "Preserve what the application checks").
//
// The type families are mask's own names, which are internal/classify's names
// (internal/classify/types.go). The *table* that maps a catalog type name onto
// one of them is no longer a copy: T-0054 moved it into mask (mask.TypeTag),
// because internal/plan's write-back check has to reduce a column to the same
// family this package does, and a plan that judged a different family than
// transform masks would be a check about a different column. The family names
// below stay spelled out here, because this package tests three of them for
// their own behaviour (isDocument, maxLen) and mask's constants are unexported;
// TestFamilyNamesMatchMask walks the two vocabularies against each other.

// The family names, as mask/domain.go spells them.
const (
	famText      = "text"
	famVarchar   = "varchar"
	famBpchar    = "bpchar"
	famCitext    = "citext"
	famBoolean   = "boolean"
	famInteger   = "integer"
	famBigint    = "bigint"
	famNumeric   = "numeric"
	famFloat     = "float"
	famDate      = "date"
	famTimestamp = "timestamp"
	famTime      = "time"
	famInterval  = "interval"
	famUUID      = "uuid"
	famInet      = "inet"
	famCIDR      = "cidr"
	famMacaddr   = "macaddr"
	famBytea     = "bytea"
	famJSON      = "json"
	famJSONB     = "jsonb"
	famHstore    = "hstore"
	// famTSVector is the family T-0054 added. A tsvector is derived from text
	// that may itself be masked, so it is never copied: internal/classify
	// decides `derived_text` on it by type alone and the masker empties it, and
	// the empty tsvector travels as "" -- which is what ''::tsvector is.
	famTSVector = "tsvector"
	famEnum     = "enum"
	famOther    = "other"
)

// columnShape is one column reduced to what masking needs. For an array column
// the family and the constraints are the *element's*: ARCHITECTURE.md §5 masks
// an array element-wise under the element type, and §4 classifies it the same
// way.
type columnShape struct {
	family      string
	array       bool
	constraints mask.Constraints
}

// shapeOf reduces one column to its family and the constraints the masker
// reads. A domain resolves to its base type and an enum to famEnum with its
// labels, because a masked enum has to come back a valid label (§5).
func (t transformer) shapeOf(tbl *pipeline.Table, col pipeline.Column) columnShape {
	name := strings.TrimSpace(col.TypeName)
	shape := columnShape{}
	for strings.HasSuffix(name, "[]") {
		shape.array = true
		name = strings.TrimSuffix(name, "[]")
	}
	if col.Domain != "" {
		if base, ok := t.domainBase(col.Domain); ok {
			name = base
			for strings.HasSuffix(name, "[]") {
				shape.array = true
				name = strings.TrimSuffix(name, "[]")
			}
		}
	}
	name = mask.StripTypmod(name)

	var labels []string
	switch fam, ok := mask.TypeTag(mask.BareTypeName(name)); {
	case ok:
		shape.family = fam
	default:
		if l, isEnum := t.enumLabels(name); isEnum {
			shape.family, labels = famEnum, l
		} else {
			shape.family = famOther
		}
	}

	shape.constraints = mask.Constraints{
		TypeTag:    shape.family,
		MaxLen:     mask.MaxLen(shape.family, col.TypMod),
		EnumLabels: labels,
		Checks:     col.Checks,
		Nullable:   col.Nullable,
		Unique:     uniqueColumn(tbl, col.Name),
	}
	return shape
}

// uniqueColumn reports that the column sits alone under a unique index or is
// the whole primary key, which is what ARCHITECTURE.md §5's uniqueness rule
// means by "the column is under a unique index". A generator reads it: an email
// masker on a unique column emits a hash-derived suffix.
//
// pipeline.Decision has a UniqueIndex field for this and internal/classify does
// not set it — it sets a category, a confidence and the rule pack's default
// masker and nothing else — so the schema is the source that is right today.
// When §5's plan-time domain check lands and fills Decision.UniqueIndex the two
// have to agree; internal/transform/CLAUDE.md records that, and Transform takes
// the Decision's answer whenever it is the stronger one.
func uniqueColumn(tbl *pipeline.Table, name string) bool {
	if tbl == nil {
		return false
	}
	if len(tbl.PK) == 1 && tbl.PK[0] == name {
		return true
	}
	for _, idx := range tbl.Indexes {
		if idx.Unique && !idx.Partial && !idx.Expression && len(idx.Columns) == 1 && idx.Columns[0] == name {
			return true
		}
	}
	return false
}

// enumLabels resolves a type name against Schema.Enums by both spellings.
// Schema.Enums is keyed "nspname.typname", while format_type writes a type
// visible in the search_path unqualified, so a lookup by the qualified key
// alone can never match one.
func (t transformer) enumLabels(name string) ([]string, bool) {
	if t.schema == nil {
		return nil, false
	}
	if labels, ok := t.schema.Enums[mask.UnquoteType(name)]; ok {
		return labels, true
	}
	bare := mask.BareTypeName(name)
	for key, labels := range t.schema.Enums {
		if mask.BareTypeName(key) == bare {
			return labels, true
		}
	}
	return nil, false
}

// domainBase returns the base type named by a domain's CREATE DOMAIN text,
// which introspect renders from the catalog's own deparser, so "AS <base type>"
// is always present and always the catalog's spelling.
func (t transformer) domainBase(domain string) (string, bool) {
	if t.schema == nil {
		return "", false
	}
	want := mask.UnquoteType(domain)
	for _, d := range t.schema.Domains {
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
