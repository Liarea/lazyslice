# testdata/

Two fixtures and nothing else: `pagila/` (the friendly schema) and
`nasty.sql` (one trap per shape that has broken a subsetting tool). No Go code
lives here — loaders are `internal/testutil`; this directory is data plus
`README.md`.

**Contract.** `README.md` in this directory is the spec: it names every table,
every row count, and every one of the 22 numbered traps in `nasty.sql` with
the exact required behaviour. `internal/testutil/fixtures_test.go` (behind
`integration`) is what checks the loaded databases against it. There is no
type in ARCHITECTURE.md this directory implements directly — it's the input
the stage packages (`internal/introspect`, `internal/plan`, `internal/classify`
above all) are proven against.

**Rules.**
- Every trap in `nasty.sql` has a numbered entry in `README.md`; a trap with
  no entry is undocumented and a red flag, and an entry describing a trap no
  longer in the file is stale — both get fixed in the same commit as the SQL
  change (`README.md`'s own opening rule).
- `pagila/` is a byte-for-byte copy of a pinned upstream commit; its SHA-256
  checksums in `README.md` are how the pin is verified. Never hand-edit
  `pagila-schema.sql` or `pagila-data.sql` — re-pin to a different upstream
  commit instead, and update the checksums and the "why this tag" note
  together.
- `nasty.sql`'s `stream_rows` gate (`\if :{?big}`) is part of the fixture's
  contract with `internal/testutil.LoadNasty`; changing one without the other
  breaks that package's own rule.

**Test.** `psql -f testdata/pagila/pagila-schema.sql -f
testdata/pagila/pagila-data.sql` and `psql -f testdata/nasty.sql` to load by
hand; `go test -tags integration ./internal/testutil/...` to check them.

**Never:** add a trap without a `README.md` entry; edit `pagila`'s SQL files
in place instead of re-pinning; let the row counts or trap list in `README.md`
go stale relative to the SQL.
