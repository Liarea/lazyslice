//go:build integration

package invariants

import (
	"context"
	"os"
	"testing"
)

// TestI3SameInputsSameTarget is invariant I3: the same source, the same
// secret and the same config produce a byte-identical target.
//
// This is what makes ADR-004's yml "a record of what happened" worth anything,
// and it is the property that lets a developer re-run a snapshot without every
// masked value in their local database changing under them. §3 "Determinism"
// spells out what has to hold for it: a FIFO queue, tables in (schema, name)
// order, edges in constraint-name order, and nothing iterating a Go map.
//
// The two runs go into the same target container, which is also the re-run
// path §11.2 describes: the second run finds its own bound marker and
// truncates. A determinism bug and a broken re-run therefore both fail here,
// and the message says which line of which table moved.
//
// Two things are asserted before the comparison, because "the two dumps are
// equal" is the easiest assertion in this package to satisfy by accident. A
// data dump of a database with no rows is a few hundred bytes of SET
// boilerplate and is byte-identical between runs, so assertDumpHasData
// requires the target to hold data at all; and the second run is compared
// against the state the first run left behind, so an invocation that exited 0
// without truncating or reloading would pass perfectly, which
// assertSecondRunHappened rules out through lazyslice_meta.
func TestI3SameInputsSameTarget(t *testing.T) {
	ctx := context.Background()

	for _, f := range fixtures {
		t.Run(f.name, func(t *testing.T) {
			db := start(ctx, t, f)
			target := connect(ctx, t, db.target)

			db.snapshot(ctx, t, f, f.root, f.take)
			first := dumpData(ctx, t, db.target)
			assertDumpHasData(t, "I3", "the "+f.name+" target after the first run", first)
			runsBefore := markerRunIDs(ctx, t, target, "I3")

			// "Same secret" is only a claim if the key was persisted. When the
			// key is ephemeral the two runs mask differently by design (§5), so
			// an I3 that did not check this would report a determinism failure
			// for a repository problem.
			if _, err := os.Stat(db.secretPath()); err != nil {
				t.Fatalf("I3: %s was not written, so the second run cannot use the same masking key: %v",
					db.secretPath(), err)
			}

			db.snapshot(ctx, t, f, f.root, f.take)
			assertSecondRunHappened(t, "I3", runsBefore, markerRunIDs(ctx, t, target, "I3"))
			second := dumpData(ctx, t, db.target)

			if diff := diffDumps(first, second); diff != "" {
				t.Errorf("I3: two runs with the same source, secret and config produced different targets, at %s", diff)
			}
		})
	}
}
