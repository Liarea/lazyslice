# internal/introspect

Reads the source catalog into a `*pipeline.Schema`: tables, columns, indexes,
constraints, sequences, enums/domains/composites, FKs, and the sample rows
used by classification. No masking, no classification decisions, no writing —
this package only reads and shapes catalog metadata plus raw samples.

**Contract.** ARCHITECTURE.md §2 "introspect": `Introspector.Introspect(ctx,
Reader) (*Schema, error)`. Definition text (`Default`, `Checks`, index and
constraint defs) must come from the catalog's own deparser (`pg_get_expr`,
`pg_get_constraintdef`, `pg_get_indexdef`) — never a printer of our own,
per §2's comment on `Column`/`Index`/`Constraint`.

**Rules.**
- Sampling is `TABLESAMPLE SYSTEM (p) REPEATABLE (seed)` for ~200 rows, never
  `LIMIT` (LIMIT samples the front of the table). A partitioned root is
  sampled through its largest leaf by `reltuples`; `Table.SampledFrom` names
  the leaf and the explanation says so (ARCHITECTURE.md §4).
- `Table.Samples` holds production values. It is the only field in `pipeline`
  that does, and it is never serialised — `TestNoValueBearingFieldSerialised`
  covers it (THREAT_MODEL.md T4). Never let a sample escape this package
  except through `Sampler.Samples`, consumed by `internal/classify`.
- A schema this package cannot recreate (a dependency on a non-`pg_catalog`
  function, a user-defined base type, etc.) is a plan-time refusal
  (ARCHITECTURE.md §11.1, exit 13) — introspect reports the dependency, it
  does not decide the refusal.
- Only `Reader.Query` is used — no `CopyFrom`, `SendBatch` or `Prepare` on a
  source connection (`TestSourceNeverCopiesOrBatches`, THREAT_MODEL.md T9).

**Test.** `go test ./internal/introspect/...`; catalog-shape correctness needs
`go test -tags integration ./internal/introspect/...` against the fixtures in
`testdata/`.

**Never:** print, log, or copy a sample value out of `Table.Samples`; issue
`CopyFrom`/`SendBatch`/`Prepare` on the source; invent DDL text instead of
using the catalog's deparser.
