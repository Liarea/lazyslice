# internal/verify

Proves what the run did: FK validation, the residual scan and its capped
source confirmation, the second net, sequence/row-count checks, and the
unmasked-column sample compare. The only stage that tests the *masker* rather
than the classifier (THREAT_MODEL.md T12). Never uses the run snapshot — it is
released by the time this stage starts.

**Contract.** ARCHITECTURE.md §2 "verify, emit" and §6 (the whole algorithm):
`Verifier.Verify(ctx, Source, Writer, *Schema, *Plan, *Classification,
Residual, *LoadResult) (*Report, error)`. `Shapes(plan, cls)` is the source
statement allowlist this stage needs registered before `Verify` runs, as
`internal/extract` and `internal/plan` export theirs.

**Rules.**
- Fails closed, always: a residual hit that cannot be confirmed (source won't
  open, a probe errors, the role lacks `SELECT`, the cap is reached with hits
  untested) is exit 9 naming the reason — never a printed note
  (ARCHITECTURE.md §6 item 3, THREAT_MODEL.md T12).
- Confirmation uses `Source.Short` (a fresh short `REPEATABLE READ READ ONLY`
  transaction), never the run's own snapshot — the indexable probe first, the
  case-folded probe only if that's false, capped at
  `--residual-probe-cap` (THREAT_MODEL.md T4's value-egress note: a probe
  parameter can land in the source's own log).
- The second net re-runs *eight of internal/classify's ten value validators*
  over the whole contents of every unmasked, non-opted-out column of a family
  this package can name, and over the string leaves of every JSON column,
  masked or not (ARCHITECTURE.md §6 item 4). It catches what the 200-row
  sample missed. **It is narrower than §6 item 4's own sentence, in three
  named ways, and this file says so in one place rather than claiming "every
  unmasked column" here and listing holes further down**: no `person_name` and
  no `free_text`, because both read internal/classify's embedded name
  dictionary; no column of a family this package cannot name (`famOther`, so a
  `tsvector` or an enum); and only the strong branch of §4's scoring, so a
  column that the neighbouring-column rule would raise to `possible` is not
  reached. Each is a decision below, with the tracker task that owes the fix.
  Never a category outside the rule pack, which is what THREAT_MODEL.md T1
  already says. A column carrying a `--unmask TABLE.COL=REASON` opt-out
  (ARCHITECTURE.md §8) is deliberately outside the net — that is why the
  opt-out requires a reason and expires on `TypeFP` change, not a scan the
  opt-out would otherwise fail every time.
- A check's *passing* line is never appended beside a failure of the same name,
  in any of the seven checks: `Report.Checks` is what the run prints, and a
  green tick under the refusal that produced the exit code is the one thing a
  report may not say. §6 item 5's *reported* lines (a step with no count to
  check, a sequence owned by no column, an unfetchable sample, unconfirmed
  hits) carry `Passed: true` too and legitimately sit beside either, which is
  why the rule is on the check's own `passed` code and not on `Passed` alone.
  `noCheckBothWays` in the integration suite is the guard.
- A source-changed mismatch (row absent, or a byte differs since the snapshot)
  is reported and counted, not failed — only a *confirmed* residual hit or a
  `possible`-or-above second-net hit is a failing exit.
- `schema` supplies column types for canonicalisation and identity columns for
  the sample fetch — verify must never reclassify from scratch.
- No value ever reaches a `Refusal`, a `Check` or an `event.Code`'s arguments.
  A refusal names the table, the column and a fixed phrase from `reasons.go`
  (THREAT_MODEL.md T4); the candidate a probe was built from is bound as a
  parameter and nothing else.

## Files

- `verify.go` — `Options`, `New`, `Verify`, the `state` one call carries, the
  order the checks run in and the exit code the report ends with.
- `codes.go` — the nineteen `event.Code`s this stage renders and the `Refusal`
  that carries a failing one. Each has a row in `internal/event/catalogue.yml`.
- `reasons.go` — the fixed phrases a `Refusal.Reason` may hold.
- `residual.go` — §6 items 1 to 3: the scan, the hit, the two probes, the cap.
- `secondnet.go`, `validators.go` — §6 item 4.
- `fk.go`, `counts.go`, `sample.go` — §6 item 5.
- `sql.go`, `shapes.go` — every statement, and the shapes the source ones match.
- `columns.go`, `value.go` — the type families, the JSON leaf walk, and the
  rendering that reproduces the residual filter's key material.

