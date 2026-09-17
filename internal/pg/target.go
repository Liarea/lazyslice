// SPDX-License-Identifier: Apache-2.0

package pg

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Liarea/lazyslice/internal/dsn"
	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// The gate's refusal codes (ARCHITECTURE.md §9, THREAT_MODEL.md T2). Every one
// of them is a row the renderer looks up, and therefore a row in
// docs/ERRORS.md; internal/event/catalogue.yml carries three of them today and
// owes the rest (see internal/pg/CLAUDE.md, "Decisions made during
// implementation").
const (
	// CodeUnreachable is the precondition, not a rule: a candidate that does not
	// answer cannot be judged, and Verdict stays NotProbed.
	CodeUnreachable event.Code = "target.refused.unreachable"
	// CodeSameDatabase is rule 1: the target is the source.
	CodeSameDatabase event.Code = "target.refused.same_database"
	// CodeRemote is rule 2: the target is not local and no flag names its host.
	CodeRemote event.Code = "target.refused.remote"
	// CodeNoCreate is rule 3.
	CodeNoCreate event.Code = "target.refused.no_create"
	// CodeTableCap is rule 5 above the cap.
	CodeTableCap event.Code = "target.refused.table_cap"
	// CodeNotEmpty is rule 5: a user table that is not empty, has row-level
	// security, or could not be probed.
	CodeNotEmpty event.Code = "target.refused.not_empty"
	// CodeProbeFailed is a probe that did not run to completion. It is a
	// refusal, never a skip (THREAT_MODEL.md T2).
	CodeProbeFailed event.Code = "target.refused.probe_failed"
)

// TableCap is the number of user tables the gate will probe. Above it the
// target is refused outright with the count printed; it is never sampled and
// never passed, because Verdict has no state in which "not probed" reads as
// eligible (ARCHITECTURE.md §9 rule 5).
const TableCap = 2000

// RowsNotCounted is the value Eligibility.RowCounts carries for a table the gate
// found to be not empty. The gate proves emptiness with SELECT EXISTS and never
// counts rows in a database it is refusing to touch, so the map is the set of
// offending tables and the number is a marker, not a count.
const RowsNotCounted int64 = -1

// bookkeepingSchema is the one schema the exemption below applies in. It is the
// schema to_regclass resolves the marker in on the default search_path, and the
// schema a framework migrates.
const bookkeepingSchema = "public"

// bookkeeping are the migration-bookkeeping tables the emptiness rule exempts
// (ADR-005 "Target"). A compose database that has had its migrations run is the
// ordinary case, and rows in these tables are the migration state, not data;
// lazyslice copies them whole from the source so that state matches the schema
// it recreates (ARCHITECTURE.md §11.1 item 7).
//
// lazyslice_meta is exempt for the same reason: it is bookkeeping we wrote. An
// unbound marker therefore falls through to the emptiness check and is judged on
// the rest of the database, which is what ARCHITECTURE.md §9 rule 4 says
// happens; without the exemption the fall-through could never reach a verdict
// other than "not empty" and rule 4's last sentence would be dead text.
//
// The names are matched in bookkeepingSchema only. Emptiness is the one control
// between the tool and the catastrophic write, and this exemption is the only
// thing in it that widens what counts as empty; a reporting.schema_migrations or
// an analytics.django_migrations is somebody's data wearing a familiar name, so
// the exemption is spelled as narrowly as the case it exists for.
var bookkeeping = map[string]bool{
	"schema_migrations":          true,
	"_prisma_migrations":         true,
	"alembic_version":            true,
	"__diesel_schema_migrations": true,
	"flyway_schema_history":      true,
	"goose_db_version":           true,
	"atlas_schema_revisions":     true,
	"knex_migrations":            true,
	"knex_migrations_lock":       true,
	"django_migrations":          true,
	strings.ToLower(MarkerTable): true,
}

// CatalogFingerprinter recomputes Schema.Fingerprint (ADR-009) over the catalog
// reachable through r. The gate needs it for rule 4: a marker is bound only when
// the fingerprint it recorded still matches the target's current catalog.
//
// It is injected rather than implemented here because the fingerprint is sha256
// over the DDL internal/load/ddl generates for the schema internal/introspect
// reads, and this package can import neither: internal/load imports internal/pg,
// so the edge back would be a cycle. load.GateFingerprint is the one caller, and
// it is the whole wiring core does for §11.2's binding. A Target with no
// fingerprinter can never find a marker bound, which is the fail-closed
// direction: the gate falls through to the emptiness check and a populated
// target is still refused.
//
// r is in a transaction this package opened and this package ends
// (catalogFingerprint below). The fingerprinter must not BEGIN, COMMIT or
// ROLLBACK on it; a SAVEPOINT, which is what introspect's sampler takes to
// survive a table it cannot read, is exactly what the transaction is there for.
type CatalogFingerprinter func(ctx context.Context, r pipeline.Reader) (string, error)

