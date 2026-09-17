// SPDX-License-Identifier: Apache-2.0

package pg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Liarea/lazyslice/internal/dsn"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// The statements this package itself sends to the source. Each is both the text
// that is executed and the template registered on the allowlist, so the two
// cannot drift: there is one string.
const (
	// sqlBeginReadOnly is written out rather than taken from pgx's own Begin,
	// because the allowlist has to match the text that is sent and pgx's text is
	// pgx's to change.
	sqlBeginReadOnly = `BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY`
	sqlRollback      = `ROLLBACK`

	sqlExportSnapshot = `SELECT pg_export_snapshot()`

	// sqlSetSnapshotShape is a template: the identifier pg_export_snapshot
	// returned cannot be a bind parameter, so it is validated against
	// snapshotIDPattern and then written into the statement.
	sqlSetSnapshotShape = `SET TRANSACTION SNAPSHOT {snapshot}`

	sqlRole = `SELECT current_user, current_setting('is_superuser') = 'on'`

	// sqlIsInRecovery is pg_is_in_recovery(): true on a streaming standby,
	// executable by PUBLIC on every supported version, unlike
	// pg_control_system() and (on a hardened cluster) pg_postmaster_start_time()
	// (T-0241, round-4 red team).
	sqlIsInRecovery = `SELECT pg_is_in_recovery()`

	// sqlWalReceiverSender names the primary a standby is streaming from, when
	// the role may read it: pg_stat_wal_receiver restricts sender_host and
	// sender_port to a superuser or a role holding pg_read_all_stats, and an
	// ordinary role sees them as NULL rather than an error, which is why
	// readReplicaStatus scans them as nullable. The view carries one row while
	// a WAL receiver process is running and none otherwise, which is why this
	// is sent only after sqlIsInRecovery has answered true. sender_port is
	// cast to text for the same reason every other numeric field here is: the
	// source pool runs in pgx.QueryExecModeExec and every value comes back in
	// text format.
	sqlWalReceiverSender = `SELECT sender_host, sender_port::text FROM pg_stat_wal_receiver`

	sqlTablePrivileges = `SELECT n.nspname, c.relname,
       has_table_privilege(c.oid, 'INSERT') OR has_table_privilege(c.oid, 'UPDATE') OR has_table_privilege(c.oid, 'DELETE'),
       has_table_privilege(c.oid, 'SELECT')
FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE c.relkind IN ('r', 'p') AND NOT c.relispartition
  AND n.nspname NOT IN ('pg_catalog', 'information_schema')
  AND n.nspname NOT LIKE 'pg\_toast%' AND n.nspname NOT LIKE 'pg\_temp%'
ORDER BY n.nspname, c.relname`
)

// snapshotIDPattern is the shape pg_export_snapshot returns
// (xmin-xmax-counter, in hex). The identifier is written into a statement
// rather than bound, so it is checked against this before it is.
var snapshotIDPattern = regexp.MustCompile(`\A[0-9A-Fa-f]+-[0-9A-Fa-f]+-[0-9]+\z`)

// SourceShapes are the statements internal/pg sends to the source. A stage
// registers its own on top of these, through Source.Register.
func SourceShapes() []Shape {
	return []Shape{
		{Name: "source.begin", SQL: sqlBeginReadOnly},
		{Name: "source.rollback", SQL: sqlRollback},
		{Name: "source.export_snapshot", SQL: sqlExportSnapshot},
		{Name: "source.set_snapshot", SQL: sqlSetSnapshotShape},
		{Name: "source.role", SQL: sqlRole},
		{Name: "source.table_privileges", SQL: sqlTablePrivileges},
	}
}

// Source is the read side (ARCHITECTURE.md §2). It is defended in three
// independent ways: every transaction it opens is REPEATABLE READ READ ONLY,
// every statement passes the shape allowlist on its Tracer, and it calls
// Query and Exec only — never CopyFrom, SendBatch or Prepare, which
// TestSourceNeverCopiesOrBatches checks by reading this package.
type Source struct {
	pool *pgxpool.Pool
	tr   *Tracer
	ref  dsn.Ref

	mu         sync.Mutex
	holder     *pgxpool.Conn
	snapshotID pipeline.SnapshotID
	serialised bool

	// serial is a one-token semaphore held by whichever Reader is using the
	// holder connection. On the pooled-endpoint fallback every reader is the
	// holder connection, and pgxpool.Conn is not safe for concurrent use: two
	// extract goroutines interleaving extended-protocol messages on one wire
	// corrupt result sets rather than failing cleanly. Extract opens one reader
	// per table (ARCHITECTURE.md §2), so the serialisation ADR-005 promises has
	// to be made here, by making the second Reader wait for the first to close.
	//
	// It is a channel and not a mutex so that a caller whose context is
	// cancelled while waiting gets its error instead of blocking, and because
	// the token is released by whichever goroutine closes the reader.
	serial chan struct{}
}

var _ pipeline.Source = (*Source)(nil)

// OpenSource opens the read side: a pool whose every connection carries the
// shape allowlist, in pgx.QueryExecModeExec so that pgx never prepares a
// statement of its own.
//
// The tracer and the pool are created together and cannot be separated, because
// a source pool without its tracer is a source with no allowlist and nothing
// downstream would notice.
func OpenSource(ctx context.Context, d dsn.DSN, extra ...Shape) (*Source, error) {
	_, r, err := dsn.Parse(string(d))
	if err != nil {
		return nil, err
	}
	tr, err := NewTracer(append(SourceShapes(), extra...)...)
	if err != nil {
		return nil, err
	}
	pool, err := Connect(ctx, d, tr)
	if err != nil {
		return nil, err
	}
	return &Source{pool: pool, tr: tr, ref: r, serial: make(chan struct{}, 1)}, nil
}

// Ref is the redacted identity of the source, for the header and the marker.
func (s *Source) Ref() dsn.Ref { return s.ref }

// Register adds shapes to the allowlist. A stage calls it once, before it
// issues the statements it describes.
func (s *Source) Register(shapes ...Shape) error { return s.tr.Register(shapes...) }

// Trace returns every statement the allowlist saw, for invariant I4.
func (s *Source) Trace() []pipeline.TracedStatement { return s.tr.Trace() }

