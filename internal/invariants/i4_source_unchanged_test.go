//go:build integration

package invariants

import (
	"context"
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
// The fingerprint is the row count and an md5 over the table's rows rendered
// as text and sorted, so it does not depend on physical row order: a VACUUM
// FULL or a HOT update that moved a row without changing it would otherwise
// read as a change. What it does catch is an inserted, deleted or edited row,
// in any column, including one lazyslice wrote to a bookkeeping table.
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

			db.snapshot(ctx, t, f, f.root, f.take)

			after := fingerprintTables(ctx, t, conn)
			compareFingerprints(t, before, after)
		})
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
