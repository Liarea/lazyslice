// SPDX-License-Identifier: Apache-2.0

//go:build integration

package load

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Liarea/lazyslice/internal/dsn"
	"github.com/Liarea/lazyslice/internal/extract"
	"github.com/Liarea/lazyslice/internal/introspect"
	"github.com/Liarea/lazyslice/internal/load/ddl"
	"github.com/Liarea/lazyslice/internal/pg"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/plan"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/internal/testutil"
)

// What the target holds after a load is a statement about a real Postgres — the
// recreated schema, the per-table transactions, the sequences, the foreign keys
// — so these run against testdata/pagila through the real introspector, the real
// planner, the real extractor and a real internal/pg on both sides.
//
// Pagila is the fixture ARCHITECTURE.md §11.1 is hardest on: an enum
// (mpaa_rating), two domains (public.year and public."bıgınt", the second with a
// non-ASCII name), twelve sequences none of which the catalog records as owned,
// a partitioned payment table with six leaves, and views, functions and triggers
// v1 does not recreate.

// The environment the kill test's child process reads. It is spawned as this
// same test binary, so the integration build tag and every package below are
// already compiled into it.
const (
	envChild  = "LAZYSLICE_LOAD_CHILD"
	envSource = "LAZYSLICE_LOAD_CHILD_SOURCE"
	envTarget = "LAZYSLICE_LOAD_CHILD_TARGET"
)

// The slice every test here loads. customer is the root CONCEPT.md's own
// example uses, and at 50 rows it reaches the partitioned payment table, the
// enum on film, both domains and eleven of Pagila's fifteen tables.
var (
	rootTable = ref.TableRef{Schema: "public", Name: "customer"}
	take      = 50
)

func planRequest() pipeline.PlanRequest {
	root := rootTable
	return pipeline.PlanRequest{Root: &root, Take: take}
}

// The slice the nasty suite loads. people is the trap table every other table
// of testdata/nasty.sql reaches, and at 25 rows it reaches
// public."LegacyCustomer", whose identity sequence is
// public.LegacyCustomer_CustomerID_seq -- a name that only resolves when it is
// quoted.
var (
	nastyRoot = ref.TableRef{Schema: "public", Name: "people"}
	nastyTake = 25
)

func nastyRequest() pipeline.PlanRequest {
	root := nastyRoot
	return pipeline.PlanRequest{
		Root: &root,
		Take: nastyTake,
		// Trap 12: public.click_stream has no row identity at all, and every run
		// over this fixture carries the flag that drops it (testdata/README.md,
		// internal/plan's own nasty suite).
		Skip: []ref.TableRef{{Schema: "public", Name: "click_stream"}},
	}
}

// classification is a mask-nothing classification. This suite is about the
// loader, so nothing is masked and the rows in the target are the rows in the
// source, which is what makes the row-count assertions exact.
func classification() *pipeline.Classification {
	return &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{}}
}

type source struct {
	url    string
	src    *pg.Source
	reader pipeline.Reader
	schema *pipeline.Schema
	plan   *pipeline.Plan
}

func allowlist() []pg.Shape {
	shapes := make([]pg.Shape, 0, 32)
	for _, s := range introspect.Shapes() {
		shapes = append(shapes, pg.Shape{Name: s.Name, SQL: s.SQL})
	}
	for _, s := range plan.Shapes() {
		shapes = append(shapes, pg.Shape{Name: s.Name, SQL: s.SQL})
	}
	return shapes
}

// openSource loads pagila into a fresh container, introspects it and plans the
// slice. The snapshot stays open until the caller releases it, exactly as a run
// holds it from the start of introspect to the end of extract (ARCHITECTURE.md
// §1).
func openSource(ctx context.Context, t *testing.T, url string, req pipeline.PlanRequest) *source {
	t.Helper()

	src, err := pg.OpenSource(ctx, dsn.DSN(url), allowlist()...)
	if err != nil {
		t.Fatalf("opening the source: %v", err)
	}
	t.Cleanup(src.Close)

	id, err := src.Snapshot(ctx)
	if err != nil {
		t.Fatalf("exporting the snapshot: %v", err)
	}
	t.Cleanup(func() { _ = src.Release(context.WithoutCancel(ctx)) })

	r, err := src.Reader(ctx, id)
	if err != nil {
		t.Fatalf("opening a reader on the snapshot: %v", err)
	}
	t.Cleanup(func() { _ = r.Close(context.WithoutCancel(ctx)) })

	schema, err := introspect.New().Introspect(ctx, r)
	if err != nil {
		t.Fatalf("introspecting: %v", err)
	}
	if bad := ddl.Recreatable(schema); bad != nil {
		t.Fatalf("the fixture is not recreatable, which it must be for this suite to mean anything: %v", bad)
	}
	p, err := plan.New().Plan(ctx, r, schema, classification(), req)
	if err != nil {
		t.Fatalf("planning: %v", err)
	}
	extra := make([]pg.Shape, 0, 8)
	for _, s := range extract.Shapes(p) {
		extra = append(extra, pg.Shape{Name: s.Name, SQL: s.SQL})
	}
	if err := src.Register(extra...); err != nil {
		t.Fatalf("registering extract's shapes: %v", err)
	}
	return &source{url: url, src: src, reader: r, schema: schema, plan: p}
}

