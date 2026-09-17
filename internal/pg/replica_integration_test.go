// SPDX-License-Identifier: Apache-2.0

//go:build integration

package pg

import (
	"context"
	"net/url"
	"testing"

	"github.com/Liarea/lazyslice/internal/dsn"
)

// T-0241 (round-4 red team, docs/reviews/2026-09-15-redteam/round4-still-leaking.json,
// "a read replica"): Source.Replica is the positive check that reads
// pg_is_in_recovery() (executable by PUBLIC) and, on a standby,
// pg_stat_wal_receiver's sender_host/sender_port.
//
// A primary-and-standby pair is the fixture this needs to exercise the
// Standby == true half, and internal/testutil cannot stand one up: doing so
// needs pg_basebackup run inside a second container, on a Docker network the
// two share, which is more than Postgres(ctx, t, image), PgBouncer(...) and
// SecondEndpoint(...) build (internal/testutil/CLAUDE.md lists all three).
// That gap is recorded in THREAT_MODEL.md T2's 2026-09-17 amendment, and
// internal/pg's cluster_test.go pins the identity-comparison half of this
// fix as a pure function instead
// (TestAStandbysStartTimeDisagreementWithoutSystemIdentifierIsUnknown).
//
// **The role matters as much as the fixture (T-0241 fix round, review).**
// The property both tests below exist to prove is that this design's whole
// premise — pg_is_in_recovery() needs no grant, and pg_stat_wal_receiver
// nulls sender_host/sender_port for a role that lacks superuser or
// pg_monitor rather than refusing the query, which is why readReplicaStatus
// scans them as sql.NullString — actually holds against a real backend.
// testutil.Postgres's connection is POSTGRES_USER, which the postgres
// docker image makes the container's own initdb superuser: a superuser can
// read anything, so running these statements through it could not tell "the
// statement is fine and the columns come back null for an unprivileged
// role" apart from "this role can do anything and the restricted-role
// question never actually arose" — which was this file's original defect.
// restrictedSource below opens the source as a fresh NOSUPERUSER role
// instead, so the tests run under the shape they claim to.
func restrictedSource(ctx context.Context, t *testing.T, f gateFixture, role string) *Source {
	t.Helper()
	f.exec(ctx, t, f.sourceURL,
		`CREATE ROLE `+role+` LOGIN PASSWORD 'lazyslice' NOSUPERUSER`,
		`GRANT CONNECT ON DATABASE `+f.sourceRef.Database+` TO `+role,
	)

	restricted := *f.base
	restricted.User = url.UserPassword(role, "lazyslice")

	src, err := OpenSource(ctx, dsn.DSN(restricted.String()))
	if err != nil {
		t.Fatalf("opening the source as %s: %v", role, err)
	}
	return src
}

// What this test proves against a real server: an ordinary, non-standby
// Postgres answers pg_is_in_recovery() false, under a genuinely restricted
// NOSUPERUSER role rather than the container's own superuser, with no error
// and no second statement sent — proving the first statement is well-formed
// and privilege-free against a real backend, not only against the mock
// strings cluster_test.go compares. It does not and cannot reach the
// wal-receiver query at all ("no second statement sent" is exactly why —
// see TestWalReceiverSenderColumnsAreQueryableUnderTheRecommendedRole below
// for the statement this one never runs).
func TestReplicaOnAnOrdinarySourceIsNotAStandby(t *testing.T) {
	ctx := context.Background()
	f := newGateFixture(ctx, t)
	src := restrictedSource(ctx, t, f, "replica_role_ordinary")
	defer src.Close()

	status, err := src.Replica(ctx)
	if err != nil {
		t.Fatalf("Replica: %v", err)
	}
	if status.Standby {
		t.Error("an ordinary, freshly created Postgres container answered pg_is_in_recovery() true")
	}
	if status.SenderHost != "" || status.SenderPort != "" {
		t.Errorf("a non-standby source carries a sender host/port: %+v", status)
	}

	if violation := src.Violation(); violation != nil {
		t.Errorf("the allowlist refused a statement Replica sent: %v", violation)
	}
}

// The other half of the same fix (T-0241 fix round, review): the
// wal-receiver sender query itself, run directly under the restricted role
// rather than through Replica, which never reaches it on a non-standby —
// this is the "second case that does not short-circuit" the review asked
// for. pg_stat_wal_receiver's restriction on sender_host/sender_port is a
// column-level null inside the view's own defining function, not a
// table-level GRANT, so the statement must run to completion under this
// role and return no permission error, whether or not it returns any rows.
//
// **What this does and does not prove, named rather than left implicit.**
// The view carries a row only while a real WAL receiver process is running,
// which needs the primary-and-standby fixture the doc comment above says
// internal/testutil does not build. Against this ordinary, non-standby
// source the query legitimately returns zero rows for every role, superuser
// included, so this test cannot show a genuine sender_host value coming
// back NULL rather than withheld with an error — that half stays the
// fixture gap THREAT_MODEL.md T2's 2026-09-17 amendment already records.
// What it does show is the necessary condition readReplicaStatus's design
// rests on: this exact statement, under this exact non-superuser role,
// completes with no permission error at all — sql.NullString has something
// to scan into rather than an error the code never reaches, if a real
// standby ever does put a row in the view.
func TestWalReceiverSenderColumnsAreQueryableUnderTheRecommendedRole(t *testing.T) {
	ctx := context.Background()
	f := newGateFixture(ctx, t)
	src := restrictedSource(ctx, t, f, "replica_role_wal_receiver")
	defer src.Close()

	if err := src.tr.Register(Shape{Name: "source.replica.wal_receiver_sender_probe", SQL: sqlWalReceiverSender}); err != nil {
		t.Fatalf("registering the probe shape: %v", err)
	}

	conn, err := src.pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquiring a source connection: %v", err)
	}
	defer conn.Release()

	if _, err = conn.Exec(ctx, sqlBeginReadOnly); err != nil {
		t.Fatalf("opening the transaction: %v", err)
	}
	defer conn.Exec(context.WithoutCancel(ctx), sqlRollback)

	rows, err := conn.Query(ctx, sqlWalReceiverSender)
	if err != nil {
		t.Fatalf("querying pg_stat_wal_receiver's sender columns as a NOSUPERUSER role: %v — "+
			"this must be a clean empty result, never a permission error, because the restriction "+
			"is a column-level null inside the view and not a table-level GRANT", err)
	}
	defer rows.Close()

	n := 0
	for rows.Next() {
		n++
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterating pg_stat_wal_receiver's sender columns as a NOSUPERUSER role: %v", err)
	}
	if n != 0 {
		t.Fatalf("got %d rows from pg_stat_wal_receiver against a non-standby source, want 0", n)
	}

	if violation := src.Violation(); violation != nil {
		t.Errorf("the allowlist refused the wal-receiver probe: %v", violation)
	}
}