// Violation returns non-nil when the allowlist refused anything during the run.
// core.Run fails the run on it, so that a swallowed error cannot hide a refusal.
func (s *Source) Violation() error { return s.tr.Violation() }

// Serialised reports that the endpoint could not give a reader of its own —
// a pooler that would not import the exported snapshot, or would not hand out a
// second connection while the holder's transaction pins the only one — so every
// reader is the holder connection and extract runs serialised (ADR-005 "Pooled
// endpoints").
func (s *Source) Serialised() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.serialised
}

// Close releases the pool. It does not end the holder transaction; call Release
// first, and close every Reader, because pgxpool.Close blocks until every
// connection it handed out has come back.
func (s *Source) Close() { s.pool.Close() }

// SystemID reads pg_control_system().system_identifier, which sees through a
// pooler alias and a DNS name to the cluster itself (ARCHITECTURE.md §9 rule 1).
// A role that cannot execute pg_control_system answers "" and not an error:
// the gate's identity rule then rests on the normalised endpoint alone.
//
// The privilege is checked first, in its own statement (sqlCanReadSystemID),
// and pg_control_system() is called only when that check passes (amended
// 2026-09-16, T-0222, R2-06's R3 replay) — see readSystemID's own comment for
// why a guard inside the same statement as the call does not work.
//
// It is a statement on the source, so it needs a shape; it is registered here
// rather than in SourceShapes because a run that never asks never sends it.
//
// It runs inside a REPEATABLE READ READ ONLY transaction like every other
// statement this package sends, and this is the one that used to be the
// exception: it was the only statement this package issued on an autocommit
// path, which is part of why Connect once set default_transaction_read_only on
// the session to cover it. The other part was internal/discover's dial, which
// is outside this package and sent three catalog reads with no BEGIN until
// T-0081 wrapped them in one. That session GUC leaked through a
// transaction-pooling PgBouncer onto the shared server connection and left
// other applications read-only after lazyslice exited (T-0076, pg.go). Scoping
// it here costs a BEGIN and a ROLLBACK on one statement and leaves nothing
// behind, and the Tracer refuses a statement that arrives on an idle source
// connection, so the scoping is checked rather than remembered (T-0082).
func (s *Source) SystemID(ctx context.Context) (string, error) {
	if err := s.tr.Register(
		Shape{Name: "source.system_id.privilege", SQL: sqlCanReadSystemID},
		Shape{Name: "source.system_id", SQL: sqlSystemID},
	); err != nil {
		return "", err
	}
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return "", fmt.Errorf("pg: acquiring a source connection: %w", err)
	}
	if _, beginErr := conn.Exec(ctx, sqlBeginReadOnly); beginErr != nil {
		conn.Release()
		return "", fmt.Errorf("pg: opening a read-only transaction on the source: %w", beginErr)
	}
	defer endTx(context.WithoutCancel(ctx), conn)

	id, err := readSystemID(ctx, conn)
	if err != nil {
		return "", err
	}
	return id, nil
}

// ClusterID is the cluster identity rule 1 falls back to when
// system_identifier is unreadable (readClusterID). It is deliberately a
// *separate* value from SystemID rather than a fallback inside it: SystemID is
// also what §11.2's marker binding is recorded against, and a value that
// changes when the source cluster restarts would refuse a target lazyslice
// itself wrote on the next run.
//
// An unreadable answer is "" and never an error, same as SystemID: the gate
// decides what to do with the absence, and it now fails closed rather than
// trusting a different endpoint spelling.
//
// The system identifier is appended as the identity's last field when this
// role can read it (amended 2026-09-15, R2-06): it is the one value that
// identifies a cluster across a restart, and a field the other side cannot
// read is skipped rather than read as a difference, so carrying it here costs
// a role that lacks EXECUTE on pg_control_system nothing.
func (s *Source) ClusterID(ctx context.Context) (string, error) {
	if err := s.tr.Register(
		Shape{Name: "source.cluster_id.privilege", SQL: sqlCanReadClusterStartTime},
		Shape{Name: "source.cluster_id.savepoint", SQL: sqlSavepointClusterStartTime},
		Shape{Name: "source.cluster_id.start_time", SQL: sqlClusterIDStartTime},
		Shape{Name: "source.cluster_id.release_savepoint", SQL: sqlReleaseSavepointClusterStartTime},
		Shape{Name: "source.cluster_id.rollback_to_savepoint", SQL: sqlRollbackToSavepointClusterStartTime},
		Shape{Name: "source.cluster_id.rest", SQL: sqlClusterIDRest},
	); err != nil {
		return "", err
	}
	systemID, err := s.SystemID(ctx)
	if err != nil {
		return "", err
	}
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return "", fmt.Errorf("pg: acquiring a source connection: %w", err)
	}
	if _, beginErr := conn.Exec(ctx, sqlBeginReadOnly); beginErr != nil {
		conn.Release()
		return "", fmt.Errorf("pg: opening a read-only transaction on the source: %w", beginErr)
	}
	defer endTx(context.WithoutCancel(ctx), conn)

	id, err := readClusterID(ctx, conn, true)
	if err != nil {
		return "", err
	}
	return withSystemID(id, systemID), nil
}

// ReplicaStatus is what Source.Replica reads about the source's own
// replication role (T-0241, round-4 red team:
// docs/reviews/2026-09-15-redteam/round4-still-leaking.json). It answers a
// question the cluster identity cannot: a streaming standby and its primary
// are one cluster by system_identifier and two different postmasters by
// every other field this package reads, so nothing in ClusterID's identity
// can, on its own, say "the database this run just wrote to is the primary
// feeding the database it just read." This is the positive, cheap,
// privilege-free check that says so directly instead.
type ReplicaStatus struct {
	// Standby is pg_is_in_recovery(): true when the source is a streaming
	// standby of some primary.
	Standby bool
	// SenderHost and SenderPort name the primary this standby is streaming
	// from (pg_stat_wal_receiver), each empty when Standby is false or the
	// role may not read it — a role without pg_read_all_stats or superuser
	// sees NULL there, not a permission error.
	SenderHost string
	SenderPort string
}