// pagilaSource starts a container, loads pagila and analyses it.
func pagilaSource(ctx context.Context, t *testing.T) string {
	t.Helper()
	testutil.SkipWithoutDocker(ctx, t)
	url := testutil.Postgres(ctx, t, "")
	if err := testutil.LoadPagila(ctx, url); err != nil {
		t.Fatalf("loading pagila: %v", err)
	}
	return url
}

// loadInto runs extract → load with no transform between them: this suite is
// about the loader.
// loadInto loads s into targetURL as core would.
//
// gate is what the target gate approved, and it is variadic because most of
// these tests load into a fresh, empty container: the zero Eligibility says
// "approved because it was empty", which is what ARCHITECTURE.md §11.2's
// lock-and-recheck then re-verifies under the ACCESS EXCLUSIVE lock it takes
// before each drop (T-0130). A test that reloads a target this package already
// wrote has to pass the gate's real verdict, because the tables there are full
// by design and the thing that authorises truncating them is the marker row.
func loadInto(ctx context.Context, s *source, targetURL string, gate ...pipeline.Eligibility) (*pipeline.LoadResult, error) {
	target, err := pg.OpenTarget(ctx, dsn.DSN(targetURL))
	if err != nil {
		return nil, err
	}
	defer target.Close()
	w, err := target.Writer(ctx)
	if err != nil {
		return nil, err
	}

	// The marker's source_fingerprint is what §11.2 binds on, so it has to be
	// this source's and not a constant: a run that writes a stranger's
	// fingerprint can never be recognised by the gate as its own.
	_, sourceRef, err := dsn.Parse(s.url)
	if err != nil {
		return nil, err
	}

	batches := make(chan pipeline.RowBatch, 8)
	extractErr := make(chan error, 1)
	go func() {
		extractErr <- extract.New(s.schema).Extract(ctx, s.reader, s.plan, batches)
	}()

	run := Run{
		ToolVersion:               "test",
		SourceFingerprint:         sourceRef.Fingerprint(),
		ClassificationFingerprint: "none",
		SecretFingerprint:         "00000000",
	}
	if len(gate) == 1 {
		run.MarkerBound = gate[0].MarkerBound
		run.MarkerRunID = gate[0].MarkerRunID
		run.MarkerStatus = gate[0].MarkerStatus
	}
	res, loadErr := New(run, nil).Load(ctx, w, s.plan, s.schema, batches)

	if err := <-extractErr; err != nil {
		return res, fmt.Errorf("extract: %w", err)
	}
	return res, loadErr
}

func connect(ctx context.Context, t *testing.T, url string) *pgx.Conn {
	t.Helper()
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close(context.WithoutCancel(ctx)) })
	return conn
}

func scalar[T any](ctx context.Context, t *testing.T, conn *pgx.Conn, sql string, args ...any) T {
	t.Helper()
	var v T
	if err := conn.QueryRow(ctx, sql, args...).Scan(&v); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return v
}

func countRows(ctx context.Context, conn *pgx.Conn, table ref.TableRef) (int64, error) {
	var n int64
	err := conn.QueryRow(ctx, `SELECT count(*) FROM `+pgx.Identifier{table.Schema, table.Name}.Sanitize()).Scan(&n)
	return n, err
}

// expected is the row count §6 item 5 says each step must have in the target:
// exactly Keys.Len() for a keyed step, the source's own count for a lookup.
func expected(ctx context.Context, t *testing.T, s *source, sourceConn *pgx.Conn) map[ref.TableRef]int64 {
	t.Helper()
	out := map[ref.TableRef]int64{}
	for _, step := range s.plan.Steps {
		switch step.Mode {
		case pipeline.SchemaOnly:
			continue
		case pipeline.Lookup:
			n, err := countRows(ctx, sourceConn, step.Table)
			if err != nil {
				t.Fatalf("counting %s in the source: %v", step.Table, err)
			}
			out[step.Table] = n
		case pipeline.ChildOK, pipeline.ParentOnly:
			if step.Keys == nil {
				out[step.Table] = 0
				continue
			}
			out[step.Table] = int64(step.Keys.Len())
		}
	}
	return out
}

