# internal/testutil

Fixture loaders for `testdata/`: `LoadPagila(ctx, url)`, `LoadNasty(ctx, url,
big)`, and the Postgres container helpers other integration tests build on.
Test support code only — nothing here is imported by non-test code.

**Contract.** `testdata/README.md` is the spec this package implements: table
lists, row counts, and every one of the 22 `nasty.sql` traps it must be
possible to assert against after loading. `fixtures_test.go` (behind
`integration`) is the enforcement.

**Rules.**
- `LoadNasty`'s `big` handling must track `nasty.sql`'s own `\if :{?big}` gate
  exactly — cut the gate text off the file, run its two statements directly
  when `big` is set, and **refuse to load at all** if the gate is missing or
  no longer fills `stream_rows` (`testdata/README.md` trap 22) — the psql path
  and the Go path must never be able to drift apart silently.
- The only psql construct this package interprets beyond plain SQL is
  `COPY ... FROM stdin` (needed for `pagila-data.sql`); any other backslash
  command in a fixture file must be a loud error, never a silent partial load.
- **A `COPY ... FROM stdin;` line is a copy header only where a statement may
  begin**: nothing part-written (the pending text is empty or ends in a
  semicolon) and no dollar-quoted string open. The same line inside a plpgsql
  body, a psql conditional or any multi-line statement is body text, and
  treating it as a header cuts the statement in two and sends both halves to
  the server as SQL — a syntax error pointing at the wrong line. The header
  match is case-insensitive at both ends, because SQL keywords are, while the
  rest of the line reaches the server exactly as written (it is an identifier
  list). The three functions this rests on — `copyHeader`,
  `trackDollarQuote`, `statementMayBegin` — are covered by
  `fixtures_unit_test.go`, which carries **no build tag and needs no
  database**, because no file in `testdata/` exercises any of those cases:
  without it `TestLoadPagila` and `TestLoadNasty` pass identically whether the
  lexing is present, absent or wrong. Change the lexing and change that test,
  or delete both.
- `LoadPagila` creates the `postgres` role with **no attributes** (not
  superuser) when missing — see the fixtures.go comment on
  `rewards_report`'s `SECURITY DEFINER`; do not "fix" this by making the role
  a superuser.
- `LoadPagila` **refuses a non-superuser connection up front**
  (`requireSuperuser`). Creating that role and the dump's own
  `ALTER ... OWNER TO postgres` both need the *connecting* user to be a
  superuser, and without the guard the failure is `42501` about forty
  statements into a 1,900-line file, which reads as a broken fixture rather
  than a wrong URL. It is a precondition, never a repair: do not have the
  loader grant itself anything.
- Change a fixture's shape and its row-count/trap assertions in the same
  commit as `testdata/README.md` (that file's own rule, restated here because
  this package is what would silently stop enforcing it).
- `assertEveryTableReachesPeople` in `fixtures_test.go` is the one assertion
  that is about the fixture's *usefulness* rather than its contents: a
  `nasty.sql` table with no foreign-key path to `public.people` is
  `SchemaOnly` in every run and its trap costs nothing to pass. Do not exempt
  a table from it; connect the table, or say in `testdata/README.md` why it
  cannot be connected.

**Test.** `go test ./internal/testutil/...` for the loader's own lexing (no
Docker), then `go test -tags integration ./internal/testutil/...` for the
fixtures themselves (needs a Docker endpoint via
`internal/testutil/postgres.go`).

**Never:** let `LoadNasty`'s Go-side `big` handling diverge from the SQL
file's own gate; grant the fixture's `postgres` role superuser; add a second
psql construct interpreter instead of erroring loudly on the unsupported one.
