// SPDX-License-Identifier: Apache-2.0

//go:build integration

package pg

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Liarea/lazyslice/internal/dsn"
	"github.com/Liarea/lazyslice/internal/testutil"
)

// ARCHITECTURE.md §2 makes every source transaction REPEATABLE READ READ ONLY,
// and that transaction is the enforcement THREAT_MODEL.md T9 names — the tracer
// is the evidence. The near-miss T9 records is what happens when the evidence is
// the only layer: while a cast's type-name continuation was `[a-z]+`,
// `SELECT t."a"::int INTO evil FROM ...` matched a registered shape, and
// `SELECT ... INTO` is CREATE TABLE AS.
//
// Source.SystemID was the one statement issued outside any BEGIN, so that path
// had the allowlist and nothing else, and it was covered for a while by
// default_transaction_read_only on the session. That session GUC leaked through
// a transaction-pooling PgBouncer onto a shared server connection and outlived
// the run (T-0076, pooler_integration_test.go), so it is gone and SystemID runs
// inside a transaction instead. These two tests are what says so: the server
// refuses a write inside the transaction this package opens, and SystemID opens
// one.

// readOnlyErrCode is SQLSTATE 25006, read_only_sql_transaction.
const readOnlyErrCode = "25006"

func TestAWriteInsideASourceTransactionIsRefusedByTheServer(t *testing.T) {
	ctx := context.Background()
	testutil.SkipWithoutDocker(ctx, t)

	url := testutil.Postgres(ctx, t, "")

	// Both statements are on the allowlist, so a refusal here is the server's.
	// Registering a write shape is something no stage does and nothing else in
	// this package can do; it exists for the length of this test only.
	src, err := OpenSource(ctx, dsn.DSN(url),
		Shape{Name: "test.write", SQL: `CREATE TABLE {ident} ({ident} {ident})`},
		Shape{Name: "test.read", SQL: `SELECT {int}`},
	)
	if err != nil {
		t.Fatalf("opening the source: %v", err)
	}
	defer src.Close()

	conn, err := src.pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquiring a source connection: %v", err)
	}
	// The transaction every statement on the source runs inside, opened with the
	// one string Snapshot, Privileges, Short and SystemID all open theirs with.
	if _, beginErr := conn.Exec(ctx, sqlBeginReadOnly); beginErr != nil {
		conn.Release()
		t.Fatalf("opening a read-only transaction on the source: %v", beginErr)
	}
	defer endTx(context.WithoutCancel(ctx), conn)

	// A read inside it works: READ ONLY makes the transaction read-only, not
	// the connection useless.
	var one int
	if scanErr := conn.QueryRow(ctx, `SELECT 1`).Scan(&one); scanErr != nil {
		t.Fatalf("a read inside the transaction failed: %v", scanErr)
	}
	if one != 1 {
		t.Fatalf("SELECT 1 returned %d", one)
	}

	_, err = conn.Exec(ctx, `CREATE TABLE evil (a int)`)
	if err == nil {
		t.Fatal("a write inside a REPEATABLE READ READ ONLY transaction succeeded on the source")
	}
	if refused := src.Violation(); refused != nil {
		t.Fatalf("the allowlist refused the write, so this test proved nothing about the server: %v", refused)
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("the write failed with %v, which is not a server error", err)
	}
	if pgErr.Code != readOnlyErrCode {
		t.Errorf("the server refused the write with SQLSTATE %s, want %s (read_only_sql_transaction)",
			pgErr.Code, readOnlyErrCode)
	}
}

// And the statement that used to run outside one now runs inside one. Asserted
// over the trace rather than over a setting, because the setting is what was
// removed: what the run has instead is a BEGIN before the read and a ROLLBACK
// after it, on the same connection.
func TestSystemIDRunsInsideAReadOnlyTransaction(t *testing.T) {
	ctx := context.Background()
	testutil.SkipWithoutDocker(ctx, t)

	url := testutil.Postgres(ctx, t, "")

	src, err := OpenSource(ctx, dsn.DSN(url))
	if err != nil {
		t.Fatalf("opening the source: %v", err)
	}
	defer src.Close()

	if _, err := src.SystemID(ctx); err != nil {
		t.Fatalf("reading the system identifier: %v", err)
	}
	if refused := src.Violation(); refused != nil {
		t.Fatalf("the allowlist refused a statement: %v", refused)
	}

	// source.connect is the ConnectTracer's record of the connection itself, not
	// a statement; what this test is about is the statements.
	var shapes []string
	for _, s := range src.Trace() {
		if s.Shape == "source.connect" {
			continue
		}
		shapes = append(shapes, s.Shape)
	}
	want := []string{"source.begin", "source.system_id", "source.rollback"}
	if len(shapes) != len(want) {
		t.Fatalf("SystemID sent the shapes %v, want %v: the identity read is the statement that used "+
			"to be on an autocommit path, and nothing this package sends may be", shapes, want)
	}
	for i := range want {
		if shapes[i] != want[i] {
			t.Fatalf("SystemID sent the shapes %v, want %v", shapes, want)
		}
	}
}

// And the rail that catches the next SystemID fires against a real server.
//
// TestASourceStatementOutsideATransactionIsRefused is the unit half, and it
// hands check a transaction status written out as a constant. What it cannot
// say is that a real, idle source connection actually reports 'I' at the moment
// TraceQueryStart runs: txStatus reads it from the PgConn pgx updates on every
// ReadyForQuery, and it answers 0 — deliberately not a violation — for a nil
// conn or a nil PgConn. So a change in how pgx reports the status, or a future
// wrapper that hands the tracer a connection without one, would turn this rail
// into a no-op with every unit test still passing, while internal/pg/CLAUDE.md
// and the comments in this package tell authors they are covered by it.
//
// This is the negative half of TestSystemIDRunsInsideAReadOnlyTransaction, in
// the same place: an allowlisted statement, no BEGIN, and a refusal that names
// the transaction and not the shape.
func TestASourceStatementSentWithNoTransactionIsRefusedOnARealConnection(t *testing.T) {
	ctx := context.Background()
	testutil.SkipWithoutDocker(ctx, t)

	url := testutil.Postgres(ctx, t, "")

	tracer, err := NewTracer(SourceShapes()...)
	if err != nil {
		t.Fatalf("NewTracer: %v", err)
	}
	pool, err := Connect(ctx, dsn.DSN(url), tracer)
	if err != nil {
		t.Fatalf("connecting to the source: %v", err)
	}
	defer pool.Close()

	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquiring a source connection: %v", err)
	}
	defer conn.Release()

	// sqlRole is on the allowlist and every call site sends it inside a
	// transaction. This one does not, which is the whole of the test.
	var role string
	var superuser bool
	if scanErr := conn.QueryRow(ctx, sqlRole).Scan(&role, &superuser); scanErr == nil {
		t.Fatal("an allowlisted statement ran on an idle source connection")
	}

	violation := tracer.Violation()
	if !errors.Is(violation, ErrOutsideTransaction) {
		t.Fatalf("Violation() = %v, want ErrOutsideTransaction: txStatus did not see the idle connection "+
			"a real server reported, so the rail is a no-op", violation)
	}
}