// ARCHITECTURE.md §9: an empty target is the case that needs no marker at all.
// This is the whole of §11.1 against the friendly fixture — the object classes
// that are recreated, the ones that are not, the rows, the foreign keys and the
// sequences.
func TestLoadPagilaIntoAnEmptyTarget(t *testing.T) {
	ctx := context.Background()
	sourceURL := pagilaSource(ctx, t)
	targetURL := testutil.Postgres(ctx, t, "")

	s := openSource(ctx, t, sourceURL, planRequest())
	res, err := loadInto(ctx, s, targetURL)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	sourceConn := connect(ctx, t, sourceURL)
	targetConn := connect(ctx, t, targetURL)
	want := expected(ctx, t, s, sourceConn)

	t.Run("every planned table holds exactly its planned rows", func(t *testing.T) {
		for table, n := range want {
			got, err := countRows(ctx, targetConn, table)
			if err != nil {
				t.Errorf("counting %s in the target: %v", table, err)
				continue
			}
			if got != n {
				t.Errorf("%s holds %d rows, the plan says %d", table, got, n)
			}
			if res.Rows[table] != n {
				t.Errorf("the load reports %d rows for %s, the plan says %d", res.Rows[table], table, n)
			}
		}
	})

	t.Run("a schema-only table exists and is empty", func(t *testing.T) {
		var schemaOnly []ref.TableRef
		for _, step := range s.plan.Steps {
			if step.Mode == pipeline.SchemaOnly {
				schemaOnly = append(schemaOnly, step.Table)
			}
		}
		if len(schemaOnly) == 0 {
			t.Skip("this plan has no schema-only step")
		}
		for _, table := range schemaOnly {
			n, err := countRows(ctx, targetConn, table)
			if err != nil {
				t.Errorf("%s was not recreated at all: %v", table, err)
				continue
			}
			if n != 0 {
				t.Errorf("%s is schema-only and holds %d rows", table, n)
			}
		}
	})

	t.Run("every foreign key is present and validated", func(t *testing.T) {
		notValid := scalar[int64](ctx, t, targetConn,
			`SELECT count(*) FROM pg_constraint WHERE contype = 'f' AND NOT convalidated`)
		if notValid != 0 {
			t.Errorf("%d foreign keys in the target are NOT VALID", notValid)
		}
		inSource := scalar[int64](ctx, t, sourceConn, `SELECT count(*) FROM pg_constraint c
JOIN pg_class r ON r.oid = c.conrelid
WHERE c.contype = 'f' AND NOT r.relispartition`)
		inTarget := scalar[int64](ctx, t, targetConn, `SELECT count(*) FROM pg_constraint WHERE contype = 'f'`)
		if inSource != inTarget {
			t.Errorf("the source has %d non-partition foreign keys and the target has %d", inSource, inTarget)
		}
	})

	t.Run("sequences are reset in the strict-NULL form", func(t *testing.T) {
		// pagila records no sequence ownership at all, so this is also the
		// assertion that the setval found its column through the default.
		maxID := scalar[int64](ctx, t, targetConn, `SELECT max(customer_id) FROM public.customer`)
		next := scalar[int64](ctx, t, targetConn, `SELECT nextval('public.customer_customer_id_seq')`)
		if next != maxID+1 {
			t.Errorf("the next customer_id is %d and the highest loaded one is %d", next, maxID)
		}
		// A table the slice left empty must have its sequence unadvanced and
		// about to hand out 1, which is what the third argument buys.
		empty := scalar[bool](ctx, t, targetConn,
			`SELECT NOT is_called FROM public.actor_actor_id_seq`)
		if n, err := countRows(ctx, targetConn, ref.TableRef{Schema: "public", Name: "actor"}); err == nil && n == 0 && !empty {
			t.Error("an empty table's sequence was advanced past 1")
		}
	})

	t.Run("the partitioned table is one plain table", func(t *testing.T) {
		kind := scalar[string](ctx, t, targetConn,
			`SELECT relkind::text FROM pg_class WHERE oid = 'public.payment'::regclass`)
		if kind != "r" {
			t.Errorf("public.payment has relkind %q, not a plain table", kind)
		}
		leaves := scalar[int64](ctx, t, targetConn,
			`SELECT count(*) FROM pg_class WHERE relkind IN ('r', 'p') AND relname LIKE 'payment_p2022%'`)
		if leaves != 0 {
			t.Errorf("%d partitions of payment were recreated", leaves)
		}
		partitioned := scalar[int64](ctx, t, targetConn,
			`SELECT count(*) FROM pg_class WHERE relkind = 'p'`)
		if partitioned != 0 {
			t.Errorf("the target holds %d partitioned tables", partitioned)
		}
	})

	t.Run("nothing v1 does not recreate is in the target", func(t *testing.T) {
		for _, c := range []struct{ what, sql string }{
			{"views", `SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
                       WHERE c.relkind IN ('v', 'm') AND n.nspname = 'public'`},
			{"functions", `SELECT count(*) FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
                       WHERE n.nspname = 'public'`},
			{"triggers", `SELECT count(*) FROM pg_trigger WHERE NOT tgisinternal`},
		} {
			if n := scalar[int64](ctx, t, targetConn, c.sql); n != 0 {
				t.Errorf("the target holds %d %s, which v1 does not recreate", n, c.what)
			}
		}
	})

	t.Run("the enum and the domains came across", func(t *testing.T) {
		// film.rating is public.mpaa_rating and film.release_year is the
		// domain public.year; a value of either reaching the target proves the
		// type was recreated before the table and that CopyFrom could encode
		// it.
		ratings := scalar[int64](ctx, t, targetConn,
			`SELECT count(*) FROM public.film WHERE rating IS NOT NULL`)
		if ratings == 0 {
			t.Error("no film in the target carries an mpaa_rating")
		}
		years := scalar[int64](ctx, t, targetConn,
			`SELECT count(*) FROM public.film WHERE release_year IS NOT NULL`)
		if years == 0 {
			t.Error("no film in the target carries a release_year, which is a domain over integer")
		}
	})

	t.Run("the marker is left running, for core to close once verify passes", func(t *testing.T) {
		// T-0133, 2026-09-14 (THREAT_MODEL.md T8 amendment): a Load that
		// succeeds no longer closes its own row. It used to write
		// StatusComplete here, before whoever called it had a chance to run
		// verify, which is docs/reviews/2026-09-09 finding 4. core.Run is the
		// caller that closes the row now (run.go's closeRun), once verify has
		// actually passed; this test drives Load directly, the way
		// load_integration_test.go always has, so the row it sees is exactly
		// what Load itself leaves behind.
		status := scalar[string](ctx, t, targetConn,
			`SELECT status FROM lazyslice_meta ORDER BY started_at DESC LIMIT 1`)
		if status != pg.StatusRunning {
			t.Errorf("the marker says %q, want %q: closing it is core's job now", status, pg.StatusRunning)
		}
		root := scalar[string](ctx, t, targetConn,
			`SELECT root_table FROM lazyslice_meta ORDER BY started_at DESC LIMIT 1`)
		if root != rootTable.String() {
			t.Errorf("the marker records the root as %q", root)
		}
	})
}

