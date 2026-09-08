// SPDX-License-Identifier: Apache-2.0

package testutil

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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

	if err := requireSuperuser(ctx, conn); err != nil {
		return err
	}

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

// requireSuperuser fails before pagila is loaded when the connecting role is
// not a superuser.
//
// Both of the two things LoadPagila does beyond issuing the dump need it, and
// neither says so when it fails. `CREATE ROLE postgres` needs `CREATEROLE` or
// superuser; the dump's own `ALTER ... OWNER TO postgres` needs the
// *connecting* user to be a superuser or a member of the target role, and it
// appears about 40 statements into a 1,900-line file. Without this guard a
// non-superuser connection fails deep inside the dump with `42501: must be
// member of role "postgres"` and a line number, which reads as a broken
// fixture rather than as a wrong connection URL.
//
// It is a precondition and not a repair: LoadPagila must never grant itself
// the privilege, and it must never create `postgres` as a superuser to get
// around this (testdata/README.md, and internal/testutil/CLAUDE.md's "Never").
func requireSuperuser(ctx context.Context, conn *pgconn.PgConn) error {
	res, err := conn.Exec(ctx, `SELECT current_user, current_setting('is_superuser')`).ReadAll()
	if err != nil {
		return fmt.Errorf("testutil: asking whether the connecting role may load pagila: %w", err)
	}
	if len(res) != 1 || len(res[0].Rows) != 1 || len(res[0].Rows[0]) != 2 {
		return errors.New("testutil: asking whether the connecting role may load pagila: unexpected result shape")
	}
	role, isSuperuser := string(res[0].Rows[0][0]), string(res[0].Rows[0][1])
	if isSuperuser == "on" {
		return nil
	}
	return fmt.Errorf("testutil: loading pagila needs a superuser connection and %q is not one: the dump "+
		"ends most objects with `ALTER ... OWNER TO postgres`, which needs the connecting role to be a "+
		"superuser, and the role itself has to be created first. Point LoadPagila at the container's own "+
		"superuser (internal/testutil.Postgres returns such a URL); do not create `postgres` as a "+
		"superuser to work around this", role)
}

// StreamRows is the number of rows LoadNasty(..., big=true) puts in
// public.stream_rows. It is the number written into nasty.sql's own gate, and
// splitNastyGate checks the two have not drifted apart.
const StreamRows = 2_000_000

// StreamDocs is the same for public.stream_docs, the text-keyed half of the
// gate (testdata/README.md trap 26). It is smaller than StreamRows and costs
// more per row to hold: a text key set is a slab of encoded tuples with an
// index rather than a []int64, and a chunk of one is a []string of separately
// allocated strings, so a stage that copies a whole key set is caught here at
// a million rows where stream_rows lets it pass at two.
//
// The gate creates that table as well as filling it, which stream_rows' half
// does not: a default load has 25 tables and a big one 26. README trap 26 gives
// the reason and names what a later task moving it above the gate has to update
// with it.
const StreamDocs = 1_000_000

// nastyGate is the first line of the psql conditional at the end of nasty.sql.
// Everything from here to the end of the file is the psql spelling of what
// LoadNasty does from Go when big is true.
const nastyGate = `\if :{?big}`

// nastyNotRecreatableGate is the first line of the psql conditional around
// trap 25's foreign key, public.price_list_notes.list_id REFERENCES
// public.price_lists_eu (list_id). That edge is ForeignKey.NotRecreatable by
// design (testdata/README.md trap 25), and checkRecreatable
// (internal/plan/plan.go) scans every edge in the schema before a root is even
// chosen and refuses any Plan call over one carrying such an edge,
// unconditionally -- so leaving the constraint in place on every load would
// make nasty.sql refuse to plan for every caller, not only the two tests this
// trap is for. LoadNasty always cuts the gated block out; LoadNastyNotRecreatable
// puts it back.
const nastyNotRecreatableGate = `\if :{?notrecreatable}`

