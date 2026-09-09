// SPDX-License-Identifier: Apache-2.0

//go:build integration && torture

package invariants

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/Liarea/lazyslice/internal/testutil"
)

// TestTortureSchemas is `make torture`: ten real open-source schemas, each
// loaded into a container with generated rows, sliced from its most-connected
// table into a second container, and then put through the invariants.
//
// What it asserts, per schema:
//
//  1. the recorded root really is the most-connected table, by the rule
//     tortureSchema.root states — so a fixture whose shape changes cannot leave
//     the run slicing from somewhere the rule no longer picks;
//  2. the run's exit code and, when it fails, its event code;
//  3. and, for a run that succeeded: the tables the slice had to reach (§6 item
//     5), I1 (every foreign key in the target resolves and every source edge
//     between two present tables was recreated), I4 (the source is unchanged, in
//     rows and in catalog), I6 where the schema has a countable root, and the
//     grep half of I2 (no email address or phone number of the source's survives
//     anywhere in the target).
//
// I3 and I5 are deliberately not here. Both compare two `pg_dump --data-only`
// runs of the target, which is a second full snapshot per schema and, on the
// four largest schemas, the majority of the suite's time; and neither is a
// statement about the *schema*, which is what these ten fixtures are for. They
// stay on pagila and nasty under `make integration`, where they run on every
// change rather than on this target alone.
func TestTortureSchemas(t *testing.T) {
	ctx := context.Background()

	for _, s := range tortureSchemas {
		t.Run(s.dir, func(t *testing.T) {
			db := startTorture(ctx, t, s)

			assertRootIsMostConnected(ctx, t, db, s)

			res := runTool(ctx, t, db.dir, db.tortureArgs(s)...)
			if res.exit != s.wantExit {
				t.Fatalf("the %s run exited %d, want %d:\n  %s", s.dir, res.exit, s.wantExit, res)
			}
			if s.wantCode != "" && !strings.Contains(res.stderr, s.wantCode) {
				t.Fatalf("the %s run was expected to fail with %s and its message does not carry that code:\n  %s",
					s.dir, s.wantCode, res)
			}
			if s.wantExit != 0 {
				// The refusal is the assertion. There is no target to check.
				return
			}

			source := connect(ctx, t, db.source)
			target := connect(ctx, t, db.target)

			beforeRows := fingerprintTables(ctx, t, source)
			beforeCatalog := fingerprintCatalog(ctx, t, source)

			db.assertTortureSliceReached(ctx, t, s)
			assertTortureForeignKeys(ctx, t, s, source, target)
			assertTortureRoot(ctx, t, db, s)
			assertTortureNoLiteralSurvives(ctx, t, source, target)

			compareFingerprints(t, beforeRows, fingerprintTables(ctx, t, source))
			compareCatalogs(t, beforeCatalog, fingerprintCatalog(ctx, t, source))
		})
	}
}

// startTorture is start() for a torture schema: two containers on the schema's
// own image, and the four scripts loaded into the source.
func startTorture(ctx context.Context, t *testing.T, s tortureSchema) *databases {
	t.Helper()

	return start(ctx, t, fixture{
		name:  s.dir,
		image: s.image,
		load:  s.load,
	})
}

// tortureArgs is the run: this source, this target, this root, this many rows,
// no questions, plus the schema's recorded flags.
func (d *databases) tortureArgs(s tortureSchema) []string {
	args := []string{
		"--source", d.source,
		"--target", d.target,
		"--root", s.root,
		"--take", strconv.Itoa(s.take),
		"--secret-file", d.secretPath(),
		"--config", d.configPath(),
		"--yes",
	}
	return append(args, s.flags...)
}

// ---------- the root ----------

// assertRootIsMostConnected fails unless the schema's recorded root is the table
// the rule in tortureSchema.root picks out of the loaded source.
//
// It is here because the root is the one part of the run nobody can check by
// reading: "the most-connected table" is a claim about a graph of a thousand
// tables, and a fixture whose generator changes a row count can move it. The
// query is the rule, written once: degree over declared constraints (each
// counted once, both directions), then incoming degree, then live row count,
// then name.
func assertRootIsMostConnected(ctx context.Context, t *testing.T, db *databases, s tortureSchema) {
	t.Helper()

	conn := connect(ctx, t, db.source)
	var got string
	err := conn.QueryRow(ctx, `
		WITH e AS (
			SELECT c.conrelid AS child, c.confrelid AS parent
			FROM pg_constraint c
			WHERE c.contype = 'f' AND c.conparentid = 0
		), d AS (
			SELECT parent AS t, 1 AS incoming FROM e
			UNION ALL
			SELECT child, 0 FROM e
		), agg AS (
			SELECT t, count(*) AS degree, sum(incoming) AS incoming FROM d GROUP BY t
		)
		SELECT n.nspname || '.' || c.relname
		FROM agg a
		JOIN pg_class c ON c.oid = a.t
		JOIN pg_namespace n ON n.oid = c.relnamespace
		ORDER BY a.degree DESC, a.incoming DESC, c.reltuples DESC, n.nspname, c.relname
		LIMIT 1`).Scan(&got)
	if err != nil {
		t.Fatalf("torture: computing the most-connected table of %s: %v", s.dir, err)
	}
	if got != s.root {
		t.Errorf("torture: %s slices from %s, but the most-connected table of the loaded source is %s; "+
			"either the fixture changed shape or the catalogue is stale (testdata/torture/%s/README.md "+
			"records the root and the rule)", s.dir, s.root, got, s.dir)
	}
}

