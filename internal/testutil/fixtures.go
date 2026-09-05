package testutil

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

// PagilaCommit is the devrimgunduz/pagila commit the files under
// testdata/pagila are copied from, byte for byte. testdata/README.md carries
// the checksums and the reason this tag rather than master.
const PagilaCommit = "fef9675714cfba1756df4719b5e36075a7ddf90e"

// LoadPagila loads the Pagila schema and data into the database at connURL.
//
// Pagila is the friendly fixture: a schema shaped like something a person would
// actually design, with a partitioned payment table and enough rows that a
// subset is visibly smaller than the whole. The two files under testdata/pagila
// are upstream's, unedited, so that the pinned commit can be verified with
// nothing but curl and shasum.
//
// The dump ends most objects with ALTER ... OWNER TO postgres, and the
// container's superuser is not called postgres (postgres.go), so the role is
// created first when it is missing. It is created with no attributes at all:
// assigning ownership needs the *connecting* user to be a superuser, never the
// target role, and pagila defines public.rewards_report as SECURITY DEFINER
// with no SET search_path, so a superuser postgres would leave every fixture
// database holding a superuser-owned SECURITY DEFINER function that any role
// can call. The role is also NOLOGIN, which is CREATE ROLE's default.
//
// ANALYZE runs at the end because pg_class.reltuples is -1 until it does, and
// the classifier's partition choice and the planner's estimates both read it.
func LoadPagila(ctx context.Context, connURL string) error {
	conn, err := pgconn.Connect(ctx, connURL)
	if err != nil {
		return fmt.Errorf("testutil: connecting to load pagila: %w", err)
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()

	const ensureOwner = `DO $ensure$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'postgres') THEN
        CREATE ROLE postgres;
    END IF;
END
$ensure$;`
	if err := execScript(ctx, conn, ensureOwner); err != nil {
		return fmt.Errorf("testutil: creating the role pagila's dump assigns ownership to: %w", err)
	}

	for _, name := range []string{"pagila/pagila-schema.sql", "pagila/pagila-data.sql"} {
		script, err := readFixture(name)
		if err != nil {
			return err
		}
		if err := execScript(ctx, conn, script); err != nil {
			return fmt.Errorf("testutil: loading %s: %w", name, err)
		}
	}

	if err := execScript(ctx, conn, "ANALYZE;"); err != nil {
		return fmt.Errorf("testutil: analysing pagila: %w", err)
	}
	return nil
}

// StreamRows is the number of rows LoadNasty(..., big=true) puts in
// public.stream_rows. It is the number written into nasty.sql's own gate, and
// loadNastyGate checks the two have not drifted apart.
const StreamRows = 2_000_000

// nastyGate is the first line of the psql conditional at the end of nasty.sql.
// Everything from here to the end of the file is the psql spelling of what
// LoadNasty does from Go when big is true.
const nastyGate = `\if :{?big}`

// LoadNasty loads testdata/nasty.sql, the hostile fixture, into the database at
// connURL. Every object in that file is a trap and testdata/README.md states
// the behaviour lazyslice must show for each one.
//
// big fills public.stream_rows with StreamRows rows. That costs a few seconds
// and a couple of hundred megabytes of table, and only the streaming tests want
// it, so every other caller passes false and gets an empty stream_rows with the
// function and the constraints still in place.
//
// The fill is the tail of nasty.sql, gated there behind `\if :{?big}` so that
// `psql -v big=1 -f testdata/nasty.sql` does the same thing. Nothing here
// interprets psql conditionals: the gate is cut off the script and, when big is
// set, its two statements are issued directly.
func LoadNasty(ctx context.Context, connURL string, big bool) error {
	script, err := readFixture("nasty.sql")
	if err != nil {
		return err
	}
	body, fill, err := splitNastyGate(script)
	if err != nil {
		return err
	}

	conn, err := pgconn.Connect(ctx, connURL)
	if err != nil {
		return fmt.Errorf("testutil: connecting to load nasty.sql: %w", err)
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()

	if err := execScript(ctx, conn, body); err != nil {
		return fmt.Errorf("testutil: loading nasty.sql: %w", err)
	}
	if !big {
		return nil
	}
	if err := execScript(ctx, conn, fill); err != nil {
		return fmt.Errorf("testutil: filling public.stream_rows: %w", err)
	}
	return nil
}

// splitNastyGate returns nasty.sql without its trailing psql gate, and the SQL
// that gate contains.
//
// The gate is required to be there and to fill StreamRows rows: a fixture edit
// that renames it, removes it or changes the row count would otherwise leave
// LoadNasty(big=true) quietly doing something other than what
// `psql -v big=1` does, and every streaming test downstream would be measuring
// the wrong table.
func splitNastyGate(script string) (body, fill string, err error) {
	body, rest, found := strings.Cut(script, nastyGate)
	if !found {
		return "", "", fmt.Errorf("testutil: nasty.sql no longer contains the gate %q", nastyGate)
	}
	fill, _, found = strings.Cut(rest, `\endif`)
	if !found {
		return "", "", errors.New(`testutil: nasty.sql's gate is not closed by \endif`)
	}
	want := "fill_stream_rows(" + strconv.Itoa(StreamRows) + ")"
	if !strings.Contains(fill, want) {
		return "", "", fmt.Errorf("testutil: nasty.sql's gate does not call %s; StreamRows and the fixture have drifted apart", want)
	}
	return body, fill, nil
}

// testdataDir returns the absolute path of testdata/, resolved from this
// source file rather than from the working directory: a test's working
// directory is its own package directory, and there is no fixed number of
// "../" from an arbitrary package to the repository root.
func testdataDir() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("testutil: cannot resolve the path of fixtures.go")
	}
	// <root>/internal/testutil/fixtures.go
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata"), nil
}

