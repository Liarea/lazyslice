//go:build integration

package invariants

import (
	"context"
	"testing"
)

// TestI6RootHoldsTakeRows is invariant I6: the root table of the target holds
// exactly --take rows.
//
// It is the smallest of the six and the one a developer checks first, because
// it is the number they typed. §6 item 5 states it in the general form — every
// step with mode ChildOK or ParentOnly has exactly Keys.Len() rows in the
// target — and the root is the case where the expected number is not the
// tool's own arithmetic but the flag.
//
// The root each fixture uses here is not necessarily the one the other five
// invariants slice from: I6 only holds for a root that no selected row reaches
// as a parent. harness_test.go's fixture.countedRoot says which, and why.
func TestI6RootHoldsTakeRows(t *testing.T) {
	ctx := context.Background()

	for _, f := range fixtures {
		t.Run(f.name, func(t *testing.T) {
			db := start(ctx, t, f)
			root := parseTable(t, f.countedRoot)

			source := countRows(ctx, t, connect(ctx, t, db.source), root)
			if source <= int64(f.countedTake) {
				t.Fatalf("I6: %s holds %d row(s) in the %s source and --take is %d, so a target holding "+
					"every row would pass; lower countedTake in harness_test.go",
					root, source, f.name, f.countedTake)
			}

			db.snapshot(ctx, t, f, f.countedRoot, f.countedTake)

			if got := countRows(ctx, t, connect(ctx, t, db.target), root); got != int64(f.countedTake) {
				t.Errorf("I6: the root %s holds %d row(s) in the target, want %d (--take %d)",
					root, got, f.countedTake, f.countedTake)
			}
		})
	}
}
