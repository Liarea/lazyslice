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
// (internal/classify/types.go). Neither list can be imported from here — mask's
// constants are unexported, and internal/classify is another stage package,
// which internal/CLAUDE.md forbids reaching into — so the vocabulary and the
// reduction below are a third copy of one thing. internal/transform/CLAUDE.md
// records that, and the fix is a shared home for the type families rather than
// a fourth copy.

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
	famEnum      = "enum"
	famOther     = "other"
)

// baseFamilies maps pg_catalog.format_type output, with any type modifier and
// any array suffix already removed, onto a family. A type not here is famOther,
// which the generators treat as a string of unknown width.
var baseFamilies = map[string]string{
	"text":                        famText,
	"character varying":           famVarchar,
	"varchar":                     famVarchar,
	"character":                   famBpchar,
	"char":                        famBpchar,
	"bpchar":                      famBpchar,
	"citext":                      famCitext,
	"name":                        famText,
	"boolean":                     famBoolean,
	"bool":                        famBoolean,
	"smallint":                    famInteger,
	"integer":                     famInteger,
	"int":                         famInteger,
	"int2":                        famInteger,
	"int4":                        famInteger,
	"bigint":                      famBigint,
	"int8":                        famBigint,
	"numeric":                     famNumeric,
	"decimal":                     famNumeric,
	"money":                       famNumeric,
	"real":                        famFloat,
	"double precision":            famFloat,
	"float4":                      famFloat,
	"float8":                      famFloat,
	"date":                        famDate,
	"timestamp":                   famTimestamp,
	"timestamp without time zone": famTimestamp,
	"timestamp with time zone":    famTimestamp,
	"timestamptz":                 famTimestamp,
	"time":                        famTime,
	"time without time zone":      famTime,
	"time with time zone":         famTime,
	"timetz":                      famTime,
	"interval":                    famInterval,
	"uuid":                        famUUID,
	"inet":                        famInet,
	"cidr":                        famCIDR,
	"macaddr":                     famMacaddr,
	"macaddr8":                    famMacaddr,
	"bytea":                       famBytea,
	"json":                        famJSON,
	"jsonb":                       famJSONB,
	"hstore":                      famHstore,
}

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
	name = stripTypmod(name)

	var labels []string
	switch fam, ok := baseFamilies[strings.ToLower(bareTypeName(name))]; {
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
		MaxLen:     maxLen(shape.family, col.TypMod),
		EnumLabels: labels,
		Checks:     col.Checks,
		Nullable:   col.Nullable,
		Unique:     uniqueColumn(tbl, col.Name),
	}
	return shape
}

// maxLen is atttypmod less the four-byte header, and only for the families
// where atttypmod is a length. numeric's modifier packs a precision and a
// scale, not a byte count, and reading it as one would clamp a masked number to
// a nonsense width — mask/domain.go's digitBudget takes MaxLen as a digit
// bound. A numeric's own precision therefore does not reach the masker; that is
// a gap in mask.Constraints, recorded in internal/transform/CLAUDE.md.
func maxLen(family string, typmod int32) int {
	switch family {
	case famVarchar, famBpchar, famCitext:
		if typmod > 4 {
			return int(typmod - 4)
		}
	}
	return 0
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
	if labels, ok := t.schema.Enums[unquoteType(name)]; ok {
		return labels, true
	}
	bare := bareTypeName(name)
	for key, labels := range t.schema.Enums {
		if bareTypeName(key) == bare {
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
	want := unquoteType(domain)
	for _, d := range t.schema.Domains {
		if unquoteType(d.Name) != want && bareTypeName(d.Name) != bareTypeName(domain) {
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

// stripTypmod removes the "(15)" from "character varying(15)" and the "(5,2)"
// from "numeric(5,2)". A quoted identifier may hold a bracket, so the cut is
// made only outside quotes.
func stripTypmod(name string) string {
	inQuote := false
	for i, r := range name {
		switch r {
		case '"':
			inQuote = !inQuote
		case '(':
			if !inQuote {
				return strings.TrimSpace(name[:i])
			}
		}
	}
	return name
}

// unquoteType turns format_type's quoted spelling back into the name
// Schema.Enums and Schema.Domains are keyed by.
func unquoteType(name string) string {
	return strings.ReplaceAll(name, `"`, "")
}

// bareTypeName drops a schema qualification from a type name, quoting-aware.
func bareTypeName(name string) string {
	inQuote := false
	cut := -1
	for i, r := range name {
		switch r {
		case '"':
			inQuote = !inQuote
		case '.':
			if !inQuote {
				cut = i
			}
		}
	}
	if cut < 0 {
		return unquoteType(name)
	}
	return unquoteType(name[cut+1:])
}
