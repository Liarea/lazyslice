// SPDX-License-Identifier: Apache-2.0

package pg

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// noConn is the transaction status check sees when it is called without a
// connection, which is how most of these tests call it: they are asking the
// allowlist about a shape, and the transaction rule has its own test below.
const noConn = byte(0)

// The allowlist is a shape allowlist and not a keyword allowlist, because a
// keyword allowlist is defeated by statements that begin with an allowed word
// (THREAT_MODEL.md T9). These are those statements.
func TestTracerRefusesWritesDressedAsReads(t *testing.T) {
	tr, err := NewTracer(Shape{Name: "plan.child_keys", SQL: `SELECT * FROM {ident} WHERE id = ANY($1)`})
	if err != nil {
		t.Fatalf("NewTracer: %v", err)
	}

	for _, sql := range []string{
		`WITH x AS (DELETE FROM users RETURNING *) SELECT * FROM x`,
		`SELECT lo_import('/etc/passwd')`,
		`SELECT * FROM users WHERE id = ANY($1); DROP TABLE users`,
		`UPDATE users SET email = 'x'`,
		`SELECT * FROM users`, // right table, wrong shape: no WHERE
	} {
		ctx := tr.check(context.Background(), noConn, sql)
		if ctx.Err() == nil {
			t.Errorf("the allowlist admitted %q", sql)
		}
	}

	if err := tr.Violation(); err == nil {
		t.Fatal("Violation() = nil after five refusals")
	}

	// And the shape it was given still passes, in either case and with the
	// whitespace a generator actually emits.
	for _, sql := range []string{
		`SELECT * FROM users WHERE id = ANY($1)`,
		"select *\n  from public.\"Users\"\n  where id = any($1)",
	} {
		ctx := tr.check(context.Background(), noConn, sql)
		if ctx.Err() != nil {
			t.Errorf("the allowlist refused its own shape: %q", sql)
		}
	}
}

// A refused statement is recorded so that the run can say what it refused. What
// it must not record is a value a bug interpolated into the text (THREAT_MODEL.md
// T4).
func TestTracerRecordsShapesAndNotValues(t *testing.T) {
	tr, err := NewTracer()
	if err != nil {
		t.Fatalf("NewTracer: %v", err)
	}
	tr.check(context.Background(), noConn, `DELETE FROM users WHERE email = 'alice@example.com' AND id = 4173`)

	trace := tr.Trace()
	if len(trace) != 1 {
		t.Fatalf("Trace() has %d entries, want 1", len(trace))
	}
	got := trace[0]
	if !got.Refused || got.Shape != "" {
		t.Errorf("Trace()[0] = %+v, want a refusal with no shape name", got)
	}
	for _, secret := range []string{"alice@example.com", "4173"} {
		if strings.Contains(got.SQL, secret) {
			t.Errorf("the trace records %q, which is a value: %q", secret, got.SQL)
		}
	}
	if !strings.Contains(strings.ToUpper(got.SQL), "DELETE FROM USERS") {
		t.Errorf("the trace = %q, want it to still say what was refused", got.SQL)
	}
}