// ---------- the invariants, per schema ----------

// assertTortureSliceReached is fixture.assertSliceReached with a tortureSchema's
// two lists. It is the §6 item 5 guard: without it every check below is
// satisfied by a target holding the root and nothing else.
func (d *databases) assertTortureSliceReached(ctx context.Context, t *testing.T, s tortureSchema) {
	t.Helper()

	d.assertSliceReached(ctx, t, fixture{
		name:         s.dir,
		root:         s.root,
		take:         s.take,
		mustHoldRows: s.mustHoldRows,
		mustBeSubset: s.mustBeSubset,
	})
}

// assertTortureForeignKeys is I1 over one torture target: every edge the target
// declares resolves, and every edge the source declares between two tables the
// target has was recreated.
func assertTortureForeignKeys(ctx context.Context, t *testing.T, s tortureSchema, source, target *pgx.Conn) {
	t.Helper()

	keys := foreignKeysOf(ctx, t, target)
	assertEveryForeignKeyRecreated(ctx, t, s.dir, source, target, keys)
	for _, fk := range keys {
		if n := danglingRows(ctx, t, target, fk); n > 0 {
			t.Errorf("I1 on %s: %s.%s: %d row(s) reference %s.%s in the target, and no such row is there "+
				"(constraint %s)",
				s.dir, fk.Child, strings.Join(fk.ChildCols, ","), n,
				fk.Parent, strings.Join(fk.ParentCols, ","), fk.Name)
		}
	}
}

// assertTortureRoot is I6: the counted root holds exactly --take rows.
//
// A schema with no countedRoot contributes no case, and that is stated in the
// catalogue rather than defaulted: the invariant only holds for a root no
// selected row reaches as a parent, and GitLab's namespaces (self-referencing
// through parent_id) and Odoo's res_users (referenced by 330 tables, several of
// which the slice reaches) are both roots a correct run legitimately widens.
func assertTortureRoot(ctx context.Context, t *testing.T, db *databases, s tortureSchema) {
	t.Helper()

	if s.countedRoot == "" {
		return
	}
	ref := parseTable(t, s.countedRoot)
	source := countRows(ctx, t, connect(ctx, t, db.source), ref)
	if source <= int64(s.countedTake) {
		t.Fatalf("I6 on %s: %s holds %d row(s) in the source and --take is %d, so a target holding every "+
			"row would pass; lower countedTake in torture_catalogue_test.go",
			s.dir, ref, source, s.countedTake)
	}
	if got := countRows(ctx, t, connect(ctx, t, db.target), ref); got != int64(s.countedTake) {
		t.Errorf("I6 on %s: the root %s holds %d row(s) in the target, want %d (--take %d)",
			s.dir, ref, got, s.countedTake, s.countedTake)
	}
}

// assertTortureNoLiteralSurvives is the grep half of I2: every email address and
// phone number the source holds, looked for in every cell of the target.
//
// It is the half that needs no configuration and cannot be satisfied by
// omission: it does not read the run's own yml, so a masker that quietly copied
// its input through is caught whatever the run wrote about itself. A source that
// held no personal literal at all is a hard failure, because then the check
// proves nothing — every one of the ten generators puts addresses and phone
// numbers in on purpose (testdata/torture/*/generate.sql).
func assertTortureNoLiteralSurvives(ctx context.Context, t *testing.T, source, target *pgx.Conn) {
	t.Helper()

	literals := collectPersonalLiterals(scanCells(ctx, t, source))
	if len(literals) == 0 {
		t.Fatal("I2: the source holds no email address and no phone number, so finding none in the target " +
			"proves nothing; the schema's generate.sql is supposed to put them there")
	}
	assertNoSourceLiteralSurvives(t, scanCells(ctx, t, target), literals)
}

