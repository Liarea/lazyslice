// SPDX-License-Identifier: Apache-2.0

package pipeline

import "strings"

// The string literals inside a deparsed SQL expression, and the rewrite of
// them (ARCHITECTURE.md section 11.1, amended 2026-09-14).
//
// A column default, a CHECK constraint and a generated-column expression are
// recreated in the target as the catalog's own text, so a literal inside one of
// them crosses the source/target boundary exactly as a row value does and is
// inside the data boundary for the same reason. The 2026-09-09 review put
// DEFAULT 'ddl.canary@example.org' on a masked email column, watched the rows
// be masked and the default survive into pg_attrdef, and exited 0: a later
// INSERT using that default materialises the original value again (finding 5,
// docs/reviews/2026-09-09/evidence/ddl_default.log).
//
// Two stage packages need to read those literals -- internal/plan, which
// refuses or rewrites before anything is dropped, and internal/verify, which
// reads the target's own pg_attrdef and pg_constraint back after the load --
// and a stage package may not import another one (internal/CLAUDE.md). A second
// copy of a scanner that decides what is and is not inside the data boundary is
// the failure mode internal/verify/validators.go already records from its own
// hand copy of the classifier's validators, so the scanner lives here, in the
// package both already import.
//
// It is a reader, not a SQL parser, and it does not need to be one: every text
// it is given was written by pg_get_expr, pg_get_constraintdef or
// pg_get_indexdef, so the only constructs are identifiers, quoted identifiers,
// string constants, operators, comments and punctuation. Where it is unsure it
// reports the literal as *not rewritable*, which is the direction that refuses
// rather than the direction that writes a default nobody checked.
//
// This is not one of the types ARCHITECTURE.md section 2 declares. The
// deviation is recorded in internal/plan/CLAUDE.md: section 2 says every type
// in it lives here and nowhere else, not that nothing else may.

// Literal is one string constant inside a deparsed SQL expression.
type Literal struct {
	// Text is the constant's value with its quoting removed: the value the
	// server would produce, for the plain '...' form, and the raw bytes between
	// the delimiters for every other form (see Rewritable).
	Text string
	// Start and End are the byte range of the whole constant in the expression,
	// delimiters included, so Expr[Start:End] is the text RewriteLiterals
	// replaces.
	Start, End int
	// Rewritable is true only for the plain '...' form, which is what
	// pg_get_expr writes for an ordinary text constant and the only form whose
	// value this package can reproduce exactly. An E'...' escape string, a
	// U&'...' unicode string and a $tag$...$tag$ dollar-quoted string are
	// reported so that a validator can look inside them, and are never
	// rewritten: their Text is the raw body, backslash escapes and all, so a
	// replacement built from it could change the value in ways nothing here
	// checks. A caller that must rewrite one refuses instead.
	Rewritable bool
	// Pattern is true when the literal is the right-hand operand of a
	// pattern-matching operator: LIKE, ILIKE, SIMILAR TO, their deparsed
	// spellings (~~, ~~*, !~~, !~~*) and the regular-expression operators (~,
	// ~*, !~, !~*). Such a literal is a *pattern* and not a value, and a caller
	// that runs a value validator over one gets nonsense: testdata/nasty.sql's
	// CHECK ("EmailAddress" LIKE '%@%.%') parses as a valid address under
	// net/mail, because % is an ordinary atext character, so the whole fixture
	// refused at exit 12 the first time this rule ran (T-0134). Rewriting one
	// would be worse -- it changes what the database accepts.
	Pattern bool
}

// Literals returns every string constant in a deparsed SQL expression, in the
// order they appear. Quoted identifiers and comments are skipped; a constant
// that is not terminated is not returned at all, because a half-read literal is
// a text this scanner does not understand.
func Literals(expr string) []Literal {
	var out []Literal
	scanLiterals(expr, func(l Literal) { out = append(out, l) })
	return out
}

// RewriteLiterals rebuilds an expression with each literal replaced by what f
// returns for it. f reports false to leave a literal alone.
//
// The second return is false when a literal f asked to replace is not
// Rewritable; in that case the expression comes back unchanged, because a
// partial rewrite of an expression is a predicate nobody wrote.
func RewriteLiterals(expr string, f func(Literal) (string, bool)) (string, bool) {
	var b strings.Builder
	at := 0
	ok := true
	scanLiterals(expr, func(l Literal) {
		if !ok {
			return
		}
		replacement, want := f(l)
		if !want {
			return
		}
		if !l.Rewritable {
			ok = false
			return
		}
		b.WriteString(expr[at:l.Start])
		b.WriteString(QuoteLiteral(replacement))
		at = l.End
	})
	if !ok {
		return expr, false
	}
	b.WriteString(expr[at:])
	return b.String(), true
}