// Replica reads pg_is_in_recovery() on the source and, when it is true, the
// primary it is streaming from. Like SystemID and ClusterID, an unreadable
// answer is the zero ReplicaStatus and never an error — the caller decides
// what the absence means — and it runs inside its own REPEATABLE READ READ
// ONLY transaction, registered against the allowlist, for the same reason
// SystemID does (T-0076, T-0082): no statement this package sends may reach
// the source outside a transaction.
func (s *Source) Replica(ctx context.Context) (ReplicaStatus, error) {
	if err := s.tr.Register(
		Shape{Name: "source.replica.is_in_recovery", SQL: sqlIsInRecovery},
		Shape{Name: "source.replica.wal_receiver_sender", SQL: sqlWalReceiverSender},
	); err != nil {
		return ReplicaStatus{}, err
	}
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return ReplicaStatus{}, fmt.Errorf("pg: acquiring a source connection: %w", err)
	}
	if _, beginErr := conn.Exec(ctx, sqlBeginReadOnly); beginErr != nil {
		conn.Release()
		return ReplicaStatus{}, fmt.Errorf("pg: opening a read-only transaction on the source: %w", beginErr)
	}
	defer endTx(context.WithoutCancel(ctx), conn)

	return readReplicaStatus(ctx, conn)
}

// readReplicaStatus is Replica's statement sequence, shared by nothing else:
// unlike SystemID and ClusterID, target.go has no reason to ask whether a
// write target is in recovery.
func readReplicaStatus(ctx context.Context, conn *pgxpool.Conn) (ReplicaStatus, error) {
	var st ReplicaStatus
	if err := conn.QueryRow(ctx, sqlIsInRecovery).Scan(&st.Standby); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return ReplicaStatus{}, err
		}
		return ReplicaStatus{}, nil
	}
	if !st.Standby {
		return st, nil
	}
	var host, port sql.NullString
	if err := conn.QueryRow(ctx, sqlWalReceiverSender).Scan(&host, &port); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return ReplicaStatus{}, err
		}
		// pg_is_in_recovery() already answered true, above: this role simply
		// cannot read pg_stat_wal_receiver's sender columns (T-0241's
		// unreadable-not-absent distinction), or the WAL receiver row was
		// gone by the time this ran (a promotion mid-run). Either way the
		// standby fact stands; only the sender is missing.
		return st, nil
	}
	if host.Valid {
		st.SenderHost = host.String
	}
	if port.Valid {
		st.SenderPort = port.String
	}
	return st, nil
}

// Privileges reads what the source role can do (ARCHITECTURE.md §2). Both
// answers change the run: a writable role is the loudest line in the header,
// and an unreadable table is resolved at plan rather than mid-extract.
func (s *Source) Privileges(ctx context.Context) (pipeline.RolePrivileges, error) {
	var priv pipeline.RolePrivileges

	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return priv, fmt.Errorf("pg: acquiring a source connection: %w", err)
	}
	if _, beginErr := conn.Exec(ctx, sqlBeginReadOnly); beginErr != nil {
		conn.Release()
		return priv, fmt.Errorf("pg: opening a read-only transaction on the source: %w", beginErr)
	}
	defer endTx(context.WithoutCancel(ctx), conn)

	if scanErr := conn.QueryRow(ctx, sqlRole).Scan(&priv.Role, &priv.Superuser); scanErr != nil {
		return priv, fmt.Errorf("pg: reading the source role: %w", scanErr)
	}

	rows, err := conn.Query(ctx, sqlTablePrivileges)
	if err != nil {
		return priv, fmt.Errorf("pg: reading table privileges on the source: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var t ref.TableRef
		var writable, readable bool
		if err := rows.Scan(&t.Schema, &t.Name, &writable, &readable); err != nil {
			return priv, fmt.Errorf("pg: reading table privileges on the source: %w", err)
		}
		if writable {
			priv.Writable = append(priv.Writable, t)
		}
		if !readable {
			priv.Unreadable = append(priv.Unreadable, t)
		}
	}
	if err := rows.Err(); err != nil {
		return priv, fmt.Errorf("pg: reading table privileges on the source: %w", err)
	}
	sortTables(priv.Writable)
	sortTables(priv.Unreadable)
	return priv, nil
}

// sqlCanReadSystemID checks EXECUTE on pg_control_system() before
// sqlSystemID ever calls it, as a *separate statement* (amended 2026-09-16,
// T-0222, R2-06's R3 replay). A `CASE WHEN has_function_privilege(...) THEN
// ... ELSE ” END` wrapped around the call in one statement looks like it
// should short-circuit and never was going to work: Postgres checks EXECUTE
// for every function call in a plan at executor-initialisation time, before
// any CASE branch, WHERE clause or scalar subquery around it is evaluated —
// measured by hand against postgres:16, where a role denied EXECUTE still
// got "permission denied for function pg_postmaster_start_time" out of a
// CASE, a scalar subquery and a CTE alike. `internal/pg`'s
// `TestGateRecognisesTheSourceClusterWithPostmasterStartTimeDenied` does not
// exercise any of those three shapes or assert a permission error; it only
// runs the already-split statements end to end. The regression pin for this
// design is `TestTheClusterStartTimeIsGuardedByASeparateStatement`, which
// fails if a future edit recombines the guard and the guarded call into one
// statement. The only way to keep the call out of the plan Postgres builds is
// to keep it out of the *statement*: check the privilege first, in a
// statement that carries no reference to the guarded function at all, and
// only send the statement that does when the check passed.
const sqlCanReadSystemID = `SELECT has_function_privilege('pg_control_system()', 'EXECUTE')`

// sqlSystemID reads system_identifier through pg_control_system(), sent only
// after sqlCanReadSystemID has passed (readSystemID).
const sqlSystemID = `SELECT system_identifier::text FROM pg_control_system()`

