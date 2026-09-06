// SPDX-License-Identifier: Apache-2.0

//go:build integration

package invariants

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Liarea/lazyslice/internal/testutil"
)

// runTimeout bounds one invocation of the binary. It is generous because a run
// pulls, plans, extracts and loads a whole fixture; it exists so that a
// deadlocked pipeline fails with its own message rather than with the package
// timeout, which says nothing about which stage hung.
const runTimeout = 10 * time.Minute

// fixture is one of testdata/'s two databases together with the run this suite
// makes against it. Both fixtures go through every invariant: pagila is the
// friendly schema and nasty.sql is the hostile one, and an invariant that only
// holds on the friendly one is not an invariant.
type fixture struct {
	// name is the subtest name.
	name string

	// load puts the fixture into a freshly started source container.
	load func(context.Context, string) error

	// root and take describe the slice I1 to I5 are asserted over. They are
	// chosen to pull something worth checking: customer reaches address, city,
	// country, store, staff, rental and payment; people reaches orders,
	// order_items, attachments, tenant_users, organisations,
	// public."LegacyCustomer", invoices in another schema, and itself.
	root string
	take int

	// mustHoldRows names tables the slice is required to reach: each must
	// exist in the target holding at least one row.
	//
	// It is the property ARCHITECTURE.md §6 item 5 states and that nothing
	// else in this package checks. I1 skips a source edge whose child or
	// parent is absent from the target, which is right for --skip-table and
	// which also means a target that simply never created the child tables has
	// no missing edge to report. I6 counts the root and nothing else. I2's
	// guard is satisfied by one masked column. So an "upward only" pipeline —
	// take the root's rows, follow foreign keys to their parents so referential
	// integrity holds, and never create or load a single child table — passes
	// all six with no orders, no rentals and no payments in the snapshot.
	// This is the assertion that says subsetting happened.
	//
	// Each list carries at least two children and one grandchild, so a walk
	// that stops at depth 1 fails here rather than passing quietly.
	mustHoldRows []string

	// mustBeSubset names the tables of mustHoldRows the target must hold
	// *fewer* rows of than the source. It is a second, shorter list rather
	// than a property of every entry, because "some of its rows" and "not all
	// of its rows" are two different claims and only the first is true of
	// every table a correct slice reaches.
	//
	// nasty is the case that forces the split. Its slice is `--root
	// public.people --take 3`, which seeds {90000, 90007, 90014}; but
	// people.manager_id is an *incoming* edge of people, so the child step
	// expands it and pulls 90021 and 90028 as well (their managers are 90000
	// and 90007). All five people are selected at depth 1, and every child of
	// the whole people table then arrives whole: orders 5 of 5, order_items 7
	// of 7, billing.invoices 3 of 3. Requiring a proper subset of those would
	// fail a pipeline that did exactly what §3 says, at every value of
	// --take, and the cheapest way to make it green would be to delete this
	// guard — which is the upward-only hole it exists to close.
	//
	// public.attachments is the one nasty table a correct run does subset: row
	// 839's uploaded_by_person_id is NULL, so no person reaches it (§3's
	// MATCH SIMPLE rule: any NULL references nothing). pagila's rental and
	// payment are subset by --take 100 of 599 customers.
	mustBeSubset []string

	// extra carries the plan flags this fixture cannot run without.
	//
	// nasty.sql's public.click_stream has no primary key, no unique index and
	// two rows identical in every column, so §3.4's identity ladder ends in
	// exit 12 for it (testdata/README.md trap 12). It is a depth-1 child of
	// public.people and nothing references it, so --skip-table is §3.6's
	// answer, and the slice keeps every other trap.
	//
	// **This depends on a reading §3 contradicts, and it is a gate 4
	// blocker.** §3's pseudo-code computes identity[t] over every table in the
	// catalogue and refuses on the first nil *before* unreadable(req, priv,
	// root), which is where §3.6 applies --skip-table. Read literally, every
	// run over nasty exits 12 at plan whatever is passed here, and all six
	// invariants are unrunnable on the hostile fixture. The correction is to
	// apply req.Skipped before the ladder; reachability is not the issue,
	// because click_stream is reachable. testdata/README.md trap 12 is the
	// record of the conflict.
	extra []string

	// countedRoot and countedTake are I6's slice.
	//
	// I6 says the root holds exactly --take rows, which is only true of a root
	// that no selected row reaches as a parent. public.customer is such a root.
	// public.people is not: manager_id is a self-reference and parents are
	// pulled uncapped, so a slice of three people legitimately ends up holding
	// their managers too (testdata/README.md trap 1). I6 therefore roots the
	// nasty run at public.tenant_users, which nothing reaches as a parent: its
	// only incoming edges are from tenant_user_sessions and tenant_user_flags,
	// and those are reached only as its own children. Its own outgoing edge
	// (owner_person_id, the one that connects the component to public.people)
	// pushes people, never itself.
	countedRoot string
	countedTake int
}

