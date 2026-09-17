// SPDX-License-Identifier: Apache-2.0

//go:build integration

package pg

import (
	"context"
	"net/url"
	"sort"
	"testing"

	"github.com/Liarea/lazyslice/internal/dsn"
	"github.com/Liarea/lazyslice/internal/ref"
)

// ProbeEmptiness is internal/load's half of T-0242's whole-target recheck
// (docs/reviews/2026-09-15-redteam/round4-still-leaking.json); this is a
// statement about what it finds on a real target, through the same kind of
// transaction the loader hands it — a pipeline.Tx from a Target's own Writer,
// not the gate's own connection.
func probeTx(ctx context.Context, t *testing.T, targetURL string) *tx {
	t.Helper()

	target, err := OpenTarget(ctx, dsn.DSN(targetURL))
	if err != nil {
		t.Fatalf("opening the target: %v", err)
	}
	t.Cleanup(target.Close)

	w, err := target.Writer(ctx)
	if err != nil {
		t.Fatalf("opening the writer: %v", err)
	}
	begun, err := w.Begin(ctx)
	if err != nil {
		t.Fatalf("beginning: %v", err)
	}
	rtx, ok := begun.(*tx)
	if !ok {
		t.Fatalf("Begin returned %T, want *tx", begun)
	}
	t.Cleanup(func() { _ = rtx.Rollback(context.WithoutCancel(ctx)) })
	return rtx
}

func sortedTables(ts []ref.TableRef) []ref.TableRef {
	out := append([]ref.TableRef(nil), ts...)
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}

// A table with rows is occupied; one with none is not; and one the gate's own
// bookkeeping exemption already carves out (schema_migrations in public) is
// invisible to this probe exactly as it is to the gate's — the query and the
// exemption are shared, not reimplemented.
func TestProbeEmptinessFindsOccupiedTablesAndExemptsBookkeeping(t *testing.T) {
	ctx := context.Background()
	f := newGateFixture(ctx, t)

	url := f.database(ctx, t, "probe_basic",
		`CREATE TABLE empty_one (id bigint PRIMARY KEY)`,
		`CREATE TABLE full_one (id bigint PRIMARY KEY)`,
		`INSERT INTO full_one VALUES (1)`,
		`CREATE TABLE schema_migrations (version bigint PRIMARY KEY)`,
		`INSERT INTO schema_migrations VALUES (1)`,
	)
	tx := probeTx(ctx, t, url)

	occupied, err := ProbeEmptiness(ctx, tx)
	if err != nil {
		t.Fatalf("ProbeEmptiness: %v", err)
	}
	want := []ref.TableRef{{Schema: "public", Name: "full_one"}}
	if got := sortedTables(occupied); len(got) != len(want) || got[0] != want[0] {
		t.Errorf("ProbeEmptiness = %v, want %v: empty_one has no rows and "+
			"schema_migrations is the gate's own bookkeeping exemption", got, want)
	}
}

// A table under FORCE ROW LEVEL SECURITY counts as occupied by rule, never by
// probe (ARCHITECTURE.md §9 rule 5): EXISTS lies about it exactly as it lies
// to the gate's own checkEmpty.
func TestProbeEmptinessCountsAnRLSTableAsOccupied(t *testing.T) {
	ctx := context.Background()
	f := newGateFixture(ctx, t)

	url := f.database(ctx, t, "probe_rls",
		`CREATE TABLE audit_log (id bigint PRIMARY KEY, actor text)`,
		`ALTER TABLE audit_log ENABLE ROW LEVEL SECURITY`,
		`ALTER TABLE audit_log FORCE ROW LEVEL SECURITY`,
	)
	tx := probeTx(ctx, t, url)

	occupied, err := ProbeEmptiness(ctx, tx)
	if err != nil {
		t.Fatalf("ProbeEmptiness: %v", err)
	}
	if len(occupied) != 1 || occupied[0] != (ref.TableRef{Schema: "public", Name: "audit_log"}) {
		t.Errorf("ProbeEmptiness = %v, want exactly [public.audit_log]", occupied)
	}
}