// sqlClusterID is the cluster identity an *ordinary* role can read, and it is
// the 2026-09-15 red team's identity-rule-1 finding.
//
// ARCHITECTURE.md §9 rule 1 refuses a target that is the source, by two
// disjuncts: the normalised endpoint, and pg_control_system's
// system_identifier. The second is the only one that survives aliasing — the
// same physical server reached under two published ports, two host spellings,
// or a pooler name — and EXECUTE on pg_control_system is not granted to PUBLIC,
// so it is silently unavailable to exactly the SELECT-only role the tool tells
// operators to create. Under that role, `--source ...:15432/newprod --target
// ...:15433/newprod` against one server passed rule 1 and the run dropped the
// production table.
//
// **Every value here is a property of the cluster, never of the connection**
// (amended 2026-09-15, R2-06). The first answer to that finding was
// pg_postmaster_start_time() with inet_server_addr() and inet_server_port(),
// and those last two are the *connection's* own address and port: they are
// NULL over a unix socket and they differ again behind anything that
// re-dials, so one cluster reached over two transports reported two
// identities and rule 1 fell to whichever spelling the operator used. The
// red team reached one production cluster over its TCP port and over its
// socket and the same-cluster arm never fired.
//
// What is left is the same for every session on the postmaster and readable
// by an ordinary role:
//
//   - pg_postmaster_start_time(), executable by PUBLIC on every supported
//     version and a microsecond timestamp: two clusters that started in the
//     same microsecond is not a case. It is rendered with to_char() in UTC
//     and never cast with ::text, because a timestamptz renders in the
//     *session's* TimeZone GUC — which is set per role (ALTER ROLE), per
//     database (ALTER DATABASE), per DSN (options=-c timezone=) and by PGTZ,
//     and the two sides here are read by two different roles against two
//     different databases. A ::text cast made one postmaster answer
//     '... 19:31:22.87433+00' to one session and '... 15:31:22.87433-04' to
//     another, which is R2-06 again through a session-dependent input.
//   - the oid of the maintenance database (`postgres`). It is initdb-assigned
//     and so distinguishes two clusters only below PostgreSQL 15; from 15 on
//     the oid is pinned and every cluster answers 5, so the field costs
//     nothing and contributes nothing there. Empty when the cluster has no
//     such database, which is legal.
//   - data_directory, read through pg_settings rather than
//     current_setting() because that GUC is superuser-only and
//     current_setting() raises on it for the role §9 recommends; the view
//     simply omits the row instead, which is the empty string here. Any '|'
//     in the path is folded to '_' so the field separator stays a separator.
//   - the server version, which pins the answer further at no cost.
//
// Under the SELECT-only role §9 recommends, on PostgreSQL 15 or newer, that
// leaves the postmaster start time and the server version and nothing else:
// data_directory is superuser-only and the maintenance oid is pinned. Say so
// rather than claim more for the identity than it has — two sibling containers
// from one `docker compose up` are told apart by the start time alone, and a
// collision there reads as "same cluster", which over-refuses rather than
// admitting the source.
//
// Source.ClusterID appends system_identifier as a fifth field when the role
// can read it (target.go's clusterIdentity does the same on the other side).
// sameClusterIdentity *decides* on that field when both sides filled it and
// falls through to the weaker positional fields only when one side could not
// read it, so a cluster that restarted between the two reads, or whose two
// sessions render a value differently, is still recognised as one cluster.
//
// **The postmaster start time is behind its own privilege check, in its own
// statement, and the other three fields are a second statement that carries
// no privileged function call at all** (amended 2026-09-16, T-0222, R2-06's
// R3 replay: docs/reviews/2026-09-15-redteam/round3-still-leaking.json). This
// used to be one SELECT concatenating all four fields with `||`, and a role
// denied EXECUTE on pg_postmaster_start_time() — a hardened production
// cluster that revokes monitoring functions from PUBLIC, the same class of
// hardening the original R2-06 finding's premise already assumed for
// pg_control_system — made the *whole* SELECT raise a permission error, which
// Source.ClusterID and target.go's clusterIdentity both caught and answered
// as "". That wiped out the maintenance-database oid, data_directory and the
// server version too, none of which needs any privilege this role did not
// already have, and turned a cluster whose identity was three-quarters
// readable into one the gate could not compare at all — exactly the "unknown"
// state rule 1's alias arm falls back from into an endpoint-spelling
// comparison, which is what an alias, a second published port or a different
// transport changes. Guarding the call with `CASE WHEN
// has_function_privilege(...)` inside that one statement does not fix it —
// see sqlCanReadSystemID's comment: Postgres checks EXECUTE for the whole
// plan before any branch of it runs. Splitting the guarded field into its own
// statement, sent only after sqlCanReadClusterStartTime has passed
// (readClusterID), is what makes the degradation real.
const (
	sqlCanReadClusterStartTime = `SELECT has_function_privilege('pg_postmaster_start_time()', 'EXECUTE')`

	// sqlClusterIDStartTime is the field sqlCanReadClusterStartTime guards.
	sqlClusterIDStartTime = `SELECT to_char(pg_postmaster_start_time() AT TIME ZONE 'UTC', 'YYYY-MM-DD HH24:MI:SS.US')`

	// sqlClusterIDRest is the three fields that carry no privilege check:
	// every role that can connect can read all three, so they are always sent
	// and readClusterID prepends sqlClusterIDStartTime's field (or "") ahead
	// of them with clusterIDSep, preserving the positional layout below.
	sqlClusterIDRest = `SELECT coalesce((SELECT oid::text FROM pg_database WHERE datname = 'postgres'), '')
  || '|' || coalesce(replace((SELECT setting FROM pg_settings WHERE name = 'data_directory'), '|', '_'), '')
  || '|' || current_setting('server_version')`
)

// The three SAVEPOINT statements readClusterID takes around
// sqlClusterIDStartTime when it is called inside an open transaction
// (Source.ClusterID). A fixed name is safe here: readClusterID never nests —
// Source.ClusterID's transaction runs one at a time on its own connection —
// so there is never a second SAVEPOINT of this name to collide with.
const (
	sqlSavepointClusterStartTime           = `SAVEPOINT cluster_start_time`
	sqlReleaseSavepointClusterStartTime    = `RELEASE SAVEPOINT cluster_start_time`
	sqlRollbackToSavepointClusterStartTime = `ROLLBACK TO SAVEPOINT cluster_start_time`
)

