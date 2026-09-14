// SPDX-License-Identifier: Apache-2.0

package pg

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Liarea/lazyslice/internal/pipeline"
)

// MarkerTable is the name of the table lazyslice creates in the target on first
// write. Its presence, bound to this source and this catalog, is what
// authorises truncating a target that is not empty (ARCHITECTURE.md §11.2).
const MarkerTable = "lazyslice_meta"

// MarkerSchemaVersion is the shape of MarkerTable this build writes and reads.
// A marker carrying a higher version was written by a newer lazyslice; it is
// not bound, and the gate falls through to the emptiness check rather than
// guessing at a shape it does not know.
const MarkerSchemaVersion = 1

// The three values of lazyslice_meta.status. A row still at StatusRunning is a
// run that died — SIGKILL, OOM, power loss — and the next run truncates the
// target exactly as it would after a complete one.
//
// T-0133, 2026-09-14 (THREAT_MODEL.md T8 amendment, docs/reviews/2026-09-09
// finding 4): internal/load used to write StatusComplete itself, the moment its
// own copy finished — before core had run verify at all — so a run whose verify
// then failed left a marker saying complete over a target that still held
// personal data. internal/load/load.go no longer writes StatusComplete on
// success at all; it leaves the row at StatusRunning, and core.Run closes it
// once verify has had its say: StatusComplete only after verify passes,
// StatusFailed after any verify failure (load's own failure path is unchanged —
// it still closes its own row to StatusFailed itself, because nothing
// downstream of load runs on that path for core to close it instead).
//
// This reused StatusRunning rather than adding a fourth status (StatusLoaded,
// say) for "load finished, verify has not run yet". Every reader that decides
// anything from status already treats running the way this state needs to be
// treated: the gate (target.go, section 11.2) authorises truncation on a bound
// marker at running exactly as it does at complete, so a run that dies between
// load returning and core closing the row — kill -9, a crash, core losing the
// target connection — truncates on the next run exactly as a run killed
// mid-copy already does, which is the correct outcome in both cases: the row
// does not yet vouch for what is in the target, so the next run should not
// trust it either way. A fourth status would have meant teaching the gate a
// value it treats identically to two it already has, to mark a distinction only
// a person reading the table by hand would ever use.
const (
	StatusRunning  = "running"
	StatusComplete = "complete"
	StatusFailed   = "failed"
)

// MarkerDDL creates the marker table. It is CREATE TABLE IF NOT EXISTS because
// a reload writes a second row into the table the first run made.
const MarkerDDL = `CREATE TABLE IF NOT EXISTS ` + MarkerTable + ` (
  run_id                     uuid PRIMARY KEY,
  tool_version               text        NOT NULL,
  schema_version             integer     NOT NULL,
  started_at                 timestamptz NOT NULL,
  finished_at                timestamptz,
  status                     text        NOT NULL,
  source_fingerprint         text        NOT NULL,
  source_system_id           text,
  schema_fingerprint         text        NOT NULL,
  classification_fingerprint text        NOT NULL,
  root_table                 text        NOT NULL,
  take                       integer     NOT NULL,
  secret_fingerprint         text        NOT NULL,
  rows_loaded                bigint
)`

// MarkerRow is one run's row in the marker table. Every field is a fingerprint,
// an identifier or a count: nothing here is a value from the source, and
// source_fingerprint in particular is sha256(host:port/database)[:16] rather
// than a DSN, so the marker cannot leak where the data came from
// (THREAT_MODEL.md T5).
type MarkerRow struct {
	RunID                     string
	ToolVersion               string
	SchemaVersion             int
	StartedAt                 time.Time
	FinishedAt                *time.Time
	Status                    string
	SourceFingerprint         string
	SourceSystemID            string
	SchemaFingerprint         string
	ClassificationFingerprint string
	RootTable                 string
	Take                      int
	SecretFingerprint         string
	RowsLoaded                *int64
}

const sqlMarkerPresent = `SELECT to_regclass($1) IS NOT NULL`

const sqlLatestMarker = `SELECT run_id::text, tool_version, schema_version, started_at, finished_at, status,
       source_fingerprint, coalesce(source_system_id, ''), schema_fingerprint,
       classification_fingerprint, root_table, take, secret_fingerprint, rows_loaded
FROM ` + MarkerTable + `
ORDER BY started_at DESC, run_id DESC
LIMIT 1`

