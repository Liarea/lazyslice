// SPDX-License-Identifier: Apache-2.0

package verify

import (
	"fmt"
	"strings"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// compositeDocuments is T-0399's mirror of internal/classify's own structural
// check (that package's types.go, compositeDocumentField; classify.go,
// decideComposite): a composite type holding a json, jsonb or hstore field --
// or a nested composite that does -- is refused at plan on the type alone,
// because compositeSignal can never read a document field's contents at all.
// The record's own quoting is not what hides it: splitCompositeLiteral
// already unquotes a field and undoes its doubled quotes, so a jsonb field's
// `{"a":"bea@example.test"}` reaches a validator as that same bare text. What
// hides it is that compositeSignal runs each whole-value validator over a
// field's text as one opaque value and never walks inside it, so a bare
// address inside the document is read only by chance (the 2026-09-25 JSON
// red team, round 1, entry 14; THREAT_MODEL.md T1's composite row).
//
// This is the second, independent look every other row-side control in this
// stage already gets (the catalog pass beside the second net, catalog.go): a
// copy that somehow reached the target holding such a column -- an
// operator's own --unmask excepted, the accepted risk that escape exists for
// -- is refused here too, rather than trusted because the plan said so.
//
// It reads only s.schema.Composites, the same catalog internal/plan read at
// plan time (§11.1 recreates a composite verbatim, so the target's own type
// holds the identical fields), over every table this run actually loaded
// (s.steps): a table the run dropped to SchemaOnly still recreates the type
// but copies no row of it, and a table outside the plan's steps holds none of
// this run's data at all.
func (s *state) compositeDocuments() {
	for _, step := range s.steps {
		if step.Mode == pipeline.SchemaOnly {
			// Recreates the type, copies no row of it (shapes.go, counts.go
			// do the same skip): --skip-table, an unreachable table, and an
			// unreadable one all land here, and none of them wrote a row
			// this net needs to catch.
			continue
		}
		t, ok := s.tables[step.Table]
		if !ok || t == nil {
			continue
		}
		for _, col := range t.Columns {
			base, ok := s.compositeTypeOf(col)
			if !ok {
				continue
			}
			holder, field, fam, ok := documentField(s.schema, base, map[string]bool{})
			if !ok {
				continue
			}
			cref := ref.ColumnRef{Table: step.Table, Column: col.Name}
			if d, has := s.decision(cref); has && optedOut(d) {
				// The operator's own --unmask for this column: the accepted
				// risk the escape exists for, not a hole this net should
				// close behind their back.
				continue
			}
			s.fail(&Refusal{
				Code: CodeRefusedCompositeDocument, Exit: exitResidual, Check: checkCatalog,
				Table: step.Table, Column: col.Name, Count: 1,
				Reason: fmt.Sprintf("its type %s is a composite whose %s field %s -- a document no validator reads inside, only its own text as a whole",
					holder, fam, field),
			})
		}
	}
}

// compositeTypeOf reports the composite type a column is declared over, after
// an array suffix and a domain have been resolved -- internal/plan's own
// compositeType (writeback.go), read here because a stage package may not
// import another (internal/CLAUDE.md).
func (s *state) compositeTypeOf(col pipeline.Column) (string, bool) {
	name := strings.TrimSpace(col.TypeName)
	for strings.HasSuffix(name, "[]") {
		name = strings.TrimSuffix(name, "[]")
	}
	if col.Domain != "" {
		if base, ok := s.domainBase(col.Domain); ok {
			name = base
			for strings.HasSuffix(name, "[]") {
				name = strings.TrimSuffix(name, "[]")
			}
		}
	}
	name = unquoteType(stripTypmod(name))
	bare := bareTypeName(name)
	for _, c := range s.schema.Composites {
		if unquoteType(c.Name) == name || bareTypeName(c.Name) == bare {
			return c.Name, true
		}
	}
	return "", false
}

// documentField, and the small parser beneath it, are internal/classify's own
// functions of the same names (types.go), duplicated for the reason every
// other copy in this file already is: a stage package may not import
// another. seen guards the walk against two composites that reference each
// other, the same way the classify copy's own comment explains. The first
// return is the type the field is actually declared on -- typeName itself for
// a direct field, a nested composite's own name one or more levels down for a
// nested one -- never the outer type alone: a message naming the wrong type
// sends an operator to a field that is not there.
func documentField(schema *pipeline.Schema, typeName string, seen map[string]bool) (holderType, field, family string, ok bool) {
	def, found := findCompositeDef(schema, typeName)
	if !found {
		return "", "", "", false
	}
	key := unquoteType(def.Name)
	if seen[key] {
		return "", "", "", false
	}
	seen[key] = true
	for _, f := range compositeFields(def.Def) {
		name := f.typ
		for strings.HasSuffix(name, "[]") {
			name = strings.TrimSuffix(name, "[]")
		}
		name = stripTypmod(name)
		// A field can itself be a domain -- CREATE DOMAIN docdom AS jsonb,
		// used as a field's type -- the same way a column can, so it is
		// resolved through Schema.Domains before the family and composite
		// checks below, the way s.domainBase already does for a column's own
		// declared type (T-0399 fix round 1 left this gap: a domain over
		// jsonb, or a domain over a composite that holds one, was read
		// undecided rather than refused).
		if base, ok := documentFieldDomainBase(schema, name); ok {
			name = base
			for strings.HasSuffix(name, "[]") {
				name = strings.TrimSuffix(name, "[]")
			}
			name = stripTypmod(name)
		}
		if fam := strings.ToLower(bareTypeName(name)); documentFamilies[fam] {
			return def.Name, f.name, fam, true
		}
		if _, nested := findCompositeDef(schema, name); nested {
			if ht, nf, nfam, nok := documentField(schema, name, seen); nok {
				return ht, nf, nfam, true
			}
		}
	}
	return "", "", "", false
}

// documentFieldDomainBase returns the base type named by a domain's CREATE
// DOMAIN text -- the same lookup as (*state).domainBase in columns.go,
// duplicated here package-level because documentField has only schema, not a
// *state, the same reason every other function in this file is already its
// own copy.
func documentFieldDomainBase(schema *pipeline.Schema, domain string) (string, bool) {
	want := unquoteType(domain)
	for _, d := range schema.Domains {
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

// documentFamilies mirrors internal/classify's own map of the same name: the
// families ARCHITECTURE.md §4 and THREAT_MODEL.md T1 already call a document
// on the row side.
var documentFamilies = map[string]bool{
	famJSON:   true,
	famJSONB:  true,
	famHstore: true,
}

// findCompositeDef resolves a type name against Schema.Composites by both
// spellings, the reason compositeTypeOf and internal/classify's own
// findComposite both do: format_type writes a type that is visible in the
// search_path unqualified, and Schema.Composites is keyed "nspname.typname".
func findCompositeDef(schema *pipeline.Schema, name string) (pipeline.NamedDef, bool) {
	want := unquoteType(name)
	bare := bareTypeName(name)
	for _, c := range schema.Composites {
		if unquoteType(c.Name) == want || bareTypeName(c.Name) == bare {
			return c, true
		}
	}
	return pipeline.NamedDef{}, false
}

// compositeField is one field of a CREATE TYPE ... AS (...) definition.
type compositeField struct {
	name string
	typ  string
}

// compositeFields parses internal/introspect's own rendering of a composite's
// field list (sqlComposites, internal/introspect/sql.go): every field's name
// through quote_ident and its type through pg_catalog.format_type, comma
// separated inside "CREATE TYPE ns.name AS (...)". A reader and not a parser:
// a field list it cannot make sense of is skipped rather than guessed at.
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
