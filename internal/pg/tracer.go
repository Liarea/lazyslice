// SPDX-License-Identifier: Apache-2.0

package pg

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Liarea/lazyslice/internal/pipeline"
)

// Shape is one entry in the source statement allowlist. Every statement the run
// sends to the source must match a registered shape by name; anything else is
// refused by the tracer before it reaches the server.
//
// SQL is a template, not a prefix and not a keyword. A first-keyword allowlist
// was rejected because `WITH x AS (DELETE ... RETURNING *) SELECT ...` and
// `SELECT lo_import(...)` both begin with an allowed keyword (THREAT_MODEL.md
// T9); a template fixes the whole shape of the statement, so the only freedom a
// caller has is the freedom the template names.
//
// The template is matched against the statement with runs of whitespace
// collapsed and letter case ignored, and it may carry these placeholders and no
// others:
//
//	{ident}       one identifier, optionally schema-qualified, quoted or not
//	{idents}      a comma-separated list of one or more {ident}
//	{int}         a non-negative integer literal
//	{snapshot}    a quoted snapshot identifier as pg_export_snapshot returns it
//	{selectlist}  a comma-separated list of select-list items, each an {ident}
//	              with an optional ::type cast and an optional AS alias
//	              (`t."a"::text AS o1`)
//	{casts}       a comma-separated list of typed array parameters, the
//	              variable-arity argument list of a chunk join
//	              (`$1::int8[], $2::text[]`)
//	{keypred}     a conjunction of key terms, each `a.b = c.d` with an optional
//	              cast on the right, or `a.b IS NOT NULL`
//	{where}       the operator's own --where predicate
//
// The last four exist because the planner's statements are built per table and
// per key arity (ARCHITECTURE.md §3, internal/plan/shapes.go) and extract's are
// built the same way (§2 "extract"), and none of the first four can express
// them. Each is still a structure and not an escape hatch: {selectlist},
// {casts} and {keypred} admit identifiers, casts and equality — no value, no
// function call, no subquery, no statement separator; the only parenthesis any
// of them admits is a type modifier's, holding digits, and the only words a
// cast may carry are the closed list of type-name continuations below — so none
// of them can spell a write. The closed list is load-bearing and was learned
// the hard way: while the continuation was `[a-z]+`, `t."a"::int INTO evil`
// parsed as a select-list item and `SELECT ... INTO evil ...` is CREATE TABLE
// AS (reTypeWord).
//
// {where} is the one placeholder whose text a person outside this program
// wrote, so it is the one with an explicit exclusion list: no `;`, no `--`, no
// `/*`, no `\`, no `$`, and parentheses that balance within the predicate to a
// bounded depth. `1=1) --` would comment out the template's own
// `ORDER BY ... LIMIT n` while still matching the shape, which is how a bounded
// seed read (THREAT_MODEL.md T11) becomes an unbounded one inside the holder
// transaction (T9); `;` would end the statement and start another; an
// unbalanced `)` would close the template's own parenthesis and append whole
// clauses (`id > 0) UNION ALL SELECT c."pan" ... WHERE (true`) with every
// parenthesis in the statement still paired. A `;`, `(` or `)` inside a string
// literal in a predicate is refused with the rest — the loud refusal costs an
// operator a rephrasing, and telling a literal from structure costs a SQL
// lexer. The `\` and `$`
// exclusions are the two forms elideLiterals cannot close over (see below), so
// a statement that matches a shape is always one whose trace is a statement
// rather than a leading keyword and `<elided>`.
//
// What the exclusions buy is exactly this and no more: one statement, no
// commented-out tail, clauses that cannot be appended to the template's own,
// and a faithful trace. They do not make a predicate safe. A balanced predicate
// is still arbitrary SQL — a function call (`id = lo_import('/etc/passwd')`, and
// a `dblink(...)` that opens its own connection), a subquery, an unbounded
// aggregate — because it is the operator's own SQL over their own source, and
// what bounds it is the READ ONLY transaction and nothing here. The predicates
// that are and are not admitted are pinned by name in
// internal/plan/shapes_test.go, including the ones admitted on purpose; the
// rephrasing an exclusion costs an operator is refused at --where instead, by
// internal/plan/where.go, so a violation recorded here always means a bug in
// our own SQL generation.
//
// Case is ignored outside a quoted identifier (see the paragraph below for
// inside one) because lazyslice generates every one of these statements and
// the templates fix their structure; ignoring case widens what our own SQL may
// look like, and widens nothing else — no case of `SELECT ... FROM {ident}`
// spells a `DELETE`. A later stage that needs a shape this grammar cannot
// express adds a placeholder here, where it is reviewed once, rather than
// registering a looser template.
//
// A quoted identifier inside a template is fixed text and is never scanned for
// placeholders (templateSegments): the per-table shapes internal/extract and
// internal/verify build carry a quoted table name, an identifier is arbitrary
// text, and a table called `{ident}` must not turn the shape that names one
// table into the shape that admits every relation. It is also the one part of a
// template matched case-sensitively and space-for-space, because that is how
// Postgres reads a quoted name: `"LegacyCustomer"` and `"legacycustomer"` are
// two tables, and `"my table"` is not `"mytable"` (compileShape).
type Shape struct {
	Name string
	// SQL is the statement template with parameters as placeholders. It is what
	// the trace records, so it never holds a value.
	SQL string
	// Added is when the shape was registered, for the trace.
	Added time.Time
}

