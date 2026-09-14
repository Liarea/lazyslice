// SPDX-License-Identifier: Apache-2.0

package pg

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Liarea/lazyslice/internal/event"
)

// CodeLeaseHeld is exit 4: another lazyslice run holds the target
// (THREAT_MODEL.md T2, amended 2026-09-14). It is a refusal like every other
// gate refusal, and it happens before the gate's first probe, because a verdict
// reached while another run is dropping the same tables is a verdict about a
// database that is being taken apart underneath it.
const CodeLeaseHeld event.Code = "target.refused.lease_held"

// leaseNamespace makes the advisory-lock key ours. Advisory locks are one
// 64-bit space shared by every application on the database, so a key that was
// merely a hash of the database name could collide with somebody else's
// convention; hashing a namespace with it means the only thing that can hold
// this key is another lazyslice.
const leaseNamespace = "lazyslice:target-lease:"

// leaseAppPrefix is what the lease transaction puts in application_name, so that
// a second run can say *which* run holds the target instead of only that one
// does. application_name is the one piece of state a holder can leave where
// another session can read it without any privilege of its own beyond seeing
// sessions of its own role, and a run id is an identifier, not a value
// (THREAT_MODEL.md T4).
const leaseAppPrefix = "lazyslice run "

const (
	// sqlLeaseBegin opens the transaction the lease lives in. Everything the
	// lease does to the connection — the lock, application_name, the idle
	// timeout — is scoped to it, which is what makes the lease safe on a target
	// behind a transaction-pooling pooler (see Lease's own comment).
	//
	// READ COMMITTED is spelled out rather than left to the server's default
	// because it is load-bearing in the other direction: a REPEATABLE READ
	// transaction sitting idle holds back the database's xmin horizon for the
	// length of the run, and this one sits idle for the whole of it. READ ONLY
	// is the lease's own Never list, made a server rule: nothing the lease does
	// writes, and an advisory lock is not a write.
	sqlLeaseBegin = `BEGIN ISOLATION LEVEL READ COMMITTED READ ONLY`
	sqlLeaseEnd   = `ROLLBACK`

	// sqlLeaseIdentity reads the two facts the lease is built on in one round
	// trip: the server's own name for this database, which the key is derived
	// from, and the backend the transaction is running on, which Release checks
	// it is still talking to.
	sqlLeaseIdentity = `SELECT current_database(), pg_backend_pid()`

	// sqlLeaseSession names the run and disarms the one timeout that could end
	// the lease early. Both are set with set_config's is_local = true, so the
	// transaction's end puts them back: a session-level set on a pooled target
	// would be set on a server connection the pooler hands to somebody else
	// (T-0076, and the same reasoning as the source's).
	//
	// idle_in_transaction_session_timeout is disarmed because this transaction
	// is idle from the moment the lock is taken until the run is over; a target
	// that sets it — some managed services do — would terminate the lease's
	// session mid-run and leave the run holding nothing. Local to the
	// transaction, so no other session on this backend inherits the change.
	sqlLeaseSession = `SELECT set_config('application_name', $1, true),
       set_config('idle_in_transaction_session_timeout', '0', true)`

	// sqlTryAdvisoryXactLock is transaction-scoped on purpose. A session lock
	// (pg_try_advisory_lock) taken outside a transaction survives the statement
	// that took it, which through a transaction-pooling pooler means it survives
	// on a server connection the pooler is free to give to another client; a
	// transaction lock is released by the transaction's end and by the session's
	// death, and there is no third outcome.
	sqlTryAdvisoryXactLock = `SELECT pg_try_advisory_xact_lock($1)`

	// sqlLeaseBackendPID is Release's check that it is ending the transaction on
	// the backend that holds the lock.
	sqlLeaseBackendPID = `SELECT pg_backend_pid()`

	// sqlLeaseHolder names the session holding the lease. classid and objid are
	// the two halves of the bigint key as pg_locks stores them, and objsubid is
	// 1 for the single-argument form of the advisory-lock functions. The
	// database predicate is belt and braces: advisory locks already carry the
	// database oid, so a key held in another database of the same cluster is not
	// this lock at all.
	sqlLeaseHolder = `SELECT coalesce(max(a.application_name), '')
FROM pg_locks l JOIN pg_stat_activity a ON a.pid = l.pid
WHERE l.locktype = 'advisory' AND l.granted
  AND l.classid::bigint = $1 AND l.objid::bigint = $2 AND l.objsubid = 1
  AND l.database = (SELECT d.oid FROM pg_database d WHERE d.datname = current_database())`
)

