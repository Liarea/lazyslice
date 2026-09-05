//go:build integration

package invariants

import (
	"context"
	"errors"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// dumpTimeout bounds pg_dump. The target is small by construction (§6 item 4),
// so anything slower than this is a hang, not a big database.
const dumpTimeout = runTimeout

// markerPattern excludes the marker table from a data dump.
//
// lazyslice_meta (§11.2) records run_id, started_at, finished_at and
// rows_loaded, so two runs that produce identical data still write different
// marker rows. Comparing it would make I3 and I5 assert that two runs happened
// at the same instant, which is not what "byte-identical target" means.
const markerPattern = "*.lazyslice_meta"

// dumpData returns `pg_dump --data-only` for a database, which is what I3 and
// I5 compare.
//
// pg_dump rather than a query of our own on purpose: the comparison has to be
// able to fail on something the suite did not think to select. A hand-written
// "SELECT every column of every table" reproduces the tool's own idea of what
// a row is, and would agree with a load that dropped a column both times.
func dumpData(ctx context.Context, t *testing.T, connURL string) string {
	t.Helper()

	path, err := exec.LookPath("pg_dump")
	if err != nil {
		// Not a skip. Skipping here would report green on the two invariants
		// that are the whole reason determinism is claimed, on any machine
		// without the client tools. SkipWithoutDocker is the only skip in this
		// package.
		t.Fatalf("invariants: pg_dump is not on PATH, and I3 and I5 compare two targets with it; "+
			"install the PostgreSQL client tools (postgresql-client / libpq): %v", err)
	}

	dumpCtx, cancel := context.WithTimeout(ctx, dumpTimeout)
	defer cancel()

	cmd := exec.CommandContext(dumpCtx, path,
		"--data-only",
		"--no-owner",
		"--no-acl",
		"--exclude-table="+markerPattern,
		connURL,
	)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if runErr := cmd.Run(); runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			t.Fatalf("invariants: pg_dump exited %d: %s", exitErr.ExitCode(), strings.TrimSpace(stderr.String()))
		}
		t.Fatalf("invariants: running pg_dump: %v", runErr)
	}
	return stdout.String()
}

// diffDumps compares two data dumps and returns "" when they are identical, or
// a message naming the table the first difference is in and the two lines.
//
// The table comes from the enclosing `COPY <table> (cols) FROM stdin;` header,
// which is how a byte difference is turned into the table-and-column message
// this suite promises. The column is named when the two rows differ in exactly
// one field, which is the common case for a masking key that changed.
func diffDumps(a, b string) string {
	left, right := strings.Split(a, "\n"), strings.Split(b, "\n")

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