// The two literal forms a quote-and-digit pattern cannot close over: the E'...'
// escape, where the backslash-escaped quote splits the string match and leaves
// the tail of the value standing, and dollar quoting, which the pattern does not
// match at all. Both are what a naive interpolation of a value with an
// apostrophe produces, and the trace is what --debug prints (THREAT_MODEL.md
// T4).
func TestTracerElidesTheLiteralFormsAPatternCannotClose(t *testing.T) {
	tr, err := NewTracer()
	if err != nil {
		t.Fatalf("NewTracer: %v", err)
	}
	for _, sql := range []string{
		`SELECT * FROM users WHERE email = E'alice\'s@example.com'`,
		`INSERT INTO t VALUES ($$alice@example.com$$)`,
		`INSERT INTO t VALUES ($tag$alice@example.com$tag$)`,
		// Two escaped literals restore the quote parity a single one breaks, so
		// the value between them survives any test that watches parity rather
		// than the escape itself. This is what interpolating two names carrying
		// an apostrophe produces.
		`INSERT INTO t (a,b,c) VALUES (E'o\'brien', 'carol@example.com', E'd\'angelo')`,
		`SELECT * FROM t WHERE a = E'o\'brien' AND email = 'carol@example.com' AND b = E'd\'angelo'`,
	} {
		tr.check(context.Background(), noConn, sql)
	}

	trace := tr.Trace()
	if len(trace) != 5 {
		t.Fatalf("Trace() has %d entries, want 5", len(trace))
	}
	for i, got := range trace {
		if strings.Contains(got.SQL, "example.com") || strings.Contains(got.SQL, "alice") || strings.Contains(got.SQL, "carol") {
			t.Errorf("Trace()[%d].SQL = %q, which carries the value", i, got.SQL)
		}
		if got.SQL == "" {
			t.Errorf("Trace()[%d].SQL is empty; the trace still has to say what was refused", i)
		}
	}
	if want := "SELECT <elided>"; trace[0].SQL != want {
		t.Errorf("Trace()[0].SQL = %q, want %q", trace[0].SQL, want)
	}

	// And an ordinary statement is still recorded in full, so the elision is not
	// simply refusing to record anything.
	tr.check(context.Background(), noConn, `SELECT current_setting('is_superuser') = 'on'`)
	last := tr.Trace()[5]
	if !strings.Contains(last.SQL, "current_setting") || strings.Contains(last.SQL, "<elided>") {
		t.Errorf("Trace()[5].SQL = %q, want the statement with its literals elided", last.SQL)
	}
}

// CopyFrom, SendBatch and Prepare are not statements to be matched. They are
// calls the source never makes, so the tracer refuses them outright.
func TestTracerRefusesCopyBatchAndPrepare(t *testing.T) {
	tr, err := NewTracer(Shape{Name: "anything", SQL: `SELECT 1`})
	if err != nil {
		t.Fatalf("NewTracer: %v", err)
	}
	for name, ctx := range map[string]context.Context{
		"CopyFrom":  tr.refuse(context.Background(), "CopyFrom users"),
		"SendBatch": tr.refuse(context.Background(), "SendBatch"),
		"Prepare":   tr.refuse(context.Background(), "Prepare stmt1"),
	} {
		if ctx.Err() == nil {
			t.Errorf("%s was not refused", name)
		}
	}
	if tr.Violation() == nil {
		t.Error("Violation() = nil after three refusals")
	}
}

// Read-only is set per transaction and never as a session default (T-0076), so
// a statement that arrives on a source connection with no transaction open has
// the allowlist and no server-side rail behind it. Until this check existed
// nothing detected that: internal/discover's dial sent three catalog reads that
// way and it was found by reading prose (T-0081, T-0082).
func TestASourceStatementOutsideATransactionIsRefused(t *testing.T) {
	tr, err := NewTracer(SourceShapes()...)
	if err != nil {
		t.Fatalf("NewTracer: %v", err)
	}

	// The statements a correct call site sends, in the order it sends them: the
	// BEGIN arrives on an idle connection because opening the transaction is
	// what it is for, and everything after it is inside one. 'E' is a
	// transaction that has failed, which is still a transaction — the ROLLBACK
	// that ends it must not be refused.
	for _, c := range []struct {
		tx  byte
		sql string
	}{
		{txIdle, sqlBeginReadOnly},
		{'T', sqlRole},
		{'E', sqlRollback},
		// No connection at all is not an idle connection: it is the tracer
		// being asked about a shape, which is how the shape tests in this
		// package and in internal/plan, internal/extract and internal/verify
		// call it.
		{noConn, sqlRole},
	} {
		if ctx := tr.check(context.Background(), c.tx, c.sql); ctx.Err() != nil {
			t.Errorf("the allowlist refused %q at transaction status %q", c.sql, statusName(c.tx))
		}
	}
	if err := tr.Violation(); err != nil {
		t.Fatalf("Violation() = %v before anything was refused", err)
	}

	// The same registered statement on an idle connection is a violation.
	if ctx := tr.check(context.Background(), txIdle, sqlRole); ctx.Err() == nil {
		t.Error("a registered statement was admitted on a connection with no transaction open")
	}

	violation := tr.Violation()
	if !errors.Is(violation, ErrOutsideTransaction) {
		t.Fatalf("Violation() = %v, want ErrOutsideTransaction", violation)
	}
	if errors.Is(violation, ErrRefused) {
		t.Error("Violation() reports an unallowlisted statement; the statement was allowlisted and had no transaction under it")
	}

	last := tr.Trace()[len(tr.Trace())-1]
	if !last.Refused {
		t.Error("the trace does not record the statement as refused")
	}
	if last.Shape != "" {
		t.Errorf("Trace() records shape %q for a refused statement; pipeline.TracedStatement documents it as empty", last.Shape)
	}
}

