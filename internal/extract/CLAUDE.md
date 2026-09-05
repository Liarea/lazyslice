# internal/extract

Streams the planned rows out of the source snapshot into `chan
pipeline.RowBatch`, one table at a time, in plan order, in chunks of 2,000
keys joined through a typed `unnest`. No masking (that's `internal/transform`),
no writing to a target — this package only reads.

**Contract.** ARCHITECTURE.md §2 "extract, transform, load" `RowBatch` and
`Extractor.Extract(ctx, Reader, *Plan, out chan<- RowBatch) error`.

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
  an array OID for it).
- Only `Reader.Query`, never `CopyFrom`/`SendBatch`/`Prepare` on the source
  (`TestSourceNeverCopiesOrBatches`).

**Test.** `go test ./internal/extract/...`; the streaming/memory claim
(resident memory not growing with row count) needs
`go test -tags integration ./internal/extract/...` against `nasty.sql`'s
`stream_rows` (2M rows with `-v big=1`).

**Never:** buffer a whole table in memory; interleave batches from two tables
on one channel; infer a table boundary from anything but `Last`; issue
`CopyFrom`/`SendBatch`/`Prepare` against the source.
