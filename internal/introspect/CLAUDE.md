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
- Sampling is `TABLESAMPLE SYSTEM (p) REPEATABLE (seed)` for ~200 rows. `LIMIT`
  is never the sampling *method* — it would take the front of the table — but a
  `LIMIT` after `TABLESAMPLE` is a stop on what the server reads, and the
  statement carries one (see the sampling decisions below). A partitioned root
  is sampled through its largest leaf by `reltuples`; `Table.SampledFrom` names
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

**Decisions made during implementation.**
- **`Shapes()` returns `introspect.Statement`, not `pg.Shape`.** Every statement
  this package sends is a constant in `sql.go`, and the source allowlist has to
  carry all of them or the run is a refusal storm (THREAT_MODEL.md T9). The
  import graph has the stage packages importing `pipeline` and nothing else of
  the tree, so the shapes are exported as `{Name, SQL}` pairs and the wiring
  that owns both sides — `internal/core` — converts them. `Introspect` takes a
  `Reader` and has nowhere to register anything itself.
- **The sample fraction is written `({int}::float8 / {int})`.** ARCHITECTURE.md
  §4 wants `p` chosen for about 200 rows, which is a fraction of a percent on a
  large table, and `internal/pg/tracer.go`'s template grammar has `{int}` and no
  float placeholder. A numerator over a denominator is one shape that covers
  every table; the alternative was widening the grammar, which is that
  package's file and its review.
- **Sampling constants.** `SampleRows` is 200 (§2). The requested fraction asks
  for three times that, because `TABLESAMPLE SYSTEM` picks whole pages and
  asking for exactly 200 returns fewer as often as not; the first 200 rows are
  kept and the rest dropped client-side. `REPEATABLE (4242)` is a constant, not
  a random seed and not derived from the masking key, because two runs over one
  snapshot must classify identically (ADR-004). Stopping at the cap does not
  consult `Rows.Err`: the statement did what it was asked to.
- **The sample fraction is a fraction of pages, and the statement carries a
  `LIMIT`.** `TABLESAMPLE SYSTEM` selects whole pages, so `reltuples / relpages`
  gives rows per page, that says how many pages hold the oversampled target, and
  that count over `relpages` is the fraction. Where the two estimates agree it is
  the row fraction; where they do not — a table analysed at ten million rows and
  since deleted down to a few thousand — the page count is what decides how much
  is actually read. `reltuples` of -1 (a relation nothing has analysed, so a
  freshly restored dump) is carried as 0 and `relpages` is 0 with it: there is
  nothing to derive a fraction from, so the statement asks for 100% and the
  `LIMIT` is the bound. The `LIMIT` is not decoration and the client-side
  `SampleRows` cap does not replace it: pgx drains the rest of a result set on
  `Rows.Close`, so breaking out of the loop bounds memory and neither the server
  nor the wire. Measured on postgres:16 over a 500,000-row table of `(int, md5
  text)`, `EXPLAIN (ANALYZE, BUFFERS)`: 5 buffers and 600 rows with the `LIMIT`,
  4,224 buffers and 500,000 rows without — the difference between a bounded read
  and reading a multi-hundred-GB table in full while the run holds the source
  snapshot (THREAT_MODEL.md T7).
- **A fraction below 100 that returns nothing is widened once, by ten.** Stale-high
  statistics put the fraction on pages that hold nothing, and a table with no
  samples classifies on its name and type alone — testdata/README.md trap 20
  (`people.ref`, a column whose name says nothing and whose values are all email
  addresses) landing at `none` and being copied in cleartext, caught only by
  verify's second net after the target already holds it. The retry's rows are the
  front of the table rather than a random sample, which is the lesser evil.
  **The widening is bounded in pages, not in rows.** The `LIMIT` bounds the wire
  and the client and nothing else: a retry at 100% is a sequential scan the
  server ends only once it has accumulated its 600 live rows, so on the very case
  the retry exists for — ten million rows of statistics over a million pages
  holding a few thousand live rows — it reads hundreds of thousands of pages, and
  on a bloated relation holding none it reads the whole relation, all while the
  run holds the source snapshot and pins the xmin horizon (THREAT_MODEL.md T7; a
  standby cancels the query after `max_standby_streaming_delay`). The retry
  therefore asks for ten times the pages the first fraction asked for, and for
  100% only where the whole relation fits inside that budget. A table whose few
  live rows are hidden in a large bloated relation still comes back with no
  samples and classifies on its name and type alone, which is the outcome an
  unreadable table already has; `pipeline.Table` has no field to say "empty
  despite non-zero relpages", so nothing announces that case — a gap in §2 rather
  than in this package.
