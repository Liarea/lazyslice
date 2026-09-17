// SPDX-License-Identifier: Apache-2.0

package pg

import (
	"strings"
	"testing"
)

// R2-06, 2026-09-15. The cluster identity of ARCHITECTURE.md §9 rule 1 is a
// property of the cluster, and the first version of it was not: it carried
// inet_server_addr() and inet_server_port(), which describe the connection.
// They are NULL over a unix socket and different again behind anything that
// re-dials, so one production cluster answered with two identities depending
// on how it had been reached, and the run wrote to the source's own server.
//
// This is the assertion at the level the defect lived at — the statement's own
// text — because a container suite reaches its server over TCP both times and
// cannot make the socket comparison the finding turned on.
func TestTheClusterIdentityReadsNothingFromTheConnection(t *testing.T) {
	forbidden := []string{
		"inet_server_addr",
		"inet_server_port",
		"inet_client_addr",
		"inet_client_port",
		"pg_backend_pid",
		"pg_stat_activity",
	}
	// T-0222, 2026-09-16: the identity is now three statements (the start
	// time's own privilege check and its guarded read, plus the fields that
	// carry no privilege check) rather than one, and every one of them must
	// meet the same rule.
	statements := map[string]string{
		"sqlCanReadClusterStartTime": sqlCanReadClusterStartTime,
		"sqlClusterIDStartTime":      sqlClusterIDStartTime,
		"sqlClusterIDRest":           sqlClusterIDRest,
	}
	for stmtName, sql := range statements {
		for _, name := range forbidden {
			if strings.Contains(strings.ToLower(sql), name) {
				t.Errorf("%s reads %s, which is a property of the connection and not of the "+
					"cluster: one cluster reached over two transports would answer with two identities "+
					"and §9 rule 1's alias arm would be defeated by the spelling again", stmtName, name)
			}
		}
	}
}

// The same rule, one level deeper, and the level the T-0190 fix round found the
// name blacklist above could not see: a value may be read from the cluster and
// still be *rendered* by the session. `pg_postmaster_start_time()::text` is a
// timestamptz cast, and a timestamptz renders in the session's TimeZone GUC —
// set per role by ALTER ROLE, per database by ALTER DATABASE, per connection
// string by options=-c timezone=, and by PGTZ. lazyslice reaches the source
// with the SELECT-only role §9 recommends and the target with a different,
// usually superuser role, against two different databases, so the two sides can
// legitimately differ in TimeZone; the identical postmaster then answered
// "2026-09-15 19:31:22.87433+00" to one and "2026-09-15 15:31:22.87433-04" to
// the other, the alias arm read two clusters, and the gate admitted the
// source's own database. The start time must be rendered in a fixed zone.
func TestTheClusterIdentityRendersTheStartTimeInAFixedZone(t *testing.T) {
	sql := strings.ToLower(sqlClusterIDStartTime)
	if !strings.Contains(sql, "pg_postmaster_start_time") {
		return
	}
	if strings.Contains(sql, "pg_postmaster_start_time()::") {
		t.Error("sqlClusterIDStartTime casts pg_postmaster_start_time() directly: a timestamptz " +
			"renders in the session's TimeZone, so one postmaster answers two sessions with two " +
			"identities")
	}
	if !strings.Contains(sql, "at time zone 'utc'") {
		t.Error("sqlClusterIDStartTime does not pin the postmaster start time to UTC: render it with " +
			"to_char(pg_postmaster_start_time() AT TIME ZONE 'UTC', ...) so every session on the " +
			"cluster reads one value")
	}
	if !strings.Contains(sql, "to_char(") {
		t.Error("sqlClusterIDStartTime does not render the start time with an explicit to_char() " +
			"format, so its text still depends on the session's DateStyle")
	}
}

