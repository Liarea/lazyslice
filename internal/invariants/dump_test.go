//go:build integration

package invariants

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Liarea/lazyslice/internal/testutil"
)

// dumpTimeout bounds pg_dump. The target is small by construction (§6 item 4),
// so anything slower than this is a hang, not a big database.
const dumpTimeout = runTimeout

// dockerTimeout bounds the two docker CLI calls this file makes to find the
// container a connection URL points at. They are local and metadata-only.
const dockerTimeout = 30 * time.Second

// markerPattern excludes the marker table from a data dump.
//
// lazyslice_meta (§11.2) records run_id, started_at, finished_at and
// rows_loaded, so two runs that produce identical data still write different
// marker rows. Comparing it would make I3 and I5 assert that two runs happened
// at the same instant, which is not what "byte-identical target" means. I3 and
// I5 read the table directly instead, to prove the second run happened at all.
const markerPattern = "*.lazyslice_meta"

// containerPort is the port Postgres listens on inside a testutil container,
// which is not the port the connection URL names: testutil publishes 5432 on
// an ephemeral host port.
const containerPort = "5432"

// dump is one `pg_dump --data-only`, normalised for comparison and counted.
//
// The counts are what stops I3 and I5 passing vacuously. `pg_dump --data-only`
// of a database with no rows is a few hundred bytes of SET boilerplate and is
// byte-identical between runs, so a pipeline that loaded nothing would satisfy
// both invariants exactly as well as a correct one.
type dump struct {
	text   string // normalised; what the comparison reads
	tables int    // COPY blocks
	rows   int    // data rows inside them
}

// dumpData returns `pg_dump --data-only` for a database, normalised, which is
// what I3 and I5 compare.
//
// pg_dump rather than a query of our own on purpose: the comparison has to be
// able to fail on something the suite did not think to select. A hand-written
// "SELECT every column of every table" reproduces the tool's own idea of what
// a row is, and would agree with a load that dropped a column both times.
func dumpData(ctx context.Context, t *testing.T, connURL string) dump {
	t.Helper()

	name, args := pgDumpCommand(ctx, t, connURL)

	dumpCtx, cancel := context.WithTimeout(ctx, dumpTimeout)
	defer cancel()

	cmd := exec.CommandContext(dumpCtx, name, args...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if runErr := cmd.Run(); runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			t.Fatalf("invariants: %s exited %d: %s", name, exitErr.ExitCode(), strings.TrimSpace(stderr.String()))
		}
		t.Fatalf("invariants: running %s: %v", name, runErr)
	}

	d, err := normaliseDump(stdout.String())
	if err != nil {
		t.Fatalf("invariants: reading the dump of the target: %v", err)
	}
	return d
}

// dumpArgs is the pg_dump invocation, whichever binary runs it.
func dumpArgs(connURL string) []string {
	return []string{
		"--data-only",
		"--no-owner",
		"--no-acl",
		"--exclude-table=" + markerPattern,
		connURL,
	}
}

// pgDumpCommand chooses how to run pg_dump against connURL.
//
// First choice is the server's own binary, through `docker exec` into the
// container testutil started. pg_dump refuses to dump a server newer than
// itself — a homebrew pg_dump 14 against the default `postgres:16` container
// aborts with "server version: 16.15; pg_dump version: 14.18" — and CI runs
// this matrix on postgres:14 and postgres:18 with whatever client the runner
// image happens to carry, which is neither. Running the container's own
// pg_dump makes the client major equal the server major by construction, on
// every leg of the matrix and on every laptop.
//
// When the container cannot be found — no docker CLI on PATH, a remote or
// non-Docker provider, an ambiguous port — it falls back to a pg_dump on PATH
// and compares the majors first, so the failure names both numbers and the
// remedy rather than pg_dump's own version-mismatch abort under a message
// telling the reader to install a client they already have.
func pgDumpCommand(ctx context.Context, t *testing.T, connURL string) (string, []string) {
	t.Helper()

	id, inside, findErr := dockerContainerFor(ctx, connURL)
	if findErr == nil {
		return "docker", append([]string{"exec", id, "pg_dump"}, dumpArgs(inside)...)
	}

	path, err := exec.LookPath("pg_dump")
	if err != nil {
		// Not a skip. Skipping here would report green on the two invariants
		// that are the whole reason determinism is claimed, on any machine
		// without the client tools. SkipWithoutDocker is the only skip in this
		// package.
		t.Fatalf("invariants: I3 and I5 compare two targets with pg_dump. The container's own pg_dump "+
			"could not be used (%v), and there is none on PATH (%v). Either make the test container "+
			"reachable through the docker CLI, or install the PostgreSQL client tools "+
			"(postgresql-client / libpq) for the server's major.", findErr, err)
	}
	assertPgDumpMajorMatches(ctx, t, path, connURL, findErr)
	return path, dumpArgs(connURL)
}