// ARCHITECTURE.md §11.2: a marked target is truncated and reloaded with a
// printed line and no question. The second run is the one that finds the first
// run's types, sequences and tables still there, so it is the run the drop path
// exists for — and the one that fails at 42710 or 42P07 if the drop path misses
// an object class the pre-data DDL creates.
func TestLoadPagilaIntoAMarkedTarget(t *testing.T) {
	ctx := context.Background()
	sourceURL := pagilaSource(ctx, t)
	targetURL := testutil.Postgres(ctx, t, "")

	first := openSource(ctx, t, sourceURL, planRequest())
	firstRes, err := loadInto(ctx, first, targetURL)
	if err != nil {
		t.Fatalf("the first load: %v", err)
	}

	targetConn := connect(ctx, t, targetURL)

	// §11.1: the fingerprint "is the same hash whether computed on the source or
	// on a target we wrote". That is the property §11.2's binding stands on, and
	// it is asserted directly here rather than only through the gate's verdict:
	// lazyslice creates lazyslice_meta in the target itself, and a fingerprint
	// that counted it would differ from the source's by one CREATE TABLE and one
	// ANALYZE, so no marker this package writes could ever bind.
	sourceFP, err := SchemaFingerprint(first.schema)
	if err != nil {
		t.Fatalf("fingerprinting the source: %v", err)
	}
	targetFP := fingerprintOf(ctx, t, targetURL)
	if sourceFP != targetFP {
		t.Errorf("the source fingerprints %s and the target lazyslice just wrote fingerprints %s, "+
			"so §11.2's marker can never bind", sourceFP, targetFP)
	}

	// The gate is what authorises the reload, and it must find the marker bound
	// to this source and this catalog before the second run touches anything.
	// The source ref is the real one the first load recorded: a foreign ref would
	// fail the source_fingerprint comparison before the schema fingerprint was
	// ever reached, and the binding this test exists for would go unexercised.
	_, sourceRef, err := dsn.Parse(sourceURL)
	if err != nil {
		t.Fatalf("parsing the source connection string: %v", err)
	}
	// GateFingerprint is the gate's end of §11.2's binding and the only wiring
	// core does for it, so it is the thing under test here rather than a
	// fingerprinter this test builds for itself: a marker written with one
	// definition and recomputed with another binds nothing.
	//
	// introspect.New() bare, with nothing between it and the gate. It used to be
	// wrapped in an inTransaction introspector that issued its own BEGIN,
	// because internal/pg handed the fingerprinter a pooled connection in
	// autocommit and introspect's SAVEPOINT is 25P01 there; that wrapper was a
	// test standing in for a defect, so the assertion below proved the test's
	// own wiring rather than the run's. internal/pg opens the transaction now
	// (Target.catalogFingerprint), and this call is exactly what core makes.
	target, err := pg.OpenTarget(ctx, dsn.DSN(targetURL), GateFingerprint(introspect.New()))
	if err != nil {
		t.Fatalf("opening the target: %v", err)
	}
	defer target.Close()
	// The source's cluster identity, which internal/core reads from
	// Source.ClusterID and hands the target before every gate call (run.go's
	// openTarget). It is rule 1's second disjunct for a role that cannot
	// execute pg_control_system, and without it the gate cannot tell two
	// containers that both call their database `postgres` from one server
	// reached under two published ports — which is the 2026-09-15 red team's
	// identity-rule-1 attack, and which the gate now fails closed on. Supplying
	// it here is supplying what a real run supplies; leaving it out made this
	// test assert the marker binding through a refusal that fires before the
	// marker is ever read.
	target.SetSourceCluster(scalar[string](ctx, t, connect(ctx, t, sourceURL),
		`SELECT pg_postmaster_start_time()::text || '|'
		     || coalesce(host(inet_server_addr()), '')
		     || '|' || coalesce(inet_server_port()::text, '')`))
	e, err := target.Gate(ctx, sourceRef, "", "")
	if err != nil {
		t.Fatalf("the gate: %v", err)
	}
	if !e.Marked {
		t.Error("the gate does not see a marker on a target this package just wrote")
	}
	if !e.MarkerBound {
		t.Fatalf("the gate does not find its own marker bound, so §11.2 authorises no truncation "+
			"and a target lazyslice wrote can only be reloaded when it is empty; the verdict is %v (%s)",
			e.Verdict, e.Reason)
	}
	if e.Verdict != pipeline.Eligible {
		t.Fatalf("a bound marker did not make the target eligible: %v (%s)", e.Verdict, e.Reason)
	}

	// The second load is authorised by that verdict and not by assumption.
	second := openSource(ctx, t, sourceURL, planRequest())
	// The gate's own verdict, not a constructed one: it is what authorises the
	// truncation, and the loader re-reads the marker row it names under the lock
	// it takes before each drop (§11.2's lock-and-recheck).
	secondRes, err := loadInto(ctx, second, targetURL, e)
	if err != nil {
		t.Fatalf("the second load into the target the first one wrote: %v", err)
	}

	if len(firstRes.Rows) != len(secondRes.Rows) {
		t.Errorf("the first run loaded %d tables and the second %d", len(firstRes.Rows), len(secondRes.Rows))
	}
	for table, n := range firstRes.Rows {
		if secondRes.Rows[table] != n {
			t.Errorf("%s held %d rows after the first run and %d after the second", table, n, secondRes.Rows[table])
		}
	}

	runs := scalar[int64](ctx, t, targetConn, `SELECT count(*) FROM lazyslice_meta`)
	if runs != 2 {
		t.Errorf("the marker holds %d rows after two runs", runs)
	}
	// T-0133: both rows are left at running by Load itself — closing either to
	// complete is core's job, once verify has passed, and this test drives Load
	// directly.
	running := scalar[int64](ctx, t, targetConn,
		`SELECT count(*) FROM lazyslice_meta WHERE status = 'running'`)
	if running != 2 {
		t.Errorf("%d of the two runs are recorded running, want 2", running)
	}
	// The reload must not have doubled anything: one table's rows are the
	// reloaded ones, not the first run's plus the second's.
	n, err := countRows(ctx, targetConn, rootTable)
	if err != nil {
		t.Fatalf("counting the root table: %v", err)
	}
	if n != firstRes.Rows[rootTable] {
		t.Errorf("%s holds %d rows after the reload, and held %d after the first run",
			rootTable, n, firstRes.Rows[rootTable])
	}
}

