// SPDX-License-Identifier: Apache-2.0

package plan

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pg"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// Shapes() is a promise about sql.go, and this is where the two are held
// together: every statement below is built by the real builder with the
// arguments the walk gives it, and run through a real pg.Tracer carrying
// Shapes() and nothing else. A statement no shape covers gets a cancelled
// context here, which is exactly what it would get on a production source
// (THREAT_MODEL.md T9) — so a clause added in sql.go and not added here fails
// this test rather than the run.

// tracerFor builds the source allowlist the planner runs behind.
func tracerFor(t *testing.T) *pg.Tracer {
	t.Helper()
	shapes := make([]pg.Shape, 0, len(Shapes()))
	for _, s := range Shapes() {
		shapes = append(shapes, pg.Shape{Name: s.Name, SQL: s.SQL})
	}
	tr, err := pg.NewTracer(shapes...)
	if err != nil {
		t.Fatalf("compiling the planner's shapes: %v", err)
	}
	return tr
}

// admits runs one statement past the allowlist and reports the shape that
// matched, or "" when it was refused.
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

// The identity encodings the walk actually produces: a bigint key, a
// character(n) key that travels as text, a uuid key, and a key whose type has
// no array of its own and travels as text with a cast back.
func keyTypes(t *testing.T) (int8Key, bpcharKey, uuidKey, otherKey keyType) {
	t.Helper()
	return typeOf(pipeline.Column{Name: "id", TypeName: "bigint", TypeOID: oidInt8}),
		typeOf(pipeline.Column{Name: "code", TypeName: "character(4)", TypeOID: oidBpchar}),
		typeOf(pipeline.Column{Name: "uid", TypeName: "uuid", TypeOID: oidUUID}),
		typeOf(pipeline.Column{Name: "taken_at", TypeName: "timestamp with time zone", TypeOID: 1184})
}

func TestShapesAreNamedAndDistinct(t *testing.T) {
	seen := make(map[string]bool)
	for _, s := range Shapes() {
		if s.Name == "" || s.SQL == "" {
			t.Errorf("Shapes() carries an incomplete entry: %+v", s)
		}
		if seen[s.Name] {
			t.Errorf("Shapes() carries %q twice", s.Name)
		}
		seen[s.Name] = true
	}
	// Thirteen, since T-POLY: eleven after T-CORE — the whole-catalog
	// unreadable-relation read and the current-role read are gone, because
	// internal/core reads privileges once through Source.Privileges and hands
	// them to the planner on PlanRequest.Priv, and one narrow read came back,
	// plan.unreadable_partition_leaves, because that query excludes partition
	// leaves and §3.3 makes an unreadable leaf the root's refusal (sql.go) —
	// plus §3.2's bounded `_type` sample, which is two shapes and not one:
	// plan.distinct_sample takes a REPEATABLE TABLESAMPLE, and
	// plan.distinct_prefix is the ordered prefix a partitioned or unanalysed
	// relation takes instead (polymorphic.go, sql.go).
	if got, want := len(Shapes()), 13; got != want {
		t.Errorf("Shapes() has %d entries, want %d; a new statement needs a shape", got, want)
	}
}

