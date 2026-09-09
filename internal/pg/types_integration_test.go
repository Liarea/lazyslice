// SPDX-License-Identifier: Apache-2.0

//go:build integration

package pg

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/Liarea/lazyslice/internal/dsn"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/internal/testutil"
)

// Type registration is a statement about a real driver against a real server:
// what OID the target gives a type, which codec pgx picks for it, and what the
// server does with the bytes. None of that is provable against a fake, so this
// suite starts two containers — two, and not two databases in one, because the
// point is that the target's OIDs are the target's and not the source's.

// userTypeDDL is the schema both databases get. Every class ARCHITECTURE.md
// §11.1 item 3 recreates is here, each as a bare column and as an array.
const userTypeDDL = `
CREATE TYPE public.mood AS ENUM ('sad', 'ok', 'happy');
CREATE TYPE public.money_amount AS (amount numeric(12,2), currency text);
CREATE DOMAIN public.postal AS text CHECK (VALUE <> '');
CREATE TABLE public.t (
    id     int PRIMARY KEY,
    m      public.mood,
    ms     public.mood[],
    c      public.money_amount,
    cs     public.money_amount[],
    p      public.postal,
    ps     public.postal[],
    txt    text[]
);`

// userTypeRows covers a value of every column, a NULL of every column, and the
// empty array — the three shapes an encode plan gets wrong separately.
const userTypeRows = `
INSERT INTO public.t VALUES
    (1, 'sad', ARRAY['sad','happy']::public.mood[],
        ROW(1234.50, 'GBP')::public.money_amount,
        ARRAY[ROW(-99.99,'USD')::public.money_amount],
        'SW1A 1AA'::public.postal, ARRAY['E1 6AN']::public.postal[], ARRAY['a','b']),
    (2, NULL, NULL, NULL, NULL, NULL, NULL, NULL),
    (3, 'ok', '{}'::public.mood[], ROW(0, '')::public.money_amount,
        '{}'::public.money_amount[], 'M1 1AE'::public.postal,
        '{}'::public.postal[], '{}');`

var userTypeCols = []string{"id", "m", "ms", "c", "cs", "p", "ps", "txt"}

// userTypeSchema is what internal/introspect would return for userTypeDDL, in
// the only two fields RegisterTypes reads.
func userTypeSchema() *pipeline.Schema {
	return &pipeline.Schema{
		Enums:      map[string][]string{"public.mood": {"sad", "ok", "happy"}},
		Domains:    []pipeline.NamedDef{{Name: "public.postal", Def: "CREATE DOMAIN public.postal AS text"}},
		Composites: []pipeline.NamedDef{{Name: "public.money_amount", Def: "CREATE TYPE public.money_amount AS (amount numeric(12,2), currency text)"}},
	}
}

func execOn(ctx context.Context, t *testing.T, connURL string, stmts ...string) {
	t.Helper()
	conn, err := pgx.Connect(ctx, connURL)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()
	for _, s := range stmts {
		if _, err := conn.Exec(ctx, s); err != nil {
			t.Fatalf("executing %q: %v", firstWords(s), err)
		}
	}
}

func firstWords(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if len(s) > 60 {
		return s[:60] + "..."
	}
	return s
}