var fixtures = []fixture{
	{
		name:        "pagila",
		load:        testutil.LoadPagila,
		root:        "public.customer",
		take:        100,
		countedRoot: "public.customer",
		countedTake: 100,
		// rental and payment are children of customer; payment is also a child
		// of rental, so it is the grandchild case as well. address is the
		// parent side, and the one that would still be there if the walk only
		// went upward — it is in the list so the message names which direction
		// failed.
		mustHoldRows: []string{"public.rental", "public.payment", "public.address"},
		mustBeSubset: []string{"public.rental", "public.payment"},
	},
	{
		name:        "nasty",
		load:        func(ctx context.Context, connURL string) error { return testutil.LoadNasty(ctx, connURL, false) },
		root:        "public.people",
		take:        3,
		extra:       []string{"--skip-table", "public.click_stream"},
		countedRoot: "public.tenant_users",
		countedTake: 3,
		// orders and attachments are children of people; order_items is a
		// grandchild through orders; billing.invoices is a child in the other
		// schema, so a loader that never created schema `billing` fails here
		// naming it (trap 8).
		mustHoldRows: []string{
			"public.orders",
			"public.order_items",
			"public.attachments",
			"billing.invoices",
		},
		// Only attachments: see mustBeSubset. The manager_id child edge pulls
		// every person into a --take 3 slice, so orders, order_items and
		// invoices are correctly whole in the target.
		mustBeSubset: []string{"public.attachments"},
	},
}

// ---------- the binary under test ----------

// binaryOnce builds cmd/lazyslice once per test binary. Every invariant runs
// the same build, so a test cannot accidentally assert against a stale one.
var binaryOnce = sync.OnceValues(buildBinary)

// binary returns the path of the built lazyslice, failing the test if it will
// not compile. A build failure is never a skip: a suite that skips when the
// thing it tests does not exist reports green on an empty repository.
func binary(t *testing.T) string {
	t.Helper()

	path, err := binaryOnce()
	if err != nil {
		t.Fatalf("invariants: %v", err)
	}
	return path
}

// buildDir is where the binary under test is written. It is a fresh directory
// per test binary, remembered so TestMain can remove it: a fixed path under
// os.TempDir() is shared between concurrent runs of this suite and between a
// developer and whatever else is on the machine, and nothing ever cleaned it
// up.
var buildDir string

// TestMain removes the build directory after the run. It is the only reason
// this package has a TestMain; the suite itself needs no setup.
func TestMain(m *testing.M) {
	code := m.Run()
	if buildDir != "" {
		_ = os.RemoveAll(buildDir)
	}
	os.Exit(code)
}

func buildBinary() (string, error) {
	root, err := repoRoot()
	if err != nil {
		return "", err
	}
	dir, err := os.MkdirTemp("", "lazyslice-invariants-")
	if err != nil {
		return "", fmt.Errorf("making the build directory: %w", err)
	}
	buildDir = dir
	path := filepath.Join(dir, "lazyslice")

	cmd := exec.Command("go", "build", "-o", path, "./cmd/lazyslice")
	cmd.Dir = root
	if out, buildErr := cmd.CombinedOutput(); buildErr != nil {
		return "", fmt.Errorf("building ./cmd/lazyslice: %w\n%s", buildErr, out)
	}
	return path, nil
}

// repoRoot resolves the repository root from this source file rather than from
// the working directory, which for a test is its own package directory.
func repoRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("invariants: cannot resolve the path of harness_test.go")
	}
	// <root>/internal/invariants/harness_test.go
	return filepath.Abs(filepath.Join(filepath.Dir(file), "..", ".."))
}

// ---------- running it ----------

// result is one invocation of the binary.
type result struct {
	args   []string
	exit   int
	stdout string
	stderr string
}