// T-0222, 2026-09-16 (R2-06's R3 replay). A guard around
// pg_postmaster_start_time() inside sqlClusterIDStartTime itself would not
// work — Postgres checks EXECUTE for a function call at executor
// initialisation, before any CASE, WHERE or subquery branch around it runs
// (measured by hand against postgres:16: a role denied EXECUTE still got
// "permission denied for function pg_postmaster_start_time" out of a CASE, a
// scalar subquery and a CTE alike — TestGateRecognisesTheSourceClusterWithPostmasterStartTimeDenied
// exercises none of those three shapes and asserts no permission error, so it
// is not the evidence for this) — so the guard has to be
// sqlCanReadClusterStartTime, a separate statement with no reference to the
// guarded function at all, checked by readClusterID before
// sqlClusterIDStartTime is ever sent.
//
// This is the regression pin, and it asserts the property directly rather
// than a shape that merely correlates with it (fix round, review): rewriting
// sqlClusterIDStartTime as a single `SELECT CASE WHEN
// has_function_privilege('pg_postmaster_start_time()','EXECUTE') THEN
// to_char(...) ELSE the empty string END` — precisely the shape measured
// above not to work — used to pass this test unchanged, because nothing here
// checked that the guard and the guarded call were textually separate
// statements at all.
func TestTheClusterStartTimeIsGuardedByASeparateStatement(t *testing.T) {
	guard := strings.ToLower(sqlCanReadClusterStartTime)
	guarded := strings.ToLower(sqlClusterIDStartTime)

	if !strings.Contains(guard, "has_function_privilege") {
		t.Error("sqlCanReadClusterStartTime does not call has_function_privilege: it is the guard " +
			"and must check the privilege before the guarded statement is ever sent")
	}
	if strings.Contains(guard, "to_char(") || strings.Contains(guard, "at time zone") {
		t.Error("sqlCanReadClusterStartTime carries the guarded call's own shape (to_char(...) AT " +
			"TIME ZONE ...): the check must be a statement Postgres can compile without touching " +
			"pg_postmaster_start_time() at all, or it fails under the same privilege it exists to " +
			"detect the absence of")
	}
	if !strings.Contains(guarded, "pg_postmaster_start_time") {
		t.Error("sqlClusterIDStartTime does not call pg_postmaster_start_time(): it is the guarded " +
			"statement and must call the function the guard checks")
	}
	if strings.Contains(guarded, "has_function_privilege") {
		t.Error("sqlClusterIDStartTime references has_function_privilege: the guard must live in " +
			"its own statement, sent and answered before this one, never folded into it — a CASE, a " +
			"scalar subquery or a CTE guard around the call in one statement does not work, because " +
			"Postgres checks EXECUTE for the whole plan before any branch of it runs")
	}
	if sqlCanReadClusterStartTime == sqlClusterIDStartTime {
		t.Fatal("sqlCanReadClusterStartTime and sqlClusterIDStartTime are the same statement: the " +
			"guard and the guarded call must be sent separately")
	}
	if strings.Contains(strings.ToLower(sqlClusterIDRest), "pg_postmaster_start_time") {
		t.Error("sqlClusterIDRest references pg_postmaster_start_time(): the fields with no " +
			"privilege check must stay in a statement that cannot fail on EXECUTE, so a role denied " +
			"that one function still gets the rest of the identity")
	}
}

// T-0222 fix round, 2026-09-16 (review). sqlSystemID/sqlCanReadSystemID split
// the same way and for the same reason as the cluster start time above
// (readSystemID, source.go), but had no regression pin of their own before
// this: a future edit could fold pg_control_system() back behind a CASE or a
// subquery inside sqlCanReadSystemID, or reference has_function_privilege
// inside sqlSystemID, and every existing test would still pass.
func TestTheSystemIdentifierIsGuardedByASeparateStatement(t *testing.T) {
	guard := strings.ToLower(sqlCanReadSystemID)
	guarded := strings.ToLower(sqlSystemID)

	if !strings.Contains(guard, "has_function_privilege") {
		t.Error("sqlCanReadSystemID does not call has_function_privilege: it is the guard and must " +
			"check the privilege before the guarded statement is ever sent")
	}
	if strings.Contains(guard, "from pg_control_system") {
		t.Error("sqlCanReadSystemID calls pg_control_system() itself rather than only checking " +
			"has_function_privilege on it: the check must never reference the function it is " +
			"guarding, or it fails under the same privilege it is meant to detect the absence of")
	}
	if !strings.Contains(guarded, "pg_control_system()") {
		t.Error("sqlSystemID does not call pg_control_system(): it is the guarded statement and " +
			"must call the function the guard checks")
	}
	if strings.Contains(guarded, "has_function_privilege") {
		t.Error("sqlSystemID references has_function_privilege: the guard must live in its own " +
			"statement, sent and answered before this one, never folded into it")
	}
	if sqlCanReadSystemID == sqlSystemID {
		t.Fatal("sqlCanReadSystemID and sqlSystemID are the same statement: the guard and the " +
			"guarded call must be sent separately")
	}
}