// ---------- the regressions ----------

// regressionHeader parses the five keys testdata/regressions/README.md defines.
var regressionHeader = regexp.MustCompile(`(?m)^--\s+(root|take|expect|found|why):\s+(.*?)\s*$`)

// TestTortureRegressions runs every file in testdata/regressions/ and asserts it
// still behaves the way its header says.
//
// Each of those files is a defect one of the ten schemas found, reduced to the
// smallest schema that still shows it. They are here rather than in
// testdata/torture/ because they are not schemas anybody uses: they are the
// difference between "GitLab fails" and a test that says why, and they are what
// stops the next change bringing the failure back without anyone loading GitLab.
//
// A file whose header says `expect: ok` must exit 0. One that says
// `expect: exit N code` must exit N with that code in its message — several of
// these are refusals lazyslice is right to make, and what was wrong was how it
// made them.
func TestTortureRegressions(t *testing.T) {
	ctx := context.Background()

	root, err := repoRoot()
	if err != nil {
		t.Fatalf("torture: %v", err)
	}
	dir := filepath.Join(root, "testdata", "regressions")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("torture: reading %s: %v", dir, err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	if len(files) == 0 {
		t.Fatalf("torture: %s holds no .sql file, so this test asserts nothing; if the regressions have "+
			"been deleted, delete this test with them", dir)
	}

	for _, name := range files {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(dir, name)
			r := parseRegression(t, path)

			db := start(ctx, t, fixture{
				name:  name,
				image: r.image,
				load: func(ctx context.Context, connURL string) error {
					return runScript(ctx, connURL, path)
				},
			})

			args := []string{
				"--source", db.source,
				"--target", db.target,
				"--root", r.root,
				"--take", strconv.Itoa(r.take),
				"--secret-file", db.secretPath(),
				"--config", db.configPath(),
				"--yes",
			}
			res := runTool(ctx, t, db.dir, args...)
			if res.exit != r.exit {
				t.Fatalf("%s exited %d, want %d (%s):\n  %s", name, res.exit, r.exit, r.why, res)
			}
			if r.code != "" && !strings.Contains(res.stderr, r.code) {
				t.Fatalf("%s exited %d as expected but its message does not carry %s (%s):\n  %s",
					name, res.exit, r.code, r.why, res)
			}
			if r.exit != 0 {
				return
			}
			// Every regression that is expected to succeed is also expected not
			// to leak. Some of these defects never changed an exit code at all:
			// 008 exited 0 both before and after, and the only thing that told
			// anyone a jsonb column of names and addresses had been copied
			// verbatim was I2's grep half. A regression suite that only compared
			// exit codes would have nothing to say about it.
			assertTortureNoLiteralSurvives(ctx, t, connect(ctx, t, db.source), connect(ctx, t, db.target))
		})
	}
}

// regression is one file's header.
type regression struct {
	root  string
	take  int
	exit  int
	code  string
	why   string
	image string
}

// parseRegression reads the header block. A missing or malformed key is a hard
// failure naming the file: a regression whose header nobody can read is one this
// test would silently skip.
func parseRegression(t *testing.T, path string) regression {
	t.Helper()

	text, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("torture: reading %s: %v", filepath.Base(path), err)
	}
	fields := map[string]string{}
	for _, m := range regressionHeader.FindAllStringSubmatch(string(text), -1) {
		if _, seen := fields[m[1]]; !seen {
			fields[m[1]] = m[2]
		}
	}
	for _, key := range []string{"root", "take", "expect", "found", "why"} {
		if fields[key] == "" {
			t.Fatalf("torture: %s has no `-- %s:` header line; testdata/regressions/README.md defines the "+
				"five keys and this test parses them", filepath.Base(path), key)
		}
	}
	take, err := strconv.Atoi(fields["take"])
	if err != nil {
		t.Fatalf("torture: %s: take: %v", filepath.Base(path), err)
	}
	r := regression{root: fields["root"], take: take, why: fields["why"]}

	// pgvector is not needed by any regression today; the field exists so that
	// one reduced from discourse can say so without a second mechanism.
	if strings.Contains(string(text), "CREATE EXTENSION IF NOT EXISTS vector") {
		r.image = "pgvector/pgvector:pg16"
	}

	switch expect := fields["expect"]; {
	case expect == "ok":
		r.exit = 0
	case strings.HasPrefix(expect, "exit "):
		parts := strings.Fields(expect)
		if len(parts) != 3 {
			t.Fatalf("torture: %s: expect: %q is not `ok` or `exit <n> <event.code>`", filepath.Base(path), expect)
		}
		n, err := strconv.Atoi(parts[1])
		if err != nil {
			t.Fatalf("torture: %s: expect: %v", filepath.Base(path), err)
		}
		r.exit, r.code = n, parts[2]
	default:
		t.Fatalf("torture: %s: expect: %q is not `ok` or `exit <n> <event.code>`", filepath.Base(path), expect)
	}
	return r
}