// ErrRefused is the error a refused statement fails with. The tracer itself
// refuses by returning a cancelled context, so the error the caller sees comes
// from pgx; this is what Source.Trace's Refused entries mean, and what
// Tracer.Violation returns.
var ErrRefused = errors.New("pg: statement does not match any registered shape on the source")

// Tracer is the source statement allowlist. It implements all five pgx tracers
// (QueryTracer, BatchTracer, CopyFromTracer, PrepareTracer, ConnectTracer) and
// is registered on the source pool and on no other pool.
//
// A statement whose shape is not registered gets a cancelled context from
// TraceQueryStart, which pgconn checks before it writes anything to the wire,
// and a recorded violation that fails the run (THREAT_MODEL.md T9). CopyFrom,
// SendBatch and Prepare are refused unconditionally: the source uses Query and
// Exec only, and a call to any of the three is a bug in this package, not a
// statement to be matched.
type Tracer struct {
	mu         sync.Mutex
	shapes     []compiledShape
	trace      []pipeline.TracedStatement
	violations int
}

type compiledShape struct {
	name string
	re   *regexp.Regexp
}

var _ interface {
	pgx.QueryTracer
	pgx.BatchTracer
	pgx.CopyFromTracer
	pgx.PrepareTracer
	pgx.ConnectTracer
} = (*Tracer)(nil)

// NewTracer compiles the allowlist. It returns an error rather than panicking
// on a bad template so that a shape added by a later stage fails the build's
// tests rather than the run.
func NewTracer(shapes ...Shape) (*Tracer, error) {
	t := &Tracer{}
	if err := t.Register(shapes...); err != nil {
		return nil, err
	}
	return t, nil
}

// Register adds shapes to the allowlist. It is additive: a stage registers the
// statements it is about to issue, and nothing removes a shape, so the trace
// and the allowlist tell the same story at the end of the run.
func (t *Tracer) Register(shapes ...Shape) error {
	compiled := make([]compiledShape, 0, len(shapes))
	for _, s := range shapes {
		if s.Name == "" {
			return errors.New("pg: a shape must be named, because the trace records the name")
		}
		re, err := compileShape(s.SQL)
		if err != nil {
			return fmt.Errorf("pg: shape %q: %w", s.Name, err)
		}
		compiled = append(compiled, compiledShape{name: s.Name, re: re})
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	t.shapes = append(t.shapes, compiled...)
	return nil
}

// Trace returns every statement the allowlist saw, in order, for invariant I4.
func (t *Tracer) Trace() []pipeline.TracedStatement {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]pipeline.TracedStatement, len(t.trace))
	copy(out, t.trace)
	return out
}

