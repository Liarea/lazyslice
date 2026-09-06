// SPDX-License-Identifier: Apache-2.0

package pg

import (
	"context"
	"strings"
	"testing"
)

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
		ctx := tr.check(context.Background(), sql)
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
		ctx := tr.check(context.Background(), sql)
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
	tr.check(context.Background(), `DELETE FROM users WHERE email = 'alice@example.com' AND id = 4173`)

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
		tr.check(context.Background(), sql)
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
	tr.check(context.Background(), `SELECT current_setting('is_superuser') = 'on'`)
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
