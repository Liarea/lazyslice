// SPDX-License-Identifier: Apache-2.0

//go:build integration

package invariants

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/jackc/pgx/v5"
)

// TestI4SourceUnchanged is invariant I4: the run does not change the source.
//
// It is the promise on the front of CONCEPT.md and the one whose violation
// nobody would notice until it mattered, because the source of a real run is
// production. Every source transaction is REPEATABLE READ READ ONLY and every
// statement passes a shape allowlist (§2 "Source"), and a unit test asserts
// that internal/pg never calls CopyFrom, SendBatch or Prepare on a source
// connection — but all of that is a statement about our code. This is the
// statement about the database: a fingerprint of every table before the run
// and the same fingerprint after it.
//
// Two fingerprints, because "unchanged" is broader than "the same rows".
//
// The row fingerprint is the row count and an md5 over the table's rows
// rendered as text and sorted, so it does not depend on physical row order: a
// VACUUM FULL or a HOT update that moved a row without changing it would
// otherwise read as a change. What it catches is an inserted, deleted or
// edited row, in any column, including one lazyslice wrote to a bookkeeping
// table.
//
// The catalog fingerprint is everything a run can leave behind that is not a
// row of an ordinary table, and it is the half an earlier version of this test
// did not have. An index created to make a keyed extract tractable and never
// dropped is a normal shortcut and a real production hazard — it takes locks,
// it consumes disk, and it outlives the run — and it changes no row anywhere.
// So do a sequence advanced by nextval, a materialised view refreshed, a
// temporary or scratch table, a trigger disabled, a comment or a statistics
// object. Each is reported as its own message naming the object, rather than
// folded into a per-table row comparison that would say nothing about it.
func TestI4SourceUnchanged(t *testing.T) {
	ctx := context.Background()

	for _, f := range fixtures {
		t.Run(f.name, func(t *testing.T) {
			db := start(ctx, t, f)
			conn := connect(ctx, t, db.source)

			before := fingerprintTables(ctx, t, conn)
			if len(before) == 0 {
				t.Fatalf("I4: the %s source has no tables to fingerprint", f.name)
			}
			catalogBefore := fingerprintCatalog(ctx, t, conn)

			db.snapshot(ctx, t, f, f.root, f.take)

			after := fingerprintTables(ctx, t, conn)
			compareFingerprints(t, before, after)
			compareCatalogs(t, catalogBefore, fingerprintCatalog(ctx, t, conn))
		})
	}
}

// catalogQueries is the catalog half of I4, one query per class of object a
// run could leave behind. Each returns one ordered text column; the md5 over
// it is compared before and after.
//
// They are separate rather than one union so that a difference names what
// changed: "the run created or dropped an index on the source" is actionable
// in a way that "the catalog changed" is not. System schemas are excluded
// throughout, and so is anything under pg_temp: a temporary table is invisible
// to another session anyway, so its absence here is a limitation this comment
// records rather than a check.
var catalogQueries = []struct {
	what string
	sql  string
}{
	{
		what: "a relation (table, index, view, materialised view, sequence or type)",
		sql: `SELECT n.nspname || '.' || c.relname || ' ' || c.relkind::text
		      FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		      WHERE n.nspname NOT IN ('pg_catalog', 'information_schema')
		        AND n.nspname NOT LIKE 'pg_toast%' AND n.nspname NOT LIKE 'pg_temp%'
		      ORDER BY 1`,
	},
	{
		what: "an index",
		sql: `SELECT n.nspname || '.' || ci.relname || ' on ' || ct.relname || ' (' || ix.indkey::text || ')'
		      FROM pg_index ix
		      JOIN pg_class ci ON ci.oid = ix.indexrelid
		      JOIN pg_class ct ON ct.oid = ix.indrelid
		      JOIN pg_namespace n ON n.oid = ci.relnamespace
		      WHERE n.nspname NOT IN ('pg_catalog', 'information_schema')
		        AND n.nspname NOT LIKE 'pg_toast%' AND n.nspname NOT LIKE 'pg_temp%'
		      ORDER BY 1`,
	},
	{
		what: "a column",
		sql: `SELECT n.nspname || '.' || c.relname || '.' || a.attname || ' ' ||
		             format_type(a.atttypid, a.atttypmod) || ' ' || a.attnotnull::text
		      FROM pg_attribute a
		      JOIN pg_class c ON c.oid = a.attrelid
		      JOIN pg_namespace n ON n.oid = c.relnamespace
		      WHERE a.attnum > 0 AND NOT a.attisdropped
		        AND n.nspname NOT IN ('pg_catalog', 'information_schema')
		        AND n.nspname NOT LIKE 'pg_toast%' AND n.nspname NOT LIKE 'pg_temp%'
		      ORDER BY 1`,
	},
	{
		what: "a constraint",
		sql: `SELECT n.nspname || '.' || con.conname || ' ' || con.contype::text || ' ' ||
		             con.convalidated::text
		      FROM pg_constraint con JOIN pg_namespace n ON n.oid = con.connamespace
		      WHERE n.nspname NOT IN ('pg_catalog', 'information_schema')
		      ORDER BY 1`,
	},
	{
		what: "a trigger's enabled state",
		sql: `SELECT n.nspname || '.' || c.relname || '.' || tg.tgname || ' ' || tg.tgenabled::text
		      FROM pg_trigger tg
		      JOIN pg_class c ON c.oid = tg.tgrelid
		      JOIN pg_namespace n ON n.oid = c.relnamespace
		      WHERE NOT tg.tgisinternal
		        AND n.nspname NOT IN ('pg_catalog', 'information_schema')
		      ORDER BY 1`,
	},
}