// Violations is how many statements the allowlist has refused so far. It lets a
// caller that gets its rows back lazily tell "this returned nothing" from "this
// was refused" at the moment of the call (see reader.Query).
func (t *Tracer) Violations() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.violations
}

// Violation returns ErrRefused wrapped with the first refused statement when
// the allowlist refused anything, and nil otherwise. A run that saw a violation
// fails even if the caller swallowed the error pgx returned.
func (t *Tracer) Violation() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.violations == 0 {
		return nil
	}
	for _, s := range t.trace {
		if s.Refused {
			return fmt.Errorf("%w: %s (%d refused in this run)", ErrRefused, s.SQL, t.violations)
		}
	}
	return ErrRefused
}

// TraceQueryStart matches the statement against the allowlist. An unmatched
// statement gets a cancelled context, which pgconn checks before it writes to
// the connection, so the statement never reaches the server.
func (t *Tracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return t.check(ctx, data.SQL)
}

// TraceQueryEnd completes the QueryTracer interface.
func (t *Tracer) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

// TraceBatchStart refuses. The source never batches (ARCHITECTURE.md §2).
func (t *Tracer) TraceBatchStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceBatchStartData) context.Context {
	return t.refuse(ctx, "SendBatch")
}

// TraceBatchQuery completes the BatchTracer interface.
func (t *Tracer) TraceBatchQuery(context.Context, *pgx.Conn, pgx.TraceBatchQueryData) {}

// TraceBatchEnd completes the BatchTracer interface.
func (t *Tracer) TraceBatchEnd(context.Context, *pgx.Conn, pgx.TraceBatchEndData) {}

// TraceCopyFromStart refuses. CopyFrom is a write path and the source is
// read-only (ARCHITECTURE.md §2).
func (t *Tracer) TraceCopyFromStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceCopyFromStartData) context.Context {
	return t.refuse(ctx, "CopyFrom "+data.TableName.Sanitize())
}

// TraceCopyFromEnd completes the CopyFromTracer interface.
func (t *Tracer) TraceCopyFromEnd(context.Context, *pgx.Conn, pgx.TraceCopyFromEndData) {}

// TracePrepareStart refuses. The source pool runs in pgx.QueryExecModeExec, so
// pgx never prepares on its own and a Prepare here is ours.
func (t *Tracer) TracePrepareStart(ctx context.Context, _ *pgx.Conn, data pgx.TracePrepareStartData) context.Context {
	return t.refuse(ctx, "Prepare "+data.Name)
}

// TracePrepareEnd completes the PrepareTracer interface.
func (t *Tracer) TracePrepareEnd(context.Context, *pgx.Conn, pgx.TracePrepareEndData) {}

// TraceConnectStart records the connection and allows it. Refusing here would
// refuse the connection the allowlist exists to defend.
func (t *Tracer) TraceConnectStart(ctx context.Context, _ pgx.TraceConnectStartData) context.Context {
	t.record(pipeline.TracedStatement{At: time.Now(), Shape: "source.connect"})
	return ctx
}

// TraceConnectEnd completes the ConnectTracer interface.
func (t *Tracer) TraceConnectEnd(context.Context, pgx.TraceConnectEndData) {}

func (t *Tracer) check(ctx context.Context, sql string) context.Context {
	norm := normaliseSQL(sql)
	name := t.match(norm)
	rec := pipeline.TracedStatement{
		At:      time.Now(),
		Shape:   name,
		SQL:     elideLiterals(norm),
		Refused: name == "",
	}
	t.record(rec)
	if rec.Refused {
		return cancelled(ctx)
	}
	return ctx
}

