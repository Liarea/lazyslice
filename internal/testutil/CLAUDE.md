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
- `LoadPagila` creates the `postgres` role with **no attributes** (not
  superuser) when missing — see the fixtures.go comment on
  `rewards_report`'s `SECURITY DEFINER`; do not "fix" this by making the role
  a superuser.
- Change a fixture's shape and its row-count/trap assertions in the same
  commit as `testdata/README.md` (that file's own rule, restated here because
  this package is what would silently stop enforcing it).

**Test.** `go test -tags integration ./internal/testutil/...` (needs a Docker
endpoint via `internal/testutil/postgres.go`).

**Never:** let `LoadNasty`'s Go-side `big` handling diverge from the SQL
file's own gate; grant the fixture's `postgres` role superuser; add a second
psql construct interpreter instead of erroring loudly on the unsupported one.