// TargetOption configures a Target.
type TargetOption func(*Target)

// WithCatalogFingerprint supplies the function that recomputes the target's
// schema fingerprint for the marker binding (ARCHITECTURE.md §11.2).
func WithCatalogFingerprint(f CatalogFingerprinter) TargetOption {
	return func(t *Target) { t.fingerprint = f }
}

// WithLocal states that the candidate is local for a reason the connection
// string does not show — a container whose compose working_dir is the cwd or an
// ancestor (ARCHITECTURE.md §9 rule 2). Without it, locality is decided from the
// endpoint alone: a Unix socket or a loopback address.
func WithLocal(local bool) TargetOption {
	return func(t *Target) { t.local = local }
}

// Target is the write side, and the gate in front of it.
type Target struct {
	pool        *pgxpool.Pool
	ref         dsn.Ref
	fingerprint CatalogFingerprinter
	local       bool
	// types is the source's user-defined types, registered on every target
	// connection once the DDL has created them (types.go, ARCHITECTURE.md
	// §11.1). It is empty until the loader calls RegisterTypes.
	types *typeRegistry
	// sourceCluster is the source's Source.ClusterID, set by internal/core
	// before Gate runs. It is rule 1's second disjunct for a role that cannot
	// read system_identifier — which is the role ARCHITECTURE.md §9 itself
	// recommends (the 2026-09-15 red team's identity-rule-1 finding).
	sourceCluster string
}

// SetSourceCluster records the source's cluster identity for rule 1. It is
// called by internal/core between OpenTarget and Gate, because the source read
// it makes needs a connection the target does not have.
func (t *Target) SetSourceCluster(id string) { t.sourceCluster = id }

var _ pipeline.Target = (*Target)(nil)

// OpenTarget opens the write side. No tracer is registered on this pool: the
// target is the side lazyslice writes to, and what defends it is the gate.
//
// This pool is the one pool in lazyslice with an AfterConnect hook, and the hook
// is the type registration ARCHITECTURE.md §11.1 and ADR-005 specify. It carries
// nothing until the loader calls RegisterTypes, because at this point the target
// has none of the source's types: lazyslice creates them itself, after the gate
// and after the drop.
func OpenTarget(ctx context.Context, d dsn.DSN, opts ...TargetOption) (*Target, error) {
	_, r, err := dsn.Parse(string(d))
	if err != nil {
		return nil, err
	}
	types := &typeRegistry{}
	pool, err := Connect(ctx, d, nil,
		withAfterConnect(types.afterConnect),
		// The run lease holds one of these connections for the whole run, so a
		// target pool of one would hand it the only connection and leave the
		// gate's own acquire waiting on a run it is itself blocking
		// (targetPoolFloor).
		withMinMaxConns(targetPoolFloor),
	)
	if err != nil {
		return nil, err
	}
	t := &Target{pool: pool, ref: r, local: r.Loopback(), types: types}
	for _, o := range opts {
		o(t)
	}
	return t, nil
}

// Ref is the redacted identity of the target.
func (t *Target) Ref() dsn.Ref { return t.ref }

// Close releases the pool.
func (t *Target) Close() { t.pool.Close() }

const (
	sqlPing      = `SELECT 1`
	sqlIdentity  = `SELECT current_database()`
	sqlCanCreate = `SELECT CASE WHEN EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = 'public')
            THEN has_schema_privilege(current_user, 'public', 'CREATE') ELSE false END`
	sqlUserTables = `SELECT n.nspname, c.relname, c.relrowsecurity OR c.relforcerowsecurity
FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE c.relkind IN ('r', 'p') AND NOT c.relispartition
  AND n.nspname NOT IN ('pg_catalog', 'information_schema')
  AND n.nspname NOT LIKE 'pg\_toast%' AND n.nspname NOT LIKE 'pg\_temp%'
  AND NOT EXISTS (SELECT 1 FROM pg_depend d
                  WHERE d.classid = 'pg_class'::regclass AND d.objid = c.oid AND d.deptype = 'e')
ORDER BY n.nspname, c.relname`
)