## Decisions made during implementation

ARCHITECTURE.md §6 is silent on each of these; the simplest correct behaviour
was chosen and is recorded here rather than only in a comment.

- **Verify asks the `Writer` for a `Query` method and refuses without one.**
  §2's `pipeline.Writer` has `Exec`, `CopyFrom` and `Begin`, and every check in
  §6 is a *read of the target*: the residual scan streams masked columns, the
  second net scans unmasked ones, the FK check counts orphans, the row counts
  count and the sample compare reads rows. There is no read path on the type,
  and `internal/pg`'s writer has no `Query` today, so a real run cannot verify
  yet. The alternatives were to reclassify the deviation as "verify opens its
  own connection to the target" (this package would then be the second one that
  connects to a database, which internal/CLAUDE.md forbids) or to take a
  reader through `New` (which needs the same method on `internal/pg` anyway,
  plus wiring in `core`). Asking the `Writer` means the day `internal/pg`'s
  writer grows `Query`, this stage works with no change in `core`.
  **Owed: `internal/pg`'s writer needs `Query(ctx, sql, args...) (Rows, error)`,
  and §2's `Writer` needs the method beside `Exec` — recorded there under
  "Type additions recorded after implementation", and owed a tracker task in E4
  that blocks T-0044 (T-CORE), because core cannot call `Verify` until that
  method exists.** Neither of those is in this package's paths; both are
  reported in T-0043's return value rather than left in this file alone, where
  the developer picking up T-0044 would not read them. Until then the only
  `Writer` that satisfies it is the one in `verify_integration_test.go`, so
  **verify cannot run against a real target today**. Verify refuses rather than
  reporting a green tick over checks that did not run.
- **A column below `minValues` non-NULL values is unproven, not clean.** §4's
  threshold is a *ratio*, and a ratio over two values means nothing, so the
  first version of this net returned before the validators ran. That is a
  fail-open, and the classifier had a floor at the same number for the same
  reason, so nothing was above it: `public.devices.owned_by` in
  `testdata/nasty.sql` — two email addresses and a NULL in a three-row table —
  reached the target in cleartext under exit 0, which
  `TestI2NothingFlaggedSurvives/nasty` caught and tracker T-0058 fixed. The
  validators now run over whatever the column holds and *any* hit below
  `minValues` is exit 9: the threshold is what a ratio buys, and there is no
  ratio to buy it with. One value that parses as an email address is still a
  production email address in the target. `TestAColumnBelowMinValuesFailsOnAnyHit`
  pins both halves — one hit in two values fails, and the same 0.5 ratio over
  four values does not.
  - **That branch is slice-size dependent, and it is the one refusal here that
    is.** `nonNull` is counted over the *target*, so whether a column is below
    `minValues` is a function of `--take` and `--cap` rather than a property of
    the source column: a table the subsetter reduces to one or two rows is
    unproven at `--take 10` and proven at `--take 500`. The two loosest
    validators are what makes that visible, because they carry no parse —
    `addressShape` wants a digit and two lettered words (`Room 12 Building A`,
    `iPhone 15 Pro`) and `looksSecret` wants 16 characters, two classes and
    Shannon ≥ 3.2, which an ordinary slug reaches. So the same database can
    verify clean at one `--take` and exit 9 at a smaller one, after the target
    is already loaded, with `--unmask` the only way past it. That is
    non-monotone and it is a real cost; it is kept because the alternative is
    the T1 leak above, and because the failure direction is a refusal rather
    than a cleartext value in the target (CLAUDE.md: "when in doubt, mask it").
    Two narrowings were considered and not taken: gating on the *source*
    column's cardinality, which this stage does not have (it reads the loaded
    target, §6 item 4), and restricting "any hit" below `minValues` to the four
    validators that carry a real parse — email, phone, network_id,
    financial_account — leaving address and credential on the ratio. The second
    is the one to revisit if the refusal proves noisy in practice: it would cost
    exactly the two-row address column, which is the case this branch exists
    for. Whoever revisits it owes a tracker task and this note updated, not a
    quiet threshold change.
- **The exit code is the first failure in §6's order and the failing check is
  also returned as an error.** Every check runs whatever the ones before it
  found, so one report names everything wrong with the target; the report's
  `ExitCode` and the returned `*Refusal` always agree. §6's order puts the
  residual scan and the second net first, so a leak is what the exit names even
  when an edge is broken too.