func (t *Tracer) refuse(ctx context.Context, what string) context.Context {
	t.record(pipeline.TracedStatement{At: time.Now(), SQL: what, Refused: true})
	return cancelled(ctx)
}

func (t *Tracer) match(norm string) string {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, s := range t.shapes {
		if s.re.MatchString(norm) {
			return s.name
		}
	}
	return ""
}

func (t *Tracer) record(s pipeline.TracedStatement) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.trace = append(t.trace, s)
	if s.Refused {
		t.violations++
	}
}

// cancelled returns a context that is already done. pgconn checks it before it
// writes, and returns an error marked safe to retry without touching the
// connection, so a refusal costs the run a statement and not the pool.
func cancelled(ctx context.Context) context.Context {
	c, cancel := context.WithCancel(ctx)
	cancel()
	return c
}

var whitespace = regexp.MustCompile(`\s+`)

func normaliseSQL(sql string) string {
	return strings.TrimSpace(whitespace.ReplaceAllString(sql, " "))
}

// The placeholder expansions. An identifier is either quoted (with "" for an
// embedded quote) or unquoted, and may carry one qualifying prefix.
const (
	reIdentPart = `(?:"(?:[^"]|"")+"|[a-z_][a-z0-9_$]*)`
	reIdent     = reIdentPart + `(?:\.` + reIdentPart + `)?`
	reIdents    = reIdent + `(?: *, *` + reIdent + `)*`
	reInt       = `[0-9]+`
	reSnapshot  = `'[0-9a-f]+-[0-9a-f]+-[0-9]+'`

	// A type name as pg_catalog.format_type writes one: an identifier, then any
	// of the multi-word spellings the built-in types have ("double precision",
	// "timestamp(3) without time zone", "interval day to second(6)"), then an
	// optional array suffix. A type whose own name needs quoting arrives from
	// format_type already quoted, and reIdent takes it.
	//
	// reTypeWord is a closed list and not "a word", because a run of arbitrary
	// words after a cast is a clause: `SELECT t."a"::int INTO evil FROM ...` is
	// CREATE TABLE AS, and with `[a-z]+` as the continuation the `INTO evil` was
	// read as part of the type name and the statement matched the seed shape.
	// Only the READ ONLY transaction refused it, which is the layering
	// THREAT_MODEL.md T9 describes the other way round — the transaction
	// enforces, the tracer is the evidence — so the evidence was the layer that
	// failed. And that transaction was not under every statement: Source.SystemID
	// queries outside any BEGIN, so an autocommit path had this allowlist and
	// nothing else until Connect began setting default_transaction_read_only=on
	// (pg.go). The list is every word format_type puts after the first one;
	// a type spelling that needs another word is added here, once.
	reTypeMod  = `(?:\( *[0-9]+ *(?:, *[0-9]+ *)?\))?`
	reTypeWord = `(?:with|without|time|zone|varying|precision|double|to|year|month|day|hour|minute|second)`
	reTypeName = reIdent + reTypeMod + `(?: ` + reTypeWord + reTypeMod + `){0,4}`
	reCast     = `::` + reTypeName + `(?:\[ *\])?`

	// A select-list item is a column, optionally read as another type, optionally
	// renamed: `t."a"`, `t."a"::text`, `c1 AS o1`, `t."a"::text AS o1`. It is not
	// an expression: the only parenthesis it can carry is a type modifier's, and
	// the only thing inside that is digits, so there is nowhere in it for a call
	// or for a value.
	reSelectItem = reIdent + `(?:` + reCast + `)?(?: AS ` + reIdentPart + `)?`
	reSelectList = reSelectItem + `(?: *, *` + reSelectItem + `)*`

	// The argument list of a chunk join: one typed array parameter per identity
	// column. The value never appears — it is bound — so this is the arity and
	// the casts and nothing else.
	reCastArg  = `\$[0-9]+::` + reTypeName + `\[ *\]`
	reCastArgs = reCastArg + `(?: *, *` + reCastArg + `)*`

	// A key predicate is the ON of a chunk join and the MATCH SIMPLE NOT NULL
	// filter beside it: a conjunction of column-to-column equalities and IS NOT
	// NULL tests, in either case over identifiers only.
	reKeyTerm = `(?:` + reIdent + ` *= *` + reIdent + `(?:` + reCast + `)?|` + reIdent + ` IS NOT NULL)`
	reKeyPred = reKeyTerm + `(?: AND ` + reKeyTerm + `)*`

	// The operator's --where predicate: text that carries no statement
	// separator, no comment introducer, no backslash and no dollar sign, and
	// whose parentheses balance within the predicate.
	//
	// The alternation is how "no `--` and no `/*`" is said without a lookahead,
	// which RE2 does not have: a `-` or a `/` is admitted only together with the
	// character after it, and that character is neither the second half of a
	// comment introducer nor itself excluded. A predicate that ends in `-` or
	// `/`, or that writes `a/-1` with no space, is refused with them; each is a
	// syntax error on the server or one space away from a predicate that passes.
	// `-(` and `/(` are the one pairing the atoms cannot express, so a group may
	// carry either as a prefix.
	//
	// Parentheses are structure here, not characters, because the template
	// writes `WHERE ({where})` and an unbalanced predicate closes that
	// parenthesis and opens a new clause: `id > 0) UNION ALL SELECT c."pan" AS
	// o1 FROM "public"."cards" c WHERE (true` used to match the seed shape with
	// every parenthesis in the statement paired. Requiring the predicate's own
	// parentheses to balance, to whereMaxDepth levels, is what makes the
	// statement the shape describes the statement that runs.
	reWhereOrd   = `[^;\\$/()-]`
	reWhereDash  = `-` + reWhereOrd
	reWhereSlash = `/[^;\\$*/()-]`
	reWhereAtom  = `(?:` + reWhereOrd + `|` + reWhereDash + `|` + reWhereSlash + `)`

	// whereMaxDepth is how deeply a predicate's own parentheses may nest. RE2
	// cannot count, so the depth is unrolled and therefore finite; six is past
	// anything a hand-written predicate reaches, and a deeper one is refused by
	// internal/plan before a statement is built rather than here.
	whereMaxDepth = 6
)

