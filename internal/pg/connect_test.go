// SPDX-License-Identifier: Apache-2.0

package pg

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/Liarea/lazyslice/internal/dsn"
)

// A pooled endpoint is a supported v1 topology (ADR-005 "Pooled endpoints",
// ARCHITECTURE.md §2 "Source.Reader", §8's --single-connection), and PgBouncer,
// Odyssey and Supavisor all refuse a *connection* that asks for a startup
// parameter outside their fixed list — client_encoding, DateStyle, TimeZone,
// standard_conforming_strings, application_name — unless it is named in
// ignore_startup_parameters. So a startup parameter of ours is not a setting
// that fails to apply: it is a source that cannot be opened at all, surfacing
// as an opaque acquire failure because pgxpool connects lazily.
//
// The other side of the same topology is that a session GUC does not fail — it
// persists, on a server connection lazyslice does not own. These two tests are
// the guard for both: Connect adds no startup parameter of its own, and it
// leaves no session state on a connection either.

// poolerStartupParams is what a pooler accepts in the startup packet.
var poolerStartupParams = map[string]bool{
	"client_encoding":             true,
	"datestyle":                   true,
	"timezone":                    true,
	"standard_conforming_strings": true,
	"application_name":            true,
	// pgx asks for these two itself, and both are on PgBouncer's default
	// track_extra_parameters/ignore list in current versions; they are not ours
	// to remove, and they are named here so that this test is about what
	// lazyslice adds.
	"extra_float_digits": true,
	"options":            true,
}

const testDSN = "postgres://someone:secret@127.0.0.1:1/db"

func TestConnectAddsNoStartupParameterAPoolerWouldRefuse(t *testing.T) {
	tr, err := NewTracer(SourceShapes()...)
	if err != nil {
		t.Fatalf("compiling the source shapes: %v", err)
	}
	pool, err := Connect(context.Background(), dsn.DSN(testDSN), tr)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer pool.Close()

	for name := range pool.Config().ConnConfig.RuntimeParams {
		if !poolerStartupParams[strings.ToLower(name)] {
			t.Errorf("the source pool asks for the startup parameter %q; a pooler refuses the connection "+
				"for anything outside its fixed list, so a session SET is the only way to set this", name)
		}
	}
}

// The other half: Connect leaves no session state behind on a connection
// either. It once set default_transaction_read_only=on in an AfterConnect exec,
// as a second layer under the READ ONLY transactions. Through a
// transaction-pooling PgBouncer that GUC is set on the shared *server*
// connection, and PgBouncer does not reset it by default, so it outlived the
// run and made other applications on the same pooler read-only for up to
// server_lifetime — measured in pooler_integration_test.go, which is where the
// end-to-end proof lives. This is the unit half: nothing is sent at connect
// time, and the read-only setting is per transaction (source.go's
// sqlBeginReadOnly, under every statement this package sends, SystemID
// included; internal/discover's dial is the caller outside it, T-0081).
//
// Restoring the AfterConnect exec fails here, and so does re-registering a
// shape wide enough to admit the SET it sent.
func TestConnectSetsNoSessionStateOnASourceConnection(t *testing.T) {
	tr, err := NewTracer(SourceShapes()...)
	if err != nil {
		t.Fatalf("compiling the source shapes: %v", err)
	}
	pool, err := Connect(context.Background(), dsn.DSN(testDSN), tr)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer pool.Close()

	if pool.Config().AfterConnect != nil {
		t.Error("the source pool runs an AfterConnect hook; anything it sets is session state on a " +
			"server connection a pooler shares with other applications after this run ends (T-0076). " +
			"The read-only setting belongs in the transaction, not the session")
	}

	// The allowlist is the other layer, and it is not a licence to set session
	// state: no SET but SET TRANSACTION SNAPSHOT is on it.
	for _, sql := range []string{
		`SET default_transaction_read_only = on`,
		`SET default_transaction_read_only = off`,
		`SET session_replication_role = replica`,
		`SET default_transaction_read_only = on; DROP TABLE t`,
	} {
		if c := tr.TraceQueryStart(context.Background(), nil, pgx.TraceQueryStartData{SQL: sql}); c.Err() == nil {
			t.Errorf("the allowlist admitted %q", sql)
		}
	}

	// And the transaction that carries READ ONLY is admitted, because it is what
	// enforces it now.
	if c := tr.TraceQueryStart(context.Background(), nil, pgx.TraceQueryStartData{SQL: sqlBeginReadOnly}); c.Err() != nil {
		t.Errorf("the allowlist refused %q, which every source statement now runs inside", sqlBeginReadOnly)
	}
}