- **A row count and a sequence take exit 7.** ADR-005's table names no code for
  either. Both are failures of the movement of rows between extract and load,
  which is what "7 extract or load" covers, and neither is a residual (9) or a
  foreign key (8). `internal/transform` records the same reading for a masker
  refusal.
- **The FK check counts orphan rows; it does not read
  `pg_constraint.convalidated`.** The loader adds every edge `NOT VALID` and
  then validates it, so reading the catalogue would be trusting the statement
  this check exists to confirm — and a constraint dropped by hand afterwards
  would read as a target with nothing wrong with it. The edges checked are the
  ones `internal/load/ddl` recreates: not virtual, not `NotRecreatable`, both
  ends recreated and neither a partition leaf.
- **A `Lookup` step's expected count is the loader's, not the plan's.** §6 item
  5 says "every `Lookup` step has its bounded count", and `pipeline.Step` has
  `Keys == nil` for a lookup, so the plan carries no number for this check. The
  loader's committed count is what is left; a lookup that copied nothing is one
  of item 5's reported cases anyway.
- **One short transaction, not one per hit.** §6 says "on a hit it opens
  `Source.Short`". Opening one per hit is a connection and a snapshot on
  production per hit; the transaction is opened on the first hit that needs one
  and reused for every probe and every sample fetch after it, so every
  confirmation in a run is answered against one consistent source. A source
  that would not open is remembered rather than retried, which would be a probe
  storm against a database that is already refusing.
- **A residual hit on a JSON leaf is untestable, and therefore exit 9.** Both
  probes in §6 item 3 ask whether the *column* holds the value, and a leaf's
  value lives inside a document, so neither can answer. Item 3 makes a hit that
  cannot be tested exit 9, so that is what a leaf hit is — and no probe is
  spent putting the candidate in the source's log to reach the same answer. A
  containment probe (`position(lower($1) in lower(col::text)) > 0`) would
  confirm one, and it is a sequential scan on production and a third probe form
  §6 does not have; if the false-positive rate ever makes this cry wolf, that is
  the change to review, not a downgrade to "unconfirmed".
- **A *number* leaf is in the filter and is deliberately not tested.**
  `internal/transform` records one, and then redraws it over 10^6 values
  (10^5 for a fractional one), so on a run that masked correctly the target's
  number at that path matches the source's canonical bytes about `N/10^6` of the
  time per path — and, by the decision above, every leaf hit is an
  unconditional exit 9 whose reason names no action and has no escape flag. At
  the default `--take 500` that is a run refused for a leak that did not happen
  often enough to ship. It is `internal/transform`'s own argument for keeping a
  boolean leaf out of the filter ("a two-valued domain"), carried to the domain
  one size up: a value redrawn over a small domain carries no residual signal,
  so a match on it is evidence about the domain and not about the masker. A
  string leaf keeps its entry, because the `free_text` masker's domain is not
  small. `TestANumberLeafIsNotAResidualHit` is the guard. If a containment probe
  ever lands (the decision above), the number leaf can come back with it.
- **A masked document is tested both ways.** `internal/transform` either walks a
  document (a filter entry per leaf, keyed by path) or collapses it (one entry
  for the whole document, under `semi_structured` at the empty path), and the
  classification does not say which happened. Both are tested; the wrong one
  simply never hits.
- **`mask.Canonical` is called with empty `Constraints`.** It reads exactly one
  field of them, `Region`, and nothing in the tree sets it — `internal/classify`
  decided against a per-table region hint and `internal/transform` builds its
  `Constraints` without one — so the bytes the filter was keyed with and the
  bytes reproduced here are computed under the same empty hint. **If a region
  hint ever lands, `transform`'s canonical form changes and `value.go` must
  change with it in the same commit**, or every masked phone column becomes
  unscannable. The guard is `TestCanonicalReadsNoConstraintButRegion`, which
  walks `mask.Constraints` by reflection and sets every field but `Region` in
  turn, asserting the canonical form does not move: the day `Canonical` reads a
  second field, that fails, including for a field nobody has added yet.
  `TestCanonicalReproducesWhatMaskApplyRecorded` cannot do that job — it hands
  both sides the same empty `Constraints`, so it pins the *rendering* and not
  this assumption.