// Every statement the planner sends, built by the builder that sends it.
func TestEveryPlannerStatementMatchesItsShape(t *testing.T) {
	tr := tracerFor(t)
	i8, bp, uu, other := keyTypes(t)
	orders := ref.TableRef{Schema: "public", Name: "orders"}
	items := ref.TableRef{Schema: "public", Name: `Order "Items"`}

	cases := []struct {
		name string
		sql  string
		want string
	}{
		{
			name: "seed, single integer identity",
			sql:  seedSQL(orders, []string{"id"}, []keyType{i8}, "", 500),
			want: "plan.seed",
		},
		{
			name: "seed, composite identity read as text, quoted table",
			sql:  seedSQL(items, []string{"code", "taken_at"}, []keyType{bp, other}, "", 1),
			want: "plan.seed",
		},
		{
			name: "seed under --where",
			sql: seedSQL(orders, []string{"id"}, []keyType{i8},
				`created_at > now() - interval '30 days' AND status IN ('paid', 'shipped')`, 500),
			want: "plan.seed_where",
		},
		{
			name: "parent step: the MATCH SIMPLE NOT NULL filter",
			sql: mapKeysSQL(items, []string{"id"}, []keyType{i8},
				[]string{"order_id"}, []keyType{i8}, []string{"order_id"}),
			want: "plan.map_keys_not_null",
		},
		{
			name: "key-space translation, composite and mixed casts",
			sql: mapKeysSQL(orders, []string{"code", "uid"}, []keyType{bp, uu},
				[]string{"id", "taken_at"}, []keyType{i8, other}, nil),
			want: "plan.map_keys",
		},
		{
			name: "child step",
			sql: childKeysSQL(items, []string{"order_id"}, []keyType{i8},
				[]string{"id"}, []keyType{i8}, 100),
			want: "plan.child_keys",
		},
		{
			name: "child step, composite edge and a text-out identity",
			sql: childKeysSQL(items, []string{"order_id", "uid"}, []keyType{i8, uu},
				[]string{"code", "taken_at"}, []keyType{bp, other}, 3),
			want: "plan.child_keys",
		},
		{
			name: "bounded lookup count",
			sql:  boundedCountSQL(orders),
			want: "plan.bounded_count",
		},
		{
			name: "--key probe over a whole small table",
			sql:  explicitKeyProbeSQL(orders, []string{"a", "b"}, 0),
			want: "plan.explicit_key_probe",
		},
		{
			name: "--key probe over a prefix",
			sql:  explicitKeyProbeSQL(orders, []string{"a"}, explicitKeyProbeRows),
			want: "plan.explicit_key_probe_bounded",
		},
		{
			name: "pseudo-key probe over a TABLESAMPLE",
			sql:  pseudoKeyProbeSQL(orders, []string{"a", "b"}, false, 100, 100),
			want: "plan.pseudo_key_probe",
		},
		{
			name: "pseudo-key probe over a bounded prefix",
			sql:  pseudoKeyProbeSQL(items, []string{"a"}, true, 0, 0),
			want: "plan.pseudo_key_probe_bounded",
		},
		{
			name: "§3.2's _type sample over a TABLESAMPLE",
			sql: distinctSampleSQL(items, []string{"owner_type"}, []keyType{bp},
				100, 100, probeSampleRows),
			want: "plan.distinct_sample",
		},
		{
			name: "§3.2's _type sample over an ordered prefix",
			sql: distinctPrefixSQL(items, []string{"owner_type"}, []keyType{bp},
				[]string{"id"}, probeSampleRows),
			want: "plan.distinct_prefix",
		},
		{
			name: "§3.2's django_content_type read",
			sql: distinctSampleSQL(orders, djangoContentTypeCols,
				[]keyType{i8, bp, other}, 1, 100, probeSampleRows),
			want: "plan.distinct_sample",
		},
		{
			name: "§3.2's django_content_type read over a composite-ordered prefix",
			sql: distinctPrefixSQL(orders, djangoContentTypeCols,
				[]keyType{i8, bp, other}, []string{"id", "taken_at"}, probeSampleRows),
			want: "plan.distinct_prefix",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := admits(t, tr, c.sql); got != c.want {
				t.Errorf("the allowlist matched %q, want %q, for:\n%s", got, c.want, c.sql)
			}
		})
	}

	if err := tr.Violation(); err != nil {
		t.Errorf("the allowlist refused a statement the planner sends: %v", err)
	}
}

