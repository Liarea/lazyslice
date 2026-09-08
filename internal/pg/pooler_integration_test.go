// SPDX-License-Identifier: Apache-2.0

//go:build integration

package pg

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Liarea/lazyslice/internal/dsn"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/testutil"
)

// A pooled source endpoint is a supported v1 topology (ADR-005 "Pooled
// endpoints", ARCHITECTURE.md §2 "Source.Reader", §8's --single-connection).
// connect_test.go's TestConnectAddsNoStartupParameterAPoolerWouldRefuse is the
// unit half of the claim and says in its own comment what it cannot do: it
// compares the parameters we send against a hand-written list of what PgBouncer
// documents it accepts, so it cannot fail on a list that has gone stale and it
// cannot see anything else a pooler refuses. This is the half where PgBouncer
// answers.
//
// Transaction pooling, which is the mode a pooled endpoint is deployed in and
// the mode research/OPEN_QUESTIONS.md item 1 is about.

// serialWait is how long a second Reader is given to *not* return while the
// first still holds the serialisation token. It is a lower bound on a wait, so
// it cannot flake the way an upper bound on one would: the test fails only if
// the second reader comes back early, which is a bug and not a slow machine.
const serialWait = 2 * time.Second

func TestASourceOpensAndReadsThroughAPooler(t *testing.T) {
	ctx := context.Background()
	testutil.SkipWithoutDocker(ctx, t)

	pooled, _ := testutil.PgBouncer(ctx, t, "")

	src, err := OpenSource(ctx, dsn.DSN(pooled),
		Shape{Name: "test.read", SQL: `SELECT {int}`},
	)
	if err != nil {
		t.Fatalf("opening the source through the pooler: %v", err)
	}
	defer src.Close()

	// The acquire is where a refused startup parameter surfaces, because
	// pgxpool connects lazily: Connect returns a pool that has opened nothing.
	// So this is the assertion, and its failure mode is the opaque one the
	// package doc warns about.
	conn, err := src.pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquiring a connection through the pooler: %v\n"+
			"a source connection could not be opened through PgBouncer at all. A startup "+
			"parameter outside the pooler's fixed list is refused as a connection error, so "+
			"check what Connect puts in ConnConfig.RuntimeParams: a session SET is the only "+
			"way to set anything else (pg.go).", err)
	}
	defer conn.Release()

	// The identity read is the statement that used to be sent on an autocommit
	// path, covered by a session GUC; it now opens a transaction of its own, and
	// through a pooler that is a BEGIN and a ROLLBACK the pooler has to route on
	// the same server connection. Which it does, in transaction mode, because
	// that is what a transaction is to it.
	if _, idErr := src.SystemID(ctx); idErr != nil {
		t.Fatalf("reading the system identifier through the pooler: %v", idErr)
	}

	// And a whole snapshot works through it: export, import, read.
	id, err := src.Snapshot(ctx)
	if err != nil {
		t.Fatalf("exporting a snapshot through the pooler: %v", err)
	}
	defer func() { _ = src.Release(context.WithoutCancel(ctx)) }()

	r, err := src.Reader(ctx, id)
	if err != nil {
		t.Fatalf("opening a reader through the pooler: %v", err)
	}
	defer func() { _ = r.Close(context.WithoutCancel(ctx)) }()

	rows, err := r.Query(ctx, `SELECT 1`)
	if err != nil {
		t.Fatalf("reading through the pooler: %v", err)
	}
	got := rows.Next()
	rows.Close()
	if !got {
		t.Fatal("a read through the pooler returned no rows")
	}
	if err := src.Violation(); err != nil {
		t.Errorf("the allowlist refused a statement: %v", err)
	}
}