// Gate implements ARCHITECTURE.md §9 "Target" in the order it states:
// reachability as a precondition, then 1 identity, 2 locality, 3 CREATE,
// 4 marker, 5 emptiness.
//
// It returns Verdict == Eligible only when every probe ran to completion. Any
// error, timeout or unprobed table is Refused; the one thing that is never
// Refused-by-rule is an unreachable candidate, which stays NotProbed because
// nothing about it was judged at all. NotProbed is not Eligible either, so no
// path through this function ends in a write that a rule did not authorise.
func (t *Target) Gate(ctx context.Context, source dsn.Ref, sourceSystemID, allowRemoteHost string) (pipeline.Eligibility, error) {
	e := pipeline.Eligibility{Verdict: pipeline.NotProbed, Local: t.local}

	conn, err := t.pool.Acquire(ctx)
	if err != nil {
		e.Reason = CodeUnreachable
		return e, fmt.Errorf("pg: gate: connecting to the target: %w", err)
	}
	defer conn.Release()

	var one int
	if pingErr := conn.QueryRow(ctx, sqlPing).Scan(&one); pingErr != nil {
		e.Reason = CodeUnreachable
		return e, fmt.Errorf("pg: gate: the target did not answer: %w", pingErr)
	}

	// From here every outcome is a judgement, so nothing may leave NotProbed.
	e.Verdict = pipeline.Refused

	// Rule 1: identity. A disjunction over sameness, not a conjunction over
	// difference: refuse on any match.
	var currentDB string
	if dbErr := conn.QueryRow(ctx, sqlIdentity).Scan(&currentDB); dbErr != nil {
		e.Reason = CodeProbeFailed
		return e, fmt.Errorf("pg: gate: reading the target database name: %w", dbErr)
	}
	targetRef := t.ref
	targetRef.Database = currentDB

	systemID, err := systemIdentifier(ctx, conn)
	if err != nil {
		e.Reason = CodeProbeFailed
		return e, err
	}
	clusterID, err := clusterIdentity(ctx, conn, systemID)
	if err != nil {
		e.Reason = CodeProbeFailed
		return e, err
	}

	clusterSame, clusterKnown := sameClusterIdentity(t.sourceCluster, clusterID)
	e.SameCluster = sameClusterVerdict(sourceSystemID, systemID, clusterSame, clusterKnown)

	sameEndpoint, err := targetRef.SameEndpoint(source)
	if err != nil {
		// A reference that cannot be compared is never "not the source".
		e.Reason = CodeProbeFailed
		return e, fmt.Errorf("pg: gate: comparing the target with the source: %w", err)
	}
	// Rule 1's second disjunct: the same catalog on the same cluster, whatever
	// the endpoint is spelled as.
	//
	// Until the 2026-09-15 red team it rested on system_identifier alone, and
	// EXECUTE on pg_control_system is not granted to PUBLIC — so for the
	// SELECT-only source role ARCHITECTURE.md §9 itself recommends, this arm
	// was unreachable and the endpoint comparison stood alone. Two published
	// ports onto one container, or a second host spelling, defeated it: the
	// run reported "dropping public.customers in the target" against the
	// production database and did it.
	//
	// clusterIdentity is the same question asked of catalog values every role
	// can read (source.go's readClusterID), so the arm now works under the
	// recommended role. Every one of those values is a property of the
	// *cluster* and not of the connection (amended 2026-09-15, R2-06): the
	// first version of this arm carried inet_server_addr() and
	// inet_server_port(), so one cluster reached over two transports — a
	// second published port, a proxy, the unix socket — answered with two
	// identities and the arm was defeated by the spelling again. And when
	// *neither* identity can be compared, a
	// target whose database has the source's own name is refused rather than
	// admitted: the endpoint spelling is exactly what an alias changes, so
	// "the endpoints differ" is not evidence of anything here. The emptiness
	// rule bounds what that used to cost — a populated production database is
	// refused as not_empty — but an empty one with the source's name on the
	// same server is the migration scenario, and the drop was real.
	sameCatalog := currentDB == source.Database &&
		(clusterUnknown(sourceSystemID, systemID, clusterKnown) ||
			(sourceSystemID != "" && systemID != "" && systemID == sourceSystemID) ||
			(clusterKnown && clusterSame))
	if sameEndpoint || sameCatalog {
		e.Reason = CodeSameDatabase
		return e, nil
	}

	// Rule 2: locality. The flag decides whether the rule refuses; it does not
	// make the target local. Eligibility.Local stays what it was set to from
	// t.local above, because the decision header is the operator's only visible
	// signal that the write is leaving this machine and a flag-admitted remote
	// target must not print as local (THREAT_MODEL.md T2).
	if !t.local && !hostNamed(allowRemoteHost, targetRef) {
		e.Reason = CodeRemote
		return e, nil
	}

	// Rule 3: CREATE on schema public.
	var canCreate bool
	if createErr := conn.QueryRow(ctx, sqlCanCreate).Scan(&canCreate); createErr != nil {
		e.Reason = CodeProbeFailed
		return e, fmt.Errorf("pg: gate: reading schema privileges on the target: %w", createErr)
	}
	if !canCreate {
		e.Reason = CodeNoCreate
		return e, nil
	}

	// Rule 4: the marker, bound to this source and this catalog.
	marker, found, err := t.latestMarker(ctx, conn)
	if err != nil {
		e.Reason = CodeProbeFailed
		return e, err
	}
	e.Marked = found
	if found {
		e.PrevKeyFP = marker.SecretFingerprint
		e.PrevClassFP = marker.ClassificationFingerprint
		// The identity of the row this verdict is about to rest on. The loader
		// re-reads it under its own lock before each drop, because by then the
		// verdict is a remembered fact (§11.2, amended 2026-09-14).
		e.MarkerRunID = marker.RunID
		e.MarkerStatus = marker.Status
		bound, err := t.markerBound(ctx, conn, marker, source, sourceSystemID)
		if err != nil {
			e.Reason = CodeProbeFailed
			return e, err
		}
		e.MarkerBound = bound
		if bound {
			e.Verdict = pipeline.Eligible
			return e, nil
		}
	}

	// Rule 5: emptiness.
	return t.checkEmpty(ctx, conn, e)
}

