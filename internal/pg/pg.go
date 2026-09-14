// SPDX-License-Identifier: Apache-2.0

// Package pg is the only package that speaks Postgres. It implements
// pipeline.Source, pipeline.Target, pipeline.Reader, pipeline.Writer and the
// target gate, and it registers the shape allowlist on the source pool.
//
// The read side is defended in three independent ways (ADR-005): every source
// transaction is REPEATABLE READ READ ONLY; all five pgx tracers are registered
// on the source pool, and a statement whose shape is not on the allowlist gets a
// cancelled context from TraceQueryStart and a recorded violation; and
// TestSourceNeverCopiesOrBatches fails if this package calls CopyFrom, SendBatch
// or Prepare on a source connection. Any one of them would do; all three are
// cheap, and the source is production.
//
// The write side is defended by the gate, which runs after discovery and never
// inside the 1 s dial. Verdict is tri-state so that "not probed" can never be
// read as eligible: any error, timeout or unprobed table is Refused.
//
// OpenSource and OpenTarget are the two entry points. A source pool and its
// tracer are created together and cannot be separated, because a source pool
// without its allowlist is a source with no allowlist and nothing downstream
// would notice.
package pg

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Liarea/lazyslice/internal/dsn"
)

// Connect opens a pool. The source and the target take different pools with
// different tracers, so this is the one place a connection is made.
//
// A non-nil tracer is the source's allowlist. It brings pgx.QueryExecModeExec
// and both statement caches switched off with it: in the cached modes pgx
// prepares statements of its own, and a Prepare on a source connection is
// refused by the allowlist (ARCHITECTURE.md §2 "Source").
//
// It sends nothing else. In particular it does not set
// default_transaction_read_only on the session, and that is a decision with a
// measurement behind it (T-0076). Connect did set it, in an AfterConnect exec,
// as a second layer under ARCHITECTURE.md §2's REPEATABLE READ READ ONLY
// transactions. But a session GUC set through a transaction-pooling PgBouncer
// is set on the *shared server connection* the pooler assigned, and PgBouncer
// in transaction mode does not run server_reset_query by default
// (server_reset_query_always = 0): the setting stayed on that server connection
// after lazyslice exited, and the next unrelated client assigned it inherited
// it — measured, not inferred, in pooler_integration_test.go, where a second
// client's CREATE TABLE failed with "cannot execute CREATE TABLE in a read-only
// transaction" for up to server_lifetime (3600 s by default). A safety rail
// that makes a neighbour's application read-only is not defence in depth; it is
// damage. So the read-only setting is per transaction and never a session
// default: every statement this package sends to the source now goes inside
// `BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY`, Source.SystemID included
// (source.go), which was the last statement here on an autocommit path. The
// enforcement is that transaction; the tracer's shape allowlist is the other
// layer and is unchanged (THREAT_MODEL.md T9).
//
// A caller that takes a tracer-carrying pool from here and queried it without
// opening a transaction would have the allowlist and nothing else, and one did:
// internal/discover's dial sent three catalog reads on the pool directly, which
// the AfterConnect exec had covered while it existed. That dial opens a
// transaction of its own now (T-0081), and the rule is no longer a convention:
// the Tracer refuses any statement that reaches a source connection with no
// transaction open, and the refusal fails the run (T-0082, tracer.go). So if
// you add a source statement, open the transaction at the call site — there is
// no pool-wide rail behind you, and the check will fail you rather than let the
// statement go out uncovered.
//
// The setting was never a startup parameter either, and that half still holds:
// PgBouncer and the other poolers accept only a fixed set of startup parameters
// and refuse the connection outright for anything else unless it is named in
// ignore_startup_parameters. A pooled endpoint is a supported v1 topology
// (ADR-005 "Pooled endpoints", ARCHITECTURE.md §2 "Source.Reader", §8's
// --single-connection), so a startup parameter of ours would make OpenSource
// unable to open any connection at all through one — and because pgxpool
// connects lazily, as an opaque acquire failure rather than at Connect.
// TestConnectAddsNoStartupParameterAPoolerWouldRefuse and
// TestAStartupParameterOutsideThePoolersListRefusesTheConnection are that
// guard; TestConnectSetsNoSessionStateOnASourceConnection is this one's.
func Connect(ctx context.Context, d dsn.DSN, tracer *Tracer, opts ...ConnectOption) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(string(d))
	if err != nil {
		// pgx's error quotes the string it failed on, which is the string that
		// holds the password, so it is not wrapped.
		return nil, fmt.Errorf("pg: the connection string could not be parsed")
	}
	if tracer != nil {
		cfg.ConnConfig.Tracer = tracer
		cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
		cfg.ConnConfig.StatementCacheCapacity = 0
		cfg.ConnConfig.DescriptionCacheCapacity = 0
	}
	for _, o := range opts {
		o(cfg)
	}
	if tracer != nil && cfg.AfterConnect != nil {
		// The source's Never list, as a check rather than a convention: a hook on
		// a source connection sets state on a server connection a pooler shares
		// with other applications (T-0076), and no caller here has a reason to.
		return nil, fmt.Errorf("pg: a source pool may not carry an AfterConnect hook")
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("pg: opening a connection pool: %w", err)
	}
	return pool, nil
}