// reWhere is the {where} expansion: balanced parentheses to whereMaxDepth, with
// the excluded characters kept out at every level.
var reWhere = whereAtDepth(whereMaxDepth)

func whereAtDepth(depth int) string {
	if depth <= 0 {
		return `(?:` + reWhereAtom + `)+`
	}
	return `(?:` + reWhereAtom + `|[-/]?\((?:` + whereAtDepth(depth-1) + `)?\))+`
}

var (
	placeholder = regexp.MustCompile(`\{(ident|idents|int|snapshot|selectlist|casts|keypred|where)\}`)
	// placeholderToken is anything shaped like a placeholder, so that every one
	// of them is checked and not merely the template as a whole.
	placeholderToken = regexp.MustCompile(`\{[a-zA-Z_]+\}`)
)

func compileShape(template string) (*regexp.Regexp, error) {
	norm := normaliseSQL(template)
	if norm == "" {
		return nil, errors.New("the template is empty")
	}

	var b strings.Builder
	b.WriteString(`(?is)\A`)
	for _, seg := range templateSegments(norm) {
		if seg.quoted {
			// A quoted identifier is a name, and a name is arbitrary text: it is
			// matched as itself, placeholder-shaped or not (see
			// templateSegments).
			//
			// As *itself* and not with quoteLiteral: that function is for the
			// template's fixed SQL text, where a space stands for a run of
			// layout and case is the writer's whim, and a name has neither.
			// `"my table"` through quoteLiteral matches `"mytable"` — ` *` is
			// zero or more spaces — and under the surrounding (?i) a shape
			// naming `"LegacyCustomer"` matched a read of `"legacycustomer"`,
			// which Postgres treats as a different table (nasty.sql ships both
			// spellings). Either is the widening a per-table shape exists to
			// close: a read of a table the plan never named. So the name is
			// regexp-quoted whole and matched case-sensitively, inside (?-i:),
			// while the SQL around it keeps the (?is) the template is compiled
			// with — an unquoted identifier is folded by the server and must
			// stay case-insensitive here.
			//
			// One widening this does not close: normaliseSQL collapses runs of
			// whitespace on both sides before either is matched, so a name
			// carrying two spaces is compared as if it carried one and a shape
			// naming `"my table"` still admits a read of `"my  table"`. Closing
			// it means normalising around quotes rather than through them,
			// which changes how every statement — including one carrying an
			// operator's --where predicate — is normalised, and is reported
			// rather than done here.
			b.WriteString(`(?-i:` + regexp.QuoteMeta(seg.text) + `)`)
			continue
		}
		if err := expandInto(&b, seg.text); err != nil {
			return nil, err
		}
	}
	b.WriteString(`\z`)
	return regexp.Compile(b.String())
}

