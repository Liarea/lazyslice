# internal/load

Recreates the target schema and copies rows in. `ddl/` generates the
ARCHITECTURE.md §11.1 object classes from `*pipeline.Schema`; this package
drives `CopyFrom`, then indexes, FKs `NOT VALID` + `VALIDATE`, `setval`,
`ANALYZE`, after data (ARCHITECTURE.md §1, §11.1). Never connects to the
source — every argument it receives is already a `RowBatch` or a `*Schema`.

**Contract.** ARCHITECTURE.md §2 "extract, transform, load":
`Loader.Load(ctx, Writer, *Plan, *Schema, <-chan RowBatch) (*LoadResult,
error)`. §11.1 is the full DDL spec `ddl/` implements, in its stated order
(schemas, extensions, enums/domains/composites, unowned sequences, tables,
then after-data: indexes, FKs, setval, ANALYZE, bookkeeping tables).

**Rules.**
- One transaction per table, opened at `Seq 0`, committed at `Last`
  (ARCHITECTURE.md §2 `RowBatch` doc) — this is the property THREAT_MODEL.md
  T8's "empty or marked, never half-loaded-and-green" depends on. Never batch
  multiple tables' rows into one transaction, and never commit before `Last`.
- `CopyFrom` uses an explicit column list excluding generated columns
  (`GENERATED ALWAYS AS (...) STORED`) — PostgreSQL rejects a write to one
  (`428C9`); `nasty.sql`'s `people.display_name` is the fixture for this.
- `setval(seq, coalesce(max(id), 1), max(id) IS NOT NULL)` after commit, never
  a literal `0` (THREAT_MODEL.md T8, HARD_PROBLEMS.md §4.1).
- A partitioned source table becomes one plain table in the target (§11.1);
  `ddl/` never recreates partitions.
- A dependency `ddl/` cannot recreate (a non-`pg_catalog` function, a
  user-defined base type/operator class/collation) is a plan-time refusal
  (exit 13) raised before this package runs, not discovered mid-load.

**Test.** `go test ./internal/load/...`; the DDL-recreation and transaction
properties need `go test -tags integration ./internal/load/...` plus
`TestKillNineLeavesRunningMarker`.

**Never:** open a connection to the source; commit a table's transaction before
its `Last` batch; write a generated column; recreate a view, function, trigger,
policy or partition (v1 does not — §11.1's `not_recreated` list).