// dockerContainerFor finds the running container that publishes connURL's port
// and returns its id together with a URL that reaches Postgres from inside it.
func dockerContainerFor(ctx context.Context, connURL string) (id, insideURL string, err error) {
	docker, err := exec.LookPath("docker")
	if err != nil {
		return "", "", fmt.Errorf("the docker CLI is not on PATH: %w", err)
	}
	u, err := testutil.URL(connURL)
	if err != nil {
		return "", "", err
	}
	port := u.Port()
	if port == "" {
		return "", "", errors.New("the connection URL names no port")
	}

	listCtx, cancel := context.WithTimeout(ctx, dockerTimeout)
	defer cancel()

	out, err := exec.CommandContext(listCtx, docker, "ps", "--format", "{{.ID}} {{.Ports}}").Output()
	if err != nil {
		return "", "", fmt.Errorf("listing running containers: %w", err)
	}

	// "a939fc7bd9f4 0.0.0.0:55171->5432/tcp, [::]:55171->5432/tcp"
	want := ":" + port + "->" + containerPort + "/tcp"
	var found []string
	for _, line := range strings.Split(string(out), "\n") {
		candidate, ports, ok := strings.Cut(strings.TrimSpace(line), " ")
		if !ok || !strings.Contains(ports, want) {
			continue
		}
		found = append(found, candidate)
	}
	if len(found) != 1 {
		return "", "", fmt.Errorf("%d running container(s) publish %s->%s/tcp, want exactly one",
			len(found), port, containerPort)
	}

	inside := *u
	inside.Host = "127.0.0.1:" + containerPort
	return found[0], inside.String(), nil
}

// pgDumpVersion reads the major out of `pg_dump --version`, whose first line is
// "pg_dump (PostgreSQL) 16.10 (Debian 16.10-1.pgdg13+2)".
var pgDumpVersion = regexp.MustCompile(`\)\s+(\d+)`)

// assertPgDumpMajorMatches fails, naming both majors, when the pg_dump on PATH
// is older than the server it is about to be pointed at.
func assertPgDumpMajorMatches(ctx context.Context, t *testing.T, path, connURL string, why error) {
	t.Helper()

	versionCtx, cancel := context.WithTimeout(ctx, dockerTimeout)
	defer cancel()

	out, err := exec.CommandContext(versionCtx, path, "--version").Output()
	if err != nil {
		t.Fatalf("invariants: running `pg_dump --version`: %v", err)
	}
	m := pgDumpVersion.FindStringSubmatch(string(out))
	if m == nil {
		t.Fatalf("invariants: cannot read a major version out of `pg_dump --version`: %s",
			strings.TrimSpace(string(out)))
	}
	client, err := strconv.Atoi(m[1])
	if err != nil {
		t.Fatalf("invariants: cannot read a major version out of `pg_dump --version`: %v", err)
	}

	server := serverMajor(ctx, t, connURL)
	if client >= server {
		return
	}
	t.Fatalf("invariants: the pg_dump on PATH is major %d and the server is major %d, and pg_dump "+
		"refuses to dump a server newer than itself, so I3 and I5 cannot run. The suite would have used "+
		"the container's own pg_dump and could not find it (%v). Install postgresql-client-%d, or make "+
		"the test container reachable through the docker CLI.", client, server, why, server)
}

// serverMajor asks the server its own major, so the message above names a
// number nobody has to guess at.
func serverMajor(ctx context.Context, t *testing.T, connURL string) int {
	t.Helper()

	var num int
	if err := connect(ctx, t, connURL).QueryRow(ctx, `SELECT current_setting('server_version_num')::int`).Scan(&num); err != nil {
		t.Fatalf("invariants: reading the server version: %v", err)
	}
	return num / 10000
}

// ---------- normalising ----------

var (
	// restrictLine matches the per-session token pg_dump has emitted since the
	// August 2025 security releases (14.19, 15.14, 16.10, 17.6 and later).
	restrictLine = regexp.MustCompile(`^\\(un)?restrict\s`)

	// bannerLine matches the two version banners near the top of every dump.
	bannerLine = regexp.MustCompile(`^-- Dumped (by|from) `)
)

const (
	removedToken  = `<session token removed by the invariant suite>`
	removedBanner = `-- <version banner removed by the invariant suite>`
)

