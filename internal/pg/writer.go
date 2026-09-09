// SPDX-License-Identifier: Apache-2.0

package pg

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// writer is the target connection. Only the loader and the verifier hold one,
// and only after the gate has returned Eligible.
type writer struct {
	pool  *pgxpool.Pool
	types *typeRegistry
}

var _ pipeline.Writer = (*writer)(nil)

// RegisterTypes is pipeline.Writer's fourth method: the loader calls it once,
// after the DDL of ARCHITECTURE.md §11.1 item 3 has created the source's enums, domains
// and composites in the target and before the first CopyFrom (types.go).
func (w *writer) RegisterTypes(ctx context.Context, s *pipeline.Schema) error {
	return registerTypes(ctx, w.pool, w.types, s)
}

func (w *writer) Exec(ctx context.Context, sql string, args ...any) error {
	if _, err := w.pool.Exec(ctx, sql, args...); err != nil {
		return err
	}
	return nil
}

func (w *writer) CopyFrom(ctx context.Context, table ref.TableRef, cols []string, rows <-chan []any) (int64, error) {
	conn, err := w.pool.Acquire(ctx)
	if err != nil {
		return 0, fmt.Errorf("pg: acquiring a target connection to copy into %s.%s: %w", table.Schema, table.Name, err)
	}
	defer conn.Release()
	return copyFrom(ctx, conn, table, cols, rows)
}

func (w *writer) Begin(ctx context.Context) (pipeline.Tx, error) {
	conn, err := w.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("pg: acquiring a target connection: %w", err)
	}
	t, err := conn.Begin(ctx)
	if err != nil {
		conn.Release()
		return nil, fmt.Errorf("pg: opening a target transaction: %w", err)
	}
	return &tx{conn: conn, tx: t}, nil
}

// tx is one target transaction. The loader opens one per table at Seq 0 and
// commits it at Last (ARCHITECTURE.md §2, RowBatch). The pooled connection is
// returned when the transaction ends, whichever way it ends.
type tx struct {
	conn *pgxpool.Conn
	tx   pgx.Tx
	done bool
}

var _ pipeline.Tx = (*tx)(nil)

func (t *tx) Exec(ctx context.Context, sql string, args ...any) error {
	if _, err := t.tx.Exec(ctx, sql, args...); err != nil {
		return err
	}
	return nil
}

func (t *tx) CopyFrom(ctx context.Context, table ref.TableRef, cols []string, rows <-chan []any) (int64, error) {
	return copyFromTx(ctx, t.tx, table, cols, rows)
}

func (t *tx) Commit(ctx context.Context) error {
	if t.done {
		return errors.New("pg: the transaction has already ended")
	}
	t.done = true
	defer t.conn.Release()
	if err := t.tx.Commit(ctx); err != nil {
		return fmt.Errorf("pg: committing a target transaction: %w", err)
	}
	return nil
}

func (t *tx) Rollback(ctx context.Context) error {
	if t.done {
		return nil
	}
	t.done = true
	defer t.conn.Release()
	if err := t.tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		return fmt.Errorf("pg: rolling back a target transaction: %w", err)
	}
	return nil
}

// copyFromSource adapts a channel of rows to pgx's CopyFromSource. The channel
// is the extract → transform → load contract's carrier, and the context is held
// here because CopyFromSource has no room for one: without it a cancelled run
// would block on a producer that has already gone away.
type copyFromSource struct {
	ctx  context.Context
	rows <-chan []any
	cur  []any
	err  error
}

func (s *copyFromSource) Next() bool {
	select {
	case <-s.ctx.Done():
		s.err = s.ctx.Err()
		return false
	case row, ok := <-s.rows:
		if !ok {
			return false
		}
		s.cur = row
		return true
	}
}

func (s *copyFromSource) Values() ([]any, error) { return s.cur, nil }

func (s *copyFromSource) Err() error { return s.err }

type copier interface {
	CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error)
}

func copyFrom(ctx context.Context, c copier, table ref.TableRef, cols []string, rows <-chan []any) (int64, error) {
	n, err := c.CopyFrom(ctx, pgx.Identifier{table.Schema, table.Name}, cols, &copyFromSource{ctx: ctx, rows: rows})
	if err != nil {
		return n, copyFailure(table, err)
	}
	return n, nil
}

// copyFailure turns the driver's error into one that is safe to print.
//
// pgx's encode errors quote the offending value: pgtype's Map.Encode formats it
// with %#v ("unable to encode main.T{Email:\"alice@example.com\"} into binary
// format for text (OID 25)"), pgx returns that text unchanged through
// encodeCopyValue and CopyFrom, and cmd/lazyslice prints a non-PgError as
// err.Error(). That is a row value leaving the process through an error message
// on the one path RenderError does not cover (THREAT_MODEL.md T4), so the text
// is replaced here rather than wrapped: the table is named, the driver's own
// words are withheld behind RenderAnyError and the flag whose name says what it
// does.
//
// A *pgconn.PgError is left alone, because RenderError already drops the fields
// that quote a row and the SQLSTATE is the useful half. A context error is left
// alone because "context canceled" is not a value.
func copyFailure(table ref.TableRef, err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) ||
		errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("pg: copying into %s.%s: %w", table.Schema, table.Name, err)
	}
	return &withheldError{
		safe: fmt.Sprintf("pg: copying into %s.%s: the driver could not encode or send a row; "+
			"its message is withheld because it quotes the value (--show-row-values-in-errors)",
			table.Schema, table.Name),
		inner: err,
	}
}

// withheldError is a driver error whose text is not printable. It deliberately
// has no Unwrap: the point is that nothing reaches the original text by
// accident, and RenderAnyError is the one thing that reaches it on purpose.
type withheldError struct {
	safe  string
	inner error
}

func (e *withheldError) Error() string { return e.safe }

func copyFromTx(ctx context.Context, t pgx.Tx, table ref.TableRef, cols []string, rows <-chan []any) (int64, error) {
	return copyFrom(ctx, t, table, cols, rows)
}