// fingerprintOf introspects a database and fingerprints it with the same
// function the loader wrote into the marker, which is what the gate recomputes.
func fingerprintOf(ctx context.Context, t *testing.T, url string) string {
	t.Helper()
	src, err := pg.OpenSource(ctx, dsn.DSN(url), allowlist()...)
	if err != nil {
		t.Fatalf("opening %s to fingerprint it: %v", url, err)
	}
	defer src.Close()
	id, err := src.Snapshot(ctx)
	if err != nil {
		t.Fatalf("exporting a snapshot to fingerprint: %v", err)
	}
	defer func() { _ = src.Release(context.WithoutCancel(ctx)) }()
	r, err := src.Reader(ctx, id)
	if err != nil {
		t.Fatalf("opening a reader to fingerprint: %v", err)
	}
	defer func() { _ = r.Close(context.WithoutCancel(ctx)) }()
	schema, err := introspect.New().Introspect(ctx, r)
	if err != nil {
		t.Fatalf("introspecting to fingerprint: %v", err)
	}
	fp, err := SchemaFingerprint(schema)
	if err != nil {
		t.Fatalf("fingerprinting: %v", err)
	}
	return fp
}

// testdata/nasty.sql, not pagila: setval's first argument is a regclass, and
// text is cast to regclass by the rules that parse an identifier, so an
// unquoted name is folded to lower case. public."LegacyCustomer"."CustomerID"
// is GENERATED ALWAYS AS IDENTITY and its sequence is therefore
// public.LegacyCustomer_CustomerID_seq, which resolves only when it is quoted.
// Pagila has no mixed-case sequence, so the friendly fixture cannot see this at
// all -- and the failure lands in the post-data, after every table has been
// copied, leaving a target fully loaded with its sequences unreset: the
// THREAT_MODEL.md T8 outcome the strict-NULL setval exists to prevent.
//
// nasty.sql is also the fixture for the rest of §11.1 at once: a second schema
// (billing) alongside public, two enums, a partitioned table with declarative
// leaves, generated columns, a self-referencing foreign key and a quoted
// mixed-case table with two unique indexes.
func TestLoadNastyResetsAMixedCaseSequence(t *testing.T) {
	ctx := context.Background()
	testutil.SkipWithoutDocker(ctx, t)
	sourceURL := testutil.Postgres(ctx, t, "")
	if err := testutil.LoadNasty(ctx, sourceURL, false); err != nil {
		t.Fatalf("loading nasty.sql: %v", err)
	}
	targetURL := testutil.Postgres(ctx, t, "")

	s := openSource(ctx, t, sourceURL, nastyRequest())
	res, err := loadInto(ctx, s, targetURL)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	targetConn := connect(ctx, t, targetURL)
	legacy := ref.TableRef{Schema: "public", Name: "LegacyCustomer"}

	t.Run("the mixed-case sequence was reset and not skipped", func(t *testing.T) {
		if seqs := res.Sequences[legacy]; len(seqs) == 0 {
			t.Fatalf("no sequence was reset for %s; the load reset %v", legacy, res.Sequences)
		}
		n, err := countRows(ctx, targetConn, legacy)
		if err != nil {
			t.Fatalf("counting %s: %v", legacy, err)
		}
		if n == 0 {
			// The strict-NULL form leaves an empty table's sequence unadvanced.
			if !scalar[bool](ctx, t, targetConn,
				`SELECT NOT is_called FROM public."LegacyCustomer_CustomerID_seq"`) {
				t.Error("an empty table's sequence was advanced past its start")
			}
			return
		}
		maxID := scalar[int32](ctx, t, targetConn, `SELECT max("CustomerID") FROM public."LegacyCustomer"`)
		next := scalar[int64](ctx, t, targetConn, `SELECT nextval('public."LegacyCustomer_CustomerID_seq"')`)
		if next != int64(maxID)+1 {
			t.Errorf("the next CustomerID is %d and the highest loaded one is %d", next, maxID)
		}
	})

	t.Run("every table the plan copies holds exactly its planned rows", func(t *testing.T) {
		sourceConn := connect(ctx, t, sourceURL)
		for table, n := range expected(ctx, t, s, sourceConn) {
			got, err := countRows(ctx, targetConn, table)
			if err != nil {
				t.Errorf("counting %s in the target: %v", table, err)
				continue
			}
			if got != n {
				t.Errorf("%s holds %d rows, the plan says %d", table, got, n)
			}
		}
	})

	t.Run("the second schema and the marker survived", func(t *testing.T) {
		if n := scalar[int64](ctx, t, targetConn,
			`SELECT count(*) FROM pg_namespace WHERE nspname = 'billing'`); n != 1 {
			t.Error("the billing schema was not created in the target")
		}
		// T-0133: Load leaves a successful run's row at StatusRunning for core
		// to close; this test drives Load directly, so that is what it sees.
		status := scalar[string](ctx, t, targetConn,
			`SELECT status FROM lazyslice_meta ORDER BY started_at DESC LIMIT 1`)
		if status != pg.StatusRunning {
			t.Errorf("the marker says %q, want %q", status, pg.StatusRunning)
		}
	})
}