// A pooled run leaves the pooler as it found it (T-0076).
//
// This is the test for the defect the T-0050 fix round measured. Connect used to
// exec `SET default_transaction_read_only = on` in an AfterConnect hook, as a
// second layer under ARCHITECTURE.md §2's READ ONLY transactions. Through a
// transaction-pooling PgBouncer that GUC is set on the *server* connection the
// pooler assigned, not on anything lazyslice owns, and PgBouncer does not run
// server_reset_query in transaction mode by default
// (server_reset_query_always = 0) — so it stayed there after lazyslice exited
// and every later client assigned that server connection inherited it, for up
// to server_lifetime (3600 s). A safety rail that turns a neighbour's
// application read-only is damage, and every source transaction was already
// `BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY` anyway; the one statement
// that ran outside a transaction, Source.SystemID, runs inside one now.
//
// One server connection for the whole pooler is what makes this a proof rather
// than a coincidence: the second client below cannot be given a different
// server connection than the one the run used, so if the run left anything on
// it, the second client sees it. Against the old AfterConnect exec this test
// fails on the CREATE TABLE with SQLSTATE 25006.
func TestAPooledRunLeavesThePoolerWritableForOtherClients(t *testing.T) {
	ctx := context.Background()
	testutil.SkipWithoutDocker(ctx, t)

	pooled, _ := testutil.PgBouncer(ctx, t, "", map[string]string{
		"MAX_DB_CONNECTIONS": "1",
		"DEFAULT_POOL_SIZE":  "1",
		// PgBouncer's own default is 120 s, which would turn a bug here into a
		// two-minute hang instead of an answer.
		"QUERY_WAIT_TIMEOUT": "10",
	})

	// A source session, in its own scope so that everything it holds is closed
	// before the second client connects: the run is over, the way it is over
	// when the binary exits.
	//
	// SystemID and the holder transaction are the whole of it. Every statement
	// this package sends to the source is one of these three forms — a
	// standalone read-only transaction, the holder's, or a reader's — and a
	// reader is left out on purpose: with one server connection the pooler has
	// none to give while the holder's transaction pins it, so opening one would
	// wait out QUERY_WAIT_TIMEOUT and exercise the serialised fallback that
	// TestAPoolerWithOneServerConnectionSerialisesTheExtract is already for.
	func() {
		src, err := OpenSource(ctx, dsn.DSN(pooled))
		if err != nil {
			t.Fatalf("opening the source through the pooler: %v", err)
		}
		defer src.Close()

		if _, idErr := src.SystemID(ctx); idErr != nil {
			t.Fatalf("reading the system identifier through the pooler: %v", idErr)
		}
		if _, snapErr := src.Snapshot(ctx); snapErr != nil {
			t.Fatalf("exporting a snapshot through the pooler: %v", snapErr)
		}
		if relErr := src.Release(context.WithoutCancel(ctx)); relErr != nil {
			t.Fatalf("releasing the snapshot: %v", relErr)
		}
		if violation := src.Violation(); violation != nil {
			t.Fatalf("the allowlist refused a statement: %v", violation)
		}
	}()

	// The neighbour: an unrelated application on the same pooler, with no
	// tracer, no allowlist and no settings of its own.
	cfg, err := pgxpool.ParseConfig(pooled)
	if err != nil {
		t.Fatalf("parsing the pooled connection string: %v", err)
	}
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	neighbour, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("opening the second client's pool: %v", err)
	}
	defer neighbour.Close()

	// Read the setting first, so that a failure below reads as what it is rather
	// than as some other reason CREATE TABLE could fail.
	var readOnly string
	if scanErr := neighbour.QueryRow(ctx, `SELECT current_setting('default_transaction_read_only')`).Scan(&readOnly); scanErr != nil {
		t.Fatalf("reading default_transaction_read_only as the second client: %v", scanErr)
	}
	if readOnly != "off" {
		t.Errorf("default_transaction_read_only is %q for an unrelated client on this pooler, want \"off\": "+
			"the lazyslice run left a session GUC on the shared server connection (T-0076). "+
			"The read-only setting is per transaction and never a session default", readOnly)
	}

	if _, err := neighbour.Exec(ctx, `CREATE TABLE neighbour_write (a int)`); err != nil {
		t.Fatalf("an unrelated client on the same pooler could not write after a lazyslice run: %v\n"+
			"this is the leak T-0076 fixed: a session GUC set on a pooled connection stays on the shared "+
			"server connection for up to server_lifetime. Nothing lazyslice sends to the source may be "+
			"session state; the read-only setting belongs in the transaction (pg.go, source.go)", err)
	}
}

