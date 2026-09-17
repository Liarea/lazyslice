// SPDX-License-Identifier: Apache-2.0

//go:build integration

package pg

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Liarea/lazyslice/internal/dsn"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/internal/testutil"
)

// The gate is a statement about a real Postgres: what a catalog query returns
// on a database that has had its migrations run, what SELECT EXISTS says under
// row-level security, what current_database() and system_identifier are on a
// second database in the same container. None of that is provable against a
// fake, so this suite starts one.
//
// One container holds every case. The gate's own emptiness rule is per-database
// and the databases here are created per case, so they do not see each other.
type gateFixture struct {
	sourceURL      string
	sourceRef      dsn.Ref
	sourceSystemID string
	base           *url.URL
}

func newGateFixture(ctx context.Context, t *testing.T) gateFixture {
	t.Helper()
	testutil.SkipWithoutDocker(ctx, t)

	sourceURL := testutil.Postgres(ctx, t, "")
	_, sourceRef, err := dsn.Parse(sourceURL)
	if err != nil {
		t.Fatalf("parsing the container url: %v", err)
	}
	src, err := OpenSource(ctx, dsn.DSN(sourceURL))
	if err != nil {
		t.Fatalf("opening the source: %v", err)
	}
	defer src.Close()
	systemID, err := src.SystemID(ctx)
	if err != nil {
		t.Fatalf("reading the source system identifier: %v", err)
	}
	if systemID == "" {
		t.Fatal("the container's superuser could not read pg_control_system, so the same-cluster cases would not be exercised")
	}

	base, err := testutil.URL(sourceURL)
	if err != nil {
		t.Fatalf("parsing the container url: %v", err)
	}
	return gateFixture{sourceURL: sourceURL, sourceRef: sourceRef, sourceSystemID: systemID, base: base}
}

// database creates a database on the same cluster as the source and returns its
// url. A second database on one cluster is the common compose setup and is the
// case ARCHITECTURE.md §9 rule 1 says is eligible.
func (f gateFixture) database(ctx context.Context, t *testing.T, name string, stmts ...string) string {
	t.Helper()
	f.exec(ctx, t, f.sourceURL, `CREATE DATABASE `+name)

	u := *f.base
	u.Path = "/" + name
	target := u.String()
	if len(stmts) > 0 {
		f.exec(ctx, t, target, stmts...)
	}
	return target
}

func (f gateFixture) exec(ctx context.Context, t *testing.T, connURL string, stmts ...string) {
	t.Helper()
	pool, err := Connect(ctx, dsn.DSN(connURL), nil)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	defer pool.Close()
	for _, s := range stmts {
		if _, err := pool.Exec(ctx, s); err != nil {
			t.Fatalf("executing %q: %v", s, err)
		}
	}
}

func (f gateFixture) gate(ctx context.Context, t *testing.T, targetURL string, opts ...TargetOption) pipeline.Eligibility {
	t.Helper()
	return f.gateAllowing(ctx, t, targetURL, "", opts...)
}

// gateAllowing is gate with a value for --allow-remote-target, which is the
// only argument of Gate that the ordinary cases leave empty.
func (f gateFixture) gateAllowing(ctx context.Context, t *testing.T, targetURL, allowRemoteHost string, opts ...TargetOption) pipeline.Eligibility {
	t.Helper()
	target, err := OpenTarget(ctx, dsn.DSN(targetURL), opts...)
	if err != nil {
		t.Fatalf("opening the target: %v", err)
	}
	defer target.Close()

	e, err := target.Gate(ctx, f.sourceRef, f.sourceSystemID, allowRemoteHost)
	if err != nil {
		t.Fatalf("Gate: %v", err)
	}
	return e
}

func TestGateAcceptsAnEmptyDatabaseOnTheSameCluster(t *testing.T) {
	ctx := context.Background()
	f := newGateFixture(ctx, t)

	e := f.gate(ctx, t, f.database(ctx, t, "gate_empty"))

	if e.Verdict != pipeline.Eligible {
		t.Fatalf("Verdict = %v (%s), want Eligible", e.Verdict, e.Reason)
	}
	// A second database on the same cluster is eligible, and the header carries
	// "same cluster as source" as a warning line, never a refusal.
	if !e.SameCluster {
		t.Error("SameCluster = false for a database in the same container as the source")
	}
	if !e.Local {
		t.Error("Local = false for a database on loopback")
	}
	if e.Marked || e.MarkerBound {
		t.Errorf("Marked = %v, MarkerBound = %v on a database we have never written to", e.Marked, e.MarkerBound)
	}
}

// Rows in a table that has had its migrations run are migration state, not
// data; rows anywhere else are somebody's database (ADR-005 "Target").
func TestGateExemptsMigrationBookkeepingAndRefusesEverythingElse(t *testing.T) {
	ctx := context.Background()
	f := newGateFixture(ctx, t)

	migrated := f.database(ctx, t, "gate_migrated",
		`CREATE TABLE schema_migrations (version bigint PRIMARY KEY)`,
		`INSERT INTO schema_migrations VALUES (20260905120000)`,
		`CREATE TABLE _prisma_migrations (id text PRIMARY KEY)`,
		`INSERT INTO _prisma_migrations VALUES ('4f2c')`,
		`CREATE TABLE customers (id bigint PRIMARY KEY, email text)`,
	)
	if e := f.gate(ctx, t, migrated); e.Verdict != pipeline.Eligible {
		t.Fatalf("a migrated-but-empty database: Verdict = %v (%s), want Eligible", e.Verdict, e.Reason)
	}

	populated := f.database(ctx, t, "gate_populated",
		`CREATE TABLE schema_migrations (version bigint PRIMARY KEY)`,
		`INSERT INTO schema_migrations VALUES (20260905120000)`,
		`CREATE TABLE customers (id bigint PRIMARY KEY, email text)`,
		`INSERT INTO customers VALUES (1, 'alice@example.com')`,
		`CREATE TABLE orders (id bigint PRIMARY KEY)`,
		// The exemption is on a name in schema public. A table wearing one of
		// those names in another schema is somebody's data, and the exemption
		// widens in the fail-open direction, so it stops at public.
		`CREATE SCHEMA reporting`,
		`CREATE TABLE reporting.schema_migrations (version bigint PRIMARY KEY, note text)`,
		`INSERT INTO reporting.schema_migrations VALUES (1, 'quarterly rollup')`,
	)
	e := f.gate(ctx, t, populated)
	if e.Verdict != pipeline.Refused {
		t.Fatalf("a database with one row in customers: Verdict = %v, want Refused", e.Verdict)
	}
	if e.Reason != CodeNotEmpty {
		t.Errorf("Reason = %q, want %q", e.Reason, CodeNotEmpty)
	}
	if _, ok := e.RowCounts[ref.TableRef{Schema: "public", Name: "customers"}]; !ok {
		t.Errorf("RowCounts = %v, want it to name public.customers", e.RowCounts)
	}
	if _, ok := e.RowCounts[ref.TableRef{Schema: "public", Name: "orders"}]; ok {
		t.Error("RowCounts names public.orders, which is empty")
	}
	if _, ok := e.RowCounts[ref.TableRef{Schema: "public", Name: "schema_migrations"}]; ok {
		t.Error("RowCounts names schema_migrations, which the emptiness rule exempts")
	}
	if _, ok := e.RowCounts[ref.TableRef{Schema: "reporting", Name: "schema_migrations"}]; !ok {
		t.Errorf("RowCounts = %v, want it to name reporting.schema_migrations: the exemption is "+
			"for the bookkeeping a framework writes in public, not for a name in any schema", e.RowCounts)
	}
}