func readFixture(name string) (string, error) {
	dir, err := testdataDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, filepath.FromSlash(name))
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("testutil: reading fixture: %w", err)
	}
	return string(b), nil
}

// execScript runs one fixture script against conn.
//
// It is not psql and it is not a SQL parser. The script goes to the server as
// written, in as few simple queries as possible, because the server already
// knows where statements end and what dollar quoting, comments and string
// literals mean. The one thing the protocol will not do for itself is a
// `COPY ... FROM stdin` block, which is how pg_dump writes data, so those are
// split out and fed through the copy protocol, taking their rows from the lines
// up to the terminating `\.`.
//
// A backslash command outside a copy block is an error rather than a silent
// skip: a fixture that grows a \connect or a \copy should fail loudly here and
// be fixed. nasty.sql's `\if :{?big}` gate is the only one in testdata/, and
// LoadNasty cuts it off before calling this.
//
// The COPY header is recognised by the shape pg_dump writes, a line of its own
// reading `COPY ... FROM stdin;`. Nothing else in testdata/ has a line like
// that -- neither pagila file has a backslash command, pagila-schema.sql and
// nasty.sql have no copy blocks at all, and no function body in any of them
// starts a line with COPY or a backslash.
func execScript(ctx context.Context, conn *pgconn.PgConn, script string) error {
	var pending strings.Builder

	flush := func() error {
		sql := pending.String()
		pending.Reset()
		if strings.TrimSpace(sql) == "" {
			return nil
		}
		if _, err := conn.Exec(ctx, sql).ReadAll(); err != nil {
			return locate(sql, err)
		}
		return nil
	}

	lines := strings.Split(script, "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSuffix(lines[i], "\r")
		trimmed := strings.TrimSpace(line)

		if copySQL, ok := copyHeader(trimmed); ok {
			if err := flush(); err != nil {
				return err
			}
			var data strings.Builder
			terminated := false
			for i+1 < len(lines) {
				i++
				if strings.TrimSuffix(lines[i], "\r") == `\.` {
					terminated = true
					break
				}
				data.WriteString(lines[i])
				data.WriteByte('\n')
			}
			if !terminated {
				return fmt.Errorf(`testutil: %s: copy block has no terminating \. line`, firstLine(copySQL))
			}
			if _, err := conn.CopyFrom(ctx, strings.NewReader(data.String()), copySQL); err != nil {
				return fmt.Errorf("%w\nstatement: %s", err, firstLine(copySQL))
			}
			continue
		}
		if strings.HasPrefix(trimmed, `\`) {
			return fmt.Errorf("testutil: %q: unsupported backslash command in a fixture", trimmed)
		}

		pending.WriteString(line)
		pending.WriteByte('\n')
	}
	return flush()
}

// copyHeader recognises pg_dump's data header and returns it unchanged, for use
// as the SQL of a copy-protocol exchange.
func copyHeader(line string) (string, bool) {
	if !strings.HasPrefix(line, "COPY ") {
		return "", false
	}
	if !strings.HasSuffix(strings.ToLower(line), "from stdin;") {
		return "", false
	}
	return line, true
}

// locate names the line a server error happened on.
//
// A whole fixture goes to the server in one query, so PgError.Position -- a
// 1-based offset in bytes into that query -- is the only thing that says where
// in a 1,900-line file the failure was, and PgError.Error() does not print it.
func locate(sql string, err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Position <= 0 || int(pgErr.Position) > len(sql) {
		return err
	}
	before := sql[:pgErr.Position-1]
	line := strings.Count(before, "\n") + 1
	start := strings.LastIndexByte(before, '\n') + 1
	end := strings.IndexByte(sql[start:], '\n')
	if end < 0 {
		end = len(sql) - start
	}
	return fmt.Errorf("%w\nat line %d of the script: %s", err, line, firstLine(sql[start:start+end]))
}

func firstLine(stmt string) string {
	s := strings.TrimSpace(stmt)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i] + " ..."
	}
	const limit = 120
	if len(s) > limit {
		s = s[:limit] + " ..."
	}
	return s
}
