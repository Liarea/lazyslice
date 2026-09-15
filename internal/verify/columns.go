// SPDX-License-Identifier: Apache-2.0

package verify

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// What this stage needs to know about a column: which type family it belongs to
// — which decides whether it is scanned as a scalar, walked as a document or
// left to the classifier — and what the classification decided about it.
//
// The family names and the reduction are internal/transform's and
// internal/classify's, reduced to what is needed here: this package needs to
// tell a document from a character column from anything else, and never needs
// mask.Constraints, because mask.Canonical does not read them (value.go). It is
// still a third copy of one vocabulary; internal/verify/CLAUDE.md records it and
// the fix is a shared home, as internal/transform/CLAUDE.md already says.

// The family names, as mask/domain.go spells them.
const (
	famText      = "text"
	famVarchar   = "varchar"
	famBpchar    = "bpchar"
	famCitext    = "citext"
	famInteger   = "integer"
	famBigint    = "bigint"
	famNumeric   = "numeric"
	famJSON      = "json"
	famJSONB     = "jsonb"
	famHstore    = "hstore"
	famBytea     = "bytea"
	famBoolean   = "boolean"
	famFloat     = "float"
	famDate      = "date"
	famTimestamp = "timestamp"
	famTime      = "time"
	famInterval  = "interval"
	famUUID      = "uuid"
	famInet      = "inet"
	famCIDR      = "cidr"
	famMacaddr   = "macaddr"
	famOther     = "other"
)

// baseFamilies maps pg_catalog.format_type output, with any modifier and array
// suffix removed, onto a family.
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

// shapeOf reduces one column to the family and the arrayness the residual scan
// and the second net read it by, resolving a domain to its base type. An
// array's family is its element's, as ARCHITECTURE.md section 4 classifies it
// and section 5 masks it.
//
// The order of the two steps is internal/transform's shapeOf
// (internal/transform/constraints.go), and it has to be, because the residual
// filter was keyed by that function's answer. transform strips the array
// suffix from the *resolved domain base* as well as from the type name, so a
// column declared over `CREATE DOMAIN emails AS text[]` is masked element-wise
// and recorded one filter entry per element under the empty path. A shapeOf
// here that read only TypeName would call such a column a scalar, canonicalise
// the whole array through textOf and test a filter entry transform never added
// — no hit, a green tick, and every element shipped in cleartext
// (THREAT_MODEL.md T12). TestArrayDomainIsScannedElementWise pins the two
// together; nothing else does.
func (s *state) shapeOf(col pipeline.Column) (string, bool) {
	name := strings.TrimSpace(col.TypeName)
	array := false
	for strings.HasSuffix(name, "[]") {
		array = true
		name = strings.TrimSuffix(name, "[]")
	}
	if col.Domain != "" {
		if base, ok := s.domainBase(col.Domain); ok {
			name = strings.TrimSpace(base)
			for strings.HasSuffix(name, "[]") {
				array = true
				name = strings.TrimSuffix(name, "[]")
			}
		}
	}
	if fam, ok := baseFamilies[strings.ToLower(bareTypeName(stripTypmod(name)))]; ok {
		return fam, array
	}
	return famOther, array
}

