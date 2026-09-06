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
// and until default_transaction_read_only=on was set at connect time that
// covered only statements inside one. Source.SystemID queries outside any
// BEGIN, so on that path the shape allowlist was the whole defence — which is
// the near-miss THREAT_MODEL.md T9 records: while a cast's type name could
// absorb any word, `SELECT t."a"::int INTO evil FROM ...` matched a registered
// shape, and `SELECT ... INTO` is CREATE TABLE AS.
//
// This is the test for the layer that closes it. It asks the server, not the
// tracer: the write below is registered on the allowlist on purpose, so that
// what refuses it can only be the connection's own read-only default.

// readOnlyErrCode is SQLSTATE 25006, read_only_sql_transaction.
const readOnlyErrCode = "25006"

func TestAWriteOutsideATransactionIsRefusedByTheServer(t *testing.T) {
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
	defer conn.Release()

	// A read outside a transaction still works: the setting makes the implicit
	// transaction read-only, not the connection useless.
	var one int
	if scanErr := conn.QueryRow(ctx, `SELECT 1`).Scan(&one); scanErr != nil {
		t.Fatalf("a read outside a transaction failed: %v", scanErr)
	}
	if one != 1 {
		t.Fatalf("SELECT 1 returned %d", one)
	}

	_, err = conn.Exec(ctx, `CREATE TABLE evil (a int)`)
	if err == nil {
		t.Fatal("a write outside a transaction succeeded on the source pool")
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