// String renders a failed invocation for a test message: the flags, the exit
// code and the tail of stderr, which is where the tool prints its one line.
//
// The flags carry two connection URLs and each of them carries a password, so
// they are redacted here. A test log is a file and a CI artefact like any
// other, and the fixture password being worthless is not the point: this is
// the one place in the suite where a DSN would be printed, so it is the one
// place that has to redact.
func (r result) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "lazyslice %s\n  exit %d", strings.Join(redactArgs(r.args), " "), r.exit)
	for _, s := range []struct{ name, text string }{{"stderr", r.stderr}, {"stdout", r.stdout}} {
		if trimmed := strings.TrimSpace(s.text); trimmed != "" {
			fmt.Fprintf(&b, "\n  %s: %s", s.name, tail(trimmed, 20))
		}
	}
	return b.String()
}

// redactArgs replaces the password in every argument that is a connection URL.
func redactArgs(args []string) []string {
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = a
		u, err := url.Parse(a)
		if err != nil || u.Scheme == "" || u.User == nil {
			continue
		}
		out[i] = u.Redacted()
	}
	return out
}

// tail returns the last n lines of s, indented, so a failure message carries
// the end of a transcript rather than all of it.
func tail(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n    ")
}

// runTool runs the binary with dir as its working directory, which is where it
// looks for lazyslice.yml and where it writes lazyslice.secret.
func runTool(ctx context.Context, t *testing.T, dir string, args ...string) result {
	t.Helper()

	runCtx, cancel := context.WithTimeout(ctx, runTimeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, binary(t), args...)
	cmd.Dir = dir
	cmd.Env = cleanEnv()

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run first, then read the builders: they are empty until it returns.
	runErr := cmd.Run()

	res := result{args: args, stdout: stdout.String(), stderr: stderr.String()}
	if runErr != nil {
		var exitErr *exec.ExitError
		if !errors.As(runErr, &exitErr) {
			t.Fatalf("invariants: running lazyslice: %v\n%s", runErr, res)
		}
		res.exit = exitErr.ExitCode()
	}
	return res
}

// cleanEnv is os.Environ() with every variable that could steer discovery
// **removed**, not set to the empty string.
//
// The difference is the whole point. An implementation that reads
// `os.LookupEnv("LAZYSLICE_SECRET")` and treats "present" as "use this" would
// run with an empty masking key under the old spelling, and I3's "same secret,
// same target" would then be asserting something else entirely. Removing the
// variable is the only spelling that says "the developer does not have one".
//
// The list is every rung of the discovery ladder and every libpq variable that
// can change where a connection goes or how it authenticates: PGPASSFILE,
// PGSSLMODE, PGOPTIONS and PGSERVICEFILE are as capable of redirecting a run
// as PGHOST is, and a developer's ~/.pg_service.conf is exactly the kind of
// thing that makes a suite pass on one laptop and not another.
func cleanEnv() []string {
	drop := map[string]bool{
		"DATABASE_URL": true, "POSTGRES_URL": true, "PG_URL": true, "DB_URL": true,
		"PGHOST": true, "PGPORT": true, "PGUSER": true, "PGPASSWORD": true,
		"PGDATABASE": true, "PGSERVICE": true, "PGSERVICEFILE": true,
		"PGPASSFILE": true, "PGSSLMODE": true, "PGOPTIONS": true,
	}

	env := os.Environ()
	out := make([]string, 0, len(env))
	for _, kv := range env {
		name, _, ok := strings.Cut(kv, "=")
		switch {
		case !ok:
			continue
		case drop[name]:
			continue
		case strings.HasPrefix(name, "LAZYSLICE_"):
			// Every knob the tool reads for itself, by prefix rather than by
			// list, so a variable added in phase 4 is excluded the day it
			// exists rather than the day someone remembers this file.
			continue
		}
		out = append(out, kv)
	}
	return out
}

// mustRun runs the binary and fails the test unless it exits 0.
//
// This is where the suite currently stops, on purpose: the pipeline is a no-op,
// so every invariant fails here naming the exit code and the line the scaffold
// prints. A test that skipped instead would report green on a tool that does
// nothing.
func mustRun(ctx context.Context, t *testing.T, dir string, args ...string) result {
	t.Helper()

	res := runTool(ctx, t, dir, args...)
	if res.exit != 0 {
		t.Fatalf("the run did not produce a target:\n  %s", res)
	}
	return res
}

// ---------- one snapshot ----------

// databases is a source container holding a fixture and an empty target
// container, plus the working directory a run happens in.
type databases struct {
	source string // connection URL; carries a password, so it never reaches a test name
	target string
	dir    string // working directory: lazyslice.yml and lazyslice.secret live here
}

