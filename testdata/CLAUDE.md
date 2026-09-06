# testdata/

Two fixtures and nothing else: `pagila/` (the friendly schema) and
`nasty.sql` (one trap per shape that has broken a subsetting tool). No Go code
lives here — loaders are `internal/testutil`; this directory is data plus
`README.md`.

**Contract.** `README.md` in this directory is the spec: it names every table,
every row count, and every one of the 24 numbered traps in `nasty.sql` (1 to
24, with 16 split into 16a and 16b because §4 prescribes two different
behaviours for the two JSON columns) with the exact required behaviour.
`internal/testutil/fixtures_test.go` (behind `integration`) is what checks the
loaded databases against it. There is no type in ARCHITECTURE.md this directory
implements directly — it's the input the stage packages
(`internal/introspect`, `internal/plan`, `internal/classify` above all) are
proven against.

**Rules.**
- Every trap in `nasty.sql` has a numbered entry in `README.md`; a trap with
  no entry is undocumented and a red flag, and an entry describing a trap no
  longer in the file is stale — both get fixed in the same commit as the SQL
  change (`README.md`'s own opening rule).
- **Every table in `nasty.sql` has a foreign-key path to `public.people`.** It
  is the root every invariant run slices from, so a table without one is
  `Step{t, SchemaOnly}` — no read, no `COPY`, no masking, no residual scan,
  zero rows in the target — and its traps cost nothing to pass.
  `assertEveryTableReachesPeople` fails on a new one; connect it rather than
  exempting it, and if it genuinely cannot be connected, say why in
  `README.md` in the same commit.
- **A trap's required behaviour is a quotation from ARCHITECTURE.md, or it
  says out loud that the architecture does not specify it yet.** Traps 12, 15
  and 19 are the three that currently disagree with the architecture: §3's
  identity ladder runs before `--skip-table` is applied, so `click_stream` — a
  depth-1 child of the root, and reachable, which is why the flag is what
  removes it and not a reordering — is refused whatever flags are passed; §4
  and §5 say nothing about arrays; §4 masks `email_verified boolean` on a name
  hit. Each entry names the conflict and stops there. **Deciding any of the
  three is an ARCHITECTURE.md edit and a gate 4 blocker, and this directory is
  not where it gets written**: do not turn a "the rule to add" paragraph here
  into the specification, and do not invent a rule §4 or §5 does not state. A
  fixture that asserts behaviour nobody specified is a phase 4 task that
  cannot be completed.
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
