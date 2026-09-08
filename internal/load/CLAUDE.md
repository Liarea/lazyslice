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
  user-defined base type/operator class/collation) is exit 13, and
  ARCHITECTURE.md §11.1 requires it **at plan** — before the snapshot is used
  for keys and before anything in the target is dropped. **Nothing calls
  `ddl.Recreatable` at plan today**, so `Load` calls it itself as its first
  statement, before the marker row and before the first drop. Read the owed
  item below before relying on the §11.1 ordering.
- `setval`'s first argument is a `regclass`, and `text` cast to `regclass` is
  parsed as an identifier: the literal carries the **quoted** name
  (`setval('"public"."LegacyCustomer_CustomerID_seq"', ...)`) or a mixed-case
  sequence resolves to a relation that does not exist. Every name this package
  puts in a statement goes through `qualified()` or `quoteIdent()`, including
  the ones inside string literals.
- `lazyslice_meta` is not an object class. `ddl.recreated` leaves it out, so it
  is neither recreated nor analysed nor fingerprinted; §11.2's binding needs a
  target lazyslice wrote to fingerprint equal to its source, and lazyslice
  creates that table in the target itself.

**Decisions made during implementation.**
- **`New` takes a `Run` and an `event.Sink`.** ARCHITECTURE.md §2's
  `Loader.Load` has nowhere to put either. The sink is there because §11.1
  requires each drop to be **printed before it happens**, and a list of drops
  returned at the end of the stage is a list printed after the tables are gone;
  `core.Run` stays the only producer of events by handing its own sink in
  (`event.Discard` when nil). `Run` carries the marker's identity fields — tool
  version, the three fingerprints, the source fingerprint and system id — which
  are the run's and not the plan's; root and take come from the plan. **Not**
  `schema_fingerprint`: see the fingerprint entry below.
- **This package imports `internal/pg`, which no other stage package does.**
  It is for the marker alone (`pg.MarkerTable`, `MarkerDDL`, `EnsureMarker`,
  `StartRun`, `FinishRun`, the three statuses). `internal/pg/marker.go` already
  declares those and says in terms that "the loader calls it before the first
  drop"; the alternative was a second copy of the `lazyslice_meta` DDL in this
  package, and two definitions of the table the gate reads and the loader writes
  is a worse failure than one import. The graph stays acyclic (`pg` imports
  `pipeline`, `ref`, `dsn`, `event` and never `load`).
- **The drop list is the source's tables plus `Run.TargetTables`, not "every
  user table in the target".** `pipeline.Writer` has `Exec`, `CopyFrom` and
  `Begin` and no way to read (§2), so this package cannot enumerate the target.
  The gate already enumerates it; `Run.TargetTables` is where that list goes,
  and until a caller passes it, a target table under a name the source does not
  use is **left alone rather than dropped silently** — the direction that
  destroys less than §11.1 licenses, not more. Passing it is owed to `core`.
- **`DropObjects` exists and §11.1 does not name it.** `CREATE TYPE` has no
  `IF NOT EXISTS` and a sequence a previous run left behind is owned by no
  table, so without dropping the sequences and types the pre-data DDL is about
  to create, the *second* run against a marked target — the case §11.2 exists to
  allow — fails at 42710 with the target's tables already dropped.
  `TestLoadPagilaIntoAMarkedTarget` is the guard. Tables are dropped `CASCADE`;
  types and sequences are **not**, so an object something unknown still depends
  on is a loud 2BP01 rather than a stranger's column quietly removed.
- **`ALTER SEQUENCE ... OWNED BY` is emitted after the tables**, which §11.1
  does not list either. It is what makes `DROP TABLE` take the sequence with it,
  so the drop path above is enough on the second run.
- **`setval` finds its column through the column default when the catalog
  records no ownership.** pagila 3.1.0 declares none: every one of its twelve
  sequences is reached only through `nextval(...)` in a default, so a `setval`
  that read `SequenceDef.Column` alone would reset nothing in the project's own
  friendly fixture — which is exactly the failure THREAT_MODEL.md T8 names. Both
  the qualified and the bare spelling of the sequence name are looked for,
  because `pg_get_expr` writes the bare one for a sequence in the search path.
- **The index behind a primary key, unique or exclusion constraint is not
  replayed.** `pg_get_indexdef` prints `CREATE UNIQUE INDEX` for it and the
  `CREATE TABLE` has already created the constraint; the filter is by name.
- **A foreign key the source has not validated is added `NOT VALID` and never
  validated.** It holds in the target exactly as much as it holds in the source;
  validating it would fail the run over rows the source itself does not check. A
  `VALIDATE` that fails is exit 8 (`load.refused.fk_invalid`), which is
  ADR-005's foreign-key code found one stage before verify.