// A statement that is both unallowlisted and on an idle connection is refused
// for the shape and not for the transaction.
//
// This is the THREAT_MODEL.md T9 case the allowlist exists for — an injected or
// hand-added write on the source — and it is the case most likely to arrive
// with no transaction under it. Reporting it as ErrOutsideTransaction would
// tell the operator the statement was one of ours and only its transaction was
// missing, which is the opposite of what happened.
func TestAnUnallowlistedStatementOnAnIdleConnectionIsRefusedForItsShape(t *testing.T) {
	tr, err := NewTracer(SourceShapes()...)
	if err != nil {
		t.Fatalf("NewTracer: %v", err)
	}

	if ctx := tr.check(context.Background(), txIdle, `DROP TABLE users`); ctx.Err() == nil {
		t.Fatal("an unallowlisted statement was admitted")
	}

	violation := tr.Violation()
	if !errors.Is(violation, ErrRefused) {
		t.Fatalf("Violation() = %v, want ErrRefused", violation)
	}
	if errors.Is(violation, ErrOutsideTransaction) {
		t.Error("Violation() blames the missing transaction; the statement was not one we generate")
	}
}

func statusName(tx byte) string {
	if tx == 0 {
		return "none"
	}
	return string(rune(tx))
}

func TestShapePlaceholders(t *testing.T) {
	tr, err := NewTracer(
		Shape{Name: "count", SQL: `SELECT count(*) FROM (SELECT {int} FROM {ident} LIMIT {int}) x`},
		Shape{Name: "cols", SQL: `SELECT {idents} FROM {ident}`},
		Shape{Name: "snapshot", SQL: sqlSetSnapshotShape},
	)
	if err != nil {
		t.Fatalf("NewTracer: %v", err)
	}

	accepted := map[string]string{
		`SELECT count(*) FROM (SELECT 1 FROM public.orders LIMIT 1001) x`: "count",
		`SELECT "a", "b c", d FROM "public"."Order Items"`:                "cols",
		`SET TRANSACTION SNAPSHOT '00000003-0000001f-1'`:                  "snapshot",
	}
	for sql, want := range accepted {
		if got := tr.match(normaliseSQL(sql)); got != want {
			t.Errorf("match(%q) = %q, want %q", sql, got, want)
		}
	}

	for _, sql := range []string{
		`SELECT a, b FROM orders JOIN customers ON true`,
		`SET TRANSACTION SNAPSHOT 'nonsense'`,
		`SELECT count(*) FROM (SELECT 1 FROM public.orders LIMIT 1001) y`,
	} {
		if got := tr.match(normaliseSQL(sql)); got != "" {
			t.Errorf("match(%q) = %q, want no match", sql, got)
		}
	}
}

func TestCompileShapeRejectsAnUnknownPlaceholder(t *testing.T) {
	if _, err := NewTracer(Shape{Name: "bad", SQL: `SELECT * FROM {table}`}); err == nil {
		t.Fatal("NewTracer accepted a template with an unknown placeholder")
	}
	if _, err := NewTracer(Shape{Name: "", SQL: `SELECT 1`}); err == nil {
		t.Fatal("NewTracer accepted an unnamed shape")
	}

	// A template that mixes a good placeholder with a typo'd one is the case
	// that matters: it compiles with {table} quoted as literal text, so the shape
	// can never match and a stage's typo becomes a refusal storm at run time
	// rather than a failing test here.
	mixed := Shape{Name: "mixed", SQL: `SELECT {ident} FROM {table} LIMIT {int}`}
	if _, err := NewTracer(mixed); err == nil {
		t.Fatalf("NewTracer accepted %q, which can never match a real statement", mixed.SQL)
	}
}

