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
func TestI5EmittedConfigReproducesTheSnapshot(t *testing.T) {
	ctx := context.Background()

	for _, f := range fixtures {
		t.Run(f.name, func(t *testing.T) {
			db := start(ctx, t, f)

			db.snapshot(ctx, t, f, f.root, f.take)
			first := dumpData(ctx, t, db.target)

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
			second := dumpData(ctx, t, db.target)

			if diff := diffDumps(first, second); diff != "" {
				t.Errorf("I5: the run driven by the emitted %s produced a different target, at %s",
					db.configPath(), diff)
			}
		})
	}
}
