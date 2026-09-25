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
	famComposite = "composite"
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
	// A composite is told from every other type this package has no family for
	// because internal/plan has to refuse a masked one and nothing else here can
	// tell it from an ltree or a PostGIS geometry (tracker T-0094). Schema.Composites
	// is the catalog's own list, read by internal/introspect from pg_type
	// typtype 'c' with relkind 'c', so a view's row type is not in it.
	if schema != nil && isComposite(schema, name) {
		ct.Family = famComposite
		return ct
	}
	ct.Family = famOther
	return ct
}

// isComposite resolves a type name against Schema.Composites by both
// spellings, for the reason isEnum does: format_type writes a type that is
// visible in the search_path unqualified, and Schema.Composites is keyed
// "nspname.typname".
func isComposite(schema *pipeline.Schema, name string) bool {
	_, ok := findComposite(schema, name)
	return ok
}

// findComposite is isComposite's own lookup, kept as a separate function
// because compositeDocumentField (below) needs the matched definition's Def
// text and isComposite does not.
func findComposite(schema *pipeline.Schema, name string) (pipeline.NamedDef, bool) {
	want := mask.UnquoteType(name)
	bare := mask.BareTypeName(name)
	for _, c := range schema.Composites {
		if mask.UnquoteType(c.Name) == want || mask.BareTypeName(c.Name) == bare {
			return c, true
		}
	}
	return pipeline.NamedDef{}, false
}

// documentFamilies are the families ARCHITECTURE.md §4 and THREAT_MODEL.md T1
// already call a document on the row side. compositeDocumentField holds a
// composite carrying one of these fields to the same rule (T-0399).
var documentFamilies = map[string]bool{
	famJSON:   true,
	famJSONB:  true,
	famHstore: true,
}

// compositeDocumentField reports the first json, jsonb or hstore field a
// composite type holds -- walking into a field that is itself a composite,
// because the same problem recurs one level down -- the type that field is
// actually declared on (which is typeName itself for a direct field, and a
// nested composite's own name one or more levels down, never the outer type
// alone), and the family of the field it found (T-0399, THREAT_MODEL.md T1,
// the 2026-09-25 JSON red team round 1's entry 14).
//
// It exists because compositeSignal cannot see a document field's contents at
// all -- not because the record's own quoting hides it. splitCompositeLiteral
// (literal.go) already unquotes a field and undoes its doubled quotes, so a
// jsonb field's own `{"a":"bea.donnelly@example.test"}` reaches compositeSignal
// as that same bare text, not the doubled-quote form. The actual gap is that
// compositeSignal runs each whole-value validator over a field's text as one
// opaque value and never walks inside it, so ValidEmail asks whether the
// *whole* field is an email and the document's own bare address inside it is
// read only by chance (a phone-shaped run of digits, a date supplying
// AddressShape's digit), the same way a plain text column holding an
// unparsed JSON blob would be. Reading the document properly would need a
// second leaf walk keyed by the field's own type, the way jsonSignal already
// walks a plain json column; simpler and safer, and what THREAT_MODEL.md's
// composite row now says, is to refuse the column on the type alone, the
// same outcome a hit from compositeSignal already produces (decideComposite,
// classify.go). A composite that nests another composite with no document
// field of its own -- a plain email field two levels down -- is not covered
// by this fix and is filed separately (T-0413, tracker/epics/E9).
//
// typeName is the composite's own name as columnType.Elem carries it --
// already resolved past an array suffix and a domain, the way decideComposite
// receives it. seen guards the walk against two composites that reference
// each other: Postgres does not let a composite nest itself by value, but two
// mutually-referencing typtype 'c' rows could still describe a cycle, and a
// fresh map is what a package-level function with no shared state needs to
// break one.
func compositeDocumentField(schema *pipeline.Schema, typeName string) (holderType, field, family string, ok bool) {
	return documentFieldWalk(schema, typeName, map[string]bool{})
}