// readSystemID runs sqlCanReadSystemID and, only when it passes,
// sqlSystemID — see sqlCanReadSystemID's comment for why the check has to be
// a separate statement. conn must already have a transaction open when the
// caller needs one (Source.SystemID's REPEATABLE READ READ ONLY); the gate's
// probes on the target run it on an ungrouped autocommit connection, which is
// just as safe here because neither statement writes anything.
//
// An error other than the context ending is read as "unreadable", the same
// answer as a failed privilege check: the caller does not distinguish why a
// field is missing, only that it is.
func readSystemID(ctx context.Context, conn *pgxpool.Conn) (string, error) {
	var can bool
	if err := conn.QueryRow(ctx, sqlCanReadSystemID).Scan(&can); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return "", err
		}
		return "", nil
	}
	if !can {
		return "", nil
	}
	var id string
	if err := conn.QueryRow(ctx, sqlSystemID).Scan(&id); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return "", err
		}
		return "", nil
	}
	return id, nil
}

// readClusterID runs sqlCanReadClusterStartTime and, only when it passes,
// sqlClusterIDStartTime, then always sqlClusterIDRest, and joins the two into
// the positional identity sameClusterIdentity compares. A role denied the
// start time still contributes the other three fields; a role denied
// everything answers "" the same way it always has. Shared by
// Source.ClusterID (its own REPEATABLE READ READ ONLY transaction) and
// target.go's clusterIdentity (the gate's ungrouped probes), which is what
// keeps the two sides of the comparison reading the same fields the same way.
//
// inTransaction must be true only when conn already has a transaction open —
// Source.ClusterID's case — and false when it does not — target.go's
// clusterIdentity, called on the gate's autocommit connection, where every
// statement is its own implicit transaction and one failing never touches the
// next. Inside an open transaction, a failed sqlClusterIDStartTime — the
// privilege revoked between the two statements, or any other server error —
// leaves the transaction aborted, and sqlClusterIDRest sent on it next would
// itself fail with 25P02 "current transaction is aborted" and be read as
// "unreadable", losing the whole identity rather than the one field the
// privilege check could not guarantee. A SAVEPOINT taken before
// sqlClusterIDStartTime and released or rolled back to afterward keeps that
// failure scoped to the one statement, so sqlClusterIDRest still runs on a
// transaction that is not aborted (T-0222 review round).
func readClusterID(ctx context.Context, conn *pgxpool.Conn, inTransaction bool) (string, error) {
	var canStartTime bool
	if err := conn.QueryRow(ctx, sqlCanReadClusterStartTime).Scan(&canStartTime); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return "", err
		}
		return "", nil
	}
	var startTime string
	if canStartTime {
		if inTransaction {
			if _, err := conn.Exec(ctx, sqlSavepointClusterStartTime); err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return "", err
				}
				return "", nil
			}
		}
		if err := conn.QueryRow(ctx, sqlClusterIDStartTime).Scan(&startTime); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return "", err
			}
			// The privilege check just said yes; a read that still fails is
			// treated as "this field is unreadable", the same as a check that
			// said no, rather than losing the fields sqlClusterIDRest can
			// still supply.
			startTime = ""
			if inTransaction {
				// The failed statement aborted the transaction. Roll back to
				// the savepoint taken above it so sqlClusterIDRest below runs
				// on a transaction that is open again, instead of itself
				// failing with 25P02 and losing the rest of the identity too.
				if _, rbErr := conn.Exec(ctx, sqlRollbackToSavepointClusterStartTime); rbErr != nil {
					if errors.Is(rbErr, context.Canceled) || errors.Is(rbErr, context.DeadlineExceeded) {
						return "", rbErr
					}
					return "", nil
				}
			}
		} else if inTransaction {
			if _, err := conn.Exec(ctx, sqlReleaseSavepointClusterStartTime); err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return "", err
				}
				return "", nil
			}
		}
	}
	var rest string
	if err := conn.QueryRow(ctx, sqlClusterIDRest).Scan(&rest); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return "", err
		}
		return "", nil
	}
	return startTime + clusterIDSep + rest, nil
}