// The other half of the allowlist: a statement outside the grammar is refused
// even when it is built from the same identifiers, and even when it begins with
// an allowed keyword. These are the statements a bug, or a predicate written to
// escape its parentheses, would have to produce.
func TestStatementsOutsideTheGrammarAreStillRefused(t *testing.T) {
	tr := tracerFor(t)
	orders := ref.TableRef{Schema: "public", Name: "orders"}
	i8, _, _, _ := keyTypes(t)

	for _, sql := range []string{
		// A write, and a write wearing a read's first keyword.
		`DELETE FROM "public"."orders" WHERE id = 1`,
		`WITH x AS (DELETE FROM "public"."orders" RETURNING *) SELECT * FROM x`,
		`SELECT lo_import('/etc/passwd')`,
		// The planner's own shapes with a bound removed: an unbounded seed, an
		// unbounded lookup count, an unbounded --key probe, and §3.2's sample
		// without the prefix that bounds it (THREAT_MODEL.md T9).
		`SELECT t."id" AS o1 FROM "public"."orders" t ORDER BY t."id"`,
		`SELECT count(*) FROM "public"."orders"`,
		`SELECT count(*) FROM (SELECT 1 FROM "public"."orders" p GROUP BY "a" HAVING count(*) > 1) s`,
		`SELECT DISTINCT t."owner_type" AS o1 FROM "public"."orders" t ORDER BY o1`,
		// A second statement smuggled onto a statement that does match.
		seedSQL(orders, []string{"id"}, []keyType{i8}, "", 500) + `; DROP TABLE "public"."orders"`,
		// A chunk join whose ON is not a key predicate but a call.
		`SELECT DISTINCT t."id" AS o1 FROM "public"."orders" t ` +
			`JOIN unnest($1::int8[]) AS k(k1) ON pg_sleep(10) IS NOT NULL ORDER BY o1`,
		// A select-list item that is an expression rather than a column.
		`SELECT pg_read_file('/etc/passwd') AS o1 FROM "public"."orders" t ORDER BY t."id" LIMIT 500`,
		// A write dressed as a select list: SELECT INTO is CREATE TABLE AS, and
		// it matched plan.seed while a cast's type name could absorb any run of
		// words (internal/pg's reTypeWord). The READ ONLY transaction refused
		// it, but the allowlist is a control in its own right
		// (THREAT_MODEL.md T9).
		`SELECT t."a"::int INTO evil FROM "public"."orders" t ORDER BY t."id" LIMIT 500`,
	} {
		if got := admits(t, tr, sql); got != "" {
			t.Errorf("the allowlist matched %q for a statement outside the grammar:\n%s", got, sql)
		}
	}

	if tr.Violation() == nil {
		t.Error("Violation() = nil after the allowlist refused every statement above")
	}
}

// The allowlist a run carries is not this package's shapes: a Source has one
// tracer and every stage registers into it additively (pg.Tracer.Register), so
// what holds at run time is the property of the union. The test above compiles
// the planner's shapes alone and cannot see a shape another stage adds that
// admits a statement this package's own shapes refuse — which is what an
// unbounded extract lookup read did to the bounded root read
// (internal/extract/shapes.go, THREAT_MODEL.md T9).
//
// extract's two templates are copied here as literals rather than imported.
// internal/CLAUDE.md's Never list forbids a stage package reaching another
// stage package directly, and this test is the wrong reason to make the first
// exception: what it needs is two strings, not a package. They are pinned on
// the other side by internal/extract's
// TestTheShapeTemplatesAreWhatTheComposedAllowlistTestCopies, which fails if
// either is edited without this copy following.
var extractShapes = []pg.Shape{
	{
		Name: "extract.rows",
		SQL: `SELECT {selectlist} FROM {ident} t ` +
			`JOIN unnest({casts}) AS k({idents}) ON {keypred} ORDER BY {idents}`,
	},
	// A lookup shape is per table and carries its bound as a literal. This is
	// the one extract would register for a plan with a lookup step on
	// public.orders, which is the root of the statements below: the composed
	// allowlist has to refuse an unbounded read of that very table anyway.
	{
		Name: "extract.lookup.public.orders",
		SQL:  `SELECT {selectlist} FROM "public"."orders" t ORDER BY {idents} LIMIT 1001`,
	},
}

