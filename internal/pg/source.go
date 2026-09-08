// SPDX-License-Identifier: Apache-2.0

package pg

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
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
// Execution of pg_control_system is not granted to PUBLIC, so an unreadable
// identifier is the empty string and not an error: the gate's identity rule
// then rests on the normalised endpoint alone.
//
// It is a statement on the source, so it needs a shape; it is registered here
// rather than in SourceShapes because a run that never asks never sends it.
func (s *Source) SystemID(ctx context.Context) (string, error) {
	if err := s.tr.Register(Shape{Name: "source.system_id", SQL: sqlSystemID}); err != nil {
		return "", err
	}
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return "", fmt.Errorf("pg: acquiring a source connection: %w", err)
	}
	defer conn.Release()

	var id string
	if err := conn.QueryRow(ctx, sqlSystemID).Scan(&id); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return "", err
		}
		return "", nil
	}
	return id, nil
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

const sqlSystemID = `SELECT system_identifier::text FROM pg_control_system()`

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