// ConnectOption adjusts the pool configuration before the pool is opened. It is
// variadic on Connect rather than a fourth parameter because internal/core and
// internal/discover call Connect too and neither has anything to pass.
//
// There is one option and it is the target's: withAfterConnect. A source pool
// that reached here with one is refused above.
type ConnectOption func(*pgxpool.Config)

// withAfterConnect runs f on every new connection in the pool. Only the target
// pool may have one (T-0076, internal/pg/CLAUDE.md's Never list).
func withAfterConnect(f func(context.Context, *pgx.Conn) error) ConnectOption {
	return func(cfg *pgxpool.Config) { cfg.AfterConnect = f }
}

// targetPoolFloor is the smallest target pool that can hold a run lease.
//
// AcquireLease takes one connection out of the target pool and never gives it
// back until the run is over (lease.go), and pool_max_conns is a connection
// string parameter an operator can write: `--target '...?pool_max_conns=1'` is
// legal, and pgxpool would hand the lease the pool's only connection. The very
// next acquire — Target.Gate's — would then wait for a connection that cannot be
// released until the run it is blocking has finished, so the run hangs on
// startup until its context expires instead of refusing. Two is the minimum that
// is not that: the lease plus one worker.
//
// It raises MaxConns and never lowers it, so an operator who asked for more
// still gets what they asked for. It is the target's only, because the source
// pool holds no lease.
const targetPoolFloor int32 = 2

// withMinMaxConns raises the pool's MaxConns to n when the parsed connection
// string left it lower.
func withMinMaxConns(n int32) ConnectOption {
	return func(cfg *pgxpool.Config) {
		if cfg.MaxConns < n {
			cfg.MaxConns = n
		}
	}
}

// RenderError turns a Postgres error into something safe to print: Message and
// SQLSTATE. Detail, Where and Hint are dropped unless
// --show-row-values-in-errors, because a unique-violation Detail quotes the
// conflicting row (THREAT_MODEL.md T4).
//
// It is real rather than a no-op because cmd/lazyslice routes every error it
// prints through it: a placeholder here would be a silent hole in the one
// redaction pass the binary has.
func RenderError(e *pgconn.PgError, showValues bool) string {
	if e == nil {
		return ""
	}
	s := fmt.Sprintf("%s (SQLSTATE %s)", e.Message, e.Code)
	if !showValues {
		return s
	}
	// The operator asked for the fields that quote the row, by a flag whose
	// name says so.
	for _, f := range []struct{ label, value string }{
		{"detail", e.Detail},
		{"where", e.Where},
		{"hint", e.Hint},
	} {
		if f.value != "" {
			s += "\n  " + f.label + ": " + f.value
		}
	}
	return s
}

// RenderAnyError renders any error this package produced, not only a Postgres
// one. It is the whole of the redaction pass for the write side: a driver error
// that is not a *pgconn.PgError can still quote a row value, because pgtype
// formats the value it could not encode with %#v, so copyFrom withholds such an
// error's text and this is what shows it when the operator asks by the flag
// whose name says what it does.
//
// An error with nothing to withhold renders as itself, so a caller can route
// every error through this without deciding which kind it has.
func RenderAnyError(err error, showValues bool) string {
	if err == nil {
		return ""
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return RenderError(pgErr, showValues)
	}
	var withheld *withheldError
	if errors.As(err, &withheld) && showValues && withheld.inner != nil {
		return withheld.safe + "\n  driver: " + withheld.inner.Error()
	}
	return err.Error()
}
