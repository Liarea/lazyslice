# internal/testutil

Fixture loaders for `testdata/`: `LoadPagila(ctx, url)`, `LoadNasty(ctx, url,
big)`, `LoadNastyNotRecreatable(ctx, url)`, and the container helpers other
integration tests build on — `Postgres(ctx, t, image)` and
`PgBouncer(ctx, t, image, settings...)`. Test support code only — nothing here
is imported by non-test code.

**Contract.** `testdata/README.md` is the spec this package implements: table
lists, row counts, and every one of the 27 `nasty.sql` traps it must be
possible to assert against after loading. `fixtures_test.go` (behind
`integration`) is the enforcement.

**Rules.**
- `LoadNasty`'s `big` handling must track `nasty.sql`'s own `\if :{?big}` gate
  exactly — cut the gate text off the file, run the statements inside it
  directly when `big` is set, and **refuse to load at all** if the gate is
  missing or no longer fills `stream_rows` (`testdata/README.md` trap 22) and
  `stream_docs` (trap 26) with `StreamRows` and `StreamDocs` rows — the psql
  path and the Go path must never be able to drift apart silently. Both fills
  are checked, not only the first: the two tables exist to measure two key
  encodings (`bigint` and 36-character `text`), and a gate that quietly stopped
  filling one would leave the test that needs it measuring an empty table.
  `stream_docs`'s `CREATE TABLE` and `CREATE FUNCTION` sit above the gate, next
  to `stream_rows`', so both tables exist on every load and `nastyTables` lists
  both at 0 rows; only the million-row fill is gated.
- **`PgBouncer` starts its own server**, on a Docker network the two containers
  share, and returns the pooled URL and the direct one. Load a fixture through
  the direct URL — the loader speaks psql constructs a pooler has no reason to
  survive — and open the source through the pooled one. It does not set
  `IGNORE_STARTUP_PARAMETERS`: the image's default is `extra_float_digits` and
  nothing else, which is the stock behaviour `internal/pg` has to meet, and
  widening it here would quietly delete the assertion the container exists to
  make.
- **`PgBouncer`'s `settings` are merged over its defaults, `DATABASE_URL` last.**
  The edoburu image writes each environment entry into the `pgbouncer.ini` it
  generates, so a test asks for a restrictive pooler with
  `MAX_DB_CONNECTIONS`/`DEFAULT_POOL_SIZE` and shortens the wait with
  `QUERY_WAIT_TIMEOUT` (`internal/pg`'s
  `TestAPoolerWithOneServerConnectionSerialisesTheExtract`, which is ADR-005's
  serialised extract driven by the pooler rather than by a synthetic snapshot
  id). A caller may override any default, `POOL_MODE` included; `DATABASE_URL`
  is written after the merge, because the pooler's route to the server this
  function started is not a test's to redirect.
- `nasty.sql` also gates trap 25's foreign key
  (`public.price_list_notes.list_id REFERENCES public.price_lists_eu
  (list_id)`) behind `\if :{?notrecreatable}`, the same device. That edge is
  `ForeignKey.NotRecreatable`, and `internal/plan`'s `checkRecreatable` refuses
  any `Plan` call over a schema carrying it, unconditionally, before a root is
  even chosen — so `LoadNasty` always cuts it out, and every caller of
  `LoadNasty` gets a fixture that plans. `LoadNastyNotRecreatable` loads the
  fixture with that one constraint added back; it exists for the two tests
  that are about trap 25 itself (`internal/introspect`'s `TestIntrospectNasty`
  and `internal/plan`'s `TestPlanNastyNotRecreatable`), not for general use.
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
`internal/testutil/postgres.go`). `PgBouncer` has no test of its own here: its
subject is `internal/pg`'s behaviour through a pooler, so
`internal/pg/pooler_integration_test.go` is what exercises it, and a helper that
started nothing would fail there.

**Never:** let `LoadNasty`'s Go-side `big` handling diverge from the SQL
file's own gate; grant the fixture's `postgres` role superuser; add a second
psql construct interpreter instead of erroring loudly on the unsupported one.

## The port-mapping retry is sized for `make torture` (T-TORTURE)

`portEndpointAttempts`/`portEndpointBudget` are thirty attempts over thirty
seconds, not ten over five. `make integration` starts about a dozen containers
and never exhausted five seconds; `make torture` starts about forty and
exhausted them roughly once a run, failing a different torture schema each time
with a message about a port rather than about anything the run was testing
(T-0052 is the race). The budget is only ever spent when the race happens: the
first attempt succeeds otherwise and the loop returns.