- **A table the source role cannot `SELECT` leaves `Table.Samples` empty and the
  run continues**, and it takes two mechanisms rather than one.
  ARCHITECTURE.md §3.6 answers an unreadable table at plan: one that is
  unreachable or child-only is dropped to `SchemaOnly` and the run goes on, an
  unreadable parent or root stops there with exit 12 naming the table, the role
  and the `GRANT` to run, and "There is no mid-extract permission failure by
  design". Introspect runs before plan and has no privileges input, so a sample
  that failed the run here would make both outcomes unreachable and hand the
  developer a wrapped 42501 instead of the command that fixes it.
  First, `sqlTables` reads `has_table_privilege(c.oid, 'SELECT')` — the same
  predicate `RolePrivileges.Unreadable` is built from — and the sampler sends no
  statement to a relation that says false. A leaf the role cannot read is not a
  sampling candidate either, so a partitioned root whose largest leaf is
  unreadable is sampled through the largest readable one rather than not at all. Asking beforehand rather than
  discovering by failing is what keeps a production source's log clean and the
  trace free of doomed statements, and it is the same shape of answer §3.6 gives.
  Second, a 42501 that arrives anyway — the privilege was revoked between the
  catalog read and the sample — is rolled back to a savepoint the sampler takes
  before its loop, and the loop goes on. **The savepoint is not optional**:
  Introspect reads the whole catalog in one transaction, so tolerating a refusal
  without one carries the run into an aborted transaction and it dies with 25P02
  naming an innocent table. That is not a hypothesis — the first version of this
  fix did exactly that on postgres:18, which is why
  `TestIntrospectSurvivesATableTheRoleCannotRead` sets up a role and revokes
  `SELECT` rather than asserting the branch in a unit test alone. Only SQLSTATE
  42501 is tolerated: swallowing every error would turn a systematically broken
  sample statement into a silent name-and-type-only classification of the whole
  database, which is THREAT_MODEL.md T1. `SAVEPOINT` and `ROLLBACK TO SAVEPOINT`
  are two more shapes on the source allowlist; both are fixed text with no
  placeholder and neither reads or writes a row. There is no `RELEASE`: the
  savepoint costs nothing on a read-only transaction that is about to end.
- **`Table.Parent` is the topmost ancestor, not the immediate one.** §2 says
  "for a partition, its root", and the root is the table the planner and the
  loader address (testdata/README.md trap 7). A partition of a partition
  therefore names the top of the tree.
- **`Table.PartitionKey` is parsed out of `pg_get_partkeydef`**, not read from
  `pg_partitioned_table.partattrs`: `partattrs` carries a zero where a key member
  is an expression, and a query that joined it to `pg_attribute` would drop that
  member without saying so. The deparsed text is split at top-level commas, so
  `LIST ((a + b), c)` is two members and `RANGE ("Odd, Name")` is one.
- **A sequence reaches `Table.Sequences` two ways.** Owned (`pg_depend` deptype
  `i` for an identity column, `a` for `OWNED BY`) sets `SequenceDef.Column`;
  referenced only by a column's `nextval` default leaves it `""`, which is what
  §2 says of a sequence `pg_get_serial_sequence` does not resolve. pagila
  v3.1.0 declares no `OWNED BY` at all, so reading ownership alone would drop
  every sequence in that fixture and leave the target's defaults calling
  sequences that do not exist. A sequence that is neither owned nor referenced
  has no table to hang from and no field in `pipeline.Table` to hold it; it is
  not represented, and that is a gap in §2 rather than in this package.
- **`Schema.NotRecreated` uses exactly the kinds `pipeline.Object` enumerates.**
  §11.1's prose also lists partitions as not recreated; `Object`'s own list does
  not carry a `"partition"` kind, and partitions are already named in
  `Table.Partitions`, so they are not counted here. Named because the two
  sections disagree.
