// SPDX-License-Identifier: Apache-2.0

package transform

import (
	"errors"
	"strings"
)

// The Postgres array output form, read and written back, for the arrays that do
// not arrive as a slice (tracker T-0118).
//
// maskArray masks a []any, which is what pgx hands back for an array type its
// map knows. The source pool registers no user types (T-0076), so an array of an
// *extension* type — a citext[] — arrives as the single string
// "{a@b.test,c@d.test}". Before this file such a value fell through to the
// scalar path: it was masked as one string and CopyFrom then refused the result
// for an _citext column ("cannot find encode plan") at exit 7 with rows already
// moving, which is why internal/plan refused the column at plan time instead.
// internal/classify reads inside the same literal (T-0103,
// internal/classify/literal.go) so that the addresses inside one are seen by the
// validators; this is the write half of that, and the two have to agree about
// what an element is.
//
// Unlike classify's reader, this is a parser and not a signal: what it produces
// is written back into the target, so a literal it cannot read is a refusal
// (fail closed) and never a value copied through, and the form it re-emits has
// to be one array_in accepts for the same element count and the same
// dimensions. It therefore keeps the nesting and the dimension prefix that
// classify's reader is free to flatten and drop.
//
// Nothing here formats an element into an error string: an element is a
// production value and an error string is a way out of the process
// (THREAT_MODEL.md T4).

// arrayNode is one node of a parsed array literal: either a nested array, or a
// single element, which is either NULL or a string.
//
// The nesting is kept because a multidimensional array's literal is only valid
// when every inner array has the same length ("{{a,b},{c,d}}"), and flattening
// it here would re-emit a literal array_in rejects.
type arrayNode struct {
	elems  []arrayNode
	nested bool
	null   bool
	text   string
}

// errArrayLiteral is what a value in an array column that is not the server's
// array output form comes back as. It names no part of the value.
var errArrayLiteral = errors.New("the value of an array column is not a Postgres array literal, so its elements cannot be masked one by one")

// parseArrayLiteral reads the server's output form of an array.
//
// The forms it accepts are the ones array_out writes: an optional dimension
// prefix ("[0:1]={a,b}") for an array whose subscripts do not start at one,
// braces, comma separators, nested braces for a multidimensional array,
// double-quoted elements with backslash escapes, and the unquoted word NULL for
// a NULL element. A quoted "NULL" is the four-letter string and stays one.
// Whitespace around an unquoted element is not part of it, which is what
// array_in does with it.
//
// The prefix is returned verbatim so that it can be written back: masking
// preserves the element count, so the bounds it declares stay true.
func parseArrayLiteral(s string) (prefix string, root arrayNode, err error) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "[") {
		i := strings.Index(s, "]=")
		if i < 0 {
			return "", arrayNode{}, errArrayLiteral
		}
		prefix, s = s[:i+2], strings.TrimSpace(s[i+2:])
	}
	p := arrayParser{s: s}
	if !p.at('{') {
		return "", arrayNode{}, errArrayLiteral
	}
	root, err = p.array()
	if err != nil {
		return "", arrayNode{}, err
	}
	p.spaces()
	if p.i != len(p.s) {
		return "", arrayNode{}, errArrayLiteral
	}
	return prefix, root, nil
}

// arrayParser walks one literal.
type arrayParser struct {
	s string
	i int
}

func (p *arrayParser) spaces() {
	for p.i < len(p.s) && isArraySpace(p.s[p.i]) {
		p.i++
	}
}

// at reports that the next non-space byte is c.
func (p *arrayParser) at(c byte) bool {
	p.spaces()
	return p.i < len(p.s) && p.s[p.i] == c
}

// array parses one "{...}", the cursor sitting on the brace.
func (p *arrayParser) array() (arrayNode, error) {
	node := arrayNode{nested: true, elems: []arrayNode{}}
	p.i++ // the '{'
	if p.at('}') {
		p.i++
		return node, nil
	}
	for {
		var (
			child arrayNode
			err   error
		)
		switch {
		case p.at('{'):
			child, err = p.array()
		default:
			child, err = p.element()
		}
		if err != nil {
			return arrayNode{}, err
		}
		node.elems = append(node.elems, child)
		switch {
		case p.at(','):
			p.i++
		case p.at('}'):
			p.i++
			return node, nil
		default:
			// Either the literal ended inside the braces or a byte no
			// separator can be sits between two elements.
			return arrayNode{}, errArrayLiteral
		}
	}
}

