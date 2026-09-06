// SPDX-License-Identifier: Apache-2.0

//go:build integration

package invariants

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// TestI1ForeignKeysResolve is invariant I1: every foreign key in the target
// resolves.
//
// This is the invariant research/COMPLAINTS.md FK-10 is about. A subset that
// copies a child row and forgets its parent is not a smaller database, it is a
// broken one, and it breaks at the application's first join rather than at
// load time — the constraints are created NOT VALID and then validated
// (§11.1), so a load that got this wrong fails loudly, but a run that dropped
// the constraint or never created it would not. So the check here reads the
// target's own catalog and does the anti-join itself: for every foreign key
// the target declares, count the child rows whose referencing columns are all
// non-null and for which no parent row exists.
//
// All non-null, not any: a MATCH SIMPLE composite foreign key with any NULL
// component references nothing at all (testdata/README.md trap 4), and a
// MATCH FULL one is either wholly NULL or wholly present (trap 5), so the same
// predicate is right for both.
//
// The anti-join alone is not the invariant, because a target that declares no
// foreign key at all passes it, and so does one that recreated most of the
// source's edges and quietly dropped the one that would have failed VALIDATE.
// assertEveryForeignKeyRecreated is the other half: the source's edges are
// read with the same query and every one whose two tables exist in the target
// must be declared there too.
func TestI1ForeignKeysResolve(t *testing.T) {
	ctx := context.Background()

	for _, f := range fixtures {
		t.Run(f.name, func(t *testing.T) {
			db := start(ctx, t, f)
			db.snapshot(ctx, t, f, f.root, f.take)

			conn := connect(ctx, t, db.target)
			keys := foreignKeysOf(ctx, t, conn)
			assertEveryForeignKeyRecreated(ctx, t, f.name, connect(ctx, t, db.source), conn, keys)

			for _, fk := range keys {
				if n := danglingRows(ctx, t, conn, fk); n > 0 {
					t.Errorf("I1: %s.%s: %d row(s) reference %s.%s in the target, and no such row is there "+
						"(constraint %s)",
						fk.Child, strings.Join(fk.ChildCols, ","), n,
						fk.Parent, strings.Join(fk.ParentCols, ","),
						fk.Name)
				}
			}
		})
	}
}

// assertEveryForeignKeyRecreated fails when the target is missing an edge the
// source declares between two tables the target has.
//
// The zero case is a special case of it: a target with no foreign keys at all
// is missing all of them, and the message names each one rather than reporting
// a count of nothing. Constraints are matched on the child table and the
// constraint name together, because §11.1 recreates them from
// pg_get_constraintdef and a name is unique per table, not per schema.
//
// Source edges whose child or parent table is absent from the target are
// skipped: §11.1 recreates the tables the plan selected, and a --skip-table
// (§3.6) or an unreachable table is not an FK the target ever promised.
func assertEveryForeignKeyRecreated(
	ctx context.Context, t *testing.T, fixture string, source, target *pgx.Conn, targetKeys []foreignKey,
) {
	t.Helper()

	have := make(map[string]bool, len(targetKeys))
	for _, fk := range targetKeys {
		have[fk.Child.String()+" "+fk.Name] = true
	}
	tables := tableSet(ctx, t, target)

	var missing []string
	for _, fk := range foreignKeysOf(ctx, t, source) {
		if !tables[fk.Child] || !tables[fk.Parent] {
			continue
		}
		if have[fk.Child.String()+" "+fk.Name] {
			continue
		}
		missing = append(missing, fk.Name+" on "+fk.Child.String()+"("+strings.Join(fk.ChildCols, ",")+
			") → "+fk.Parent.String()+"("+strings.Join(fk.ParentCols, ",")+")")
	}
	if len(missing) == 0 {
		return
	}
	sort.Strings(missing)
	t.Fatalf("I1: the target declares %d foreign key(s) and the %s source declares %d more between "+
		"tables the target has, so the anti-join below is silent about them; §11.1 says every foreign "+
		"key is recreated NOT VALID and then validated:\n  %s",
		len(targetKeys), fixture, len(missing), strings.Join(missing, "\n  "))
}

// foreignKey is one declared edge of a database.
type foreignKey struct {
	Name       string
	Child      tableRef
	ChildCols  []string
	Parent     tableRef
	ParentCols []string // aligned with ChildCols
}

