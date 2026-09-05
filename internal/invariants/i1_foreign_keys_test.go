//go:build integration

package invariants

import (
	"context"
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
func TestI1ForeignKeysResolve(t *testing.T) {
	ctx := context.Background()

	for _, f := range fixtures {
		t.Run(f.name, func(t *testing.T) {
			db := start(ctx, t, f)
			db.snapshot(ctx, t, f, f.root, f.take)

			conn := connect(ctx, t, db.target)
			keys := foreignKeysOf(ctx, t, conn)
			if len(keys) == 0 {
				t.Fatalf("I1: the target declares no foreign keys at all, so this invariant would pass "+
					"vacuously; %s has %d and §11.1 says they are recreated and validated",
					f.name, len(foreignKeysOf(ctx, t, connect(ctx, t, db.source))))
			}

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

// foreignKey is one declared edge of a database, as information_schema reports
// it.
type foreignKey struct {
	Name       string
	Child      tableRef
	ChildCols  []string
	Parent     tableRef
	ParentCols []string // aligned with ChildCols
}

// foreignKeysOf reads every foreign key of a database from information_schema.
//
// information_schema rather than pg_constraint because I1 is a statement about
// the database a user gets, expressed in the vocabulary that database exposes:
// a run that produced a target whose constraints are only visible through
// PostgreSQL's own catalog would still be reported here, and one that produced
// no constraints at all is caught by the vacuity check in the test.
//
// The column alignment is the fiddly part. key_column_usage gives the
// referencing columns in ordinal_position order, each carrying
// position_in_unique_constraint: the 1-based position of the matching column
// in the referenced unique or primary key constraint. That is what makes a
// composite key (testdata/README.md trap 4) line up in the declared order
// rather than alphabetically.
func foreignKeysOf(ctx context.Context, t *testing.T, conn *pgx.Conn) []foreignKey {
	t.Helper()

	rows, err := conn.Query(ctx, `
		SELECT rc.constraint_schema, rc.constraint_name,
		       child.table_schema,  child.table_name,
		       parent.table_schema, parent.table_name,
		       rc.unique_constraint_schema, rc.unique_constraint_name
		FROM information_schema.referential_constraints rc
		JOIN information_schema.table_constraints child
		  ON child.constraint_schema = rc.constraint_schema
		 AND child.constraint_name   = rc.constraint_name
		JOIN information_schema.table_constraints parent
		  ON parent.constraint_schema = rc.unique_constraint_schema
		 AND parent.constraint_name   = rc.unique_constraint_name
		WHERE child.constraint_type = 'FOREIGN KEY'
		  AND child.table_schema NOT IN ('pg_catalog', 'information_schema')
		ORDER BY rc.constraint_schema, rc.constraint_name`)
	if err != nil {
		t.Fatalf("invariants: listing foreign keys: %v", err)
	}

	type header struct {
		schema, name             string
		child, parent            tableRef
		uniqueSchema, uniqueName string
	}
	var headers []header
	for rows.Next() {
		var h header
		if scanErr := rows.Scan(
			&h.schema, &h.name,
			&h.child.Schema, &h.child.Name,
			&h.parent.Schema, &h.parent.Name,
			&h.uniqueSchema, &h.uniqueName,
		); scanErr != nil {
			rows.Close()
			t.Fatalf("invariants: listing foreign keys: %v", scanErr)
		}
		headers = append(headers, h)
	}
	if rows.Err() != nil {
		rows.Close()
		t.Fatalf("invariants: listing foreign keys: %v", rows.Err())
	}
	rows.Close()

	out := make([]foreignKey, 0, len(headers))
	for _, h := range headers {
		childCols, positions := referencingColumns(ctx, t, conn, h.schema, h.name)
		parentCols := referencedColumns(ctx, t, conn, h.uniqueSchema, h.uniqueName)

		aligned := make([]string, 0, len(childCols))
		for i, pos := range positions {
			if pos < 1 || pos > len(parentCols) {
				t.Fatalf("invariants: constraint %s on %s: column %s claims position %d in %s, which has %d columns",
					h.name, h.child, childCols[i], pos, h.uniqueName, len(parentCols))
			}
			aligned = append(aligned, parentCols[pos-1])
		}
		out = append(out, foreignKey{
			Name: h.name, Child: h.child, ChildCols: childCols,
			Parent: h.parent, ParentCols: aligned,
		})
	}
	return out
}

// referencingColumns returns the child columns of one foreign key in declared
// order, with each column's position in the referenced constraint.
func referencingColumns(ctx context.Context, t *testing.T, conn *pgx.Conn, schema, name string) ([]string, []int) {
	t.Helper()

	rows, err := conn.Query(ctx, `
		SELECT column_name, position_in_unique_constraint
		FROM information_schema.key_column_usage
		WHERE constraint_schema = $1 AND constraint_name = $2
		ORDER BY ordinal_position`, schema, name)
	if err != nil {
		t.Fatalf("invariants: reading the columns of constraint %s: %v", name, err)
	}
	defer rows.Close()

	var cols []string
	var positions []int
	for rows.Next() {
		var col string
		var pos *int
		if scanErr := rows.Scan(&col, &pos); scanErr != nil {
			t.Fatalf("invariants: reading the columns of constraint %s: %v", name, scanErr)
		}
		if pos == nil {
			t.Fatalf("invariants: constraint %s: column %s has no position in the referenced constraint", name, col)
		}
		cols = append(cols, col)
		positions = append(positions, *pos)
	}
	if rows.Err() != nil {
		t.Fatalf("invariants: reading the columns of constraint %s: %v", name, rows.Err())
	}
	return cols, positions
}

// referencedColumns returns the columns of the primary or unique constraint a
// foreign key points at, in declared order.
func referencedColumns(ctx context.Context, t *testing.T, conn *pgx.Conn, schema, name string) []string {
	t.Helper()

	rows, err := conn.Query(ctx, `
		SELECT column_name
		FROM information_schema.key_column_usage
		WHERE constraint_schema = $1 AND constraint_name = $2
		ORDER BY ordinal_position`, schema, name)
	if err != nil {
		t.Fatalf("invariants: reading the columns of constraint %s: %v", name, err)
	}
	defer rows.Close()

	var cols []string
	for rows.Next() {
		var col string
		if scanErr := rows.Scan(&col); scanErr != nil {
			t.Fatalf("invariants: reading the columns of constraint %s: %v", name, scanErr)
		}
		cols = append(cols, col)
	}
	if rows.Err() != nil {
		t.Fatalf("invariants: reading the columns of constraint %s: %v", name, rows.Err())
	}
	return cols
}

// danglingRows counts the child rows of one foreign key whose parent is not in
// the target.
func danglingRows(ctx context.Context, t *testing.T, conn *pgx.Conn, fk foreignKey) int64 {
	t.Helper()

	if len(fk.ChildCols) != len(fk.ParentCols) || len(fk.ChildCols) == 0 {
		t.Fatalf("invariants: constraint %s on %s has %d referencing and %d referenced columns",
			fk.Name, fk.Child, len(fk.ChildCols), len(fk.ParentCols))
	}

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
