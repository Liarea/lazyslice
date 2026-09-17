// SPDX-License-Identifier: Apache-2.0

//go:build integration && torture

package invariants

import (
	"context"
	"fmt"
	"net/mail"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/Liarea/lazyslice/internal/testutil"
	"github.com/Liarea/lazyslice/mask"
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

			source := connect(ctx, t, db.source)

			// Fingerprinted before runTool so that a mutation the run makes to
			// the source during execution — not just one left behind after it
			// — is caught by the comparison below. Fingerprinting after the
			// run cannot see that (docs/reviews/2026-09-09/REVIEW.md finding
			// 11); TestTortureNegativeControl proves the comparison itself
			// would catch such a mutation.
			beforeRows := fingerprintTables(ctx, t, source)
			beforeCatalog := fingerprintCatalog(ctx, t, source)

			res := runTool(ctx, t, db.dir, db.tortureArgs(s)...)
			if res.exit != s.wantExit {
				t.Fatalf("the %s run exited %d, want %d:\n  %s", s.dir, res.exit, s.wantExit, res)
			}
			if s.wantCode != "" && !strings.Contains(res.stderr, s.wantCode) {
				t.Fatalf("the %s run was expected to fail with %s and its message does not carry that code:\n  %s",
					s.dir, s.wantCode, res)
			}

			// I4 is checked for every schema, including a refusal: that is
			// the one run that aborts partway through, and so the one most
			// likely to leave a scratch table, an index, or an advanced
			// sequence behind on the source.
			compareFingerprints(t, beforeRows, fingerprintTables(ctx, t, source))
			compareCatalogs(t, beforeCatalog, fingerprintCatalog(ctx, t, source))

			if s.wantExit != 0 {
				// The refusal is the assertion beyond I4. There is no target to check.
				return
			}

			target := connect(ctx, t, db.target)

			db.assertTortureSliceReached(ctx, t, s)
			assertTortureForeignKeys(ctx, t, s, source, target)
			assertTortureRoot(ctx, t, db, s)
			assertTortureNoLiteralSurvives(ctx, t, source, target)
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

// regressionHeader parses the five required keys and the four optional keys
// testdata/regressions/README.md defines.
var regressionHeader = regexp.MustCompile(`(?m)^--\s+(root|take|expect|found|why|unique-masked|equal-masked|masked-default|not-copied|not-masked|phone-region):\s+(.*?)\s*$`)

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
//
// `expect: ok` on its own is a weak assertion for a file whose defect was a
// *collision*, because a run that copied the column verbatim would also exit 0.
// That is what the optional `unique-masked:` key is for (T-0113): it names the
// columns whose whole point is that they mask to a distinct, unusable value, and
// the target is read for exactly that.
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
			if r.phoneRegion != "" {
				args = append(args, "--phone-region", r.phoneRegion)
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
			for _, col := range r.uniqueMasked {
				assertTortureColumnIsUniquelyMasked(ctx, t, connect(ctx, t, db.target), col)
			}
			for _, pair := range r.equalMasked {
				assertTortureColumnsMaskAlike(ctx, t, connect(ctx, t, db.target), pair)
			}
			for _, col := range r.maskedDefault {
				assertTortureDefaultIsMasked(ctx, t, connect(ctx, t, db.target), col)
			}
			for _, col := range r.notCopied {
				assertTortureColumnNotCopied(ctx, t, connect(ctx, t, db.source), connect(ctx, t, db.target), col)
			}
			for _, col := range r.notMasked {
				assertTortureColumnNotMasked(t, db.configPath(), col)
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
	root string
	take int
	exit int
	code string
	why  string
	// uniqueMasked are the schema.table.column names the `unique-masked:` key
	// lists: columns the target must hold masked to distinct
	// mask.CredentialUniquePrefix values. Empty for every file that does not
	// carry the key.
	uniqueMasked []string
	// equalMasked are the `equal-masked:` pairs: two columns joined by a
	// foreign key whose masked values must still be equal in the target
	// (T-0132). Empty for every file that does not carry the key.
	equalMasked []equalPair
	// maskedDefault are the `masked-default:` key's schema.table.column
	// entries (T-0161): a masked column whose DEFAULT must hold, in the
	// target's own pg_attrdef, a masked address rather than the source's —
	// ARCHITECTURE.md §11.1 arm 1's central case. Empty for every file that
	// does not carry the key.
	maskedDefault []string
	// notCopied are the `not-copied:` key's schema.table.column entries
	// (T-0187): every distinct non-NULL value the SOURCE holds for the column
	// must not appear anywhere in the target, checked byte for byte rather
	// than by a pattern. It exists because scan_test.go's two literal
	// detectors are deliberately narrow — an email shape and a phone one, its
	// own doc comment says so — so a leak of a value neither recognises (a
	// national identifier, and any shape after it) needs its exact source
	// values read and checked, the way unique-masked and equal-masked read
	// the target directly rather than trusting expect: ok alone. Empty for
	// every file that does not carry the key.
	notCopied []string
	// notMasked are the `not-masked:` key's schema.table.column entries
	// (T-0221): the emitted yml must record no `masker:` and no `unmask:` for
	// the column, i.e. the classifier decided none. It is the negative
	// mirror of not-copied: that one proves a value the run should have
	// masked did not survive; this one proves a value the run correctly left
	// alone was not masked anyway on no real evidence -- `expect: ok` alone
	// cannot tell "left alone" from "masked, but the masker happened not to
	// change anything a --not-copied check would catch", since phone's own
	// masker accepts the same numeric families a false-positive guess would
	// have reached.
	notMasked []string
	// phoneRegion is the optional `phone-region:` key (T-0221): a value here
	// is passed to the run as --phone-region, for a regression whose defect
	// is specific to the region-aware phone reading rather than to the
	// classifier's ordinary name/value signals. Empty for every file that
	// does not carry the key, which is every file before this one.
	phoneRegion string
	image       string
}

// equalPair is one `equal-masked: CHILD = PARENT` claim. Every non-NULL value
// of child must also be a value of parent.
type equalPair struct {
	child  string
	parent string
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
	r := regression{root: fields["root"], take: take, why: fields["why"], phoneRegion: fields["phone-region"]}

	// The one optional key: a comma-separated list of schema.table.column.
	for _, spec := range strings.Split(fields["unique-masked"], ",") {
		spec = strings.TrimSpace(spec)
		if spec == "" {
			continue
		}
		if len(strings.Split(spec, ".")) != 3 {
			t.Fatalf("torture: %s: unique-masked: %q is not schema.table.column",
				filepath.Base(path), spec)
		}
		r.uniqueMasked = append(r.uniqueMasked, spec)
	}

	// The second optional key: a comma-separated list of `CHILD = PARENT`.
	for _, spec := range strings.Split(fields["equal-masked"], ",") {
		spec = strings.TrimSpace(spec)
		if spec == "" {
			continue
		}
		ends := strings.SplitN(spec, "=", 2)
		if len(ends) != 2 {
			t.Fatalf("torture: %s: equal-masked: %q is not `schema.table.column = schema.table.column`",
				filepath.Base(path), spec)
		}
		pair := equalPair{child: strings.TrimSpace(ends[0]), parent: strings.TrimSpace(ends[1])}
		for _, end := range []string{pair.child, pair.parent} {
			if len(strings.Split(end, ".")) != 3 {
				t.Fatalf("torture: %s: equal-masked: %q is not schema.table.column",
					filepath.Base(path), end)
			}
		}
		r.equalMasked = append(r.equalMasked, pair)
	}

	// The third optional key: a comma-separated list of schema.table.column,
	// each a masked column whose DEFAULT must hold a masked address in the
	// target's own catalog (T-0161).
	for _, spec := range strings.Split(fields["masked-default"], ",") {
		spec = strings.TrimSpace(spec)
		if spec == "" {
			continue
		}
		if len(strings.Split(spec, ".")) != 3 {
			t.Fatalf("torture: %s: masked-default: %q is not schema.table.column",
				filepath.Base(path), spec)
		}
		r.maskedDefault = append(r.maskedDefault, spec)
	}

	// The fourth optional key: a comma-separated list of schema.table.column,
	// each a column whose source values must not survive anywhere in the
	// target (T-0187).
	for _, spec := range strings.Split(fields["not-copied"], ",") {
		spec = strings.TrimSpace(spec)
		if spec == "" {
			continue
		}
		if len(strings.Split(spec, ".")) != 3 {
			t.Fatalf("torture: %s: not-copied: %q is not schema.table.column",
				filepath.Base(path), spec)
		}
		r.notCopied = append(r.notCopied, spec)
	}

	// The fifth optional key: a comma-separated list of schema.table.column,
	// each a column the emitted yml must record as unmasked -- no masker:,
	// no unmask: (T-0221).
	for _, spec := range strings.Split(fields["not-masked"], ",") {
		spec = strings.TrimSpace(spec)
		if spec == "" {
			continue
		}
		if len(strings.Split(spec, ".")) != 3 {
			t.Fatalf("torture: %s: not-masked: %q is not schema.table.column",
				filepath.Base(path), spec)
		}
		r.notMasked = append(r.notMasked, spec)
	}

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

// assertTortureColumnIsUniquelyMasked reads one column of the loaded target and
// asserts what a `unique-masked:` header claims: every non-NULL value carries
// mask.CredentialUniquePrefix, no two rows share one, and there is at least one
// row to say it about.
//
// It exists because of T-0113. Regressions 004 and 007 reduce a *credential*
// column under a unique index, and their whole defect was a collision: before
// mask.credentialUniqueMasker there was one credential generator, the fixed
// literal, whose Domain() is 1, so the plan could not satisfy ARCHITECTURE.md
// §5's d_required and refused at exit 12 — which is what those two files
// asserted. The escalation removed the refusal and both runs now exit 0, so the
// header had to move; but `expect: ok` alone would pass a run that copied the
// tokens into the target verbatim, which is the very thing --unmask does and
// the very thing the defect is about. This is the half of the assertion that
// exit code cannot carry.
func assertTortureColumnIsUniquelyMasked(ctx context.Context, t *testing.T, target *pgx.Conn, spec string) {
	t.Helper()

	parts := strings.Split(spec, ".")
	table := pgx.Identifier{parts[0], parts[1]}.Sanitize()
	column := pgx.Identifier{parts[2]}.Sanitize()

	var values, prefixed, distinct int64
	q := fmt.Sprintf(`SELECT count(%[1]s), count(*) FILTER (WHERE %[1]s LIKE $1), count(DISTINCT %[1]s) FROM %[2]s`,
		column, table)
	if err := target.QueryRow(ctx, q, mask.CredentialUniquePrefix+"%").Scan(&values, &prefixed, &distinct); err != nil {
		t.Fatalf("torture: reading %s in the target: %v", spec, err)
	}
	switch {
	case values == 0:
		t.Errorf("torture: %s holds no non-NULL value in the target, so masking it distinctly proves "+
			"nothing; the regression is supposed to load rows", spec)
	case prefixed != values:
		t.Errorf("torture: %d of %s's %d values in the target do not start with %q; a credential column "+
			"under a unique index masks to that prefix, and anything else is the value the source held "+
			"or the fixed literal that collides", values-prefixed, spec, values, mask.CredentialUniquePrefix)
	case distinct != values:
		t.Errorf("torture: %s holds %d distinct values over %d rows in the target; the column is under a "+
			"unique index and a repeat is the collision this regression exists for", spec, distinct, values)
	}
}

// assertTortureColumnsMaskAlike reads both ends of an `equal-masked:` pair out
// of the loaded target and asserts the relation a foreign key is: every non-NULL
// value of the child is also a value of the parent, and the child holds at least
// one value to say it about.
//
// It exists because of T-0132 and finding 3 of docs/reviews/2026-09-09/REVIEW.md.
// The plan used to escalate a unique column's masker on its own, so the parent
// of a key became `lazyslice-invalid-…` while its child kept the fixed literal,
// and the load ended at exit 8. `expect: ok` does catch *that* run — the loader
// validates the key — but it would equally pass a run that copied both columns
// verbatim, which is what `unique-masked:` on the parent rules out. The two keys
// together are the whole claim: the parent's values are masked, distinct and
// unusable, and the child holds the same ones.
func assertTortureColumnsMaskAlike(ctx context.Context, t *testing.T, target *pgx.Conn, pair equalPair) {
	t.Helper()

	child := strings.Split(pair.child, ".")
	parent := strings.Split(pair.parent, ".")
	childTable := pgx.Identifier{child[0], child[1]}.Sanitize()
	childCol := pgx.Identifier{child[2]}.Sanitize()
	parentTable := pgx.Identifier{parent[0], parent[1]}.Sanitize()
	parentCol := pgx.Identifier{parent[2]}.Sanitize()

	var values, matched int64
	q := fmt.Sprintf(`SELECT count(c.%[2]s),
	       count(*) FILTER (WHERE c.%[2]s IS NOT NULL AND EXISTS (SELECT 1 FROM %[3]s p WHERE p.%[4]s = c.%[2]s))
	  FROM %[1]s c`, childTable, childCol, parentTable, parentCol)
	if err := target.QueryRow(ctx, q).Scan(&values, &matched); err != nil {
		t.Fatalf("torture: comparing %s with %s in the target: %v", pair.child, pair.parent, err)
	}
	switch {
	case values == 0:
		t.Errorf("torture: %s holds no non-NULL value in the target, so masking it alike with %s proves "+
			"nothing; the regression is supposed to load rows", pair.child, pair.parent)
	case matched != values:
		t.Errorf("torture: %d of %s's %d values in the target are not values of %s; the two are joined by a "+
			"foreign key and must mask to the same value, which is the defect this regression exists for",
			values-matched, pair.child, values, pair.parent)
	}
}

// assertTortureDefaultIsMasked reads a masked column's DEFAULT back out of
// the target's own pg_attrdef and asserts what a `masked-default:` header
// claims (T-0161, ARCHITECTURE.md §11.1 arm 1): the literal in it is a masked
// address, not the source's own.
//
// It is deliberately independent of internal/plan's and internal/verify's own
// literal scanner: internal/invariants imports internal/testutil and nothing
// else of ours (this package's own CLAUDE.md), so this reads pg_attrdef the
// way any operator with psql could and checks the literal with net/mail, the
// standard library's own address parser, rather than a second copy of
// pipeline.Literals.
//
// `expect: ok` alone would pass a run that left the source's own default in
// place — the loader would still have recreated the object and the run would
// still exit 0 — so this is the half of the claim the exit code cannot carry,
// the way `unique-masked:` is for 004 and 007.
func assertTortureDefaultIsMasked(ctx context.Context, t *testing.T, target *pgx.Conn, spec string) {
	t.Helper()

	parts := strings.Split(spec, ".")
	var def string
	q := `SELECT pg_get_expr(ad.adbin, ad.adrelid)
	        FROM pg_attrdef ad
	        JOIN pg_class c ON c.oid = ad.adrelid
	        JOIN pg_namespace n ON n.oid = c.relnamespace
	        JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum = ad.adnum
	       WHERE n.nspname = $1 AND c.relname = $2 AND a.attname = $3`
	if err := target.QueryRow(ctx, q, parts[0], parts[1], parts[2]).Scan(&def); err != nil {
		t.Fatalf("torture: reading %s's default in the target: %v", spec, err)
	}

	m := regexp.MustCompile(`'((?:[^']|'')*)'`).FindStringSubmatch(def)
	if m == nil {
		t.Fatalf("torture: %s's default %q in the target carries no string literal to check", spec, def)
	}
	literal := strings.ReplaceAll(m[1], "''", "'")

	if _, err := mail.ParseAddress(literal); err != nil {
		t.Errorf("torture: %s's default in the target is %q, which does not parse as an address (%v); "+
			"the column masks to email and its default should mask to one too", spec, literal, err)
	}
	if strings.Contains(def, "ddl.canary") {
		t.Errorf("torture: %s's default in the target is still %q, the source's own literal; "+
			"ARCHITECTURE.md §11.1 arm 1 masks a masked column's default through its own masker and "+
			"this regression exists to prove it runs from the CLI", spec, def)
	}
}

// assertTortureColumnNotCopied reads every distinct non-NULL value a
// `not-copied:` key names out of the SOURCE and greps every cell of the whole
// TARGET for each one, byte for byte (T-0187).
//
// It exists because assertTortureNoLiteralSurvives — run unconditionally on
// every `expect: ok` regression — only ever proves the absence of the two
// shapes scan_test.go's own detectors recognise, an email and a phone number
// (that file's own doc comment: "deliberately not the classifier's... take
// every address and every number that is a phone"). A national identifier is
// neither, so a regression whose defect was one crossing verbatim needs its
// own values checked directly, the same way unique-masked and equal-masked
// read the target directly rather than trusting expect: ok alone to mean more
// than "the run did not refuse".
//
// **An array column is read one element at a time** (a review round's own
// finding on this function): a `text[]` renders as the whole literal
// (`{AB123456D,CE234567A}`) under a bare `::text` cast, and internal/transform
// masks such a column element-wise (its own CLAUDE.md, T-0118), so a
// regression where only *one* element crossed unmasked — exactly the partial
// failure the array carrier (019) exists to catch — would leave the whole
// array's rendering absent from the target while one of its elements is
// present in it, and a whole-string `strings.Contains` would not find that. So
// the type is read out of the source's own catalogue (`pg_type.typcategory`,
// which is 'A' for every array type Postgres has, not only the ones this
// package's other type tables happen to name) and an array column is unnested
// in the source query instead: what is checked is then each *element* the
// source held, the same unit internal/transform masks, so `not-copied:`
// proves no individual source value survives rather than merely that the
// array's own text rendering does not.
func assertTortureColumnNotCopied(ctx context.Context, t *testing.T, source, target *pgx.Conn, spec string) {
	t.Helper()

	col, ok := parseColumnRef(spec)
	if !ok {
		t.Fatalf("torture: not-copied: %q is not schema.table.column", spec)
	}
	ident := pgx.Identifier{col.Column}.Sanitize()

	var isArray bool
	typeQ := `SELECT t.typcategory = 'A'
	            FROM pg_attribute a
	            JOIN pg_type t ON t.oid = a.atttypid
	           WHERE a.attrelid = $1::regclass AND a.attname = $2 AND NOT a.attisdropped`
	if err := source.QueryRow(ctx, typeQ, col.Table.quoted(), col.Column).Scan(&isArray); err != nil {
		t.Fatalf("torture: reading %s's type in the source: %v", spec, err)
	}

	var q string
	if isArray {
		q = fmt.Sprintf(`SELECT DISTINCT e::text FROM %s, unnest(%s) AS e WHERE e IS NOT NULL`,
			col.Table.quoted(), ident)
	} else {
		q = fmt.Sprintf(`SELECT DISTINCT %s::text FROM %s WHERE %s IS NOT NULL`, ident, col.Table.quoted(), ident)
	}
	rows, err := source.Query(ctx, q)
	if err != nil {
		t.Fatalf("torture: reading %s in the source: %v", spec, err)
	}
	var values []string
	for rows.Next() {
		var v string
		if scanErr := rows.Scan(&v); scanErr != nil {
			t.Fatalf("torture: reading %s in the source: %v", spec, scanErr)
		}
		values = append(values, v)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatalf("torture: reading %s in the source: %v", spec, err)
	}
	if len(values) == 0 {
		t.Fatalf("torture: %s holds no non-NULL value in the source, so proving none of them crossed into "+
			"the target proves nothing; the regression is supposed to load rows", spec)
	}

	targetCells := scanCells(ctx, t, target)
	for _, v := range values {
		for _, c := range targetCells {
			if strings.Contains(c.Value, v) {
				t.Errorf("torture: %s's source value %q survives in the target, at %s: national identifiers "+
					"are not one of scan_test.go's two literal patterns, so only this direct check catches "+
					"it", spec, v, c.Column)
			}
		}
	}
}

// assertTortureColumnNotMasked is not-masked's own check (T-0221): the
// emitted yml must record no masker: and no unmask: for the column --
// internal/classify decided none. It is not-copied's negative mirror: that
// one proves a value the run should have masked did not survive anywhere in
// the target; this one proves a value the run correctly left alone was not
// masked at all, which `expect: ok` on its own cannot tell apart from "masked,
// but the masker's own output happened not to trip a not-copied check" --
// phone's masker accepts the same numeric and text families a false-positive
// guessed-region hit would have reached, so a wrongly masked column here can
// still pass every other check this suite has.
func assertTortureColumnNotMasked(t *testing.T, path, spec string) {
	t.Helper()

	want, ok := parseColumnRef(spec)
	if !ok {
		t.Fatalf("torture: not-masked: %q is not schema.table.column", spec)
	}
	cfg := readEmittedConfig(t, path)
	for name, col := range cfg.Columns {
		ref, ok := parseColumnRef(name)
		if !ok || ref != want {
			continue
		}
		if col.Masker != "" {
			t.Errorf("torture: not-masked: %s: the emitted %s records masker: %s, want none -- a "+
				"guessed-region phone hit was masked with no corroboration", spec, path, col.Masker)
		}
		if col.Unmask != nil {
			t.Errorf("torture: not-masked: %s: the emitted %s records an unmask: block, want none",
				spec, path)
		}
		return
	}
	t.Fatalf("torture: not-masked: %s has no entry in the emitted %s", spec, path)
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
	//
	// It was 37/7/1 until T-0112. `credential_unique` (mask/gen_credential.go,
	// T-HARD-A) gave CatCredential a generator wide enough for a unique column,
	// and the eighteen --unmask flags that existed only because no such
	// generator did were removed and the suite re-run: supabase-auth 6,
	// gitlab 8, mastodon 2, calcom 1, discourse 1.
	//
	// It was 19/7/1 until T-0257. internal/plan's new checkFKPairRefusal
	// (fkpair.go) reads Decision.Refused, which internal/classify has set
	// since T-0253 whenever a validated foreign key's partner cannot be
	// raised the same way; metabase's public.core_session.id/
	// public.login_history.session_id pair needed a second --unmask to
	// clear it (docs/TORTURE.md's own T-0257 section has the full account).
	byKind := map[string]int{}
	for _, s := range tortureSchemas {
		for _, f := range s.flags {
			if strings.HasPrefix(f, "--") {
				byKind[f]++
			}
		}
	}
	for kind, want := range map[string]int{"--unmask": 20, "--skip-table": 7, "--key": 1} {
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

// TestTortureNegativeControl is the negative control docs/reviews/2026-09-09/
// REVIEW.md finding 11 asked for: proof that compareFingerprints actually
// fails when the source changes between the baseline and the comparison,
// rather than trusting a suite that has never once seen its own I4 check
// fail.
//
// It does not run the tool at all. A single UPDATE between the two
// fingerprintTables calls stands in for whatever a mutation during a real run
// would do, and a fake *testing.T (a zero-value testing.T is never registered
// with a parent, so failing it cannot fail this test or anything above it)
// lets the test inspect whether compareFingerprints called Errorf without
// that failure escaping.
func TestTortureNegativeControl(t *testing.T) {
	ctx := context.Background()
	testutil.SkipWithoutDocker(ctx, t)

	url := testutil.Postgres(ctx, t, testutil.DefaultImage)
	conn := connect(ctx, t, url)

	if _, err := conn.Exec(ctx, `CREATE TABLE public.canary (id int PRIMARY KEY, note text)`); err != nil {
		t.Fatalf("torture: creating the canary table: %v", err)
	}
	if _, err := conn.Exec(ctx, `INSERT INTO public.canary VALUES (1, 'before')`); err != nil {
		t.Fatalf("torture: seeding the canary table: %v", err)
	}

	before := fingerprintTables(ctx, t, conn)

	if _, err := conn.Exec(ctx, `UPDATE public.canary SET note = 'after' WHERE id = 1`); err != nil {
		t.Fatalf("torture: mutating the canary table: %v", err)
	}

	after := fingerprintTables(ctx, t, conn)

	fake := &testing.T{}
	compareFingerprints(fake, before, after)
	if !fake.Failed() {
		t.Fatalf("torture: compareFingerprints did not fail on a source mutated between the baseline " +
			"and the comparison; the negative control this test exists to provide is not holding, " +
			"which is exactly how the ordering bug in docs/reviews/2026-09-09/REVIEW.md finding 11 " +
			"went unnoticed")
	}
}