// domainBase returns the base type named by a domain's CREATE DOMAIN text.
func (s *state) domainBase(domain string) (string, bool) {
	if s.schema == nil {
		return "", false
	}
	want := unquoteType(domain)
	for _, d := range s.schema.Domains {
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

func unquoteType(name string) string { return strings.ReplaceAll(name, `"`, "") }

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

// document reports the families ARCHITECTURE.md section 4 masks whole rather
// than as a scalar, and therefore the ones whose leaves the residual scan and
// the second net walk.
func document(family string) bool {
	switch family {
	case famJSON, famJSONB, famHstore:
		return true
	}
	return false
}

// character reports the character families: the ones whose value is text and
// nothing else, and which the second net's text validators run over.
func character(family string) bool {
	switch family {
	case famText, famVarchar, famBpchar, famCitext:
		return true
	}
	return false
}

// netText reports every family the second net runs its text validators over: the
// character families, plus the four this package can name and render whose
// values are exactly what a validator recognises — a uuid, an inet, a cidr and a
// macaddr.
//
// famOther is deliberately not among them, and that is the one recall hole in
// this net's coverage. A tsvector is famOther and renders as
// "'ace':1 'administr':9", which is a number and two words and therefore an
// address to addressShape; a type this package cannot name is also a type it
// cannot canonicalise. internal/verify/CLAUDE.md records the hole under
// "Decisions made during implementation" and it is what the file's coverage
// sentence says, rather than "every unmasked column".
func netText(family string) bool {
	if character(family) {
		return true
	}
	switch family {
	case famUUID, famInet, famCIDR, famMacaddr:
		return true
	}
	return false
}

// numeric reports the families the second net runs its digit validators over: a
// payment card in a bigint column is the case ARCHITECTURE.md section 4's
// surrogate-key exemption is most likely to have let through.
func numeric(family string) bool {
	switch family {
	case famInteger, famBigint, famNumeric:
		return true
	}
	return false
}

// columnOf finds one column of a table by name.
func columnOf(t *pipeline.Table, name string) (pipeline.Column, bool) {
	for _, c := range t.Columns {
		if c.Name == name {
			return c, true
		}
	}
	return pipeline.Column{}, false
}

// decision is the classification's verdict on one column, and whether it has
// one at all.
func (s *state) decision(col ref.ColumnRef) (pipeline.Decision, bool) {
	if s.cls == nil {
		return pipeline.Decision{}, false
	}
	d, ok := s.cls.Decisions[col]
	return d, ok
}

// optedOut reports a column carrying an --unmask opt-out, from the flag or from
// the committed yml. Such a column is deliberately outside the second net's
// coverage: that is why the opt-out requires a reason and expires when the
// column's type changes (ARCHITECTURE.md section 8, THREAT_MODEL.md T3).
func optedOut(d pipeline.Decision) bool {
	switch d.Source {
	case pipeline.ByYmlUnmask, pipeline.ByFlagUnmask:
		return true
	case pipeline.ByClassifier, pipeline.ByYmlRaise, pipeline.ByFKPropagation, pipeline.ByNeighbour:
		return false
	}
	return false
}

// leaf is one scalar leaf of a document, at its JSON path.
type leaf struct {
	path string
	text string
	// str reports a string leaf, as against a number. The residual scan tests
	// both, because internal/transform records both; the second net reads the
	// string leaves only, which is what ARCHITECTURE.md section 6 item 4 names.
	str bool
}

// leaves walks a decoded document and returns every leaf internal/transform
// would have masked and recorded, in the same spelling: a string leaf as
// itself, a number leaf as its JSON spelling, and neither a boolean nor a null,
// which transform records nowhere (internal/transform/CLAUDE.md, "the
// residual-filter contract").
//
// The path spelling is transform's: $.a.b[0], starting at $.
func leaves(v any) []leaf {
	doc, ok := decodeDocument(v)
	if !ok {
		return nil
	}
	var out []leaf
	walk("$", doc, &out)
	return out
}

// decodeDocument parses a document into the shape the walker takes, exactly as
// internal/transform does: a value pgx already decoded is walked as it stands,
// a string is parsed with json.Number so an integer leaf is not silently a
// float64.
func decodeDocument(v any) (any, bool) {
	s, isText := v.(string)
	if !isText {
		return v, v != nil
	}
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	var out any
	if err := dec.Decode(&out); err != nil {
		return nil, false
	}
	return out, true
}

func walk(path string, node any, out *[]leaf) {
	switch n := node.(type) {
	case map[string]any:
		for name, child := range n {
			walk(path+"."+name, child, out)
		}
	case []any:
		for i, item := range n {
			walk(path+"["+strconv.Itoa(i)+"]", item, out)
		}
	case string:
		*out = append(*out, leaf{path: path, text: n, str: true})
	case json.Number:
		*out = append(*out, leaf{path: path, text: n.String()})
	case float64:
		*out = append(*out, leaf{path: path, text: strconv.FormatFloat(n, 'g', -1, 64)})
	}
}

// keyOccurrence is one object key of a decoded document, at the JSON path of
// the position it keys — the same path spelling internal/transform's walk
// records a masked key's residual entry under (json.go, "A masked object key
// is keyed at its own path", T-0137 review round finding 1).
type keyOccurrence struct {
	path string
	name string
}

// documentKeys returns every object key of a decoded document, at every
// level, in the spelling the target holds today — internal/transform's own
// masked spelling on a run that masked a key, the source's on one that never
// needed to (T-0137, docs/reviews/2026-09-09/REVIEW.md finding 8) — together
// with its path.
//
// It walks values too, because a key can nest inside an array of objects and
// not only inside another object.
func documentKeys(v any) []keyOccurrence {
	doc, ok := decodeDocument(v)
	if !ok {
		return nil
	}
	var out []keyOccurrence
	walkKeys("$", doc, &out)
	return out
}

func walkKeys(path string, node any, out *[]keyOccurrence) {
	switch n := node.(type) {
	case map[string]any:
		for name, child := range n {
			childPath := path + "." + name
			*out = append(*out, keyOccurrence{path: childPath, name: name})
			walkKeys(childPath, child, out)
		}
	case []any:
		for i, item := range n {
			walkKeys(path+"["+strconv.Itoa(i)+"]", item, out)
		}
	}
}
