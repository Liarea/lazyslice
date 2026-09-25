// SPDX-License-Identifier: Apache-2.0

package plan

import (
	"strings"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/mask"
)

// documentField, and the small parser beneath it, are internal/classify's own
// functions of the same names (types.go), duplicated here for the reason
// every other copy in this file already is (compositeType, domainBase): a
// stage package may not import another (internal/CLAUDE.md), and
// checkWriteBack's own composite branch wants the offending field's name in
// its exit-12 message, not only the type (T-0399, THREAT_MODEL.md T1, the
// 2026-09-25 JSON red team round 1's entry 14).
//
// This package decides nothing new with it: checkWriteBack calls documentField
// only after internal/classify's own decideComposite has already set
// d.Masked, whether that was decided from a name, a value hit, or this same
// structural check running there first. What this copy adds is the field's
// name in the message an operator actually reads, because "type public.wrap
// is a composite" names the type but not why -- and --skip-table/--unmask
// are the same two escapes the plain composite refusal already offers.
// The first return is the type the field is actually declared on, which is
// typeName itself for a direct field and a nested composite's own name one or
// more levels down for a nested one -- never the outer type alone, the same
// distinction internal/classify's own copy makes and for the same reason: a
// message naming the wrong type sends an operator to a field that is not
// there.
func documentField(schema *pipeline.Schema, typeName string) (holderType, field, family string, ok bool) {
	return documentFieldWalk(schema, typeName, map[string]bool{})
}

func documentFieldWalk(schema *pipeline.Schema, typeName string, seen map[string]bool) (holderType, field, family string, ok bool) {
	def, found := findCompositeDef(schema, typeName)
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
		// checks below, the way p.compositeType already does for a column's
		// own declared type (T-0399 fix round 1 left this gap: a domain over
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
		if _, nested := findCompositeDef(schema, name); nested {
			if ht, nf, nfam, nok := documentFieldWalk(schema, name, seen); nok {
				return ht, nf, nfam, true
			}
		}
	}
	return "", "", "", false
}

// domainBase returns the base type named by a domain's CREATE DOMAIN text --
// the same lookup as (*run).domainBase in writeback.go, duplicated here
// package-level because documentFieldWalk has only schema, not a *run, the
// same reason every other function in this file is already its own copy.
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
	"json":   true,
	"jsonb":  true,
	"hstore": true,
}

// findCompositeDef is compositeType's own lookup (writeback.go), generalised
// to take an arbitrary type name rather than a column: the walk above has to
// resolve a *field's* type, not only a column's declared one.
func findCompositeDef(schema *pipeline.Schema, name string) (pipeline.NamedDef, bool) {
	want := mask.UnquoteType(name)
	bare := mask.BareTypeName(name)
	for _, c := range schema.Composites {
		if mask.UnquoteType(c.Name) == want || mask.BareTypeName(c.Name) == bare {
			return c, true
		}
	}
	return pipeline.NamedDef{}, false
}

// compositeField, compositeFields, splitTopLevelCommas and splitFieldNameType
// parse internal/introspect's own rendering of a composite's field list
// (sqlComposites, internal/introspect/sql.go) -- a reader and not a parser,
// the same as every other literal reader in this tree: a field list either
// cannot make sense of is skipped rather than guessed at.
type compositeField struct {
	name string
	typ  string
}

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
