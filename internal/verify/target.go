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
	raw := s.rawJSONColumn(t, column)
	for rows.Next() {
		v, err := scanCell(rows, raw)
		if err != nil {
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

// scanRows hands every row of several columns of one target table to fn, one
// row at a time, in the order cols names them. It is scanColumn over more than
// one column, for the identity columns ADR-015's row check reads beside a
// masked one (explain.go).
func (s *state) scanRows(ctx context.Context, t ref.TableRef, cols []string, fn func([]any) error) error {
	rows, err := s.target.Query(ctx, scanRowsSQL(t, cols))
	if err != nil {
		return fmt.Errorf("verify: reading %s in the target: %w", t, err)
	}
	defer rows.Close()
	raw := make([]bool, len(cols))
	for i, c := range cols {
		raw[i] = s.rawJSONColumn(t, c)
	}
	dest := make([]any, len(cols))
	for rows.Next() {
		row := make([]any, len(cols))
		for i := range row {
			dest[i] = &row[i]
		}
		var bufs [][]byte
		for i, isRaw := range raw {
			if !isRaw {
				continue
			}
			if bufs == nil {
				bufs = make([][]byte, len(cols))
			}
			dest[i] = &bufs[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return fmt.Errorf("verify: reading %s in the target: %w", t, err)
		}
		for i, isRaw := range raw {
			if !isRaw {
				continue
			}
			if bufs[i] == nil {
				row[i] = nil
			} else {
				row[i] = string(bufs[i])
			}
		}
		if err := fn(row); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("verify: reading %s in the target: %w", t, err)
	}
	return nil
}

// rawJSONColumn reports whether t.column is a scalar json or jsonb column (its
// base type, through a domain), whose value scanColumn and scanRows must read
// as raw text rather than let the driver decode it: pgx's own JSON codec
// unmarshals a *any destination with encoding/json's plain Unmarshal, which
// turns an integer past 2^53 into a rounded float64 before any validator here
// ever sees it (T-0402, the 2026-09-25 JSON red team's A11) — a 19-digit
// Luhn-valid card number stored as a json number leaf came back rounded and
// the leaf read clean. decodeDocument already decodes a string with
// encoding/json's Decoder and UseNumber to keep every digit, exactly as
// internal/transform's own copy does; this is what makes every column this
// stage reads take that path, not only a domain over jsonb (which pgx cannot
// resolve a codec for and so was already read as text). hstore is also
// document(family) but is not JSON text and has no such codec, so it is left
// alone.
//
// It answers false for a json[] or jsonb[] column (shapeOf's array flag): the
// wire text for an array is a Postgres array literal — `{"{\"a\":1}"}` — and
// scanning that into *[]byte hands decodeDocument a string that is not JSON
// at all, so it fails to parse and every leaf of the array is silently lost
// to the residual scan rather than merely left float64-rounded (fix round
// after T-0402's first pass, a reviewer's finding). pgx's own array codec is
// left to decode such a column as it always has — into []any of the same
// natively-decoded elements a plain jsonb column used to arrive as before
// this task — which keeps every leaf visible to documentHits even though a
// number leaf inside it can still lose digits past 2^53 (that gap is the
// array carrier internal/pg/CLAUDE.md's own T-0402 note names as still open,
// tracked separately).
func (s *state) rawJSONColumn(t ref.TableRef, column string) bool {
	tbl := s.tables[t]
	if tbl == nil {
		return false
	}
	col, ok := columnOf(tbl, column)
	if !ok {
		return false
	}
	family, array := s.shapeOf(col)
	if array {
		return false
	}
	return family == famJSON || family == famJSONB
}

// scanCell reads one column's value from the current row. raw asks for a json
// or jsonb column's exact source text (rawJSONColumn): scanning it into *[]byte
// keeps a NULL a nil slice, the same distinction *any would have made, and
// keeps the bytes pgx's own JSON codec would otherwise have decoded and
// rounded.
func scanCell(rows pipeline.Rows, raw bool) (any, error) {
	if !raw {
		var v any
		err := rows.Scan(&v)
		return v, err
	}
	var b []byte
	if err := rows.Scan(&b); err != nil {
		return nil, err
	}
	if b == nil {
		return nil, nil
	}
	return string(b), nil
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
