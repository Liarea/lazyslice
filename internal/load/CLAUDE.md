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
  for keys and before anything in the target is dropped. That is where it is
  raised: `internal/core`'s `planStage` calls `ddl.Recreatable` before it builds
  the plan request (T-0097). `Load` does **not** check again — one refusal,
  raised once — so a caller that drives this package without `core.Run` (the
  integration suite) makes the call itself.
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
- **`ddl.Recreatable` is called by `core`, not here (settled, T-0097).** §11.1
  raises the not-recreatable refusal at plan; `internal/plan/plan.go`'s
  `checkRecreatable` covers `ForeignKey.NotRecreatable` and nothing else, and
  stage packages do not import each other, so the caller §11.1 describes is
  `core` — which did not exist when this package landed and does now.
  `internal/core`'s `planStage` makes the call before `planRequest` and before
  `Plan`, so the refusal costs one introspect and no key query, and the two exit
  13 codes in `internal/event/catalogue.yml` are raised at the `stage: plan`
  they already declared. `Load` no longer calls it: the check was here as a
  stand-in, and a second copy would be one refusal two stages could raise.
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
- **The source's user types are registered between the pre-data DDL and the
  first `CopyFrom`** (`registerTypes` in `load.go`, T-0083). Measured on
  postgres:16: an enum, a *scalar* domain, a text array, `tsvector`, `interval`,
  `jsonb` and `bytea` all copy correctly *without* registration (an enum's
  binary form is its label; a domain is reported as its base type), while a
  composite fails 42804, an enum array fails 54000 and an **array of a domain**
  fails 54000 too — loudly, mid-table, with T8's per-table transaction leaving
  the target empty or complete and the run failed anyway. The domain-array
  measurement is why domains stay in the registered set although a scalar domain
  column needs nothing: `internal/pg/types.go` records it beside the code.
  ARCHITECTURE.md §11.1 and ADR-005 always said the load registers types in
  `AfterConnect`; nothing did until T-0083, and `internal/pg/CLAUDE.md` had
  deferred it to "the task that builds `internal/load`", which is the task that
  shipped this package without taking it.
  **It is a hook on the target pool now, and only the target pool.** The
  registration itself is `internal/pg`'s (`writer.RegisterTypes`,
  `internal/pg/types.go`); this package reaches it through `pipeline.Writer`
  itself, whose fourth method it is since T-0093. The source pool still
  may never have an `AfterConnect` (T-0076), and `pg.Connect` refuses one.
  **The ordering is the whole of it.** Before item 3 the target has none of the
  source's types; after the first `CopyFrom` is too late. A registration that
  fails fails the load before any row moves, rather than surfacing later as a
  driver error that quotes a value (THREAT_MODEL.md T4).
  **A `Writer` that cannot register types no longer compiles** (T-0093), which
  is why `registerTypes` is four lines. It went through three shapes: a silent
  `return nil` on a missed `w.(pipeline.TypeRegistrar)` assertion, justified by
  this package's own fake; then a run-time refusal naming the writer's type,
  after a review pointed out that `internal/core` wraps this same writer in a
  `readableWriter` for verify — which embeds the `Writer` *interface* and so was
  not a registrar — so one refactor stood between a silent skip and a run that
  fails mid-copy on a composite column, in the integration suite and nowhere
  else; then the method on `Writer`, which is the only one of the three the
  compiler checks. The refusal and its `plainWriter` double and
  `TestALoadWhoseWriterCannotRegisterTypesIsRefused` are gone with the assertion
  that needed them; `fakeWriter` implements `RegisterTypes` and
  `registeringWriter` records *when* the loader called it, which is the part the
  compiler still does not check. ARCHITECTURE.md §2 owes the fourth method.
  `TestLoadRegistersTheSourcesUserTypesBeforeTheFirstCopy` and
  `TestALoadWhoseTypeRegistrationFailsCopiesNothing` are the rest of the unit
  half;
  `TestLoadNastyCopiesAnEnumArrayAndACompositeColumn` is the half against a real
  server, over `testdata/nasty.sql` trap 27, and it fails when the
  `registerTypes` call is removed (verified).
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
`TestLoadNastyResetsAMixedCaseSequence`,
`TestLoadNastyCopiesAnEnumArrayAndACompositeColumn` — testdata/nasty.sql rather than
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

## setval and an identity column's sequence (T-TORTURE)

`ddl.Setvals` addresses an **identity** column's sequence through
`pg_get_serial_sequence` on the target, coalescing to the source's quoted name,
rather than asserting the source's name. `sequences()` already skipped an
identity sequence when emitting `CREATE SEQUENCE`, because the column's own
`GENERATED ... AS IDENTITY` creates it — and the target names it itself, so the
two names differ as soon as the source's table has been renamed. Metabase renamed
`group_table_access_policy` to `sandboxes` and Postgres left the sequence behind
under the old name; `setval` then named a relation the target has never had and
the run died at `42P01` **after every table had been copied**, which is the
THREAT_MODEL.md T8 outcome the strict-NULL form exists to prevent
(`testdata/regressions/006-identity-sequence-renamed-table.sql`).
`internal/verify` resolves it the same way and the two have to stay in step.