// The negative control, and the reason the read-only setting was never a
// startup parameter: the same pooler, the same server, one parameter in the
// startup packet, and no connection can be opened at all.
//
// Without this the decision in Connect is a comment. With it, answering the
// leak above by moving the setting into RuntimeParams fails here — both doors
// out of the transaction are shut, which is why the setting is in it.
func TestAStartupParameterOutsideThePoolersListRefusesTheConnection(t *testing.T) {
	ctx := context.Background()
	testutil.SkipWithoutDocker(ctx, t)

	pooled, _ := testutil.PgBouncer(ctx, t, "")

	acquire := func(t *testing.T, withParameter bool) error {
		t.Helper()
		cfg, err := pgxpool.ParseConfig(pooled)
		if err != nil {
			t.Fatalf("parsing the pooled connection string: %v", err)
		}
		if withParameter {
			// This is what Connect did before it used a session SET.
			cfg.ConnConfig.RuntimeParams["default_transaction_read_only"] = "on"
		}
		cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec

		pool, err := pgxpool.NewWithConfig(ctx, cfg)
		if err != nil {
			t.Fatalf("opening the pool: %v", err)
		}
		defer pool.Close()

		conn, err := pool.Acquire(ctx)
		if err == nil {
			conn.Release()
		}
		return err
	}

	// The positive control comes first, so that the failure below is
	// attributable to the parameter and to nothing else: the same pooled URL and
	// the same pool settings, without the RuntimeParams entry. A pooler that
	// never started, a Docker network that is not there and a mapped port that
	// moved all fail here instead, where they read as what they are.
	if err := acquire(t, false); err != nil {
		t.Fatalf("the same pooled URL could not be connected to without the startup parameter: %v\n"+
			"this test cannot say anything about the parameter until the connection itself works", err)
	}

	err := acquire(t, true)
	if err == nil {
		t.Fatal("PgBouncer accepted default_transaction_read_only in the startup packet; " +
			"if that is now true of every pooler, the reason Connect uses a session SET has changed " +
			"and pg.go's comment needs rewriting rather than this test deleting")
	}
	// The shape of the failure matters as much as the failure: it is the pooler
	// refusing that parameter, not a setting that quietly did not apply and not
	// the connection failing for some reason of its own. Asserted and not
	// logged: a check whose only consequence is a log line passes for a pooler
	// that was never reachable, which is the proposition this test exists to
	// establish.
	if lower := strings.ToLower(err.Error()); !strings.Contains(lower, "startup parameter") &&
		!strings.Contains(lower, "unsupported") {
		t.Errorf("the pooler refused the connection with %v; want a refusal naming the unsupported "+
			"startup parameter. The connection without it succeeded, so this is the parameter — but "+
			"if the wording has changed, widen the match rather than dropping the assertion", err)
	}
}

// The serialised fallback (ADR-005 "Pooled endpoints", §8's
// --single-connection): an endpoint that cannot import the exported snapshot
// gets the holder connection itself, Serialised() says so, and a second Reader
// waits for the first to close rather than writing to the same wire.
//
// PgBouncer in transaction pooling *can* import the snapshot when it has a
// second server connection to give — it pins one for the length of the holder's
// transaction, so the exporting transaction is still open when the second client
// asks — which is worth knowing and is why this test does not assert that it
// cannot. What drives the fallback here is one half of the condition: a snapshot
// identifier the server will not import. That is a synthetic trigger and it is
// not what a pooler does; the other half, a pooler with no second server
// connection to give, is
// TestAPoolerWithOneServerConnectionSerialisesTheExtract below, and that one is
// driven by the pooler itself.
func TestTheSerialisedFallbackWorksThroughAPooler(t *testing.T) {
	ctx := context.Background()
	testutil.SkipWithoutDocker(ctx, t)

	pooled, _ := testutil.PgBouncer(ctx, t, "")

	src, err := OpenSource(ctx, dsn.DSN(pooled), Shape{Name: "test.read", SQL: `SELECT {int}`})
	if err != nil {
		t.Fatalf("opening the source through the pooler: %v", err)
	}
	defer src.Close()

	if _, snapErr := src.Snapshot(ctx); snapErr != nil {
		t.Fatalf("exporting a snapshot through the pooler: %v", snapErr)
	}
	defer func() { _ = src.Release(context.WithoutCancel(ctx)) }()

	if src.Serialised() {
		t.Fatal("Serialised() is true before any reader has been opened; it is the report of a " +
			"fallback that happened, never a prediction")
	}

	// Well-formed (snapshotIDPattern) and not a snapshot this server exported,
	// so SET TRANSACTION SNAPSHOT fails the way it fails on an endpoint that
	// cannot import one.
	const notExported = pipeline.SnapshotID("00000003-0000ffff-1")
	r, err := src.Reader(ctx, notExported)
	if err != nil {
		t.Fatalf("Reader did not fall back on a snapshot the endpoint would not import: %v", err)
	}
	if !src.Serialised() {
		t.Error("Serialised() is false after the fallback; --single-connection's automatic half " +
			"would never be reported")
	}

	// The reader that came back is the holder, and it still reads.
	rows, err := r.Query(ctx, `SELECT 1`)
	if err != nil {
		t.Fatalf("reading through the serialised reader: %v", err)
	}
	got := rows.Next()
	rows.Close()
	if !got {
		t.Fatal("the serialised reader returned no rows")
	}

	// A second reader waits for the first: the holder connection is one wire and
	// pgxpool.Conn is not safe for concurrent use, so the serialisation is the
	// guarantee and Serialised() is only the report.
	second := make(chan struct{})
	go func() {
		defer close(second)
		r2, err := src.Reader(ctx, notExported)
		if err != nil {
			t.Errorf("the second serialised reader: %v", err)
			return
		}
		_ = r2.Close(context.WithoutCancel(ctx))
	}()

	select {
	case <-second:
		t.Error("a second Reader was handed the holder connection while the first still held it")
	case <-time.After(serialWait):
	}

	if err := r.Close(context.WithoutCancel(ctx)); err != nil {
		t.Fatalf("closing the serialised reader: %v", err)
	}
	<-second
}