// element parses one value: quoted, or a bare run up to the next separator.
func (p *arrayParser) element() (arrayNode, error) {
	p.spaces()
	if p.i < len(p.s) && p.s[p.i] == '"' {
		return p.quoted()
	}
	start := p.i
	for p.i < len(p.s) {
		switch c := p.s[p.i]; c {
		case ',', '}', '{', '"':
			return bareElement(p.s[start:p.i])
		case '\\':
			// array_out never writes a backslash outside quotes, and array_in
			// would read it as an escape; a literal carrying one is not the
			// server's output form.
			return arrayNode{}, errArrayLiteral
		default:
			p.i++
		}
	}
	return arrayNode{}, errArrayLiteral
}

// bareElement is an unquoted element: whitespace around it is not part of it,
// and the unquoted word NULL is the NULL element rather than a four-letter
// string.
//
// An unquoted element with nothing in it ("{,}", "{a,}") is not a form array_out
// writes — the empty string is written quoted — so it is refused rather than
// read as an empty element. Guessing would change the array's length, and the
// length is the one thing the loader cannot check.
func bareElement(raw string) (arrayNode, error) {
	text := strings.Trim(raw, arraySpace)
	switch {
	case text == "":
		return arrayNode{}, errArrayLiteral
	case strings.EqualFold(text, "null"):
		return arrayNode{null: true}, nil
	}
	return arrayNode{text: text}, nil
}

// quoted parses a double-quoted element, the cursor sitting on the opening
// quote. Inside quotes a backslash escapes the byte after it, which is how
// array_out writes a quote, a backslash, a brace, a comma or leading
// whitespace.
func (p *arrayParser) quoted() (arrayNode, error) {
	p.i++ // the opening quote
	var b strings.Builder
	for p.i < len(p.s) {
		switch c := p.s[p.i]; c {
		case '\\':
			if p.i+1 >= len(p.s) {
				return arrayNode{}, errArrayLiteral
			}
			b.WriteByte(p.s[p.i+1])
			p.i += 2
		case '"':
			p.i++
			return arrayNode{text: b.String()}, nil
		default:
			b.WriteByte(c)
			p.i++
		}
	}
	return arrayNode{}, errArrayLiteral
}

// renderArrayLiteral writes the literal back in the form array_in reads, with
// the dimension prefix it arrived with.
func renderArrayLiteral(prefix string, root arrayNode) string {
	var b strings.Builder
	b.WriteString(prefix)
	writeArrayNode(&b, root)
	return b.String()
}

func writeArrayNode(b *strings.Builder, n arrayNode) {
	if n.nested {
		b.WriteByte('{')
		for i := range n.elems {
			if i > 0 {
				b.WriteByte(',')
			}
			writeArrayNode(b, n.elems[i])
		}
		b.WriteByte('}')
		return
	}
	if n.null {
		b.WriteString("NULL")
		return
	}
	b.WriteString(quoteArrayElement(n.text))
}

// quoteArrayElement quotes an element exactly when array_out would: when it is
// empty, when it spells NULL in any case, and when it holds a byte that would
// otherwise be read as syntax — a brace, the comma delimiter, a quote, a
// backslash or whitespace.
func quoteArrayElement(s string) string {
	if !needsArrayQuotes(s) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		if c := s[i]; c == '"' || c == '\\' {
			b.WriteByte('\\')
		}
		b.WriteByte(s[i])
	}
	b.WriteByte('"')
	return b.String()
}

func needsArrayQuotes(s string) bool {
	if s == "" || strings.EqualFold(s, "null") {
		return true
	}
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '{', c == '}', c == ',', c == '"', c == '\\':
			return true
		case isArraySpace(c):
			return true
		}
	}
	return false
}

// arraySpace is array_isspace: the bytes array_in skips around an element.
const arraySpace = " \t\n\r\v\f"

func isArraySpace(c byte) bool { return strings.IndexByte(arraySpace, c) >= 0 }