// sameClusterIdentity compares two cluster identities.
//
// It is not string equality, because the two sides are read by two roles and
// a field one role may not read is the empty string rather than a different
// value: system_identifier is readable by a superuser target and not by the
// SELECT-only source role §9 recommends, and data_directory is the same shape
// of privilege.
//
// The comparison is hierarchical, not a vote (amended 2026-09-15, T-0190 fix
// round). system_identifier is the strongest identity a cluster has and the
// only one that survives a restart, so when both sides filled it, it decides
// alone: equal is one cluster, unequal is two, and either way the answer is
// known. Giving every field an equal veto meant a matching system identifier
// was outvoted by any disagreement in a weaker field — a postmaster restart
// between the source read and the gate's read changes the start time while the
// system identifier is unchanged, and the gate stopped recognising the source's
// own cluster.
//
// Only when system_identifier is missing on either side do the weaker
// positional fields decide: each is skipped when it is empty on either side,
// and a field both sides filled and disagree on is two clusters. known is
// false when nothing could be compared at all, and the gate reads that as
// "unknown", which fails closed.
//
// Not every positional field is evidence of sameness, and not every field is
// unconditionally evidence of difference either (T-0222 review round; the
// carve-outs below are T-0241, the round-4 red team, and T-0255, the round-5
// red team). data_directory is specific enough to a running cluster that two
// clusters agreeing on it is not a case worth worrying about, and — outside
// the standby carve-out below — disagreeing on it is real evidence of
// difference; the maintenance database's oid and the server version prove
// neither — the oid is pinned at 5 for every cluster from PostgreSQL 15 on
// (clusterIDWeakField), and the version is shared by every cluster built from
// the same image, so agreement on either is not evidence of one cluster.
// Two identities whose only common fields are those two, and which agree on
// both, have proven nothing about sameness: known stays false and the gate
// reads it as unknown, the same fail-closed direction as no common field at
// all, rather than a confident "same cluster" a role denied the start time (a
// hardened cluster that revokes pg_postmaster_start_time from PUBLIC) could
// produce against any other cluster built from the same image. Disagreement
// on either weak field still decides "different cluster" on its own, because
// a mismatch needs no specificity to be believed.
//
// **The postmaster start time is one field whose disagreement is never
// believed (T-0241, docs/reviews/2026-09-15-redteam/round4-still-leaking.json).**
// A primary and its own streaming standby are two postmasters of one
// cluster, and a standby's start time is necessarily later than its
// primary's — it started when pg_basebackup finished, not when the cluster
// did — so a start-time mismatch is not evidence the two clusters differ.
// Unconditionally, on every source: a start-time mismatch contributes
// nothing at all, neither same nor known is touched by it, the same as a
// field neither side filled. Agreement on the start time is unaffected and
// remains as strong evidence of sameness as ever — two independent
// postmasters starting in the same microsecond is not a case. The
// consequence is the one ARCHITECTURE.md §9's fail-closed reasoning already
// states for the oid and the version, read in the opposite direction: when
// system_identifier is missing on either side and the start time is the
// only field that disagrees, the verdict downgrades to unknown rather than
// a confident "different cluster" — which is what let a run whose --source
// was a standby and whose --target was a database on that standby's own
// primary print no warning and write there.
//
// **data_directory joins it, but only conditionally — only when the source
// is a standby (T-0255, round-5 red team, below).** On an ordinary,
// non-standby source, data_directory keeps deciding "different cluster" on
// disagreement, the same as every field but the start time: an unrelated
// cluster has no reason to share a data directory with this one, so a
// mismatch is still real evidence. A co-hosted standby and its primary are
// the exception — pg_basebackup into a second directory beside its source
// is the ordinary shape of standing one up, so the two disagree on
// data_directory by construction, and under a role that can read it
// (pg_read_all_settings, a monitoring-grade grant) that disagreement used to
// read as confidently "different cluster" even after this start-time
// carve-out closed the same gap for the weaker field. Unlike the start
// time, whose disagreement is discounted for every source, data_directory's
// is discounted only when standby is true.
//
// Fields are positional, so a field may be added only at the end — and
// clusterIDSystemIDField must move with it.
//
// standby is Source.Replica's Standby answer for this run (T-0255, round-5
// red team: docs/reviews/2026-09-15-redteam/round5-still-leaking.json,
// "the standby data_directory variant") — the carve-out the paragraph above
// describes. It is read only, never called from a place that lacks it, and
// clusterIDDifferenceUnreliableField's own comment below is the most precise
// place for the full reasoning.
func sameClusterIdentity(a, b string, standby bool) (same, known bool) {
	if a == "" || b == "" {
		return false, false
	}
	fa := strings.Split(a, clusterIDSep)
	fb := strings.Split(b, clusterIDSep)
	if sa, sb := clusterIDField(fa, clusterIDSystemIDField), clusterIDField(fb, clusterIDSystemIDField); sa != "" && sb != "" {
		return sa == sb, true
	}
	n := len(fa)
	if len(fb) < n {
		n = len(fb)
	}
	same = true
	decisive := false
	for i := 0; i < n; i++ {
		if fa[i] == "" || fb[i] == "" {
			continue
		}
		if fa[i] == fb[i] {
			known = true
			if !clusterIDWeakField(i) {
				decisive = true
			}
			continue
		}
		if clusterIDDifferenceUnreliableField(i, standby) {
			// The start time, always; data_directory too when the source is
			// a standby: a standby's postmaster always started later than
			// its primary's, and a standby's data directory always differs
			// from its primary's on a co-hosted pair. A mismatch on either,
			// under the condition that makes it unreliable, is not evidence
			// the clusters differ. Read it exactly as a field neither side
			// filled — it decides nothing, in either direction.
			continue
		}
		known = true
		same = false
		if !clusterIDWeakField(i) {
			decisive = true
		}
	}
	if !known {
		return false, false
	}
	if same && !decisive {
		// Every field that could be compared was a weak one, and they all
		// agreed. That is not evidence of one cluster — only disagreement on
		// a weak field would have been — so this is reported as unknown
		// rather than a "same cluster" the fields do not support.
		return false, false
	}
	return same, true
}

// clusterIDStartTimeField is the postmaster start time's position — always
// one of clusterIDDifferenceUnreliableField's fields.
const clusterIDStartTimeField = 0

// clusterIDMaintenanceOIDField, clusterIDDataDirectoryField and
// clusterIDServerVersionField are the positions clusterIDWeakField and (for
// the data directory) clusterIDDifferenceUnreliableField name.
const (
	clusterIDMaintenanceOIDField = 1
	clusterIDDataDirectoryField  = 2
	clusterIDServerVersionField  = 3
)

// clusterIDWeakField reports whether positional field i can only prove two
// cluster identities different, never that they are the same. The
// maintenance database's oid (position 1) is initdb-assigned only below
// PostgreSQL 15; from 15 on it is pinned at 5 for every cluster, so two
// clusters agreeing on it says nothing about them being the same cluster —
// and the server version (position 3) is shared by every cluster built from
// the same image, most obviously two sibling containers from one `docker
// compose up`. Disagreement on either is still real: a version mismatch, or
// an oid that differs below PostgreSQL 15, proves two different clusters.
func clusterIDWeakField(i int) bool {
	return i == clusterIDMaintenanceOIDField || i == clusterIDServerVersionField
}

// clusterIDDifferenceUnreliableField reports whether positional field i can
// only prove two cluster identities the same, never that they differ — the
// mirror image of clusterIDWeakField. The postmaster start time (position 0)
// is unconditionally one of them (T-0241, round-4 red team:
// docs/reviews/2026-09-15-redteam/round4-still-leaking.json). It is
// otherwise the strongest field sameClusterIdentity has short of
// system_identifier itself, which is exactly why an unqualified
// disagreement on it used to decide "different cluster" outright — and
// exactly why that was wrong for a primary and its own streaming standby,
// whose start times can never agree: the standby's postmaster started when
// pg_basebackup finished, strictly after the primary's. Agreement is
// unaffected: two independent postmasters starting in the same microsecond
// is not a case worth worrying about, so a match here remains as decisive as
// any other non-weak field.
//
// data_directory (position 2) joins it, but only when standby is true
// (T-0255, round-5 red team: docs/reviews/2026-09-15-redteam/round5-still-leaking.json,
// "the standby data_directory variant"). T-0241's own amendment said
// data_directory "still decides on disagreement, because nothing about a
// standby forces its data directory to disagree with its primary's the way
// the start time is forced to" — which is false for the ordinary way a
// standby is actually stood up: pg_basebackup into a second directory
// beside its source is the normal shape of a co-hosted standby and primary,
// and any such pair disagrees on data_directory by necessity. A monitoring
// role (pg_read_all_settings) that can read it therefore reached a confident
// "different cluster" on data_directory alone, exactly the case this
// function's start-time carve-out already exists to prevent, wide open again
// through a second field. Unlike the start time, data_directory's
// disagreement is only unreliable *conditionally*: a source that is not a
// standby has no reason at all to share a data directory with an unrelated
// cluster, so TestADataDirectoryDisagreementStillProvesTwoClusters still
// requires the non-standby case to stay decisive — widening this
// unconditionally, the way an earlier draft of this fix did, would have
// made that test pass for the wrong reason (every field becomes unreliable,
// so nothing this function guards can ever prove difference).
func clusterIDDifferenceUnreliableField(i int, standby bool) bool {
	if i == clusterIDStartTimeField {
		return true
	}
	return standby && i == clusterIDDataDirectoryField
}