The owed item above is closed. **One** of the ten schemas in `testdata/torture/`
reaches the not-recreatable refusal as the fixtures stand — Mastodon's
`timestamp_id` on nine primary keys, carried unedited for that reason. GitLab
reaches the same refusal *upstream* on two objects (`organizations.uuid`'s
`DEFAULT gen_random_uuid_v7()` and the index
`index_todos_coalesced_snoozed_until_created_at` on `timestamp_coalesce`), and
its 43-table subset drops both, so the fixture exits 0
(`testdata/torture/gitlab/README.md`, `docs/TORTURE.md`). From inside `Load` an
operator with either schema paid for a whole extract, holding the source
snapshot throughout, before being told the target could not be built. T-0097
moved the call into `internal/core`'s `planStage`, where §11.1 says it belongs;
`core.asStop` maps the refusal to its own code and exit 13
(`testdata/regressions/002-function-default-refusal-uncoded.sql`).

That leaves `Load` with an **unchecked precondition**: its caller must have run
`ddl.Recreatable` over the same `*pipeline.Schema` it passes, and nothing in this
package enforces it. `core.Run` does; `load_integration_test.go` calls
`ddl.Recreatable` over its fixture, but as a fixture assertion rather than a
guard on `Load`. A direct caller that skips it drops every table in the target
and then fails at `CREATE TABLE` with `42883` under exit 7 — the failure the
check used to prevent from here.

## Lock-and-recheck before every drop (T-0130)

`drop` no longer runs `DROP TABLE` on the writer in autocommit. Each table goes
through `dropOne`: one transaction that asks `to_regclass` whether the table is
there, takes `LOCK TABLE ... IN ACCESS EXCLUSIVE MODE NOWAIT` when it is,
re-verifies what the gate approved, drops, and commits. The order is the whole
of it — the lock first so the recheck is about a table nobody else can be
writing to, the recheck before the `DROP` so a refusal costs nothing, and both
inside the transaction the `DROP` commits in, so no moment passes between "still
as approved" and "gone".

- **Which question the recheck asks is which question the gate answered.**
  `Run.MarkerBound` false — the zero value — means an unmarked target approved
  for being empty, so the table must still be empty; true means a bound marker
  authorised the truncation, so `Run.MarkerRunID`'s row must still be there with
  `Run.MarkerStatus`. Asking the wrong one either refuses every reload (a marked
  target's tables are full by design) or approves a stranger's rows. **The
  fail-closed direction is the default**: a caller that says nothing gets the
  stricter check. That is why `load_integration_test.go`'s `loadInto` takes the
  gate's real `pipeline.Eligibility` for the two tests that reload a target this
  package wrote, and nothing for the ones that load into a fresh container.
- **The refusals are exit 4, not exit 7.** None of the three is a load that went
  wrong: each is ARCHITECTURE.md §9's target refusal arriving later, because the
  evidence arrived later. `Refusal.Rows` carries the count the message names; it
  is a number, and it is zero for every other refusal.
- **What a refusal undoes is one transaction, not the loop.** The transaction
  that refuses rolls back, so the table it refused about is untouched; tables
  `drop` already dropped in *earlier* transactions stay dropped, because those
  drops committed after their own lock-and-recheck. Nothing unauthorised is
  destroyed — an unmarked target's earlier tables were each verified empty under
  their own lock, a bound marker's truncation was authorised — but table
  definitions are gone and the target is left part-way through a rebuild, which
  the next run's gate refuses (its marker no longer binds against a half-dropped
  catalog, so the gate falls through to the emptiness check and prints the
  command that clears the database). The first wording of both amendments said
  "nothing was destroyed", which is a claim about the refusing transaction and
  not about the loop; both are corrected, and
  `internal/core`'s `TestARefusalOnTheSecondTableLeavesTheFirstDropped` is what
  holds them to it. **One transaction over every drop is deliberately not what
  this does**: the gate admits up to 2,000 user tables, `DROP ... CASCADE` takes
  a lock per dependent index, sequence and toast relation, and one transaction
  over all of them meets `max_locks_per_transaction` instead of finishing.
- **Only SQLSTATE 55P03 is a contended target.** `sqlTableExists` asks
  `to_regclass`, which answers non-NULL for any relation kind, so the `LOCK
  TABLE` can also raise 42809 ("is not a table" — a sequence or an index under
  that name), 42501 or 42P01 (the relation went away between the two
  statements). Turning every one of those into `CodeRefusedTargetLocked` told an
  operator to go and find who is using the database, after a third of a second
  of retry, for a load failure that is exit 7. `lockNotAvailable` (codes.go) is
  the discrimination and `TestALockFailureThatIsNotContentionIsALoadFailure` is
  the guard.
