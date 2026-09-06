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
//	{ident}     one identifier, optionally schema-qualified, quoted or not
//	{idents}    a comma-separated list of one or more {ident}
//	{int}       a non-negative integer literal
//	{snapshot}  a quoted snapshot identifier as pg_export_snapshot returns it
//
// Case is ignored because lazyslice generates every one of these statements and
// the templates fix their structure; ignoring case widens what our own SQL may
// look like, and widens nothing else — no case of `SELECT ... FROM {ident}`
// spells a `DELETE`. A later stage that needs a shape this grammar cannot
// express adds a placeholder here, where it is reviewed once, rather than
// registering a looser template.
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
)

var (
	placeholder = regexp.MustCompile(`\{(ident|idents|int|snapshot)\}`)
	// placeholderToken is anything shaped like a placeholder, so that every one
	// of them is checked and not merely the template as a whole.
	placeholderToken = regexp.MustCompile(`\{[a-zA-Z_]+\}`)
)

func compileShape(template string) (*regexp.Regexp, error) {
	norm := normaliseSQL(template)
	if norm == "" {
		return nil, errors.New("the template is empty")
	}
	// Every placeholder-shaped token is checked, not just whether the template
	// holds one good placeholder somewhere: `SELECT {ident} FROM {table}` would
	// otherwise compile with {table} regex-quoted as literal text, producing a
	// shape that can never match and turning a stage's typo into a refusal storm
	// at run time. This function exists so that such a typo fails the build's
	// tests instead.
	for _, tok := range placeholderToken.FindAllString(norm, -1) {
		if !placeholder.MatchString(tok) {
			return nil, fmt.Errorf("the template carries %s, which is not one of {ident}, {idents}, {int}, {snapshot}", tok)
		}
	}

	var b strings.Builder
	b.WriteString(`(?is)\A`)
	last := 0
	for _, m := range placeholder.FindAllStringSubmatchIndex(norm, -1) {
		b.WriteString(quoteLiteral(norm[last:m[0]]))
		switch norm[m[2]:m[3]] {
		case "ident":
			b.WriteString(reIdent)
		case "idents":
			b.WriteString(reIdents)
		case "int":
			b.WriteString(reInt)
		case "snapshot":
			b.WriteString(reSnapshot)
		}
		last = m[1]
	}
	b.WriteString(quoteLiteral(norm[last:]))
	b.WriteString(`\z`)
	return regexp.Compile(b.String())
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