// foreignKeysOf reads every foreign key of a database from pg_constraint.
//
// pg_constraint rather than information_schema, which an earlier draft used
// and which cannot express two of the shapes testdata/ contains.
//
//   - A constraint name is unique per table, not per schema. Joining
//     referential_constraints to table_constraints on (schema, name) alone
//     pairs a partitioned table's cloned constraint with the wrong header —
//     public.events' FK is cloned to both its leaves under one name, so the
//     join returns nine rows for one edge — and, for two genuinely different
//     same-named constraints in one schema, merges their column lists and
//     anti-joins the wrong columns.
//   - referential_constraints.unique_constraint_name is NULL when the
//     referenced uniqueness is a bare CREATE UNIQUE INDEX rather than a table
//     constraint, so an inner join drops the edge entirely and I1 reports
//     "every foreign key resolves" having never looked at it. nasty.sql's
//     public.audit_log is index-only unique, so the shape is one fixture edit
//     away from existing.
//
// conkey and confkey are attribute-number arrays in declared order, and
// unnest WITH ORDINALITY keeps that order, which is what makes a composite key
// (testdata/README.md trap 4) line up positionally rather than alphabetically.
// conparentid = 0 keeps the constraint as it was declared and drops the copies
// PostgreSQL clones onto each partition, so one declared edge is one row.
func foreignKeysOf(ctx context.Context, t *testing.T, conn *pgx.Conn) []foreignKey {
	t.Helper()

	rows, err := conn.Query(ctx, `
		SELECT con.conname,
		       cn.nspname, cl.relname,
		       pn.nspname, pr.relname,
		       (SELECT array_agg(a.attname ORDER BY k.ord)
		          FROM unnest(con.conkey) WITH ORDINALITY AS k(attnum, ord)
		          JOIN pg_attribute a ON a.attrelid = con.conrelid AND a.attnum = k.attnum),
		       (SELECT array_agg(a.attname ORDER BY k.ord)
		          FROM unnest(con.confkey) WITH ORDINALITY AS k(attnum, ord)
		          JOIN pg_attribute a ON a.attrelid = con.confrelid AND a.attnum = k.attnum)
		FROM pg_constraint con
		JOIN pg_class cl     ON cl.oid = con.conrelid
		JOIN pg_namespace cn ON cn.oid = cl.relnamespace
		JOIN pg_class pr     ON pr.oid = con.confrelid
		JOIN pg_namespace pn ON pn.oid = pr.relnamespace
		WHERE con.contype = 'f'
		  AND con.conparentid = 0
		  AND cn.nspname NOT IN ('pg_catalog', 'information_schema')
		ORDER BY cn.nspname, cl.relname, con.conname`)
	if err != nil {
		t.Fatalf("invariants: listing foreign keys: %v", err)
	}
	defer rows.Close()

	var out []foreignKey
	for rows.Next() {
		var fk foreignKey
		if scanErr := rows.Scan(
			&fk.Name,
			&fk.Child.Schema, &fk.Child.Name,
			&fk.Parent.Schema, &fk.Parent.Name,
			&fk.ChildCols, &fk.ParentCols,
		); scanErr != nil {
			t.Fatalf("invariants: listing foreign keys: %v", scanErr)
		}
		if len(fk.ChildCols) != len(fk.ParentCols) || len(fk.ChildCols) == 0 {
			t.Fatalf("invariants: constraint %s on %s has %d referencing and %d referenced columns",
				fk.Name, fk.Child, len(fk.ChildCols), len(fk.ParentCols))
		}
		out = append(out, fk)
	}
	if rows.Err() != nil {
		t.Fatalf("invariants: listing foreign keys: %v", rows.Err())
	}
	return out
}

// tableSet is every relation a foreign key can be declared on or point at:
// ordinary and partitioned tables outside the system schemas.
//
// Partitioned roots are included here and excluded from dataTables, because
// the two questions are different: dataTables asks which relations hold rows
// of their own, and this asks which names exist. A partitioned source table
// becomes one plain table in the target (§11.1) under the same name.
func tableSet(ctx context.Context, t *testing.T, conn *pgx.Conn) map[tableRef]bool {
	t.Helper()

	rows, err := conn.Query(ctx, `
		SELECT n.nspname, c.relname
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind IN ('r', 'p')
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema')
		  AND n.nspname NOT LIKE 'pg_toast%'
		  AND n.nspname NOT LIKE 'pg_temp%'`)
	if err != nil {
		t.Fatalf("invariants: listing tables: %v", err)
	}
	defer rows.Close()

	out := map[tableRef]bool{}
	for rows.Next() {
		var ref tableRef
		if scanErr := rows.Scan(&ref.Schema, &ref.Name); scanErr != nil {
			t.Fatalf("invariants: listing tables: %v", scanErr)
		}
		out[ref] = true
	}
	if rows.Err() != nil {
		t.Fatalf("invariants: listing tables: %v", rows.Err())
	}
	return out
}

// danglingRows counts the child rows of one foreign key whose parent is not in
// the target.
func danglingRows(ctx context.Context, t *testing.T, conn *pgx.Conn, fk foreignKey) int64 {
	t.Helper()

	notNull := make([]string, 0, len(fk.ChildCols))
	join := make([]string, 0, len(fk.ChildCols))
	for i, cc := range fk.ChildCols {
		child := "c." + pgx.Identifier{cc}.Sanitize()
		parent := "p." + pgx.Identifier{fk.ParentCols[i]}.Sanitize()
		notNull = append(notNull, child+" IS NOT NULL")
		join = append(join, parent+" = "+child)
	}

	sql := "SELECT count(*) FROM " + fk.Child.quoted() + " c" +
		" WHERE " + strings.Join(notNull, " AND ") +
		" AND NOT EXISTS (SELECT 1 FROM " + fk.Parent.quoted() + " p WHERE " + strings.Join(join, " AND ") + ")"

	var n int64
	if err := conn.QueryRow(ctx, sql).Scan(&n); err != nil {
		t.Fatalf("invariants: checking constraint %s on %s: %v", fk.Name, fk.Child, err)
	}
	return n
}
