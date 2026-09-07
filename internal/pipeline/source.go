// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"context"
	"time"

	"github.com/Liarea/lazyslice/internal/dsn"
	"github.com/Liarea/lazyslice/internal/event"
)

// SnapshotID is the identifier pg_export_snapshot returns. Every reader in a
// run imports the same one, so introspect, plan and extract see one consistent
// database.
type SnapshotID string

// RolePrivileges is what the source role can do. It is read once, before
// anything else, because both answers it carries change the run: a writable
// role prints the loudest warning in the header, and an unreadable table is
// resolved at plan rather than mid-extract.
type RolePrivileges struct {
	Role      string
	Superuser bool
	// Writable lists tables where the role has INSERT, UPDATE or DELETE.
	Writable []TableRef
	// Unreadable lists tables where has_table_privilege(..., 'SELECT') is false.
	// internal/core reads these privileges once, before the snapshot is opened,
	// and hands them to the planner on PlanRequest.Priv; the planner applies
	// section 3.6 to them and asks the catalog nothing of its own. Extract never
	// consumes them, because by then the question is already settled.
	Unreadable []TableRef
}

// Source is the read side. Every transaction it opens is REPEATABLE READ READ
// ONLY and every statement passes the shape allowlist (ADR-005 "Source"): all
// five pgx tracers are registered on the source pool, and a statement whose
// shape is not registered gets a cancelled context from TraceQueryStart and a
// recorded violation.
//
// Source code paths use Query only. TestSourceNeverCopiesOrBatches fails if
// internal/pg calls CopyFrom, SendBatch or Prepare on a Source connection.
type Source interface {
	Privileges(ctx context.Context) (RolePrivileges, error)
	// Snapshot opens the holder transaction and calls pg_export_snapshot.
	Snapshot(ctx context.Context) (SnapshotID, error)
	// Reader opens a connection that runs SET TRANSACTION SNAPSHOT id. With a
	// pooler that cannot import a snapshot, Reader returns the holder connection
	// itself and the run is serialised (ADR-005 "Pooled endpoints").
	Reader(ctx context.Context, id SnapshotID) (Reader, error)
	// Short opens a fresh, short REPEATABLE READ READ ONLY transaction on a new
	// snapshot. Only verify uses it, for residual confirmation and the unmasked
	// sample comparison, and only after the run snapshot has been released.
	Short(ctx context.Context) (Reader, error)
	// Release ends the holder transaction.
	Release(ctx context.Context) error
	// Trace returns every statement the allowlist saw, for invariant I4.
	Trace() []TracedStatement
}

// TracedStatement is one entry in the source statement trace. SQL carries the
// statement text with parameters elided; it never carries a value.
type TracedStatement struct {
	At time.Time
	// Shape is the registered shape's name, such as "plan.child_keys". It is
	// empty when the statement was refused.
	Shape   string
	SQL     string
	Refused bool
}

// Reader exposes only Query. This is authorship hygiene, not enforcement:
// Query executes any SQL, so read-only is enforced by the tracer and the READ
// ONLY transaction, not by this type.
type Reader interface {
	Query(ctx context.Context, sql string, args ...any) (Rows, error)
	Close(ctx context.Context) error
}

// Rows is the driver-independent result set. It is deliberately the smallest
// surface that the stages need, so that a second engine (ADR-003) is a new
// implementation rather than a new interface.
type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close()
}

// Verdict is tri-state so that "not probed" can never be read as eligible. The
// zero value is NotProbed.
type Verdict int

// The three verdicts. Anything that is not Eligible refuses the write.
const (
	NotProbed Verdict = iota
	Refused
	Eligible
)

// Eligibility is the gate's answer about a target (ARCHITECTURE.md section 9).
type Eligibility struct {
	Verdict Verdict
	// Reason is rendered by the sink; every refusal reason is a row in
	// docs/ERRORS.md.
	Reason      event.Code
	RowCounts   map[TableRef]int64 // filled when refused for rows
	TableCount  int                // filled when refused for the table cap
	Marked      bool               // lazyslice_meta present
	MarkerBound bool               // marker present, readable, and bound to this source and this catalog
	PrevKeyFP   string             // from the marker, "" when unbound
	PrevClassFP string             // from the marker, "" when unbound
	// PrevToolVersion is the marker's tool_version, "" when unbound or when the
	// gate did not read it. ARCHITECTURE.md section 11.2 requires three warnings
	// on a bound marker written by another run — secret changed, classification
	// changed, tool version changed — and this is the third one's input;
	// internal/core compares it and emits target.marker.tool_changed. It stays
	// empty until internal/pg fills it, and an empty one prints nothing.
	PrevToolVersion string
	// SameCluster reports that system_identifier equals the source's. It is a
	// printed warning, never a refusal: app and app_test in one container is the
	// common compose setup.
	SameCluster bool
	// Local is Candidate.Local. A remote target needs --allow-remote-target HOST.
	Local bool
}

// Target is the write side.
type Target interface {
	// Gate implements ADR-005 "Target". It returns Verdict == Eligible only when
	// every probe ran to completion; any error, timeout or unprobed table is
	// Refused, never NotProbed treated as ok. allowRemoteHost is the value of
	// --allow-remote-target, "" when absent.
	Gate(ctx context.Context, source dsn.Ref, sourceSystemID, allowRemoteHost string) (Eligibility, error)
	Writer(ctx context.Context) (Writer, error)
}

// Writer is the target connection. Only the loader and the verifier hold one,
// and only after the gate has passed.
type Writer interface {
	Exec(ctx context.Context, sql string, args ...any) error
	CopyFrom(ctx context.Context, table TableRef, cols []string, rows <-chan []any) (int64, error)
	Begin(ctx context.Context) (Tx, error)
}

// Tx is one target transaction. The loader opens one per table at Seq 0 and
// commits it at Last (see RowBatch).
type Tx interface {
	Exec(ctx context.Context, sql string, args ...any) error
	CopyFrom(ctx context.Context, table TableRef, cols []string, rows <-chan []any) (int64, error)
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}
