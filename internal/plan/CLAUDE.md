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
- **The memory budget also counts the residual filter's emitted count**
  (`emittedMemory`, `plan.go`; ADR-015 proposed, T-0302): 16 bytes per
  distinct output of every masked column whose masker has a vocabulary
  (`mask.Emitting` under the column's `constraintsOf` and `Decision.Role`),
  bounded by that vocabulary (`mask.Admissible`) and, for a scalar column, by
  the table's selected rows. It is checked in `checkBudgets` beside
  `keyMemory` and `filterMemory` and is not a field of `Estimate`, which is
  outside this package; at a name list's size it is kilobytes.
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
- A `Decision.Refused` is a refusal, not a hint: `checkFKPairRefusal`
  (`fkpair.go`, T-0253, T-0257) reads it beside `checkWriteBack`, before any
  key is fetched, and stops the run at exit 12 naming both columns of the
  pair unless the operator has explicitly `--unmask`ed both of them
  (`internal/classify/CLAUDE.md`'s own "A validated foreign key's columns are
  raised together, or not at all" has the full account).

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
    of 2026-09-08 now records (the message bound it records is superseded by
    T-0131's count-only findings — see below, and ARCHITECTURE.md's own
    2026-09-14 amendment). What it is no longer
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
    `Plan.Virtual` is what was inferred and followed; `Plan.Unmapped` is one
    finding per column of sampled values that name no table, spelled
    `public.attachments.owner_type: 3 distinct values mapping to no table;
    inspect the distinct values of owner_type on public.attachments in the
    source to see what they are`; `Plan.Polymorphic` is what inference could
    not resolve and still prints `polymorphic pair detected, not followed: no
    constraint`. A pair whose sample produced no followable value at all is
    listed whole (`public.attachments (owner_type, owner_id)`), and a single
    unfollowable value of an otherwise resolved pair is listed as
    `public.attachments (owner_type, owner_id) resolves to public.widgets,
    which this run cannot follow` — the parent name, because `mapTypeValue`
    already resolved the value to that table and only `virtualEdgeTo` declined
    the edge (out of scope, no usable key); the table name is an identifier,
    not a source-row value (§10/§14), and it is the one part of this finding
    an operator can act on. Saying nothing about the value at all is the
    silence FK-10 exists to make impossible.

    **No `_type` value is ever printed, and no digest of one either (T-0131,
    decided 2026-09-14).** `showValue` used to quote whatever the sample read
    into `Plan.Unmapped`, `Plan.Polymorphic` and `Step.Why`, and the
    2026-09-09 review (finding 2) reproduced that string reaching stdout,
    `--json` and any log with a masked column's real value in it. The fix
    that landed first replaced it with an HMAC-SHA256 digest keyed on
    `schema.Fingerprint`, so two findings about the same value would still
    read as the same finding without naming it — but that fingerprint is not
    secret: `internal/emit` writes it to `lazyslice.yml` and `internal/load`
    writes it into the target's `lazyslice_meta` (ARCHITECTURE.md §10, §11),
    so anyone holding a transcript plus either artifact could recompute
    `HMAC(fingerprint, candidate)` for a guessed value and confirm it — a
    membership oracle, THREAT_MODEL.md T4. Keying the digest on the actual run
    key instead would not have closed that: it would only hand the same
    oracle to everyone who holds the run key, which THREAT_MODEL.md T13
    already treats as a real population, not a hypothetical one. The
    orchestrator's decision was therefore no digest at all, keyed or not.
    `unmappedFinding` and `unknownFindings` (`polymorphic.go`) report a
    column's unmapped or unknown values as a count and nothing else, with one
    fixed remedy: an operator who needs the actual values can run `SELECT
    DISTINCT <type_col> FROM <table>` on the source themselves — this run
    does not grant that access and must not act as though it could.
    `TestUnmappedFindingCarriesOnlyAnIdentifierAndACount` and
    `TestUnknownTypeValuesAreReportedNotDropped` (`polymorphic_test.go`) hold
    the format at the unit level; the canary test in `cmd/lazyslice` holds it
    at the output-sink level, over every text column of a fixture, not only a
    polymorphic one.
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
  - **A composite is refused by type** (`compositeType`, T-0094, T-HARD-B), and
    it is the one type this check refuses on the type alone. Everything else
    `constraintsOf` declines to judge is a type `mask` has no tag for and might
    still load — an ltree, a PostGIS geometry — but a record can hold nothing
    any generator emits, so declining there was a masked composite handed to
    `internal/transform`, masked as if it were a scalar, dying in the loader
    with rows already moving; and a composite the classifier left below the
    threshold was copied verbatim with the personal data inside it
    (THREAT_MODEL.md T1). `internal/classify` now reaches `possible` on a
    composite whenever its name or any field of any sample says personal data
    (`decideComposite`), so the two halves are one decision: classify fails
    closed, and this is where the run stops. The refusal reuses
    `plan.refused.unwritable` and exit 12 rather than adding a code — the
    sentence "cannot be masked in place" is exactly true of a record — and its
    `{reason}` carries the composite type and the two escapes, `--skip-table
    TABLE` or `--unmask TABLE.COL=REASON`. A distinct code would want a
    `catalogue.yml` row and a `docs/ERRORS.md` regeneration, and `docs/` was
    outside T-HARD-B's paths; if one is wanted later, the message text is here.
    `TestMaskedCompositeIsRefusedAtPlan` and
    `TestUnmaskedCompositeIsNotRefusedAtPlan` hold both sides, including that an
    ltree is still not refused.
  - **An array whose samples arrive as a text literal is no longer refused
    here** (T-0127). `arrayArrivesAsLiteral` was a stand-in for the masker half
    of T-0103: pgx hands such a column back as the single string
    `{a@b.test,c@d.test}` because the source pool registers no user types
    (T-0076), and before T-0118 `internal/transform`'s `maskArray` fired only on
    a `[]any` — so masking the literal as one scalar handed `CopyFrom` a string
    for an array column and it died with "cannot find encode plan" at exit 7,
    rows already moving. T-0118 taught `internal/transform` to parse and mask
    such a literal element-wise, and T-0129 taught `internal/verify` to split it
    the same way for the residual scan, so the column this check used to refuse
    is now planned, masked and verified like any other array. Both fixture cases
    stay covered by `TestArrayThatArrivesAsALiteralIsNotRefusedAtPlan` (the
    citext case) and `TestArrayTheDriverDecodesIsNotRefusedAtPlan` (the text[]
    case that was never refused). The removed branch's comment recorded a hard
    ordering: this task does not land before T-0129, because while the refusal
    stood, no masked array literal reached the target for T-0129's blindness to
    cost anything. Removing this refusal also moves the failure for an
    *unparseable* array literal from here at plan time (exit 12, before a key
    is fetched) to load time (`transform.refused.masker`, exit 7, mid-stream
    with earlier tables already committed); no plan-time parse check was kept
    to hold the failure at this stage. `internal/transform/CLAUDE.md`'s T-0127
    entry records why that is fail-closed rather than merely moved: classify's
    reader and this package's masker read different grammars (a trailing
    empty field and a doubled quote split one way and refuse the other), but
    every value either package sees for an array column is Postgres's own
    `array_out` output, and `array_out` never writes either form — so the
    divergence has no reachable column, only an unreachable one.
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

## The unique-index domain rule (T-TORTURE)

`unique.go` is ARCHITECTURE.md §5's other domain rule, and it landed with the
torture schemas because that is what found it missing: `mask.Pick` had
implemented the whole of it — pick the widest generator for the category, refuse
with a `*DomainError` when even that cannot emit `d_required = n²/2ε` — and
nothing called it. `writeback.go` said so in a comment. Three of the ten schemas
in `testdata/torture/` died **in the loader** on a unique violation as a result
(`testdata/regressions/001-unique-index-masking-collision.sql`).

- **It runs after `walk` and before `assemble`**, because it is the only check
  whose question needs *n*: `d_required` is over the planned row count, which is
  what the walk decides. `plannedRows` is that number and never the source's.
- **It writes the chosen masker back onto `pipeline.Decision`.** §5 says "the
  plan picks, within the column's category, the registered generator with the
  largest `Domain()`", and `internal/transform` masks with `Decision.Masker`, so
  the pick has nowhere else to go. That is how `phone` becomes `phone_unique` and
  `network_id` becomes `ip_unique` on a unique column. **`Classification.Fingerprint`
  is computed before this happens** and therefore does not cover the escalation;
  `internal/core/domain.go` writes `Domain` and `SmallDomain` after the same
  fingerprint for the same structural reason. T-0101 carries it.
- **Which columns it applies to is `internal/classify`'s answer, not this
  package's.** `Decision.UniqueIndex` is set there, and what it means — a column
  that carries the uniqueness alone, plus the two approximations for composite
  and partial indexes — is stated in `indexKeys` and `raiseCompositeUnique`.
  Do not second-guess it here; the two have to agree, and the schema is read
  once.
- **`plan.refused.unique_domain` prints §5's three escapes**, and prints the row
  count one only when there is one: `MaxRows` is zero for a generator with a
  domain of 1, which is what `credential` has, and "take at most 0 rows" is not
  advice (T-0098).
- **The unit of the choice is the equality group, not the column (T-0132,
  ARCHITECTURE.md §5's amendment of 2026-09-14).** Applying §5 per column is
  what broke foreign keys: classification propagates a *category* along a key,
  nothing propagated the *generator*, so a unique parent escalated to
  `credential_unique` while its non-unique child kept the category default and
  the load ended at **exit 8** with the key unvalidatable (finding 3 of the
  2026-09-09 review, `docs/reviews/2026-09-09/evidence/fk_masker.log`).
  `equality.go` is the rule and `checkUniqueDomain` is now its caller.
  - **The group** is the transitive closure of "appears at either end of a
    declared foreign key", intersected with the masked columns this run will
    load, split by category. A column no key touches is a group of one and gets
    exactly the check it always had, which is why `unique_test.go` is unchanged.
  - **The masker is the widest generator *any member needs*** — each member's
    own `mask.Pick` answer at its maximum — and not the widest the category has.
    A group with no unique member needs the default and keeps it; the other
    reading would move every non-unique masked column in the schema onto an
    alternate nobody asked for.
  - **`fitsGroup` asks four questions and a group has to pass all four**:
    writable in every member, wide enough for *every* unique member's
    `d_required` (not only the one the generator came from), drawing on the same
    declared value list where any member is a closed column, and emitting the
    same `Domain()` in every member — because `credential_unique` and the
    free-text generator are length-fitted, so one masker over a `varchar(25)`
    and a `text` column is still two mappings and the key still breaks. A
    failure of any of them is exit 12 naming the group and every column in it.
  - **The closed-column question is about the labels, not their count**
    (T-0132 review, finding 1). Every generator answers a closed column with one
    of *that column's own* labels (`mask/domain.go`'s `labelValue`), so a parent
    under `CHECK (label IN ('c','d'))` and a child under `CHECK (label IN
    ('a','b'))` both report `Domain() == 2`, sail through the count question that
    was written to catch exactly this, and mask one input to `'c'` at one end and
    `'a'` at the other. `closedColumn` asks `mask.ColumnDomain` whether the
    declarations bound the column at all rather than re-parsing a `CHECK` here —
    a second copy of that grammar is what `constraintsOf` above exists to avoid —
    and `sameClosedSet` then compares the `CHECK` text verbatim, which is
    *stricter* than comparing the parsed lists: two ends that spell one list two
    ways are refused although they would mask alike. That is the recoverable
    direction (a refusal at plan with an escape, against exit 8 in the loader
    with rows already moved), and **T-0158** owes the exact comparison via an
    exported label list from `mask`. What is still not compared is
    `Constraints.TypeTag`: `inet` against `cidr`, `date` against `timestamp`, and
    the other families a generator branches on, filed as **T-0159** rather than
    guessed at, because a blanket tag-equality rule would refuse the ordinary
    `text`-against-`varchar` pair that masks alike.
  - **The refusal has its own code, `plan.refused.equality_group`** (T-0132
    review, finding 3). It borrowed `plan.refused.unique_domain` when T-0132
    landed, and that template opens "is under a unique index" — untrue of two of
    the three causes, which fire with no member under one at all: the
    length-mismatch branch is reachable on any foreign key between two masked
    text columns of different declared lengths, which is ordinary, not a corner.
    A group of one is the per-column unique refusal it always was and keeps the
    old code. `docs/` is outside this task's paths, so **`make docs` has not been
    run for the new row and `make docs-check` fails until it is** — **T-0160**.
  - **The escapes are printed per cause** (T-0132 review, finding 2).
    `groupEscapes` prints "lower the row count" only on the `d_required` path,
    where rows are the cause; the other two causes are a type that cannot hold
    the value and members that would not mask alike, and neither is fixed by a
    smaller `--take`. And for a group of more than one the `--unmask` escape is
    **every column of the group or none**, never "one of them": `maskedMembers`
    skips a column whose decision is not `Masked`, so unmasking one end leaves
    that end's production values in the target *and* the key still unvalidatable.
    `equalityNote` carries the same correction onto `uniqueDomainReason`'s
    single-column `--unmask` sentence when the refused column is in a group.
  - **Only declared foreign keys are group edges.** `p.fks`, not the inferred
    edges of §3.2: the target never carries a `virtual_fks:` or polymorphic
    edge as a constraint, nothing validates one at load, and `internal/classify`
    does not propagate a category along one either, so unifying maskers over one
    would be a decision no other stage agrees exists. **T-0154** owes that
    question an answer.
  - **`internal/transform` is unchanged** and must stay so: it masks with
    `Decision.Masker` (`transform.go`'s `plan`), so writing the group's choice
    onto every member's decision *is* "transform masks every member
    identically". Do not add a second place that decides a masker.
  - **The guards are `equality_test.go` and
    `testdata/regressions/010-fk-connected-columns-mask-differently.sql`.** The
    regression is the review's own two-table reduction, asserted with `expect:
    ok` plus `unique-masked:` on the parent and the new `equal-masked:` key on
    the child — checked to fail on a revert of `equality.go`, which reproduces
    the review's exit 8 verbatim.
- **Both outcomes are held by a unit test** (`unique_test.go`), for the reason
  `writeback_test.go` exists: the evidence for this check is otherwise ten
  Docker-gated schemas and eight files under `testdata/regressions/`, none of
  which CI runs, so a revert would be caught by nothing on the path a change
  actually takes. `TestUniqueDomainPicksTheWiderMasker` is `phone` becoming
  `phone_unique`; `TestUniqueDomainRefusesWhenNoMaskerFits` is `credential` and
  asserts the whole message, not only the code;
  `TestUniqueDomainSkipsATableThisRunWillNotLoad` is the guard against refusing
  on the source's shape. They call `checkUniqueDomain` directly because *n* is
  the planned row count and a `Plan` over a reader with no rows plans none.

## The recreated-DDL literal rule (T-0134, ARCHITECTURE.md §11.1's 2026-09-14 amendment)

`ddlliteral.go` is §11.1's new sentence: a string literal inside a column
`DEFAULT`, a `CHECK` constraint or a generated-column expression is **inside the
data boundary**, because §11.1 recreates all three as the catalog's own text and
the target therefore receives the literal exactly as it receives a row value.
Nothing in §6 could see it — the residual filter holds only cells the
transformer masked and the second net scans columns — so the 2026-09-09 review's
finding 5 is a leak under **exit 0**: `DEFAULT 'ddl.canary@example.org'` on a
masked email column, every row masked, the default intact in the target's
`pg_attrdef`, and the application's next `INSERT` putting the address back into
a row (`docs/reviews/2026-09-09/evidence/ddl_default.log`).

- **Where it runs, and why there.** After `checkUniqueDomain`, because a masked
  column's default is rewritten with the masker its *rows* will go through and
  `unique.go`/`equality.go` are what overwrite `Decision.Masker`. It is still
  before `assemble`, so no key has left this stage and nothing in the target has
  been touched — all §11.1 asks of a refusal raised "at plan".
- **Which tables.** Every non-partition table of the schema, not the in-scope
  ones. §11.1 recreates a `SchemaOnly` table's DDL too, so its default reaches
  the target whether or not a row does, and a check that skipped it would let
  `--skip-table` carry the literal through. That is the opposite of
  `checkWriteBack`'s scope and deliberately so: that check is about a value this
  run will write, and this one is about text the loader will write regardless.
- **This is the one place the planner mutates `pipeline.Schema`, and the
  mutation is idempotent.** The rewrite is written back through `p.byRef`, which
  points into `p.schema`; `p.tables` holds copies — shallow ones, sharing the
  same `Columns` backing array, so the two writes land in one slot. Every read
  of a default in `ddlliteral.go` goes through `originalDefault`, and the
  rewrite records the catalog's own text on `pipeline.Column.DefaultOriginal`:
  a second `Plan` over the same in-memory schema therefore masks the *source's*
  literal again rather than the first plan's output. That is not hypothetical —
  `internal/tui` re-plans against a cached schema and
  `plan_integration_test.go` plans twice over one snapshot to compare two plans
  — and without it the default became `mask(mask(x))`, a different value on the
  second plan than on the first.
  `TestPlanningTwiceOverOneSchemaProducesOneDefault` is the guard, including the
  re-plan under a *different* key, which is that key's value rather than a
  composition of both. `internal/load/ddl` generates the target's DDL from that schema,
  and `load.SchemaFingerprint` and `load.GateFingerprint` both hash generated
  DDL, so both ends of §11.2's binding see the text the target actually
  receives. `Schema.Fingerprint`, which `internal/core` filled right after
  introspection, is the *source's* value and is not recomputed — it is what
  `--tui`'s review pinning and `internal/emit` read, and both sides of that
  comparison are pre-plan, so they still agree with each other.
- **"Strong" is email, phone, payment card, IBAN and national_id**, and nothing
  else. This text is
  SQL: a `CHECK` is full of English words and a default is full of identifiers,
  so the dictionary-backed signals of §4 over them would refuse ordinary schemas
  over labels that are not personal data. The last two joined at the 2026-09-15
  red team's A20, which put `SSN 123-45-6789` and an IBAN in a `CHECK` on an
  unmasked column and watched them cross under exit 0: THREAT_MODEL.md T1 stated
  the *name and address* half of that miss and gave the reason above, and that
  reason does not reach a strict pattern or a mod-97 checksum. `person_name` and
  `address` stay out for exactly the reason given. IBAN could not have joined
  before `textsig.ValidIBAN` began requiring the two ISO 13616 check digits —
  without them five of pagila's own film titles pass mod-97. A `CHECK (status IN
  ('active','banned'))` on a masked column is a closed value list the masker
  already honours (§5, `mask.Constraints.Checks`) and is not a refusal. The cost
  — a name or a street address in a `CHECK` on an unmasked column is not found —
  is stated in THREAT_MODEL.md T1 beside the rest of that row's admissions.
- **The default is masked under the same `mask.Constraints` the rows are, and
  `Unique` is the one that bites.** `constraintsOf` (writeback.go) deliberately
  leaves `Unique` unset, because `mask.Writable` does not read it; a generator
  does. `internal/transform` derives it from the table — its own `uniqueColumn`
  over the primary key and the single-column, non-partial, non-expression unique
  indexes — and then ORs `Decision.UniqueIndex` on top (`transform.go`), because
  classify's field asks a narrower question than the schema does (`classify.go`
  says so) and neither answer subsumes the other. `columnDefault` does both, and
  `uniqueColumn` here is a **copy** of transform's for the reason `constraintsOf`
  is one: a stage package may not import another. Taking only
  `Decision.UniqueIndex`, as the first version did, gave a masked unique column's
  default a value from the *non-unique* generator while every row got one from
  the unique generator — a `DEFAULT` no row of that column can hold.
  `TestAMaskedDefaultUnderAUniqueIndexIsMaskedAsAUniqueColumn` pins the pair by
  asserting the bytes (and asserts the two generators really do differ on that
  literal, so it cannot pass for the wrong reason), and
  `TestUniqueColumnIsSpeltAsTransformSpellsIt` carries the case list.
  **T-0164** owes the two spellings a shared home, as T-0162 owes the scanner
  one.
- **`defaultIsRewritable` declines three shapes, and each is exit 13 when the
  literal is plainly personal data rather than a pass**: an array column (§5
  masks element-wise, and a text[] default is one literal holding the whole
  array's text form — `testdata/regressions/009` is the row-side version of that
  mistake), a `json`/`jsonb`/`hstore` column (§4 replaces a document per leaf),
  and a default calling `nextval` (its literal is a relation name, and a masked
  one is a `CREATE TABLE` that fails after every table has been dropped).
- **The scanner is `internal/pipeline`'s**, not this package's, because
  `internal/verify`'s catalog pass needs the same one and a stage package may
  not import another (internal/CLAUDE.md). A second copy of a scanner that
  decides what is and is not inside the data boundary is the failure
  `internal/verify/validators.go` already records from its own hand copy of the
  classifier's validators. `internal/pipeline/CLAUDE.md` says that package holds
  no implementation, so this is a stated exception with a task against it:
  **T-0162** moves `Literal`, `Literals`, `RewriteLiterals` and `QuoteLiteral`
  into a leaf beside `internal/textsig`.
- **Arm 1 runs from the CLI (T-0161, 2026-09-14).** `internal/core`'s
  `planRequest()` fills `pipeline.PlanRequest.Key` from `r.key`, and `execute`
  now calls `keyBeforePlan` ahead of `planStage` for a run that will write —
  `resolveKey()` used to run only inside `move()`, a stage after the plan, so
  every masked default whose literal a strong validator hits reached this file
  with a nil key and was refused at exit 13 under arm 2's last clause instead
  of masked. `testdata/regressions/011-masked-column-default-holds-a-literal.sql`'s
  header is `ok`, its `masked-default:` key asserts the target's `pg_attrdef`
  holds a masked address, and this rule is a full landing: all three arms are
  live from the CLI. `internal/verify`'s catalog pass is unaffected by any of
  this — it always exempted a masked column's `DEFAULT` only where
  `pipeline.Column.DefaultOriginal` says this rewrite ran
  (`internal/verify/catalog.go`'s `rewroteDefault`), so nothing about closing
  the arm-1 gap changed what that pass judges; the pair stays a cross-package
  contract, the field written here and read there. **T-0163** is a gap that
  remains: this pass reads no index predicate and no domain `CHECK`, although
  §11.1 recreates both — the `internal/verify` catalog pass reads both, so
  what the miss costs is the earlier and cheaper refusal, not the control.
  A run with no key at all — a plan-only run with neither $LAZYSLICE_SECRET
  nor a committed `lazyslice.secret` — is not a gap either: `keyBeforePlan`
  resolves only a key that already exists for such a run rather than creating
  one nobody asked for, `columnDefault` treats that nil `Key` as "no key yet",
  not "cannot be rewritten", and reports the column on `Plan.PendingKeyDefaults`
  instead of refusing (`TestAMaskedDefaultWithNoKeyIsPendingNotRefused`).

  **The pending state is read off `PlanRequest.KeyPending`, not inferred from
  a nil `Key` alone (2026-09-14 review of T-0161).** The first landing took
  "this is a plan-only run" from `p.req.Key == nil`, held up only by two
  hand-kept copies of the same condition agreeing in `internal/core/run.go`
  (`keyBeforePlan` and the plan-only early return in `execute`) — nothing
  pinned them together, and a fourth entry point calling `planStage` without
  `keyBeforePlan` would have reached this branch with a nil key on a writing
  run and recreated the source's literal in the target's DDL with no refusal.
  `internal/core` now sets `KeyPending` from one shared `planOnly()` method
  read by both `keyBeforePlan` and `planRequest`, and this file's gate is
  `shapeRewritable && Key == nil && KeyPending`: a nil `Key` with `KeyPending`
  false — the drift case — falls through to the same refusal a
  not-rewritable shape gets, rather than being silently skipped.
  `TestAMaskedDefaultWithNoKeyAndNoKeyPendingIsRefused` (`ddlliteral_test.go`)
  and `internal/core`'s `TestKeyBeforePlanRunsBeforePlanStage` (the AST
  ordering pin, in the style of `refingerprint_test.go`'s) hold the two
  halves. `resolveKeyIfPresent` also stopped sharing `resolveKeyState` with
  `resolveKey` in the same review round: it no longer calls `repo.Protect`,
  because that call appends to `.gitignore` and can refuse on a tracked
  secret file, and a plan-only run (`lazyslice plan`, and the TUI's Preview
  pass) must not mutate the repository or hard-abort merely by being asked
  what it would mask — see `internal/core/CLAUDE.md`'s own note on this, and
  `internal/core/run.go`'s comment on `resolveKeyIfPresent`.
- **Types are objects too** (`checkTypeLiterals`, the 2026-09-15 red team's A4b,
  A11 and A12). §11.1 recreates an enum with `CREATE TYPE ... AS ENUM ('a','b')`
  and replays a domain's whole `CREATE DOMAIN` text, so an enum label and a
  domain's `DEFAULT` and `CHECK` cross into the target exactly as a column
  `DEFAULT` does — and this pass read `p.tables` only, so all three crossed
  under exit 0 with an address and a phone number in the target's catalog. A
  domain `CHECK` is the gap **T-0163** already named; the other two were named
  nowhere. All three are read now, and **none is rewritten**: every row of every
  column of an enum type references a label *by value*, so masking one would
  either break the column or silently remap rows, and a domain's `DEFAULT`
  belongs to the type rather than to a column, so there is no single masker
  whose output is the right replacement — the same argument `columnDefault`
  makes for a shape it declines. So `CodeTypeLiteral` at **exit 13**, naming the
  type and the label's *ordinal*, never the label text (THREAT_MODEL.md T4).
  `typeliteral_test.go` holds both directions, including that an ordinary status
  enum and a money domain do not refuse a run.
  - **The escape is `--allow-type-literal TYPE=REASON`** (the T-REDFIX review's
    fourth finding). It was `--skip-table`, which cannot clear this refusal by
    any route: `--skip-table` drops a table to *schema only*, so its DDL — and
    every type that DDL names — is still recreated, nothing in `internal/core`
    prunes `Schema.Enums` or `Schema.Domains`, and `internal/load/ddl`'s
    `typeOrder` recreates every one of them regardless. A source schema with one
    enum label or domain definition a strong validator hits was therefore
    **permanently unrunnable**, under a refusal naming a flag with no effect on
    it. The opt-out is §8's per-column `--unmask` for the object class that is
    not a column: a reason is required, the name is resolved against the
    source's own enums and domains in `internal/core` (`resolveType`, so a name
    that matches nothing is exit 2 rather than a rail the operator believes they
    lifted), and the types it names go onto `Plan.AllowedTypeLiterals` so that
    **`internal/verify`'s catalog pass honours the same list** — an escape the
    planner grants and the verifier refuses would load the target and then exit
    9 over the very object the operator was told they had allowed.
    `TestAllowTypeLiteralClearsTheRefusal` and
    `TestAllowTypeLiteralForAnAbsentTypeIsNotRecorded` are this side's guards,
    `TestAllowedTypeLiteralIsExemptFromTheCatalogPass` the other's.
    **Owed:** ARCHITECTURE.md §8's flag table does not list the flag and §11.1's
    type-literal paragraph still names `--skip-table` — tracker **T-0185**;
    neither file was in this task's paths. The opt-out is also **not** recorded
    in `lazyslice.yml`, where `--unmask` is, so it must be passed again on every
    run — tracker **T-0186**.
- **The guards.** `ddlliteral_test.go` holds all four arms without a database,
  including the one the CLI could not reach before T-0161: a masked column's
  default really being rewritten, to the byte, to what `mask.Apply` gives for
  that literal, under the same `mask.Constraints` its rows go through. Replacing
  `checkDDLLiterals`'s body with `return nil` fails four of its tests.

## The validator set widened, indexes joined, and a pattern got read (T-0189, 2026-09-15 round-2 red team)

R2-07, R2-08, R2-09 and R2-10 (`docs/reviews/2026-09-15-redteam/round2-still-
leaking.json`) all reduce to one sentence: the object classes this file read
and the category list its validators covered were each a *subset* of what
§11.1 recreates and of what the row pipeline masks, and the gap between the
two was where a value crossed. Three changes, closed together because the
tracker task naming them (T-0189) promotes **T-0163** in the same landing.

- **`strongValidators` runs every category that is a parse or a shape,
  not only the original five.** `network_id` (`ValidIP`/`ValidMAC`),
  `online_id` (`ValidURL`), `person_name` (`Dictionary().NameShape`),
  `address` (`AddressShape`) and `free_text` (`Dictionary().ProseName`) join
  email, phone, the Luhn and IBAN halves of `financial_account`, and
  national_id. The argument that had kept `person_name` and `address` out —
  and, by the same reasoning, kept `free_text` from ever joining — is an
  argument about *identifiers* in the surrounding SQL text ("a `CHECK` is
  full of English words"), and this file's scanner (`pipeline.Literals`)
  never returns an identifier, only the quoted string constants a deparsed
  expression carries. R2-07 is a table `CHECK` carrying a person's full name
  and one carrying a postal address, both crossing under exit 0; R2-09 is the
  same two shapes, plus a special-category sentence, inside an enum label, a
  domain `CHECK`, a domain `DEFAULT` and a generated expression — the four
  object classes `checkTypeLiterals` already read, which the widened set now
  judges too, with no change to that function at all. **The special-category
  sentence is not closed by this list, only narrowed by accident** (T-0189
  fix round, 2026-09-15 review, finding 4): `pipeline.CatSpecial` has no
  entry in `strongValidators` at all, so R2-09's own canary is caught only
  because its date supplies `AddressShape`'s digit — strip the date and a
  health/special-category sentence with no name pair and no digit still
  crosses at exit 0. Tracker **T-0198** carries a validator for it.
  - **`credential` (`LooksSecret`) does not join, and this was found rather
    than reasoned about.** The first landing of this change included it, on
    the same "every parse or shape" argument, and three of
    `testdata/torture/`'s ten real-world schemas (calcom, gitlab, discourse)
    and regression 006 all refused over it: `LooksSecret` is an entropy
    guess over *any* string sixteen characters or longer carrying two of
    {lowercase, uppercase, digit}, not a parse or a dictionary shape, and a
    column `DEFAULT` calling `nextval` embeds exactly that shape by
    construction — the literal this file reads out of
    `nextval('public."AccessCode_id_seq"'::regclass)` is the sequence's own
    quoted, mixed-case relation name, and `refuseUnmaskedLiteral`/
    `refuseNotRewritable` scan every literal `pipeline.Literals` finds in a
    `DEFAULT`, including that one, with no notion that a `nextval` argument
    is a relation name and not a value. `make torture` is what found this,
    not a unit test: the four synthetic fixtures this task added all use
    `person_name`/`address`/`free_text` values chosen to be unambiguous, and
    none of them is shaped like an identifier, so the false-positive class an
    entropy guess is prone to had nothing to trip it. The lesson generalises
    beyond this one category: a validator's own precision, not the
    "dictionary vs. literal" argument this task answers, is still the
    question for anything added here in future, and `strongValidators`' own
    comment carries the measured evidence rather than only the conclusion.
- **A pattern operand is exempt from rewriting only, never from detection**
  (R2-10). `strongHit` used to return `""` unconditionally for `lit.Pattern`;
  it now runs `pipeline.StripPatternMeta` over the pattern's text first —
  stripping the wildcard and anchor syntax a pattern operator reads as
  metacharacters and unescaping a backslash-escaped one to the literal
  character it stands for — and validates what is left.
  `CHECK (email !~ '^ceo@bigcorp\.example$')` reduces to
  `ceo@bigcorp.example`, intact; `CHECK (email LIKE '%@%.%')`
  (`testdata/nasty.sql`'s own trap, T-0134's reason `Pattern` exists at all)
  reduces to `@`, which nothing validates, so the fixture that motivated the
  exemption is not what this closes. Rewriting is unaffected: `columnDefault`'s
  `RewriteLiterals` callback still declines every `Pattern` literal
  unconditionally, because rewriting one changes what the database accepts,
  which detecting one does not.

  **This task's own claim that "a pattern operand cannot appear in a
  `DEFAULT` anyway" was wrong, and the fix round found the bug it hid**
  (T-0189 fix round, 2026-09-15 review, finding 1). A `DEFAULT` is a value
  expression of the column's own type, which is not the same as "never
  boolean, never a `CASE`" — nothing stops `DEFAULT (email !~
  '^ceo@bigcorp\.example$')` on a boolean column, or a pattern operand inside
  a `CASE` arm of any type. Because that case was believed impossible,
  `columnDefault`'s success path never re-ran `strongHit` over what the
  callback above declined: `RewriteLiterals` leaves a declined literal's text
  exactly as it stood and still reports `ok == true`, so a `Pattern` (or
  empty-text) literal in an otherwise-rewritable masked column's `DEFAULT`
  was written back to `pipeline.Schema` and returned as success, unexamined —
  R2-10's own gap, reopened on the one arm R2-10's fix never reached.
  `columnDefault` now re-scans every literal the callback declined against
  `strongHit` before accepting the rewrite, refusing exactly as
  `fixedExpression` does on a `CHECK`.
  `TestRedTeamR210FixRoundPatternOperandInARewritableDefaultIsStillDetected`
  is the guard.
- **`tableDDLLiterals` walks `t.Indexes`, closing T-0163's plan-time half.**
  Every index, in name order (the same determinism reason the constraint
  loop sorts), through `fixedExpression` — the same never-rewritten rule a
  `CHECK` gets, because an index predicate is the application's and
  `internal/load/ddl` replays it verbatim. `idx.Def` is `pg_get_indexdef`'s
  whole text (name, columns or expression, and a partial index's `WHERE`),
  and `namedColumns(idx.Def, t)` — already the constraint loop's own
  text-token match — decides which of the table's columns the index names,
  so an index over an unmasked column still gets the `--unmask` escape and
  one over a masked column still refuses outright with no rewrite arm, the
  same shape a `CHECK` already had. `internal/verify/catalog.go`'s own
  `pg_index` read closed the *catalog*-pass half of T-0163 in T-0134's review
  round; this is the half that remained, and the one that matters more, since
  a plan-time refusal is before anything is dropped and a catalog refusal is
  after.

  **`namedColumns` read the index's own table name as a column (T-0189 fix
  round, finding 3).** `idx.Def` is `pg_get_indexdef`'s whole text, which —
  unlike `pg_get_constraintdef` — always opens with `CREATE INDEX name ON
  schema.table`, so a column that happened to share the table's own name was
  matched as though the predicate named it: table `public.items` with
  columns `{id, items, email}`, index `... ON public.items USING btree
  (email) WHERE (email = '...')`, returned `named == [items email]`. Two
  consequences followed from the one false match: `anyMasked` could flip true
  off the phantom column (exit 13 with no escape named, instead of exit 12
  naming the real one), and `refuseUnmaskedLiteral`'s opt-out loop returns
  `nil` on the *first* named column carrying `--unmask` — so an opt-out on
  the phantom column silently cleared a genuine hit on a column it was never
  about. `namedColumns` now scans only the text after `" USING "`, which
  every index carries (the access method is never omitted) and which is
  always past the `ON schema.table` clause; an exclusion constraint's own
  `EXCLUDE USING gist (...)` has no table name before that point either, so
  the trim only drops a keyword there, never a match target.
  `TestNamedColumnsDoesNotMatchTheTableNameInAnIndexDef` pins the scanner
  directly and
  `TestRedTeamR211FixRoundAnOptOutOnATableNamedColumnDoesNotClearAGenuineHit`
  pins the fail-open end to end.
- **`address`'s validator was too loose for a one-hit refusal (T-0189 fix
  round, finding 2).** `textsig.AddressShape` — "a digit somewhere, and at
  least two words that carry a letter" — is calibrated for
  `internal/verify`'s second net, which asks it of a whole column and fails
  only once `validatorThreshold` of many rows agree (`validators.go` marks
  it explicitly not strong, for exactly this reason); this scanner asks it of
  one literal and had been treating the bare shape as a one-hit refusal since
  the widening above. Measured against ordinary `CHECK` value-list and
  enum-label text it also hits "Basic 1 user", "Pro 5 users", "tier 2 plus",
  "level 1 support", "P1 High Priority", "Top 10 sellers", "Building 4
  Lobby" and "version 2 draft" — pricing tiers and priority labels, never a
  person's address — and a hit on a masked column is exit 13 with no escape
  at all, while a hit on an unmasked one is exit 12 whose only escape,
  `--unmask`, says the whole column is not personal rather than that this one
  literal is not. `addressLiteralShape` corroborates it the way
  `ValidNationalIDStructured` already corroborates the national_id entry
  above: a feature that is actually diagnostic, not merely necessary. Almost
  every real address line carries a street-type word (`addressSuffixWords`:
  street, avenue, road, lane, drive, and their kin), and none of the false
  positives above do; R2-07's own canary, "1742 Kestrel Hollow Lane, Ashford
  VT 05024", keeps its hit through "Lane". `textsig.AddressShape` itself is
  untouched — `internal/textsig` is outside this task's paths, and
  `internal/verify`'s second net still wants the loose shape it already has —
  so the corroboration is a local wrapper, duplicated in
  `internal/verify/catalog.go`'s own `strongCatalogHit` for the reason every
  other entry in that list is already a duplicate.
  `TestAddressStrongHitNeedsAStreetSuffixWord` is the guard.
- **Guards.** `ddlliteral_test.go` gained
  `TestRedTeamR207PersonNameAndAddressInCheckAreRefused`,
  `TestRedTeamR208PartialIndexPredicateIsReadAtPlan`,
  `TestRedTeamR210PatternOperandCarryingAValueIsStillDetected`,
  `TestPatternMetaStripping`,
  `TestRedTeamR210FixRoundPatternOperandInARewritableDefaultIsStillDetected`,
  `TestNamedColumnsDoesNotMatchTheTableNameInAnIndexDef`,
  `TestRedTeamR211FixRoundAnOptOutOnATableNamedColumnDoesNotClearAGenuineHit`
  and `TestAddressStrongHitNeedsAStreetSuffixWord`; `typeliteral_test.go`'s
  `TestRedTeamTypeLiteralsAreRefused` gained a person's name in an enum
  label and a postal address in a domain default, and
  `TestOrdinaryTypesAreNotRefused` is unchanged and still passes, which is
  the guard against the widened set refusing an ordinary schema. None of
  these needed a database; `make torture` against the ten real schemas and
  `testdata/regressions/` is what caught the `credential` false positive
  that a hand-picked fixture could not have.

## `special_category` gets a value validator, and the broadened net is scoped to a `DEFAULT` and a generated expression only (T-0198, 2026-09-16, and its fix round)

`pipeline.CatSpecial` — health, religion, sexual orientation and gender
identity, ethnicity, trade-union membership, political opinion, the six
sub-categories `rules.yml`'s own `special_category` name pattern already
masks a column on — had no value validator anywhere until T-0198
(`textsig.SpecialCategoryVocabulary`, a precision-over-recall term list,
internal/textsig's own paths). It joins `strongValidators`/`strongHit` as an
eleventh entry, the same footing every other parse or shape already has;
`internal/verify/catalog.go`'s `strongCatalogHit` and, since the fix round
below, `internal/verify/validators.go`'s row-scanning second net carry the
identical entry. This closes the round-3 red team's own canaries (finding
15, `docs/reviews/2026-09-15-redteam/round3-still-leaking.json`) on the
ordinary strongHit path and costs nothing on `make torture`: none of the ten
real schemas' generated data happens to carry a special-category term.

**A second, wider change landed the same day and did not survive
re-measurement.** T-0198's own tracker log proposed refusing a masked
column's own `CHECK`, generated expression, index predicate or
non-rewritable `DEFAULT` on *any* literal, whatever it parses as, once no
strongHit and none of the closed-value-list/empty-collection/Pattern
exemptions applied — with an explicit conditional: if more than two of the
ten schemas newly refused, land the wider rule behind the existing `--unmask`
escape rather than accept the cost. The first landing measured five newly
refusing (after an empty-collection exemption for a masked column's own
default, which is `unrewritableLiteral`'s own comment and ARCHITECTURE.md
§5's rule restated, not invented for the measurement) and landed as default
anyway, with `docs/TORTURE.md` recording four as still needing curation.

**The fix round (2026-09-16) found curating the rest was not merely large,
but twice genuinely unsafe, and narrowed the rule instead of finishing the
curation.** `fixedExpression`'s scoping judges an object — a `CHECK` or an
index, which can name any number of columns — whole once *any* named column
is masked, with no notion of which column a given literal is actually about.
Two real schemas (not a synthetic fixture; a reduced fixture cannot
reproduce a real table's own column collisions) found where that breaks:

- **odoo's `res_partner_check_name`**
  (`CHECK ((type = 'contact' AND name IS NOT NULL) OR type <> 'contact')`)
  names both `type` and `name` on `res_partner`, Odoo's CRM contacts table —
  `name` there is a real person's or company's name, correctly masked. The
  literal `'contact'` is about `type` and never about `name`, but the only
  escape the rule could offer was `--unmask public.res_partner.name`: the
  actual name the column exists to protect.
- **odoo's `res_partner_mobile_partial_gin_idx`**, a GIN trigram index over
  `regexp_replace(mobile::text, '[\s\\./\(\)\-]', '', 'g')` — `mobile` is a
  real phone number, the index's only masked column, no ambiguity at all —
  and its two non-empty literals are a punctuation character class and a
  regexp flag, neither a value about the phone number. The only escape was
  `--unmask public.res_partner.mobile`: the phone number itself.

Curating either would have shipped real personal data under the one flag
whose entire purpose is to say a column carries none — a defect in the
rule's own design, not a curation cost to accept. **The fix: the broadened
net (`unrewritableLiteral`, still doing the same work — a Pattern operand, a
closed value list, an empty collection, and, new in this fix round, a
literal immediately cast to a non-text type such as `'-1'::integer`, all
still exempt) now runs only for a `DEFAULT` and a generated expression,
never for a `CHECK`, an exclusion constraint or an index of any kind.**
Both of those two object classes are a single column by construction —
`tableDDLLiterals` always calls `fixedExpression` with `named ==
[]string{col.Name}` for a generated expression, and `columnDefault`'s own
fallback loops always judge `col.Name` itself — so the ambiguity both odoo
cases turn on cannot arise there. `fixedExpression` takes a `broadNet bool`
now (true only for the generated-expression call; false for the constraint
and index loops), and `maskedSubset(named, masked)` — the renamed, list-
returning `anyMasked` — is what both decides "unmasked" (`len == 0`) and
supplies `refuseUnrewritableLiteral`'s `--unmask` escape when it fires. A
`CHECK` and an index still refuse on a strongHit exactly as §11.1's original
rule always has; only the newer, escape-free-when-masked net that refused on
a literal no validator recognised at all is gone from those two classes.
`internal/verify/catalog.go`'s `unrewritableKind` carries the identical
scoping (`kindDefault`/`kindGenerated` only), and its own `maskedNamedColumn`
requires *exactly one* masked column match rather than merely "any", belt
and braces alongside the plan-side change.

**Re-measured, `make torture` needed none of the curation the first landing
called for**, gitlab and supabase-auth included, and metabase's own two
flags for an expression index — landed to close the first measurement —
turned out not to be needed either once indexes stopped carrying the wider
net. The suite is back to the original twenty-seven flags, nineteen
`--unmask`, unmoved by this task net of the reversal. docs/TORTURE.md's own
"T-0198" section and "the fix round" carry the full account, including the
`gofmt`/`staticcheck`-clean final diff; ARCHITECTURE.md §11.1 and
THREAT_MODEL.md T1 carry the corresponding amendments.

`refuseUnrewritableLiteral` also gained a `masked []string` parameter in the
same fix round (a reviewer's own finding, independent of the scoping
change): its message used to name only the object (an index or a
constraint's own name — "the index ... is masked", which is never true, an
index is not masked, its columns are) and offered no escape a real run could
act on. It now names the masked column(s) the object's text actually
carries and points `--unmask` at the first of them. In the current call
graph this branch is exercised only in the degenerate case where the object
*is* the single masked column already (a `DEFAULT` or a generated
expression, where `object == masked[0]` always) — the exact multi-column
case it was built for can no longer reach this function at all, now that a
`CHECK` and an index never call it under the broadened net. It is kept
anyway: correct and harmless for the paths that do reach it, and it closes
off the wrong message from ever being possible again if a future caller
passes `broadNet` true for an object that can name more than one column.

## The root's own row count can exceed `--take` (T-0288, ARCHITECTURE.md §3.7)

The root table is not exempt from the parent rule: any selected row, reached
by any edge, pushes its parent onto the queue as `PARENT_ONLY`, uncapped —
and that parent can be the root table itself, through an edge other than the
one that reached the root's own seeded rows. Pagila proved this is not a
corner case: `payment` is a child of both `rental` and `customer`, a
payment's own `customer_id` occasionally differs from its rental's, and
`--root public.customer -n 200` against the real fixture plans 204
customers, not 200 — verified with `SELECT count(*) FROM payment p JOIN
rental r ON p.rental_id = r.rental_id WHERE p.customer_id <> r.customer_id`
restricted to the 200 lowest-id customers' rentals, which returns the same
four extra customer ids the plan pulls in. This is referential completeness,
not a defect (root CLAUDE.md, THREAT_MODEL.md): the alternative is a
`payment` row in the target whose own foreign key cannot validate.

- **`run.rootSeedCount`** (`plan.go`, set in `walk` right after `seedKeys`)
  is what `--take`/`--where` actually chose, before the closure adds
  anything. `rootWhy` (`plan.go`, called from `assemble`) compares it against
  the root step's final `ks.Len()` and, only when the closure grew the table
  (`M > 0`), replaces the walk's plain `"root"` `Why` with `"root: N chosen,
  M pulled in by references"`. `M == 0` — every fixture in this package except
  the one built to prove this — leaves `Why` exactly `"root"`, unchanged.
- **The root keeps its name on that line.** `rootWhy` writes `"root: N
  chosen, M pulled in by references"`, not the bare count, because
  `internal/tui/collect.go`'s `planRow.root()` finds the plan screen's root
  row by its `Why` (exactly `"root"`, or the `"root: "` prefix) to strike
  `--root` and `--skip-table` through on it; Pagila's own `--root
  public.customer` grows the root every time (204 = 200 + 4).
  `internal/tui/collect_test.go`'s drift guard pins both strings in this
  file.
- **Two integration tests, not a `testdata/` fixture.** §3.7's shape needs
  two tables and one extra foreign key column
  (`TestPlanRootLineNamesRowsPulledInByReferences`,
  `TestPlanRootLineIsUnchangedWhenNothingIsPulledIn`,
  `plan_integration_test.go`'s own `rootClosureSchema`), the same reduction
  the diagnosis above used, rather than a third shared fixture: `nasty.sql`
  and pagila are both read by suites outside this task's paths that count
  their tables, exactly the reason `extraSchema` above lives in this file
  and not in `testdata/`.

## A framework metadata table bypasses the lookup rule, and says so when it cannot keep its promise (T-0314)

`findLookups` forces `schema_migrations`, `ar_internal_metadata` and the rest
of `pipeline.IsFrameworkMetadataTable`'s list to a `Lookup` step regardless of
reachability, the mask check and the 1,000-row lookup ceiling: dogfood
session 1 found `schema_migrations` reached by no foreign key at all — the
ordinary shape of migration bookkeeping, not a corner case — so §3's own "at
least one incoming edge" clause left it `SchemaOnly` and a fresh checkout
re-ran every migration against the target. `frameworkMetadataWhy` is the
`plan.step` line it gets instead of the bare `"lookup"` every other Lookup
step carries, and `ar_internal_metadata` gets a second sentence when
`pipeline.ArInternalMetadataEnvironmentColumns` finds its key/value columns
where Rails put them, announcing the rewrite `internal/load` performs before
any row moves rather than leaving it for an operator to discover by reading
the target afterwards.

**"Copied whole" was not always true, and the plan used to say it anyway**
(the T-0314 review round, finding 1). Bypassing the row ceiling does not
widen what one read can actually return: `internal/extract`'s own Lookup
read (`lookupLimit`, `internal/extract/sql.go`) carries the identical
1,001-row bound this package's `boundedCount` probe does
(`countProbeLimit`), for the same THREAT_MODEL.md T9 reason — no statement
this tool sends may scan a whole table unbounded. A framework table past
1,000 rows (a mature Rails app's migrations, easily) is therefore truncated
by that bound regardless of what `findLookups` decides, and the first
landing's `frameworkMetadataWhy` did not look at `boundedCount`'s own answer
at all: every framework table's line read "copied whole regardless of
reachability", true or not. `frameworkMetadataWhy` now takes `n`, the same
count `findLookups` already fetched, and switches the sentence once `n`
exceeds `lookupRowCeiling`: "has more than 1000 rows -- only the first 1000
(ordered by identity) are copied; T-0347 tracks copying it in full" — naming
the tracked gap rather than a bare number, since `--take`, `--cap` and every
other row-count flag on this tool are about the *slice*, not about a lookup
table's own bound, and this is not a flag an operator can raise today.
`TestPlanFrameworkMetadataTableOverTheLookupCeilingSaysSo`
(`plan_integration_test.go`) is the guard, over a standalone 1,002-row
fixture; reverting `frameworkMetadataWhy`'s `n`-aware branch fails it.

**What is still open, and stays open on purpose.** Raising the bound itself
— so a framework table past 1,000 rows is actually copied whole rather than
merely told about its own shortfall — needs `internal/extract`'s
`lookupLimit` to grow or gain an explicit, registered larger bound for a
framework table specifically, and `internal/extract` is outside every task
that has touched this file so far. **T-0347** is that gap, filed by the
developer who found it; this package's own half is the honest sentence
above, not a bound it cannot see past.

**A second, independent gap the T-0314 review round found while measuring
this one: `internal/verify`'s second net does not exempt a framework
metadata table from its own scan at all**, beyond the narrow dense-sequence
case `Decision.NeverMasked` already gates there. A framework table whose
real values happen to validate strongly — a Rails migration timestamp that
clears the Luhn check, the exact shape dogfood session 1 hit — refuses the
whole run at exit 9 even though this package and `internal/classify` both
correctly leave it unmasked. `internal/classify/CLAUDE.md`'s own T-0314
section has the measurement; **T-0348** is where it is filed, since
`internal/verify` is a third stage package neither this task nor T-0314's
original one may touch.

## Collecting refusals instead of stopping at the first (T-0318, 2026-09-24)

Dogfood session 1's own transcript is the brief: nine runs to reach a green
`verify`, each one refused on the next single cause — one join table with no
identity (a second, identical one was implied by the schema but not named
until the first was cleared, costing a run of its own), then ten masked
columns under unique indexes named one per run, then the second net naming
one column per run. Four checks now collect every refusal they can find
instead of returning the moment they find one: `resolveIdentities`
(`identity.go`, `plan.refused.no_identity`), `checkWriteBack`
(`writeback.go`, `plan.refused.unwritable`), the skip-cannot-drop-a-parent
branches of `applySkipAndPrivileges` (`plan.go`, `plan.refused.skip_parent`)
and `checkUniqueDomain` (`unique.go`, `plan.refused.unique_domain` and its
`plan.refused.equality_group` sibling). Every other refusal in this
package — `checkWhere`, `checkRecreatable`, `chooseRoot`, the
unreadable-table and root/skip-table refusals inside
`applySkipAndPrivileges` itself, the row and memory budgets, `checkFKPairRefusal`
and `checkDDLLiterals` — is still fail-fast, unchanged: each of those is a
usage mistake or a single hard stop, not one of several independent causes an
operator would otherwise fix one exit-12 at a time.

- **`run.collect` and `run.refusals` (`plan.go`) are the whole mechanism.** A
  collecting check calls `p.collect(r)` and keeps going — over the next
  table, the next column, the next equality group — instead of `return r`.
  `plan()` itself still checks `len(p.refusals) > 0` at exactly two points:
  right after `resolveIdentities`, and it checks `!p.inScope[root]` there
  specifically rather than the refusal count, because the root is the one
  table `resolveIdentities` cannot drop and carry on from — `walk`'s
  `seedKeys` has nothing to order the seed by without it — so a root with no
  identity stops the pass with whatever it already found rather than trying
  to walk a queue that was never seeded. The second and only other check is
  the very last thing `plan()` does before `checkDDLLiterals` and
  `assemble`: every collecting check has run by then, so this is the one
  place a non-empty `p.refusals` can be returned instead of building a plan
  the operator cannot act on. Nothing in between the two checks ever asks
  `len(p.refusals) > 0` early — `checkWriteBack` and
  `applySkipAndPrivileges` return `nil` after collecting, on purpose, so a
  cause found early in the pass never hides one only the walk or the
  unique-index check would have found.
- **A no-identity table is dropped, not merely refused.** `resolveIdentities`
  treats `CodeNoIdentity` the way `applySkipAndPrivileges` already treats an
  unreadable child-only table: collect the refusal, `p.drop` the table out of
  `inScope`, and move on. Every later stage that reads `p.inScope` — the
  walk's parent and child loops, `findLookups`, `checkUniqueDomain`'s
  `plannedRows` — already treats a dropped table as absent, which is the
  entire reason dropping was the right verb here rather than teaching four
  more functions to also skip a table with no identity. Any *other* identity
  refusal — an explicit `--key` naming a system column, a column the table
  does not have, or one that is not unique — is still fail-fast: those are
  mistakes about one flag, not an independent cause among several.
- **A run that collects anything now sends more statements to the source
  than the same refusal used to.** `checkWriteBack`'s and
  `applySkipAndPrivileges`'s own doc comments used to promise "before a key
  is fetched" as a ceiling on the whole run, and that promise still holds for
  *those checks' own* position in the pipeline — neither moved — but it no
  longer holds for the run as a whole: finding a `unique_domain` cause in the
  same pass needs the walk, which reads keys, so a schema with both an
  unwritable column and an unrelated unique-index collision now reads more
  than the one privilege-pass statement before it refuses.
  `writeback_test.go`'s two query-count assertions were loosened from "the
  privilege pass and nothing else" to "the privilege pass and this
  one-table fixture's own root seed read, and no more" for exactly this
  reason — the trade is deliberate: more reads, in exchange for every
  independent cause in one run instead of one per run.
- **`run.fail` (`plan.go`) is what every call site downstream of
  `applySkipAndPrivileges` wraps its own error in before returning it,
  instead of the bare `return nil, err` every one of them used to be.** A
  fail-fast refusal this package never taught to collect — an unreadable
  parent, a row or memory budget, an FK-pair refusal, a bad explicit
  `--key` — still stops the run the moment it happens, and `fail` is a
  no-op the moment `p.refusals` is still empty, which is every ordinary run:
  `err` comes back unchanged and every existing refusal reaches core exactly
  as it always has. What it exists for is the rarer combination where a
  fail-fast cause arrives *after* this pass has already collected
  something — `--skip-table` naming one table the slice needs as a parent
  (collected) beside a second, unrelated table the role cannot read at all
  (fail-fast, inside the very same `applySkipAndPrivileges` call) is the
  shape `TestAFailFastRefusalJoinsWhatWasAlreadyCollected`
  (`refusals_test.go`) pins. Without it, `plan()`'s `if err != nil { return
  nil, err }` would have returned the bare fail-fast `*Refusal` and the
  skip_parent refusal already sitting in `p.refusals` would never have
  reached core at all — collected, and then silently dropped, which is worse
  than never collecting it. `fail` joins a `*Refusal` onto the list and
  returns the aggregate; anything else (a context cancellation, a real
  driver error) is not a refusal to collect and passes through unchanged.
- **The aggregate type is `Refusals` (`refusal.go`), `[]*Refusal` with an
  `Error() string` that renders one numbered line per member.** `plan()`
  returns it, as the plain `error` interface, exactly where it used to
  return a bare `*Refusal` — the signature `Planner.Plan(...) (*Plan, error)`
  (§2) has not changed and could not without moving every caller. `Code`,
  `Exit`, `Table`, `Column`, `Args` and `Message` are deliberately not fields
  of `Refusals` itself: `internal/core.asStop` and `cmd/lazyslice`'s `report`
  both read a single refusal's shape to build the process's one exit code and
  the one line printed after the transcript, and the first member — first in
  the order the four checks above run, which is also the order they are
  listed above — answers for both, because every one of them is already
  ADR-005's exit 12 regardless of which is first. "Exiting 12 with the first
  code" (the brief's own words) is therefore true by construction and not by
  a separate rule comparing exit codes: there is only ever one exit code
  among the four to begin with.
- **`Refusals.Unwrap() []error` is what keeps every existing single-refusal
  caller correct without being rewritten.** `errors.As(err, &singleRefusal)`
  — `internal/core/names.go`'s own `*plan.Refusal` case in `asStop`, and
  every "held without a database" unit test in this package that predates
  T-0318 (`writeback_test.go`, `fkpair_test.go`, the not-recreatable case in
  `plan_test.go`) — still finds the first member through the standard
  library's own multi-error unwrapping, unchanged. What does *not* survive
  unwrapping is a direct call to one of the four collecting functions
  themselves: `checkUniqueDomain()` and `checkWriteBack()` now return `nil`
  on the collected path and leave the finding in `p.refusals`, so
  `unique_test.go` and `equality_test.go`'s three refusal-asserting cases
  were rewritten to read `p.refusals` after the call rather than the call's
  own return value — the same shift `refusals_test.go`'s own fixture assumes
  throughout.
- **`internal/core.planStage` intercepts `plan.Refusals` before it ever
  reaches `asStop`.** `asStop` builds exactly one `*Stop` from exactly one
  refusal's fields, and a *Stop* is one event's worth of `Code`/`Exit`/
  `Args` — it has nowhere to put a second refusal. `run.reportPlanRefusals`
  (`internal/core/run.go`) is the new, narrow escape hatch: it sends one
  `event.Error` per member, in collection order, then returns the `*Stop`
  the first member's fields build, marked `sent` so `run.report`'s own
  single-event send does not print that first refusal a second time — the
  identical guard `refusalStop` already uses for the discovery ladder's own
  refusal (`internal/core/CLAUDE.md`, "Decisions made during
  implementation"). `internal/core/plan_refusals_test.go` pins both halves —
  one event per member and the no-double-send guard — without a database,
  the way `refusal_test.go` pins the ladder's own single-send guard.
- **The `--key` hint on `CodeNoIdentity` names the table's own columns when
  the table has three columns or fewer** (`keyHint`, `smallTableKeyColumns`,
  `identity.go`). A Rails `habtm` join table is exactly two foreign-key
  columns and no primary key at all — the composite of the whole row *is*
  its identity — so past the ladder's every other rung failing, naming those
  columns back to the operator turns "pass `--key TABLE=col,col`" from a
  placeholder into a command they can paste unedited. The cutoff is the
  table's total column count, not the count of columns the hint ends up
  printing: a three-column table with one generated column still gets the
  other two named, but a four-column table gets the bare placeholder even if
  three of its columns are generated, because past three columns nothing
  about the shape says "this is a join table" any more and a wrong guess is
  worse than the placeholder it would replace. A generated column is left
  out of the printed list for the same reason `pseudoKeyColumns` already
  excludes one: the target recomputes it, so it can never be part of a
  `--key` an operator passes on the *source*.
- **The fixture is Go, not a `testdata/regressions/*.sql` file.** Every other
  entry in that directory reduces a real failing run against one of the ten
  `testdata/torture/` schemas, and this defect is a pipeline-ordering one —
  what stops when — rather than a shape one particular schema carries; the
  closest real analogue (`testdata/regressions/010`, T-0132's own foreign-key
  equality defect) is itself pinned as a unit test first and a regression
  file second, for the identical reason `unique_test.go`'s own header states:
  the evidence would otherwise be Docker-gated and outside what a plain `go
  test` runs. `refusals_test.go`'s fixture builds two Rails-shaped join
  tables with no identity and two credential columns under a unique index
  too narrow for `credential_unique` to widen, and drives
  `resolveIdentities`/`checkUniqueDomain` directly, the same "held without a
  database" pattern `writeback_test.go` and `unique_test.go` already use for
  the two checks it exercises.

## The `--key` hint's review round: a nullable or incomparable column, and a hint that repeats a failed probe (T-0318 review, 2026-09-24)

Landing the hint above found two ways it could make a run worse rather than
shorter, both in the same review pass, before either shipped past this
package's own tests.

- **`smallTableKeyColumns` no longer suggests a column that is nullable or of
  a type the key encoding cannot compare.** The first version suggested every
  non-generated column of a table with three columns or fewer, which is not
  what §3.4's own pseudo-key rung admits and for the same two reasons that
  rung's doc comment already states at length. A nullable column passes
  `count(DISTINCT (a, b))` — a tuple holding a NULL counts as distinct — and
  then loses every row with a NULL in a key column at `readKeys`, which is
  `testdata/README.md` trap 12 reached through an operator pasting this exact
  hint rather than through the probe trap 12 names; `create_table :a_b, id:
  false { t.belongs_to :a; t.belongs_to :b }` is the ordinary Rails shape that
  produces two nullable foreign-key columns and no primary key, not a corner
  case. A NOT NULL column of a type with no default btree opclass — `json`,
  `xml`, a geometric type — does not even reach a refusal if suggested:
  `count(DISTINCT (...))` and the join's `ORDER BY` both die inside pgx with
  "could not identify a comparison function for type json", which carries
  none of §3.4's exit code or remedy. `comparableType` (`identity.go`) is
  `isDiscriminator`'s own type test, pulled out so this rule can ask it of a
  column that is neither a foreign key nor discriminator-named — comparability
  is a property of the type alone and was never tied to either. A column
  excluded for being generated is still simply left out, as before; a column
  excluded for being nullable or incomparable withdraws the whole suggestion
  instead, because the composite the rule promised is no longer the row's
  actual identity once one of its columns cannot safely be in it.
  `TestKeyHintFallsBackToThePlaceholderWhenAColumnIsNullable` and
  `TestKeyHintFallsBackToThePlaceholderWhenAColumnIsIncomparable`
  (`refusals_test.go`) hold both.
- **The hint does not repeat a column set the pseudo-key rung already probed
  and found duplicated.** For a join table whose FK columns are also its only
  pseudo-key candidate — the ordinary shape, since a table small enough for
  `smallTableKeyColumns` to name columns at all is small enough that its own
  FK columns usually *are* the pseudo-key rung's candidate — `resolveIdentity`
  reaches this hint only after `probePseudoKey` has already run a uniqueness
  probe over exactly those columns and found them not unique. The first
  version of the hint suggested them back anyway: an operator who pastes
  `--key TABLE=a_id,b_id` gets `plan.refused.key_not_unique` on the next run,
  a second run spent confirming an answer this run already had, which is the
  opposite of what T-0318 exists to fix. `resolveIdentity` now passes the
  rung's own probed-and-failed candidate into `keyHint`
  (`probedFailed`), and when `smallTableKeyColumns`' suggestion equals it or
  is contained in it, the hint drops to `--skip-table TABLE` alone rather than
  offering a `--key` that would fail identically. There is no third candidate
  to fall back to instead: for a table of three columns or fewer, the two
  rules can only ever agree or disagree on the same short list.
  `TestKeyHintFallsBackWhenThePseudoKeyProbeAlreadyFailedOnTheSameColumns` is
  the guard. The generic placeholder is unaffected either way — when
  `smallTableKeyColumns` declines for its own reason (too many columns, or a
  nullable/incomparable one) the placeholder is still printed, whether or not
  some unrelated candidate on the same table was probed and failed; suppressing
  it in that case was the first draft's own regression, caught by
  `TestPlanUncomparableColumnStillRefusesWithNoIdentity` (`plan_integration_test.go`,
  §3.4's json-column fixture), which still expects both `--key` and
  `--skip-table` in the message.
- **A `Plan()`-level fixture, through a real Postgres, joined the
  held-without-a-database one.** `refusals_test.go`'s own fixture calls
  `resolveIdentities` and `checkUniqueDomain` directly and never goes through
  `Plan()`/`plan()`, so a regression in `plan()`'s own sequencing — an early
  return once `len(p.refusals) > 0` anywhere before the last collecting check,
  say — would not fail it. `TestPlanCollectsFourIndependentRefusalsInOneRun`
  (`plan_integration_test.go`) is the same reduction — two Rails habtm join
  tables with no derivable identity, and a root/child pair each carrying one
  masked column under a unique index too narrow to widen into — driven through
  one `New().Plan()` call, and asserts the four refusals arrive in the order
  the checks that produce them run.

## A Lookup step's row count travels packed into its own Why, and owes a proper field (T-0346)

The Orchestrator's evaluation of `--tui` against Pagila found `public.country`
— a parent reached only as a lookup (no outgoing edge, an incoming one from
`city`, few rows, no masked column) — printed `0 rows, lookup; lookup` on its
plan line and its plan-screen row, while the estimate two lines below already
added `country`'s 109 rows in. `stepRows` (`internal/core/names.go`) looked at
nothing but `Step.Keys`, which is nil for a `Lookup` step by design (§2 — it is
copied whole, not walked), so it had nowhere to read a row count from; the
count this package computes in `findLookups` (`p.lookupRows`) reached the
estimate's running total and never a `pipeline.Step` a caller outside this
package can read.

**The proper fix — a `Rows int64` field on `pipeline.Step` — is outside this
task's paths.** `pipeline.Step` is declared in `internal/pipeline/plan.go`,
which T-0346's paths did not include (`internal/plan`, `internal/core`,
`internal/tui`, `internal/render`, `ARCHITECTURE.md`, `docs/`,
`testdata/regressions/`), and `Step` already documents its four non-`Table`/
`Mode` fields precisely (`Keys` nil for `Lookup`/`SchemaOnly`, `Cap` "0 when
unused", `Depth`) — none of them is spare for a row count, and repurposing one
would be exactly the kind of silent deviation root CLAUDE.md forbids. Filed as
**T-0379**.

**What ships instead: `assemble` packs the count into `Why`, and
`internal/core` reads it back off.** `lookupWhyWithRows(why, n)` appends
`" (N rows)"` to a `Lookup` step's own `Why` — the same carrier §3.7's root
line already uses for `"200 chosen, 4 pulled in by references"` — and
`ParseLookupRows` is its one inverse, exported for `internal/core`'s `stepRows`
(the number) and `stepWhy` (the same `Why`, with the suffix stripped so a
lookup's plan line does not say `"109 rows, lookup; copied whole (109 rows)"`).
`ordinary lookup`'s `Why` is also now `"copied whole"` rather than the bare
`"lookup"` T-0346 found — `frameworkMetadataWhy`'s own sentences (T-0314)
already said `"copied whole"` for the same shape, and a reader could not tell a
lookup that copied 109 rows from one that copied none from `"lookup"` alone.
`TestPlanPagilaLookupCountryCarriesItsRowCount` (`plan_integration_test.go`)
pins the shape: `country`'s step over the Pagila fixture, `Mode == Lookup`,
`Keys == nil`, and `ParseLookupRows` recovering both `"copied whole"` and the
table's actual row count (checked against a direct `count(*)`, not a hardcoded
literal, so the pin survives the fixture's own drift).

**When T-0379 lands:** `assemble`'s `Lookup` case sets `Step.Rows` directly
instead of calling `lookupWhyWithRows`; `lookupWhyWithRows` and
`ParseLookupRows` are deleted from this file, and
`internal/core/names.go`'s `stepRows`/`stepWhy` read `s.Rows` instead of
parsing `s.Why` (`stepWhy` then collapses to returning `s.Why` unchanged, or
is deleted and its one call site in `run.go` reverts to `s.Why`).

**The packed count itself was wrong above the ceiling (T-0346 review round,
finding 1).** `assemble` packed `p.lookupRows[t.Ref]` straight onto every
`Lookup` step's `Why`, including a framework metadata table's. Above
`lookupRowCeiling`, that value is `findLookups`' `boundedCount` answer, capped
at `countProbeLimit` (1,001) — not the table's real size — while
`frameworkMetadataWhy`'s own sentence right next to it already says "only the
first 1000 ... are copied". A Rails `schema_migrations` past 1,000 migrations
therefore planned with a line reading `1001 rows, lookup; framework metadata
table, has more than 1000 rows -- only the first 1000 ... are copied`: a
number that contradicts the sentence beside it and matches neither the source
count nor what the sentence says is copied. `planRows` now caps the reported
count at `lookupRowCeiling` whenever `boundedCount`'s answer exceeds it, so
the number agrees with the sentence; the exact count
`internal/extract`'s Lookup read actually yields for such a table stays
T-0347's call. `TestPlanFrameworkMetadataTableOverTheLookupCeilingSaysSo` now
also checks `ParseLookupRows`' own count.
