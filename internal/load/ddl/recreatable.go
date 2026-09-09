// SPDX-License-Identifier: Apache-2.0

package ddl

import (
	"fmt"
	"strings"

	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// The refusals ARCHITECTURE.md section 11.1 raises when a recreated object
// depends on one v1 does not recreate. Section 11.1 raises them at plan — before
// the snapshot is used for keys and before anything in the target is dropped —
// so their row in internal/event/catalogue.yml carries stage plan.
//
// That is where it runs, since T-0097: internal/core's planStage calls
// Recreatable before it builds the plan request and before Plan issues its first
// key query, so the refusal costs one introspect and nothing else.
// internal/plan's own checkRecreatable is a different check on the same
// sentence, covering ForeignKey.NotRecreatable. Until T-0097 the only caller was
// load.Load, as its first statement -- before the marker row and before the
// first drop, so the target was never destroyed for a schema that cannot be
// recreated, but the refusal arrived after the snapshot had been used for every
// key and every row had been extracted. One of the ten schemas in
// testdata/torture/ reaches it as the fixture stands (mastodon, unedited);
// gitlab reaches it upstream on two objects its 43-table subset removes, which
// docs/TORTURE.md and testdata/torture/gitlab/README.md both record. That is
// what made the cost measurable.
//
// There is deliberately no flag that drops the offending default and carries on,
// because the application's first INSERT is the point of the tool. The remedy is
// the load-into-existing-schema mode ADR-005's reversal condition names, which
// ARCHITECTURE.md section 14 defers with an epic.
const (
	// CodeNotRecreatableFunction is a default, check, generated expression or
	// index expression that calls a function v1 does not recreate.
	CodeNotRecreatableFunction event.Code = "target.schema.not_recreatable.function"

	// CodeNotRecreatableCollation is a column or index using a collation v1
	// does not recreate.
	CodeNotRecreatableCollation event.Code = "target.schema.not_recreatable.collation"
)

// ExitNotRecreatable is ADR-005's exit code 13, "target schema not recreatable".
const ExitNotRecreatable = 13

// Refusal is a dependency on an object class v1 does not recreate. It names the
// table, the object inside it, the dependency and its kind, which is what
// ARCHITECTURE.md section 11.1 requires the refusal to print.
//
// Object carries a column, index, constraint or domain name — whichever of them
// holds the dependency. event.ArgKey has no key for an index or a constraint
// (internal/event/event.go), so all three travel under {column}; adding keys for
// them and re-templating the two messages is owed to the task that adds an
// ArgKey.
type Refusal struct {
	Code       event.Code
	Exit       int
	Table      ref.TableRef
	Object     string
	Dependency string
}

func (r *Refusal) Error() string {
	where := r.Object
	if r.Table.Name != "" {
		where = r.Table.String() + "." + r.Object
	}
	return fmt.Sprintf("ddl: %s: %s depends on %s, which lazyslice does not recreate",
		r.Code, where, r.Dependency)
}

// Recreatable checks, at plan time, that no object PreData or PostData creates
// depends on one v1 does not recreate. A dependency is a *Refusal carrying exit
// 13.
//
// What it detects, and what it does not, from the *pipeline.Schema it is given:
//
//   - A default, generated expression, check constraint, exclusion constraint or
//     index definition whose text calls a function or procedure that appears in
//     Schema.NotRecreated. This is a name match over the catalog's own deparsed
//     text, not a parse: a user function sharing a name with a built-in one it
//     shadows would be refused where pg_depend would have allowed it, which is
//     the direction that fails closed.
//   - A column or index using a collation that appears in Schema.NotRecreated.
//
// Not detected, because *pipeline.Schema does not carry what would answer it,
// and each is an owed introspect field rather than a silent pass: a column of a
// user-defined base type (indistinguishable here from a column of an extension's
// type, which is recreated with the extension), an index using a user-defined
// operator class (pipeline.Object has no "operator class" kind), and a domain
// whose CHECK calls a user function through an operator rather than a call.
// internal/load/CLAUDE.md records the three.
func Recreatable(s *pipeline.Schema) error {
	if s == nil {
		return nil
	}
	funcs := notRecreatedNames(s, "function", "procedure")
	colls := notRecreatedNames(s, "collation")

	for _, t := range recreated(s) {
		for _, c := range t.Columns {
			for _, text := range []string{c.Default, c.Generated} {
				if dep := calledFrom(text, funcs); dep != "" {
					return functionRefusal(t.Ref, c.Name, dep)
				}
			}
			if c.Collation != "" && colls[c.Collation] {
				return &Refusal{
					Code:       CodeNotRecreatableCollation,
					Exit:       ExitNotRecreatable,
					Table:      t.Ref,
					Object:     c.Name,
					Dependency: c.Collation,
				}
			}
		}
		for _, con := range t.Constraints {
			if con.Kind != 'c' && con.Kind != 'x' {
				continue
			}
			if dep := calledFrom(con.Def, funcs); dep != "" {
				return functionRefusal(t.Ref, con.Name, dep)
			}
		}
		for _, idx := range t.Indexes {
			if dep := calledFrom(idx.Def, funcs); dep != "" {
				return functionRefusal(t.Ref, idx.Name, dep)
			}
			if dep := usesCollation(idx.Def, colls); dep != "" {
				return &Refusal{
					Code:       CodeNotRecreatableCollation,
					Exit:       ExitNotRecreatable,
					Table:      t.Ref,
					Object:     idx.Name,
					Dependency: dep,
				}
			}
		}
	}
	for _, d := range s.Domains {
		if dep := calledFrom(d.Def, funcs); dep != "" {
			return functionRefusal(ref.TableRef{}, d.Name, dep)
		}
	}
	return nil
}

func functionRefusal(t ref.TableRef, object, dep string) *Refusal {
	return &Refusal{
		Code:       CodeNotRecreatableFunction,
		Exit:       ExitNotRecreatable,
		Table:      t,
		Object:     object,
		Dependency: dep,
	}
}

// notRecreatedNames is the objects of the given kinds, by both their qualified
// and their bare name: a deparsed expression writes the qualified form only when
// the object is outside the search path.
func notRecreatedNames(s *pipeline.Schema, kinds ...string) map[string]bool {
	want := map[string]bool{}
	for _, k := range kinds {
		want[k] = true
	}
	out := map[string]bool{}
	for _, o := range s.NotRecreated {
		if !want[o.Kind] {
			continue
		}
		out[o.Name] = true
		if i := strings.Index(o.Name, "."); i >= 0 {
			out[o.Name[i+1:]] = true
		}
	}
	return out
}

// calledFrom returns the first name in names that the text calls, or "".
func calledFrom(text string, names map[string]bool) string {
	if text == "" || len(names) == 0 {
		return ""
	}
	for _, tok := range tokens(text) {
		if tok.call && names[tok.name] {
			return tok.name
		}
	}
	return ""
}

// usesCollation returns the first collation in colls the definition names after
// a COLLATE keyword, or "".
func usesCollation(def string, colls map[string]bool) string {
	if def == "" || len(colls) == 0 {
		return ""
	}
	toks := tokens(def)
	for i, tok := range toks {
		if !strings.EqualFold(tok.name, "COLLATE") || tok.quoted {
			continue
		}
		if i+1 < len(toks) && colls[toks[i+1].name] {
			return toks[i+1].name
		}
	}
	return ""
}

// token is one identifier of a deparsed SQL expression.
type token struct {
	name string
	// call is true when the identifier is immediately followed by "(", which
	// is how the catalog's deparser writes a function call and is not how it
	// writes a keyword (CHECK, USING and VALUES all keep their space).
	call bool
	// quoted is true when the identifier arrived inside double quotes, so a
	// column actually named "collate" is not read as the keyword.
	quoted bool
}

// tokens splits a deparsed expression into its identifiers, joining a qualified
// name into one token and skipping string literals entirely.
//
// It is not a SQL parser and does not need to be: every text it reads was
// written by pg_get_expr, pg_get_constraintdef or pg_get_indexdef, so the only
// constructs are identifiers, quoted identifiers, string literals, operators and
// punctuation.
func tokens(s string) []token {
	var out []token
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == '\'':
			i++
			for i < len(s) {
				if s[i] == '\'' {
					i++
					if i < len(s) && s[i] == '\'' {
						i++
						continue
					}
					break
				}
				i++
			}
		case c == '"' || isIdentStart(c):
			name, quoted, next := readQualified(s, i)
			i = next
			call := i < len(s) && s[i] == '('
			out = append(out, token{name: name, call: call, quoted: quoted})
		default:
			i++
		}
	}
	return out
}