// LoadNasty loads testdata/nasty.sql, the hostile fixture, into the database at
// connURL. Every object in that file is a trap and testdata/README.md states
// the behaviour lazyslice must show for each one.
//
// big fills public.stream_rows with StreamRows rows, and creates and fills
// public.stream_docs with StreamDocs rows. That costs some seconds and a few
// hundred megabytes of table, and only the streaming tests want it, so every
// other caller passes false and gets an empty stream_rows -- its fill function
// and its constraints still in place -- and no stream_docs at all.
//
// The fills are the tail of nasty.sql, gated there behind `\if :{?big}` so that
// `psql -v big=1 -f testdata/nasty.sql` does the same thing. Nothing here
// interprets psql conditionals: the gate is cut off the script and, when big is
// set, the statements inside it are issued directly.
//
// LoadNasty also cuts out trap 25's foreign key (nastyNotRecreatableGate),
// unconditionally, so that every caller here -- and everything downstream that
// calls Plan on the schema this loads -- gets a fixture that plans. Use
// LoadNastyNotRecreatable to load the fixture with that edge present.
func LoadNasty(ctx context.Context, connURL string, big bool) error {
	script, err := readFixture("nasty.sql")
	if err != nil {
		return err
	}
	script, _, err = cutGate(script, nastyNotRecreatableGate)
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
		return fmt.Errorf("testutil: filling public.stream_rows and public.stream_docs: %w", err)
	}
	return nil
}

// LoadNastyNotRecreatable loads testdata/nasty.sql exactly as
// LoadNasty(ctx, connURL, false) does, plus trap 25's foreign key:
// public.price_list_notes.list_id REFERENCES public.price_lists_eu (list_id).
// introspect must report that edge NotRecreatable, and the planner must refuse
// any Plan call over a schema carrying it (ARCHITECTURE.md section 11.1, exit
// 13, target.schema.not_recreatable) before a root is even chosen. LoadNasty
// leaves the edge out so every other caller gets a plannable fixture; only
// tests about trap 25 itself want this loader.
func LoadNastyNotRecreatable(ctx context.Context, connURL string) error {
	script, err := readFixture("nasty.sql")
	if err != nil {
		return err
	}
	withoutFK, fk, err := cutGate(script, nastyNotRecreatableGate)
	if err != nil {
		return err
	}
	if !strings.Contains(fk, "price_list_notes") || !strings.Contains(fk, "price_lists_eu") {
		return errors.New("testutil: nasty.sql's notrecreatable gate no longer adds the " +
			"price_list_notes -> price_lists_eu foreign key; trap 25 and LoadNastyNotRecreatable have drifted apart")
	}
	body, _, err := splitNastyGate(withoutFK)
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
	if err := execScript(ctx, conn, fk); err != nil {
		return fmt.Errorf("testutil: adding trap 25's foreign key: %w", err)
	}
	return nil
}

// cutGate returns script with the psql conditional opened by gate (and closed
// by the next `\endif`) excised, and the SQL that conditional contains.
func cutGate(script, gate string) (rest, inside string, err error) {
	before, after, found := strings.Cut(script, gate)
	if !found {
		return "", "", fmt.Errorf("testutil: nasty.sql no longer contains the gate %q", gate)
	}
	inside, tail, found := strings.Cut(after, `\endif`)
	if !found {
		return "", "", fmt.Errorf("testutil: nasty.sql's %q gate is not closed by \\endif", gate)
	}
	return before + tail, inside, nil
}