// LeaseKey is the advisory-lock key for a target database name.
//
// It is derived from the name rather than from the endpoint on purpose: two
// runs can reach one database through a loopback address, a container alias and
// a pooler, and all three must collide on the same key. current_database() is
// what the gate already reads for rule 1, so the name the key is built from is
// the server's own and not the spelling in the connection string.
func LeaseKey(database string) int64 {
	sum := sha256.Sum256([]byte(leaseNamespace + database))
	return int64(binary.BigEndian.Uint64(sum[:8])) //nolint:gosec // G115: a 64-bit key, reinterpreted, not truncated
}

// LeaseHeld is the refusal a second run gets: the target is already owned.
//
// Holder is the run id named in the holder's application_name, and is empty
// when it could not be read — a target whose role may not see other sessions in
// pg_stat_activity, or a holder that had not yet named itself. An unnamed holder
// is still a refusal; it is only the message that is poorer.
type LeaseHeld struct {
	Database string
	Holder   string
}

func (e *LeaseHeld) Error() string {
	if e.Holder == "" {
		return fmt.Sprintf("pg: another lazyslice run holds the target %s", e.Database)
	}
	return fmt.Sprintf("pg: run %s holds the target %s", e.Holder, e.Database)
}

// Lease is one run's exclusive claim on a target, held by a connection of its
// own for as long as the run is entitled to write.
//
// ARCHITECTURE.md section 9's gate is a decision made at one moment and acted on
// at a later one: the gate releases its connection, core introspects, classifies
// and plans, and only then does the loader acquire a writer and start dropping
// tables. Nothing owned the target across that interval, and two lazyslice runs
// against one target could each pass the gate and then take the same database
// apart in parallel. The lease is the ownership: an advisory lock taken before
// the gate's first probe and released when the run is over, so the second run is
// refused at exit 4 before it has read anything.
//
// # The lock is transaction-scoped, and the transaction stays open
//
// The first version of this took pg_try_advisory_lock — a *session* lock — on a
// pooled connection outside any transaction, and that is the failure this repo
// has already measured once and banned for the source (T-0076, pg.go): a target
// may be reached through a transaction-pooling pooler (ARCHITECTURE.md §9), and
// in transaction mode the server connection PgBouncer assigned goes back into
// its pool the moment the statement's implicit transaction commits. A session
// lock taken that way is left on a backend lazyslice does not own: the pooler
// hands it to somebody else, pg_advisory_unlock at the end of the run is very
// likely routed to a *different* backend and returns false, and the orphan sits
// on the shared connection until server_lifetime (3600 s by default) recycles
// it. Every later lazyslice run routed onto that backend would be refused at
// exit 4 naming a holder that does not exist, with no command to clear it —
// a self-inflicted lockout, introduced by the control that was supposed to
// prevent one, and invisible because Release deliberately discards its result.
//
// So the lock is pg_try_advisory_xact_lock, taken inside a transaction this type
// opens and holds open for the life of the lease. That changes three things at
// once, all of them structural rather than conventional:
//
//   - The lock cannot outlive anything. A transaction-level advisory lock is
//     released by COMMIT, by ROLLBACK, by the backend dying and by the client
//     going away. There is no path on which it is left behind, on a pooler or
//     anywhere else, and no cleanup command an operator could ever need.
//   - The backend is ours for the duration. A pooler in transaction mode pins
//     the server connection it assigned for the length of the client's
//     transaction — measured in this package's own suite, where the source's
//     holder transaction pins one the same way — so the lock, the name and the
//     pid are all on a backend nobody else can be given meanwhile.
//   - Nothing is left on the session. application_name and the idle timeout are
//     set with set_config's is_local = true and put back by the transaction's
//     end, so the T-0076 leak has no form here either.
//
// The cost is stated in the room check in AcquireLease: an open transaction
// occupies a server connection, so a pooler with a single server connection
// cannot serve the rest of the run while the lease holds one. That is refused,
// by name, at the moment the lease is taken.
//
// What the lease does not do is stop an *application* writing to the target.
// Advisory locks coordinate clients that ask for them and nothing else; the
// control against a stranger's INSERT is the per-table lock-and-recheck in
// internal/load, and the residual is stated in THREAT_MODEL.md T2.
type Lease struct {
	conn     *pgxpool.Conn
	key      int64
	runID    string
	pid      int32
	released bool
}

