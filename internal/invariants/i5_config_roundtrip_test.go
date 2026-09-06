// SPDX-License-Identifier: Apache-2.0

//go:build integration

package invariants

import (
	"context"
	"os"
	"path/filepath"
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
// The second run happens in a **fresh working directory** holding a copy of
// the emitted yml and nothing else. That is not tidiness. Two channels can
// carry the plan instead of the file, and re-running in the first run's own
// directory closes neither: anything the tool caches beside itself (a
// .lazyslice/ directory, a plan cache, a lock file), and the target's own
// lazyslice_meta, which §11.2 already has carrying the run's
// classification_fingerprint and tool_version. An implementation that emitted
// a yml with nothing but a `columns:` map and recovered root, take, caps,
// depth and skipped tables from either channel would pass — and the first
// person to commit that yml and run it on a clean checkout would get a
// different snapshot. The temp directory closes the first channel;
// copyEmittedConfig closes the second, by parsing the file and requiring
// `root:` and `take:` to be top-level scalars holding the run's own values
// before it is fed back.
//
// Three guards stand in front of the comparison, for the reasons I3 gives at
// length: an empty target dumps identically twice; a second invocation that
// exited 0 and did nothing leaves the first run's target in place; and
// lazyslice_meta is the implementation's own account of whether it ran, so the
// target is emptied of one loaded table between the two runs and the
// comparison is what proves the reload.
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

			config := copyEmittedConfig(t, db, f)
			emptied := db.mutateTarget(ctx, t, f, "I5")

			// No --root, no --take, no --skip-table, no --cap: everything the
			// first run was told has to come back out of the file. The working
			// directory holds the yml and nothing else, and --secret-file is
			// an absolute path back to the original, so the key is the same
			// one and nothing else travels with it.
			mustRun(ctx, t, filepath.Dir(config),
				"--source", db.source,
				"--target", db.target,
				"--secret-file", db.secretPath(),
				"--config", config,
				"--yes",
			)
			assertSecondRunHappened(t, "I5", runsBefore, markerRunIDs(ctx, t, target, "I5"))
			second := dumpData(ctx, t, db.target)

			if diff := diffDumps(first, second); diff != "" {
				t.Errorf("I5: the run driven by the emitted %s, from a directory holding nothing else, "+
					"produced a different target, at %s\n(%s was emptied between the two runs, so a "+
					"second run that reloaded nothing fails here rather than passing)",
					db.configPath(), diff, emptied)
			}
		})
	}
}

// copyEmittedConfig copies the emitted yml into a fresh directory and returns
// the copy's path, after checking that the file names the two things the
// second run is not given on the command line.
//
// The root and the take are asserted before the file is used, so that a yml
// which omits them fails on the file — naming the missing key — rather than on
// a dump diff twenty seconds later that says a row moved. They are the two
// values a reader can check by eye, and the ones an implementation is most
// likely to leave to the marker table.
//
// They are read as **top-level scalars**, not searched for as text. A
// substring search cannot tell the difference: any `columns:` key beginning
// `public.customer.` contains "public.customer", `public.people.contact`
// contains "public.people", and nasty's take of 3 is matched by `depth: 3`, by
// `row_budget: 1000000` and by any hex fingerprint with a 3 in it. The
// implementation the guard exists to catch — a yml holding nothing but a
// `columns:` map, with root, take, caps and depth recovered from
// lazyslice_meta — passes the text search and then passes the dump comparison,
// because the second run is against the same marked target.
func copyEmittedConfig(t *testing.T, db *databases, f fixture) string {
	t.Helper()

	data, err := os.ReadFile(db.configPath())
	if err != nil {
		t.Fatalf("I5: the run wrote no %s, so there is nothing to feed back in: %v", db.configPath(), err)
	}

	cfg := readEmittedConfig(t, db.configPath())
	const why = "; §10 records both as top-level keys, and ADR-004 calls this file a record of what " +
		"happened. An implementation recovering them from lazyslice_meta or from a cache in the " +
		"working directory passes a comparison and hands the next person a different snapshot"
	switch {
	case cfg.Root == "":
		t.Fatalf("I5: the emitted %s has no top-level `root:` key%s", db.configPath(), why)
	case cfg.Root != f.root:
		t.Fatalf("I5: the emitted %s records `root: %s`, and the run was made with --root %s%s",
			db.configPath(), cfg.Root, f.root, why)
	case cfg.Take == 0:
		t.Fatalf("I5: the emitted %s has no top-level `take:` key%s", db.configPath(), why)
	case cfg.Take != f.take:
		t.Fatalf("I5: the emitted %s records `take: %d`, and the run was made with --take %d%s",
			db.configPath(), cfg.Take, f.take, why)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, filepath.Base(db.configPath()))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("I5: copying the emitted config into a fresh directory: %v", err)
	}
	return path
}