// readQualified reads one identifier, or a dotted chain of them, and returns the
// last two parts joined — which is the "schema.name" shape Schema.NotRecreated
// carries — together with the offset after it.
func readQualified(s string, i int) (name string, quoted bool, next int) {
	var parts []string
	for {
		part, q, n := readIdent(s, i)
		if n == i {
			break
		}
		quoted = quoted || q
		parts = append(parts, part)
		i = n
		if i < len(s) && s[i] == '.' {
			i++
			continue
		}
		break
	}
	switch len(parts) {
	case 0:
		return "", false, i + 1
	case 1:
		return parts[0], quoted, i
	default:
		return parts[len(parts)-2] + "." + parts[len(parts)-1], quoted, i
	}
}

func readIdent(s string, i int) (name string, quoted bool, next int) {
	if i < len(s) && s[i] == '"' {
		i++
		var b strings.Builder
		for i < len(s) {
			if s[i] == '"' {
				i++
				if i < len(s) && s[i] == '"' {
					b.WriteByte('"')
					i++
					continue
				}
				break
			}
			b.WriteByte(s[i])
			i++
		}
		return b.String(), true, i
	}
	start := i
	for i < len(s) && isIdentPart(s[i]) {
		i++
	}
	return s[start:i], false, i
}

func isIdentStart(c byte) bool {
	return c == '_' || c >= 0x80 || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isIdentPart(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9') || c == '$'
}

// identifiers is tokens' names, used by the type ordering in ddl.go.
func identifiers(s string) []string {
	toks := tokens(s)
	out := make([]string, 0, len(toks))
	for _, t := range toks {
		out = append(out, t.name)
	}
	return out
}