// splitNastyGate returns nasty.sql (or the tail of it left after cutGate has
// already removed an earlier gate) without its trailing `\if :{?big}` gate,
// and the SQL that gate contains.
//
// The gate is required to be there and to fill StreamRows and StreamDocs rows:
// a fixture edit that renames it, removes it or changes a row count would
// otherwise leave LoadNasty(big=true) quietly doing something other than what
// `psql -v big=1` does, and every streaming test downstream would be measuring
// the wrong table. Both fills are checked, not only the first: adding the
// second one to this file and not to the gate, or the other way round, is
// exactly the drift this function exists to refuse.
func splitNastyGate(script string) (body, fill string, err error) {
	body, fill, err = cutGate(script, nastyGate)
	if err != nil {
		return "", "", err
	}
	for _, want := range []string{
		"fill_stream_rows(" + strconv.Itoa(StreamRows) + ")",
		"fill_stream_docs(" + strconv.Itoa(StreamDocs) + ")",
	} {
		if !strings.Contains(fill, want) {
			return "", "", fmt.Errorf("testutil: nasty.sql's gate does not call %s; the row-count constants and the fixture have drifted apart", want)
		}
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
// reading `COPY ... FROM stdin;`, and only where a statement may actually
// begin. That second condition is the one an earlier version of this loader
// did not have. A line reading `COPY ... FROM stdin;` inside a dollar-quoted
// function body, inside a psql conditional, or halfway through any statement
// this loader is deliberately not parsing, is not a copy header: cutting the
// statement there sends two halves to the server as SQL, and the failure is a
// syntax error pointing at the wrong line. So the header is taken only when
// nothing is part-written -- the pending text is empty, or ends in a
// semicolon -- and never inside a dollar-quoted string. Anything else stays
// where it is and goes to the server with the statement around it, which is
// the only reading that can be right without a parser.
func execScript(ctx context.Context, conn *pgconn.PgConn, script string) error {
	var pending strings.Builder
	inDollarQuote := ""

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

		if copySQL, ok := copyHeader(trimmed); ok && statementMayBegin(pending.String(), inDollarQuote) {
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
		if strings.HasPrefix(trimmed, `\`) && inDollarQuote == "" {
			return fmt.Errorf("testutil: %q: unsupported backslash command in a fixture", trimmed)
		}

		pending.WriteString(line)
		pending.WriteByte('\n')
		inDollarQuote = trackDollarQuote(inDollarQuote, line)
	}
	return flush()
}

// statementMayBegin reports whether the next line of a script could start a
// statement: nothing is part-written in pending, and no dollar-quoted string is
// open.
//
// Comments and blank lines do not count as part-written, because pg_dump puts a
// comment block immediately above every COPY header it writes; nor do the
// completed statements of earlier lines, because those end in a semicolon and
// are flushed by the caller anyway.
//
// Its three cases are covered by fixtures_unit_test.go, because none of the
// files in testdata/ contains a `COPY ... FROM stdin;` line inside a
// dollar-quoted body or halfway through a statement: this function is right or
// wrong entirely on the strength of that test.
func statementMayBegin(pending, inDollarQuote string) bool {
	if inDollarQuote != "" {
		return false
	}
	var last byte
	for _, line := range strings.Split(pending, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			continue
		}
		last = trimmed[len(trimmed)-1]
	}
	return last == 0 || last == ';'
}

// dollarTag matches a dollar-quote delimiter: `$$` or `$tag$`.
var dollarTag = regexp.MustCompile(`\$[A-Za-z_\x80-￿][A-Za-z0-9_\x80-￿]*\$|\$\$`)

// trackDollarQuote returns the dollar-quote tag still open after line, given
// the one open before it, and "" when none is.
//
// It is not a lexer and does not need to be. Its only job is to keep
// execScript from mistaking a line inside a function body for the start of a
// statement: the delimiter that opens a body has to be matched exactly to
// close it (PostgreSQL's own rule), and nothing in testdata/ nests one dollar
// quote inside another under the same tag.
func trackDollarQuote(open, line string) string {
	// A line comment cannot open a dollar quote, and pg_dump writes none
	// inside a body, so the cheap approximation is to stop at `--` when
	// nothing is open. When something is open, `--` is body text.
	if open == "" {
		if i := strings.Index(line, "--"); i >= 0 {
			line = line[:i]
		}
	}
	for _, tag := range dollarTag.FindAllString(line, -1) {
		switch open {
		case "":
			open = tag
		case tag:
			open = ""
		}
	}
	return open
}

// copyHeader recognises pg_dump's data header and returns it unchanged, for use
// as the SQL of a copy-protocol exchange.
//
// Both ends of the match are case-insensitive. SQL keywords are, pg_dump
// happens to write them upper case, and a fixture hand-written or re-dumped by
// a tool that writes `copy public.t (a, b) from stdin;` would otherwise be
// accumulated as ordinary SQL and fail against the server with `57014` or a
// syntax error on its first data line -- a loud failure, but one that names
// the data rather than the header. strings.EqualFold on the prefix rather than
// ToLower on the whole line, because the rest of the line is an identifier
// list that must reach the server exactly as written.
func copyHeader(line string) (string, bool) {
	const prefix = "COPY "
	if len(line) < len(prefix) || !strings.EqualFold(line[:len(prefix)], prefix) {
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