// frameworkMetadataSchema adds T-0314's own two tables on top of nasty.sql for
// the length of one test: schema_migrations and ar_internal_metadata, both
// reached by no foreign key at all, which is the ordinary shape migration
// bookkeeping has. They live here rather than in testdata/nasty.sql because
// that file is shared with introspect, classify and the gate, and each of
// them counts its tables (internal/plan's own extraSchema records the
// identical reasoning for its copy of this same shape).
const frameworkMetadataSchema = `
CREATE TABLE public.schema_migrations (
    version character varying NOT NULL PRIMARY KEY
);

INSERT INTO public.schema_migrations (version) VALUES
    ('20250101000000'), ('20250102000000'), ('20250103000000');

CREATE TABLE public.ar_internal_metadata (
    key        character varying NOT NULL PRIMARY KEY,
    value      character varying,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);

INSERT INTO public.ar_internal_metadata (key, value, created_at, updated_at) VALUES
    ('environment', 'production', now(), now());
`

// loadNastyPlusFrameworkMetadata loads nasty.sql and then frameworkMetadataSchema.
func loadNastyPlusFrameworkMetadata(ctx context.Context, url string) error {
	if err := testutil.LoadNasty(ctx, url, false); err != nil {
		return err
	}
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		return fmt.Errorf("connecting to add the framework metadata tables: %w", err)
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()
	if _, err := conn.Exec(ctx, frameworkMetadataSchema); err != nil {
		return fmt.Errorf("adding the framework metadata tables: %w", err)
	}
	return nil
}

// TestLoadRewritesArInternalMetadataEnvironment is T-0314's pin on this
// package's own half. schema_migrations has no incoming foreign key —
// internal/plan's own T-0314 fix is what still plans it as a Lookup step —
// so this is also the end-to-end proof that such a table is copied whole
// rather than left SchemaOnly: dogfood session 1's whole complaint was zero
// migration rows. ar_internal_metadata's environment row is checked against
// the source's own "production" to prove the rewrite, not merely that
// *some* value ended up in the column.
func TestLoadRewritesArInternalMetadataEnvironment(t *testing.T) {
	ctx := context.Background()
	testutil.SkipWithoutDocker(ctx, t)
	sourceURL := testutil.Postgres(ctx, t, "")
	if err := loadNastyPlusFrameworkMetadata(ctx, sourceURL); err != nil {
		t.Fatalf("loading nasty.sql plus the framework metadata tables: %v", err)
	}
	targetURL := testutil.Postgres(ctx, t, "")

	s := openSource(ctx, t, sourceURL, nastyRequest())
	if _, err := loadInto(ctx, s, targetURL); err != nil {
		t.Fatalf("Load: %v", err)
	}

	sourceConn := connect(ctx, t, sourceURL)
	targetConn := connect(ctx, t, targetURL)

	migrations := ref.TableRef{Schema: "public", Name: "schema_migrations"}

	t.Run("schema_migrations is copied whole, not left SchemaOnly", func(t *testing.T) {
		want, err := countRows(ctx, sourceConn, migrations)
		if err != nil {
			t.Fatalf("counting %s in the source: %v", migrations, err)
		}
		if want == 0 {
			t.Fatalf("the source fixture holds no migration rows; the test proves nothing")
		}
		got, err := countRows(ctx, targetConn, migrations)
		if err != nil {
			t.Fatalf("counting %s in the target: %v", migrations, err)
		}
		if got != want {
			t.Errorf("%s holds %d rows in the target, the source holds %d: a table with no incoming "+
				"foreign key must still be copied whole", migrations, got, want)
		}
	})

	t.Run("ar_internal_metadata's environment was rewritten to development", func(t *testing.T) {
		sourceEnv := scalar[string](ctx, t, sourceConn,
			`SELECT value FROM public.ar_internal_metadata WHERE key = 'environment'`)
		if sourceEnv != "production" {
			t.Fatalf("the source fixture's environment row is %q, want %q; the test proves nothing "+
				"about the rewrite unless the source still says production", sourceEnv, "production")
		}
		targetEnv := scalar[string](ctx, t, targetConn,
			`SELECT value FROM public.ar_internal_metadata WHERE key = 'environment'`)
		if targetEnv != "development" {
			t.Errorf("the target's environment row is %q, want %q: a snapshot's ar_internal_metadata "+
				"must not make a development checkout refuse a destructive task the way the source's "+
				"own %q would", targetEnv, "development", sourceEnv)
		}
	})
}