func TestSameClusterIdentitySkipsFieldsOneSideCouldNotRead(t *testing.T) {
	const (
		// start | maintenance db oid | data directory | server version | system identifier
		superuser = "2026-09-15 01:54:11.534898|5|/var/lib/postgresql/data|16.4|7000000000000000001"
		// The SELECT-only role §9 recommends: no EXECUTE on pg_control_system,
		// and data_directory is superuser-only, so both fields are empty.
		readonly  = "2026-09-15 01:54:11.534898|5||16.4|"
		different = "2026-09-15 01:54:12.001122|5|/var/lib/postgresql/data|16.4|7000000000000000002"
		// One cluster, restarted between the source read and the gate's read:
		// the start time moved and the system identifier did not.
		restarted = "2026-09-15 09:12:03.778100|5|/var/lib/postgresql/data|16.4|7000000000000000001"
	)

	for _, c := range []struct {
		name        string
		a, b        string
		same, known bool
	}{
		{"one cluster, two roles", superuser, readonly, true, true},
		{"one cluster, one role", superuser, superuser, true, true},
		{"two clusters", superuser, different, false, true},
		// T-0241, round-4 red team (docs/reviews/2026-09-15-redteam/round4-still-leaking.json):
		// this used to be "two clusters, weaker role", asserting (false, true)
		// on the strength of the start time alone — readonly (the SELECT-only
		// role §9 recommends) can read neither data_directory nor
		// system_identifier, so start time and the two weak fields were all
		// it had. That was wrong: a standby's postmaster start time always
		// disagrees with its primary's, so the same three fields describe a
		// standby of the same cluster as `different` just as well as they
		// describe a second, unrelated one, and there is nothing left here to
		// tell the two apart. The verdict is now the fail-closed "unknown",
		// not a confident "different cluster" — see
		// TestAStandbysStartTimeDisagreementWithoutSystemIdentifierIsUnknown
		// below for the full round-4 shape, and
		// TestGateDistinguishesAGenuinelyDifferentClusterFromWeakFieldsAlone
		// (gate_integration_test.go) for what still does distinguish a
		// weaker role's two genuinely different clusters (a version or a
		// pre-PG15 oid mismatch).
		{"a weaker role sees only a start-time disagreement: unknown, not a genuinely different cluster",
			readonly, different, false, false},
		{"the system identifier decides over a restarted postmaster", superuser, restarted, true, true},
		{"the system identifier decides against agreeing weaker fields", superuser,
			"2026-09-15 01:54:11.534898|5|/var/lib/postgresql/data|16.4|7000000000000000002", false, true},
		{"nothing on one side", superuser, "", false, false},
		{"nothing on either side", "", "", false, false},
		{"no field filled on both sides", "||||", superuser, false, false},
		// T-0222, 2026-09-16 (R2-06's R3 replay): a role denied EXECUTE on
		// pg_postmaster_start_time() as well as pg_control_system() — a
		// hardened cluster that revokes monitoring functions from PUBLIC —
		// used to lose the whole cluster identity, because the pre-fix
		// sqlClusterID was one SELECT and a permission error on any one field
		// failed the row. source.go's has_function_privilege guard means only
		// the denied field goes empty; the maintenance db oid and the server
		// version, neither of which needs any privilege this role lacked,
		// still compare — but review round R2 found that "still compare" had
		// been implemented as "still decide same cluster", and the oid is
		// pinned to 5 for every PG15+ cluster while the version is shared by
		// every cluster built from one image, so two genuinely different
		// clusters from the same image agree on both. Only disagreement on
		// these two is decisive; agreement on only these two is an honest
		// unknown, the same as no common field at all.
		{"start time denied too: oid and version alone are not decisive, so this is unknown", superuser,
			"|5||16.4|", false, false},
		// Disagreement on a weak field is still decisive, even alone: it
		// takes no specificity to prove two clusters apart, only to prove
		// them the same.
		{"start time denied on both sides, two different clusters by version",
			"|5||16.4|", "|5||16.5|", false, true},
		{"start time denied on both sides, two different clusters by a pre-PG15 oid",
			"|3||16.4|", "|5||16.4|", false, true},
	} {
		// standby is false throughout this table: every case here is about
		// which role can read which field, not about a standby's data
		// directory — TestAStandbysStartTimeDisagreementWithoutSystemIdentifierIsUnknown
		// below covers standby true.
		same, known := sameClusterIdentity(c.a, c.b, false)
		if same != c.same || known != c.known {
			t.Errorf("%s: sameClusterIdentity = (%v, %v), want (%v, %v)",
				c.name, same, known, c.same, c.known)
		}
	}
}