// readTableLikeExtract reads the table the way internal/extract does: through a
// pool in pgx.QueryExecModeExec, scanning each cell into an `any`. That is the
// whole reason the target needs a codec — every value arrives as its text form.
func readTableLikeExtract(ctx context.Context, t *testing.T, sourceURL string) [][]any {
	t.Helper()
	cfg, err := pgx.ParseConfig(sourceURL)
	if err != nil {
		t.Fatalf("parsing the source url: %v", err)
	}
	cfg.DefaultQueryExecMode = pgx.QueryExecModeExec
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("connecting to the source: %v", err)
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()

	rows, err := conn.Query(ctx, `SELECT id, m, ms, c, cs, p, ps, txt FROM public.t ORDER BY id`)
	if err != nil {
		t.Fatalf("reading the source: %v", err)
	}
	defer rows.Close()
	var out [][]any
	for rows.Next() {
		row := make([]any, len(userTypeCols))
		dest := make([]any, len(userTypeCols))
		for i := range row {
			dest[i] = &row[i]
		}
		if err := rows.Scan(dest...); err != nil {
			t.Fatalf("scanning the source: %v", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("reading the source: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("read %d rows from the source, want 3", len(out))
	}
	return out
}

// dumpTable renders the whole table as one string through the server's own
// composite output, so the comparison is the server's opinion of the rows and
// not the driver's.
func dumpTable(ctx context.Context, t *testing.T, connURL string) string {
	t.Helper()
	conn, err := pgx.Connect(ctx, connURL)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()
	var dump string
	if err := conn.QueryRow(ctx,
		`SELECT coalesce(string_agg(t::text, E'\n' ORDER BY id), '<empty>') FROM public.t t`).Scan(&dump); err != nil {
		t.Fatalf("dumping public.t: %v", err)
	}
	return dump
}

func copyRows(ctx context.Context, t *testing.T, w pipeline.Writer, rows [][]any) (int64, error) {
	t.Helper()
	ch := make(chan []any, len(rows))
	for _, r := range rows {
		ch <- r
	}
	close(ch)
	return w.CopyFrom(ctx, ref.TableRef{Schema: "public", Name: "t"}, userTypeCols, ch)
}

// TestATargetWithoutTypeRegistrationCannotCopyAnEnumArray is the negative
// control, and it is what makes the positive test below mean something.
//
// ARCHITECTURE.md §11.1 and ADR-005 say the load registers types in
// AfterConnect. Nothing did until T-0083, and this is the failure that was
// waiting: pgx's COPY is binary, an unregistered OID has no codec, and the
// server reads the text form of an enum array as a binary array header. The
// SQLSTATE is 54000 and the number in the message is the first four bytes of
// `{sad,happy}` read as an int32.
func TestATargetWithoutTypeRegistrationCannotCopyAnEnumArray(t *testing.T) {
	ctx := context.Background()
	testutil.SkipWithoutDocker(ctx, t)

	sourceURL := testutil.Postgres(ctx, t, "")
	targetURL := testutil.Postgres(ctx, t, "")
	execOn(ctx, t, sourceURL, userTypeDDL, userTypeRows)
	execOn(ctx, t, targetURL, userTypeDDL)

	rows := readTableLikeExtract(ctx, t, sourceURL)

	target, err := OpenTarget(ctx, dsn.DSN(targetURL))
	if err != nil {
		t.Fatalf("opening the target: %v", err)
	}
	defer target.Close()
	w, err := target.Writer(ctx)
	if err != nil {
		t.Fatalf("Writer: %v", err)
	}

	// No RegisterTypes call: this is the state the tree was in.
	n, err := copyRows(ctx, t, w, rows)
	if err == nil {
		t.Fatalf("the copy of %d rows succeeded with no type registered; the failure this "+
			"registration exists for is gone and the positive test below proves nothing", n)
	}
	if got := RenderAnyError(err, false); !strings.Contains(got, "54000") {
		t.Errorf("the copy failed with %q, want SQLSTATE 54000 (the enum array read as a binary array header)", got)
	}
	if dump := dumpTable(ctx, t, targetURL); dump != "<empty>" {
		t.Errorf("the target holds %q after the failed copy, want nothing", dump)
	}
}

// TestATargetConnectionCarriesTheSourcesUserTypes is ARCHITECTURE.md §11.1's
// "types registered in AfterConnect", end to end: read the values the way
// extract reads them, register, copy, and require the target to hold what the
// source holds — the composite, the arrays over every user type and the empty
// array included.
func TestATargetConnectionCarriesTheSourcesUserTypes(t *testing.T) {
	ctx := context.Background()
	testutil.SkipWithoutDocker(ctx, t)

	sourceURL := testutil.Postgres(ctx, t, "")
	targetURL := testutil.Postgres(ctx, t, "")
	execOn(ctx, t, sourceURL, userTypeDDL, userTypeRows)
	// Three types created first and dropped again, so that the target's OIDs
	// for mood, money_amount and postal cannot be the source's. A registration
	// that carried the source's OIDs across would pass without this.
	execOn(ctx, t, targetURL,
		`CREATE TYPE public.filler_a AS ENUM ('a')`,
		`CREATE TYPE public.filler_b AS (x int)`,
		`CREATE DOMAIN public.filler_c AS text`,
		userTypeDDL)

	rows := readTableLikeExtract(ctx, t, sourceURL)

	target, err := OpenTarget(ctx, dsn.DSN(targetURL))
	if err != nil {
		t.Fatalf("opening the target: %v", err)
	}
	defer target.Close()
	w, err := target.Writer(ctx)
	if err != nil {
		t.Fatalf("Writer: %v", err)
	}

	if regErr := w.RegisterTypes(ctx, userTypeSchema()); regErr != nil {
		t.Fatalf("RegisterTypes: %v", regErr)
	}

	n, err := copyRows(ctx, t, w, rows)
	if err != nil {
		t.Fatalf("copying into the target: %s", RenderAnyError(err, true))
	}
	if n != 3 {
		t.Errorf("copied %d rows, want 3", n)
	}

	want := dumpTable(ctx, t, sourceURL)
	got := dumpTable(ctx, t, targetURL)
	if got != want {
		t.Errorf("the target holds\n%s\nthe source holds\n%s", got, want)
	}
	if !strings.Contains(want, "1234.50,GBP") {
		t.Errorf("the source dump %q does not show the composite value; this test is not comparing what it says it is", want)
	}
}

// TestRegisteringTypesRetiresConnectionsMadeBeforeTheDDL is the ordering the
// AfterConnect hook cannot fix on its own. pgx runs the hook once per
// connection, and the gate, the drops and the DDL itself all run on target
// connections opened before the types existed. Without the pool reset those
// connections come back out of the pool for the first CopyFrom carrying a type
// map that was built when there was nothing to register.
func TestRegisteringTypesRetiresConnectionsMadeBeforeTheDDL(t *testing.T) {
	ctx := context.Background()
	testutil.SkipWithoutDocker(ctx, t)

	sourceURL := testutil.Postgres(ctx, t, "")
	targetURL := testutil.Postgres(ctx, t, "")
	execOn(ctx, t, sourceURL, userTypeDDL, userTypeRows)

	target, err := OpenTarget(ctx, dsn.DSN(targetURL))
	if err != nil {
		t.Fatalf("opening the target: %v", err)
	}
	defer target.Close()
	w, err := target.Writer(ctx)
	if err != nil {
		t.Fatalf("Writer: %v", err)
	}

	// A connection made before the types exist, exactly as the gate's is, and
	// returned to the pool for the loader to be handed back.
	if err := w.Exec(ctx, `SELECT 1`); err != nil {
		t.Fatalf("warming the pool: %v", err)
	}
	// Then the DDL, on that same pooled connection.
	if err := w.Exec(ctx, userTypeDDL); err != nil {
		t.Fatalf("creating the target schema: %v", err)
	}

	if err := w.RegisterTypes(ctx, userTypeSchema()); err != nil {
		t.Fatalf("RegisterTypes: %v", err)
	}

	if _, err := copyRows(ctx, t, w, readTableLikeExtract(ctx, t, sourceURL)); err != nil {
		t.Fatalf("copying on a pool whose connections predate the DDL: %s", RenderAnyError(err, true))
	}
}

// citextDDL is the schema of the case one unregisterable type used to take the
// whole run down with: a domain and a composite over citext, beside an enum
// array that does need registering.
//
// citext is not an idle choice. ARCHITECTURE.md §11.1 item 2 recreates
// extensions, internal/plan/keyset.go already special-cases citext, and
// `CREATE DOMAIN email AS citext` is the canonical use of it — so this is a
// schema lazyslice supports, and `CREATE DOMAIN d AS money` needs no extension
// at all to do the same thing (measured).
const citextDDL = `
CREATE EXTENSION IF NOT EXISTS citext;
CREATE TYPE public.mood AS ENUM ('sad', 'ok', 'happy');
CREATE DOMAIN public.email AS public.citext;
CREATE TYPE public.tagged AS (label text, tag public.citext);
CREATE TABLE public.t (
    id int PRIMARY KEY,
    ms public.mood[],
    e  public.email
);`

const citextRows = `
INSERT INTO public.t VALUES
    (1, ARRAY['sad','happy']::public.mood[], 'A@Example.test'),
    (2, NULL, NULL);`

// citextSchema is what internal/introspect would return for citextDDL: pgx can
// build a codec for mood and for neither of the other two, because their
// dependency is citext and citext is not in pgx's default type map.
func citextSchema() *pipeline.Schema {
	return &pipeline.Schema{
		Enums:      map[string][]string{"public.mood": {"sad", "ok", "happy"}},
		Domains:    []pipeline.NamedDef{{Name: "public.email", Def: "CREATE DOMAIN public.email AS public.citext"}},
		Composites: []pipeline.NamedDef{{Name: "public.tagged", Def: "CREATE TYPE public.tagged AS (label text, tag public.citext)"}},
	}
}

// TestATypeWithAnUnresolvableDependencyIsSkippedNotFatal is the case that made
// this registration worse than not having it.
//
// pgx's LoadTypes is all-or-nothing over the list it is given: one type whose
// dependency is absent from its default type map ends the call, so the
// AfterConnect hook returned an error, so *every* target connection failed, so
// the run died — after the drop and the pre-data DDL, leaving the operator an
// empty target and a message from inside the driver. Measured before the fix:
// `RegisterTypes` returned `Domain base type OID 16386 was not already
// registered, needed for "email"`, and every `Exec` on the pool after it
// returned the same thing.
//
// What must happen instead: the types that resolve are registered, the ones
// that do not are skipped — which is exactly the state they were in before any
// of this existed — and the load goes on.
func TestATypeWithAnUnresolvableDependencyIsSkippedNotFatal(t *testing.T) {
	ctx := context.Background()
	testutil.SkipWithoutDocker(ctx, t)

	sourceURL := testutil.Postgres(ctx, t, "")
	targetURL := testutil.Postgres(ctx, t, "")
	execOn(ctx, t, sourceURL, citextDDL, citextRows)
	execOn(ctx, t, targetURL, citextDDL)

	cols := []string{"id", "ms", "e"}
	rows := readColumns(ctx, t, sourceURL, cols)

	target, err := OpenTarget(ctx, dsn.DSN(targetURL))
	if err != nil {
		t.Fatalf("opening the target: %v", err)
	}
	defer target.Close()
	w, err := target.Writer(ctx)
	if err != nil {
		t.Fatalf("Writer: %v", err)
	}

	if regErr := w.RegisterTypes(ctx, citextSchema()); regErr != nil {
		t.Fatalf("RegisterTypes over a schema pgx cannot fully resolve: %v\n"+
			"one unregisterable type must not fail the registration: it fails every connection in the pool with it", regErr)
	}
	// The pool is alive. Before the fix this Exec carried the hook's error, and
	// so did every later one — pg.FinishRun could not even close the marker.
	if execErr := w.Exec(ctx, `SELECT 1`); execErr != nil {
		t.Fatalf("the target pool is dead after the registration: %v", execErr)
	}

	// The enum array is registered, so it copies; the two citext-dependent types
	// are not, and the registry says so rather than staying silent about it.
	skipped := target.types.skipped()
	for _, want := range []string{"public.email", "public.tagged"} {
		found := false
		for _, s := range skipped {
			if s == want {
				found = true
			}
		}
		if !found {
			t.Errorf("skipped = %v, want it to name %s", skipped, want)
		}
	}
	for _, s := range skipped {
		if strings.HasSuffix(s, "mood") {
			t.Errorf("skipped = %v, but public.mood resolves on its own; the retry must not "+
				"skip a type whose dependencies pgx has", skipped)
		}
	}

	ch := make(chan []any, len(rows))
	for _, r := range rows {
		ch <- r
	}
	close(ch)
	n, err := w.CopyFrom(ctx, ref.TableRef{Schema: "public", Name: "t"}, cols, ch)
	if err != nil {
		t.Fatalf("copying a table whose enum array registered: %s", RenderAnyError(err, true))
	}
	if n != 2 {
		t.Errorf("copied %d rows, want 2", n)
	}
	if got, want := dumpTable(ctx, t, targetURL), dumpTable(ctx, t, sourceURL); got != want {
		t.Errorf("the target holds\n%s\nthe source holds\n%s", got, want)
	}
}

// readColumns is readTableLikeExtract over a named column list.
func readColumns(ctx context.Context, t *testing.T, sourceURL string, cols []string) [][]any {
	t.Helper()
	cfg, err := pgx.ParseConfig(sourceURL)
	if err != nil {
		t.Fatalf("parsing the source url: %v", err)
	}
	cfg.DefaultQueryExecMode = pgx.QueryExecModeExec
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("connecting to the source: %v", err)
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()

	rows, err := conn.Query(ctx, `SELECT `+strings.Join(cols, ", ")+` FROM public.t ORDER BY id`)
	if err != nil {
		t.Fatalf("reading the source: %v", err)
	}
	defer rows.Close()
	var out [][]any
	for rows.Next() {
		row := make([]any, len(cols))
		dest := make([]any, len(cols))
		for i := range row {
			dest[i] = &row[i]
		}
		if err := rows.Scan(dest...); err != nil {
			t.Fatalf("scanning the source: %v", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("reading the source: %v", err)
	}
	return out
}
