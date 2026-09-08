# internal/plan

The subset planner: the client-side monotone FIFO worklist with provenance
tags. Row identity fallback, root-table default, unreadable-table handling,
SCC/topological load order, budget checks, and the printed estimate. No SQL
pushdown, ever, and no extraction — this package decides which keys, not
which rows' bytes.

**Contract.** ARCHITECTURE.md §3 (the algorithm in full) and §2 "plan":
`Planner.Plan(ctx, Reader, *Schema, *Classification, PlanRequest) (*Plan,
error)`.

**Rules.**
- Determinism is load-bearing, not a nicety: FIFO queue, tables in
  `(schema, name)` order, edges in constraint-name order, every `KeySet`/
  `Chunk`/SQL result in identity-column order, never a Go map iteration.
  `TestPlanIsDeterministic` checks two runs over one snapshot produce
  byte-identical `selected` sets — this is what makes `lazyslice.yml` "a
  record of what happened" (ADR-004).
- A row's mode (`ChildOK`/`ParentOnly`) is fixed the first time it is popped
  and never revisited — this is both the size control and the privacy control
  (THREAT_MODEL.md T11, the Jailer #126 case).
- `RowBudget`/`MemoryBudget` are checked during the walk, not after
  (THREAT_MODEL.md T11): exceeding either is exit 11 naming the table.
- `internal/pipeline.KeySet.Bytes()` is the single source of truth for memory
  estimates — do not compute a separate estimate here, and do not add storage
  the two `Bytes()` formulas do not describe. A key set holds a `[]int64` or a
  slab and a span index, and nothing else (`keyset.go`); a dedup map beside
  them would make the budget's number smaller than the process's.
- **Both key sets implement `Chunks(n)`, `EachChunk(n, f)` and
  `FirstChunk(n)`** (T-0050), and the difference between them is memory, not
  taste: a `Chunk` holds its *own* copy of the keys it carries (a fresh typed
  array per identity column), so `Chunks` materialises a second copy of the
  whole set and the other two do not. `EachChunk` is what `internal/extract`
  walks a keyed step with and `FirstChunk` is what `internal/verify`'s sample
  compare takes; this package's own walk still uses `Chunks`, where the chunks
  are consumed inside one function and the set is being built rather than
  streamed. A third method that materialises the set is not the way to add a
  new caller.
- Every statement is bounded. No count, no aggregate and no sample reads a
  whole table inside the holder transaction (THREAT_MODEL.md T9): the lookup
  count stops at 1,001 rows, the pseudo-key probe takes a `TABLESAMPLE` or a
  bounded prefix, the `--key` probe is gated on `reltuples` and falls back to a
  prefix, and the seed keeps its `LIMIT` under `--where`.
- Unreadable tables are resolved via `RolePrivileges.Unreadable` before any
  key is fetched (§3.6); a parent the role cannot read is exit 12, never
  silently dropped.

**Test.** `go test ./internal/plan/...` for the cases no fixture can carry (the
exit-13 refusal, and the exit-12 write-back refusal in `writeback_test.go`),
then `go test -tags integration ./internal/plan/...` for
the suite that matters: `plan_integration_test.go` runs the walk against both
fixtures, named for the `testdata/README.md` traps it covers, and asserts that
two runs over one snapshot produce byte-identical plans.

**Never:** push SQL predicates down to the source instead of walking client-side;
let a table's mode change after its first pop; skip a budget check to "plan
faster"; iterate a Go map where output order matters.

## Decisions made during implementation

ARCHITECTURE.md §3 is the specification. Where it is silent, or where §2's
signature does not carry what §3 asks for, these are the choices made and the
reason for each.

- **Statement shapes are registered** (`shapes.go`, `Shapes()`), and the
  integration suite plans through a real `pg.Source` with them on its
  allowlist. This closed the gap an earlier version of this file recorded:
  `internal/pg`'s grammar had `{ident}`, `{idents}`, `{int}` and `{snapshot}`
  only, and the planner's statements — built per table and per key arity — were
  not expressible in it. The four placeholders that closed it live in
  `internal/pg/tracer.go`, where they are reviewed once: `{selectlist}`,
  `{casts}`, `{keypred}` and `{where}`.
  - **A clause the planner sometimes omits is its own shape**, not an optional
    part of one: `plan.seed` and `plan.seed_where`, `plan.map_keys` and
    `plan.map_keys_not_null`, and a bounded and an unbounded form of each key
    probe. A template cannot say "this clause may be absent", and the direction
    that fails safe is the narrow one — a shape too narrow refuses a statement
    of ours and `shapes_test.go` catches it, where a shape with an optional
    `LIMIT` would admit the statement that lost its bound (THREAT_MODEL.md T9).
  - `shapes_test.go` builds a statement of every shape with the real builders
    in `sql.go` and runs it through a real `pg.Tracer`, so a clause added there
    and not added here fails a unit test rather than the first run against a
    production source. It is a unit test and needs no database; the import of
    `internal/pg` is a test-only edge, the one `internal/introspect`'s
    integration suite already has.
  - **The second layer is a property of the composed allowlist, not of this
    file.** A `Source` carries one tracer and every stage registers into it
    additively, so another stage's shape that admits an unbounded read admits
    it for our statements too — which is what a table-agnostic
    `SELECT ... FROM t ORDER BY ...` in `pg.ExtractShapes()` did to the
    planner's bounded root read until it was given a `LIMIT` of its own. A
    tracer over `Shapes()` alone cannot see that, so
    `TestTheComposedAllowlistStillRefusesAnUnboundedRead` compiles the union of
    `Shapes()`, `pg.SourceShapes()` and `pg.ExtractShapes()` and asserts the
    unbounded seed is still refused. That test, not
    `TestStatementsOutsideTheGrammarAreStillRefused`, is what says what holds at
    run time.
- **`--where` is checked where it arrives** (`where.go`, `checkWhere`, called
  first in `run.plan`), against the same characters and the same parenthesis
  balance `internal/pg`'s `{where}` admits, and a predicate that breaks the rule
  is `plan.refused.where_syntax` at exit 2 naming the character and its
  position. The tracer stays the backstop it is meant to be. Leaving it as the
  only check cost two things: an ordinary predicate carrying a backslash or a
  dollar sign (`email ~ '^\w+@example\.com$'`) reached the operator as "the
  source refused a statement: statement does not match any registered shape",
  which names neither `--where` nor the character — and documentation is not
  the fix for that (root CLAUDE.md) — and the refusal incremented the tracer's
  violation count, so an operator's typo was recorded in the trace and the
  report as a T9 allowlist violation, indistinguishable from a statement one of
  our own bugs generated. The rule is duplicated rather than imported because
  §2's import graph has the stage packages importing `pipeline` and nothing else
  of the tree; `TestTheWhereCheckAndTheShapeAgree` runs both over one list of
  predicates so the two cannot drift.
- **Privileges are read here.** §3.6 consults `RolePrivileges.Unreadable`, and
  `Planner.Plan`'s signature (§2) has no privileges argument and cannot gain
  one from this package's paths. The planner therefore asks the catalog the
  same question itself (`sql.go`, `sqlUnreadableTables`) and reads
  `current_user` for the `GRANT` the refusal prints. `internal/pg`'s
  `Source.Privileges` answers the same question with a different schema filter,
  so the two can disagree about a table in an odd schema; the right shape is
  one query feeding the other through the request, which needs a field on
  `pipeline.PlanRequest`. The query here includes partition leaves and
  attributes an unreadable leaf to its root, because §3.3 fetches a partitioned
  table's keys from the root and lets Postgres route to the leaves: the refusal
  names the root the slice needs and the `GRANT` names the leaf.
- **"Needed as a parent" is decided on the graph, not on the rows.** §3.6 wants
  the answer before a key is fetched, so `staticReach` walks the table graph
  under the same mode and depth rules as the row walk. It over-approximates: a
  table no selected row would reach is still "needed", which fails towards a
  refusal naming a `GRANT` rather than towards a mid-extract permission error.
  The same computation answers "`--skip-table` cannot skip a parent". The edge
  rules themselves are not duplicated: `followsAsParent` and `followsAsChild`
  are the only statement of them and both walks call them, so the two can
  differ in scope (`staticReach` runs before the drops) and in nothing else.
- **Polymorphic pairs are inferred and followed in the parent direction**
  (`polymorphic.go`, §3.2, T-POLY). Detection is unchanged: `<x>_type`/`<x>_id`
  and `content_type_id`/`object_id`, minus any pair whose id column a declared
  foreign key already covers. What landed on top of it is the mapping half —
  the distinct `_type` values are sampled over a bounded, repeatable sample,
  each is mapped to a table (Rails: demodulize, underscore, pluralise; Django:
  the `django_content_type` row the id names, whose default table is
  `<app_label>_<model>`), and each mapping becomes one `pipeline.ForeignKey`
  with `Virtual` set, in `Plan.Virtual`.
  - **The inferred edges do not travel through `p.outgoing`, and
    `followsAsParent` still refuses a `Virtual` edge.** §3's pseudo-code writes
    the parent step as `fk.Validated ∨ fk.Virtual`, and this is the deviation:
    a `pipeline.ForeignKey` has nowhere to carry the discriminator, and an
    inferred polymorphic edge without its `_type` guard is
    research/COMPLAINTS.md FK-10 — Greenmask's #396, the polymorphic reference
    whose generated predicate lost its guard and selected almost nothing.
    `virtualParents` is therefore its own step beside the declared parent loop,
    reading the pair's two columns together, and `followsAsParent`/
    `followsAsChild` stay the rule for the declared graph alone. A `Virtual`
    edge arriving on `pipeline.Schema` from anywhere else (`Config.VirtualFKs`
    is declared and unwired) is still followed in neither direction, because it
    carries no discriminator either.
  - **The discriminator is applied client-side, not in the statement.** The
    walk reads `SELECT DISTINCT owner_type, owner_id FROM child JOIN chunk ...`
    — the ordinary `plan.map_keys_not_null` shape with both columns of the pair
    in its select list — and splits the result by value in Go. One read answers
    every value of a pair, and no `_type` value is ever written into a
    statement: `{keypred}` in `internal/pg`'s grammar admits identifiers and
    equality, never a bound parameter or a literal, so a `WHERE owner_type =
    $2` would have needed a new placeholder in the grammar for a predicate that
    is not needed at all.
  - **Two bounds, both load-bearing.** A virtual edge is parent-direction only,
    so a row reached through one is `PARENT_ONLY` and never has its own children
    pulled: the inference can widen the slice by the parents of rows already in
    it and by nothing else, which is what ARCHITECTURE.md §14 means by "virtual
    edges are parent-direction only, so enabling it later widens nothing the
    caps do not bound". And a pair with more than `polymorphicValueCap` (50)
    distinct values in its sample is not followed at all — a `_type` column
    holding free text is not a discriminator, and one virtual edge per value
    would be an unbounded fan-out of parent tables from one column. The cap is a
    product decision this file introduced and ARCHITECTURE.md §3.2's amendment
    of 2026-09-08 now records, together with the 64-byte message bound. What it
    is no longer
    is *silent*: a pair the cap stopped is reported as `public.attachments
    (owner_type, owner_id), the sample carries more than 50 distinct owner_type
    values`, so the one reason that is a threshold of ours rather than a
    property of the schema is named. Every other reason a pair is not followed
    keeps the bare sentence, because every other reason is the source's.
  - **The sample is bounded *and* repeatable**, and it is two shapes because it
    is two statements. `plan.distinct_sample` (`distinctSampleSQL`) is the one
    to prefer: `TABLESAMPLE SYSTEM` at a fraction that reads about
    `probeSampleRows` (2,000) rows, `REPEATABLE` at `probeSampleSeed`, under a
    `LIMIT` for a stale `reltuples`. `plan.distinct_prefix`
    (`distinctPrefixSQL`) is the fallback for the two relations `TABLESAMPLE`
    cannot be taken on — a partitioned table, and one nothing has analysed,
    where a fraction of an unknown row count is a full scan wearing a probe's
    name — and it orders the prefix by the table's identity columns. Both
    bounds are in the shape as well as in the builder, because a bare `SELECT
    DISTINCT col FROM t` is not bounded by a `LIMIT` above it: the aggregate
    consumes its whole input first, which is the unbounded read inside the
    holder transaction THREAT_MODEL.md T9 forbids.

    Repeatability is the half that was missing and it is not a nicety here.
    An unordered `LIMIT 2000` reads whichever rows the scan reaches first, and
    `synchronize_seqscans` (on by default) starts a second scan of a large table
    where a recent one left off, so two runs over one snapshot could sample
    different rows — and this sample decides which virtual edges exist, hence
    which rows are selected. That is §3's determinism rule and I3 both. The
    constants are `probeSampleRows`/`probeSampleSeed` in `sql.go`, shared with
    §3.4's pseudo-key probe, because they are one bound and one seed rather than
    two: this is the most rows a probe may read inside the holder transaction.

    The known cost of the fallback: ordering by a *probed pseudo-key* on a table
    nothing has analysed is a sort of the table rather than an index scan. It is
    the narrow corner where a reproducible sample has nothing cheaper to offer,
    and it is bounded in what it returns rather than in what it reads.
  - **An edge is built only between one key space and the same one.** The
    referenced columns are the parent's primary key; the key must be a single
    column, and its `keyType.kind` must equal the `_id` column's, which must
    itself be `kindInt`, `kindUUID` or `kindText`. `bpchar` is out because its
    comparison strips padding and `kindOther` because its join carries a cast
    back to a type no polymorphic id has.
  - **The candidate names are §3.2's rule first, and Django's are §3.2's rule
    only.** `railsCandidates` tries the underscored plural — of the value and of
    the value without its module path — before anything else, because §3.2 says
    "underscore and pluralise"; an earlier version put the raw and un-pluralised
    forms first, which inverted the rule and resolved `User` to a `user` table
    in preference to `users`. The un-pluralised forms are still tried *after*
    it, and that is a deliberate deviation from §3.2: a `_type` column holding
    the table name itself is the shape `testdata/nasty.sql` trap 6 carries and
    the shape hand-rolled polymorphism takes, and without the fallback both
    values of the only fixture this feature has would be unmapped. ARCHITECTURE.md §3.2's amendment of 2026-09-08 admits the fallback in that order. `djangoCandidates` has no fallback at all — the bare model
    name would bind an `auth`/`user` content type to any app's `public.user` —
    so a model with an explicit `db_table` resolves to no table and is reported
    as an unmapped value, which is the finding the feature already has for a
    name it cannot place.
  - **A dangling polymorphic id never enters the parent's key set.**
    `pushVirtual` translates the referenced values through the parent with
    `virtualIdentityKeys`, which always issues the `plan.map_keys` read;
    `identityKeys` short-circuits when the referenced columns are already the
    identity, and that short-circuit is sound only behind a *declared*
    constraint, which is what guarantees the referenced row exists. Behind an
    inferred edge nothing does. Taking it there put a key for a row that does
    not exist into `selected`, which inflates `Estimate.Rows` and the printed
    step count, makes extract return one row fewer than the plan promised, and
    ends as `internal/verify`'s exit 7 with the target already dropped and
    loaded — a planner defect reported as a load failure.
    `TestPlanVirtualEdgeToADanglingRow` is the guard, over a fixture of its own
    in `plan_integration_test.go`: `nasty.sql`'s dangling owner is on the one
    attachment no declared edge reaches, so its pair is never read and the
    assertion there could not fail.
  - **Three findings, three lists, and the v1 sentence is kept for the third.**
    `Plan.Virtual` is what was inferred and followed; `Plan.Unmapped` is a
    sampled value that names no table, spelled `public.attachments.owner_type
    value "widgets"`; `Plan.Polymorphic` is what inference could not resolve and
    still prints `polymorphic pair detected, not followed: no constraint`. A
    pair whose sample produced no followable value at all is listed whole
    (`public.attachments (owner_type, owner_id)`), and a single unfollowable
    value of an otherwise resolved pair is listed as `public.attachments
    (owner_type, owner_id) where owner_type = "events"` — saying nothing about
    it is the silence FK-10 exists to make impossible. A `_type` value is
    admitted into a message because §14 says it identifies a class and not a
    row, and it is quoted and truncated at 64 bytes for the column that turned
    out to hold something else (THREAT_MODEL.md T4).
  - **A fourth finding, and the walk is what produces it.** A `_type` value the
    walk meets that the sample never produced has no edge, so its rows' parents
    are not followed — and the first version of this file dropped it in silence
    while reporting the pair as fully resolved, which is research/COMPLAINTS.md
    FK-10 exactly. `noteUnknownType` records it during the walk and
    `unknownFindings` appends it to `Plan.Polymorphic` in `assemble`, spelled
    `public.attachments (owner_type, owner_id) where owner_type = "Widget", a
    value the sample did not produce`, in pair order and then value order.
    It is bounded by `polymorphicValueCap` per pair, and what is past the bound
    is one further finding rather than a silence. `TestUnknownTypeValues\
    AreReportedNotDropped` holds the bookkeeping; the integration suite cannot
    reach it, since every fixture table is analysed and small enough that the
    sample sees all of it.
  - **Only a value the sample really did miss is reported that way.**
    `pairPlan.sampled` is every value the sample produced, and `virtualParents`
    consults it before `noteUnknownType`: a value with no edge that *is* in it
    was already reported — under `Unmapped`, or as an unfollowable value — so
    saying it again as "a value the sample did not produce" would print one
    value twice under two reasons, the second false, pointing at a remedy
    (widen the sample) that would change nothing. The `'Ghost'` row in
    `plan_integration_test.go`'s `note_links` is the guard: the sample maps it
    to no table and the walk then meets it on a selected row.
  - **Every followed virtual edge is printed before extraction.** `internal/core`
    emits `plan.polymorphic.inferred` per `Plan.Virtual` entry beside the
    `Plan.Polymorphic` and `Plan.Unmapped` lines (T-0072: `internal/core/codes.go`,
    `internal/event/catalogue.yml`, `planStage` in `run.go`, `internal/tui/collect.go`).
    `internal/emit`'s `virtualList` comment and its `virtual_fks:` bullet were
    corrected in the same landing, ARCHITECTURE.md §3.2 and §14 were amended by
    the orchestrator on 2026-09-08, and `testdata/nasty.sql` and `testdata/README.md`
    trap 6 now describe the inferred-and-followed behaviour with a Rails-spelled
    row. Nothing here is owed.
  - **`ForeignKey.Name` carries identifiers and never the `_type` value.** It is
    `public.attachments.owner_type`: the discriminator column the inference
    read, and nothing else. `internal/emit` copies `Plan.Virtual` into
    `lazyslice.yml`'s `virtual_fks:`, and ARCHITECTURE.md §10 says every value in
    that file is an identifier, a count, a fingerprint or a flag value with
    literals withheld — a sampled column value is none of those, and the yml is
    the file the tool tells people to commit. The value stays where §14 does
    admit it: the log, through `Step.Why` and through the unmapped and
    unfollowable findings. The name stops at the discriminator because
    `emit.virtualList` renders `Name Child (cols) -> Parent (cols)`: a name that
    also carried the parent made that line print the arrow clause twice, and the
    parent is already on the edge. What the name adds is the one thing the
    reconstructed edge cannot say — which column the discriminator was. Two
    edges of one pair therefore differ by parent rather than by name, so
    `inferPolymorphic` dedupes on name *and* parent, and two values naming the
    same table are still one edge reported once.
  - **`staticReach` does not know about the inferred edges.** §3.6 runs before
    the inference does, because the inference reads the source and the
    privilege pass is what decides whether this role may. A table reachable only
    through a virtual edge is therefore judged unreachable there, and an
    unreadable one is dropped to `SchemaOnly` rather than refused with a
    `GRANT`. That fails towards the quiet direction rather than the loud one,
    which is the wrong way round for §3.6; `virtualEdgeTo` refuses to build an
    edge into an out-of-scope table, so the walk cannot then read it, and the
    value is reported as unfollowable instead. Closing it properly needs the
    sample to move ahead of the privilege pass, which means reading a table
    before knowing the role may — the reason it is where it is.
  - **The Django half has no fixture.** Neither `testdata/nasty.sql` nor pagila
    carries a `django_content_type` table, so `djangoContentTypes` is exercised
    only by `TestDjangoCandidatesFollowTheDefaultTableName` over the name
    mapping. A Django-shaped torture schema is owed (phase 5, ten schemas).
- **Defaults are applied here.** A `PlanRequest` field left at zero takes §3's
  default (take 500, cap 100, depth 3, row budget 1,000,000, memory budget
  256 MiB). A caller that means an explicit zero — no root rows, no rows per
  parent key, no children — passes a **negative** `Take`, `Cap` or `Depth`,
  which clamps to zero, because zero cannot mean both "unset" and "none" in an
  int field. This is a trap laid for the flag parser: `--take 0` typed by an
  operator must not come out as 500. Core owes the translation until
  `pipeline.PlanRequest` can carry the difference itself (pointers, or a set
  mask).
- **A nil `*Classification` is allowed** and means "nothing is masked" for the
  filter estimate, which is what `lazyslice plan` over a schema alone needs. It
  does **not** mean "everything may be a lookup": `hasMaskedColumn` answers yes
  for every table under a nil classification, so an unclassified run produces
  no `Lookup` step and walks instead. The privacy half of §3's lookup rule is
  vacuously true when nothing was classified, and a lookup-shaped table — no
  outgoing edge, one incoming edge, under a thousand rows — is the shape of
  `staff`, `admins`, `api_keys` and `contacts`; copying one whole hands over
  every production row of it, surrogate keys included (§6 item 6).
- **The identity ladder's third rung requires NOT NULL columns.** A unique
  index over a nullable column does not identify a row, because Postgres allows
  any number of NULLs in one. Ties between usable indexes go to the fewest
  columns, then the index name.
- **The pseudo-key candidate** is §3.4's own list — the table's foreign-key
  columns plus its NOT NULL discriminators — in table column order, minus
  generated columns (the target recomputes those, so they are never copied). A
  nullable column is never a candidate, foreign key or not: `count(DISTINCT
  (a, b))` counts a tuple holding a NULL as distinct, so a nullable candidate
  passes the probe and then loses every row with a NULL in it at `readKeys` —
  the silent omission of trap 12 without the exit 12. An empty `TABLESAMPLE` is
  not a pass either: the probe fails closed.

  §3.4 does not define "discriminator", and this is the reading:
  a NOT NULL, non-generated column that is `boolean`, an enum type, or named
  like §3.2's polymorphic `_type` half or §3.1's type/status/kind/code family —
  and, for the name-matched case only, of a type `keyset.go` already carries
  (int, text, `character(n)`, uuid). Every admitted column is therefore
  comparable **by construction**: a foreign-key column because its referenced
  side carries a btree unique index and the constraint could not exist without a
  shared btree equality operator, and the rest because boolean, enum and those
  four kinds all have a default btree opclass. This is what keeps the rung's
  failure a refusal. The candidate was briefly every NOT NULL column, and that
  admitted a NOT NULL `json`, `xml` or geometric column, whose type has no
  default btree opclass: the probe's `count(DISTINCT (...))` and
  `childKeysSQL`'s `ORDER BY` then die inside pgx with "could not identify a
  comparison function for type json" (SQLSTATE 42883) — a wrapped driver error
  with no exit code and no `--key`/`--skip-table` remedy, where §3.4 promises
  exit 12. Widening also made exit 12 nearly unreachable (any table without two
  byte-identical rows acquired an "identity") and spread the sampled-uniqueness
  risk §3.4 accepts for a narrow candidate across every table without a primary
  key. `TestPlanUncomparableColumnStillRefusesWithNoIdentity` is the guard, over
  a `page_views` table in `extraSchema`; nothing in `testdata/` has that shape,
  and a fixture table is owed.

  Two known edges of the reading, both failing towards a refusal rather than
  towards a driver error: a domain over `json` named `payload_type` is not
  admitted (the name matches but the kind is `kindOther`), and a genuinely
  useful non-discriminator column — a `taken_at timestamptz` beside a foreign
  key — is not admitted either, so such a table reaches exit 12 naming `--key`.
  An explicit `--key` naming an uncomparable column is **not** covered: it still
  surfaces as the raw driver error, because the positive rule above is too
  strict for an explicit key (`timestamptz`, `numeric` and `date` keys are
  legal and are all `kindOther`). Closing that needs the catalog test —
  `pg_type` → `pg_opclass` for a default btree opclass — which is a read this
  package would have to add.
- **An explicit `--key` is probed on a bound, not on the whole table.** A
  `HAVING count(*) > 1` is not stopped by a `LIMIT` above it — the aggregate
  consumes its whole input first, and `--key` is passed precisely when no index
  covers the columns — so an unbounded probe is the shape THREAT_MODEL.md T9
  forbids, repeated on every run once the key is in `lazyslice.yml`. A table
  whose `reltuples` is at or below `explicitKeyProbeRows` (100,000) is probed
  whole and yields `IdentityUnique`; a larger table, and one nothing has
  analysed, is probed over a prefix of that many rows and yields
  `IdentityPseudo`, which is what that rung means. A non-unique explicit key is
  refused by count, never by printing the duplicated values (THREAT_MODEL.md
  T4).
- **`--key ctid`** is refused with `plan.refused.key_column` (exit 2, usage)
  rather than silently becoming an identity. There is no ctid rung (ADR-005).
- **The seed keeps its LIMIT under `--where`.** §3 writes the two as
  alternatives; bounding both is what THREAT_MODEL.md T11 asks for.
- **The root is never a lookup.** §3's lookup set is computed over every table;
  excluding the root keeps a lookup-shaped root walkable.
- **Key sets store what §2 says they store.** A single int8 key set is a sorted
  `[]int64`; every other key set is one order-preserving byte slab with a span
  index. Duplicates go by sorting and scanning that storage — a batch is
  appended and merged, never held in a parallel `map[string]struct{}` — because
  `Bytes()` returns §2's two formulas and a set that held a map besides would
  report a fraction of what the process holds. Measured over 1,000,000 single
  int8 keys: `Bytes()` 16 MB against 8 MB of heap. The slab encoding is
  order-preserving (big-endian int8 with the sign bit flipped, raw uuid bytes,
  strings with `0x00` escaped and terminated) so that sorting the spans sorts
  the set in identity-column order without decoding.
- **`character(n)` keys travel as text.** `bpchar` is blank-padded on disk and
  its comparison against a text array is not: `t."c" = k.k1` resolves to the
  text equality, which strips the padding from the column (verified on
  postgres:16, and `TestPlanBlankPaddedKey` fails with a whole child table
  empty when this is wrong). The select list therefore reads a `bpchar` key as
  `::text`, so the stored key is the form the join compares.
- **`Estimate.Bytes`** has no formula in §3. It is Σ rows × an estimated row
  width summed from the column types (`query.go`). It is an order of magnitude
  for the plan to print, and the flag that changes it is the one that changes
  the row count.
- **`Estimate.FilterMemory` counts JSON leaves individually**, which is what
  §3's formula says. A masked `json`, `jsonb` or `hstore` column costs a row as
  many residual-filter cells as the document has scalar leaves (§4 replaces
  every leaf; §6 keys the filter by path), and the leaf count is the average
  over the samples `introspect` already read, rounded up. A document column no
  sample parsed for counts `jsonLeafDefault` (8) rather than 1: the budget is a
  control against an OOM, and being low lets a run past it.
- **`Plan.SnapshotID` is left empty.** The planner is not given the snapshot
  id; core holds it and fills the field in.
- **A foreign key that references something other than the parent's identity**
  is translated with one extra read per chunk (`referencedKeys`,
  `identityKeys`). No fixture in `testdata/` has such an edge — every foreign
  key there references a primary key — so `plan_integration_test.go` creates
  one of its own (`extraSchema`) alongside a `character(n)` key, and walks each
  in both directions against a real Postgres. A table in `testdata/nasty.sql`
  is still owed: these two live here only because that file is shared with the
  suites that count its tables.

- **The plan-time write-back check** (`writeback.go`, `plan.refused.unwritable`,
  exit 12, T-0054). For every column the classification masks, in every table
  still in scope, the plan asks `mask` whether the chosen generator can write
  into that column's type, and refuses by name if it cannot. It runs after
  `--skip-table` and the privilege pass — so a column in a table this run will
  never read cannot refuse it — and before the first key is fetched.
  - **Why it exists.** `internal/transform` could only discover a mismatch per
    value: it masks, tries to parse the masker's text back into the Go kind the
    column arrived as, fails, and refuses at exit 7 with rows already moved. On
    pagila the classifier decided `credential` on every `last_update
    timestamptz`, `$lazyslice$invalid` is in none of the timestamp layouts, and
    every whole-pipeline run over the fixture died there. Transform keeps that
    refusal as a backstop; this is the check that makes it unreachable
    (`internal/transform`'s `TestTransformNeverRefusesWhatThePlanCheckAdmits`).
  - **Why here and not only in `internal/classify`.** That package now gates a
    category by the column's type on the way *in*, and it is the better place to
    catch it. But the category on a `Decision` at the end of classification is
    not always the one that gate saw: FK propagation, the same-column-name rule
    and a committed `lazyslice.yml` all move a category onto a column after the
    fact. This check runs over the answer rather than over each step to it, and
    it reads `mask`'s own declaration rather than the rule pack, so the two have
    to agree for a plan to pass.
  - **It asks two questions and both are `mask`'s** (`mask.Writable`): is the
    admissible domain non-zero — a `phone` column declared `integer` has nothing
    to write, E.164 does not fit in nine digits — and can the type hold the kind
    of value the category emits. A domain alone would not catch the blocker: the
    credential literal has a domain of 1 on every column in the world.
  - **What it does not judge, deliberately.** An enum or any type `mask` has no
    family for is passed: every generator answers a labelled column with one of
    its labels, and refusing on ignorance would turn a working run into a plan
    refusal. A masked column with no category at all is also passed — that is a
    classification bug, and transform names it as one.
  - **It is not §5's unique-index domain rule.** `d_required = n²/2ε` and the
    refusal that prints the largest `--take` a column can carry is a separate
    plan-time check that has not landed; `constraintsOf` leaves `Unique`, `Rows`
    and `Distinct` unset because `Writable` does not read them.
  - **The catalogue row borrows `{reason}`.** The message has to name the
    category and the type family to be actionable — the operator's fix is either
    an `--unmask` or a rule change, and neither is choosable from a table and a
    column name — and `event.ArgKey` has a key for neither. Both halves are
    identifiers, never a row (THREAT_MODEL.md T4). An `ArgCategory` and an
    `ArgType`, and this message re-templated onto them, are owed to a task whose
    paths include `internal/event/event.go`.
  - **The type reduction is `mask`'s** (`mask.TypeTag`, `mask.MaxLen`, and the
    three quoting helpers `mask.StripTypmod`, `mask.UnquoteType` and
    `mask.BareTypeName`), the same one `internal/transform` builds its
    `Constraints` with. A check that judged a different family than transform
    masks would be no check at all, so the table lives in one place and every
    caller reads it. What this file still writes for itself is `domainBase` and
    the enum lookup, which need `pipeline.Schema` and so cannot live in `mask`:
    `internal/classify` and `internal/transform` have their own copies of that
    resolution, three in all, and the one home for it would be
    `internal/pipeline`, whose file T-0054's paths did not include.
    `internal/verify/columns.go` has a fourth copy of the quoting as well, and
    is outside those paths too.
  - **The refusal is held by a unit test as well as by the fixture suite**
    (`writeback_test.go`). `writeback_integration_test.go` asserts that the real
    classifier over both fixtures produces nothing this check refuses, but it
    re-derives that answer from `mask.Writable` rather than calling
    `checkWriteBack`, so it would stay green if the refusal were lost.
    `TestUnwritableColumnIsRefusedAtPlan` builds a schema and a classification
    by hand — `credential` on a `timestamptz` — and asserts the code, the exit,
    the table, the column, the message and that only the privilege pass ran
    before it; `TestWritableColumnIsNotRefusedAtPlan` is the other side, so a
    check that refused everything fails too.
- **`writeback_integration_test.go` is the suite T-CORE reads.** Every other
  test in this package hands the planner a classification written by hand, and
  `internal/transform`'s own tests build both the schema and the batch; neither
  can see a disagreement *between* the stages, which is what T-0054 was. This
  one introspects both fixtures for real, classifies with the real classifier
  and rule pack, plans pagila from `customer --take 50` and nasty from
  `tenant_users --take 3`, and masks a batch of each planned table's own rows.
  It reads those rows on a connection of its own rather than through the
  snapshot reader: "twenty rows of whatever this table holds" is not a shape the
  planner sends, and the source's allowlist would refuse it (THREAT_MODEL.md
  T9).