// expandInto writes one run of template text — everything outside a quoted
// identifier — to b, with each placeholder replaced by its expansion.
func expandInto(b *strings.Builder, text string) error {
	// Every placeholder-shaped token is checked, not just whether the template
	// holds one good placeholder somewhere: `SELECT {ident} FROM {table}` would
	// otherwise compile with {table} regex-quoted as literal text, producing a
	// shape that can never match and turning a stage's typo into a refusal storm
	// at run time. This function exists so that such a typo fails the build's
	// tests instead.
	for _, tok := range placeholderToken.FindAllString(text, -1) {
		if !placeholder.MatchString(tok) {
			return fmt.Errorf("the template carries %s, which is not one of {ident}, {idents}, {int}, {snapshot}, {selectlist}, {casts}, {keypred}, {where}", tok)
		}
	}

	last := 0
	for _, m := range placeholder.FindAllStringSubmatchIndex(text, -1) {
		b.WriteString(quoteLiteral(text[last:m[0]]))
		switch text[m[2]:m[3]] {
		case "ident":
			b.WriteString(reIdent)
		case "idents":
			b.WriteString(reIdents)
		case "int":
			b.WriteString(reInt)
		case "snapshot":
			b.WriteString(reSnapshot)
		case "selectlist":
			b.WriteString(reSelectList)
		case "casts":
			b.WriteString(reCastArgs)
		case "keypred":
			b.WriteString(reKeyPred)
		case "where":
			b.WriteString(reWhere)
		}
		last = m[1]
	}
	b.WriteString(quoteLiteral(text[last:]))
	return nil
}

// segment is one run of a template: either template text, in which a
// placeholder is a placeholder, or a quoted identifier, in which nothing is.
type segment struct {
	text   string
	quoted bool
}

// templateSegments splits a normalised template into those runs, in order. A
// quoted run carries its own delimiting quotes.
//
// It exists because a template's fixed text may carry an identifier the caller
// quoted into it: internal/extract names its per-table lookup shape that way
// (`SELECT {selectlist} FROM "public"."categories" t ORDER BY {idents} LIMIT
// 1001`) and internal/verify names its per-table sample shape and its per-column
// probe shapes the same way. An identifier is arbitrary text — Postgres will
// quote anything — so a table actually called `{ident}` used to be read as a
// placeholder, and the shape that names one table compiled into the shape that
// admits an ordered read of *every* relation, which is precisely the widening
// naming the table exists to prevent (internal/extract/shapes.go). A table
// called `{table}` was the other half: the token is not one of the eight, so
// compiling the shape failed and took the run with it.
//
// Both are fixed by never looking for a placeholder inside a quoted identifier.
// No template lazyslice writes carries a placeholder there — every placeholder
// stands for a whole identifier, quotes included — so nothing legitimate is
// lost, and the only direction this can move a shape is narrower.
//
// An unterminated quote takes the rest of the template as one quoted run, for
// the same reason: a shape that matches nothing refuses, and refusing is the
// direction a malformed template should fail in.
func templateSegments(s string) []segment {
	var out []segment
	start := 0
	for i := 0; i < len(s); {
		if s[i] != '"' {
			i++
			continue
		}
		if start < i {
			out = append(out, segment{text: s[start:i]})
		}
		j := i + 1
		for j < len(s) {
			if s[j] != '"' {
				j++
				continue
			}
			// "" is an embedded quote and not the end of the identifier.
			if j+1 < len(s) && s[j+1] == '"' {
				j += 2
				continue
			}
			j++
			break
		}
		out = append(out, segment{text: s[i:j], quoted: true})
		i, start = j, j
	}
	if start < len(s) {
		out = append(out, segment{text: s[start:]})
	}
	return out
}