- **`shapeOf` resolves the domain before it strips `[]`, because
  `internal/transform`'s does.** A column over `CREATE DOMAIN emails AS text[]`
  is masked element-wise by transform and recorded one filter entry per element;
  a `shapeOf` here that read only `TypeName` would call it a scalar,
  canonicalise the whole `[]any` through `textOf` and test bytes transform never
  added — no hit, a green tick, and every element shipped in cleartext. The two
  functions are a cross-package contract stated in both CLAUDE.md files and
  nothing else holds them together, so `TestArrayDomainIsScannedElementWise`
  pins the pair.
- **The second net reads the character families and four more, plus the integer
  families for the digit validators.** §6 says "canonicalisation and validator
  choice are driven by the column types in `Schema`". `text`/`varchar`/
  `bpchar`/`citext` reach the text validators, and so do `uuid`, `inet`, `cidr`
  and `macaddr`, whose values are exactly what a validator recognises.
  `integer`, `bigint` and `numeric` reach the Luhn and IBAN validators, because
  a payment card in a `bigint` column is exactly what §4's surrogate-key
  exemption is most likely to have let through. A `json`/`jsonb`/`hstore`
  column has its string leaves read whether or not it was masked: an unmasked
  document had no masker at all, which is a stronger reason to read it and not
  a reason to skip it. **`famOther` is the hole**: a `tsvector` renders as
  `'ace':1 'administr':9` — a number and two words, which is an address to a
  validator — and a type this package cannot name is a type it cannot
  canonicalise either, so a column of that family is left to the classifier's
  own signal. That is a recall hole and it is stated here and in the Rules
  above, not only here.
- **The net has no `person_name` and no `free_text`, which is the largest hole
  in it.** `internal/classify` registers ten value validators; the two missing
  here are `looksLikeName` and `prose`, and both read that package's embedded
  name dictionary. They are value validators over one string exactly like the
  other eight — the omission is *not* "a category recognised only by its column
  name", and an earlier version of `validators.go` said so wrongly. They are
  missing because a dictionary is a rule pack and this package will not carry a
  second copy of one (`internal/transform` refused a second rule pack for the
  same reason). The cost is concrete: a target column holding real person names,
  or a notes column carrying other rows' names (testdata/README.md trap 17),
  passes this net, and THREAT_MODEL.md T1 names the net as one of exactly two
  v1-blocking controls on classifier recall. **Owed: the shared home below has
  to hold the dictionary too, and T1's wording has to name these two
  categories**; reported in T-0043's return value.
- **The net implements only the strong branch of §4's scoring.** A column at
  ≥0.8 of non-NULL values validating fails. `internal/classify` has a second
  route to the mask threshold — a weak signal at ≥0.5 records the column at
  `low`, and the neighbouring-column rule raises `low` to `possible` in a table
  that holds a `likely` column — and §6 item 4 says a column reaching `possible`
  is exit 9. This package holds everything the rule needs (the whole
  `Classification`, and its own per-column ratios), so this is a scope decision
  and not a missing input: the raise is a second piece of *scoring*, which is
  the half of the rule pack this package refused to copy, and landing it beside
  a validator set that is already two validators short would be tuning a net
  with a known hole in it. **Owed: the weak threshold and the neighbouring-column
  raise, in the same task as the shared home**; reported in T-0043's return
  value.
- **The validators are a copy of `internal/classify`'s, and the JSON leaf walk,
  the value rendering, the type families and the chunk-join SQL are copies of
  `internal/transform`'s and `internal/extract`'s.** Stage packages may not
  import each other (internal/CLAUDE.md) and every one of those is unexported
  where it lives. What was copied is the value-only half in each case — a
  validator over one string, a canonical rendering, a family table, an
  identifier quoter — and never a rule pack: `internal/transform` refused a
  second rule pack for the same reason and this package refuses one too. **The
  fix is a shared home for each, not a third copy**; `internal/transform`'s
  CLAUDE.md already owes the type families one. A change to
  `internal/classify/validators.go` or to `internal/transform`'s residual-filter
  contract has to be made here in the same commit, and the unit tests in this
  package are what fail when it is not. About 480 of this package's lines are
  those copies and the debt is now visible only in three package CLAUDE.md
  files, which is not where a developer editing `internal/classify` reads;
  **owed: one tracker task for a leaf package under `internal/` holding the
  value-only halves — the validators *and the name dictionary*, the `textOf` /
  `mask.Canonical` reproduction, the type-family table and the identifier
  quoter**, reported in T-0043's return value.