- **`lockAttempts`/`lockRetryPause` are three attempts a tenth of a second
  apart.** NOWAIT is the point — a `DROP` that waits can sit behind an
  application's long transaction, and waiting also lengthens the window this
  mechanism exists to close — but the lock a freshly loaded target most often
  loses a race to is autovacuum's, which a *waiting* `DROP` would have cancelled
  automatically and a NOWAIT one simply fails against. A third of a second is
  still bounded, and it is the difference between an honest refusal and a coin
  toss.
- **`ddl.TableName` is exported for this.** The `LOCK`, the recheck and the
  `DROP` must name one relation; two spellings of a quoted identifier are two
  things to keep in step.
- **`pipeline.Tx` grew `Query` for this and for nothing else.** The recheck has
  to read between `LOCK TABLE` and `DROP TABLE`, inside that transaction; a read
  anywhere else answers a question about a moment that has already passed, which
  is the defect this closes.

- **The three refusals are unit-tested against a programmable fake, because the
  fake's `Query` used to answer "no such table" unconditionally.** Every unit
  test therefore stopped short of `recheck`, and a `recheck` stubbed to return
  `nil` passed all of them; the only real coverage was the unmarked/empty branch,
  through `internal/core`'s race suite. `fakeWriter.answer` and `recheckAnswers`
  (load_test.go) are how a test says what the target replies, and the tests drive
  `dropOne`/`dropTable` directly rather than `Load`, because a fake that claims
  to hold tables would have to keep pretending for the rest of a run.
  `TestADropRefusesWhenATableApprovedEmptyHasRows`,
  `TestADropRefusesWhenTheMarkerRowThatAuthorisedItIsGone`,
  `TestADropRefusesWhenTheMarkerRowChangedStatus`,
  `TestADropRefusesWhenTheRunNamesNoMarkerRow` (the `MarkerRunID == ""`
  fail-closed arm, which nothing in the tree reaches because `core` fills all
  three fields together), `TestADropProceedsWhenTheMarkerRowIsStillAsApproved`
  (the other direction, so a stub returning `nil` fails),
  `TestALockThatIsNotFreeIsRetriedAndRefused` and
  `TestALockFailureThatIsNotContentionIsALoadFailure`.

Why: docs/reviews/2026-09-09 finding 1 inserted a row into an empty unmarked
target after the gate approved it, and the run deleted the row and exited 0.
`internal/core/race_integration_test.go`'s
`TestARowInsertedAfterTheGateIsNotDropped` is that script as a regression, and it
fails the old way — exit 0, row gone — with the recheck removed.
`TestAMarkerDeletedAfterTheGateIsNotTruncated` is the same instrument on the
marked-target branch — the production reload path — deleting the gate-approved
marker row between the gate and the load and asserting exit 4
`load.refused.marker_changed` with the previous run's rows still in place. The residual is
in THREAT_MODEL.md T2: a writer that inserts *after* the drop commits is blocked
by the lock until then and afterwards writes into the fresh table, which is the
application behaving normally.

`internal/event/catalogue.yml` carries the four new rows; **docs/ERRORS.md is
generated from it and was outside this task's paths**, so `make docs-check` fails
until someone runs `make docs` and commits the result — tracker **T-0148**.

## The quarantine drops objects, not only tables (the 2026-09-15 red team's A07)

`DropLoaded` is THREAT_MODEL.md T8's promise that after a content-class verify
failure the target ends the run "either empty or holding nothing this run
wrote". It iterated `ddl.DropTables` and nothing else, so the promise was true
of tables and false of every other object this package creates. The red team put
the address in a **domain's `CHECK`** rather than a table's, watched
`internal/verify`'s catalog pass refuse the run at exit 9 naming it, watched the
quarantine drop the tables, and read the domain straight back out of the
target's `pg_constraint` — exit 9 again on every rerun, with the object still
there.

`ddl.ObjectDrops` is `DropTables`' `TableDrop` for the object classes that are
not tables — the sequences and types `PreData` creates, each with its name so
the drop can be announced before it happens (`DropObjects` is now a thin
spelling of it, for the reload path that only needs the SQL). `DropLoaded` runs
them **after** the tables, because there is no `CASCADE` here and a type
something still depends on after every table this run knows about is gone is
something the run has not been told about — a loud `2BP01` naming it beats
dropping a stranger's column. A failure joins the others rather than stopping
the sweep, the same as a table's.

`dropLoadedObject` takes no lock ahead of its statement, unlike
`dropLoadedTable`: there is no `ACCESS EXCLUSIVE` lock to take `NOWAIT` on a
type or a sequence, and `DROP TYPE` on an object nothing references does not
block, so the lock-contention case there is no retry to make here.

Each object is announced under its own code, `load.target.quarantine_dropping_object`,
rather than reusing the table one: a transcript that named only the tables was
the evidence for a promise this call was not keeping.

What is still owed: `internal/verify` does not re-read the target to confirm the
object it refused over is actually gone before the run returns — **T-0183**.