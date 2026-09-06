// SPDX-License-Identifier: Apache-2.0

package extract

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/Liarea/lazyslice/internal/pg"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// Shapes() is a promise about sql.go, and this is where the two are held
// together: every statement below is built by the real builder with the
// arguments a step gives it, and run through a real pg.Tracer carrying
// Shapes(plan) and nothing else. A statement no shape covers gets a cancelled
// context here, which is exactly what it would get on a production source
// (THREAT_MODEL.md T9) — so a clause added in sql.go and not added here fails
// this test rather than the run.
//
// These cases are the ones reviewed as pg.ExtractShapes() before this package
// existed (internal/pg/shapes_extract.go, now deleted); they moved here with
// the shapes.

func tracerFor(t *testing.T, p *pipeline.Plan) *pg.Tracer {
	t.Helper()
	shapes := make([]pg.Shape, 0, len(Shapes(p)))
	for _, s := range Shapes(p) {
		shapes = append(shapes, pg.Shape{Name: s.Name, SQL: s.SQL})
	}
	tr, err := pg.NewTracer(shapes...)
	if err != nil {
		t.Fatalf("compiling the extract allowlist: %v", err)
	}
	return tr
}

// admits reports the shape name the allowlist matched, "" for a refusal. It
// asks the tracer the only way a caller can: by tracing the statement.
func admits(t *testing.T, tr *pg.Tracer, sql string) string {
	t.Helper()
	before := len(tr.Trace())
	ctx := tr.TraceQueryStart(context.Background(), nil, pgx.TraceQueryStartData{SQL: sql})
	trace := tr.Trace()
	if len(trace) != before+1 {
		t.Fatalf("the tracer recorded %d statements for one call", len(trace)-before)
	}
	rec := trace[len(trace)-1]
	if (ctx.Err() == nil) == rec.Refused {
		t.Fatalf("the tracer's context and its record disagree about %q", sql)
	}
	return rec.Shape
}

func lookupTable() ref.TableRef { return ref.TableRef{Schema: "public", Name: "categories"} }
func customers() ref.TableRef   { return ref.TableRef{Schema: "public", Name: "customers"} }
func invoices() ref.TableRef    { return ref.TableRef{Schema: "billing", Name: "invoices"} }
func readings() ref.TableRef    { return ref.TableRef{Schema: "public", Name: "device_readings"} }
func planWithLookup() *pipeline.Plan {
	return &pipeline.Plan{Steps: []pipeline.Step{
		{Table: customers(), Mode: pipeline.ChildOK},
		{Table: lookupTable(), Mode: pipeline.Lookup},
	}}
}

func TestEveryStatementExtractBuildsMatchesAShape(t *testing.T) {
	tr := tracerFor(t, planWithLookup())

	cases := []struct {
		name string
		sql  string
		want string
	}{
		{
			name: "one int8 key",
			sql: rowsSQL(customers(),
				[]string{"id", "email", "created_at"},
				[]string{"id"}, []string{""},
				fakeChunk{casts: []string{"::int8[]"}, cols: []any{[]int64{1, 2}}, n: 2}),
			want: "extract.rows",
		},
		{
			name: "a composite key of text and uuid",
			sql: rowsSQL(invoices(),
				[]string{"code", "uid", "Payload"},
				[]string{"code", "uid"}, []string{"", ""},
				fakeChunk{casts: []string{"::text[]", "::uuid[]"}, cols: []any{[]string{"a"}, []string{"b"}}, n: 1}),
			want: "extract.rows",
		},
		{
			name: "a key that travels as text and is cast back",
			sql: rowsSQL(readings(),
				[]string{"a"},
				[]string{"taken_at"}, []string{"::timestamp with time zone"},
				fakeChunk{casts: []string{"::text[]"}, cols: []any{[]string{"x"}}, n: 1}),
			want: "extract.rows",
		},
		{
			name: "a lookup read, named and bounded",
			sql:  lookupSQL(lookupTable(), []string{"id", "name"}, []string{"id"}),
			want: "extract.lookup.public.categories",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := admits(t, tr, c.sql); got != c.want {
				t.Errorf("admits(%q) = %q, want %q", c.sql, got, c.want)
			}
		})
	}
}