// The transposed-arguments report is real (research/COMPLAINTS.md PII-13), and
// this is the rule that catches it: refuse iff the normalised host:port/database
// is the source's, or the system identifier and the database name both are.
func TestGateRefusesTheSourceItself(t *testing.T) {
	ctx := context.Background()
	f := newGateFixture(ctx, t)

	e := f.gate(ctx, t, f.sourceURL)
	if e.Verdict != pipeline.Refused {
		t.Fatalf("Verdict = %v, want Refused: the target is the source", e.Verdict)
	}
	if e.Reason != CodeSameDatabase {
		t.Errorf("Reason = %q, want %q", e.Reason, CodeSameDatabase)
	}

	// Same cluster, same database, reached by another spelling of the host: the
	// endpoint comparison and the system_identifier comparison each catch it on
	// their own, and the gate needs only one of them.
	spelled := *f.base
	spelled.Host = strings.Replace(spelled.Host, "127.0.0.1", "localhost", 1)
	if spelled.Host != f.base.Host {
		if e := f.gate(ctx, t, spelled.String()); e.Verdict != pipeline.Refused || e.Reason != CodeSameDatabase {
			t.Errorf("the source under another spelling of loopback: Verdict = %v (%s), want Refused/%s",
				e.Verdict, e.Reason, CodeSameDatabase)
		}
	}
}

// Rule 1 is a disjunction, and this is the half SameEndpoint cannot answer: the
// source reached under a name that does not normalise to the one the source was
// reached by — a pooler alias, or a second DNS name in front of one cluster.
// Only system_identifier plus current_database() refuses it, so hard-coding that
// half to false would pass every other case in this file.
func TestGateRefusesTheSourceUnderAnotherName(t *testing.T) {
	ctx := context.Background()
	f := newGateFixture(ctx, t)

	pool, err := Connect(ctx, dsn.DSN(f.sourceURL), nil)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	defer pool.Close()

	aliased := dsn.Ref{
		Host:     "db.internal.example.com",
		Port:     5432,
		Database: f.sourceRef.Database,
		User:     f.sourceRef.User,
	}
	if same, sameErr := aliased.SameEndpoint(f.sourceRef); sameErr != nil || same {
		t.Fatalf("SameEndpoint(alias, source) = %v, %v; the case needs an alias the endpoint "+
			"comparison misses, or it proves nothing about the other half of rule 1", same, sameErr)
	}

	// The connection is the source's; the reference is not one the endpoint
	// comparison can match. local is true so that rule 2 cannot be what refuses.
	target := &Target{pool: pool, ref: aliased, local: true}
	e, err := target.Gate(ctx, f.sourceRef, f.sourceSystemID, "")
	if err != nil {
		t.Fatalf("Gate: %v", err)
	}
	if e.Verdict != pipeline.Refused || e.Reason != CodeSameDatabase {
		t.Fatalf("Verdict = %v (%s), want Refused/%s: the target is the source under another name",
			e.Verdict, e.Reason, CodeSameDatabase)
	}
}

// Row-level security is "not empty" by rule and never by probe: under FORCE ROW
// LEVEL SECURITY with no matching policy, EXISTS returns false over rows the
// session cannot see while DROP TABLE still succeeds. The obvious probe lies,
// which is the whole reason the rule exists (ARCHITECTURE.md §9 rule 5).
func TestGateRefusesRLSTable(t *testing.T) {
	ctx := context.Background()
	f := newGateFixture(ctx, t)

	// audit_log holds no rows at all, so the only thing that can name it is the
	// relrowsecurity branch.
	rls := f.database(ctx, t, "gate_rls",
		`CREATE TABLE audit_log (id bigint PRIMARY KEY, actor text)`,
		`ALTER TABLE audit_log ENABLE ROW LEVEL SECURITY`,
		`ALTER TABLE audit_log FORCE ROW LEVEL SECURITY`,
		`CREATE TABLE orders (id bigint PRIMARY KEY)`,
	)

	e := f.gate(ctx, t, rls)
	if e.Verdict != pipeline.Refused {
		t.Fatalf("Verdict = %v (%s), want Refused", e.Verdict, e.Reason)
	}
	if e.Reason != CodeNotEmpty {
		t.Errorf("Reason = %q, want %q", e.Reason, CodeNotEmpty)
	}
	audit := ref.TableRef{Schema: "public", Name: "audit_log"}
	if got, ok := e.RowCounts[audit]; !ok || got != RowsNotCounted {
		t.Errorf("RowCounts = %v, want it to name %s: a table with row-level security is not empty by rule",
			e.RowCounts, audit)
	}
	if _, ok := e.RowCounts[ref.TableRef{Schema: "public", Name: "orders"}]; ok {
		t.Error("RowCounts names public.orders, which is empty and carries no row-level security")
	}
}

