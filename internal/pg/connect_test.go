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
// as an opaque acquire failure because pgxpool connects lazily. These two tests
// are the guard: read-only is a session statement on the allowlist, and Connect
// adds no startup parameter of its own.

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

// The other half: the setting is still there, as a statement the allowlist
// admits and nothing wider. Without the shape the first acquire would be
// refused by our own tracer rather than by the server.
func TestTheReadOnlySettingIsASessionStatementOnTheAllowlist(t *testing.T) {
	tr, err := NewTracer(SourceShapes()...)
	if err != nil {
		t.Fatalf("compiling the source shapes: %v", err)
	}
	pool, err := Connect(context.Background(), dsn.DSN(testDSN), tr)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer pool.Close()

	if pool.Config().AfterConnect == nil {
		t.Fatal("the source pool sets nothing on a new connection; default_transaction_read_only would never be on")
	}

	ctx := tr.TraceQueryStart(context.Background(), nil, pgx.TraceQueryStartData{SQL: sqlSetReadOnly})
	if ctx.Err() != nil {
		t.Errorf("the allowlist refused %q, which Connect sends on every connection", sqlSetReadOnly)
	}
	trace := tr.Trace()
	if len(trace) != 1 || trace[0].Shape != shapeReadOnly {
		t.Errorf("the read-only statement matched %+v, want shape %q", trace, shapeReadOnly)
	}

	// It is a literal template, so it is not a licence to set anything else.
	for _, sql := range []string{
		`SET default_transaction_read_only = off`,
		`SET session_replication_role = replica`,
		`SET default_transaction_read_only = on; DROP TABLE t`,
	} {
		if c := tr.TraceQueryStart(context.Background(), nil, pgx.TraceQueryStartData{SQL: sql}); c.Err() == nil {
			t.Errorf("the allowlist admitted %q", sql)
		}
	}
}