const sqlInsertMarker = `INSERT INTO ` + MarkerTable + ` (
  run_id, tool_version, schema_version, started_at, status,
  source_fingerprint, source_system_id, schema_fingerprint,
  classification_fingerprint, root_table, take, secret_fingerprint
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

const sqlFinishMarker = `UPDATE ` + MarkerTable + `
SET status = $2, finished_at = now(), rows_loaded = $3
WHERE run_id = $1`

// EnsureMarker creates the marker table if it is not there. The loader calls it
// before the first drop, so that a run which dies part-way still leaves a row at
// StatusRunning and the next run knows to truncate.
func EnsureMarker(ctx context.Context, w pipeline.Writer) error {
	if err := w.Exec(ctx, MarkerDDL); err != nil {
		return fmt.Errorf("pg: creating %s: %w", MarkerTable, err)
	}
	return nil
}

// NewRunID is a run id, made before anything is written.
//
// The marker row is written halfway through the run, but the id it will carry
// is needed earlier than that: the run lease names itself with it on the target
// connection so that a second run refused at the lease can say which run holds
// it (lease.go). core makes one id at the top of the run and it reaches both.
func NewRunID() (string, error) { return uuidV4() }

// StartRun writes this run's row at StatusRunning and returns its run id. Fields
// the caller leaves empty are written as they are; the start time is this
// function's, and so is the run id unless the caller brought one — core does,
// because the run lease named itself with it before the gate ran and a marker
// under a different id would leave the two unlinkable.
func StartRun(ctx context.Context, w pipeline.Writer, row MarkerRow) (string, error) {
	id := row.RunID
	if id == "" {
		made, err := uuidV4()
		if err != nil {
			return "", err
		}
		id = made
	}
	started := time.Now().UTC()
	err := w.Exec(ctx, sqlInsertMarker,
		id, row.ToolVersion, MarkerSchemaVersion, started, StatusRunning,
		row.SourceFingerprint, nullIfEmpty(row.SourceSystemID), row.SchemaFingerprint,
		row.ClassificationFingerprint, row.RootTable, row.Take, row.SecretFingerprint)
	if err != nil {
		return "", fmt.Errorf("pg: recording the run in %s: %w", MarkerTable, err)
	}
	return id, nil
}

// FinishRun moves a run's row to StatusComplete or StatusFailed.
func FinishRun(ctx context.Context, w pipeline.Writer, runID, status string, rowsLoaded int64) error {
	switch status {
	case StatusComplete, StatusFailed:
	default:
		return fmt.Errorf("pg: %q is not a status %s carries", status, MarkerTable)
	}
	if err := w.Exec(ctx, sqlFinishMarker, runID, status, rowsLoaded); err != nil {
		return fmt.Errorf("pg: closing the run in %s: %w", MarkerTable, err)
	}
	return nil
}

// latestMarker reads the most recent row of the marker table.
//
// A marker that is present but cannot be read — a different shape, a table
// somebody else planted under the name, a role that may not select from it — is
// reported as found with a zero row. It is then not bound, and the gate falls
// through to the emptiness check, which is what ARCHITECTURE.md §9 rule 4 says
// happens to a marker that does not pass.
func (t *Target) latestMarker(ctx context.Context, conn *pgxpool.Conn) (MarkerRow, bool, error) {
	var present bool
	if err := conn.QueryRow(ctx, sqlMarkerPresent, MarkerTable).Scan(&present); err != nil {
		return MarkerRow{}, false, fmt.Errorf("pg: gate: looking for %s: %w", MarkerTable, err)
	}
	if !present {
		return MarkerRow{}, false, nil
	}

	var m MarkerRow
	err := conn.QueryRow(ctx, sqlLatestMarker).Scan(
		&m.RunID, &m.ToolVersion, &m.SchemaVersion, &m.StartedAt, &m.FinishedAt, &m.Status,
		&m.SourceFingerprint, &m.SourceSystemID, &m.SchemaFingerprint,
		&m.ClassificationFingerprint, &m.RootTable, &m.Take, &m.SecretFingerprint, &m.RowsLoaded)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return MarkerRow{}, true, fmt.Errorf("pg: gate: reading %s: %w", MarkerTable, err)
		}
		return MarkerRow{}, true, nil
	}
	return m, true, nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// uuidV4 formats sixteen random bytes as a version 4 UUID. It is here rather
// than from a dependency because it is the only UUID lazyslice ever makes, and
// go.mod carries no direct dependency for it.
func uuidV4() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("pg: generating a run id: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32], nil
}
