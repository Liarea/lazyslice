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
	// order_items, attachments, invoices in another schema, and itself.
	root string
	take int

	// extra carries the plan flags this fixture cannot run without.
	//
	// nasty.sql's public.click_stream has no primary key, no unique index and
	// two rows identical in every column, so §3.4's identity ladder ends in
	// exit 12 for it (testdata/README.md trap 12). It is child-only, so
	// --skip-table is §3.6's answer, and the slice keeps every other trap.
	extra []string

	// countedRoot and countedTake are I6's slice.
	//
	// I6 says the root holds exactly --take rows, which is only true of a root
	// that no selected row reaches as a parent. public.customer is such a root.
	// public.people is not: manager_id is a self-reference and parents are
	// pulled uncapped, so a slice of three people legitimately ends up holding
	// their managers too (testdata/README.md trap 1). I6 therefore roots the
	// nasty run at public.tenant_users, which has no outgoing foreign key and
	// is referenced only by its children.
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
	},
	{
		name:        "nasty",
		load:        func(ctx context.Context, connURL string) error { return testutil.LoadNasty(ctx, connURL, false) },
		root:        "public.people",
		take:        3,
		extra:       []string{"--skip-table", "public.click_stream"},
		countedRoot: "public.tenant_users",
		countedTake: 3,
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

func buildBinary() (string, error) {
	root, err := repoRoot()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(os.TempDir(), "lazyslice-invariants")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("making the build directory: %w", err)
	}
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
	// A clean environment except for what a compiler and a Docker client need:
	// the developer's own DATABASE_URL, PGHOST or LAZYSLICE_SECRET would be
	// found by the discovery ladder and quietly change what is being tested.
	cmd.Env = append(os.Environ(),
		"DATABASE_URL=", "POSTGRES_URL=", "PG_URL=", "DB_URL=",
		"PGHOST=", "PGPORT=", "PGUSER=", "PGPASSWORD=", "PGDATABASE=", "PGSERVICE=",
		"LAZYSLICE_SECRET=",
	)

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
func (d *databases) snapshot(ctx context.Context, t *testing.T, f fixture, root string, take int) result {
	t.Helper()
	return mustRun(ctx, t, d.dir, d.snapshotArgs(f, root, take)...)
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