- **`Schema.Extensions` is narrowed through `pg_depend`, as §2 and §11.1 item 2
  both say** — "every extension a recreated column type, default, index or
  operator depends on" — and is not the installed list. A superset is not free in
  either direction: §11.1 item 2 turns each entry into `CREATE EXTENSION IF NOT
  EXISTS` on the target, so an extension the target cannot create (pg_cron,
  pgaudit, pg_stat_statements, postgis on an image without it) fails the load at
  DDL after §11.1 has already dropped the target's tables; and
  `Schema.Fingerprint` hashes the list, so an extension the source has and the
  target does not would stop §11.2's marker from ever binding. The walk is one
  hop of `pg_depend` from the recreated objects — tables, their defaults,
  constraints and indexes, and the enums, domains and composites of a user schema
  — then one further hop through `pg_type` for an array's element type and a
  domain's base type, because `pg_depend` records a `citext[]` column's
  dependency on `_citext` while extension membership is recorded on `citext`.
  **A user type enters the walk under three object identities, because Postgres
  files its dependencies under three.** The `pg_type` row carries a domain's base
  type; a composite's *fields* are attributes of its `pg_class` relation, so they
  are recorded as `(pg_class, typrelid, attnum)` and a walk that knew only the
  `pg_type` row missed them entirely; and a domain's `CHECK` constraints are
  `pg_constraint` rows with `conrelid = 0` and `contypid` set, which the
  `conrelid` branch cannot see. Both holes had the same shape as the one this
  narrowing was written for, arrived at from the other side: `Schema.Composites`
  still carried `CREATE TYPE public.addr AS (email citext, note text)` while
  `Schema.Extensions` was empty, so §11.1 item 2 would create no extension and
  item 3 would fail with "type citext does not exist" — after item 1 had dropped
  every user table in the target.
  Neither fixture installs an extension, so the suite asserts "empty" rather than
  exercising the walk; it was checked by hand on postgres:16 with citext, hstore
  and dblink installed, one database per case — a `citext` column, a `citext[]`
  column and a `citext` domain each return citext and nothing else; an `hstore`
  column returns hstore; a composite with a `citext` field, a composite with a
  `citext[]` field and a domain whose `CHECK` calls `hstore()` each return the
  extension they need (all three returned nothing before this change); and
  dblink, installed and unused, never appears in any of them. A fixture that
  installs an extension is owed, and until there is one this query is verified by
  hand and not by CI.
  **A view's row type is `typtype` 'c' as well, and must not enter the walk.**
  Postgres gives every relation a composite type, so a view and a materialised
  view each have a `pg_type` row of `typtype` 'c' whose `typrelid` is the view;
  the composite branch above matched them and pulled `(pg_class, view oid)` in,
  and a view's columns are attributes of that relation, so every extension a
  view alone used was collected. The branch therefore carries `relkind = 'c'`,
  exactly as `sqlComposites` already does — a *standalone* composite type and
  nothing else. This is not cosmetic in either direction: §11.1 recreates no
  view, so the extension may be one the target cannot create and the load fails
  at DDL after item 1 has dropped its tables; and `Schema.Fingerprint` hashes
  the extension list, so a target lazyslice wrote — which holds no views —
  could never fingerprint equal to its source, the marker would never bind
  (§11.2), and every second run against that target would be refused with exit
  4. `TestAnExtensionUsedOnlyByAViewIsNotCollected` is the integration case: a
  `text` column with a view and a materialised view casting it to `citext`
  returns no extension, while a `citext` column in a second database on the
  same server still returns citext.
- **An extension's own enum, domain and composite types are excluded**, the same
  way its tables, functions and operators already were. §11.1 item 2 creates the
  extension before item 3 creates the types, so recreating one of them is a
  42710 against a target whose tables have already been dropped — `CREATE
  EXTENSION dblink` ships `public.dblink_pkey_results`, and PostGIS ships
  several. `sqlEnums`, `sqlDomains` and `sqlComposites` carry the same
  `extensionFilter` on `pg_type` that `sqlTables` carries on `pg_class`; checked
  by hand on postgres:16, where `CREATE EXTENSION dblink` puts
  `public.dblink_pkey_results` in `pg_type` and the filtered statement returns
  only the composite the fixture declared.