// testdata/README.md trap 27: the two tables whose column types the driver has
// no codec for until the load gives it one. This is the end-to-end half of
// ARCHITECTURE.md §11.1's "types registered in AfterConnect" — the source read
// through the real extractor, the real DDL, the real registration and a real
// CopyFrom — and it is the test that fails if internal/pg stops registering.
//
// Without registration public.account_statuses.seen fails at 54000 and
// public.settlements.booked at 42804, both mid-table; internal/pg's own
// TestATargetWithoutTypeRegistrationCannotCopyAnEnumArray is that failure held
// against a real server, and this is the same property over the fixture the
// architecture is judged on.
func TestLoadNastyCopiesAnEnumArrayAndACompositeColumn(t *testing.T) {
	ctx := context.Background()
	testutil.SkipWithoutDocker(ctx, t)
	sourceURL := testutil.Postgres(ctx, t, "")
	if err := testutil.LoadNasty(ctx, sourceURL, false); err != nil {
		t.Fatalf("loading nasty.sql: %v", err)
	}
	targetURL := testutil.Postgres(ctx, t, "")

	s := openSource(ctx, t, sourceURL, nastyRequest())
	if _, err := loadInto(ctx, s, targetURL); err != nil {
		t.Fatalf("Load: %s", pg.RenderAnyError(err, false))
	}

	sourceConn := connect(ctx, t, sourceURL)
	targetConn := connect(ctx, t, targetURL)

	// The composite type itself must exist in the target: §11.1 item 3.
	if n := scalar[int64](ctx, t, targetConn,
		`SELECT count(*) FROM pg_type t JOIN pg_namespace n ON n.oid = t.typnamespace
		 WHERE n.nspname = 'public' AND t.typname = 'money_amount' AND t.typtype = 'c'`); n != 1 {
		t.Fatal("public.money_amount was not recreated in the target")
	}

	// Row for row, through the server's own output of each type, so what is
	// compared is the server's opinion and not the driver's.
	for _, q := range []struct {
		name string
		sql  string
	}{
		{
			"public.account_statuses",
			`SELECT coalesce(string_agg(person_id || ' ' || seen::text || ' ' || latest::text, E'\n'
			                            ORDER BY person_id), '<empty>')
			 FROM public.account_statuses`,
		},
		{
			"public.settlements",
			`SELECT coalesce(string_agg(person_id || ' ' || booked::text || ' ' || coalesce(reversed::text, 'NULL'),
			                            E'\n' ORDER BY person_id), '<empty>')
			 FROM public.settlements`,
		},
	} {
		want := scalar[string](ctx, t, sourceConn, q.sql)
		got := scalar[string](ctx, t, targetConn, q.sql)
		if want == "<empty>" {
			t.Fatalf("%s is empty in the source; trap 27 has lost its rows and this test proves nothing", q.name)
		}
		if got != want {
			t.Errorf("%s holds\n%s\nthe source holds\n%s", q.name, got, want)
		}
	}

	// The empty array is one of the three rows and is the one an encode plan
	// gets wrong on its own, so it is named rather than left to the comparison.
	if n := scalar[int64](ctx, t, targetConn,
		`SELECT count(*) FROM public.account_statuses WHERE cardinality(seen) = 0`); n != 1 {
		t.Errorf("%d rows of public.account_statuses hold the empty array, want 1", n)
	}
}