- **Owed: `internal/plan` (or `core`, before the drop) must call
  `ddl.Recreatable`.** §11.1 raises the not-recreatable refusal at plan;
  `internal/plan/plan.go`'s `checkRecreatable` covers `ForeignKey.
  NotRecreatable` and nothing else, and stage packages do not import each
  other, so the natural caller is `core`, which does not exist yet. Until it
  does, the only caller in a real run is `load.Load`, which checks before the
  marker row and before the first drop — so the target is not destroyed for a
  schema that cannot be recreated, but the refusal arrives after the snapshot
  has been used for keys, which §11.1's ordering exists to avoid. The two exit
  13 codes in `internal/event/catalogue.yml` still carry `stage: plan`, which
  is where the check belongs and where it must move.
- **`internal/pg` opens the transaction the `CatalogFingerprinter` runs in**
  (T-FPR, ADR-009). It used to call the fingerprinter with
  `&reader{conn: conn, own: false}` on a pooled connection in autocommit, where
  `introspect.Introspect`'s sampling `SAVEPOINT` is 25P01, so the obvious
  wiring (`Introspect` then `ddl.Fingerprint`) failed at the gate every time
  and §11.2's binding could never be confirmed;
  `TestLoadPagilaIntoAMarkedTarget` carried an `inTransaction` introspector
  that issued its own `BEGIN` — a test working around a defect. That wrapper is
  gone and the test hands `GateFingerprint` a bare `introspect.New()`, which is
  what `core` will pass. `GateFingerprint` still issues no `BEGIN` of its own,
  and must not: it would be ending a transaction it did not start.
- **Owed: a source table named `lazyslice_meta` is not recreated at all.**
  `ddl.recreated` skips it by name in any schema, so a source that really has a
  table of that name loses it in the target and its rows fail at `CopyFrom`
  with 42P01. Before this change the same source failed at `CREATE TABLE` with
  42P07, because `EnsureMarker` had already created the marker; neither is a
  refusal that says what happened. The gate is where a source owning the
  bookkeeping name should be refused, and it does not check.
- **Both ends of §11.2's binding are this package's, so they cannot be wired
  apart.** `ddl.Fingerprint` is over the DDL text, as §11.1 says, and ADR-009
  makes it the only definition: `internal/introspect`'s `schemaFingerprint`,
  which hashed the catalog fields that text is rendered from, is deleted, and
  `Introspect` now returns `Schema.Fingerprint` empty. **`internal/core` fills
  `pipeline.Schema.Fingerprint`, from `SchemaFingerprint`, right after
  introspection** (`internal/core/run.go`) — it is the caller that has both
  halves — and nothing else may fill it. Both ends here still compute their own
  value rather than reading that field: `Load` through `SchemaFingerprint`, the
  gate through `GateFingerprint`. That is what makes a `Schema.Fingerprint`
  which came from somewhere else unreachable from either end, whoever set it.
  What is not left to a caller's memory is which one each end uses: `load.SchemaFingerprint` is the single name, `Load` computes
  the marker's value with it rather than taking a `Run.SchemaFingerprint`, and
  `load.GateFingerprint(introspector)` is the `pg.TargetOption` `core` wires
  into the gate. A marker written with one definition and recomputed with the
  other binds nothing, which per §11.2 is exit 4 on a target lazyslice itself
  wrote with no way forward; the prose that used to say "core must use one of
  them for both ends" is now two functions instead.
  `TestLoadPagilaIntoAMarkedTarget` exercises the real wiring — fingerprint the
  source, load, re-introspect the target, require the two hashes equal, and
  require the gate reached through `GateFingerprint` to report `MarkerBound`
  against the real source ref — so the round trip §11.1 claims is a test and
  not a sentence — and it does so through the wiring `core` will use, with no
  transaction of the test's own. The two properties that made the catalog hash
  fail to round trip are asserted as unit tests here rather than left to the
  integration suite: `TestTheMarkerTableIsNotAnObjectClass` for
  `lazyslice_meta`, and `TestFingerprintCountsOnlyEdgesBetweenRecreatedTables`
  for the edges — the catalog hash counted every non-virtual foreign key,
  including one onto a leaf partition the target does not have.
- **What `ddl.Recreatable` detects, and what it does not.** It detects a
  default, generated expression, check, exclusion or index definition whose text
  calls a function or procedure listed in `Schema.NotRecreated`, and a column or
  index using a collation listed there. It cannot detect a column of a
  user-defined base type (indistinguishable, from `*pipeline.Schema` alone, from
  a column of an extension's type, which *is* recreated), an index using a
  user-defined operator class (`pipeline.Object` has no such kind), or a domain
  check reaching a user function through an operator. Each needs a `pg_depend`
  field `internal/introspect` does not yet read, and each is reported as an owed
  item rather than passed silently.
- **No type registration, so a composite column or an array of a user type
  fails at `CopyFrom`.** Measured on postgres:16: an enum, a domain, a text
  array, `tsvector`, `interval`, `jsonb` and `bytea` all copy correctly without
  registration (an enum's binary form is its label; a domain is reported as its
  base type), while a composite fails 42804 and an enum array fails 54000 —
  loudly, in both cases. Registering the source's user types on each target
  connection is `internal/pg`'s `AfterConnect` and its own recorded debt; it is
  not reachable from `pipeline.Writer`.
- **Unlogged tables are recreated logged.** §11.1 says "Unlogged tables are
  recreated unlogged" and `pipeline.Table` has no `relpersistence` field for
  `internal/introspect` to fill. Owed there.
- **The RLS `INSERT` fallback ADR-005 names is not implemented.** Detecting
  row-level security on a *target* table needs a read this package cannot make.
  A target table with RLS cannot exist in v1 by construction (policies are not
  recreated); the case it was for is a marked target where a person added a
  policy by hand, and it is reported rather than guessed at.
- **The batch channel closing is not the stream finishing.**
  `internal/extract` closes the channel with `defer close(out)` on every path,
  its error returns included, so an extract that dies between tables reaches
  `copy` as an orderly end of channel: an early prefix of tables committed, the
  rest created and empty, every foreign key validating (an empty child
  satisfies any foreign key) and the marker about to say `complete` — which is
  THREAT_MODEL.md T8's "some tables loaded and status = complete" exactly.
  `copy` therefore takes the plan and requires every step whose `Mode` is not
  `SchemaOnly` to have delivered a batch marked `Last`; a missing boundary is a
  load failure, so the marker is closed `failed`.
  `TestAStreamThatEndsBetweenTablesIsALoadFailure` is the guard. This does not
  make the loader notice an extract error on its own — the error travels back
  through `core`, which joins the two — it makes the loader refuse to call a
  truncated stream a complete load.
- **A `Load` that fails writes `failed`; a `Load` that succeeds writes
  `complete` at the end of this stage, not the end of the run.** A verify
  failure afterwards does not rewrite the row, and that is safe: §11.2 has the
  gate treat `running` and `complete` identically — both authorise truncation —
  so the marker never says more than "a compatible lazyslice wrote exactly this
  here".

- **`GateFingerprint` reads the target's catalog without sampling it**
  (T-0053). It asks the introspector it was handed for
  `pipeline.SchemaOnlyIntrospector` and uses `IntrospectSchema` when it is
  offered, falling back to `Introspect`. This end hashes generated DDL, and
  nothing in `internal/load/ddl` reads `Table.Samples`, so the samples a full
  read takes are discarded — a `TABLESAMPLE` per table over rows in a database
  the gate has not yet agreed to touch, held in memory (THREAT_MODEL.md T4),
  inside the `REPEATABLE READ` transaction `internal/pg` keeps open around the
  call. The assertion is made here rather than at the call site because
  `internal/core` passes `introspect.New()` and should not have to know which
  read the gate wants; the fall-back is correct and only slower, since the
  fields a full read adds are fields `SchemaFingerprint` does not hash, so both
  spellings give the marker's end and this one the same value.
  `TestLoadPagilaIntoAMarkedTarget` is what shows the two ends still agree: it
  writes the marker from a full read of the source and binds it from this read
  of the target.
  **The branch itself is pinned by two unit tests**, because a type assertion
  that misses degrades silently: an introspector that reaches the gate wrapped
  — a decorator, a fake, a `core` refactor — satisfies `Introspector` and not
  `SchemaOnlyIntrospector`, the sampling read comes back, and
  `TestLoadPagilaIntoAMarkedTarget` passes exactly as before because both reads
  hash the same. `TestGateFingerprintAsksForTheSchemaOnlyRead` and
  `TestGateFingerprintFallsBackToTheFullRead` hand `gateFingerprint` fakes that
  record which method was called; `gateFingerprint` is a named function rather
  than a closure inside `GateFingerprint` only so that they can, since a
  `pg.TargetOption` hides the fingerprinter in an unexported field.

**Test.** `go test ./internal/load/...` for the DDL statement lists and the
transaction contract against a fake `Writer`; `go test -tags integration
./internal/load/...` for what a real target holds —
`TestLoadPagilaIntoAnEmptyTarget`, `TestLoadPagilaIntoAMarkedTarget`,
`TestLoadNastyResetsAMixedCaseSequence` — testdata/nasty.sql rather than
pagila, because pagila has no mixed-case sequence and cannot see a `setval`
whose `regclass` argument is unquoted — and
`TestKillNineLeavesEveryTableEmptyOrComplete`, which re-execs this test binary
as a child, kills it with SIGKILL mid-load and asserts that every table holds
none of its rows or all of them and that the marker is still at `running`. Do
not replace that child process with a cancelled context: a context runs the
deferred rollback, and what T8 is about is the kill that runs nothing.

**Never:** open a connection to the source; commit a table's transaction before
its `Last` batch; write a generated column; recreate a view, function, trigger,
policy or partition (v1 does not — §11.1's `not_recreated` list).