func TestStatementsExtractDoesNotBuildAreRefused(t *testing.T) {
	tr := tracerFor(t, planWithLookup())

	for _, sql := range []string{
		// A whole-table read that is not a lookup's ordered, bounded read.
		`SELECT * FROM "public"."customers" t`,
		// The lookup read with its LIMIT dropped. Without the bound in the
		// template this shape is internal/plan's seed read minus its bound, for
		// every relation, from the moment the two sets sit on one tracer.
		`SELECT t."id", t."name" FROM "public"."categories" t ORDER BY t."id"`,
		// The same read against a table the plan never named. The shape is
		// per-table, so a lookup shape cannot become a licence to read
		// anything else.
		`SELECT t."rolname", t."rolpassword" FROM "pg_catalog"."pg_authid" t ORDER BY t."rolname" LIMIT 1001`,
		// A write dressed as a lookup's select list: SELECT INTO is CREATE
		// TABLE AS, and it reached a shape while a cast's type name could
		// absorb any word (internal/pg/tracer.go reTypeWord).
		`SELECT t."id"::int INTO evil FROM "public"."categories" t ORDER BY t."id" LIMIT 1001`,
		// A lock on the source, which pins rows on a production database.
		`SELECT t."id" FROM "public"."customers" t ` +
			`JOIN unnest($1::int8[]) AS k(k1) ON t."id" = k.k1 ORDER BY t."id" FOR UPDATE`,
		// A write dressed as an extract.
		`INSERT INTO "public"."customers" SELECT t."id" FROM "public"."customers" t ` +
			`JOIN unnest($1::int8[]) AS k(k1) ON t."id" = k.k1 ORDER BY t."id"`,
		`WITH x AS (DELETE FROM "public"."customers" RETURNING *) SELECT * FROM x`,
	} {
		if got := admits(t, tr, sql); got != "" {
			t.Errorf("the extract allowlist matched %q for a statement extract never builds:\n%s", got, sql)
		}
	}
}

// A plan with no lookup step registers no lookup shape, so a lookup read is
// refused. The allowlist is the plan's, not the stage's.
func TestALookupShapeExistsOnlyForALookupStep(t *testing.T) {
	tr := tracerFor(t, &pipeline.Plan{Steps: []pipeline.Step{{Table: customers(), Mode: pipeline.ChildOK}}})
	sql := lookupSQL(lookupTable(), []string{"id", "name"}, []string{"id"})
	if got := admits(t, tr, sql); got != "" {
		t.Errorf("admits(%q) = %q, want a refusal: no step made public.categories a lookup", sql, got)
	}
}

// internal/plan's TestTheComposedAllowlistStillRefusesAnUnboundedRead compiles
// the union of every stage's shapes and pins that the union still refuses an
// unbounded read. It cannot call Shapes(): internal/CLAUDE.md forbids a stage
// package importing another, and that test's own subject is a rule about
// keeping the allowlist narrow, so it carries these two templates as literals.
// This is the other end of that copy — edit a template and this fails, naming
// the file that has to follow.
func TestTheShapeTemplatesAreWhatTheComposedAllowlistTestCopies(t *testing.T) {
	want := map[string]string{
		"extract.rows": `SELECT {selectlist} FROM {ident} t ` +
			`JOIN unnest({casts}) AS k({idents}) ON {keypred} ORDER BY {idents}`,
		"extract.lookup.public.orders": `SELECT {selectlist} FROM "public"."orders" t ORDER BY {idents} LIMIT 1001`,
	}
	got := map[string]string{}
	p := &pipeline.Plan{Steps: []pipeline.Step{
		{Table: ref.TableRef{Schema: "public", Name: "orders"}, Mode: pipeline.Lookup},
	}}
	for _, s := range Shapes(p) {
		got[s.Name] = s.SQL
	}
	for name, sql := range want {
		if got[name] != sql {
			t.Errorf("shape %q is now\n\t%q\nand internal/plan/shapes_test.go's extractShapes still carries\n\t%q\n"+
				"update that copy in the same change", name, got[name], sql)
		}
	}
}