// start brings up both containers and loads the fixture into the source.
//
// Two containers, never one database with two schemas: the target gate refuses
// a target on the same host, port and database as the source (§9 rule 1), and
// a suite that could not tell the two apart would be asserting against a
// configuration lazyslice refuses to run.
func start(ctx context.Context, t *testing.T, f fixture) *databases {
	t.Helper()

	testutil.SkipWithoutDocker(ctx, t)

	source := testutil.Postgres(ctx, t, "")
	if err := f.load(ctx, source); err != nil {
		t.Fatalf("loading the %s fixture into the source: %v", f.name, err)
	}
	target := testutil.Postgres(ctx, t, "")

	return &databases{source: source, target: target, dir: t.TempDir()}
}

// configPath and secretPath are the two files a run owns in its working
// directory. Both are passed explicitly rather than left to the defaults, so
// that a test reads as the command a person would type.
func (d *databases) configPath() string { return filepath.Join(d.dir, "lazyslice.yml") }
func (d *databases) secretPath() string { return filepath.Join(d.dir, "lazyslice.secret") }

// snapshotArgs is the run every invariant makes: this source, this target,
// this root, this many rows, no questions.
func (d *databases) snapshotArgs(f fixture, root string, take int) []string {
	args := []string{
		"--source", d.source,
		"--target", d.target,
		"--root", root,
		"--take", strconv.Itoa(take),
		"--secret-file", d.secretPath(),
		"--config", d.configPath(),
		"--yes",
	}
	return append(args, f.extra...)
}

// snapshot runs the pipeline once and returns the invocation.
//
// When the run is the fixture's own slice — the root and take I1 to I5 are
// asserted over — the tables the slice is required to reach are checked
// immediately, before any invariant looks at the target. Every one of the six
// is otherwise satisfiable by a snapshot that contains no child rows at all
// (see fixture.mustHoldRows), so this is the coverage claim and not I1's.
func (d *databases) snapshot(ctx context.Context, t *testing.T, f fixture, root string, take int) result {
	t.Helper()

	res := mustRun(ctx, t, d.dir, d.snapshotArgs(f, root, take)...)
	if root == f.root && take == f.take {
		d.assertSliceReached(ctx, t, f)
	}
	return res
}

// assertSliceReached fails unless every table in f.mustHoldRows is in the
// target holding some of its rows, and every table in f.mustBeSubset holds
// fewer of them than the source does.
//
// Two claims, kept apart because only the first is true of every table a
// correct slice reaches. "Some of its rows" has two failures and they mean
// different things. Absent: the loader never created the table, which is the
// "upward only" pipeline — it satisfies I1 (no edge between two present tables
// is missing), I6 (the root is exactly --take) and I2 (one masked column
// arrived), and it contains none of the data anybody wanted. Zero rows: the
// table was created and the walk never reached it, which is
// research/COMPLAINTS.md FK-10's silently empty slice.
//
// "Not all of its rows" is the opposite failure — nothing was subset, the cap,
// the take and the depth did nothing — and it is asserted only over
// f.mustBeSubset, because a table whose every row a correct slice legitimately
// reaches (nasty's orders, order_items and billing.invoices: see mustBeSubset)
// would otherwise fail on a pipeline that is right.
func (d *databases) assertSliceReached(ctx context.Context, t *testing.T, f fixture) {
	t.Helper()

	if len(f.mustHoldRows) == 0 {
		t.Fatalf("the %s fixture names no table the slice must reach, so every invariant in this "+
			"package is satisfied by a target holding the root and nothing else", f.name)
	}
	if len(f.mustBeSubset) == 0 {
		t.Fatalf("the %s fixture names no table the slice must hold *fewer* rows of than the source, "+
			"so a run that copied every table whole passes this guard; name at least one table a "+
			"correct slice does not reach entirely", f.name)
	}
	mustBeSubset := map[string]bool{}
	for _, name := range f.mustBeSubset {
		mustBeSubset[name] = true
	}

	source := connect(ctx, t, d.source)
	target := connect(ctx, t, d.target)
	present := tableSet(ctx, t, target)

	for _, name := range f.mustHoldRows {
		ref := parseTable(t, name)
		if !present[ref] {
			t.Errorf("the slice: %s is not in the target at all, so the snapshot holds no %s row; "+
				"§6 item 5 says every step with mode ChildOK or ParentOnly has exactly Keys.Len() rows "+
				"in the target, and a table nobody created has none of them",
				ref, ref.Name)
			continue
		}
		want := countRows(ctx, t, source, ref)
		if want == 0 {
			t.Errorf("the slice: %s holds no rows in the %s source, so requiring the target to hold "+
				"some of them proves nothing; fix the fixture or drop it from mustHoldRows", ref, f.name)
			continue
		}
		got := countRows(ctx, t, target, ref)
		if got == 0 {
			t.Errorf("the slice: %s holds %d row(s) in the %s source and none in the target, so the walk "+
				"never reached it (research/COMPLAINTS.md FK-10: the silently empty slice)", ref, want, f.name)
			continue
		}
		if mustBeSubset[name] && got >= want {
			t.Errorf("the slice: %s holds %d row(s) in the %s source and %d in the target, so nothing was "+
				"subset: --take %d, --cap and --depth changed nothing for this table. It is in "+
				"mustBeSubset because a correct run cannot reach all of its rows",
				ref, want, f.name, got, f.take)
		}
	}

	// mustBeSubset is a list of names; a typo in it asserts nothing and says
	// nothing, which is the shape this guard exists to refuse.
	for _, name := range f.mustBeSubset {
		if !slices.Contains(f.mustHoldRows, name) {
			t.Errorf("the %s fixture's mustBeSubset names %s, which is not in mustHoldRows, so nothing "+
				"counted its rows", f.name, name)
		}
	}
}