// The condition a restrictive pooler actually produces, which the synthetic
// snapshot identifier above does not: one server connection, pinned by the
// holder's transaction, and a reader that cannot have a second one.
//
// This is the topology ADR-005's serialised extract and §8's
// --single-connection exist for, and until T-0050's fix round nothing drove it.
// Source.Reader treated only a failed `SET TRANSACTION SNAPSHOT` as "this
// endpoint cannot give a reader", so a pooler that answered the request with
// query_wait_timeout instead — on the acquire or on the BEGIN, whichever asked
// the pooler for a server connection first — ended the run with that error and
// the automatic half of --single-connection was unreachable on exactly the
// endpoint it was written for. The fallback now covers both answers, and this
// test is what says so: the pooler is what refuses, and the extract continues
// on the holder.
func TestAPoolerWithOneServerConnectionSerialisesTheExtract(t *testing.T) {
	ctx := context.Background()
	testutil.SkipWithoutDocker(ctx, t)

	pooled, _ := testutil.PgBouncer(ctx, t, "", map[string]string{
		// One server connection for the whole pooler, which the holder
		// transaction takes and does not give back until Release.
		"MAX_DB_CONNECTIONS": "1",
		"DEFAULT_POOL_SIZE":  "1",
		// PgBouncer's own default here is 120 seconds, which would make the
		// fallback look like a hang. This is the wait that turns "no server
		// connection" into an answer, and it is the pooler's answer and not the
		// test's: nothing on the client side is cancelling anything.
		"QUERY_WAIT_TIMEOUT": "3",
	})

	src, err := OpenSource(ctx, dsn.DSN(pooled), Shape{Name: "test.read", SQL: `SELECT {int}`})
	if err != nil {
		t.Fatalf("opening the source through the one-connection pooler: %v", err)
	}
	defer src.Close()

	id, err := src.Snapshot(ctx)
	if err != nil {
		t.Fatalf("exporting a snapshot through the one-connection pooler: %v", err)
	}
	defer func() { _ = src.Release(context.WithoutCancel(ctx)) }()

	if src.Serialised() {
		t.Fatal("Serialised() is true before any reader has been opened; it is the report of a " +
			"fallback that happened, never a prediction")
	}

	// The real snapshot this server exported, so the only thing standing between
	// this call and a parallel reader is the pooler having no connection to give.
	r, err := src.Reader(ctx, id)
	if err != nil {
		t.Fatalf("Reader failed on a pooler with one server connection instead of serialising: %v\n"+
			"this is the automatic half of --single-connection (ADR-005 \"Pooled endpoints\"): a pooler "+
			"that cannot hand out a second connection is an endpoint that cannot give a reader, and "+
			"Source.Reader has to answer it with the holder rather than with the pooler's error", err)
	}
	defer func() { _ = r.Close(context.WithoutCancel(ctx)) }()

	if !src.Serialised() {
		t.Error("Serialised() is false after the fallback; --single-connection's automatic half " +
			"would never be reported to the operator")
	}

	// And the reader that came back reads: it is the holder, inside the run's
	// own snapshot, through the same pooler.
	rows, err := r.Query(ctx, `SELECT 1`)
	if err != nil {
		t.Fatalf("reading through the serialised reader: %v", err)
	}
	got := rows.Next()
	rows.Close()
	if !got {
		t.Fatal("the serialised reader returned no rows")
	}
	if err := src.Violation(); err != nil {
		t.Errorf("the allowlist refused a statement: %v", err)
	}
}