// ---------- the catalogue's own guards ----------

// TestTortureCatalogueMatchesTheFixtures fails when the catalogue and the
// directory have drifted apart: a schema on disk that nothing runs, or a
// catalogue entry whose files are gone.
//
// It runs without Docker, which is the point — a contributor who adds a schema
// and forgets the catalogue finds out from `go vet`-speed feedback rather than
// from twenty minutes of containers.
func TestTortureCatalogueMatchesTheFixtures(t *testing.T) {
	dir, err := tortureRoot()
	if err != nil {
		t.Fatalf("torture: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("torture: reading %s: %v", dir, err)
	}

	onDisk := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() && e.Name() != "_common" {
			onDisk[e.Name()] = true
		}
	}
	inCatalogue := map[string]bool{}
	for _, s := range tortureSchemas {
		inCatalogue[s.dir] = true
		for _, f := range []string{"schema.sql", "generate.sql", "README.md"} {
			if _, err := os.Stat(filepath.Join(dir, s.dir, f)); err != nil {
				t.Errorf("torture: the catalogue names %s and %s is missing: %v", s.dir, f, err)
			}
		}
		if s.root == "" || s.take <= 0 {
			t.Errorf("torture: %s names no root or no take", s.dir)
		}
		if s.wantExit == 0 && len(s.mustHoldRows) == 0 {
			t.Errorf("torture: %s expects a successful run and names no table the slice must reach, so "+
				"every check in TestTortureSchemas is satisfied by a target holding the root alone", s.dir)
		}
	}
	for name := range onDisk {
		if !inCatalogue[name] {
			t.Errorf("torture: testdata/torture/%s is not in tortureSchemas, so nothing runs it", name)
		}
	}
	for name := range inCatalogue {
		if !onDisk[name] {
			t.Errorf("torture: tortureSchemas names %s and testdata/torture/%s does not exist", name, name)
		}
	}
	if len(tortureSchemas) != 10 {
		t.Errorf("torture: the catalogue holds %d schemas; docs/BUILD_PLAN.md's gate 5 says ten",
			len(tortureSchemas))
	}
	failing := 0
	for _, s := range tortureSchemas {
		if s.wantExit != 0 {
			failing++
		}
	}
	if failing > 1 {
		t.Errorf("torture: %d of the ten schemas are recorded as failing; gate 5 allows one, and it has to "+
			"fail with a message that names the exact cause", failing)
	}

	// The flag counts are gate-5 evidence, and the kinds are not
	// interchangeable: --unmask copies a column of personal data into the target
	// verbatim, --skip-table drops a table and --key supplies a key the ladder
	// could not find. docs/TORTURE.md and internal/invariants/CLAUDE.md both
	// quote this split, and it was quoted wrong once (as forty-five --unmask
	// flags) because nothing counted it. This is what they are counted from.
	byKind := map[string]int{}
	for _, s := range tortureSchemas {
		for _, f := range s.flags {
			if strings.HasPrefix(f, "--") {
				byKind[f]++
			}
		}
	}
	for kind, want := range map[string]int{"--unmask": 37, "--skip-table": 7, "--key": 1} {
		if byKind[kind] != want {
			t.Errorf("torture: the catalogue holds %d %s flags and docs/TORTURE.md says %d; "+
				"re-measure the split there and on ROADMAP.md's gate-5 line (T-0106) rather than "+
				"editing this number alone", byKind[kind], kind, want)
		}
	}
}

// TestTortureImagesAreReachable is the one thing about the catalogue that a
// person cannot check by reading it: whether the image a schema names can be
// pulled at all. It is a skip without Docker and a failure with it.
func TestTortureImagesAreReachable(t *testing.T) {
	ctx := context.Background()
	testutil.SkipWithoutDocker(ctx, t)

	seen := map[string]bool{}
	for _, s := range tortureSchemas {
		if s.image == "" || seen[s.image] {
			continue
		}
		seen[s.image] = true
		image := s.image
		t.Run(image, func(t *testing.T) {
			url := testutil.Postgres(ctx, t, image)
			conn := connect(ctx, t, url)
			var one int
			if err := conn.QueryRow(ctx, "SELECT 1").Scan(&one); err != nil {
				t.Fatalf("torture: %s did not answer: %v", image, err)
			}
		})
	}
}