// mutateTarget deletes a handful of rows from a loaded table of the target and
// returns the table it touched.
//
// It is what makes I3's and I5's second run provable. Both compare the target
// after a second invocation against the state the first invocation left in it,
// so an invocation that exited 0 without truncating and reloading — one that
// found its own bound marker and short-circuited, say — produces a
// byte-identical dump: the strongest possible pass for the weakest possible
// pipeline. assertSecondRunHappened reads lazyslice_meta, which is a second
// signal but still the implementation's own account of itself (§11.2 says the
// marker row is inserted before the first drop, so the guard is trusting the
// thing under test). This is not: the rows are gone, and only a run that
// actually truncated and reloaded can put them back.
//
// The table comes from mustHoldRows, so it is one the slice is required to
// have reached, and the deletion is of the whole table rather than of some
// rows, because a foreign key from a table not yet dropped would refuse a
// partial delete. It runs after the first dump is taken.
func (d *databases) mutateTarget(ctx context.Context, t *testing.T, f fixture, invariant string) tableRef {
	t.Helper()

	conn := connect(ctx, t, d.target)
	for _, name := range f.mustHoldRows {
		ref := parseTable(t, name)
		if countRows(ctx, t, conn, ref) == 0 {
			continue
		}
		if _, err := conn.Exec(ctx, `DELETE FROM `+ref.quoted()); err != nil {
			// A foreign key pointing at this table refuses the delete. That is
			// not a failure of the pipeline, so try the next candidate.
			continue
		}
		if n := countRows(ctx, t, conn, ref); n != 0 {
			t.Fatalf("%s: emptying %s left %d row(s)", invariant, ref, n)
		}
		return ref
	}
	t.Fatalf("%s: no table in the %s fixture's mustHoldRows could be emptied, so this test cannot tell "+
		"a second run that reloaded the target from one that exited 0 and did nothing", invariant, f.name)
	return tableRef{}
}

// ---------- talking to the databases ----------

// connect opens one connection and closes it when the test finishes.
func connect(ctx context.Context, t *testing.T, connURL string) *pgx.Conn {
	t.Helper()

	conn, err := pgx.Connect(ctx, connURL)
	if err != nil {
		t.Fatalf("invariants: connecting: %v", err)
	}
	t.Cleanup(func() {
		_ = conn.Close(context.WithoutCancel(ctx))
	})
	return conn
}

// tableRef is a schema-qualified relation. It is a struct rather than a string
// because every message in this package names a table and a column, and
// splitting "public.customer" back apart at the point of the message is how a
// quoted, mixed-case identifier (testdata/README.md trap 9) gets mangled.
type tableRef struct{ Schema, Name string }

func (t tableRef) String() string { return t.Schema + "." + t.Name }

// quoted renders the reference as SQL, quoting both halves, because
// public."LegacyCustomer" exists only when quoted.
func (t tableRef) quoted() string { return pgx.Identifier{t.Schema, t.Name}.Sanitize() }

// parseTable splits a fixture's "schema.name" into a reference. The fixtures'
// roots are all lower case and unquoted, so this is a split, not a parser.
func parseTable(t *testing.T, s string) tableRef {
	t.Helper()

	schema, name, ok := strings.Cut(s, ".")
	if !ok || schema == "" || name == "" {
		t.Fatalf("invariants: %q is not a schema-qualified table name", s)
	}
	return tableRef{Schema: schema, Name: name}
}

