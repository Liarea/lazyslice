# internal/extract

Streams the planned rows out of the source snapshot into `chan
pipeline.RowBatch`, one table at a time, in plan order, in chunks of 2,000
keys joined through a typed `unnest`. No masking (that's `internal/transform`),
no writing to a target — this package only reads.

**Contract.** ARCHITECTURE.md §2 "extract, transform, load" `RowBatch` and
`Extractor.Extract(ctx, Reader, *Plan, out chan<- RowBatch) error`, plus
§12's "chunked typed unnest joins over the snapshot".

**Rules.**
- Tables are strictly sequential on the channel: every batch of table T
  precedes the first batch of the next table, and `Last` marks T's final
  batch. A table with zero rows still sends one empty batch with `Last` set —
  the loader infers boundaries only from `Last`, never from a change of
  `Table` (ARCHITECTURE.md §2 `RowBatch` doc). Changing this contract needs an
  ARCHITECTURE.md update first (it says so explicitly: "a future parallel
  extract must change this contract, not work around it").
- Runs entirely inside the run's `REPEATABLE READ READ ONLY` snapshot, through
  the shape allowlist — extract is the stage holding the snapshot open on
  production, and the hold estimate was printed before this started
  (THREAT_MODEL.md T9).
- `Chunk` encoding per ARCHITECTURE.md §2: `[]int64` for integer types,
  `[]string` for text/varchar/bpchar/citext, `[]pgtype.UUID` for uuid, `[]string`
  with a cast otherwise. `[]any` is never passed to `Query` (pgx cannot infer
  an array OID for it). The typed array and its cast both come off the `Chunk`
  itself, so this package cannot disagree with the one that built it.
- Only `Reader.Query`, never `CopyFrom`/`SendBatch`/`Prepare` on the source
  (`TestSourceNeverCopiesOrBatches`).
- Every read is `ORDER BY`ed. Two runs over one snapshot have to hand the
  loader byte-identical batches (invariant I3), and an unordered read is only
  reproducible by accident.

**Decisions made during implementation.**
- **`New` takes the `*pipeline.Schema`.** §2's `Extract` is given the plan and
  not the schema, and a `Step` names a table, a mode, an identity and a key
  set — no column list. The copied columns are §11.1's ("explicit column lists
  excluding generated columns", which the target recomputes), and those are a
  property of the schema. `Loader.Load` is given both; this constructor is the
  smallest way to give extract the same two without changing an interface §2
  fixes. §2's `Extractor` comment is owed the same line.
- **`Shapes(plan)` takes the plan**, unlike `introspect.Shapes()` and
  `plan.Shapes()`, because the lookup reads are named per table and the plan is
  what knows which tables those are. `Shapes(nil)` is the keyed read alone, for
  a caller compiling the allowlist to reason about it rather than to run a plan
  through it (`internal/plan`'s
  `TestTheComposedAllowlistStillRefusesAnUnboundedRead`). `internal/core`
  registers these **after** planning and before the first read. That test cannot
  call `Shapes()` — internal/CLAUDE.md's Never list forbids one stage package
  reaching another, and a test about keeping the allowlist narrow is the wrong
  place to make the first exception — so it carries the two templates as
  literals, and `TestTheShapeTemplatesAreWhatTheComposedAllowlistTestCopies`
  here is the other end of that copy: edit a template and it fails, naming the
  file that has to follow.
- **The lookup read is named and bounded in the template**, not only in the
  builder: `SELECT {selectlist} FROM "public"."categories" t ORDER BY {idents}
  LIMIT 1001`. The allowlist is one per-`Source` union that every stage
  registers into additively, so a table-agnostic, unbounded form here would be
  `internal/plan`'s seed read with its bound removed, for every relation
  including `pg_catalog.pg_authid`. The bound is `internal/plan`'s
  `countProbeLimit` (1001), which the planner already proved the table is
  under, so it costs a real lookup read nothing. This replaces the wider
  `pg.ExtractShapes()` the hand-off note asked for narrowing
  (internal/pg/CLAUDE.md, now deleted with the file).
- **A `SchemaOnly` step sends no batch at all.** It has no key set and §11.1
  recreates its DDL and nothing else, so there is no data phase to bound; verify
  reports it rather than counting rows for it (§6 item 5). Every other step
  sends at least one batch and exactly one `Last`.
- **A table whose every column is generated is copied as zero columns** and
  still gets its boundary batch, so the loader sees the table.
- **A `Lookup` step's ordering comes from the table**, because
  `internal/plan` builds a lookup `Step` with the mode and nothing else — no
  `Identity`. It is the primary key, else the first unique, non-partial,
  non-expression, immediate index, else every copied column. The last rung is a
  total order over distinct rows and is also the one that can fail, on an
  unkeyed lookup holding a `json`, `xml` or `point` column: that is a loud 42883
  naming the table, and the alternative is an unordered read two runs need not
  agree on.
- **`joinCast` (keys.go) is the only thing this package re-derives from a
  column.** It is the `kindOther` case of `internal/plan`'s `typeOf`: a key that
  travelled as text is cast back to the column's own type in the join, so the
  comparison is that type's equality. `bpchar` is deliberately *not* cast back
  — `character(n)` is blank-padded and the planner reads such a key trimmed, so
  a cast back would match no row. `internal/plan`'s encoding is unexported and a
  stage package may not import another; if a third copy of this rule appears,
  the fix is a shared home for it.
- **A standby cancellation is `extract.refused.standby_cancelled`, exit 7**
  (ADR-005: "Standby cancellation is exit 7 with a retry message"). It is
  recognised by SQLSTATE 40001 alone; a `PgError`'s `Detail`, `Where` and `Hint`
  quote the row and never reach the refusal (THREAT_MODEL.md T4). Every other
  read failure is a wrapped error naming the table. pgx surfaces a cancelled
  read either at the call or from `Rows.Err()` depending on when the server said
  so, so both paths are recognised and both are tested
  (`TestAStandbyCancellationIsACodedExitSeven`).
- **The keys are the one thing not bounded by construction.**
  `pipeline.KeySet.Chunks` materialises every chunk in one call and each chunk
  holds its *own copy* of the keys (`internal/plan/keyset.go` allocates a fresh
  typed array per chunk), so at the top of a keyed step this package briefly
  holds a second copy of that step's key set — 16 MiB for the 2,000,000-row int8
  identity `TestMemoryStaysBoundedOnTwoMillionRows` uses, proportionally more for
  a composite text or uuid identity. `step` drops each chunk as it consumes it
  (`chunks[i] = nil`), so the *sustained* cost is one chunk; the *peak* is one
  key set. Bounding the peak needs a chunk-at-a-time iterator on
  `pipeline.KeySet`, which is an ARCHITECTURE.md §2 change and is reported
  rather than made here. The package doc says exactly this; it used to claim
  "one chunk of keys ... live at a time", which the code did not hold.
- **Rows are scanned into `*any`**, which is the plan pgx uses for `Rows.Values`:
  each column comes back as its own Go type (`int64`, `time.Time`,
  `netip.Prefix`, `pgtype.Numeric`, `map[string]any` for `jsonb`) rather than as
  text, which is what `internal/transform` masks and what the loader encodes
  back. An array arrives as `[]any`, which loses dimensions and lower bounds; a
  multi-dimensional array will therefore reload as one-dimensional, and that is
  reported as an open question rather than handled here.

**Test.** `go test ./internal/extract/...` for the channel contract, the
shapes and the refusal a cancelled read raises (a fake `Reader` returning a
`*pgconn.PgError` from `Query` and from `Rows.Err()`, asserting the code, the
exit, the table, the SQLSTATE and that no `Detail`/`Where`/`Hint` text reaches
the error string); `go test -tags integration ./internal/extract/...` for the reads
themselves against `nasty.sql`, including
`TestMemoryStaysBoundedOnTwoMillionRows`, which extracts **and masks**
`stream_rows` (2M rows, `-v big=1`) under a `runtime.MemStats` ceiling of
64 MiB above the post-plan baseline. Observed growth is about 20 MiB; a run
that buffered a table would need hundreds.

**Never:** buffer a whole table in memory; interleave batches from two tables
on one channel; infer a table boundary from anything but `Last`; issue
`CopyFrom`/`SendBatch`/`Prepare` against the source; register a shape wide
enough to make another stage's bound optional.
