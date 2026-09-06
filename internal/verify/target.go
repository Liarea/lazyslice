// SPDX-License-Identifier: Apache-2.0

package verify

import (
	"context"
	"fmt"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// Reading the target. Every statement is built in sql.go and every one of them
// is a read: this stage writes nothing, and the Writer it holds is the loader's
// connection only because that is where the target is (ARCHITECTURE.md section
// 2, "Only the loader and the verifier hold one").

// countRows is the number of rows one target table holds.
func (s *state) countRows(ctx context.Context, t ref.TableRef) (int64, error) {
	var n int64
	if err := s.one(ctx, s.target, countSQL(t), &n); err != nil {
		return 0, fmt.Errorf("verify: counting %s in the target: %w", t, err)
	}
	return n, nil
}

// scanColumn hands every value of one target column to fn, one row at a time.
// Nothing holds the column: the target is small but a column of it is still
// production-shaped data, and the resident set of this stage is one value.
func (s *state) scanColumn(ctx context.Context, t ref.TableRef, column string, fn func(any) error) error {
	rows, err := s.target.Query(ctx, scanSQL(t, column))
	if err != nil {
		return fmt.Errorf("verify: reading %s.%s in the target: %w", t, column, err)
	}
	defer rows.Close()
	for rows.Next() {
		var v any
		if err := rows.Scan(&v); err != nil {
			return fmt.Errorf("verify: reading %s.%s in the target: %w", t, column, err)
		}
		if err := fn(v); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("verify: reading %s.%s in the target: %w", t, column, err)
	}
	return nil
}

// one runs a statement that returns one row and scans it into dest.
func (s *state) one(ctx context.Context, q targetReader, sql string, dest ...any) error {
	rows, err := q.Query(ctx, sql)
	if err != nil {
		return err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}
		return fmt.Errorf("the statement returned no row")
	}
	if err := rows.Scan(dest...); err != nil {
		return err
	}
	return rows.Err()
}

// rowValues reads a statement's rows as slices of scanned values, which is what
// the sample comparison compares. It is used on both sides — the target
// directly, the source through the short transaction — so it takes the reader.
func rowValues(ctx context.Context, q querier, sql string, ncols int, args ...any) ([][]any, error) {
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out [][]any
	dest := make([]any, ncols)
	for rows.Next() {
		row := make([]any, ncols)
		for i := range row {
			dest[i] = &row[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// querier is what both sides of the sample comparison have in common: the
// target reader this stage was handed, and pipeline.Reader on the source.
type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pipeline.Rows, error)
}

var (
	_ querier = targetReader(nil)
	_ querier = pipeline.Reader(nil)
)