// columnRef is one column of one relation, in catalogue spelling: the
// identifiers as pg_attribute holds them, never quoted.
//
// Everything this package compares a column by goes through it — the emitted
// yml's `columns:` keys, the `--json` stream's table and column fields, and
// the cells read out of both databases — because those three channels spell an
// identifier three ways. §10's examples are all unquoted lower case; a
// statement lazyslice generates has to write public."LegacyCustomer" quoted
// (testdata/README.md trap 9); and the catalogue holds LegacyCustomer bare.
// Comparing the raw strings drops the quoted columns from every assertion in
// I2 without saying so.
type columnRef struct {
	Table  tableRef
	Column string
}

func (c columnRef) String() string { return c.Table.String() + "." + c.Column }

// parseColumnRef reads a possibly-quoted, dot-separated `schema.table.column`.
func parseColumnRef(s string) (columnRef, bool) {
	parts, ok := splitQualifiedName(s)
	if !ok || len(parts) != 3 {
		return columnRef{}, false
	}
	for _, p := range parts {
		if p == "" {
			return columnRef{}, false
		}
	}
	return columnRef{Table: tableRef{Schema: parts[0], Name: parts[1]}, Column: parts[2]}, true
}

// splitQualifiedName splits a dotted identifier into its parts, unquoting each
// one, and reports whether the quoting was well formed.
//
// A dot inside double quotes is part of the identifier, not a separator:
// public."a.b".c is three parts, not four. `""` inside a quoted part is one
// double quote, which is SQL's own escape.
func splitQualifiedName(s string) ([]string, bool) {
	var parts []string
	var cur strings.Builder
	quoted := false
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '"' && quoted && i+1 < len(s) && s[i+1] == '"':
			cur.WriteByte('"')
			i++
		case c == '"':
			quoted = !quoted
		case c == '.' && !quoted:
			parts = append(parts, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	if quoted {
		return nil, false
	}
	return append(parts, cur.String()), true
}

// partitionRoots maps every partition leaf to the partitioned table at the top
// of its inheritance chain.
//
// §3.3: "A partition is never a step; its root is", and "the target holds the
// root as a plain table". So the source's rows live in public.events_2024 and
// public.events_2025 while the run's yml and the target both say public.events.
// Without this map every comparison I2 makes over a partitioned table's
// columns silently matches nothing and reports clean by omission.
//
// Only declarative partitioning is collapsed (the top of the chain must be
// relkind 'p'), because that is what §3.3 is about; old-style inheritance is a
// different question and testdata/ has none.
func partitionRoots(ctx context.Context, t *testing.T, conn *pgx.Conn) map[tableRef]tableRef {
	t.Helper()

	rows, err := conn.Query(ctx, `
		WITH RECURSIVE up(leaf, ancestor) AS (
			SELECT i.inhrelid, i.inhparent FROM pg_inherits i
			UNION ALL
			SELECT u.leaf, i.inhparent FROM up u JOIN pg_inherits i ON i.inhrelid = u.ancestor
		)
		SELECT ln.nspname, lc.relname, rn.nspname, rc.relname
		FROM up
		JOIN pg_class lc ON lc.oid = up.leaf
		JOIN pg_namespace ln ON ln.oid = lc.relnamespace
		JOIN pg_class rc ON rc.oid = up.ancestor
		JOIN pg_namespace rn ON rn.oid = rc.relnamespace
		WHERE rc.relkind = 'p'
		  AND NOT EXISTS (SELECT 1 FROM pg_inherits i2 WHERE i2.inhrelid = up.ancestor)`)
	if err != nil {
		t.Fatalf("invariants: listing partitions: %v", err)
	}
	defer rows.Close()

	out := map[tableRef]tableRef{}
	for rows.Next() {
		var leaf, root tableRef
		if scanErr := rows.Scan(&leaf.Schema, &leaf.Name, &root.Schema, &root.Name); scanErr != nil {
			t.Fatalf("invariants: listing partitions: %v", scanErr)
		}
		out[leaf] = root
	}
	if rows.Err() != nil {
		t.Fatalf("invariants: listing partitions: %v", rows.Err())
	}
	return out
}

// allColumns is every column of every ordinary or partitioned relation, in
// catalogue spelling.
//
// Partitioned roots are included ('p'), because §3.3 makes the root the step
// and a leaf's columns are the root's. It is what says whether a column name
// in the emitted yml names anything at all.
func allColumns(ctx context.Context, t *testing.T, conn *pgx.Conn) map[columnRef]bool {
	t.Helper()

	rows, err := conn.Query(ctx, `
		SELECT n.nspname, c.relname, a.attname
		FROM pg_attribute a
		JOIN pg_class c ON c.oid = a.attrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind IN ('r', 'p')
		  AND a.attnum > 0 AND NOT a.attisdropped
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema')
		  AND n.nspname NOT LIKE 'pg_toast%'
		  AND n.nspname NOT LIKE 'pg_temp%'`)
	if err != nil {
		t.Fatalf("invariants: listing columns: %v", err)
	}
	defer rows.Close()

	out := map[columnRef]bool{}
	for rows.Next() {
		var ref columnRef
		if scanErr := rows.Scan(&ref.Table.Schema, &ref.Table.Name, &ref.Column); scanErr != nil {
			t.Fatalf("invariants: listing columns: %v", scanErr)
		}
		out[ref] = true
	}
	if rows.Err() != nil {
		t.Fatalf("invariants: listing columns: %v", rows.Err())
	}
	return out
}

// admissibleDomains is the number of distinct values each column of small,
// countable type can hold, for the columns where the catalogue knows it:
// enums (their label count), booleans (2), and `varchar(n)`/`char(n)` at n ≤ 2.
//
// It is §5's `d = min(column domain, generator.Domain())` from the side this
// suite can see. Every other type is absent from the map rather than given a
// large number, because a guess would be a claim about a generator's Domain()
// that phase 4 has not written yet.
//
// Domains (typtype 'd') resolve to their base type, which is how pagila's
// public.year and public."bıgınt" reach the right answer.
func admissibleDomains(ctx context.Context, t *testing.T, conn *pgx.Conn) map[columnRef]int64 {
	t.Helper()

	rows, err := conn.Query(ctx, `
		SELECT n.nspname, c.relname, a.attname,
		       CASE
		         WHEN bt.typtype = 'e'
		           THEN (SELECT count(*) FROM pg_enum e WHERE e.enumtypid = bt.oid)
		         WHEN bt.typname = 'bool' THEN 2
		         WHEN bt.typname IN ('varchar', 'bpchar')
		              AND a.atttypmod > 4 AND a.atttypmod - 4 <= 2
		           THEN power(95, a.atttypmod - 4)::bigint
		       END AS domain
		FROM pg_attribute a
		JOIN pg_class c ON c.oid = a.attrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		JOIN pg_type ty ON ty.oid = a.atttypid
		JOIN pg_type bt ON bt.oid = CASE WHEN ty.typtype = 'd' THEN ty.typbasetype ELSE ty.oid END
		WHERE c.relkind IN ('r', 'p')
		  AND a.attnum > 0 AND NOT a.attisdropped
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema')
		  AND n.nspname NOT LIKE 'pg_toast%'
		  AND n.nspname NOT LIKE 'pg_temp%'`)
	if err != nil {
		t.Fatalf("invariants: reading column domains: %v", err)
	}
	defer rows.Close()

	out := map[columnRef]int64{}
	for rows.Next() {
		var ref columnRef
		var domain *int64
		if scanErr := rows.Scan(&ref.Table.Schema, &ref.Table.Name, &ref.Column, &domain); scanErr != nil {
			t.Fatalf("invariants: reading column domains: %v", scanErr)
		}
		if domain != nil {
			out[ref] = *domain
		}
	}
	if rows.Err() != nil {
		t.Fatalf("invariants: reading column domains: %v", rows.Err())
	}
	return out
}

// dataTables lists every ordinary table holding rows of its own: relkind 'r',
// outside the system schemas. Partitioned roots (relkind 'p') are left out
// because their rows are their leaves' rows and would be counted twice.
func dataTables(ctx context.Context, t *testing.T, conn *pgx.Conn) []tableRef {
	t.Helper()

	rows, err := conn.Query(ctx, `
		SELECT n.nspname, c.relname
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind = 'r'
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema')
		  AND n.nspname NOT LIKE 'pg_toast%'
		  AND n.nspname NOT LIKE 'pg_temp%'
		ORDER BY n.nspname, c.relname`)
	if err != nil {
		t.Fatalf("invariants: listing tables: %v", err)
	}
	defer rows.Close()

	var out []tableRef
	for rows.Next() {
		var ref tableRef
		if scanErr := rows.Scan(&ref.Schema, &ref.Name); scanErr != nil {
			t.Fatalf("invariants: listing tables: %v", scanErr)
		}
		out = append(out, ref)
	}
	if rows.Err() != nil {
		t.Fatalf("invariants: listing tables: %v", rows.Err())
	}
	return out
}

// columnsOf lists the columns of one table in attribute order, dropped columns
// and system columns excluded.
func columnsOf(ctx context.Context, t *testing.T, conn *pgx.Conn, ref tableRef) []string {
	t.Helper()

	rows, err := conn.Query(ctx, `
		SELECT a.attname
		FROM pg_attribute a
		JOIN pg_class c ON c.oid = a.attrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1 AND c.relname = $2 AND a.attnum > 0 AND NOT a.attisdropped
		ORDER BY a.attnum`, ref.Schema, ref.Name)
	if err != nil {
		t.Fatalf("invariants: listing the columns of %s: %v", ref, err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var name string
		if scanErr := rows.Scan(&name); scanErr != nil {
			t.Fatalf("invariants: listing the columns of %s: %v", ref, scanErr)
		}
		out = append(out, name)
	}
	if rows.Err() != nil {
		t.Fatalf("invariants: listing the columns of %s: %v", ref, rows.Err())
	}
	return out
}

// countRows counts one table.
func countRows(ctx context.Context, t *testing.T, conn *pgx.Conn, ref tableRef) int64 {
	t.Helper()

	var n int64
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM `+ref.quoted()).Scan(&n); err != nil {
		t.Fatalf("invariants: counting %s: %v", ref, err)
	}
	return n
}

// ---------- the marker table ----------

// markerRunIDs reads the run_id of every row in the target's lazyslice_meta
// (§11.2).
//
// It is how I3 and I5 tell a second run from a second invocation that did
// nothing. Both compare the target against the state the first run left in it,
// so an invocation that exited 0 without truncating or reloading produces an
// identical dump — the strongest possible pass, for the weakest possible
// pipeline. §11.2 says every run inserts its row with status = running before
// its first drop, so a run that touched the target left a run_id behind.
//
// lazyslice_meta is deliberately excluded from the dumps (markerPattern):
// comparing it would make I3 and I5 assert that two runs happened at the same
// instant. This reads it directly instead, which is the opposite assertion.
func markerRunIDs(ctx context.Context, t *testing.T, conn *pgx.Conn, invariant string) []string {
	t.Helper()

	var schema string
	err := conn.QueryRow(ctx, `
		SELECT n.nspname
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relname = 'lazyslice_meta'
		  AND c.relkind = 'r'
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema')
		ORDER BY n.nspname
		LIMIT 1`).Scan(&schema)
	if err != nil {
		t.Fatalf("%s: the target has no lazyslice_meta table, so this test cannot tell a second run from "+
			"an invocation that exited 0 and did nothing; ARCHITECTURE.md §11.2 says every run inserts a "+
			"row there before its first write: %v", invariant, err)
	}

	ref := tableRef{Schema: schema, Name: "lazyslice_meta"}
	rows, err := conn.Query(ctx, `SELECT run_id::text FROM `+ref.quoted()+` ORDER BY run_id`)
	if err != nil {
		t.Fatalf("%s: reading %s: %v", invariant, ref, err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var id string
		if scanErr := rows.Scan(&id); scanErr != nil {
			t.Fatalf("%s: reading %s: %v", invariant, ref, scanErr)
		}
		out = append(out, id)
	}
	if rows.Err() != nil {
		t.Fatalf("%s: reading %s: %v", invariant, ref, rows.Err())
	}
	return out
}

// assertSecondRunHappened fails unless lazyslice_meta gained a run_id that was
// not there before the second invocation.
//
// A set difference rather than a count, because §11.2 does not say the table
// keeps every row for ever: a run that replaced the marker row still writes a
// new run_id, and one that wrote nothing cannot.
func assertSecondRunHappened(t *testing.T, invariant string, before, after []string) {
	t.Helper()

	seen := make(map[string]bool, len(before))
	for _, id := range before {
		seen[id] = true
	}
	for _, id := range after {
		if !seen[id] {
			return
		}
	}
	t.Fatalf("%s: lazyslice_meta records no run that was not there before the second invocation "+
		"(%d row(s) before, %d after), so the second run wrote nothing to the target and comparing "+
		"the target with itself proves nothing", invariant, len(before), len(after))
}