// QuoteLiteral renders a string as a plain SQL constant, doubling the quotes
// inside it. It is deliberately the same rule internal/load/ddl's own
// quoteLiteral uses; a value that would need an E” string cannot arrive here,
// because every replacement is a masker's output and no generator emits a
// backslash or a control character.
func QuoteLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// patternMetaChars is every character StripPatternMeta discards on its own,
// unescaped, because a pattern operator (LIKE, ILIKE, SIMILAR TO, ~, ~*, !~,
// !~*) reads it as a wildcard, an anchor or a grouping construct rather than
// as a character of the value: LIKE's own % and _, and the basic regular
// expression metacharacters SIMILAR TO and the tilde operators share with
// POSIX/PCRE (^ $ . * + ? [ ] ( ) { } |). The backslash is in the set too, so
// that a lone trailing backslash — a malformed escape, not one this scanner
// can resolve — is discarded rather than kept as a stray character; an
// ordinary backslash-escaped metacharacter never reaches this set at all,
// because StripPatternMeta consumes the pair before checking membership.
const patternMetaChars = `^$%_\.*+?[](){}|`

// StripPatternMeta is ARCHITECTURE.md §11.1's 2026-09-15 amendment (the T-0189
// red team, R2-10): the text left over from a pattern-operand literal once its
// syntax is removed, so a validator can still be run over the *value* hiding
// inside a *pattern*.
//
// Pattern carries a real distinction -- '%@%.%' is a shape and not a value,
// and net/mail parses it as one anyway (testdata/nasty.sql's own trap, which
// is why the Pattern field exists at all, T-0134) -- but "not a value" and
// "exempt from every validator that would otherwise catch the value inside
// it" are not the same rule, and the second is what the 2026-09-15 red team's
// R2-10 found: `CHECK (email !~ '^ceo@bigcorp\.example$')` carries the exact
// address `ceo@bigcorp.example` and crossed into the target under exit 0,
// beside a `CHECK (email <> 'ceo@bigcorp.example')` on the same value that
// refused at exit 13 — the semantically identical predicate written as a
// pattern was the one route through.
//
// A caller runs this over a Pattern literal's Text before validating it, never
// over one that RewriteLiterals would touch: what this returns is a lossy
// projection built for detection only, and a caller that tried to write it
// back would change what the database accepts. That is Pattern's other half,
// unchanged: it exempts a literal from rewriting and nothing else.
//
// The reduction is one pass: a backslash immediately followed by another
// character is an escape, and the escaped character is kept literally rather
// than discarded twice over — '\\.' is an escaped dot, one literal ".",  not
// "backslash, removed; dot, removed; nothing left". Every other occurrence of
// a character in patternMetaChars, including a lone trailing backslash, is
// discarded outright. So '%@%.%' (LIKE's own wildcard, unescaped) reduces to
// "@" -- which no validator matches -- and '^ceo@bigcorp\.example$' (the
// red team's own regex) reduces to "ceo@bigcorp.example" -- the address,
// intact, because its one metacharacter was an *escaped* literal dot and the
// anchors around it carried no value at all.
func StripPatternMeta(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\\' && i+1 < len(s) {
			b.WriteByte(s[i+1])
			i++
			continue
		}
		if strings.IndexByte(patternMetaChars, c) >= 0 {
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

// nonTextCastTypes is the type names CastToNonText answers true for (T-0198
// fix round, finding 3): a literal immediately cast to one of these can never
// hold free text, whatever category a name or a shape signal would otherwise
// guess at. Deliberately narrow and deliberately the bare, unschemaed spelling
// pg_get_expr and pg_get_constraintdef write -- a text-compatible cast
// (::text, ::varchar, ::citext, ::json, ...) is not on it, and neither is a
// composite, domain or array type this scanner has no catalog to resolve;
// "double" stands in for `double precision`, because the cast is two words
// and this function only ever reads the first identifier-shaped token after
// `::`.
var nonTextCastTypes = map[string]bool{
	"int": true, "int2": true, "int4": true, "int8": true,
	"smallint": true, "integer": true, "bigint": true,
	"numeric": true, "decimal": true,
	"real": true, "float4": true, "float8": true, "double": true,
	"boolean": true, "bool": true,
}

// CastToNonText reports whether the literal at l is immediately followed, in
// expr, by an explicit `::type` cast to one of nonTextCastTypes.
//
// pg_get_expr and pg_get_constraintdef always quote a scalar constant and
// cast it directly afterwards, with no space the deparser ever omits:
// `COALESCE(user_id, '-1'::integer)` is odoo's own shape
// (public.ir_filters), and discourse's `COALESCE(parent_category_id,
// '-1'::integer)` is the identical one on a different column -- a sentinel
// this scanner had been reading as a plain string literal, because
// pg_get_expr writes the integer constant -1 quoted before casting it, and
// unrewritableLiteral (internal/plan/ddlliteral.go, internal/verify/
// catalog.go) then saw a non-empty, non-pattern, non-closed-list literal and
// refused a masked column's index on it under T-0198's broadened rule. A
// literal cast to integer, bigint, numeric or boolean immediately afterwards
// is not a value a person could be in, on the identical "shape, not
// category" footing the empty-collection and closed-value-list exemptions
// already stand on in both callers.
func CastToNonText(expr string, l Literal) bool {
	i := l.End
	for i < len(expr) && isSpaceByte(expr[i]) {
		i++
	}
	if i+1 >= len(expr) || expr[i] != ':' || expr[i+1] != ':' {
		return false
	}
	i += 2
	for i < len(expr) && isSpaceByte(expr[i]) {
		i++
	}
	j := i
	for j < len(expr) && isNameByte(expr[j]) {
		j++
	}
	name := expr[i:j]
	if k := strings.LastIndexByte(name, '.'); k >= 0 {
		name = name[k+1:]
	}
	return nonTextCastTypes[strings.ToLower(name)]
}

// scanLiterals walks the expression once and calls f for each string constant.
func scanLiterals(expr string, f func(Literal)) {
	for i := 0; i < len(expr); {
		switch {
		case expr[i] == '\'':
			i = plainString(expr, i, f)
		case isEscapePrefix(expr, i):
			i = prefixedString(expr, i, i+1, false, f)
		case isUnicodePrefix(expr, i):
			i = prefixedString(expr, i, i+2, true, f)
		case expr[i] == '"':
			i = quotedIdent(expr, i)
		case expr[i] == '$':
			i = dollarString(expr, i, f)
		case strings.HasPrefix(expr[i:], "--"):
			if j := strings.IndexByte(expr[i:], '\n'); j >= 0 {
				i += j + 1
			} else {
				i = len(expr)
			}
		case strings.HasPrefix(expr[i:], "/*"):
			if j := strings.Index(expr[i+2:], "*/"); j >= 0 {
				i += 2 + j + 2
			} else {
				i = len(expr)
			}
		default:
			i++
		}
	}
}

// isEscapePrefix reports whether an E” escape string starts at i. The letter
// must not be part of a longer identifier: "value" ends in an e and is not a
// prefix.
func isEscapePrefix(expr string, i int) bool {
	if expr[i] != 'E' && expr[i] != 'e' {
		return false
	}
	if i+1 >= len(expr) || expr[i+1] != '\'' {
		return false
	}
	return i == 0 || !isNameByte(expr[i-1])
}

// isUnicodePrefix reports whether a U&” unicode string starts at i.
func isUnicodePrefix(expr string, i int) bool {
	if expr[i] != 'U' && expr[i] != 'u' {
		return false
	}
	if i+2 >= len(expr) || expr[i+1] != '&' || expr[i+2] != '\'' {
		return false
	}
	return i == 0 || !isNameByte(expr[i-1])
}

func isNameByte(c byte) bool {
	return c == '_' || c == '$' || c >= 0x80 ||
		(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// plainString reads '...' with ” as the escape, which is the form pg_get_expr
// writes for an ordinary text constant, and is the only rewritable one.
func plainString(expr string, start int, f func(Literal)) int {
	var b strings.Builder
	i := start + 1
	for i < len(expr) {
		if expr[i] != '\'' {
			b.WriteByte(expr[i])
			i++
			continue
		}
		i++
		if i < len(expr) && expr[i] == '\'' {
			b.WriteByte('\'')
			i++
			continue
		}
		f(Literal{
			Text: b.String(), Start: start, End: i, Rewritable: true,
			Pattern: afterPatternOperator(expr, start),
		})
		return i
	}
	// Unterminated: report nothing and stop, rather than inventing a value.
	return len(expr)
}

// prefixedString reads E'...' and U&'...'. Neither is rewritable: the body is
// reported raw so that a validator can look inside it, and a replacement built
// from that body could change the value.
func prefixedString(expr string, start, quote int, unicodeEscape bool, f func(Literal)) int {
	i := quote + 1
	for i < len(expr) {
		switch {
		case !unicodeEscape && expr[i] == '\\' && i+1 < len(expr):
			i += 2
		case expr[i] == '\'':
			if i+1 < len(expr) && expr[i+1] == '\'' {
				i += 2
				continue
			}
			f(Literal{
				Text: expr[quote+1 : i], Start: start, End: i + 1,
				Pattern: afterPatternOperator(expr, start),
			})
			return i + 1
		default:
			i++
		}
	}
	return len(expr)
}

// dollarString reads $tag$...$tag$, and returns start+1 for a dollar that does
// not open one (a positional parameter, or an identifier's dollar).
func dollarString(expr string, start int, f func(Literal)) int {
	end := -1
	for j := start + 1; j < len(expr); j++ {
		if expr[j] == '$' {
			end = j
			break
		}
		if !isTagByte(expr[j]) {
			return start + 1
		}
	}
	if end < 0 {
		return start + 1
	}
	tag := expr[start : end+1]
	rest := expr[end+1:]
	j := strings.Index(rest, tag)
	if j < 0 {
		return len(expr)
	}
	f(Literal{
		Text: rest[:j], Start: start, End: end + 1 + j + len(tag),
		Pattern: afterPatternOperator(expr, start),
	})
	return end + 1 + j + len(tag)
}

func isTagByte(c byte) bool {
	return c == '_' || c >= 0x80 || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// quotedIdent skips "..." with "" as the escape.
func quotedIdent(expr string, start int) int {
	i := start + 1
	for i < len(expr) {
		if expr[i] != '"' {
			i++
			continue
		}
		i++
		if i < len(expr) && expr[i] == '"' {
			i++
			continue
		}
		return i
	}
	return len(expr)
}

// afterPatternOperator reports whether the token immediately before a literal is
// a pattern-matching operator, in either the keyword spelling a hand-written
// constraint uses or the operator spelling pg_get_constraintdef deparses it to.
//
// It looks back over whitespace and then at one token: an operator made of
// ~, ! and * carrying a tilde, or the word LIKE or ILIKE, or the word TO
// preceded by SIMILAR, or the literal sitting as the first argument of a call
// to similar_escape(. A NOT before LIKE does not matter -- what is being asked
// is whether the literal is a pattern, not whether the predicate is negated.
//
// similar_escape is deparser output, never something a hand-written CHECK or
// index predicate calls directly: pg_get_expr and pg_get_constraintdef render
// every SIMILAR TO (and NOT SIMILAR TO) as `col OPERATOR(pg_catalog.~)
// similar_escape('pattern', escape)`, not as the SIMILAR TO keyword form the
// operator branch above already reads (T-0198, found on gitlab's
// `index_issues_on_description_trigram_non_latin`: a partial index predicate
// carrying `title !~ similar_escape('[<a Unicode codepoint range>]*'::text,
// NULL::text)`, a character-class shape and not a value, went undetected as a
// pattern and refused a masked column's index outright under T-0198's
// broadened rule). The escape argument is not a pattern in the same sense --
// it is one literal character or NULL -- but this scanner only ever sees
// similar_escape's *first* argument here, because the escape argument is
// never the literal this function is asked about at its own call site
// (RewriteLiterals still declines every Pattern literal from rewriting
// regardless, so treating both arguments the same would cost nothing even if
// it were reached).
func afterPatternOperator(expr string, start int) bool {
	i := start
	for i > 0 && isSpaceByte(expr[i-1]) {
		i--
	}
	if i == 0 {
		return false
	}
	if expr[i-1] == '(' {
		j := i - 1
		for j > 0 && isSpaceByte(expr[j-1]) {
			j--
		}
		k := j
		for k > 0 && isNameByte(expr[k-1]) {
			k--
		}
		if strings.EqualFold(expr[k:j], "similar_escape") {
			return true
		}
	}
	if isOperatorByte(expr[i-1]) {
		j := i
		for j > 0 && isOperatorByte(expr[j-1]) {
			j--
		}
		return strings.Contains(expr[j:i], "~")
	}
	j := i
	for j > 0 && isNameByte(expr[j-1]) {
		j--
	}
	switch strings.ToUpper(expr[j:i]) {
	case "LIKE", "ILIKE":
		return true
	case "TO":
		for j > 0 && isSpaceByte(expr[j-1]) {
			j--
		}
		k := j
		for k > 0 && isNameByte(expr[k-1]) {
			k--
		}
		return strings.EqualFold(expr[k:j], "SIMILAR")
	}
	return false
}

func isSpaceByte(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

// isOperatorByte is the subset of Postgres's operator characters a
// pattern-matching operator is built from.
func isOperatorByte(c byte) bool { return c == '~' || c == '!' || c == '*' }
