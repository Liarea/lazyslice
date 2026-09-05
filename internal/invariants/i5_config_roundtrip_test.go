//go:build integration

package invariants

import (
	"context"
	"os"
	"testing"
)

// TestI5EmittedConfigReproducesTheSnapshot is invariant I5: the lazyslice.yml
// the run emits, fed back in, reproduces the snapshot.
//
// ADR-004 calls the yml a record of what happened, and §10 makes the second
// run of a project the zero-question one. Both claims are only worth
// something if the file is complete: a root, a take, the caps, the depth, the
// skipped tables, the keys and the classification, all of it. This test takes
// them away one at a time in the only way that proves anything — it runs again
// with none of them on the command line and asserts the same target comes out.
//
// The two connection URLs are still passed, and that is not a hole in the
// test: §10 records the source and target as references and never a password,
// so a run driven by the yml alone would stop at Q4 for a credential. What the
// second run does not get is a single plan or classify flag.
//
// Two guards stand in front of the comparison, for the reasons I3 gives at
// length. An empty target dumps identically twice, so assertDumpHasData
// insists the target holds rows; and the second run is compared against the
// target the first run left behind, so "the config reproduced the snapshot"
// would otherwise be indistinguishable from "the second invocation exited 0
// and did nothing" — assertSecondRunHappened reads lazyslice_meta, which the
// dumps deliberately exclude, and requires a run_id that was not there before.
func TestI5EmittedConfigReproducesTheSnapshot(t *testing.T) {
	ctx := context.Background()

	for _, f := range fixtures {
		t.Run(f.name, func(t *testing.T) {
			db := start(ctx, t, f)
			target := connect(ctx, t, db.target)

			db.snapshot(ctx, t, f, f.root, f.take)
			first := dumpData(ctx, t, db.target)
			assertDumpHasData(t, "I5", "the "+f.name+" target after the first run", first)
			runsBefore := markerRunIDs(ctx, t, target, "I5")

			if _, err := os.Stat(db.configPath()); err != nil {
				t.Fatalf("I5: the run wrote no %s, so there is nothing to feed back in: %v",
					db.configPath(), err)
			}

			// No --root, no --take, no --skip-table, no --cap: everything the
			// first run was told has to come back out of the file.
			mustRun(ctx, t, db.dir,
				"--source", db.source,
				"--target", db.target,
				"--secret-file", db.secretPath(),
				"--config", db.configPath(),
				"--yes",
			)
			assertSecondRunHappened(t, "I5", runsBefore, markerRunIDs(ctx, t, target, "I5"))
			second := dumpData(ctx, t, db.target)

			if diff := diffDumps(first, second); diff != "" {
				t.Errorf("I5: the run driven by the emitted %s produced a different target, at %s",
					db.configPath(), diff)
			}
		})
	}
}
