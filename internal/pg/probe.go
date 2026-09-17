// SPDX-License-Identifier: Apache-2.0

package pg

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// ProbeEmptiness re-runs ARCHITECTURE.md section 9 rule 5 — the gate's own
// emptiness probe — over the target's *current* user tables, fresh from the
// catalog, through a transaction the caller already holds.
//
// It exists for internal/load's lock-and-recheck (ARCHITECTURE.md section
// 11.2, T-0130): dropOne's own per-table recheck only ever locks and
// re-verifies a table that is already a member of this run's plan, so a table
// a third party creates in the target between the gate's approval and this
// run's first drop is never a member of it and was never locked, never
// rechecked, never named (T-0242,
// docs/reviews/2026-09-15-redteam/round4-still-leaking.json — a table
// created outside the plan, holding production rows, while the run was
// stalled on the source between the gate and the first drop). Before the
// first drop, under the run lease already held, the loader calls this once
// over the whole target and refuses naming whatever it finds occupied that
// its own plan does not already know about — which, because the gate
// approved this identical query as entirely empty (an unmarked target) or
// never asked it of anything outside the plan at all (a bound marker), is
// necessarily a table that changed since.
//
// It reuses the gate's own query (sqlUserTables) and its own bookkeeping
// exemption (exemptFromEmptiness) so the two probes can never silently drift
// into asking different questions, and it never counts rows, for the same
// reason the gate's own probe does not: it exists to refuse a write to a
// database it has decided not to touch, not to describe one.
//
// Unlike checkEmpty's, this sweep runs inside the caller's own transaction
// rather than on an autocommit connection, because that transaction is the
// one the drops that follow have to happen in too (T-0130). In Postgres any
// statement error inside a transaction block aborts the whole transaction,
// so without a savepoint the first existsOne to fail — 42501 on a table a
// third party created under another role, exactly the case this probe
// exists to catch — would poison every existsOne after it, and each would
// fail with 25P02 rather than answer its own table; the refusal would then
// name every remaining table alphabetically whether or not it holds a row.
// A savepoint taken once before the sweep and rolled back to (never
// released) on each existsOne failure is the same instrument
// internal/introspect's sampler uses for the identical reason
// (readSamples, sample.go): rolling back to a savepoint does not consume it,
// so one SAVEPOINT statement covers every failure in the loop.
//
// tx is anything the caller already has open — internal/load hands it a
// pipeline.Tx from the writer's own Begin, not a second connection, so the
// read happens on the same session as the drops that follow it.
func ProbeEmptiness(ctx context.Context, tx pipeline.Tx) ([]ref.TableRef, error) {
	type userTable struct {
		ref ref.TableRef
		rls bool
	}

	rows, err := tx.Query(ctx, sqlUserTables)
	if err != nil {
		return nil, fmt.Errorf("pg: probing the target's current user tables: %w", err)
	}
	var tables []userTable
	for rows.Next() {
		var u userTable
		if scanErr := rows.Scan(&u.ref.Schema, &u.ref.Name, &u.rls); scanErr != nil {
			rows.Close()
			return nil, fmt.Errorf("pg: probing the target's current user tables: %w", scanErr)
		}
		if exemptFromEmptiness(u.ref) {
			continue
		}
		tables = append(tables, u)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("pg: probing the target's current user tables: %w", err)
	}

	var haveSavepoint bool
	var occupied []ref.TableRef
	for _, u := range tables {
		if u.rls {
			// Under FORCE ROW LEVEL SECURITY with no matching policy, EXISTS
			// returns false over millions of rows the session cannot see,
			// while DROP TABLE still succeeds — the same trap checkEmpty
			// guards against, counted as occupied by rule and never probed.
			occupied = append(occupied, u.ref)
			continue
		}
		if !haveSavepoint {
			// Taken lazily, on the first table that is actually probed, so a
			// caller whose sweep never reaches existsOne (every table exempt
			// or RLS) never sends a statement it has no use for.
			if spErr := tx.Exec(ctx, sqlProbeSavepoint); spErr != nil {
				return nil, fmt.Errorf("pg: probing the target's current user tables: %w", spErr)
			}
			haveSavepoint = true
		}
		exists, existsErr := existsOne(ctx, tx, u.ref)
		if existsErr != nil {
			if errors.Is(existsErr, context.Canceled) || errors.Is(existsErr, context.DeadlineExceeded) {
				return nil, fmt.Errorf("pg: probing %s.%s: %w", u.ref.Schema, u.ref.Name, existsErr)
			}
			// A probe that errors for any other reason, 42501 included, counts
			// as occupied — the gate's own rule for a table it cannot read.
			// The statement aborted the transaction; roll back to the
			// savepoint taken above so the *next* table's own EXISTS runs
			// clean rather than failing with 25P02 and being misreported as
			// occupied for a reason that has nothing to do with its own rows.
			if rbErr := tx.Exec(ctx, sqlProbeRollback); rbErr != nil {
				return nil, fmt.Errorf("pg: probing %s.%s: %w, and rolling back to the probe savepoint failed: %w",
					u.ref.Schema, u.ref.Name, existsErr, rbErr)
			}
			occupied = append(occupied, u.ref)
			continue
		}
		if exists {
			occupied = append(occupied, u.ref)
		}
	}
	return occupied, nil
}

// sqlProbeSavepoint and sqlProbeRollback bound each existsOne call so that one
// table's error — 42501 included — cannot abort the sweep for every table
// after it (see ProbeEmptiness's doc comment). Rolling back to a savepoint
// does not consume it, so the one SAVEPOINT taken before the loop is rolled
// back to as many times as tables fail, the same instrument
// internal/introspect's sampler uses (sample.go, readSamples).
const (
	sqlProbeSavepoint = `SAVEPOINT lazyslice_probe_emptiness`
	sqlProbeRollback  = `ROLLBACK TO SAVEPOINT lazyslice_probe_emptiness`
)

// existsOne runs the gate's own SELECT EXISTS over one table, through a
// pipeline.Tx rather than the gate's own *pgxpool.Conn, which is the only
// difference between this probe and checkEmpty's.
func existsOne(ctx context.Context, tx pipeline.Tx, table ref.TableRef) (bool, error) {
	q := `SELECT EXISTS (SELECT 1 FROM ` + pgx.Identifier{table.Schema, table.Name}.Sanitize() + `)`
	rows, err := tx.Query(ctx, q)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	var exists bool
	if !rows.Next() {
		if rowsErr := rows.Err(); rowsErr != nil {
			return false, rowsErr
		}
		return false, fmt.Errorf("pg: %s.%s did not answer whether it holds rows", table.Schema, table.Name)
	}
	if scanErr := rows.Scan(&exists); scanErr != nil {
		return false, scanErr
	}
	return exists, rows.Err()
}
