# internal/event

The progress event model: `Stage`, `Kind`, `Code`, `Event`, `Args`, `Sink`.
Nothing that produces events beyond the type definitions — `core.Run` is the
only producer, and it lives in `internal/core`, not here. The code catalogue
(`catalogue.yml`, source of `docs/ERRORS.md`) lives here too.

**Contract.** ARCHITECTURE.md §7, `type Event struct`. `event.Event` is one of
the four "value-free types" `TestNoValueBearingFieldSerialised` walks (§2); its
transitive field types must never reach `dsn.DSN`, `pipeline.RowBatch` or
`pipeline.Table` (and so `Table.Samples`).

**Rules.**
- No free-form string field on `Event`. A message is a `Code` plus `Args`
  drawn from the fixed `ArgKey` enum — identifiers and formatted numbers only,
  never a sample value (THREAT_MODEL.md T4).
- Import only `ref`. This package must never import `pipeline` — that edge
  reversed would let a value-bearing pipeline type reach an event by mistake,
  and `TestImportGraph` fails on it either way.
- Every `Code` used anywhere in the tree has a row in `catalogue.yml`; CI fails
  on a code missing from the catalogue. Every `Error`-kind code carries its
  ADR-005 exit code there.
- A `Code`'s template may only reference `Args` keys inside the `ArgKey` enum;
  a test parses every template against every enum key.

**Test.** `go test ./internal/event/...`.

**Never:** add a `string` field to `Event` for a formatted message; import
`pipeline`; let a template reference an `Args` key that isn't declared.
