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