// hostNamed reports whether --allow-remote-target named this target's host. The
// flag names a host exactly; it is compared after normalisation so that the
// spelling in the flag and the spelling in the connection string do not have to
// match character for character, and it is never resolved.
func hostNamed(allowRemoteHost string, target dsn.Ref) bool {
	if allowRemoteHost == "" {
		return false
	}
	allowed := dsn.Ref{Host: allowRemoteHost, Port: target.Port, Database: target.Database}
	same, err := allowed.SameCluster(target)
	return err == nil && same
}

// systemIdentifier reads the target's system_identifier through
// source.go's readSystemID (T-0222, R2-06's R3 replay): the privilege check
// and the call are two statements, never one guarded statement, because
// Postgres checks EXECUTE for the whole plan before any guard around the call
// runs (sqlCanReadSystemID's comment). A role that cannot execute
// pg_control_system answers "": that is not an identifier that differs, and
// the endpoint comparison stands on its own with the run saying so in the
// header.
func systemIdentifier(ctx context.Context, conn *pgxpool.Conn) (string, error) {
	id, err := readSystemID(ctx, conn)
	if err != nil {
		return "", fmt.Errorf("pg: gate: reading the target system identifier: %w", err)
	}
	return id, nil
}

// sameClusterVerdict is rule 1's identity comparison, and fills
// Eligibility.SameCluster. system_identifier decides alone when both sides
// could read it; failing that, the ordinary-role cluster identity decides
// when sameClusterIdentity could compare at least one field on both sides
// (clusterKnown); and when neither is comparable at all, the target is
// answered as *possibly* the source's own cluster rather than as different.
//
// That last case used to fall back to targetRef.SameCluster(source), a
// comparison of the two normalised endpoint spellings — which answers
// "different" for one cluster reached over two transports (a published TCP
// port and the unix socket, a pooler, an SSH tunnel), because that is
// precisely what an endpoint spelling cannot see through. Alias-defeating is
// what rule 1's identity check exists for in the first place, so falling
// back to the thing it was built to defeat, exactly when the identity check
// has nothing to go on, answered the R3 replay of R2-06 with a clean "not the
// same cluster" and a production write (T-0222, 2026-09-16:
// docs/reviews/2026-09-15-redteam/round3-still-leaking.json — a target
// database named differently from the source's, so rule 1's sameCatalog arm
// below does not fire either, reached over a transport the endpoint
// comparison reads as a different server).
//
// A false "possibly same cluster" costs one extra warning line, or — for a
// headless run whose target the ladder chose rather than the operator named —
// ADR-013's refusal naming --target. A false "different cluster" is a
// production write. When nothing can be compared, the answer is true.
func sameClusterVerdict(sourceSystemID, systemID string, clusterSame, clusterKnown bool) bool {
	switch {
	case sourceSystemID != "" && systemID != "":
		return sourceSystemID == systemID
	case clusterKnown:
		return clusterSame
	default:
		return true
	}
}

