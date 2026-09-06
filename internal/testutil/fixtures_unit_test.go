// SPDX-License-Identifier: Apache-2.0

package testutil

import "testing"

// The three functions execScript's COPY handling rests on, tested without a
// database, because nothing else tests them at all.
//
// Neither fixture in testdata/ contains a `COPY ... FROM stdin;` line inside a
// dollar-quoted body, inside a part-written statement, or spelled in lower
// case, so TestLoadPagila and TestLoadNasty pass identically whether this
// lexing is present, absent or wrong. These are the cases the loader would get
// wrong first, and they are the whole justification for the code being here:
// if a case below stops mattering, delete the code rather than the test.
//
// No build tag: `make test` runs these, and they need no Docker endpoint.

func TestCopyHeader(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		line string
		want string // "" means: not a copy header
	}{
		{
			name: "pg_dump's own spelling",
			line: `COPY public.actor (actor_id, first_name) FROM stdin;`,
			want: `COPY public.actor (actor_id, first_name) FROM stdin;`,
		},
		{
			// A re-dump by a tool that lower-cases keywords would otherwise be
			// accumulated as ordinary SQL and fail against the server on its
			// first data line, naming the data rather than the header.
			name: "lower case at both ends",
			line: `copy public.t (a, b) from stdin;`,
			want: `copy public.t (a, b) from stdin;`,
		},
		{
			name: "mixed case",
			line: `Copy public.t From Stdin;`,
			want: `Copy public.t From Stdin;`,
		},
		{
			// The rest of the line is an identifier list and must reach the
			// server exactly as written, quotes and case included.
			name: "a quoted, mixed-case identifier is returned unchanged",
			line: `COPY public."LegacyCustomer" ("CustomerID") FROM stdin;`,
			want: `COPY public."LegacyCustomer" ("CustomerID") FROM stdin;`,
		},
		{name: "COPY TO is not a header", line: `COPY public.t TO stdout;`},
		{name: "COPY FROM a file is not a header", line: `COPY public.t FROM '/tmp/t.csv';`},
		{name: "a header with options is not the shape pg_dump writes", line: `COPY public.t FROM stdin WITH (FORMAT csv);`},
		{name: "no trailing semicolon", line: `COPY public.t FROM stdin`},
		{name: "a word beginning COPY", line: `COPYING public.t FROM stdin;`},
		{name: "empty", line: ``},
		{name: "shorter than the prefix", line: `COP`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, ok := copyHeader(tc.line)
			if ok != (tc.want != "") {
				t.Fatalf("copyHeader(%q) recognised = %v, want %v", tc.line, ok, tc.want != "")
			}
			if got != tc.want {
				t.Errorf("copyHeader(%q) = %q, want %q", tc.line, got, tc.want)
			}
		})
	}
}

func TestTrackDollarQuote(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		open string
		line string
		want string
	}{
		{name: "nothing open, nothing on the line", line: `SELECT 1;`},
		{name: "an anonymous quote opens", line: `AS $$`, want: `$$`},
		{name: "a tagged quote opens", line: `AS $body$`, want: `$body$`},
		{name: "opened and closed on one line", line: `AS $$ SELECT 1 $$;`},
		{name: "the matching tag closes it", open: `$body$`, line: `$body$ LANGUAGE sql;`},
		{
			// PostgreSQL's own rule: the delimiter that opened a body has to
			// be matched exactly. A different tag inside it is body text.
			name: "a different tag does not close it",
			open: `$body$`,
			line: `  x := $$inner$$;`,
			want: `$body$`,
		},
		{
			name: "an unrelated line inside a body leaves it open",
			open: `$$`,
			line: `  RETURN 1;`,
			want: `$$`,
		},
		{
			// The cheap approximation the function documents: with nothing
			// open, a line comment cannot open a dollar quote.
			name: "a comment cannot open a quote",
			line: `-- $$ not a quote`,
		},
		{
			// With something open, `--` is body text and the tag that follows
			// it still closes the body.
			name: "inside a body a comment marker is text",
			open: `$$`,
			line: `-- $$`,
			want: ``,
		},
		{name: "a bare dollar is not a delimiter", line: `SELECT '$' || x;`},
		{name: "a positional parameter is not a delimiter", open: ``, line: `WHERE a = $1 AND b = $2;`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := trackDollarQuote(tc.open, tc.line); got != tc.want {
				t.Errorf("trackDollarQuote(%q, %q) = %q, want %q", tc.open, tc.line, got, tc.want)
			}
		})
	}
}

func TestStatementMayBegin(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name          string
		pending       string
		inDollarQuote string
		want          bool
	}{
		{name: "nothing pending", want: true},
		{name: "blank lines only", pending: "\n\n   \n", want: true},
		{
			// pg_dump writes a comment block immediately above every COPY
			// header, so a comment must not count as part-written.
			name:    "a pg_dump comment block",
			pending: "--\n-- Data for Name: actor; Type: TABLE DATA\n--\n",
			want:    true,
		},
		{name: "a completed statement", pending: "CREATE TABLE t (a int);\n", want: true},
		{
			name:    "a completed statement followed by a comment",
			pending: "CREATE TABLE t (a int);\n\n--\n-- Data\n--\n",
			want:    true,
		},
		{name: "a part-written statement", pending: "CREATE TABLE t (\n  a int,\n", want: false},
		{
			name:    "a part-written statement under a comment",
			pending: "CREATE TABLE t (\n-- a comment inside it\n  a int,\n",
			want:    false,
		},
		{
			// The case the second condition exists for: inside a function body
			// a COPY line is body text, not a header, whatever the pending
			// text looks like.
			name:          "inside a dollar-quoted body",
			pending:       "CREATE FUNCTION f() RETURNS void AS $$\n",
			inDollarQuote: "$$",
			want:          false,
		},
		{
			name:          "inside a body that would otherwise look complete",
			pending:       "CREATE FUNCTION f() RETURNS void AS $$\n  PERFORM 1;\n",
			inDollarQuote: "$$",
			want:          false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := statementMayBegin(tc.pending, tc.inDollarQuote); got != tc.want {
				t.Errorf("statementMayBegin(%q, %q) = %v, want %v",
					tc.pending, tc.inDollarQuote, got, tc.want)
			}
		})
	}
}