// The fail-closed half of rule 1: an identity that could not be compared is
// "unknown", and a target carrying the source's own database name is refused
// on unknown rather than admitted.
func TestClusterUnknownIsTrueWhenNeitherIdentityCouldBeCompared(t *testing.T) {
	for _, c := range []struct {
		name                     string
		sourceSystemID, systemID string
		clusterKnown, unknown    bool
	}{
		{"both system identifiers", "1", "2", false, false},
		{"no source system identifier, cluster comparable", "", "2", true, false},
		{"no system identifier anywhere, cluster comparable", "", "", true, false},
		{"no system identifier anywhere, nothing comparable", "", "", false, true},
		{"target system identifier only, nothing comparable", "", "2", false, true},
	} {
		if got := clusterUnknown(c.sourceSystemID, c.systemID, c.clusterKnown); got != c.unknown {
			t.Errorf("%s: clusterUnknown = %v, want %v", c.name, got, c.unknown)
		}
	}
}

// T-0222, 2026-09-16 (R2-06's R3 replay). sameClusterVerdict is what fills
// Eligibility.SameCluster, and the case that matters is the last one: when
// neither identity could be compared, the answer must be true (the target
// may be on the source's own cluster) rather than falling back to an
// endpoint-spelling comparison, which is exactly what an alias, a second
// published port or a different transport defeats — the R3 replay reached one
// cluster over a TCP port and a unix socket, with both cluster identities
// denied and a target database named differently from the source's own, and
// the pre-fix fallback answered "different cluster" and dropped production
// tables. A false "same cluster" costs a warning line, or ADR-013's headless
// refusal naming --target; a false "different cluster" is a production write.
func TestSameClusterVerdict(t *testing.T) {
	for _, c := range []struct {
		name                      string
		sourceSystemID, systemID  string
		clusterSame, clusterKnown bool
		want                      bool
	}{
		{"system identifiers agree", "1", "1", false, false, true},
		{"system identifiers disagree", "1", "2", true, false, false},
		{"no system identifier, cluster identity agrees", "", "", true, true, true},
		{"no system identifier, cluster identity disagrees", "", "", false, true, false},
		{"nothing comparable at all: fail closed as possibly same cluster",
			"", "", false, false, true},
	} {
		if got := sameClusterVerdict(c.sourceSystemID, c.systemID, c.clusterSame, c.clusterKnown); got != c.want {
			t.Errorf("%s: sameClusterVerdict = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestAStandbysStartTimeDisagreementWithoutSystemIdentifierIsUnknown pins the
// round-4 red team's read-replica shape end to end (T-0241,
// docs/reviews/2026-09-15-redteam/round4-still-leaking.json, "a read
// replica"): --source a streaming standby under the SELECT-only role §9
// recommends, --target a database on that standby's own primary. A primary
// and its standby share system_identifier but never the postmaster start
// time — the standby's postmaster necessarily started later, when
// pg_basebackup finished — and the SELECT-only role cannot read
// system_identifier at all, so before this fix the one surviving decisive
// field (the start time) disagreed and the gate read "different cluster"
// with a green exit.
//
// This is a pure-function pin over sameClusterIdentity and sameClusterVerdict
// together, standing in for the primary-and-standby integration test
// THREAT_MODEL.md T2 (2026-09-17 amendment) records as a gap:
// internal/testutil has no way to stand up a streaming standby (it would need
// pg_basebackup run inside a second container, on a network the two share,
// which nothing here builds), so the two cluster identities below are
// hand-written exactly as Source.ClusterID and target.go's clusterIdentity
// would build them from a real primary/standby pair, the way
// TestSameClusterIdentitySkipsFieldsOneSideCouldNotRead and
// TestGateRecognisesTheSourceClusterWithPostmasterStartTimeDenied already
// stand in for privilege states no fixture grants a role by name.
//
// Extended for T-0255 (round-5 red team,
// docs/reviews/2026-09-15-redteam/round5-still-leaking.json, "the standby
// data_directory variant") with the reproduction's own shape: a source role
// that *can* read data_directory (GRANT pg_read_all_settings, a
// monitoring-grade read-only role — exactly what the recommended snippet's
// other reads ask a DBA to hand out), streaming from a co-hosted primary
// whose data directory necessarily differs from its own. Before this fix
// data_directory was the one field T-0241 left out of the carve-out, so it
// took over exactly the role the start time used to play.
func TestAStandbysStartTimeDisagreementWithoutSystemIdentifierIsUnknown(t *testing.T) {
	const (
		// start | maintenance db oid | data directory | server version | system identifier
		//
		// The source: a streaming standby, read under the SELECT-only role,
		// which can execute neither pg_control_system() nor read
		// data_directory (superuser-only). Started well after the primary.
		standbySource = "2026-09-17 09:41:07.220118|5||16.4|"
		// The target: a database on the standby's own primary, read by a
		// superuser role (the shape every one of the round-4 reproductions
		// uses for --target), which can read everything. The primary started
		// first, so the start time disagrees; the system identifier — the
		// one field that would decide this — the source could not fill.
		primaryTarget = "2026-09-17 08:58:19.435516|5|/var/lib/postgresql/data|16.4|7686364092739128680"

		// The round-5 variant: a source role that can read data_directory too
		// (pg_read_all_settings), streaming from the same co-hosted primary —
		// so both sides now fill data_directory, and it disagrees, the way any
		// standby's must against its own primary's directory on the same host.
		monitoringStandbySource = "2026-09-17 09:41:07.220118|5|/var/lib/postgresql/r5/replica|16.4|"
		coHostedPrimaryTarget   = "2026-09-17 08:58:19.435516|5|/var/lib/postgresql/r5/pgdata|16.4|7686364092739128680"
	)

	same, known := sameClusterIdentity(standbySource, primaryTarget, true)
	if same || known {
		t.Errorf("sameClusterIdentity(standby, primary, true) = (%v, %v), want (false, false): "+
			"a start-time disagreement with no system_identifier on the source side must not read "+
			"as a confident \"different cluster\" — it is exactly the standby/primary shape",
			same, known)
	}

	// sameClusterVerdict is what Eligibility.SameCluster actually becomes:
	// with the identity unknown and no system identifier on the source side,
	// the gate must answer "possibly the same cluster" (true) rather than
	// "different" (false) — the direction that printed no warning and wrote
	// into the standby's own primary.
	if verdict := sameClusterVerdict("", "7686364092739128680", same, known); !verdict {
		t.Error("sameClusterVerdict is false for the standby/primary shape: SameCluster must be " +
			"true so the same_cluster warning prints (and, headlessly with no --target, ADR-013's " +
			"escalation refuses) rather than staying silent the way the round-4 red team measured")
	}

	// T-0255's own shape: data_directory filled and differing on both sides,
	// with the source known to be a standby. Before this fix data_directory
	// was still clusterIDWeakField's opposite number — decisive on
	// disagreement unconditionally — so this table's start-time agreement
	// (were it to agree) or disagreement made no difference: data_directory
	// alone read "different cluster" with total confidence.
	monSame, monKnown := sameClusterIdentity(monitoringStandbySource, coHostedPrimaryTarget, true)
	if monSame || monKnown {
		t.Errorf("sameClusterIdentity(monitoring standby, co-hosted primary, true) = (%v, %v), "+
			"want (false, false): a co-hosted standby and primary disagree on data_directory by "+
			"construction, so a role that can read it (pg_read_all_settings) must not turn that "+
			"disagreement into a confident \"different cluster\" any more than the start time may",
			monSame, monKnown)
	}
	if verdict := sameClusterVerdict("", "7686364092739128680", monSame, monKnown); !verdict {
		t.Error("sameClusterVerdict is false for the monitoring-role standby/primary shape: " +
			"SameCluster must be true so the same_cluster warning prints instead of the round-5 " +
			"red team's silent exit 0 against a named production target")
	}
}

// A start-time disagreement is the one field this package no longer
// believes when the source is not a standby; every other field still is,
// including one it can prove "different" with when it is the only field
// that disagrees. Without this, clusterIDDifferenceUnreliableField could be
// widened to cover every field unconditionally — answering every comparison
// "unknown" — and every test above would still pass.
func TestADataDirectoryDisagreementStillProvesTwoClusters(t *testing.T) {
	const (
		a = "2026-09-17 09:41:07.220118|5|/var/lib/postgresql/data|16.4|"
		b = "2026-09-17 09:41:07.220118|5|/var/lib/postgresql/data-2|16.4|"
	)
	// standby is false here: an ordinary, non-standby source has no reason to
	// share a data directory with an unrelated cluster, so disagreement on it
	// must stay decisive. TestAStandbysStartTimeDisagreementWithoutSystemIdentifierIsUnknown
	// above is the standby==true counterpart, where the same disagreement is
	// unreliable.
	same, known := sameClusterIdentity(a, b, false)
	if same || !known {
		t.Errorf("sameClusterIdentity(a, b, false) = (%v, %v), want (false, true): two identities "+
			"that agree on the start time but disagree on data_directory are still two different "+
			"clusters when the source is not a standby, and that must still be decisive", same, known)
	}
}

// The standby carve-out is data_directory and nothing else: with standby true,
// a disagreement on a field that is not part of either carve-out (the server
// version) must still prove two clusters different. Without this, mutating
// clusterIDDifferenceUnreliableField's `standby && i ==
// clusterIDDataDirectoryField` to plain `standby` — every field's
// disagreement unreliable whenever the source is a standby — would leave the
// package's tests green, because nothing above pins a non-carve-out field's
// disagreement while standby is true.
func TestAStandbySourceStillProvesDifferenceOnANonCarveOutField(t *testing.T) {
	const (
		a = "2026-09-17 09:41:07.220118|5|/var/lib/postgresql/data|16.4|"
		b = "2026-09-17 09:41:07.220118|5|/var/lib/postgresql/data|14.12|"
	)
	// start time and data_directory agree; only the server version, a plain
	// clusterIDWeakField and no part of either carve-out, disagrees.
	same, known := sameClusterIdentity(a, b, true)
	if same || !known {
		t.Errorf("sameClusterIdentity(a, b, true) = (%v, %v), want (false, true): the standby "+
			"carve-out excuses data_directory's disagreement, not the server version's — a standby "+
			"source and a genuinely unrelated PG14 target must still be told apart", same, known)
	}
}
