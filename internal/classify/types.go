// SPDX-License-Identifier: Apache-2.0

package classify

import (
	"strings"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/mask"
)

// A type family is the classifier's normalised name for a PostgreSQL type. The
// rule pack's `accepts:` lists are written in these names and a reason line
// prints one as {type}, so the set is closed: every family is a lower-case
// identifier, which is what keeps ARCHITECTURE.md §4's conflict sentence
// ("boolean is not an accepted type for email") inside the reason grammar.
//
// An unrecognised type is famOther, which no category accepts. That is
// deliberate: a name hit on a type lazyslice does not understand is recorded at
// `low` and named, rather than masked with a generator that would fail the load.
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
	famTSVector  = "tsvector"
	famXML       = "xml"
	famEnum      = "enum"
	famOther     = "other"
)

// baseFamilies maps pg_catalog.format_type output, with any type modifier and
// any array suffix already removed, onto a family.
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
	"tsvector":                    famTSVector,
	"xml":                         famXML,
}

// typeSignals are the families ARCHITECTURE.md §4 lists as signals in their own
// right: a column of one of these types carries a category even when its name
// says nothing at all (testdata/README.md trap 18).
var typeSignals = map[string]pipeline.Category{
	famInet:    pipeline.CatNetworkID,
	famCIDR:    pipeline.CatNetworkID,
	famMacaddr: pipeline.CatNetworkID,
	famJSON:    pipeline.CatSemiStruct,
	famJSONB:   pipeline.CatSemiStruct,
	famHstore:  pipeline.CatSemiStruct,
}

// columnType is one column's type reduced to what classification needs.
type columnType struct {
	// Family is the element family for an array column: ARCHITECTURE.md §4
	// classifies an array on its element type, so text[] named like an email
	// column is email exactly as text would be.
	Family string
	// Array reports that the column is an array, so the reason says which
	// element type decided it and the validators run over the elements.
	Array bool
	// Elem is the element type name as written, for the reason line.
	Elem string
}

// typeOf reduces a column's declared type to a family, resolving a domain to
// its base type and an enum to famEnum. It never guesses: a type it does not
// know is famOther, which no category accepts.
func typeOf(schema *pipeline.Schema, col pipeline.Column) columnType {
	name := strings.TrimSpace(col.TypeName)
	ct := columnType{}
	for strings.HasSuffix(name, "[]") {
		ct.Array = true
		name = strings.TrimSuffix(name, "[]")
	}
	// A domain is resolved through its CREATE DOMAIN text, which introspect
	// renders from the catalog's own deparser, so "AS <base type>" is always
	// present and always the catalog's spelling.
	if col.Domain != "" {
		if base, ok := domainBase(schema, col.Domain); ok {
			name = base
			for strings.HasSuffix(name, "[]") {
				ct.Array = true
				name = strings.TrimSuffix(name, "[]")
			}
		}
	}
	name = mask.StripTypmod(name)
	ct.Elem = name
	// Column.TypeName is pg_catalog.format_type output, which qualifies any type
	// that is not visible in the connection's search_path: an extension
	// installed into its own schema (ARCHITECTURE.md §11.1 item 2) spells its
	// types "extensions.citext" and "public.hstore", and a role with a custom
	// search_path can produce that spelling for anything. The family is
	// therefore matched on the bare type name as well as on the whole string,
	// so that citext, public.citext and extensions.citext are one type. Without
	// it an email column of a qualified citext is famOther, which no category
	// accepts, and §4's own examples ("inet, macaddr, citext, domains, jsonb")
	// would be silent cleartext.
	if fam, ok := baseFamilies[strings.ToLower(mask.BareTypeName(name))]; ok {
		ct.Family = fam
		return ct
	}
	if schema != nil && isEnum(schema, name) {
		ct.Family = famEnum
		return ct
	}
	ct.Family = famOther
	return ct
}

// isEnum resolves a type name against Schema.Enums by both spellings.
// Schema.Enums is keyed "nspname.typname" (internal/introspect/sql.go), while
// format_type writes an enum that is visible in the search_path unqualified, so
// a lookup by the qualified key alone can never match one.
func isEnum(schema *pipeline.Schema, name string) bool {
	if _, ok := schema.Enums[mask.UnquoteType(name)]; ok {
		return true
	}
	bare := mask.BareTypeName(name)
	for key := range schema.Enums {
		if mask.BareTypeName(key) == bare {
			return true
		}
	}
	return false
}

// domainBase returns the base type named by a domain's CREATE DOMAIN text.
func domainBase(schema *pipeline.Schema, domain string) (string, bool) {
	if schema == nil {
		return "", false
	}
	want := mask.UnquoteType(domain)
	for _, d := range schema.Domains {
		// The qualified spelling first, then the bare one: Column.Domain is
		// written "nspname.typname" today, but a domain named unqualified
		// somewhere must resolve to the same base type rather than to famOther.
		if mask.UnquoteType(d.Name) != want && mask.BareTypeName(d.Name) != mask.BareTypeName(domain) {
			continue
		}
		const as = " AS "
		i := strings.Index(d.Def, as)
		if i < 0 {
			return "", false
		}
		rest := d.Def[i+len(as):]
		// The clauses that may follow the base type, in the order
		// internal/introspect renders them.
		for _, clause := range []string{" COLLATE ", " DEFAULT ", " NOT NULL", " CONSTRAINT "} {
			if j := strings.Index(rest, clause); j >= 0 {
				rest = rest[:j]
			}
		}
		return strings.TrimSpace(rest), true
	}
	return "", false
}