func TestTheComposedAllowlistStillRefusesAnUnboundedRead(t *testing.T) {
	shapes := make([]pg.Shape, 0, len(Shapes()))
	for _, s := range Shapes() {
		shapes = append(shapes, pg.Shape{Name: s.Name, SQL: s.SQL})
	}
	shapes = append(shapes, pg.SourceShapes()...)
	shapes = append(shapes, extractShapes...)
	tr, err := pg.NewTracer(shapes...)
	if err != nil {
		t.Fatalf("compiling the composed allowlist: %v", err)
	}

	for _, sql := range []string{
		// The seed with its LIMIT dropped, over the root and over a table the
		// slice never names.
		`SELECT t."id" AS o1 FROM "public"."orders" t ORDER BY t."id"`,
		`SELECT t."rolname", t."rolpassword" FROM "pg_catalog"."pg_authid" t ORDER BY t."rolname"`,
		// The lookup count with its bound dropped.
		`SELECT count(*) FROM "public"."orders"`,
		// A write dressed as a select list.
		`SELECT t."a"::int INTO evil FROM "public"."orders" t ORDER BY t."id" LIMIT 500`,
	} {
		if got := admits(t, tr, sql); got != "" {
			t.Errorf("the composed allowlist matched %q for an unbounded or writing statement:\n%s", got, sql)
		}
	}
}

// --where is the one part of a planner statement a person outside this program
// wrote, so the seed shape is the place its text is bounded. A predicate that
// stops being a predicate — one that ends the statement, or comments the
// template's own LIMIT away — matches no shape and never reaches the server.
// admittedPredicates is what an operator ordinarily writes, and the shape and
// checkWhere both take all of it.
func admittedPredicates() []string {
	return []string{
		`id > 100`,
		`created_at >= now() - interval '30 days'`,
		`status IN ('paid', 'shipped') AND NOT cancelled`,
		`"Total" / 2 > 10`,
		`lower(email) LIKE '%@example.com'`,
		`id % 2 = 0`,
		`id > -1`,
		`id > -(1)`,
		`(a AND (b OR (c AND (d OR (e AND (f))))))`,
	}
}

func TestWherePredicatesThatWouldEscapeTheSeedAreRefused(t *testing.T) {
	tr := tracerFor(t)
	orders := ref.TableRef{Schema: "public", Name: "orders"}
	i8, _, _, _ := keyTypes(t)

	for _, where := range admittedPredicates() {
		sql := seedSQL(orders, []string{"id"}, []keyType{i8}, where, 500)
		if got := admits(t, tr, sql); got != "plan.seed_where" {
			t.Errorf("the allowlist refused an ordinary predicate %q:\n%s", where, sql)
		}
	}

	for name, where := range refusedPredicates() {
		sql := seedSQL(orders, []string{"id"}, []keyType{i8}, where, 500)
		if got := admits(t, tr, sql); got != "" {
			t.Errorf("%s: the allowlist matched %q:\n%s", name, got, sql)
		}
	}
}

// refusedPredicates is the list both halves of the --where rule are measured
// against: the shape (above) and the check that refuses at the flag
// (where.go, below). They are one list because they are one rule, and a rule
// enforced in only one of the two places is the failure each half exists to
// prevent — a confusing refusal from the source, or a violation recorded
// against an operator's typing.
func refusedPredicates() map[string]string {
	return map[string]string{
		"a line comment hiding the LIMIT":  `id > 0) --`,
		"a block comment hiding the LIMIT": `id > 0) /*`,
		"a second statement":               `id > 0); DELETE FROM users`,
		"a second statement in a CTE":      `id > 0) UNION ALL SELECT 1; UPDATE users SET email = 'x'`,
		"a dollar-quoted string":           `name = $$o'brien$$`,
		"a backslash escape":               `name = E'o\'brien'`,
		"a bind parameter of its own":      `id = $9`,
		// Every parenthesis in the finished statement is paired, so nothing but
		// the predicate's own balance refuses this: it closes the template's
		// `WHERE (`, appends a union over a table the slice never named, and
		// opens a parenthesis for the template's `)` to close.
		"a balanced break-out appending a clause": `id > 0) UNION ALL SELECT c."pan" AS o1 FROM "public"."cards" c WHERE (true`,
		"an unclosed parenthesis":                 `id IN (SELECT id FROM users`,
		// An ordinary predicate an operator would write, which the exclusions
		// refuse: the regex carries both a backslash and a dollar sign. It is
		// here to say that the cost of the exclusions is real, and it is the
		// case checkWhere exists to name.
		"a regex with an escape and an anchor": `email ~ '^\w+@example\.com$'`,
		"nesting past the shape's depth":       `((((((( id > 0 )))))))`,
	}
}

