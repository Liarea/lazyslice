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
- **A batch is cut at `batchRows` (2,000) or `batchBytes` (8 MiB), whichever
  it reaches first** (T-PERF, docs/reviews/2026-09-09/REVIEW.md finding 9:
  "extraction batches 2,000 rows regardless of byte size. Even one batch of
  2,000 one-MiB values can require roughly two GiB before transformation
  overhead"). `batcher.add` (extract.go) tracks a cheap running estimate,
  `rowEstimate` (sql.go's `batchBytes`, extract.go's `rowEstimate`): a
  `string` or `[]byte` value counts its own length, everything else a small
  fixed constant, so the estimate needs no reflection and no second pass over
  a value the batcher does not otherwise inspect. It is a lower bound, not a
  claim about wire size or the batch's true heap footprint — the point is
  only that 2,000 one-MiB values no longer form one 2 GiB batch, not an exact
  accounting. `TestBatchesAreAlsoCutByBytes` is the guard: 20 rows of a 1 MiB
  value cut into several batches well short of the 2,000-row cap.
  docs/PERF.md has the before/after batch counts and RSS for a wide-row run.
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
  **A table name is arbitrary text and quoting it is not enough on its own**
  (T-0050): `CREATE TABLE public."{ident}"` is a table a person can make, and
  the template compiler used to read the placeholder inside the quoted name —
  turning the shape that names one table into `SELECT {selectlist} FROM {ident}
  t ORDER BY {idents} LIMIT 1001`, an ordered read of every relation, which is
  the exact widening the per-table name exists to prevent. A table called
  `{table}` was the other half: an unknown token, so the shape failed to
  compile and took the run with it. `internal/pg`'s `templateSegments` now
  treats a quoted identifier in a template as fixed text and never scans it for
  placeholders; the guard at this end is
  `TestALookupShapeForATableNamedWithBracesNamesThatTableAlone`.
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
- **The keys are bounded by construction too, since T-0050.** `step` walks a
  keyed step with `pipeline.KeySet.EachChunk(2000, f)`, the chunk-at-a-time
  iterator `internal/pipeline`'s `KeySet` carries beside `Chunks` and
  `FirstChunk`. `Chunks` builds every chunk before it returns any and each chunk
  holds its *own copy* of the keys (`internal/plan/keyset.go` allocates a fresh
  typed array per identity column per chunk), so ranging over that call held a
  second copy of the whole key set for the length of the table; `EachChunk`
  builds one, hands it over and drops it.
  `TestAChunkIsReleasedAsItIsConsumed` holds the release,
  `TestMemoryStaysBoundedOnATextKeyedTable` holds the size: on `stream_docs`
  (`testdata/README.md` trap 26, a million 36-character text keys) the growth is
  0 to 5 MiB with `EachChunk` and 63 MiB with the `Chunks` call it replaced,
  against a 32 MiB ceiling. `TestMemoryStaysBoundedOnTwoMillionRows` cannot say
  that: its identity is one `bigint`, so a second copy is 16 MiB and fits inside
  its own ceiling either way.
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
themselves against `nasty.sql`, including the two memory tests, which extract
**and mask** a gated table under a `runtime.MemStats` ceiling above the
post-plan baseline: `TestMemoryStaysBoundedOnTwoMillionRows` over `stream_rows`
(2M `bigint`-keyed rows, ceiling 64 MiB, observed ~5 MiB) and
`TestMemoryStaysBoundedOnATextKeyedTable` over `stream_docs` (1M text-keyed
rows, ceiling 32 MiB, observed ~1 MiB against 63 MiB for the buffering
version). Each skips the other's table **in the plan**, so each measures one key
encoding — but not in the fixture: `LoadNasty(big)` runs the whole gate, so both
tables are filled for either test and each pays for the other's fill (about 37s
for the text-keyed one). A selective fill means a second gate variable in
`testdata/nasty.sql`, which `psql -v big=1` and `splitNastyGate` both have to
agree with, and is reported rather than done here.
`stream_docs`, like `stream_rows`, exists on every load and is empty unless
`-v big=1` fills it (T-0077), so every test in the tree sees the same 26 tables.

**Never:** buffer a whole table in memory; interleave batches from two tables
on one channel; infer a table boundary from anything but `Last`; issue
`CopyFrom`/`SendBatch`/`Prepare` against the source; register a shape wide
enough to make another stage's bound optional.