// fingerprintCatalog hashes each catalog query's ordered output, and reads the
// position of every sequence separately.
//
// The sequences are read as values rather than as catalog rows because that is
// where the change would be: a run that called nextval on a source sequence
// leaves pg_class untouched and last_value moved, and the next insert a person
// makes into production skips an id.
func fingerprintCatalog(ctx context.Context, t *testing.T, conn *pgx.Conn) map[string]string {
	t.Helper()

	out := map[string]string{}
	for _, q := range catalogQueries {
		var digest string
		sql := `SELECT coalesce(md5(string_agg(r, E'\n')), '') FROM (` + q.sql + `) s(r)`
		if err := conn.QueryRow(ctx, sql).Scan(&digest); err != nil {
			t.Fatalf("invariants: fingerprinting %s in the source: %v", q.what, err)
		}
		out[q.what] = digest
	}

	for _, seq := range sequences(ctx, t, conn) {
		var last int64
		var called bool
		if err := conn.QueryRow(ctx, `SELECT last_value, is_called FROM `+seq.quoted()).Scan(&last, &called); err != nil {
			t.Fatalf("invariants: reading the sequence %s in the source: %v", seq, err)
		}
		out["the sequence "+seq.String()] = fmt.Sprintf("%d/%t", last, called)
	}
	return out
}

// sequences lists every sequence outside the system schemas.
func sequences(ctx context.Context, t *testing.T, conn *pgx.Conn) []tableRef {
	t.Helper()

	rows, err := conn.Query(ctx, `
		SELECT n.nspname, c.relname
		FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind = 'S'
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema')
		  AND n.nspname NOT LIKE 'pg_temp%'
		ORDER BY n.nspname, c.relname`)
	if err != nil {
		t.Fatalf("invariants: listing sequences: %v", err)
	}
	defer rows.Close()

	var out []tableRef
	for rows.Next() {
		var ref tableRef
		if scanErr := rows.Scan(&ref.Schema, &ref.Name); scanErr != nil {
			t.Fatalf("invariants: listing sequences: %v", scanErr)
		}
		out = append(out, ref)
	}
	if rows.Err() != nil {
		t.Fatalf("invariants: listing sequences: %v", rows.Err())
	}
	return out
}

// compareCatalogs reports each class of catalog object that differs, naming it.
func compareCatalogs(t *testing.T, before, after map[string]string) {
	t.Helper()

	keys := map[string]bool{}
	for k := range before {
		keys[k] = true
	}
	for k := range after {
		keys[k] = true
	}
	names := make([]string, 0, len(keys))
	for k := range keys {
		names = append(names, k)
	}
	sort.Strings(names)

	for _, what := range names {
		b, hadBefore := before[what]
		a, hadAfter := after[what]
		switch {
		case !hadBefore:
			t.Errorf("I4: the run created %s in the source", what)
		case !hadAfter:
			t.Errorf("I4: the run dropped %s from the source", what)
		case b != a:
			t.Errorf("I4: the run changed %s in the source (%s before, %s after); no row of any ordinary "+
				"table has to move for that to be true — an index left behind to make an extract "+
				"tractable, or a sequence advanced by nextval, is a change to production that outlives "+
				"the run", what, b, a)
		}
	}
}

// fingerprint is one table's row count and content hash.
type fingerprint struct {
	Rows int64
	MD5  string
}

// fingerprintTables fingerprints every ordinary table of a database.
func fingerprintTables(ctx context.Context, t *testing.T, conn *pgx.Conn) map[tableRef]fingerprint {
	t.Helper()

	out := map[tableRef]fingerprint{}
	for _, ref := range dataTables(ctx, t, conn) {
		// t::text renders the whole row through the type output functions, so
		// a change in any column of any row changes the aggregate; ORDER BY
		// makes the aggregate independent of physical order. An empty table
		// hashes to the empty string rather than to NULL.
		sql := `SELECT count(*), coalesce(md5(string_agg(r, E'\n' ORDER BY r)), '')
		        FROM (SELECT x::text AS r FROM ` + ref.quoted() + ` x) s`

		var fp fingerprint
		if err := conn.QueryRow(ctx, sql).Scan(&fp.Rows, &fp.MD5); err != nil {
			t.Fatalf("invariants: fingerprinting %s: %v", ref, err)
		}
		out[ref] = fp
	}
	return out
}

// compareFingerprints reports every table that changed, naming it.
func compareFingerprints(t *testing.T, before, after map[tableRef]fingerprint) {
	t.Helper()

	refs := map[tableRef]bool{}
	for ref := range before {
		refs[ref] = true
	}
	for ref := range after {
		refs[ref] = true
	}

	names := make([]tableRef, 0, len(refs))
	for ref := range refs {
		names = append(names, ref)
	}
	sort.Slice(names, func(i, j int) bool { return names[i].String() < names[j].String() })

	for _, ref := range names {
		b, hadBefore := before[ref]
		a, hadAfter := after[ref]
		switch {
		case !hadBefore:
			t.Errorf("I4: the run created %s in the source", ref)
		case !hadAfter:
			t.Errorf("I4: the run dropped %s from the source", ref)
		case b.Rows != a.Rows:
			t.Errorf("I4: %s had %d row(s) in the source before the run and %d after", ref, b.Rows, a.Rows)
		case b.MD5 != a.MD5:
			t.Errorf("I4: %s still has %d row(s) in the source but their contents changed "+
				"(md5 %s before the run, %s after)", ref, b.Rows, b.MD5, a.MD5)
		}
	}
}