// A table's name is arbitrary text, and the per-table shapes internal/extract
// and internal/verify build quote one into the template. A table called
// `{ident}` used to compile into the shape that admits an ordered read of every
// relation — the exact widening naming the table exists to prevent — and one
// called `{table}` used to fail compilation and take the run with it. A quoted
// identifier is fixed text now, so both are shapes about one table.
func TestAPlaceholderShapedTableNameIsAName(t *testing.T) {
	// Names a psql user can create and Postgres will hand back quoted:
	// CREATE TABLE public."{ident}" (...).
	const (
		braced  = `SELECT {selectlist} FROM "public"."{ident}" t ORDER BY {idents} LIMIT 1001`
		unknown = `SELECT {selectlist} FROM "public"."{table}" t ORDER BY {idents} LIMIT 1001`
	)
	tr, err := NewTracer(
		Shape{Name: "extract.lookup.public.{ident}", SQL: braced},
		Shape{Name: "extract.lookup.public.{table}", SQL: unknown},
	)
	if err != nil {
		// The {table} half: an unknown placeholder inside a quoted identifier is
		// a table name, not a typo, so compiling it must not fail.
		t.Fatalf("NewTracer refused a shape naming a table called {ident} or {table}: %v", err)
	}

	// The read of the table the shape actually names still matches.
	for name, sql := range map[string]string{
		"extract.lookup.public.{ident}": `SELECT t."id" FROM "public"."{ident}" t ORDER BY t."id" LIMIT 1001`,
		"extract.lookup.public.{table}": `SELECT t."id" FROM "public"."{table}" t ORDER BY t."id" LIMIT 1001`,
	} {
		if got := tr.match(normaliseSQL(sql)); got != name {
			t.Errorf("match(%q) = %q, want %q", sql, got, name)
		}
	}

	// And the reads it does not name are still refused — which is the whole
	// point of naming the table in the template. The last two are the near
	// misses, and they are the ones a test using wholly different names cannot
	// catch: a table whose name differs from the one the shape names only by
	// case, or only by a space.
	for _, sql := range []string{
		`SELECT t."id" FROM "public"."orders" t ORDER BY t."id" LIMIT 1001`,
		`SELECT t."rolname", t."rolpassword" FROM "pg_catalog"."pg_authid" t ORDER BY t."rolname" LIMIT 1001`,
		`SELECT t."id" FROM public.people t ORDER BY t."id" LIMIT 1001`,
		`SELECT t."id" FROM "public"."{IDENT}" t ORDER BY t."id" LIMIT 1001`,
		`SELECT t."id" FROM "public"."{ ident }" t ORDER BY t."id" LIMIT 1001`,
	} {
		if got := tr.match(normaliseSQL(sql)); got != "" {
			t.Errorf("match(%q) = %q; a shape naming one table admitted another", sql, got)
		}
	}
}

// A quoted name is matched the way Postgres reads one: case-sensitively, and
// space for space. The rest of the template is not — the server folds an
// unquoted identifier and nobody's SQL keyword case is load-bearing — so the
// two halves are asserted together, in both directions.
//
// Both near misses were real. The whole pattern is compiled with (?i), so a
// shape naming `"LegacyCustomer"` (testdata/nasty.sql ships that table)
// admitted a read of `"legacycustomer"`, which on the server is a different
// table; and quoteLiteral lets a space in the template stand for a run of zero
// or more spaces in the statement, so a shape naming `"my table"` admitted a
// read of `"mytable"`. Each is the widening a per-table shape exists to close:
// a read of a table the plan never named.
func TestAQuotedNameInAShapeIsMatchedAsPostgresReadsOne(t *testing.T) {
	const (
		mixedCase = `SELECT {selectlist} FROM "public"."LegacyCustomer" t ORDER BY {idents} LIMIT 1001`
		spaced    = `SELECT {selectlist} FROM "public"."my table" t ORDER BY {idents} LIMIT 1001`
	)
	tr, err := NewTracer(
		Shape{Name: "extract.lookup.public.LegacyCustomer", SQL: mixedCase},
		Shape{Name: "extract.lookup.public.my table", SQL: spaced},
	)
	if err != nil {
		t.Fatalf("NewTracer: %v", err)
	}

	// The table the shape names, read as the statement builders spell it — and
	// with the keywords in another case, because case outside the name is still
	// ignored.
	for name, sql := range map[string]string{
		"extract.lookup.public.LegacyCustomer": `SELECT t."id" FROM "public"."LegacyCustomer" t ORDER BY t."id" LIMIT 1001`,
		"extract.lookup.public.my table":       `select t."id" from "public"."my table" t order by t."id" limit 1001`,
	} {
		if got := tr.match(normaliseSQL(sql)); got != name {
			t.Errorf("match(%q) = %q, want %q", sql, got, name)
		}
	}

	// The tables it does not name, each one character from the one it does.
	for _, sql := range []string{
		`SELECT t."id" FROM "public"."legacycustomer" t ORDER BY t."id" LIMIT 1001`,
		`SELECT t."id" FROM "public"."LEGACYCUSTOMER" t ORDER BY t."id" LIMIT 1001`,
		`SELECT t."id" FROM "public"."Legacycustomer" t ORDER BY t."id" LIMIT 1001`,
		`SELECT t."id" FROM "public"."mytable" t ORDER BY t."id" LIMIT 1001`,
		`SELECT t."id" FROM "public"."MY TABLE" t ORDER BY t."id" LIMIT 1001`,
	} {
		if got := tr.match(normaliseSQL(sql)); got != "" {
			t.Errorf("match(%q) = %q; a shape naming one table admitted another whose name differs "+
				"only by case or by a space", sql, got)
		}
	}
}