- **The first chunk of a key set is asked for through an optional accessor.**
  `pipeline.KeySet` has `Len`, `Bytes` and `Chunks`, and `Chunks` materialises
  the *whole* set as typed arrays; the sample compare needs 100 keys, so calling
  `Chunks(100)` on a step at the `--row-budget` ceiling allocates a second copy
  of that step's key set, per table, at verify time — after `--memory-budget`
  (§8, exit 11) has been checked at plan and can no longer refuse anything.
  `sample.go` therefore type-asserts the key set for `FirstChunk(n) Chunk` and
  falls back to `Chunks` when it has none, which is the same shape of deviation
  as `Writer.Query` above: a bounded sample is worth less than the check it
  would otherwise skip on exactly the largest tables. **Owed: `FirstChunk` on
  §2's `KeySet` and on `internal/plan`'s two implementations** — tracker T-0050
  already carries "KeySet chunk iterator" for `internal/extract`'s sake, and
  this is the same accessor.
- **The sample comparison skips a step with no keys and a step with a masked
  identity column.** §6 item 5 says "a sample of 100 rows per table fetched by
  identity"; a `Lookup` step carries no key set to fetch by, and a step whose
  identity is masked has no join that reaches the same row in both databases
  (§4 propagates a masked parent key onto every column referencing it). Both are
  reported, which is what item 5 does with everything it cannot check. A sample
  the source will not answer is reported too — item 5's own failures are all
  reported, and it is the *residual* hit that fails closed.
- **A schema-only step stays in the loop.** Its table exists in the target and
  is empty, and every check reads that as the empty answer, including the
  strict-NULL half of the sequence check.
- **Probes are named per column in the allowlist.** `Shapes` emits the two
  probe templates for each masked column of a loaded table, with the table and
  the column written into the template, and a sample shape per keyed step. A
  table-agnostic `SELECT EXISTS` would admit an equality test of any value
  against any relation, which is the widening `internal/extract` names in its
  own lookup shape.

**Test.** `go test ./internal/verify/...` for what is provable without a
database: every statement this package sends to the source matches a shape it
exports (through a real `pg.Tracer`), the canonical bytes it reproduces are
`mask.Apply`'s own and are computed under a `Constraints` `mask.Canonical` does
not read, the leaf spelling is `internal/transform`'s, a domain over an array is
still an array, and a number leaf is not a hit.

Then `go test -tags integration ./internal/verify/...` for the green case and
one case per failure §6 can produce, because a check nobody has ever seen fail
is a check nobody has tested: `TestVerifyPassesACorrectTarget`;
`TestVerifyFailsOnASourceValueInAMaskedColumn` (tracker T-0035's negative
control: a known source email planted in a column the run masked, exit 9 naming
the table and the column); `TestVerifyFailsWhenAHitCannotBeConfirmed` (the same
plant under `Options{ProbeCap: -1}`, exit 9 as *unconfirmable*, which is the
branch that decides whether this stage fails closed);
`TestVerifyFailsOnUnmaskedPersonalDataInAnUnmaskedColumn` (addresses planted in
a column the classifier did not mask, exit 9 from the second net);
`TestVerifyFailsOnABrokenForeignKey` (exit 8, with the constraint dropped first,
so the check is the orphan count and not the catalogue);
`TestVerifyFailsOnARowCountThePlanDoesNotPredict` (one row added, exit 7); and
`TestVerifyFailsOnASequenceThatWasNotReset` (`setval(..., 1, false)`, exit 7).
Every failing case also runs `noCheckBothWays`.

**The integration suite no longer carries a workaround.** Pagila used to force
every whole-pipeline run through a per-column `--unmask TABLE.COL=REASON` prior
on five columns, because the classifier decided `credential` on the
`last_update` timestamps and `address` on `film.fulltext`, and
`internal/transform` could not write those maskers' output back into a
`timestamp` or a `tsvector`. T-0054 fixed the classifier's decision on both
shapes, so `TestVerifyPassesACorrectTarget` hands `Classify` a nil prior and
asserts the fix directly: `public.film.fulltext` is decided `derived_text` and
lands in the target as the empty tsvector, and `actor.last_update`,
`address.last_update`, `category.last_update` and `film.last_update` are
decided unmasked and copied.

**Never:** use the run's own (released) snapshot; treat an unconfirmable
residual hit as anything but exit 9; skip the cap on confirmation probes; treat
a source-changed mismatch as a pass; let a value reach a refusal, a check or an
event.
