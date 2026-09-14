// SPDX-License-Identifier: Apache-2.0

package verify

import (
	"errors"
	"strings"
)

// Reading the Postgres array output form, which is how an array of an element
// type the pool's map does not know arrives: the source pool registers no user
// types (T-0076), so a citext[] comes back as the single string
// "{a@b.test,c@d.test}" rather than as a []any (tracker T-0118, T-0129).
//
// internal/transform masks such a column element-wise — it parses the literal
// with the grammar in internal/transform/array.go, masks each element under the
// column's masker with h computed per element, and records one residual-filter
// entry per element under the column's empty path — so the residual scan has to
// read the target's literal the same way. Canonicalising the whole literal
// through textOf tests bytes transform never added: no hit, a green tick, and
// every address inside the braces shipped in cleartext, which is the masker
// failing open that ARCHITECTURE.md section 6 item 1 is the only control
// against (THREAT_MODEL.md T12).
//
// This is a copy of transform's reader, for the reason value.go's textOf is
// one: transform is another stage package and internal/CLAUDE.md forbids
// reaching into one. It keeps only the half this stage needs — the non-NULL
// elements, in order — and none of the writing half, because nothing here
// writes a value back. What it must keep is the *grammar*: a literal
// transform's parser accepts, this one accepts and splits the same way, and a
// literal transform's parser refuses, this one refuses. The two case lists are
// TestArrayLiteralElementsAreSpeltAsTransformSpellsThem and
// TestArrayLiteralRefusesWhatIsNotOne here, against
// TestArrayLiteralRoundTripsTheGrammar and TestArrayLiteralRefusesWhatIsNotOne
// in internal/transform/array_test.go; internal/verify/CLAUDE.md records the
// duplication and that the fix is a shared home for it rather than a third
// copy.
//
// A NULL element yields nothing, because transform left it NULL and recorded
// nothing for it, and neither does an empty element, because mask.Apply passes
// the empty value through under its own rule (value.go, canonicalOf).
//
// Nothing here puts an element into an error: an element is a production value
// and an error string is a way out of the process (THREAT_MODEL.md T4).

// errArrayLiteral is a value in an array column that is not the server's array
// output form. It names no part of the value.
var errArrayLiteral = errors.New(
	"the value of an array column is not a Postgres array literal, so its elements cannot be tested one by one")

// arrayLiteralElements is every non-NULL element of one array literal, in
// order, with a multidimensional array's inner arrays flattened: the residual
// filter carries one entry per element under one path, so position and nesting
// say nothing about which entry an element belongs to.
//
// The forms it accepts are the ones array_out writes, and they are
// internal/transform's: an optional dimension prefix ("[0:1]={a,b}"), braces,
// comma separators, nested braces, double-quoted elements with backslash
// escapes, and the unquoted word NULL for a NULL element. A quoted "NULL" is
// the four-letter string. Whitespace around an unquoted element is not part of
// it.
func arrayLiteralElements(s string) ([]string, error) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "[") {
		// The dimension prefix is dropped rather than kept: masking preserved
		// the element count, and nothing here writes the literal back.
		i := strings.Index(s, "]=")
		if i < 0 {
			return nil, errArrayLiteral
		}
		s = strings.TrimSpace(s[i+2:])
	}
	p := arrayLiteralParser{s: s}
	if !p.at('{') {
		return nil, errArrayLiteral
	}
	if err := p.array(); err != nil {
		return nil, err
	}
	p.spaces()
	if p.i != len(p.s) {
		return nil, errArrayLiteral
	}
	return p.out, nil
}

// arrayLiteralParser walks one literal, collecting its non-NULL elements.
type arrayLiteralParser struct {
	s   string
	i   int
	out []string
}

func (p *arrayLiteralParser) spaces() {
	for p.i < len(p.s) && isArrayLiteralSpace(p.s[p.i]) {
		p.i++
	}
}

// at reports that the next non-space byte is c.
func (p *arrayLiteralParser) at(c byte) bool {
	p.spaces()
	return p.i < len(p.s) && p.s[p.i] == c
}

// array parses one "{...}", the cursor sitting on the brace.
func (p *arrayLiteralParser) array() error {
	p.i++ // the '{'
	if p.at('}') {
		p.i++
		return nil
	}
	for {
		var err error
		if p.at('{') {
			err = p.array()
		} else {
			err = p.element()
		}
		if err != nil {
			return err
		}
		switch {
		case p.at(','):
			p.i++
		case p.at('}'):
			p.i++
			return nil
		default:
			// Either the literal ended inside the braces or a byte no
			// separator can be sits between two elements.
			return errArrayLiteral
		}
	}
}

// element parses one value: quoted, or a bare run up to the next separator.
func (p *arrayLiteralParser) element() error {
	p.spaces()
	if p.i < len(p.s) && p.s[p.i] == '"' {
		return p.quoted()
	}
	start := p.i
	for p.i < len(p.s) {
		switch p.s[p.i] {
		case ',', '}', '{', '"':
			return p.bare(p.s[start:p.i])
		case '\\':
			// array_out never writes a backslash outside quotes, and array_in
			// would read it as an escape; a literal carrying one is not the
			// server's output form.
			return errArrayLiteral
		default:
			p.i++
		}
	}
	return errArrayLiteral
}

// bare takes an unquoted element. An unquoted element with nothing in it
// ("{,}", "{a,}") is not a form array_out writes — the empty string is written
// quoted — so it is refused rather than read as an empty element, which is what
// internal/transform does with it: reading it as one would split the literal
// into a different number of elements than the masker did.
func (p *arrayLiteralParser) bare(raw string) error {
	text := strings.Trim(raw, arrayLiteralSpace)
	switch {
	case text == "":
		return errArrayLiteral
	case strings.EqualFold(text, "null"):
		return nil
	}
	p.out = append(p.out, text)
	return nil
}

// quoted parses a double-quoted element, the cursor sitting on the opening
// quote. Inside quotes a backslash escapes the byte after it, which is how
// array_out writes a quote, a backslash, a brace, a comma or leading
// whitespace.
func (p *arrayLiteralParser) quoted() error {
	p.i++ // the opening quote
	var b strings.Builder
	for p.i < len(p.s) {
		switch c := p.s[p.i]; c {
		case '\\':
			if p.i+1 >= len(p.s) {
				return errArrayLiteral
			}
			b.WriteByte(p.s[p.i+1])
			p.i += 2
		case '"':
			p.i++
			p.out = append(p.out, b.String())
			return nil
		default:
			b.WriteByte(c)
			p.i++
		}
	}
	return errArrayLiteral
}

// arrayLiteralSpace is array_isspace: the bytes array_in skips around an
// element.
const arrayLiteralSpace = " \t\n\r\v\f"

func isArrayLiteralSpace(c byte) bool { return strings.IndexByte(arrayLiteralSpace, c) >= 0 }