// clusterUnknown reports that neither identity could be compared: not the
// system identifier, because a role that may not execute pg_control_system
// reads "" for it, and not the ordinary-role cluster identity either. It is the
// fail-closed half of rule 1 above.
//
// clusterKnown is sameClusterIdentity's second answer: two identities can be
// compared when at least one field is filled on both sides (amended
// 2026-09-15, R2-06). Before that amendment this took the two identities and
// asked only whether each was non-empty, which read "the two strings differ" as
// positive evidence of two different clusters — and the strings differed
// whenever the transport differed, which is the whole of the finding.
func clusterUnknown(sourceSystemID, systemID string, clusterKnown bool) bool {
	if sourceSystemID != "" && systemID != "" {
		return false
	}
	return !clusterKnown
}

// clusterIdentity is source.go's readClusterID against the target, with the
// target's system identifier appended as its last field (withSystemID, which
// is what the source side composes too). Like systemIdentifier it answers ""
// for a read that failed for any reason other than the context ending: the
// gate's own rule above decides what an unknown identity means, and it means
// "refuse a target with the source's database name".
func clusterIdentity(ctx context.Context, conn *pgxpool.Conn, systemID string) (string, error) {
	// false: this connection is the gate's autocommit connection, never inside
	// an open transaction, so a failed sqlClusterIDStartTime never aborts a
	// transaction sqlClusterIDRest depends on — no SAVEPOINT is needed here.
	id, err := readClusterID(ctx, conn, false)
	if err != nil {
		return "", fmt.Errorf("pg: gate: reading the target cluster identity: %w", err)
	}
	return withSystemID(id, systemID), nil
}

func (t *Target) markerBound(ctx context.Context, conn *pgxpool.Conn, m MarkerRow, source dsn.Ref, sourceSystemID string) (bool, error) {
	if m.SchemaVersion > MarkerSchemaVersion {
		// A marker written by a newer lazyslice. It authorises nothing, and the
		// gate falls through to emptiness (ARCHITECTURE.md §11.2).
		return false, nil
	}
	if m.SourceFingerprint != source.Fingerprint() {
		return false, nil
	}
	if m.SourceSystemID != "" && sourceSystemID != "" && m.SourceSystemID != sourceSystemID {
		return false, nil
	}
	if t.fingerprint == nil {
		// Nothing can recompute the catalog fingerprint, so the binding cannot
		// be confirmed and the marker authorises nothing.
		return false, nil
	}
	current, err := t.catalogFingerprint(ctx, conn)
	if err != nil {
		return false, fmt.Errorf("pg: gate: recomputing the target schema fingerprint: %w", err)
	}
	return current != "" && current == m.SchemaFingerprint, nil
}

// catalogFingerprint runs the injected fingerprinter over the target's catalog
// inside a transaction this package opens and ends itself.
//
// The transaction is not a nicety; it is what CatalogFingerprinter's contract
// promises. A fingerprinter reads the whole catalog — dozens of statements that
// must see one version of it — and a SAVEPOINT is legal for whichever
// fingerprinter needs one, because outside a transaction block a SAVEPOINT is
// 25P01. That was learned the hard way: the fingerprinter used to take a full
// introspection, whose sampler wraps itself in a SAVEPOINT so that a table the
// role cannot read does not end the run, and on a pooled connection in
// autocommit it failed every time — §11.2's binding could never be confirmed
// and a target lazyslice itself wrote was refused with exit 4. Today's
// production fingerprinter (load.GateFingerprint, which asks introspect for the
// schema-only read) takes no savepoint of its own, so nothing in the tree
// currently exercises that half of the promise and
// TestGateRunsTheFingerprinterInsideATransaction is what keeps it true. It is
// opened here rather than by the caller because a BEGIN issued from the other
// side of the injection would end a transaction it did not start, and because
// this package owns the connection.
//
// READ ONLY, because recomputing a fingerprint reads: it makes a fingerprinter
// that tried to write fail at the server rather than at review. REPEATABLE READ
// for the one-version-of-the-catalog property.
//
// ROLLBACK ends it either way — there is nothing to commit. A rollback that
// fails is both reported and acted on. Reported, because every caller of this
// function turns an error into CodeProbeFailed and a refused run, which is the
// fail-closed direction for a binding that could not be confirmed: rule 4 is the
// last rule that runs on this connection before rule 5, and a run that returns
// here does not reach rule 5 at all. Acted on, because a failed rollback leaves
// the connection in an unknown transaction state and the pool would hand it to
// whoever acquires next — so it is closed with the same discipline as
// source.go's endTx, and pgxpool discards a closed connection when the gate's
// own deferred Release returns it.
func (t *Target) catalogFingerprint(ctx context.Context, conn *pgxpool.Conn) (string, error) {
	if _, err := conn.Exec(ctx, sqlBeginReadOnly); err != nil {
		return "", fmt.Errorf("pg: opening the transaction to read the target catalog: %w", err)
	}
	fp, fpErr := t.fingerprint(ctx, &reader{conn: conn, own: false})
	// ROLLBACK succeeds on an aborted transaction, so it runs whether or not the
	// fingerprinter failed, and the fingerprinter's own error is the one
	// returned when both go wrong.
	if _, err := conn.Exec(ctx, sqlRollback); err != nil {
		discard(context.WithoutCancel(ctx), conn)
		if fpErr == nil {
			return "", fmt.Errorf("pg: ending the transaction that read the target catalog: %w", err)
		}
	}
	if fpErr != nil {
		return "", fpErr
	}
	return fp, nil
}