// RunID is the run this lease was taken for.
func (l *Lease) RunID() string {
	if l == nil {
		return ""
	}
	return l.runID
}

// AcquireLease takes the run lease on this target.
//
// It returns a *LeaseHeld when another run holds it, which every caller turns
// into exit 4 and CodeLeaseHeld. Any other error is a target that could not be
// asked, which is a refusal too: a lease that cannot be taken is not a lease
// that is free (THREAT_MODEL.md T2's fail-closed direction).
//
// The connection is held for the life of the lease, with its transaction open,
// and never given back to the pool in the meantime. That is the whole mechanism
// — a transaction-level advisory lock lives on its transaction — and it costs
// one connection out of the target pool. That the pool has one to spare is not
// left to the default: pool_max_conns is a connection string parameter,
// `--target '...?pool_max_conns=1'` is legal, and a lease holding the only
// connection would leave the gate's next acquire waiting on the run that is
// blocking it. OpenTarget puts a floor of targetPoolFloor under MaxConns for
// exactly this (pg.go).
func (t *Target) AcquireLease(ctx context.Context, runID string) (*Lease, error) {
	var (
		currentDB string
		pid       int32
	)

	conn, err := t.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("pg: acquiring the connection that holds the target: %w", err)
	}
	kept := false
	defer func() {
		if !kept {
			endLeaseTx(ctx, conn)
			conn.Release()
		}
	}()

	// Everything below happens inside this transaction, and ends with it.
	if _, beginErr := conn.Exec(ctx, sqlLeaseBegin); beginErr != nil {
		return nil, fmt.Errorf("pg: opening the transaction that holds the target: %w", beginErr)
	}

	// The key is the server's own name for this database, so that two runs that
	// spelled the endpoint differently still collide. The pid is what Release
	// checks it is still talking to.
	if idErr := conn.QueryRow(ctx, sqlLeaseIdentity).Scan(&currentDB, &pid); idErr != nil {
		return nil, fmt.Errorf("pg: reading the target database name for the run lease: %w", idErr)
	}
	key := LeaseKey(currentDB)

	// The name goes on before the lock, not after: a second run that finds the
	// lock taken reads this to say who has it, and a holder that named itself
	// only after locking would leave that window unattributable.
	if _, nameErr := conn.Exec(ctx, sqlLeaseSession, leaseAppPrefix+runID); nameErr != nil {
		return nil, fmt.Errorf("pg: naming the run on the target connection: %w", nameErr)
	}

	var got bool
	if lockErr := conn.QueryRow(ctx, sqlTryAdvisoryXactLock, key).Scan(&got); lockErr != nil {
		return nil, fmt.Errorf("pg: taking the run lease on the target: %w", lockErr)
	}
	if !got {
		return nil, &LeaseHeld{Database: currentDB, Holder: leaseHolder(ctx, conn, key)}
	}

	if roomErr := t.leaseLeavesRoom(ctx); roomErr != nil {
		return nil, roomErr
	}

	kept = true
	return &Lease{conn: conn, key: key, runID: runID, pid: pid}, nil
}