// Above the cap the target is refused outright with the count printed. It is
// never a sample of 2,000 and never a pass, because Verdict has no state in
// which "not probed" reads as eligible (ARCHITECTURE.md §9 rule 5).
func TestGateRefusesTargetAboveTableCap(t *testing.T) {
	ctx := context.Background()
	f := newGateFixture(ctx, t)

	over := f.database(ctx, t, "gate_table_cap", fmt.Sprintf(
		`DO $cap$ BEGIN FOR i IN 1..%d LOOP EXECUTE format('CREATE TABLE t%%s ()', i); END LOOP; END $cap$`,
		TableCap+1))

	e := f.gate(ctx, t, over)
	if e.Verdict != pipeline.Refused {
		t.Fatalf("Verdict = %v (%s), want Refused", e.Verdict, e.Reason)
	}
	if e.Reason != CodeTableCap {
		t.Errorf("Reason = %q, want %q", e.Reason, CodeTableCap)
	}
	if e.TableCount != TableCap+1 {
		t.Errorf("TableCount = %d, want %d: the refusal prints the count", e.TableCount, TableCap+1)
	}
	if len(e.RowCounts) != 0 {
		t.Errorf("RowCounts = %v, want none: above the cap nothing is probed", e.RowCounts)
	}
}

// A freshly provisioned remote database is empty and would pass every other
// rule, which is why locality is a rule of its own (ARCHITECTURE.md §9 rule 2,
// THREAT_MODEL.md T2). The flag admits such a target; it does not make it local,
// and the decision header is the operator's only visible signal that the write
// is leaving this machine.
func TestGateRefusesRemoteTargetWithoutFlag(t *testing.T) {
	ctx := context.Background()
	f := newGateFixture(ctx, t)

	// WithLocal carries the answer the connection string cannot give. False is
	// what discovery computes for a candidate that is neither loopback nor a
	// container of this project.
	remote := f.database(ctx, t, "gate_remote")

	e := f.gate(ctx, t, remote, WithLocal(false))
	if e.Verdict != pipeline.Refused {
		t.Fatalf("Verdict = %v (%s), want Refused: no --allow-remote-target names the host", e.Verdict, e.Reason)
	}
	if e.Reason != CodeRemote {
		t.Errorf("Reason = %q, want %q", e.Reason, CodeRemote)
	}
	if e.Local {
		t.Error("Local = true for a target the gate refused as remote")
	}

	host := f.base.Hostname()
	allowed := f.gateAllowing(ctx, t, remote, host, WithLocal(false))
	if allowed.Verdict != pipeline.Eligible {
		t.Fatalf("with --allow-remote-target %s: Verdict = %v (%s), want Eligible",
			host, allowed.Verdict, allowed.Reason)
	}
	if allowed.Local {
		t.Error("Local = true for a remote target admitted by --allow-remote-target; the flag " +
			"permits the write, it does not move the database")
	}
}

// A marker authorises truncation only when it is bound to this source and this
// catalog (ARCHITECTURE.md §11.2). The same populated database is eligible with
// the binding and refused without it, which is the whole of the control.
func TestGateAcceptsABoundMarkerAndIgnoresAnUnboundOne(t *testing.T) {
	ctx := context.Background()
	f := newGateFixture(ctx, t)

	const catalogFP = "5f2c9e0b1a7d4c83"
	fingerprinter := WithCatalogFingerprint(func(context.Context, pipeline.Reader) (string, error) {
		return catalogFP, nil
	})

	marked := f.database(ctx, t, "gate_marked",
		MarkerDDL,
		`CREATE TABLE customers (id bigint PRIMARY KEY, email text)`,
		`INSERT INTO customers VALUES (1, 'alice@example.com')`,
	)
	f.exec(ctx, t, marked, fmt.Sprintf(
		`INSERT INTO %s (run_id, tool_version, schema_version, started_at, status,
		    source_fingerprint, source_system_id, schema_fingerprint,
		    classification_fingerprint, root_table, take, secret_fingerprint)
		 VALUES ('%s', '0.1.0', %d, now(), '%s', '%s', '%s', '%s', 'c1a55', 'public.customers', 500, '5ecec7')`,
		MarkerTable, mustUUID(t), MarkerSchemaVersion, StatusRunning,
		f.sourceRef.Fingerprint(), f.sourceSystemID, catalogFP))

	e := f.gate(ctx, t, marked, fingerprinter)
	if e.Verdict != pipeline.Eligible {
		t.Fatalf("a bound marker: Verdict = %v (%s), want Eligible", e.Verdict, e.Reason)
	}
	if !e.Marked || !e.MarkerBound {
		t.Errorf("Marked = %v, MarkerBound = %v; want both true", e.Marked, e.MarkerBound)
	}
	if e.PrevKeyFP != "5ecec7" || e.PrevClassFP != "c1a55" {
		t.Errorf("PrevKeyFP = %q, PrevClassFP = %q; want the marker's own fingerprints", e.PrevKeyFP, e.PrevClassFP)
	}

	// A row still at "running" is a run that died. It authorises the truncation
	// exactly as a complete one does, which the case above has just shown; what
	// must not authorise anything is a catalog that has changed since we wrote
	// it, a marker from another source, or no way to check either.
	unbound := WithCatalogFingerprint(func(context.Context, pipeline.Reader) (string, error) {
		return "0000000000000000", nil
	})
	for name, opt := range map[string]TargetOption{
		"the catalog has changed since the marker was written": unbound,
		"nothing can recompute the fingerprint":                WithLocal(true),
	} {
		e := f.gate(ctx, t, marked, opt)
		if e.Verdict != pipeline.Refused {
			t.Errorf("%s: Verdict = %v, want Refused", name, e.Verdict)
		}
		if !e.Marked || e.MarkerBound {
			t.Errorf("%s: Marked = %v, MarkerBound = %v; want true, false", name, e.Marked, e.MarkerBound)
		}
		if e.Reason != CodeNotEmpty {
			t.Errorf("%s: Reason = %q, want the emptiness refusal it falls through to (%q)", name, e.Reason, CodeNotEmpty)
		}
	}

	// A marker beside an otherwise empty database is the fall-through's other
	// half: unbound authorises nothing, and nothing is what it has to refuse.
	empty := f.database(ctx, t, "gate_marker_only", MarkerDDL)
	f.exec(ctx, t, empty, fmt.Sprintf(
		`INSERT INTO %s (run_id, tool_version, schema_version, started_at, status,
		    source_fingerprint, schema_fingerprint, classification_fingerprint,
		    root_table, take, secret_fingerprint)
		 VALUES ('%s', '0.1.0', %d, now(), '%s', 'deadbeefdeadbeef', 'nope', 'c1a55', 'public.customers', 500, '5ecec7')`,
		MarkerTable, mustUUID(t), MarkerSchemaVersion, StatusComplete))

	if e := f.gate(ctx, t, empty, fingerprinter); e.Verdict != pipeline.Eligible || e.MarkerBound {
		t.Errorf("a foreign marker beside an empty database: Verdict = %v (%s), MarkerBound = %v; want Eligible and unbound",
			e.Verdict, e.Reason, e.MarkerBound)
	}
}