// normaliseDump replaces the parts of a dump that differ between two dumps of
// the same, untouched database, and counts the data that is left.
//
// pg_dump writes `\restrict <token>` near the top of every dump and a matching
// `\unrestrict <token>` at the end, where the token is generated per session.
// Two dumps of one database therefore differ at line 5, before any COPY header
// has been seen, so I3 and I5 would fail permanently naming no table, no
// column and nothing to do with determinism. The version banners are
// normalised for the same reason, defensively.
//
// Lines are replaced rather than deleted, so a line number in a failure
// message is the line number of the dump a reader can produce themselves.
//
// The self-check is the default branch: outside a COPY block, any other line
// starting with a backslash is a psql meta-command this suite has never seen,
// and the next one pg_dump adds may carry a per-session token too. That is a
// hard failure naming the line, so a future spelling is loud rather than a
// permanent, mystifying byte difference.
func normaliseDump(text string) (dump, error) {
	lines := strings.Split(text, "\n")

	var d dump
	inCopy := false
	for i, line := range lines {
		if inCopy {
			if line == `\.` {
				inCopy = false
				continue
			}
			d.rows++
			continue
		}
		switch {
		case restrictLine.MatchString(line):
			lines[i] = restrictLine.FindString(line) + removedToken
		case bannerLine.MatchString(line):
			lines[i] = removedBanner
		case isCopyHeader(line):
			d.tables++
			inCopy = true
		case strings.HasPrefix(line, `\`):
			return dump{}, fmt.Errorf("line %d is a pg_dump meta-command this suite does not recognise, "+
				"and an unrecognised one may carry a per-session token that differs between two dumps of "+
				"one database: %s", i+1, truncate(line))
		}
	}
	if inCopy {
		return dump{}, errors.New("the dump ends inside a COPY block")
	}

	d.text = strings.Join(lines, "\n")
	return d, nil
}

// assertDumpHasData fails when a dump carries no rows, because two such dumps
// are byte-identical whatever the pipeline did.
func assertDumpHasData(t *testing.T, invariant, what string, d dump) {
	t.Helper()

	if d.tables > 0 && d.rows > 0 {
		return
	}
	t.Fatalf("%s: the dump of %s holds %d COPY block(s) and %d data row(s), so two dumps of it are "+
		"byte-identical whatever the run did; a pipeline that loaded no rows would satisfy this "+
		"invariant exactly as well as a correct one", invariant, what, d.tables, d.rows)
}

// ---------- comparing ----------

// diffDumps compares two data dumps and returns "" when they are identical, or
// a message naming the table the first difference is in and the two lines.
//
// The table comes from the enclosing `COPY <table> (cols) FROM stdin;` header,
// which is how a byte difference is turned into the table-and-column message
// this suite promises. The column is named when the two rows differ in exactly
// one field, which is the common case for a masking key that changed.
func diffDumps(a, b dump) string {
	left, right := strings.Split(a.text, "\n"), strings.Split(b.text, "\n")

	var table, columns string
	for i := 0; i < len(left) && i < len(right); i++ {
		if header, cols, ok := copyHeader(left[i]); ok {
			table, columns = header, cols
		}
		if left[i] == right[i] {
			continue
		}
		return describeDiff(i+1, table, columns, left[i], right[i])
	}
	if len(left) != len(right) {
		return dumpLengthDiff(table, len(left), len(right))
	}
	return ""
}

// dumpLengthDiff reports two dumps that agree line for line until one ends.
func dumpLengthDiff(table string, left, right int) string {
	where := "the two dumps"
	if table != "" {
		where = "the dump, last inside " + table + ","
	}
	return where + " are different lengths: " +
		strconv.Itoa(left) + " lines against " + strconv.Itoa(right) + " lines"
}

// describeDiff renders one differing line, naming the table and, when the
// difference is in a single field of a COPY row, the column.
func describeDiff(line int, table, columns, left, right string) string {
	where := "line " + strconv.Itoa(line)
	if table != "" {
		where += " of " + table
	}
	if col := differingColumn(columns, left, right); col != "" {
		where += ", column " + col
	}
	return where + ":\n  first  run: " + truncate(left) + "\n  second run: " + truncate(right)
}

// copyHeader recognises pg_dump's data header and returns the table name and
// the parenthesised column list.
func copyHeader(line string) (table, columns string, ok bool) {
	if !strings.HasPrefix(line, "COPY ") || !strings.HasSuffix(line, "FROM stdin;") {
		return "", "", false
	}
	rest := strings.TrimPrefix(line, "COPY ")
	open := strings.Index(rest, "(")
	closing := strings.LastIndex(rest, ")")
	if open < 0 || closing < open {
		return strings.TrimSpace(rest), "", true
	}
	return strings.TrimSpace(rest[:open]), rest[open+1 : closing], true
}

// isCopyHeader is copyHeader with the names thrown away.
func isCopyHeader(line string) bool {
	_, _, ok := copyHeader(line)
	return ok
}

// differingColumn names the column when two COPY rows differ in exactly one
// tab-separated field and the header's column list lines up with them.
func differingColumn(columns, left, right string) string {
	if columns == "" {
		return ""
	}
	names := strings.Split(columns, ",")
	lf, rf := strings.Split(left, "\t"), strings.Split(right, "\t")
	if len(lf) != len(rf) || len(lf) != len(names) {
		return ""
	}
	found := ""
	for i := range lf {
		if lf[i] == rf[i] {
			continue
		}
		if found != "" {
			return "" // more than one field differs; naming one would mislead
		}
		found = strings.TrimSpace(names[i])
	}
	return found
}

// truncate keeps a differing line readable in a test message. A dumped row is
// source data, so this is the one place the suite prints one: it prints only
// on a failure, and only from a database the test itself created.
func truncate(s string) string {
	const limit = 160
	if len(s) > limit {
		return s[:limit] + " ..."
	}
	return s
}