// leaseLeavesRoom proves that the target can still serve a second connection
// while the lease holds one, and names the cause when it cannot.
//
// The lease's open transaction occupies a server connection for the whole run.
// Against a server reached directly that is the cost of one backend and nothing
// else, and OpenTarget's pool floor guarantees pgxpool has a second connection
// to give (targetPoolFloor, pg.go), so this check passes in one round trip. But a
// target reached through a pooler configured with a single server connection
// (PgBouncer's max_db_connections = 1) cannot: the pooler pins its one server
// connection to the lease's transaction, and the gate's very next statement
// waits out query_wait_timeout and comes back as an opaque 08P01 from inside the
// driver, seconds or minutes later, naming nothing an operator could act on.
//
// So the lease pays for one round trip on a second connection to find that out
// here instead, where the failure can say what it is. The wait is the same wait
// the gate would have done; what is bought is the sentence.
func (t *Target) leaseLeavesRoom(ctx context.Context) error {
	conn, err := t.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("pg: the target has no second connection to give while the run lease holds one "+
			"(a pooler in front of the target with a single server connection does this): %w", err)
	}
	defer conn.Release()

	var one int
	if pingErr := conn.QueryRow(ctx, sqlPing).Scan(&one); pingErr != nil {
		return fmt.Errorf("pg: the target would not answer on a second connection while the run lease holds one "+
			"(a pooler in front of the target with a single server connection does this): %w", pingErr)
	}
	return nil
}

// Release gives the lease up. It is idempotent, and it is deliberately quiet:
// the run is over by the time it is called, and every way this can fail ends the
// session, which ends the lock.
//
// That last clause is the reason nothing here is reported. A transaction-level
// advisory lock is released by the transaction ending or by the session ending,
// so the failure modes are: the ROLLBACK succeeds, and the lock is gone; the
// ROLLBACK fails, and the connection is closed here, and the lock is gone with
// the session; or Release is never reached at all because the process died, and
// the lock is gone with the connection. pgxpool's own Release destroys a
// connection whose transaction status is not idle, which is the fourth net under
// the same guarantee. There is no outcome in which a lock is left behind for a
// later run to trip over, which is precisely what the session-level lock this
// replaced could not say.
//
// The pid check is not ceremony: it asserts the invariant the transaction is
// supposed to give — that the statement ending this lease is reaching the
// backend that took it — and a connection that answers with any other pid is
// closed rather than handed back to the pool, because a connection whose backend
// moved underneath an open transaction is one this package does not understand.
func (l *Lease) Release(ctx context.Context) {
	if l == nil || l.released {
		return
	}
	l.released = true
	defer l.conn.Release()

	ctx = context.WithoutCancel(ctx)

	var pid int32
	if pidErr := l.conn.QueryRow(ctx, sqlLeaseBackendPID).Scan(&pid); pidErr != nil || pid != l.pid {
		discard(ctx, l.conn)
		return
	}
	endLeaseTx(ctx, l.conn)
}

// endLeaseTx ends the lease's transaction, and closes the connection if it
// cannot. Either way the advisory lock is released; the close is what stops a
// connection with an unknown transaction state going back into the pool
// (source.go's endTx discipline, through the same discard helper).
func endLeaseTx(ctx context.Context, conn *pgxpool.Conn) {
	if _, err := conn.Exec(context.WithoutCancel(ctx), sqlLeaseEnd); err != nil {
		discard(context.WithoutCancel(ctx), conn)
	}
}

// leaseHolder reads the run id out of the holding session's application_name.
//
// An empty answer is not an error: pg_stat_activity shows another role's
// application_name only to a role that may see it, and a run refused without a
// name to print is still refused. The caller says "another lazyslice run" in
// that case rather than inventing one.
func leaseHolder(ctx context.Context, conn *pgxpool.Conn, key int64) string {
	hi := int64(uint32(uint64(key) >> 32)) //nolint:gosec // G115: the documented two halves of the key pg_locks stores
	lo := int64(uint32(uint64(key)))       //nolint:gosec // G115: as above

	var name string
	if err := conn.QueryRow(ctx, sqlLeaseHolder, hi, lo).Scan(&name); err != nil {
		return ""
	}
	if !strings.HasPrefix(name, leaseAppPrefix) {
		return ""
	}
	return strings.TrimPrefix(name, leaseAppPrefix)
}