- **`Table.Constraints` carries only the five kinds §2 enumerates** (`p`, `u`,
  `c`, `f`, `x`). PostgreSQL 18 stores a column's `NOT NULL` in `pg_constraint`
  as contype `n`: unfiltered, nasty.sql yields 43 constraints on 14 and 16 and
  122 on 18. The same logical schema would then fingerprint differently per
  major, so a PG16 source loaded into a PG18 target could never bind its own
  marker (§11.2), and §11.1 item 5 would emit PG18-only
  `ADD CONSTRAINT ... NOT NULL a` duplicating the column's own `NOT NULL`. The
  integration suite asserts the counts by kind on both fixtures, so the
  divergence fails as a test rather than as a load.
- **An index is reported only when it is live, valid *and* ready.** A failed
  `CREATE UNIQUE INDEX CONCURRENTLY` leaves `indislive` true with `indisvalid`
  and `indisready` false, and `pg_get_indexdef` still prints
  `CREATE UNIQUE INDEX` while the table genuinely holds duplicates (verified on
  postgres:16). §3.4's identity ladder takes the first non-partial,
  non-expression unique index as row identity and has no way to tell that one
  apart, so it would become the key for a table where it identifies two rows at
  once — testdata/README.md trap 12 arrived at through the catalog instead of a
  guess. Such an index enforces nothing, so it is not a key; whether §11.1 item 6
  should nonetheless recreate it is a separate question and today it does not,
  because it is not in `Table.Indexes` at all.
- **`Column.Fingerprint`** is the first eight hex characters of `sha256` over the
  §5 length-prefixed encoding of `(TypeOID, TypMod, Nullable, Domain)` — the
  four fields §2 names, and no others, so `Column.Checks` changing does not
  expire an `--unmask` opt-out.
- **`Schema.Fingerprint`** is a full `sha256` in hex over a canonical rendering
  of §11.1's object classes 1 to 7, in that order: the schemas recreated tables
  live in, extensions, enums, domains, composites, sequences not owned by an
  identity column, tables with their columns and non-foreign-key constraints,
  then indexes and foreign keys. A leaf partition contributes nothing and
  neither does a partition key, because §11.1 recreates a partitioned source
  table as one plain table — the property §11.2's binding needs is that the
  source and a target lazyslice wrote hash alike. **A partitioned root's index
  enters as `plainIndexDef` of its definition**, for the same property:
  `pg_get_indexdef` prints `CREATE UNIQUE INDEX ev_pkey ON ONLY public.ev ...`
  for an index on a partitioned table and prints the same index on a plain table
  without `ONLY`, and `ON ONLY` is accepted at creation but is not round-tripped
  (verified on postgres:16). Hashing the raw text meant that any source holding a
  partitioned table with an index or a primary key could never equal its own
  target's hash: the marker would never bind, the gate would fall through to the
  emptiness rule, and lazyslice would refuse a non-empty target it wrote itself
  with exit 4 and no way forward — testdata/nasty.sql's `public.events` has
  `PRIMARY KEY (event_id, occurred_at)`, so the project's own fixture hit it on
  every second run.
  **This diverges from §11.1 and is provisional.** §11.1 hashes those classes "as
  their DDL text"; this hashes the catalog fields that text is rendered from,
  because `internal/load/ddl` (§12) is still the scaffold whose every function
  returns `ErrNotImplemented`, and a hash cannot be taken over text nothing
  produces. The two agree on *what* is hashed and not on
  *how*, which leaves a second definition of "the recreated schema" outside the
  package that owns the first. Resolving it is an ADR — either the hash moves to
  `internal/load/ddl` and takes the DDL text, with `Introspect` leaving
  `Schema.Fingerprint` for its caller to fill, or §11.1 is amended to say the
  catalog fields — not a decision this package may keep making alone. It is an
  owed item on T-INTROSPECT rather than a settled design.
  `TestSchemaFingerprintIsTheRecreatedObjectsOnly` asserts the half of the
  binding property that can be asserted today.
- **A generated column's expression is read from `pg_attrdef`** — that is where
  Postgres stores it — with `attgenerated` deciding whether it lands in
  `Column.Generated` or `Column.Default`. One column never carries both.
- **`Index.Columns` is nil for an expression index**, not a partial list.
  `indkey` carries a zero per expression member, and a partial list would read
  as a plain index on fewer columns, which §3.4's identity ladder would then
  accept as a key.