// clusterIDField reads one positional field, or "" when the identity is shorter
// than that. A short identity is not an error here: an older field order is
// only ever extended at the end, and a missing field is "could not read it".
func clusterIDField(fields []string, i int) string {
	if i >= len(fields) {
		return ""
	}
	return fields[i]
}

// clusterIDSystemIDField is the position withSystemID appends the system
// identifier at: start time, maintenance oid, data directory, server version,
// system identifier.
const clusterIDSystemIDField = 4

// clusterIDSep separates the fields of a cluster identity. It is a character
// no field can contain: sqlClusterIDRest folds it out of data_directory, and
// every other field is a timestamp, an oid, a version or a system identifier.
const clusterIDSep = "|"

// withSystemID appends the system identifier to a cluster identity as its last
// field. An unreadable identifier is the empty string, which sameClusterIdentity
// skips rather than reads as a difference.
func withSystemID(clusterID, systemID string) string {
	if clusterID == "" {
		return ""
	}
	return clusterID + clusterIDSep + systemID
}

func sortTables(ts []ref.TableRef) {
	sort.Slice(ts, func(i, j int) bool {
		if ts[i].Schema != ts[j].Schema {
			return ts[i].Schema < ts[j].Schema
		}
		return ts[i].Name < ts[j].Name
	})
}

// Snapshot opens the holder transaction and exports its snapshot. The
// transaction is held from introspect to the end of extract and released before
// load, which is what bounds the xmin pin on the source (THREAT_MODEL.md T9).
func (s *Source) Snapshot(ctx context.Context) (pipeline.SnapshotID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.holder != nil {
		return s.snapshotID, nil
	}

	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return "", fmt.Errorf("pg: acquiring the snapshot holder connection: %w", err)
	}
	if _, err := conn.Exec(ctx, sqlBeginReadOnly); err != nil {
		conn.Release()
		return "", fmt.Errorf("pg: opening the holder transaction: %w", err)
	}
	var id string
	if scanErr := conn.QueryRow(ctx, sqlExportSnapshot).Scan(&id); scanErr != nil {
		endTx(context.WithoutCancel(ctx), conn)
		return "", fmt.Errorf("pg: exporting the snapshot: %w", scanErr)
	}
	if !snapshotIDPattern.MatchString(id) {
		endTx(context.WithoutCancel(ctx), conn)
		return "", fmt.Errorf("pg: the server returned a snapshot identifier of an unexpected shape (%d bytes)", len(id))
	}

	s.holder = conn
	s.snapshotID = pipeline.SnapshotID(id)
	return s.snapshotID, nil
}

// Reader opens a connection that imports the run's snapshot. An endpoint that
// cannot give one — a pooler that will not import the snapshot, or one that
// will not hand out a second server connection while the holder's transaction
// pins the only one — gets the holder connection itself and the run is
// serialised (ADR-005 "Pooled endpoints"), which is slower and still one
// consistent database; extracting without a snapshot is not an option.
//
// Both halves of that condition are the fallback because both are the same
// answer from the endpoint. Until they were, a pooler configured with one
// server connection failed the run with its own query_wait_timeout instead of
// serialising, which is the topology --single-connection exists for.
//
// On that fallback the reader holds the source's one serialisation token, and a
// second Reader blocks until the first is closed. A caller that opens a reader
// per table therefore still gets one reader at a time on the holder connection
// rather than several goroutines writing to one wire. Serialised reports the
// mode, but only once the first fallback has happened: the endpoint says nothing
// about whether it can import a snapshot until one is offered to it, so the
// waiting is the guarantee and the flag is only the report.
func (s *Source) Reader(ctx context.Context, id pipeline.SnapshotID) (pipeline.Reader, error) {
	if !snapshotIDPattern.MatchString(string(id)) {
		return nil, errors.New("pg: reader: the snapshot identifier is not one pg_export_snapshot produced")
	}
	s.mu.Lock()
	holder := s.holder
	s.mu.Unlock()
	if holder == nil {
		return nil, errors.New("pg: reader: no snapshot is held; call Snapshot first")
	}

	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		// No second connection at all is the same answer as a snapshot the
		// endpoint will not import, and it is what a restrictive pooler actually
		// says: PgBouncer with one server connection holds the acquire (or the
		// BEGIN below) until query_wait_timeout and then errors, because the
		// holder's transaction has that connection pinned. Reading only the
		// import failure as "this endpoint cannot give a reader" left
		// --single-connection's automatic half (ADR-005 "Pooled endpoints")
		// unreachable on exactly the topology it exists for, so the fallback
		// covers this too. The reader that comes back is the holder, inside the
		// run's own snapshot, so the slice is the same one either way.
		return s.serialisedReader(ctx, holder, err, "acquiring a source reader")
	}
	if _, beginErr := conn.Exec(ctx, sqlBeginReadOnly); beginErr != nil {
		conn.Release()
		return s.serialisedReader(ctx, holder, beginErr, "opening a read-only transaction on the source")
	}
	if _, importErr := conn.Exec(ctx, `SET TRANSACTION SNAPSHOT '`+string(id)+`'`); importErr != nil {
		endTx(context.WithoutCancel(ctx), conn)
		return s.serialisedReader(ctx, holder, importErr, "importing the snapshot")
	}
	return &reader{conn: conn, tr: s.tr, own: true}, nil
}