// The other half of the same rule: a placeholder-shaped token outside a quoted
// identifier is still checked, so a stage's typo still fails the build.
func TestTemplateSegmentsSplitsOnQuotedIdentifiers(t *testing.T) {
	cases := []struct {
		in   string
		want []segment
	}{
		{in: `SELECT {ident}`, want: []segment{{text: `SELECT {ident}`}}},
		{
			in: `SELECT {selectlist} FROM "a"."{ident}" t`,
			want: []segment{
				{text: `SELECT {selectlist} FROM `},
				{text: `"a"`, quoted: true},
				{text: `.`},
				{text: `"{ident}"`, quoted: true},
				{text: ` t`},
			},
		},
		// An embedded quote is written "" and does not end the identifier.
		{
			in: `FROM "a""{int}b" t`,
			want: []segment{
				{text: `FROM `},
				{text: `"a""{int}b"`, quoted: true},
				{text: ` t`},
			},
		},
		// An unterminated quote takes the rest, so the shape matches nothing.
		{
			in: `FROM "a`,
			want: []segment{
				{text: `FROM `},
				{text: `"a`, quoted: true},
			},
		},
	}
	for _, c := range cases {
		got := templateSegments(c.in)
		if len(got) != len(c.want) {
			t.Errorf("templateSegments(%q) = %v, want %v", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("templateSegments(%q)[%d] = %v, want %v", c.in, i, got[i], c.want[i])
			}
		}
	}
}

// Every statement this package sends to the source has to be on the allowlist
// this package registers, or the first real run refuses itself.
func TestSourceShapesCoverThisPackagesOwnStatements(t *testing.T) {
	tr, err := NewTracer(SourceShapes()...)
	if err != nil {
		t.Fatalf("NewTracer: %v", err)
	}
	if err := tr.Register(Shape{Name: "source.system_id", SQL: sqlSystemID}); err != nil {
		t.Fatalf("registering the system-identifier shape: %v", err)
	}
	for _, sql := range []string{
		sqlBeginReadOnly,
		sqlRollback,
		sqlExportSnapshot,
		sqlRole,
		sqlTablePrivileges,
		sqlSystemID,
		`SET TRANSACTION SNAPSHOT '00000003-0000001F-1'`,
	} {
		if got := tr.match(normaliseSQL(sql)); got == "" {
			t.Errorf("the allowlist does not carry a statement internal/pg sends: %q", sql)
		}
	}
}