// Nothing there, nothing occupied: the zero-tables and all-empty cases both
// answer with an empty slice rather than an error.
func TestProbeEmptinessFindsNothingOnAnEmptyDatabase(t *testing.T) {
	ctx := context.Background()
	f := newGateFixture(ctx, t)

	url := f.database(ctx, t, "probe_empty",
		`CREATE TABLE orders (id bigint PRIMARY KEY)`,
	)
	tx := probeTx(ctx, t, url)

	occupied, err := ProbeEmptiness(ctx, tx)
	if err != nil {
		t.Fatalf("ProbeEmptiness: %v", err)
	}
	if len(occupied) != 0 {
		t.Errorf("ProbeEmptiness = %v, want none", occupied)
	}
}

// One table's read error must not abort the sweep for every table after it.
//
// checkEmpty runs on an autocommit *pgxpool.Conn, where a failed statement
// affects nothing but itself; ProbeEmptiness runs inside the caller's own
// transaction (T-0130's lock-and-recheck needs the same session the drops
// happen in), and in Postgres a statement error inside a transaction block
// aborts the whole transaction. Before this fix, the first table this probe
// could not read — 42501 on a table created under another role, exactly the
// case this probe exists to catch — poisoned the transaction, and every
// EXISTS after it failed with 25P02 and was folded into "occupied" the same
// way, whether or not it held a row: the refusal this feeds names every table
// alphabetically after the unreadable one, which is wrong guidance on the
// operator's only way to find out what actually happened.
//
// table_a sorts before the unreadable table and table_z after it
// (sqlUserTables orders by schema, name); table_a proves the ordinary case
// still works and table_z is the one this fix is about — it must be read
// cleanly by its own EXISTS rather than inherited as occupied from a poisoned
// transaction.
func TestProbeEmptinessSurvivesAnUnreadableTableInTheMiddle(t *testing.T) {
	ctx := context.Background()
	f := newGateFixture(ctx, t)

	const dbName = "probe_unreadable"
	targetURL := f.database(ctx, t, dbName,
		`CREATE TABLE table_a (id bigint PRIMARY KEY)`,
		`CREATE TABLE table_m (id bigint PRIMARY KEY, email text)`,
		`INSERT INTO table_m VALUES (1, 'ceo@bigcorp.example')`,
		`CREATE TABLE table_z (id bigint PRIMARY KEY)`,
	)

	const role = "probe_no_select_on_m"
	f.exec(ctx, t, f.sourceURL,
		`CREATE ROLE `+role+` LOGIN PASSWORD 'lazyslice' NOSUPERUSER`,
		`GRANT CONNECT ON DATABASE `+dbName+` TO `+role,
	)
	f.exec(ctx, t, targetURL,
		`GRANT USAGE ON SCHEMA public TO `+role,
		`GRANT SELECT ON table_a, table_z TO `+role,
		// table_m carries no SELECT grant for this role at all — the
		// third-party-created, permission-denied case.
	)

	restricted, err := url.Parse(targetURL)
	if err != nil {
		t.Fatalf("parsing the database url: %v", err)
	}
	restricted.User = url.UserPassword(role, "lazyslice")

	tx := probeTx(ctx, t, restricted.String())

	occupied, err := ProbeEmptiness(ctx, tx)
	if err != nil {
		t.Fatalf("ProbeEmptiness: %v, want no error: a table this role cannot read counts as occupied, "+
			"it does not abort the sweep", err)
	}

	want := []ref.TableRef{{Schema: "public", Name: "table_m"}}
	if got := sortedTables(occupied); len(got) != len(want) || got[0] != want[0] {
		t.Errorf("ProbeEmptiness = %v, want exactly %v: table_a and table_z are both readable and both empty, "+
			"so only the unreadable table_m should be reported — a savepoint-less sweep reports table_z too, "+
			"poisoned by table_m's own 42501", got, want)
	}
}