## Decisions made during implementation (T-CORE, 2026-09-06)

- **Privileges arrive on the request.** `PlanRequest.Priv` is filled by
  `internal/core` from one `Source.Privileges` call, and `sqlUnreadableTables`,
  `sqlCurrentRole`, `readUnreadable` and `readRole` are gone with the two
  allowlist shapes that carried them (`plan.unreadable_tables`,
  `plan.current_role`). One read, one answer: the role the decision header
  prints and the role a §3.6 refusal names cannot now differ.
- **Partition leaves are read here, and only they**
  (`sqlUnreadablePartitionLeaves`, shape `plan.unreadable_partition_leaves`).
  `internal/pg`'s privileges query excludes partition leaves
  (`NOT c.relispartition`), because a leaf is not a table anything else plans,
  loads or counts — and §3.3 makes it a privilege question all the same: a
  partitioned table's keys are fetched from the root and Postgres routes to the
  leaves, so a leaf the role cannot SELECT fails the *root's* read. Dropping the
  planner's own query without this left that as a raw 42501 from pgx partway
  through extract, with the target already dropped and partly loaded, where §3.6
  promises exit 12 and a `GRANT` before any row moves and says "there is no
  mid-extract permission failure by design". The query is deliberately
  leaves-only, so it and `Priv.Unreadable` can never disagree about the same
  relation; `unreadableRelations` merges the two, re-sorts, dedupes and keeps
  the root attribution. Folding it into `internal/pg`'s read is the better shape
  and is owed there.
- **`Plan.Polymorphic` and `Plan.Unmapped` are two lists.** A detected
  `<x>_type`/`<x>_id` pair goes under `Polymorphic`; `Unmapped` is for the
  sampled `_type` values that map to no table, which v1 never produces because
  it never samples them. They were one list, so a renderer printed a pair name
  under a heading that says it is a value.
- **`withDefaults` stays** even though `internal/core` now substitutes §3's
  defaults before calling `Plan`. It is a defensive net for a direct caller —
  `plan_integration_test.go` builds a `PlanRequest` with no `Take` — and a zero
  reaching the walk would mean "select no rows" rather than "select 500".