// The gate opens the transaction the fingerprinter runs in, and that BEGIN is
// pinned here — in this package's own suite, on this package's connection —
// rather than left to internal/load's, where it was found.
//
// The fingerprinter below takes a SAVEPOINT through the reader it is handed,
// which is what internal/introspect's sampler does to survive a table the role
// cannot read. Outside a transaction block a SAVEPOINT is 25P01, so on a pooled
// connection in autocommit the fingerprinter failed every time, ARCHITECTURE.md
// §11.2's binding could never be confirmed, and a target lazyslice itself wrote
// was refused with exit 4. Deleting the BEGIN from Target.catalogFingerprint
// fails this test with that SQLSTATE and nothing else here.
func TestGateRunsTheFingerprinterInsideATransaction(t *testing.T) {
	ctx := context.Background()
	f := newGateFixture(ctx, t)

	const catalogFP = "9c1d7b4e2a05f386"
	var (
		called       bool
		savepointErr error
	)
	fingerprinter := WithCatalogFingerprint(func(ctx context.Context, r pipeline.Reader) (string, error) {
		called = true
		// Reader has only Query (ARCHITECTURE.md §2), and a server error can
		// surface on Rows.Err rather than on Query, so both are consulted —
		// which is how internal/introspect runs the same statement.
		rows, err := r.Query(ctx, `SAVEPOINT lazyslice_gate_probe`)
		if err == nil {
			for rows.Next() {
			}
			rows.Close()
			err = rows.Err()
		}
		if err != nil {
			savepointErr = err
			return "", err
		}
		return catalogFP, nil
	})

	marked := f.database(ctx, t, "gate_fingerprint_tx",
		MarkerDDL,
		`CREATE TABLE customers (id bigint PRIMARY KEY, email text)`,
		`INSERT INTO customers VALUES (1, 'alice@example.com')`,
	)
	f.exec(ctx, t, marked, fmt.Sprintf(
		`INSERT INTO %s (run_id, tool_version, schema_version, started_at, status,
		    source_fingerprint, source_system_id, schema_fingerprint,
		    classification_fingerprint, root_table, take, secret_fingerprint)
		 VALUES ('%s', '0.1.0', %d, now(), '%s', '%s', '%s', '%s', 'c1a55', 'public.customers', 500, '5ecec7')`,
		MarkerTable, mustUUID(t), MarkerSchemaVersion, StatusComplete,
		f.sourceRef.Fingerprint(), f.sourceSystemID, catalogFP))

	// Opened here rather than through the fixture's gate helper so that the
	// SAVEPOINT's own error is what this test reports when it fails.
	target, err := OpenTarget(ctx, dsn.DSN(marked), fingerprinter)
	if err != nil {
		t.Fatalf("opening the target: %v", err)
	}
	defer target.Close()

	e, gateErr := target.Gate(ctx, f.sourceRef, f.sourceSystemID, "")

	var pgErr *pgconn.PgError
	if errors.As(savepointErr, &pgErr) && pgErr.Code == "25P01" {
		t.Fatalf("the fingerprinter's SAVEPOINT came back 25P01 (%s): the gate ran it outside a "+
			"transaction block, so no marker can ever bind and a target lazyslice wrote is refused",
			pgErr.Message)
	}
	if savepointErr != nil {
		t.Fatalf("the fingerprinter's SAVEPOINT failed: %v", savepointErr)
	}
	if !called {
		t.Fatal("the fingerprinter never ran, so this test proved nothing about the transaction around it")
	}
	if gateErr != nil {
		t.Fatalf("Gate: %v", gateErr)
	}
	if e.Verdict != pipeline.Eligible || !e.MarkerBound {
		t.Errorf("Verdict = %v (%s), MarkerBound = %v; want Eligible and bound, which is what a "+
			"fingerprinter that could take its SAVEPOINT returns", e.Verdict, e.Reason, e.MarkerBound)
	}
}

