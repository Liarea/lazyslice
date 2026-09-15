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
	for _, name := range forbidden {
		if strings.Contains(strings.ToLower(sqlClusterID), name) {
			t.Errorf("sqlClusterID reads %s, which is a property of the connection and not of the "+
				"cluster: one cluster reached over two transports would answer with two identities "+
				"and §9 rule 1's alias arm would be defeated by the spelling again", name)
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
	sql := strings.ToLower(sqlClusterID)
	if !strings.Contains(sql, "pg_postmaster_start_time") {
		return
	}
	if strings.Contains(sql, "pg_postmaster_start_time()::") {
		t.Error("sqlClusterID casts pg_postmaster_start_time() directly: a timestamptz renders in " +
			"the session's TimeZone, so one postmaster answers two sessions with two identities")
	}
	if !strings.Contains(sql, "at time zone 'utc'") {
		t.Error("sqlClusterID does not pin the postmaster start time to UTC: render it with " +
			"to_char(pg_postmaster_start_time() AT TIME ZONE 'UTC', ...) so every session on the " +
			"cluster reads one value")
	}
	if !strings.Contains(sql, "to_char(") {
		t.Error("sqlClusterID does not render the start time with an explicit to_char() format, so " +
			"its text still depends on the session's DateStyle")
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
		{"two clusters, weaker role", readonly, different, false, true},
		{"the system identifier decides over a restarted postmaster", superuser, restarted, true, true},
		{"the system identifier decides against agreeing weaker fields", superuser,
			"2026-09-15 01:54:11.534898|5|/var/lib/postgresql/data|16.4|7000000000000000002", false, true},
		{"nothing on one side", superuser, "", false, false},
		{"nothing on either side", "", "", false, false},
		{"no field filled on both sides", "||||", superuser, false, false},
	} {
		same, known := sameClusterIdentity(c.a, c.b)
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
