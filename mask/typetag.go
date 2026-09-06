// SPDX-License-Identifier: Apache-2.0

package mask

import "strings"

// The one home for "which family is this type?".
//
// Constraints.TypeTag is a family name, and until T-0054 three packages worked
// one out for themselves: internal/classify (types.go, to gate a category by
// the rule pack's accepts:), internal/transform (constraints.go, to build the
// Constraints a masker reads) and, once the plan-time write-back check landed,
// internal/plan. Three copies of one table is three chances for them to
// disagree about a column, and a disagreement is not a cosmetic one: the plan
// check would then be answering a question about a different column than the
// one transform masks, which is exactly the failure T-0054 part 3 exists to
// close ("transform must never be the first place a type mismatch is
// discovered").
//
// So the table lives here, beside the generators whose output it is a question
// about, and internal/transform and internal/plan both read it.
// internal/classify keeps its own table, because it maps a type onto more than
// a family: it carries famEnum, famXML and famOther, which nothing here has a
// tag for and which its own value-signal gate reads. Its quoting is not a
// second copy any more — it calls the three helpers below —
// and internal/classify/CLAUDE.md records the remaining pair.

// baseTypes maps pg_catalog.format_type output — with any type modifier and any
// array suffix already removed by the caller — onto a type tag. A type that is
// not here is not a family this module knows; the caller decides what to do
// with that, and both of today's callers treat it as a column they cannot judge
// rather than one they refuse.
var baseTypes = map[string]string{
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
	"tsvector":                    famTSVector,
}

// TypeTag maps one base type name onto the tag Constraints.TypeTag carries.
//
// The name is the catalog's own spelling with the type modifier and the array
// suffix already stripped and the schema qualification already dropped: an
// array is masked element-wise under its element type (ARCHITECTURE.md §5) and
// a domain under its base type, and both reductions need the schema, which this
// module cannot see. The second return is false for a type no family covers —
// an enum, a composite, an extension type — which is a column the caller has
// more to say about than this table does.
func TypeTag(name string) (string, bool) {
	tag, ok := baseTypes[strings.ToLower(strings.TrimSpace(name))]
	return tag, ok
}

// MaxLen is Constraints.MaxLen for a column of this type tag: pg_attribute's
// atttypmod less the four-byte header, and only for the families where
// atttypmod is a length.
//
// numeric's modifier packs a precision and a scale, not a byte count, and
// reading it as one would clamp a masked number to a nonsense width —
// digitBudget takes MaxLen as a digit bound. A numeric's own precision
// therefore does not reach the generators; that is a gap in Constraints, and
// internal/transform/CLAUDE.md records it.
//
// It lives here for the same reason TypeTag does: internal/plan asks the
// question to decide whether a column can be written into at all, and
// internal/transform asks it to build the Constraints a generator reads. Two
// answers would be two different columns.
func MaxLen(tag string, typmod int32) int {
	switch tag {
	case famVarchar, famBpchar, famCitext:
		if typmod > 4 {
			return int(typmod - 4)
		}
	}
	return 0
}

// The three helpers below reduce pg_catalog.format_type's spelling of a type
// to the name TypeTag and a Schema.Enums / Schema.Domains key are written in.
//
// They live beside TypeTag for the reason TypeTag lives here: their only job is
// to produce its argument, and a caller that stripped a type modifier
// differently would be asking about a different column. T-0054 moved the family
// table here and left the quoting to each caller, which made a fourth copy of
// it; these exports retire three of those (internal/classify, internal/plan,
// internal/transform). internal/verify has its own copy still and is outside
// T-0054's paths.
//
// What is not here is the domain and enum resolution: both need the schema,
// which this module cannot see.

// StripTypmod removes the "(15)" from "character varying(15)" and the "(5,2)"
// from "numeric(5,2)". A quoted identifier may hold a bracket, so the cut is
// made only outside quotes.
func StripTypmod(name string) string {
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

// UnquoteType turns format_type's quoted spelling, public."bıgınt", back into
// the name a schema's enum and domain maps are keyed by.
func UnquoteType(name string) string { return strings.ReplaceAll(name, `"`, "") }

// BareTypeName drops a schema qualification from a type name, quoting-aware:
// "public.citext" is citext and `public."My Type"` is `My Type`. A name with no
// qualification, including a multi-word built-in such as "character varying",
// is returned unquoted and otherwise unchanged.
func BareTypeName(name string) string {
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
		return UnquoteType(name)
	}
	return UnquoteType(name[cut+1:])
}
