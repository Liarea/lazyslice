// SPDX-License-Identifier: Apache-2.0

package classify

import "strings"

// The Postgres text form of an array and of a composite, split back into the
// values inside it.
//
// Why this is here at all (tracker T-0103). scalars() flattens an array sample
// element-wise, which is what lets a text[] of addresses be decided email, and
// it only works when pgx hands back a slice — which pgx does only for an array
// type its map knows. The source pool runs in QueryExecModeExec and registers
// no user types (internal/pg forbids an AfterConnect hook on the source pool,
// T-0076), so an array of an *extension* type arrives as the single string
// "{a@b.test,c@d.test}", no validator matches it, and the column is decided
// `none` and copied. Plausible's monthly_reports.recipients — the list of
// addresses a site's report is emailed to — is a citext[] and is exactly that
// column, so this is a real leak surface on a real schema (THREAT_MODEL.md T1).
//
// The composite half is T-0094's: a record arrives as "(1234.50,GBP)" for the
// same reason, and the fields inside it are the only place a validator could
// find personal data.
//
// This is a reader, not a parser: it is liberal in what it accepts, because
// every caller is a *signal* and a literal it cannot read falls back to being
// one opaque value rather than to an error. Nothing here is used to write SQL
// and nothing here reaches a reason string.

// splitArrayLiteral splits the Postgres output form of an array into its
// non-NULL elements. A nested array is flattened, because ARCHITECTURE.md §4
// classifies an array on its element type however many dimensions it has.
//
// The forms it reads are the ones the server writes: an optional dimension
// prefix ("[0:1]={a,b}") for an array whose subscripts do not start at one,
// braces, comma separators, double-quoted elements with backslash escapes, and
// the unquoted word NULL for a NULL element. A quoted "NULL" is the four-letter
// string and is kept.
func splitArrayLiteral(s string) ([]string, bool) {
	s = strings.TrimSpace(s)
	// The dimension prefix, "[1:2][1:2]=", which precedes the braces.
	if strings.HasPrefix(s, "[") {
		i := strings.Index(s, "]=")
		if i < 0 {
			return nil, false
		}
		s = strings.TrimSpace(s[i+2:])
	}
	fields, ok := splitLiteral(s, '{', '}')
	if !ok {
		return nil, false
	}
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if !f.quoted && strings.EqualFold(strings.TrimSpace(f.text), "null") {
			continue
		}
		out = append(out, f.value())
	}
	return out, true
}

// splitCompositeLiteral splits the Postgres output form of a composite into its
// non-NULL fields. A NULL field is written as nothing at all between the
// separators — "(,GBP)" is a record whose first field is NULL — which is the
// one rule that differs from an array.
func splitCompositeLiteral(s string) ([]string, bool) {
	fields, ok := splitLiteral(strings.TrimSpace(s), '(', ')')
	if !ok {
		return nil, false
	}
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if !f.quoted && strings.TrimSpace(f.text) == "" {
			continue
		}
		out = append(out, f.value())
	}
	return out, true
}

// literalField is one field of a literal, with whether it arrived quoted. The
// flag is what tells the NULL marker from a value that spells it.
type literalField struct {
	text   string
	quoted bool
}

// value is the field as a validator should read it. An unquoted field carries
// no significant whitespace in the server's output form; a quoted one carries
// exactly what is inside the quotes.
func (f literalField) value() string {
	if f.quoted {
		return f.text
	}
	return strings.TrimSpace(f.text)
}

// splitLiteral walks one literal between the given delimiters and returns its
// fields. It reports false for anything that is not a literal of that shape:
// the delimiters must open and close, and a quote must be closed.
//
// Both escape conventions are accepted inside quotes — a backslash before any
// character, which is what an array element uses, and a doubled quote, which is
// what a composite field uses — because accepting the one the other form does
// not produce costs nothing and misreading a literal costs a signal.
//
// A nested literal is flattened: an inner "{" or "(" opens a new depth and its
// fields join the outer list. Postgres quotes a nested *composite* field, so
// that case arrives as one quoted field and is left whole; a multidimensional
// array is the case that actually nests.
func splitLiteral(s string, open, closer byte) ([]literalField, bool) {
	if len(s) < 2 || s[0] != open || s[len(s)-1] != closer {
		return nil, false
	}
	var (
		out      []literalField
		cur      strings.Builder
		depth    int
		inQuotes bool
		escaped  bool
		quoted   bool // the field being built arrived quoted
		started  bool // anything at all has been seen since the last separator
	)
	flush := func() {
		if started {
			out = append(out, literalField{text: cur.String(), quoted: quoted})
		}
		cur.Reset()
		quoted, started = false, false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case escaped:
			cur.WriteByte(c)
			started, escaped = true, false
		case inQuotes && c == '\\':
			escaped = true
		case inQuotes && c == '"':
			if i+1 < len(s) && s[i+1] == '"' {
				cur.WriteByte('"')
				i++
				continue
			}
			inQuotes = false
		case inQuotes:
			cur.WriteByte(c)
		case c == '"':
			inQuotes, quoted, started = true, true, true
		case c == open:
			depth++
		case c == closer:
			flush()
			depth--
			if depth < 0 {
				return nil, false
			}
		case c == ',':
			flush()
		default:
			cur.WriteByte(c)
			started = true
		}
	}
	if depth != 0 || inQuotes || escaped {
		return nil, false
	}
	return out, true
}
