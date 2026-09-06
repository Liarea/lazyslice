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
func Connect(ctx context.Context, d dsn.DSN, tracer *Tracer) (*pgxpool.Pool, error) {
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
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("pg: opening a connection pool: %w", err)
	}
	return pool, nil
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