// The allowlist, not the READ ONLY transaction, is what has to refuse this: a
// read-only transaction returns SQLSTATE 25006 from the server, which means the
// statement reached production (THREAT_MODEL.md T9).
func TestSourceTracerRefusesAWriteBeforeItReachesTheServer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	testutil.SkipWithoutDocker(ctx, t)

	sourceURL := testutil.Postgres(ctx, t, "")
	src, err := OpenSource(ctx, dsn.DSN(sourceURL))
	if err != nil {
		t.Fatalf("opening the source: %v", err)
	}
	defer src.Close()

	// Seeded outside the source handle, because the source handle is the thing
	// that cannot write.
	pool, err := Connect(ctx, dsn.DSN(sourceURL), nil)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	defer pool.Close()
	for _, s := range []string{
		`CREATE TABLE customers (id bigint PRIMARY KEY, email text)`,
		`INSERT INTO customers VALUES (1, 'alice@example.com')`,
	} {
		if _, seedErr := pool.Exec(ctx, s); seedErr != nil {
			t.Fatalf("seeding: %v", seedErr)
		}
	}

	r, err := src.Short(ctx)
	if err != nil {
		t.Fatalf("Short: %v", err)
	}
	defer func() { _ = r.Close(context.WithoutCancel(ctx)) }()

	for _, sql := range []string{
		`DELETE FROM customers`,
		`WITH gone AS (DELETE FROM customers RETURNING *) SELECT * FROM gone`,
		`UPDATE customers SET email = 'x'`,
	} {
		_, err := r.Query(ctx, sql)
		if err == nil {
			t.Fatalf("the source ran %q", sql)
		}
		if !errors.Is(err, ErrRefused) {
			t.Errorf("%q failed with %v; that is not the allowlist refusing, so the statement "+
				"may have reached the server", sql, err)
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			t.Errorf("%q came back with SQLSTATE %s, which means the server saw it", sql, pgErr.Code)
		}
	}

	if src.Violation() == nil {
		t.Error("Violation() = nil after three refused statements")
	}
	refused := 0
	for _, s := range src.Trace() {
		if s.Refused {
			refused++
		}
		if strings.Contains(s.SQL, "alice@example.com") {
			t.Errorf("the trace records a value: %q", s.SQL)
		}
	}
	if refused != 3 {
		t.Errorf("the trace holds %d refusals, want 3", refused)
	}

	// The rows are still there, which is the point.
	var n int64
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM customers`).Scan(&n); err != nil {
		t.Fatalf("counting: %v", err)
	}
	if n != 1 {
		t.Errorf("customers holds %d rows, want 1", n)
	}
}

func mustUUID(t *testing.T) string {
	t.Helper()
	id, err := uuidV4()
	if err != nil {
		t.Fatalf("uuidV4: %v", err)
	}
	return id
}

// The 2026-09-15 red team's identity-rule-1 attack. ARCHITECTURE.md §9 rule 1
// is a disjunction, and only the system_identifier half survives aliasing: the
// same physical server reached under two published ports, two host spellings or
// a pooler name normalises to a different endpoint every time. EXECUTE on
// pg_control_system is not granted to PUBLIC, so under the SELECT-only source
// role §9 itself recommends that half answers "" and the endpoint comparison
// stands alone — and the red team dropped a production table through the gap.
//
// The source here is the container's own database under an endpoint spelling
// that does not match (a second published port onto one server is exactly this
// shape), with no system identifier on either side — which is what the
// recommended role produces. The only thing left that can tell the truth is the
// cluster identity an ordinary role can read.
func TestGateRefusesAnAliasedSourceWithNoSystemIdentifier(t *testing.T) {
	ctx := context.Background()
	f := newGateFixture(ctx, t)

	// The source, reached under a port the endpoint comparison will not match.
	aliased := f.sourceRef
	aliased.Port = f.sourceRef.Port + 1

	target, err := OpenTarget(ctx, dsn.DSN(f.sourceURL))
	if err != nil {
		t.Fatalf("opening the target: %v", err)
	}
	defer target.Close()
	target.SetSourceCluster(f.clusterID(ctx, t, f.sourceURL))

	e, err := target.Gate(ctx, aliased, "", "")
	if err != nil {
		t.Fatalf("Gate: %v", err)
	}
	if e.Verdict != pipeline.Refused || e.Reason != CodeSameDatabase {
		t.Fatalf("Verdict = %v (%s), want Refused/%s: the target is the source under a second "+
			"endpoint spelling, and a run that passes here drops the production tables",
			e.Verdict, e.Reason, CodeSameDatabase)
	}
}

// The other side of the same rule: two genuinely different clusters whose
// databases happen to share a name. That is the ordinary case — a production
// database called `app` and a local one called `app` — and it must stay
// eligible, or the fail-closed arm above would refuse nearly every real run
// made with the recommended role.
func TestGateAcceptsASameNamedDatabaseOnADifferentCluster(t *testing.T) {
	ctx := context.Background()
	f := newGateFixture(ctx, t)

	other := testutil.Postgres(ctx, t, "")
	_, otherRef, err := dsn.Parse(other)
	if err != nil {
		t.Fatalf("parsing the second container url: %v", err)
	}
	if otherRef.Database != f.sourceRef.Database {
		t.Skipf("the two containers do not share a database name (%q and %q), "+
			"so this case is not the one being tested", otherRef.Database, f.sourceRef.Database)
	}

	target, err := OpenTarget(ctx, dsn.DSN(other))
	if err != nil {
		t.Fatalf("opening the target: %v", err)
	}
	defer target.Close()
	target.SetSourceCluster(f.clusterID(ctx, t, f.sourceURL))

	// No system identifier on either side: the SELECT-only role §9 recommends.
	e, err := target.Gate(ctx, f.sourceRef, "", "")
	if err != nil {
		t.Fatalf("Gate: %v", err)
	}
	if e.Verdict != pipeline.Eligible {
		t.Fatalf("Verdict = %v (%s), want Eligible: a database of the same name on a different "+
			"cluster is an ordinary target", e.Verdict, e.Reason)
	}
	if e.SameCluster {
		t.Error("SameCluster = true for a database in a different container")
	}
}

// clusterID is what internal/core reads from Source.ClusterID and hands the
// target before every gate call.
func (f gateFixture) clusterID(ctx context.Context, t *testing.T, connURL string) string {
	t.Helper()
	src, err := OpenSource(ctx, dsn.DSN(connURL))
	if err != nil {
		t.Fatalf("opening the source: %v", err)
	}
	defer src.Close()
	id, err := src.ClusterID(ctx)
	if err != nil {
		t.Fatalf("reading the source cluster identity: %v", err)
	}
	if id == "" {
		t.Fatal("the cluster identity is empty, so the case this test is about cannot be exercised")
	}
	return id
}

// R2-06, 2026-09-15. One cluster, two published endpoints: the container's own
// port, and a second route to the same server (testutil.SecondEndpoint). The
// cluster identity §9 rule 1 leans on must be the same over both, and the gate
// must refuse the source reached over the second one.
//
// **This test does not discriminate the defect it was written for, and must not
// be cited as the thing that pins it** (T-0190 fix round, 2026-09-15). Restore
// the pre-fix sqlClusterID — pg_postmaster_start_time() with inet_server_addr()
// and inet_server_port() — and it still passes, by construction:
// testutil.SecondEndpoint dials the same backend host:port the direct
// connection uses, so the *server's* end of the socket, which is exactly what
// inet_server_addr()/inet_server_port() report, is identical over both routes;
// two published docker ports NAT to the container's 5432 the same way. The
// container suite cannot reach the container over a genuinely different
// server-side socket, so it cannot reproduce the transport difference at all.
// What pins that class is cluster_test.go's check on the statement's own text.
//
// What this test is still worth keeping for is the end-to-end half: an alias
// the gate has to see through, and a refusal on the source reached under a
// second spelling.
//
// Both sides are gated with no system identifier, which is what the SELECT-only
// source role §9 recommends produces: pg_control_system is not executable by
// PUBLIC, so the alias is refused on the ordinary-role identity or not at all.
func TestOneClusterReachedOverTwoEndpointsHasOneClusterIdentity(t *testing.T) {
	ctx := context.Background()
	f := newGateFixture(ctx, t)

	second := testutil.SecondEndpoint(ctx, t, f.sourceURL)

	direct := f.clusterID(ctx, t, f.sourceURL)
	aliased := f.clusterID(ctx, t, second)
	if direct != aliased {
		t.Fatalf("the cluster identity is %q over the container's own endpoint and %q over a second "+
			"endpoint onto the same server: rule 1's alias arm is defeated by the spelling, and a "+
			"run pointed at the source over the second endpoint drops the source's own tables",
			direct, aliased)
	}

	_, secondRef, err := dsn.Parse(second)
	if err != nil {
		t.Fatalf("parsing the second endpoint: %v", err)
	}
	if secondRef.Port == f.sourceRef.Port && secondRef.Host == f.sourceRef.Host {
		t.Fatalf("the second endpoint %s:%d is the container's own, so nothing is aliased here",
			secondRef.Host, secondRef.Port)
	}

	// The attack: --source names the cluster one way, --target names the same
	// database on the same cluster the other way.
	target, err := OpenTarget(ctx, dsn.DSN(second))
	if err != nil {
		t.Fatalf("opening the target over the second endpoint: %v", err)
	}
	defer target.Close()
	target.SetSourceCluster(direct)

	e, err := target.Gate(ctx, f.sourceRef, "", "")
	if err != nil {
		t.Fatalf("Gate: %v", err)
	}
	if e.Verdict != pipeline.Refused || e.Reason != CodeSameDatabase {
		t.Fatalf("Verdict = %v (%s), want Refused/%s: the target is the source reached over a "+
			"second endpoint", e.Verdict, e.Reason, CodeSameDatabase)
	}
	if !e.SameCluster {
		t.Error("SameCluster = false for the source's own cluster reached over a second endpoint")
	}
}

// The transport is not the only thing a cluster identity can accidentally
// depend on: a *session* can render one cluster value two ways. TimeZone is set
// per role (ALTER ROLE) and per database (ALTER DATABASE), and lazyslice reads
// the source with the SELECT-only role §9 recommends and the target with a
// different role against a different database, so the two sides differing is
// ordinary rather than adversarial. With the start time cast to text, one
// postmaster answered the two sessions with two identities, the alias arm read
// two clusters, and the gate admitted the source's own database on the source's
// own cluster (T-0190 fix round, 2026-09-15).
//
// Unlike the second-endpoint test above, this one discriminates: it fails
// against a sqlClusterID that casts the timestamptz.
func TestOneClusterReadUnderTwoSessionTimeZonesHasOneClusterIdentity(t *testing.T) {
	ctx := context.Background()
	f := newGateFixture(ctx, t)

	utc := f.clusterID(ctx, t, f.sourceURL)

	f.exec(ctx, t, f.sourceURL, `ALTER DATABASE `+f.sourceRef.Database+` SET TimeZone = 'Asia/Tokyo'`)
	t.Cleanup(func() {
		f.exec(context.WithoutCancel(ctx), t, f.sourceURL,
			`ALTER DATABASE `+f.sourceRef.Database+` RESET TimeZone`)
	})

	tokyo := f.clusterID(ctx, t, f.sourceURL)
	if utc != tokyo {
		t.Fatalf("one cluster answered %q to a session in the default time zone and %q to a session "+
			"in Asia/Tokyo: the identity is rendered by the session, so a source and a target whose "+
			"roles or databases carry different TimeZone settings read one cluster as two and the "+
			"gate admits the source's own database", utc, tokyo)
	}
}

// T-0222, 2026-09-16. The round-3 replay of R2-06
// (docs/reviews/2026-09-15-redteam/round3-still-leaking.json): a hardened
// cluster that revokes monitoring functions from PUBLIC denies the source
// role EXECUTE on pg_postmaster_start_time() too — pg_control_system was
// already unreadable by an ordinary role, which is the premise the original
// finding assumed. Before this fix, sqlClusterID was one SELECT
// concatenating that field with three others, so the permission error on it
// failed the whole row: Source.ClusterID collapsed to "" even though the
// maintenance database's oid and the server version, neither privileged,
// were still readable. With the identity gone on both sides of the
// comparison, the gate's rule 1 fell back to comparing the two endpoints'
// spelling — and a second endpoint onto the same server (testutil.
// SecondEndpoint, this red team's own shape) reads as a different server, so
// an empty database named differently from the source's own (the
// mid-migration case: "app" reached directly, "newprod" reached over a
// second route to the same cluster) passed rule 1 with no same-cluster
// warning at all.
//
// This test creates exactly the role the replay describes, confirms
// ClusterID still answers under it, and then runs the gate against that
// second-endpoint, differently-named target the way the red team did.
func TestGateRecognisesTheSourceClusterWithPostmasterStartTimeDenied(t *testing.T) {
	ctx := context.Background()
	f := newGateFixture(ctx, t)

	const role = "gate_no_start_time"
	f.exec(ctx, t, f.sourceURL,
		`CREATE ROLE `+role+` LOGIN PASSWORD 'lazyslice' NOSUPERUSER`,
		`GRANT CONNECT ON DATABASE `+f.sourceRef.Database+` TO `+role,
		// EXECUTE on pg_control_system() is granted to PUBLIC by default on
		// every currently supported Postgres (measured against postgres:16;
		// ARCHITECTURE.md §9 and THREAT_MODEL.md T2 both say otherwise and are
		// owed a correction — filed as T-0224), so it is revoked here to
		// reach the SELECT-only shape those sections describe.
		// pg_postmaster_start_time() genuinely is PUBLIC-executable by
		// default; this REVOKE is the replay's own hardening step on top.
		`REVOKE EXECUTE ON FUNCTION pg_control_system() FROM PUBLIC`,
		`REVOKE EXECUTE ON FUNCTION pg_postmaster_start_time() FROM PUBLIC`,
	)
	t.Cleanup(func() {
		f.exec(context.WithoutCancel(ctx), t, f.sourceURL,
			`GRANT EXECUTE ON FUNCTION pg_control_system() TO PUBLIC`,
			`GRANT EXECUTE ON FUNCTION pg_postmaster_start_time() TO PUBLIC`)
	})

	restricted := *f.base
	restricted.User = url.UserPassword(role, "lazyslice")

	src, err := OpenSource(ctx, dsn.DSN(restricted.String()))
	if err != nil {
		t.Fatalf("opening the source as %s: %v", role, err)
	}
	defer src.Close()

	sourceSystemID, err := src.SystemID(ctx)
	if err != nil {
		t.Fatalf("SystemID: %v", err)
	}
	if sourceSystemID != "" {
		t.Fatalf("SystemID = %q for a role with no EXECUTE on pg_control_system, want \"\"", sourceSystemID)
	}

	sourceCluster, err := src.ClusterID(ctx)
	if err != nil {
		t.Fatalf("ClusterID: %v", err)
	}
	// Assert the degradation field by field, not only that the joined string
	// is non-empty: field 0 (the start time) must be lost, because this role
	// has no EXECUTE on pg_postmaster_start_time(), and fields 1 and 3 (the
	// maintenance database's oid and the server version) must survive it,
	// because neither needs any privilege this role lacks (fix round, review
	// — a stub that always returned some non-empty constant passed the old
	// "!= \"\"" check unchanged).
	fields := strings.Split(sourceCluster, clusterIDSep)
	if len(fields) <= clusterIDServerVersionField {
		t.Fatalf("ClusterID = %q, want at least %d positional fields", sourceCluster, clusterIDServerVersionField+1)
	}
	if fields[0] != "" {
		t.Errorf("ClusterID start-time field = %q, want empty: this role has no EXECUTE on "+
			"pg_postmaster_start_time()", fields[0])
	}
	if fields[clusterIDMaintenanceOIDField] == "" || fields[clusterIDServerVersionField] == "" {
		t.Fatalf("ClusterID = %q: the maintenance database's oid (field %d) and the server version "+
			"(field %d) need no privilege this role lacks, and must not be lost along with the start "+
			"time (T-0222)", sourceCluster, clusterIDMaintenanceOIDField, clusterIDServerVersionField)
	}

	// The mid-migration shape: an empty database named differently from the
	// source's, reached over a second route to the same server.
	newprod := f.database(ctx, t, "gate_newprod")
	second := testutil.SecondEndpoint(ctx, t, newprod)

	target, err := OpenTarget(ctx, dsn.DSN(second))
	if err != nil {
		t.Fatalf("opening the target over the second endpoint: %v", err)
	}
	defer target.Close()
	target.SetSourceCluster(sourceCluster)

	e, err := target.Gate(ctx, f.sourceRef, sourceSystemID, "")
	if err != nil {
		t.Fatalf("Gate: %v", err)
	}
	if !e.SameCluster {
		// This case is the fields-agree-or-are-missing shape: both sides can
		// only fill the two weak fields here, and they agree, so
		// sameClusterIdentity reports unknown (fix round, review — weak-field
		// agreement is not evidence of sameness) and sameClusterVerdict's
		// unknown-defaults-true arm is what actually answers true. It is not
		// evidence that the split can tell this cluster apart from a
		// different one; TestGateDistinguishesAGenuinelyDifferentClusterFromWeakFieldsAlone
		// below is the case where the fields actually decide.
		t.Fatal("SameCluster = false: gate_newprod is an empty database on the source's own " +
			"cluster, reached over a second endpoint, with the source read under a role denied " +
			"EXECUTE on pg_postmaster_start_time() — nothing here is comparable but the two weak " +
			"fields, and they agree, so this must fail closed as \"possibly same cluster\" rather " +
			"than fall back to the endpoint spelling and answer \"different cluster\"")
	}
	if e.Verdict != pipeline.Eligible {
		t.Fatalf("Verdict = %v (%s), want Eligible: an empty database on the source's own cluster "+
			"is eligible by design (ARCHITECTURE.md §9 rule 1) — only SameCluster should carry the "+
			"warning", e.Verdict, e.Reason)
	}
}

// T-0222 fix round, 2026-09-16 (review). The test above only reaches the
// shape where the surviving weak fields (the maintenance database's oid and
// the server version) happen to agree, because both reads are of one cluster
// — and sameClusterVerdict's unknown-defaults-true arm answers SameCluster =
// true in that shape with or without the T-0222 split: it is also what a
// reverted has_function_privilege split, or a readClusterID with the
// privilege check deleted outright, would produce, since either leaves
// sourceCluster == "" and clusterKnown == false the same way an empty
// identity always has.
//
// This is the one outcome only the split can produce: a genuinely different
// cluster, read under the same role denied EXECUTE on
// pg_postmaster_start_time(), whose surviving weak fields disagree. The
// server version does that on its own between any two different Postgres
// majors, so the second cluster here is a postgres:14 container rather than
// a sibling of the source's own postgres:16 — testutil/CLAUDE.md's own rule
// ("a test that wants two clusters starts a second container"). Neither side
// can read a system identifier or a start time, so sameClusterIdentity has
// only the two weak fields to compare, and it must read their disagreement as
// "different cluster" on its own — the fail-open default arm cannot produce
// false here, because clusterKnown is true and clusterSame is false the
// moment any field, weak or not, disagrees.
func TestGateDistinguishesAGenuinelyDifferentClusterFromWeakFieldsAlone(t *testing.T) {
	ctx := context.Background()
	f := newGateFixture(ctx, t)

	const role = "gate_no_start_time_2"
	f.exec(ctx, t, f.sourceURL,
		`CREATE ROLE `+role+` LOGIN PASSWORD 'lazyslice' NOSUPERUSER`,
		`GRANT CONNECT ON DATABASE `+f.sourceRef.Database+` TO `+role,
		`REVOKE EXECUTE ON FUNCTION pg_control_system() FROM PUBLIC`,
		`REVOKE EXECUTE ON FUNCTION pg_postmaster_start_time() FROM PUBLIC`,
	)
	t.Cleanup(func() {
		f.exec(context.WithoutCancel(ctx), t, f.sourceURL,
			`GRANT EXECUTE ON FUNCTION pg_control_system() TO PUBLIC`,
			`GRANT EXECUTE ON FUNCTION pg_postmaster_start_time() TO PUBLIC`)
	})

	restricted := *f.base
	restricted.User = url.UserPassword(role, "lazyslice")

	src, err := OpenSource(ctx, dsn.DSN(restricted.String()))
	if err != nil {
		t.Fatalf("opening the source as %s: %v", role, err)
	}
	defer src.Close()

	sourceSystemID, err := src.SystemID(ctx)
	if err != nil {
		t.Fatalf("SystemID: %v", err)
	}
	if sourceSystemID != "" {
		t.Fatalf("SystemID = %q for a role with no EXECUTE on pg_control_system, want \"\"", sourceSystemID)
	}

	sourceCluster, err := src.ClusterID(ctx)
	if err != nil {
		t.Fatalf("ClusterID: %v", err)
	}
	fields := strings.Split(sourceCluster, clusterIDSep)
	if len(fields) <= clusterIDServerVersionField || fields[clusterIDServerVersionField] == "" {
		t.Fatalf("ClusterID = %q, want a non-empty server-version field (%d)", sourceCluster, clusterIDServerVersionField)
	}

	// A second, genuinely different cluster: postgres:14 rather than the
	// source's own postgres:16, so the server version — the one weak field
	// this role can read on both sides — is certain to disagree.
	otherURL := testutil.Postgres(ctx, t, "postgres:14")
	target, err := OpenTarget(ctx, dsn.DSN(otherURL))
	if err != nil {
		t.Fatalf("opening the target: %v", err)
	}
	defer target.Close()
	target.SetSourceCluster(sourceCluster)

	e, err := target.Gate(ctx, f.sourceRef, sourceSystemID, "")
	if err != nil {
		t.Fatalf("Gate: %v", err)
	}
	if e.SameCluster {
		t.Fatal("SameCluster = true: the target is a postgres:14 container, genuinely different " +
			"from the postgres:16 source, and its server version disagrees with the source's own " +
			"under a role that can read no other field — sameClusterIdentity's weak-field " +
			"disagreement must decide \"different cluster\" on its own here, not fall back to the " +
			"unknown-defaults-true arm the test above exercises")
	}
}

// T-0222 fix round, 2026-09-16 (review). readClusterID takes a SAVEPOINT
// around the guarded start-time read only when it runs inside an open
// transaction — Source.ClusterID's case — because a failed
// sqlClusterIDStartTime otherwise leaves that transaction aborted:
// sqlClusterIDRest, sent on it next, would itself fail with 25P02 "current
// transaction is aborted" and be read as "unreadable", losing the whole
// identity rather than the one field the privilege check could not
// guarantee.
//
// This forces exactly that failure on a role that genuinely has EXECUTE on
// pg_postmaster_start_time() — the privilege check truthfully answers yes —
// by shadowing the built-in to_char(timestamp, text) that sqlClusterIDStartTime
// calls with one that raises. Postgres always searches pg_catalog first
// unless it is named explicitly elsewhere in search_path, so putting public
// ahead of pg_catalog and defining to_char(timestamp, text) there is enough
// to make the guarded statement fail with no privilege involved at all — a
// stand-in for "the guarded read failed for a reason unrelated to the
// privilege check", which is what the fix must survive.
func TestReadClusterIDRecoversFromAFailedStartTimeReadInsideATransaction(t *testing.T) {
	ctx := context.Background()
	f := newGateFixture(ctx, t)

	f.exec(ctx, t, f.sourceURL,
		`CREATE FUNCTION public.to_char(timestamp, text) RETURNS text AS $$
         BEGIN RAISE EXCEPTION 'shadowed to_char for the T-0222 fix-round test'; END;
         $$ LANGUAGE plpgsql`,
	)
	t.Cleanup(func() {
		f.exec(context.WithoutCancel(ctx), t, f.sourceURL,
			`DROP FUNCTION public.to_char(timestamp, text)`)
	})

	pool, err := Connect(ctx, dsn.DSN(f.sourceURL), nil)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	defer pool.Close()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquiring a connection: %v", err)
	}
	defer conn.Release()

	if _, err = conn.Exec(ctx, `SET search_path = public, pg_catalog`); err != nil {
		t.Fatalf("setting search_path so public.to_char shadows the built-in: %v", err)
	}
	if _, err = conn.Exec(ctx, sqlBeginReadOnly); err != nil {
		t.Fatalf("opening the transaction: %v", err)
	}
	defer conn.Exec(context.WithoutCancel(ctx), sqlRollback)

	id, err := readClusterID(ctx, conn, true)
	if err != nil {
		t.Fatalf("readClusterID: %v", err)
	}
	fields := strings.Split(id, clusterIDSep)
	if len(fields) < 4 {
		t.Fatalf("readClusterID = %q, want at least 4 positional fields", id)
	}
	if fields[0] != "" {
		t.Errorf("start time field = %q, want empty: the shadowed to_char should have failed the "+
			"guarded read", fields[0])
	}
	if fields[1] == "" || fields[3] == "" {
		t.Fatalf("readClusterID = %q: the maintenance oid (field 1) and server version (field 3) "+
			"need no privilege the shadowed to_char failure touches, and must not be lost along with "+
			"the start time — recovering them is what the SAVEPOINT this fix adds is for", id)
	}

	// The transaction the caller is still holding must be usable: a plain
	// statement sent on it after readClusterID must not fail with 25P02,
	// which is what an unrecovered abort would do to every later statement
	// on this same connection, not only to sqlClusterIDRest.
	var one int
	if err := conn.QueryRow(ctx, `SELECT 1`).Scan(&one); err != nil {
		t.Fatalf("the transaction is not usable after readClusterID: %v", err)
	}
}