// What the exclusions do not buy. A balanced predicate is still the operator's
// own SQL: it can call a function and it can carry a subquery, and what bounds
// it is the READ ONLY transaction, not the allowlist. These are admitted on
// purpose and pinned here so that the next reader inherits the guarantee the
// code makes rather than a larger one (internal/pg/tracer.go's {where}).
func TestWhatTheWhereExclusionsDoNotStop(t *testing.T) {
	tr := tracerFor(t)
	orders := ref.TableRef{Schema: "public", Name: "orders"}
	i8, _, _, _ := keyTypes(t)

	for name, where := range map[string]string{
		// READ ONLY refuses lo_import(); it does not refuse a function that
		// opens its own connection.
		"a function call in the predicate": `id = lo_import('/etc/passwd')`,
		// An unbounded aggregate inside the holder transaction, which the
		// planner's own probes are bounded to keep out (THREAT_MODEL.md T9).
		"a subquery with an unbounded aggregate": `(SELECT count(*) FROM huge) > 0`,
	} {
		sql := seedSQL(orders, []string{"id"}, []keyType{i8}, where, 500)
		if got := admits(t, tr, sql); got != "plan.seed_where" {
			t.Errorf("%s: the allowlist matched %q, want plan.seed_where; if this is now refused, "+
				"say so in internal/pg/tracer.go rather than deleting the case:\n%s", name, got, sql)
		}
	}
}

// checkWhere is the same rule read at the flag, and it has to be at least as
// strict as the shape: a predicate it lets through and the allowlist then
// refuses is the confusing refusal it exists to prevent, and one the allowlist
// admits and it refuses would turn a working --where into an exit 2.
func TestTheWhereCheckAndTheShapeAgree(t *testing.T) {
	tr := tracerFor(t)
	orders := ref.TableRef{Schema: "public", Name: "orders"}
	i8, _, _, _ := keyTypes(t)

	for name, where := range refusedPredicates() {
		r := checkWhere(where)
		if r == nil {
			t.Errorf("%s: checkWhere admitted a predicate the allowlist refuses", name)
			continue
		}
		if r.Code != CodeWhereSyntax || r.Exit != exitUsage {
			t.Errorf("%s: checkWhere returned %s exit %d, want %s exit %d",
				name, r.Code, r.Exit, CodeWhereSyntax, exitUsage)
		}
		if r.Args[event.ArgFlag] != whereFlag || r.Args[event.ArgReason] == "" || r.Args[event.ArgCount] == "" {
			t.Errorf("%s: checkWhere's args are %v, want the flag, the reason and the position", name, r.Args)
		}
	}

	for _, where := range append(admittedPredicates(),
		`id = lo_import('/etc/passwd')`,
		`(SELECT count(*) FROM huge) > 0`,
	) {
		if r := checkWhere(where); r != nil {
			t.Errorf("checkWhere refused %q the allowlist admits: %v", where, r)
		}
		sql := seedSQL(orders, []string{"id"}, []keyType{i8}, where, 500)
		if got := admits(t, tr, sql); got != "plan.seed_where" {
			t.Errorf("the allowlist matched %q for a predicate checkWhere admits:\n%s", got, sql)
		}
	}

	// No --where is not a predicate, and seedSQL writes no WHERE clause for it.
	if r := checkWhere(""); r != nil {
		t.Errorf("checkWhere refused the empty predicate: %v", r)
	}
}