// quoteLiteral escapes the fixed text of a template and lets a single space in
// the template stand for any run of whitespace in the statement.
func quoteLiteral(s string) string {
	parts := strings.Split(s, " ")
	for i, p := range parts {
		parts[i] = regexp.QuoteMeta(p)
	}
	return strings.Join(parts, ` *`)
}

var (
	stringLiteral  = regexp.MustCompile(`'(?:[^']|'')*'`)
	numericLiteral = regexp.MustCompile(`\b[0-9]+(?:\.[0-9]+)?\b`)
	// dollarQuote is a $$ or $tag$ delimiter. It is not elided, it is detected:
	// see elideLiterals.
	dollarQuote = regexp.MustCompile(`\$[a-zA-Z_0-9]*\$`)
	// leadingKeyword is the first word of a statement, which is all that is
	// recorded when the text cannot be shown to be free of values.
	leadingKeyword = regexp.MustCompile(`\A[a-zA-Z]+`)
)

// elideLiterals removes the string and numeric literals from a statement before
// it is recorded, so that the trace of a refused statement cannot carry the row
// value that a bug interpolated into it (THREAT_MODEL.md T4). Bind parameters
// are already outside the text; this is for the text itself. The trace is what
// --debug prints (ARCHITECTURE.md §8: statement shapes with parameters elided,
// never values), so this is the whole of the control on that path.
//
// Two literal forms the patterns cannot close over: E'alice\'s@example.com',
// where the backslash-escaped quote splits the string match and leaves the tail
// of the value standing, and $$...$$ / $tag$...$tag$, which the string pattern
// does not match at all. Both are what a naive interpolation of a value
// containing an apostrophe produces. Rather than chase the forms, the escape
// form is refused as itself and the elided text is checked for structure.
//
// The backslash escape is detected by the backslash, not by its side effect on
// quote parity, because parity is restorable: two escaped literals in one
// statement — what interpolating two names carrying an apostrophe produces —
// leave an even number of quotes with a whole value standing between them, so
// `INSERT INTO t (a,b,c) VALUES (E'o\'brien', 'carol@example.com',
// E'd\'angelo')` passes a parity test with the address intact. lazyslice
// generates no statement containing a backslash, so a backslash anywhere in the
// text means the elision cannot be shown to have worked. The same goes for a
// remaining dollar-quote delimiter, and for an odd number of quotes. In each
// case the statement is recorded as its leading keyword alone.
func elideLiterals(sql string) string {
	if strings.ContainsRune(sql, '\\') {
		return elided(sql)
	}
	out := stringLiteral.ReplaceAllString(sql, "'?'")
	out = numericLiteral.ReplaceAllString(out, "?")
	if strings.Count(out, "'")%2 == 0 && !dollarQuote.MatchString(out) {
		return out
	}
	return elided(out)
}

// elided is what is recorded when the text cannot be shown to be free of
// values: the leading keyword, which says what kind of statement it was, and
// nothing else.
func elided(sql string) string {
	if kw := leadingKeyword.FindString(sql); kw != "" {
		return kw + " <elided>"
	}
	return "<elided>"
}