- **`ForeignKey.Indexed` is computed here, in Go**, from the child's indexes: an
  edge is covered when some index has the child columns as its leading key
  columns, in any order, which is the shape an equality lookup uses.
- **An edge in `Schema.FKs` can only name a table `Schema.Tables` describes, and
  never a partition unless it is marked `NotRecreatable`** (the exception is at
  the end of this entry). Two filters and one rewrite do it. `conparentid = 0` drops
  the copies Postgres clones onto every partition, without which `public.events`
  would contribute three copies of one edge and the planner would address leaves
  by name. Both ends are then filtered exactly as `sqlTables` filters the table
  list — `relkind` in ('r','p'), not owned by an extension — because an extension
  whose tables live outside a `pg_` schema (`_timescaledb_catalog`, pgq) and a
  foreign table are both absent from `Schema.Tables`, and an edge naming one is
  an edge the planner would walk to a table that is not there.
  **A partition end is then re-pointed at the root, not dropped**
  (`repointPartitionForeignKeys`). A foreign key *declared on* a leaf, and one
  *referencing* a leaf, both have `conparentid = 0` (verified on postgres:16), so
  neither is a clone: each is a constraint somebody wrote, over columns that live
  on the root. Dropping it silently lost a real dependency of the root's data —
  the planner never walked it, the parent rows were never pulled, and because the
  constraint was not recreated either the load succeeded on a referentially
  incomplete slice with nothing printed. The root is the table §11.1 recreates
  and the only one §3.3 lets anything address, so both ends are mapped through
  `Table.Parent`: the planner walks the edge, §11.1 item 6 recreates it against a
  table the target has, and the hash is taken over the same edge on the source and
  on a target lazyslice wrote. Re-pointing widens what the edge covers, because
  the root holds every partition's rows, so it pulls a superset of the parents the
  source needed and never a subset. A re-pointed edge that lands on one the
  schema already declares — the common shape, the same foreign key declared on
  every leaf — is collapsed into it: same child, same parent, same columns, same
  match type and the same validity is one constraint, and recreating it once per
  leaf would be several names for one check. Validity is part of that sameness
  because an unvalidated edge is a hint the planner does not follow (§3.1), so a
  copy the source has not validated must never absorb one it has. **Only a
  re-pointed edge is ever collapsed**, so a plain table that genuinely carries
  the same foreign key twice still carries both.
  Two constraints of *different* shape sharing a name on two leaves of one root
  is legal in the catalog (`pg_constraint` is unique per relation) and would
  reach the target as two `ADD CONSTRAINT` of one name, which Postgres refuses
  with 42710 — loud, at the end of the load, and preferable to dropping one of
  them here without saying so; no schema anyone has shown us is written that way.
  **pagila v3.1.0 is the fixture for this, and it is why the drop mattered**:
  every payment foreign key there is declared on a partition and none on the
  root, so six leaves times customer, rental and staff is eighteen edges with
  `conparentid = 0`, and dropping them left `public.payment` with no outgoing
  edge at all — no customer, rental or staff row was ever pulled for a payment.
  On the root they collapse to three.
  `TestIntrospectPagila/PaymentPartitionEdgesMoveToTheRoot` asserts that, and
  that no edge in the fixture names a partition;
  `TestAPartitionLocalForeignKeyMovesToTheRoot` covers the collapse and a leaf in
  the parent position, which pagila does not declare. testdata/nasty.sql declares
  neither, which is why the reviewers' "no fixture declares one" read true.
  **The parent end moves only onto a key the root actually carries** — the third
  consequence, beside the widening and the collapse. A foreign key needs a unique
  key over the columns it references, and a leaf can carry one the root cannot: a partitioned table's unique constraint must include every
  partition key column and a leaf's need not, so `ev_2024 UNIQUE (id)` is legal
  under a root partitioned by `at` whose own narrowest key is `(id, at)`. §11.1
  recreates none of a leaf's constraints or indexes, so re-pointing such an edge
  emitted an `ADD CONSTRAINT` that fails with "there is no unique constraint
  matching given keys for referenced table", at item 6 — after item 1 had dropped
  every user table in the target. So `hasKeyOver` asks whether the root carries a
  key over exactly the edge's `ParentCols` (as a set: Postgres matches them
  unordered); if it has not, the parent end stays on the leaf and the edge is
  marked `ForeignKey.NotRecreatable` for the planner to refuse at plan under
  §11.1 (exit 13), before anything is dropped. **A marked edge is the one place
  `Schema.FKs` may still name a partition**, which is why the "never a partition"
  rule above is stated of unmarked edges; its child end still moves, because that
  half is right however far the run gets.
  **A key here is a primary key, a unique constraint *or* a bare unique index**,
  because Postgres accepts all three as a referenced key and §11.1 item 6 creates
  every index before it adds any foreign key, so an index the source has is on
  the target by the time the `ADD CONSTRAINT` runs. Reading only the constraints
  was the first version of this and it was too narrow in a way that costs the
  user the run: a root partitioned by `at` carrying `CREATE UNIQUE INDEX
  ev_id_at_uidx ON ev (id, at)` and no unique *constraint* declares zero `p`/`u`
  rows in `pg_constraint` (verified on postgres:16), so an edge referencing it
  was marked and refused with exit 13 — whose remedy path §11.1 defers to a later
  epic — on a schema that would have recreated and loaded. The question is
  therefore asked of `Table.Indexes` rather than of `pg_get_constraintdef`'s
  text, which spells the same thing three ways once `NULLS NOT DISTINCT` and
  quoted identifiers are in it: a primary key and a unique constraint each have a
  backing index under the constraint's own name, so one scan answers all three
  cases. Partial and expression indexes are excluded, because Postgres refuses
  those as a referenced key; an index that is not live, valid and ready is
  already absent from `Table.Indexes` (see the index entry above).
  **A DEFERRABLE key is excluded too, and that is why `pipeline.Index` carries
  `Immediate`.** Postgres refuses one on the referenced side —
  `REFERENCES pl (id, at)` against `UNIQUE (id, at) DEFERRABLE` is
  `ERROR: cannot use a deferrable unique constraint for referenced table "pl"`
  (verified on postgres:16) — while its backing index is unique, non-partial,
  non-expression, valid, live and ready, so nothing but `pg_index.indimmediate`
  tells it apart from a usable key. A source may legally hold the shape that
  makes this bite: a root partitioned by `at` with `UNIQUE (id, at) DEFERRABLE`,
  a leaf carrying its own non-deferrable `UNIQUE (id, at)`, and an edge
  referencing the leaf, which the source accepts because of the leaf's key.
  Without the guard the edge was re-pointed onto the root and left unmarked, and
  §11.1 item 6 then failed with that error — after item 1 had dropped every user
  table in the target, which is the failure `NotRecreatable` exists to prevent.
  The primary key is answered through its backing index for the same reason
  rather than from `Table.PK`, which carries column names and cannot say whether
  the constraint over them is deferrable.
  `TestAPartitionLocalForeignKeyMovesToTheRoot` carries six edges, one per
  answer: the root's `PRIMARY KEY (id, at)`, its `UNIQUE (owner_id, at)` and its
  bare `ev_code_at_uidx` each move, while the leaf's own `UNIQUE (id)`, the
  root's *partial* unique index over `(tag, at)` and its *deferrable*
  `UNIQUE (kind, at)` are marked and left on the leaf. The four that must differ
  do differ: reducing `hasKeyOver` to `Table.PK` fails the constraint and
  bare-index edges, dropping the partial guard fails the partial one, and
  dropping the `Immediate` guard fails the deferrable one (checked by mutating
  the function and re-running).
- **Introspect emits no events**, because `Introspector.Introspect` takes no
  `event.Sink` (§2). It therefore adds no rows to
  `internal/event/catalogue.yml`; the five rows added alongside this package are
  `internal/pg`'s, which that task could not write.
- **The user-schema filter is spelled with `left(nspname, 3)`**, not with a
  `NOT LIKE` pattern, so that no statement this package sends carries a
  backslash: `internal/pg/tracer.go` records a statement containing one as its
  leading keyword alone, on the grounds that its literals cannot be shown to
  have been elided.

**Test.** `go test ./internal/introspect/...`; catalog-shape correctness needs
`go test -tags integration ./internal/introspect/...` against the fixtures in
`testdata/`.

**Never:** print, log, or copy a sample value out of `Table.Samples`; issue
`CopyFrom`/`SendBatch`/`Prepare` on the source; invent DDL text instead of
using the catalog's deparser.