// serialisedReader is the fallback: cause is why a reader of its own could not
// be opened, and what is handed back instead is the holder connection, one
// caller at a time.
//
// A cancelled or expired context is never a pooler. The allowlist refuses a
// statement by cancelling the context (Tracer), and a caller that went away
// cancels its own, so either would otherwise be read as "this endpoint cannot
// give a reader" and turn a refused statement into a silently serialised run.
// Those two are returned as themselves, wrapped in what was being attempted.
//
// Every other cause is a fallback and not an error, and the cause is not
// reported any further: on this path the run continues against the holder, in
// the run's own snapshot, and Serialised() is what says so (§8's
// --single-connection, ADR-005 "Pooled endpoints"). A cause that is really the
// source going away rather than a pooler refusing a second connection is not
// hidden by that — the holder is on the same endpoint, so the first read
// through it fails with the server's own error.
func (s *Source) serialisedReader(ctx context.Context, holder *pgxpool.Conn, cause error, doing string) (pipeline.Reader, error) {
	if errors.Is(cause, context.Canceled) || errors.Is(cause, context.DeadlineExceeded) {
		return nil, fmt.Errorf("pg: %s: %w", doing, cause)
	}
	s.mu.Lock()
	s.serialised = true
	s.mu.Unlock()
	select {
	case s.serial <- struct{}{}:
	case <-ctx.Done():
		return nil, fmt.Errorf("pg: waiting for the serialised source reader: %w", ctx.Err())
	}
	return &reader{conn: holder, tr: s.tr, own: false, serial: s.serial}, nil
}

// Short opens a fresh, short REPEATABLE READ READ ONLY transaction on a new
// snapshot. Only verify uses it, for residual confirmation and the unmasked
// sample comparison, and only after the run snapshot has been released
// (ARCHITECTURE.md §6).
func (s *Source) Short(ctx context.Context) (pipeline.Reader, error) {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("pg: acquiring a short source transaction: %w", err)
	}
	if _, err := conn.Exec(ctx, sqlBeginReadOnly); err != nil {
		conn.Release()
		return nil, fmt.Errorf("pg: opening a short read-only transaction on the source: %w", err)
	}
	return &reader{conn: conn, tr: s.tr, own: true}, nil
}

// Release ends the holder transaction and returns its connection to the pool.
// It is called at the end of extract, before load and verify.
func (s *Source) Release(ctx context.Context) error {
	s.mu.Lock()
	holder := s.holder
	s.holder = nil
	s.snapshotID = ""
	s.mu.Unlock()
	if holder == nil {
		return nil
	}
	_, err := holder.Exec(ctx, sqlRollback)
	holder.Release()
	if err != nil {
		return fmt.Errorf("pg: ending the holder transaction: %w", err)
	}
	return nil
}

// reader is one read-only transaction on the source. own is false for the
// serialised fallback, where the connection is the holder's and closing the
// reader must not end the run's snapshot — it must hand the holder back
// instead, which is what serial carries.
type reader struct {
	conn *pgxpool.Conn
	tr   *Tracer
	own  bool
	// serial is Source.serial when this reader holds the holder connection, and
	// nil otherwise. Close returns the token to it exactly once.
	serial   chan struct{}
	released sync.Once
}

var _ pipeline.Reader = (*reader)(nil)

func (r *reader) Query(ctx context.Context, sql string, args ...any) (pipeline.Rows, error) {
	// In pgx.QueryExecModeExec the error of a query that never ran surfaces on
	// Rows.Err rather than here, so a refused statement would look to a careless
	// caller like a table with no rows. The allowlist counts its refusals, so
	// the refusal is turned back into an error at the call that caused it.
	//
	// The count is the tracer's and not this reader's, so a refusal on another
	// connection in the same instant is attributed here. That is the direction
	// that fails the run rather than continuing it.
	before := 0
	if r.tr != nil {
		before = r.tr.Violations()
	}
	rows, err := r.conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	if r.tr != nil && r.tr.Violations() > before {
		rows.Close()
		return nil, fmt.Errorf("pg: the source refused a statement: %w", ErrRefused)
	}
	return rows, nil
}

func (r *reader) Close(ctx context.Context) error {
	if !r.own {
		// The serialised fallback. The connection is the holder's, so the
		// transaction stays open; what ends here is this reader's exclusive use
		// of it, which is what lets the next Reader proceed.
		r.released.Do(func() {
			if r.serial != nil {
				<-r.serial
			}
		})
		return nil
	}
	_, err := r.conn.Exec(ctx, sqlRollback)
	r.conn.Release()
	if err != nil {
		return fmt.Errorf("pg: closing a source reader: %w", err)
	}
	return nil
}

// endTx ends a read-only transaction and returns its connection to the pool.
//
// It reports nothing because there is nothing to report: the statement it is
// undoing read rows and wrote none. A rollback that fails, though, leaves the
// connection in an unknown transaction state, and that connection must not go
// back to the pool for the next reader to inherit — so it is closed first, and
// pgxpool discards a closed connection on release.
func endTx(ctx context.Context, conn *pgxpool.Conn) {
	if _, err := conn.Exec(ctx, sqlRollback); err != nil {
		discard(ctx, conn)
	}
	conn.Release()
}

// discard closes the connection under conn so that pgxpool throws it away when
// it is released, instead of handing it to the next caller.
//
// It is the second half of the rollback discipline above, split out because the
// gate's fingerprint transaction ends on a connection it does not own the
// release of (Target.catalogFingerprint): the caller there releases, so this
// closes and nothing else. A close that itself fails leaves the connection in
// whatever state it was already in and is not worth reporting over the error
// that got here; the release still happens.
func discard(ctx context.Context, conn *pgxpool.Conn) {
	if c := conn.Conn(); c != nil && !c.IsClosed() {
		_ = c.Close(ctx)
	}
}

// pgx.Rows already has exactly the four methods pipeline.Rows names, so the
// read side needs no adapter. This assertion is what says so.
var _ pipeline.Rows = (pgx.Rows)(nil)