func documentFieldWalk(schema *pipeline.Schema, typeName string, seen map[string]bool) (holderType, field, family string, ok bool) {
	if schema == nil {
		return "", "", "", false
	}
	def, found := findComposite(schema, typeName)
	if !found {
		return "", "", "", false
	}
	key := mask.UnquoteType(def.Name)
	if seen[key] {
		return "", "", "", false
	}
	seen[key] = true
	for _, f := range compositeFields(def.Def) {
		name := f.typ
		for strings.HasSuffix(name, "[]") {
			name = strings.TrimSuffix(name, "[]")
		}
		name = mask.StripTypmod(name)
		// A field can itself be a domain -- CREATE DOMAIN docdom AS jsonb,
		// used as a field's type -- the same way a column can, so it is
		// resolved through Schema.Domains before the family and composite
		// checks below, the way typeOf already does for a column's own
		// declared type (T-0399 fix round 1 left this gap: a domain over
		// jsonb, or a domain over a composite that holds one, was read
		// undecided rather than refused).
		if base, ok := domainBase(schema, name); ok {
			name = base
			for strings.HasSuffix(name, "[]") {
				name = strings.TrimSuffix(name, "[]")
			}
			name = mask.StripTypmod(name)
		}
		if fam := strings.ToLower(mask.BareTypeName(name)); documentFamilies[fam] {
			return def.Name, f.name, fam, true
		}
		if isComposite(schema, name) {
			if ht, nf, nfam, nok := documentFieldWalk(schema, name, seen); nok {
				return ht, nf, nfam, true
			}
		}
	}
	return "", "", "", false
}

// compositeField is one field of a CREATE TYPE ... AS (...) definition.
type compositeField struct {
	name string
	typ  string
}

// compositeFields parses internal/introspect's own rendering of a composite's
// field list (sqlComposites, internal/introspect/sql.go): every field's name
// through quote_ident and its type through pg_catalog.format_type, comma
// separated inside "CREATE TYPE ns.name AS (...)". A reader and not a parser
// -- literal.go's own words for splitCompositeLiteral, which this mirrors for
// a definition's text rather than a sampled value -- so a field list it
// cannot make sense of is skipped rather than guessed at.
func compositeFields(def string) []compositeField {
	open := strings.Index(def, "(")
	if open < 0 || !strings.HasSuffix(def, ")") {
		return nil
	}
	body := def[open+1 : len(def)-1]
	var fields []compositeField
	for _, part := range splitTopLevelCommas(body) {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if name, typ, ok := splitFieldNameType(part); ok {
			fields = append(fields, compositeField{name: name, typ: typ})
		}
	}
	return fields
}

// splitTopLevelCommas splits on a comma that is outside a quoted identifier
// and outside parentheses -- a field's own type modifier, "numeric(12,2)".
func splitTopLevelCommas(s string) []string {
	var parts []string
	depth := 0
	inQuote := false
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			inQuote = !inQuote
		case '(':
			if !inQuote {
				depth++
			}
		case ')':
			if !inQuote {
				depth--
			}
		case ',':
			if !inQuote && depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, s[start:])
	return parts
}

// splitFieldNameType splits one field's "name type" text at the field name's
// own boundary: quote_ident's bare form ends at the first space, and its
// quoted form (an embedded quote doubled) ends at its own closing quote, so a
// quoted name carrying a space is never mistaken for the start of the type.
func splitFieldNameType(part string) (name, typ string, ok bool) {
	if part == "" || part[0] != '"' {
		i := strings.IndexByte(part, ' ')
		if i < 0 {
			return "", "", false
		}
		return part[:i], strings.TrimSpace(part[i+1:]), true
	}
	i := 1
	for i < len(part) {
		if part[i] == '"' {
			if i+1 < len(part) && part[i+1] == '"' {
				i += 2
				continue
			}
			break
		}
		i++
	}
	if i >= len(part) {
		return "", "", false
	}
	name = strings.ReplaceAll(part[1:i], `""`, `"`)
	rest := strings.TrimSpace(part[i+1:])
	if rest == "" {
		return "", "", false
	}
	return name, rest, true
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