// THREAT_MODEL.md T8, stated as it is: after a kill -9 mid-load the target is
// either empty or carries a marker row at running or failed, and every table in
// it holds either none of its rows or all of them. Load commits one transaction
// per table, so a failure at table five leaves tables one to four committed;
// under SIGKILL no cleanup of ours runs at all, so the row stays at running and
// the next run's gate truncates on that.
//
// The load runs in a child process, because that is the only place a signal that
// runs no deferred function can be delivered. The child is this same test
// binary; the containers are the parent's.
func TestKillNineLeavesEveryTableEmptyOrComplete(t *testing.T) {
	if os.Getenv(envChild) != "" {
		t.Skip("this process is the child")
	}
	ctx := context.Background()
	sourceURL := pagilaSource(ctx, t)
	targetURL := testutil.Postgres(ctx, t, "")

	// The parent plans first, so that it knows what a complete table looks
	// like; the child plans again over the same source and, the walk being
	// deterministic (invariant I3), reaches the same plan.
	s := openSource(ctx, t, sourceURL, planRequest())
	sourceConn := connect(ctx, t, sourceURL)
	want := expected(ctx, t, s, sourceConn)
	order := make([]ref.TableRef, 0, len(s.plan.Steps))
	for _, step := range s.plan.Steps {
		if step.Mode != pipeline.SchemaOnly {
			order = append(order, step.Table)
		}
	}
	if len(order) < 3 {
		t.Fatalf("a slice of %d tables is too small to be killed in the middle of", len(order))
	}
	if err := s.src.Release(ctx); err != nil {
		t.Fatalf("releasing the parent's snapshot: %v", err)
	}

	child := exec.Command(os.Args[0], "-test.run=TestChildLoadsUntilKilled", "-test.v")
	child.Env = append(os.Environ(),
		envChild+"=1", envSource+"="+sourceURL, envTarget+"="+targetURL)
	var out strings.Builder
	child.Stdout, child.Stderr = &out, &out
	if err := child.Start(); err != nil {
		t.Fatalf("starting the child: %v", err)
	}

	targetConn := connect(ctx, t, targetURL)
	killed := waitForFirstCommittedTable(ctx, t, targetConn, order[0], child)
	if err := child.Process.Kill(); err != nil {
		t.Fatalf("killing the child: %v", err)
	}
	_ = child.Wait()
	if !killed {
		t.Fatalf("the child finished the whole load before it could be killed; "+
			"make the slice larger. Its output was:\n%s", out.String())
	}

	// The connection the child was copying through is gone; give the server a
	// moment to roll its open transaction back before counting.
	waitForNoOtherBackend(ctx, t, targetConn)

	var loaded, empty []string
	for _, table := range order {
		got, err := countRows(ctx, targetConn, table)
		if err != nil {
			// A table the child had not recreated yet is not in the target at
			// all, which is the empty half of the property.
			empty = append(empty, table.String())
			continue
		}
		switch got {
		case 0:
			empty = append(empty, table.String())
		case want[table]:
			loaded = append(loaded, table.String())
		default:
			t.Errorf("%s holds %d rows: the plan says %d, and a killed load must leave a table "+
				"with none of its rows or all of them", table, got, want[table])
		}
	}
	sort.Strings(loaded)
	sort.Strings(empty)
	t.Logf("after the kill: %d tables complete (%s), %d empty (%s)",
		len(loaded), strings.Join(loaded, ", "), len(empty), strings.Join(empty, ", "))

	if len(loaded) == 0 {
		t.Fatalf("the child was killed before it committed anything, so this run proves nothing "+
			"about a half-finished load. Its output was:\n%s", out.String())
	}
	if len(empty) == 0 {
		t.Fatalf("the child finished every table before it was killed. Its output was:\n%s", out.String())
	}

	status := scalar[string](ctx, t, targetConn,
		`SELECT status FROM lazyslice_meta ORDER BY started_at DESC LIMIT 1`)
	if status != pg.StatusRunning {
		t.Errorf("the marker says %q after a kill -9; no cleanup of ours runs under SIGKILL, so it must say %q",
			status, pg.StatusRunning)
	}

	// And the property that makes the marker worth writing: the next run is
	// authorised to truncate, and does.
	again := openSource(ctx, t, sourceURL, planRequest())
	// A row still at running is a run that died, and §11.2 says the next run
	// truncates exactly as it would after a complete one — so the reload carries
	// that row as its authorisation, which is what the lock-and-recheck
	// re-verifies before each drop.
	authorised := pipeline.Eligibility{
		MarkerBound: true,
		MarkerRunID: scalar[string](ctx, t, targetConn,
			`SELECT run_id::text FROM lazyslice_meta ORDER BY started_at DESC LIMIT 1`),
		MarkerStatus: pg.StatusRunning,
	}
	if _, err := loadInto(ctx, again, targetURL, authorised); err != nil {
		t.Fatalf("the run after the killed one: %v", err)
	}
	for table, n := range want {
		got, err := countRows(ctx, targetConn, table)
		if err != nil {
			t.Errorf("counting %s after the reload: %v", table, err)
			continue
		}
		if got != n {
			t.Errorf("%s holds %d rows after the reload, the plan says %d", table, got, n)
		}
	}
}

// waitForFirstCommittedTable polls until the first table of the plan holds a
// committed row, which is the earliest moment at which killing the child proves
// anything. It returns false when the child exited first.
func waitForFirstCommittedTable(
	ctx context.Context,
	t *testing.T,
	conn *pgx.Conn,
	first ref.TableRef,
	child *exec.Cmd,
) bool {
	t.Helper()
	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		if child.ProcessState != nil {
			return false
		}
		n, err := countRows(ctx, conn, first)
		if err == nil && n > 0 {
			return true
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("the child never committed %s", first)
	return false
}

// waitForNoOtherBackend waits until the killed child's backend is gone, so that
// the transaction it left open has been rolled back by the server and the row
// counts below are the committed ones.
func waitForNoOtherBackend(ctx context.Context, t *testing.T, conn *pgx.Conn) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		var n int64
		err := conn.QueryRow(ctx,
			`SELECT count(*) FROM pg_stat_activity
              WHERE datname = current_database() AND pid <> pg_backend_pid()`).Scan(&n)
		if err == nil && n == 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("the killed child's backends are still connected")
}

// TestChildLoadsUntilKilled is the child half of the kill test. It is a test
// only so that the parent can re-exec this binary; it does nothing at all in an
// ordinary run.
func TestChildLoadsUntilKilled(t *testing.T) {
	if os.Getenv(envChild) == "" {
		t.Skip("not the child process")
	}
	ctx := context.Background()
	s := openSource(ctx, t, os.Getenv(envSource), planRequest())
	if _, err := loadInto(ctx, s, os.Getenv(envTarget)); err != nil {
		var refusal *Refusal
		if errors.As(err, &refusal) {
			t.Logf("the child's load was refused: %v", refusal)
			return
		}
		t.Logf("the child's load ended: %v", err)
	}
}