// exemptFromEmptiness reports whether the emptiness rule skips this table. It
// is a qualified match: the same name in another schema is data, not
// bookkeeping (see bookkeeping).
func exemptFromEmptiness(t ref.TableRef) bool {
	return t.Schema == bookkeepingSchema && bookkeeping[strings.ToLower(t.Name)]
}

func (t *Target) checkEmpty(ctx context.Context, conn *pgxpool.Conn, e pipeline.Eligibility) (pipeline.Eligibility, error) {
	type userTable struct {
		ref ref.TableRef
		rls bool
	}

	rows, err := conn.Query(ctx, sqlUserTables)
	if err != nil {
		e.Reason = CodeProbeFailed
		return e, fmt.Errorf("pg: gate: listing the target's user tables: %w", err)
	}
	var tables []userTable
	for rows.Next() {
		var u userTable
		if err := rows.Scan(&u.ref.Schema, &u.ref.Name, &u.rls); err != nil {
			rows.Close()
			e.Reason = CodeProbeFailed
			return e, fmt.Errorf("pg: gate: listing the target's user tables: %w", err)
		}
		if exemptFromEmptiness(u.ref) {
			continue
		}
		tables = append(tables, u)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		e.Reason = CodeProbeFailed
		return e, fmt.Errorf("pg: gate: listing the target's user tables: %w", err)
	}

	e.TableCount = len(tables)
	if len(tables) > TableCap {
		e.Reason = CodeTableCap
		return e, nil
	}

	e.RowCounts = map[ref.TableRef]int64{}
	for _, u := range tables {
		if u.rls {
			// Under FORCE ROW LEVEL SECURITY with no matching policy, EXISTS
			// returns false over millions of rows the session cannot see, while
			// DROP TABLE still succeeds. Row-level security is therefore "not
			// empty" by rule, never by probe.
			e.RowCounts[u.ref] = RowsNotCounted
			continue
		}
		var occupied bool
		q := `SELECT EXISTS (SELECT 1 FROM ` + pgx.Identifier{u.ref.Schema, u.ref.Name}.Sanitize() + `)`
		if err := conn.QueryRow(ctx, q).Scan(&occupied); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				e.Reason = CodeProbeFailed
				return e, fmt.Errorf("pg: gate: probing %s.%s: %w", u.ref.Schema, u.ref.Name, err)
			}
			// A probe that errors for any reason, 42501 included, counts as not
			// empty. It is named in the refusal like any other.
			e.RowCounts[u.ref] = RowsNotCounted
			continue
		}
		if occupied {
			e.RowCounts[u.ref] = RowsNotCounted
		}
	}

	if len(e.RowCounts) > 0 {
		e.Reason = CodeNotEmpty
		return e, nil
	}
	e.RowCounts = nil
	e.Verdict = pipeline.Eligible
	return e, nil
}

// Writer opens the write side. Nothing calls it before Gate has returned
// Eligible; that ordering is core.Run's, and this package does not second-guess
// it, because a Writer that re-ran the gate would run it twice on every load.
func (t *Target) Writer(_ context.Context) (pipeline.Writer, error) {
	return &writer{pool: t.pool, types: t.types}, nil
}