// The four placeholders the planner and extract needed and the first four could
// not express: a select-list item, a variable-arity typed array argument list, a
// key predicate, and the operator's own --where text (internal/plan/shapes.go).
// Each is a structure, so the test is what it refuses as much as what it takes.
func TestKeyJoinPlaceholders(t *testing.T) {
	const (
		mapKeys = `SELECT DISTINCT {selectlist} FROM {ident} t ` +
			`JOIN unnest({casts}) AS k({idents}) ON {keypred} WHERE {keypred} ORDER BY {idents}`
		seedWhere = `SELECT {selectlist} FROM {ident} t WHERE ({where}) ORDER BY {idents} LIMIT {int}`
	)
	tr, err := NewTracer(
		Shape{Name: "map_keys", SQL: mapKeys},
		Shape{Name: "seed_where", SQL: seedWhere},
	)
	if err != nil {
		t.Fatalf("NewTracer: %v", err)
	}

	accepted := map[string]string{
		// A composite key whose columns travel as int8, as text, and as text
		// cast back to a type that has no array of its own.
		`SELECT DISTINCT t."id" AS o1, t."code"::text AS o2 FROM "public"."orders" t ` +
			`JOIN unnest($1::int8[], $2::text[], $3::uuid[]) AS k(k1, k2, k3) ` +
			`ON t."a" = k.k1 AND t."b" = k.k2::timestamp with time zone AND t."c" = k.k3 ` +
			`WHERE t."a" IS NOT NULL AND t."b" IS NOT NULL ORDER BY o1, o2`: "map_keys",
		`SELECT t."id" AS o1 FROM "public"."orders" t WHERE (id > 100) ORDER BY t."id" LIMIT 500`: "seed_where",
	}
	for sql, want := range accepted {
		if got := tr.match(normaliseSQL(sql)); got != want {
			t.Errorf("match(%q) = %q, want %q", sql, got, want)
		}
	}

	for _, sql := range []string{
		// A select-list item that is a call rather than a column.
		`SELECT DISTINCT pg_read_file('/etc/passwd') AS o1 FROM "public"."orders" t ` +
			`JOIN unnest($1::int8[]) AS k(k1) ON t."a" = k.k1 WHERE t."a" IS NOT NULL ORDER BY o1`,
		// A literal in the join instead of a bound array: the one thing the
		// chunk join exists to keep out of the statement text.
		`SELECT DISTINCT t."id" AS o1 FROM "public"."orders" t ` +
			`JOIN unnest(ARRAY[1,2,3]) AS k(k1) ON t."a" = k.k1 WHERE t."a" IS NOT NULL ORDER BY o1`,
		// A join condition that is a call, and one that compares to a literal.
		`SELECT DISTINCT t."id" AS o1 FROM "public"."orders" t ` +
			`JOIN unnest($1::int8[]) AS k(k1) ON pg_sleep(10) IS NOT NULL WHERE t."a" IS NOT NULL ORDER BY o1`,
		`SELECT DISTINCT t."id" AS o1 FROM "public"."orders" t ` +
			`JOIN unnest($1::int8[]) AS k(k1) ON t."a" = 'alice@example.com' WHERE t."a" IS NOT NULL ORDER BY o1`,
		// A write dressed as a select list. SELECT INTO is CREATE TABLE AS, and
		// `INTO evil` is two letters-only words, which the cast's type name
		// swallowed while its continuation was `[a-z]+` (reTypeWord). Only the
		// READ ONLY transaction refused it, and the tracer is meant to be a
		// control of its own (THREAT_MODEL.md T9). The trailing item has to end
		// in a cast with no alias for the trick to work, so both spellings are
		// here.
		`SELECT t."a"::int INTO evil FROM "public"."orders" t WHERE (id > 0) ORDER BY t."id" LIMIT 500`,
		`SELECT DISTINCT t."id"::int8 into unlogged evil FROM "public"."orders" t ` +
			`JOIN unnest($1::int8[]) AS k(k1) ON t."a" = k.k1 WHERE t."a" IS NOT NULL ORDER BY o1`,
		// A predicate that balances the template's own parenthesis and appends
		// a clause of its own. Every parenthesis in the statement is paired, so
		// only the predicate's own balance refuses it.
		`SELECT t."id" AS o1 FROM "public"."orders" t ` +
			`WHERE (id > 0) UNION ALL SELECT c."pan" AS o1 FROM "public"."cards" c WHERE (true) ` +
			`ORDER BY t."id" LIMIT 500`,
		// A predicate that ends the statement, and one that comments the
		// template's own LIMIT away.
		`SELECT t."id" AS o1 FROM "public"."orders" t WHERE (id > 0); DROP TABLE users ORDER BY t."id" LIMIT 500`,
		`SELECT t."id" AS o1 FROM "public"."orders" t WHERE (id > 0) --) ORDER BY t."id" LIMIT 500`,
		`SELECT t."id" AS o1 FROM "public"."orders" t WHERE (id > 0) /*) ORDER BY t."id" LIMIT 500`,
	} {
		if got := tr.match(normaliseSQL(sql)); got != "" {
			t.Errorf("match(%q) = %q, want no match", sql, got)
		}
	}
}
