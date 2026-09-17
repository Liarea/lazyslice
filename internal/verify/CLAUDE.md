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
- The second net re-runs *all twelve of internal/classify's value validators*
  over the whole contents of every unmasked, non-opted-out column of a family
  this package can name, and over the string leaves of every JSON column,
  masked or not (ARCHITECTURE.md §6 item 4). Twelve, not eleven: the twelfth
  is `national_id`, which this package gained at **T-0187** (the 2026-09-15
  round-2 red team, below) in the same shape the eleventh — `textsig.ValidURL`
  — already teaches: `internal/classify` gained the validator first,
  `internal/verify` was a separate task's paths, and between the two a
  national identifier was a shape *neither* net could see. Before that, the
  eleventh was `textsig.ValidURL`, which `internal/classify` gained at T-0100
  when `textsig.LooksSecret` stopped reading a URL as a credential, and which
  this package only gained at **T-0122** — between those two tasks a URL was a
  shape *neither* net could see, and a profile URI that reached the target
  unmasked passed every validator here. None of the twelve is missing now;
  two are answered by narrower validators on purpose (the dictionary rule,
  below).
  `internal/textsig/CLAUDE.md` still describes that hole as open, because
  `internal/textsig` was outside T-0122's paths; correcting it is tracker
  **T-0126**, and the rule it teaches stands either way: a validator narrowed in
  that package is narrowed for both nets, and both need an answer in the same
  change. It
  catches what the 200-row sample missed. **It is still narrower than §6 item 4's own sentence, in three
  named ways, and this file says so in one place rather than claiming "every
  unmasked column" here and listing holes further down**: no column of a family
  this package cannot name (`famOther`, so a `tsvector` or an enum — **`bytea`
  was on that list too until the 2026-09-15 red team and is not any more**: it is
  read as text whenever `textsig.PrintableText` accepts the value, which is the
  A4a fix and is, **since the T-REDFIX review's second finding**, genuinely the
  same guard `internal/classify` applies to the same bytes on its side — that
  package used to require 95% of the *sample set* to be readable on top of it,
  which made a half-printable `bytea` column unmasked there and exit 9 here with
  no green path); only the
  strong branch of §4's scoring, so a column that the neighbouring-column rule
  would raise to `possible` is not reached; and `person_name` and `free_text`
  read narrower validators than the classifier's and are scored under **the
  dictionary rule** rather than under its thresholds, so a one-word name column,
  a name written surname-first and a document's leaves all pass this net
  (T-0055 and its review). The first two are decisions below with the tracker
  task that owes the fix; the third is a decision below with the reason it is
  deliberate, and the leaves half of it owes **T-0087**. Never a category
  outside the rule pack, which is what THREAT_MODEL.md T1 already says. A
  column carrying a `--unmask TABLE.COL=REASON` opt-out (ARCHITECTURE.md §8) is
  deliberately outside the net — that is why the opt-out requires a reason and
  expires on `TypeFP` change, not a scan the opt-out would otherwise fail every
  time.
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
- `secondnet.go`, `validators.go` — §6 item 4: the scan and the scoring, and
  the twelve validators attached to their categories, in thirteen entries here
  (T-0136 split Luhn and IBAN back apart, then split Luhn again by family —
  strong on the character side, ratio on the digits side, its review round's
  finding 2; T-0187 split national_id the same way, below; only IP and MAC
  still share one entry). The validators and the
  name dictionary themselves are `internal/textsig`, which `internal/classify`
  imports too (T-0055).
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
    - **That narrowing was taken, but only above `minValues`, and by a
      different name** (`strong`, T-0136). The 2026-09-09 review's finding 7
      found the branch this note did not anticipate: a *proven* column, where
      the ratio is supposed to be the answer, still let a single strong hit
      through as long as it stayed under `validatorThreshold` — one email
      address among nineteen ordinary strings is 5%, and the net said nothing.
      So `validator.strong` (`validators.go`) now marks the five entries
      that carry a real parse — email, phone, network_id, the Luhn half of
      financial_account, online_id — and `netColumn` fails a proven column on
      any hit from one of them, exactly as it already did below `minValues`;
      `address` and `credential` keep the ratio at every size, which is this
      paragraph's own answer restated rather than reopened. **IBAN is the
      one of the six parse-shaped categories that is not strong**, split back
      into its own entry for exactly that (T-0136's own review round): it is
      a mod-97 checksum over fifteen to thirty-four letters-and-digits rather
      than over a run of digits, so an ordinary all-caps string passes it
      about as often as any string of the right shape would — five of
      pagila's own film titles do, and `TestVerifyPassesACorrectTarget` is
      what found it, failing on `public.film.title: financial_account` the
      first time this branch shipped with IBAN still joined to Luhn. The two
      "any hit" branches are independent and still say different things below
      `minValues`: every non-dict validator fails on any hit there, strong or
      not, because a ratio over one or two values means nothing at all —
      narrowing *that* branch to the parse validators (five now, not four) is
      still the open question this paragraph names.
    - **Luhn is `strong` on the character side only; the digits side keeps
      the ratio** (T-0136 review round, finding 2). The entry above marked
      the whole Luhn validator strong regardless of family, so *any* hit —
      whatever the ratio — failed an `integer`/`bigint`/`numeric` column too.
      Roughly one in ten 12-to-19-digit identifiers passes the Luhn check
      digit by chance (snowflake IDs, epoch-millisecond timestamps, EAN-13
      barcodes, order numbers), so an ordinary unmasked bigint id column of
      any realistic size holds at least one, and this net failed it at exit
      9 with no ratio escape and no `--unmask` on a column that carried no
      personal data — "a refusal an operator cannot act on and would learn
      to route around with `--unmask`", the same sentence the dictionary
      rule above argues against, and the state such a column reaches once
      `internal/classify`'s matching gate (finding 1, same review; see
      `internal/classify/CLAUDE.md`'s `strongHit` note) stops routing it to
      an unwritable `free_text` and leaves it unmasked instead. `validators`
      now carries the Luhn entry twice: `{text: true, strong: true}` for a
      digit run inside a character column, where the claim is still a
      precise parse over a typed-in card number and any hit still fails;
      `{digits: true}` (no `strong`) for the numeric families, where the
      ratio rule applies exactly as it does to `address` and `credential` —
      `proven && ratio < validatorThreshold` is what decides it, and a
      column below `minValues` still fails on any hit (T-0058's floor,
      unaffected). This is the family split IBAN already has (`text` only,
      never `digits`), applied to the other half of `financial_account`;
      what makes it a *different* split from IBAN's is that IBAN never
      reaches `digits` at all, where Luhn now reaches `digits` at the
      ordinary ratio rather than not reaching it.
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
- **The JSON walk now scans object keys, not only values** (`keyHits`,
  T-0137, docs/reviews/2026-09-09/REVIEW.md finding 8). `internal/transform`'s
  `maskKey` masks a key that parses as an email, a phone number or a
  credit-card number through that category's own masker. `keyHits` walks
  every key of the *target's* document — `documentKeys` in `columns.go`, at
  every level and inside arrays of objects too — and tests the ones that
  still match one of the same three strong validators (`strongKeyCategory`,
  `internal/transform`'s own `keyCategory` restated, the way `catalog.go`'s
  `strongCatalogHit` already restates `internal/plan`'s) against the filter.
  Every hit is a document-leaf hit: it lives inside a key and not in a
  column's own value, so neither probe of item 3 can ask a column whether a
  key appeared inside it, and a hit is exit 9 the same way a leaf value's is.
  - **It now tests at the key's own path, not the empty path**
    (**2026-09-14 review round, finding 1**). The first version of both
    sides recorded and queried a masked key at the document's empty path,
    because `internal/transform`'s `walk` used to build a masked key's
    *children's* paths from the source key — a spelling the target could
    never hold again — which left the empty path as the only place both
    sides could agree. `internal/transform` now builds every path beneath a
    masked key from the masked spelling itself (its own CLAUDE.md, "A masked
    object key is keyed at its own path"), so `documentKeys` here now returns
    each key's path alongside its name (`keyOccurrence`, `columns.go`) and
    `keyHits` tests the filter at that path rather than at `""`. The two
    sides were previously silently correct only because both were wrong the
    same way — neither actually needed the *value* recorded at the right
    path, since the value was moved wholesale to `""` on both sides — but
    every string leaf nested beneath a masked key was a blind spot the empty
    path could not see at all: `keyHits` never reached those paths and
    `leaves`/`documentHits` queried them under the source key's spelling,
    which the target no longer held. That half of the fix is
    `internal/transform`'s, and this side only had to follow the path it
    now agrees on.
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
- **An array that arrives as a text literal is split with
  `internal/transform`'s grammar, and a masked one that will not split is exit 9
  (T-0129).** The source pool registers no user types (T-0076) and pgx hands
  back a slice only for an array type its own map knows, so a `citext[]` — and
  every other array of an extension's base type — arrives as the single string
  `{a@b.test,c@d.test}`. `internal/transform` masks such a column *element-wise*
  anyway (T-0118, `internal/transform/array.go`): it parses the literal, masks
  each element, and records one filter entry per element under the column's
  empty path. `arrayHits` used to fall back to `scalarHits` on the whole value
  whenever it was not a `[]any`, which canonicalised the whole literal, matched
  none of those entries, and reported a green residual tick over exactly the
  column class T-0118 enables — the masker failing open, with §6 item 1 the only
  control T12 has against it. So `residual.go` now follows transform's own
  carriers in transform's own order (`cell`): a `[]any` element-wise, a `string`
  or `[]byte` split by `arrayLiteralElements` and tested element-wise, and any
  other carrier through the scalar path, because that is what transform does
  with it. **A masked array column whose literal will not parse is
  `verify.refused.residual_unconfirmable` at exit 9 naming the column**
  (`reasonArrayLiteral`), never a fall back to the scalar path: it is untestable
  for the same reason a JSON leaf is — the entries are per element and there is
  no element to test — and the scalar fall back is the same green tick by a
  shorter route. `arrayliteral.go` is a **copy** of transform's reader, for the
  reason `textOf` is one (a stage package may not import another), so the two
  grammars are a cross-package contract: what transform accepts this accepts and
  splits the same way, what transform refuses this refuses.
  `TestArrayLiteralElementsAreSpeltAsTransformSpellsThem` and
  `TestArrayLiteralRefusesWhatIsNotOne` carry
  `internal/transform/array_test.go`'s own case lists; the fix for the third
  copy is a shared home, as for `textOf` and the identifier quoting.
  **The second net splits the same literal and does *not* refuse one it cannot
  parse**, which is the deliberate asymmetry: its subject is a column nothing
  masked, so there is no masker to have failed open, and reading the whole value
  is a recall hole of the kind `famOther` already is, while exit 9 there would
  be a refusal on a column the classifier had no reason to mask.
  **The debt this leaves is `T-0146`**: both probes in `sql.go` ask whether the
  *column* holds the candidate, and a hit here carries an *element*, so binding
  it against an array column errors and every element hit — on this path and on
  the `[]any` path, which has always behaved this way — is exit 9
  *unconfirmable* rather than *residual*, after spending a probe that had to
  fail. Fail-closed, so not a leak; a Bloom false positive on a masked array
  column is a correct run refused with a reason naming no action.
- **The second net reads the character families and four more, plus the integer
  families for the digit validators.** §6 says "canonicalisation and validator
  choice are driven by the column types in `Schema`". `text`/`varchar`/
  `bpchar`/`citext` reach the text validators, and so do `uuid`, `inet`, `cidr`
  and `macaddr`, whose values are exactly what a validator recognises.
  `integer`, `bigint` and `numeric` reach the Luhn and IBAN validators, because
  a payment card in a `bigint` column is exactly what §4's surrogate-key
  exemption is most likely to have let through. A `json`/`jsonb`/`hstore`
  column has its string leaves *and its object keys* read whether or not it
  was masked: an unmasked document had no masker at all, which is a stronger
  reason to read it and not a reason to skip it.
  - **The keys were missed until the 2026-09-14 review round, finding 3.**
    `keyHits` (`residual.go`) already tested a masked column's own keys
    against the filter, but `netValues` (`secondnet.go`) called `leaves(v)`
    alone, which never yields a key — so an email, a phone number or a card
    used as a JSON key inside a column classified `none` or carrying
    `--unmask` was masked by nothing (there is no masker on such a column)
    and read by neither net, while the identical value as a *value* in the
    same document was already caught by this one. `netValues` now folds
    `documentKeys(v)` in alongside `leaves(v)` whenever `mode.leaves` is set,
    so a key reaches every validator `mode.text` already runs over values
    with. The dictionary-backed validators still never see a key — `applies`
    already excludes them from `mode.leaves` — because a key is never a
    sentence and the argument that excludes a document's *value* leaves from
    them applies at least as strongly to its keys.
  - **A masked document column's own key is excluded from the net when it is
    the masker's own output, and only then** (**T-0172**). Finding 3 above
    made `netValues` test *every* document key against every validator
    `mode.text` runs, with no regard for whether this column's keys were
    themselves masked. On a *masked* document column, `json.go`'s `maskKey`
    already ran every key through `keyCategory` — the same three-validator
    question (email, phone, the Luhn half of `financial_account`) — and
    replaced every match with that category's own masker; a category masker's
    output is, by construction, still a value of that category (an email
    masker's output is another address, THREAT_MODEL.md T1's DDL-canary
    paragraph says the same thing about a `DEFAULT`). So a masked key that
    still matches the validator that made transform mask it is not a hit,
    it is the masker working — and the net was refusing the run for it:
    `testdata/regressions/013` is one row, one `jsonb` column, one key
    (`canary.person@example.org`), masked to another `example.com` address
    and refused at exit 9 as `email` on a run that masked correctly.
    `netMode` now carries `docMasked` (`has && d.Masked`, set beside
    `leaves` in the same `document(family)` case), and `netValues` drops a
    key from its output exactly when `docMasked` is true and
    `strongKeyCategory` — `residual.go`'s own restatement of `keyCategory`,
    already used to scope `keyHits` — matches it.
    - **Why excluding exactly that set costs no recall.** For a masked
      document column, a key in the *target* at a given position is one of
      two things: its source text matched `keyCategory`, in which case
      `maskKey` replaced it and the target holds masker output that is
      guaranteed to match the same category again; or its source text did
      not match `keyCategory`, in which case the target holds the source
      text unchanged, and since `strongKeyCategory` asks the identical
      question `keyCategory` does, that unchanged text cannot match
      `strongKeyCategory` either (if it did, it would have been masked).
      So a target key matching `strongKeyCategory` on a masked document
      column is provably masker output and never a surviving source value —
      there is no case this drops that the net was ever able to use.
      `residual.go`'s `keyHits` is the check this key actually needs: it
      tests canonical equality against the *source*, not "does this look
      like the category", and it already proves no source key survived at
      that path without this change.
    - **What stays in the net's input, and why the fix is scoped to keys and
      not to columns.** A key that does not match `strongKeyCategory` is
      kept regardless of `docMasked`, because `keyCategory` never masks it
      either way: only three of the net's five *strong* categories are also
      ones `keyCategory` recognises (email, phone, the Luhn half of
      `financial_account`); `network_id` and `online_id` are strong here but
      `keyCategory` has no IP, MAC or URL branch, and `credential` and
      `address` are shape guesses `keyCategory` never runs at all. A
      network_id-, online_id-, credential- or address-shaped key inside a
      masked document column is exactly as unmasked as one inside an
      unmasked column, and the net still has to catch it — which is why the
      exclusion is per key and per the three categories `keyCategory`
      actually rewrites, not "the net skips a masked document column's keys"
      wholesale. The wider form was considered — it is the shape the task
      that opened T-0172 offered first — and rejected: it would silently
      reopen finding 3's own gap for the other two strong categories and
      both shape guesses, on exactly the column class THREAT_MODEL.md T1
      calls a blocking control.
    - **Leaves have the same question and not the same answer.** A masked
      document's string leaves are `leafCategory`'s (`json.go`) — every
      string leaf is masked as `free_text` regardless of its key, a decision
      `internal/transform/CLAUDE.md` records — and `free_text`'s filler
      (`mask/gen_text.go`'s `filler`, drawing from `mask/words.go`'s
      `fillerWords`) is neutral, space-separated words with no digit, no
      `@` and no scheme, so it cannot itself parse as an email, a phone
      number, a card number, an IP, a URL, or clear `LooksSecret`'s entropy
      guard. A masked document's leaves therefore do not have the keys'
      exposure, and `netValues`'s leaf half (`leaves(v)`) is unchanged by
      this task. `TestSecondNetReadsDocumentKeysAsWellAsValues`'s three new
      `masked: true` cases carry a filler-shaped leaf beside each masked key
      for exactly this reason: the absence is pinned by a test, not left
      argued only here.
    - **Teaching the validators to recognise the masker's own output space
      instead was considered and rejected**, per the task that opened
      T-0172: a strong validator could exempt an RFC 2606 domain, the
      `phone_unique` shape or the `credential_unique` prefix everywhere,
      rather than only on a column this run actually masked. It reads
      simpler, and it is wrong — a source column that genuinely holds
      `example.com` addresses would then pass the net *unmasked*, which
      THREAT_MODEL.md T1 does not accept (`mask`'s own CLAUDE.md and
      ARCHITECTURE.md section 5 already call the documentation-domain
      collision "a known false-positive surface, not a leak" for the
      *residual* scan, precisely because that check is scoped to columns
      this run masked; a second net that stopped scoping the same way would
      widen the hole T1 exists to close). Scoping the exclusion to
      `Decision.Masked` — the same gate every other row in this file reads
      before trusting a masker ran — is what keeps a genuinely unmasked
      `example.com` address a real hit.
  **`famOther` is the hole**: a `tsvector` renders as
  `'ace':1 'administr':9` — a number and two words, which is an address to a
  validator — and a type this package cannot name is a type it cannot
  canonicalise either, so a column of that family is left to the classifier's
  own signal. That is a recall hole and it is stated here and in the Rules
  above, not only here.
- **`person_name` and `free_text` are in the net, under the dictionary rule**
  (T-0055). They used to be missing — the two of `internal/classify`'s
  validators that read its embedded name dictionary, which this package could
  not import and would not copy — so a target column of real person names, or a
  notes column carrying other rows' names (testdata/README.md trap 17), passed
  the net that THREAT_MODEL.md T1 names as one of exactly two v1-blocking
  controls on classifier recall. The dictionary now lives in
  `internal/textsig` with the validators and both packages import it, so both
  are registered here.
  - **The rule, by name.** A dictionary-backed validator fails a column only
    when **the hit ratio is at or over `validatorThreshold` across at least
    `minValues` distinct hitting values**, and only on a *shape* rather than a
    word — and the shape is the same one for both: a given name immediately
    followed by a surname. `person_name` counts `textsig.Dict.NameShape` — two
    or three dictionary words carrying that pair — never the classifier's
    `LooksLikeName`, which accepts one word, and never the plain multi-token
    shape either. `free_text` counts `textsig.Dict.ProseName` — six words or
    more carrying that pair somewhere inside — never the classifier's
    `Dict.Prose`, which fires on one dictionary word. Neither runs over a
    document's leaves at all (`applies`, in `secondnet.go`).
  - **Why it is not the classifier's threshold, and not its validators
    either.** Black, Brown, Hill, Green and Wood are all surnames, so
    `product.colour` validates as `person_name` on 100% of its rows under
    `LooksLikeName`. The classifier's answer to that is to mask the column,
    which costs a lookup table; this net's answer would be exit 9 on a database
    that is already loaded, with no green path short of `--unmask` on a column
    holding no personal data — a refusal an operator cannot act on and would
    learn to route around. It is the same asymmetry the classifier records from
    its side: the two packages are answering different questions about the same
    evidence.
    - **Requiring a shape and not a word is what makes that true, and the
      first version of this rule did not.** It asked for "two or three
      dictionary words" and for the classifier's own `Prose`, and about two
      hundred of `names.txt`'s surnames are ordinary English words — green,
      lane, west, hill, stone, black, may, price, read, little, long — so a
      column of street names (`green lane`, `marsh lane`) or of compound
      colours (`hunter green`, `stone gray`) was still exit 9 as `person_name`,
      and any column of English sentences carrying one such word was exit 9 as
      `free_text` (T-0055's review). The given-then-surname pair is evidence a
      street name, a colour and a contract clause cannot carry, and
      `internal/textsig` keeps `names.txt`'s two sections apart so the question
      can be asked.
  - **The two dictionary validators do not run over a document's leaves, and
    that is a hole with a task against it.** `internal/classify` decides every
    `json`/`jsonb`/`hstore` column on its own leaf signal, which asks the rule
    pack's key patterns plus email, phone, IP, IBAN and Luhn about a leaf and
    never consults the dictionary. Scoring `person_name` or `free_text` over
    leaves here would therefore refuse a loaded target on evidence the
    classifier is structurally unable to have seen — a masked `jsonb` column of
    ordinary notes at exit 9, with `--unmask` the only way past it — which is
    the same refusal the rule exists to prevent, in the one place the
    classifier cannot pre-empt it. **Owed: `tracker T-0087`** — give classify's
    leaf signal a dictionary question and this exclusion comes off with it, in
    the same commit. The other six validators do run over leaves, because they
    are the classifier's own leaf questions.
  - **The `minValues` branch above does not extend to these two.** For the six
    validators that carry a parse, any hit below `minValues` is exit 9
    (T-0058). A dictionary word is not a parse — one two-word value in a
    two-row lookup table is not evidence of a person — so the strong ratio is
    required here whatever the column's size, and the distinct-value floor
    means one literal repeated down a column cannot reach it either.
  - **What it costs, stated rather than hidden.** A one-word name column the
    classifier did not mask — a `forename` the rule pack's patterns missed —
    passes this net, and so do a name column written surname-first (`Hopper,
    Grace`), a note naming a person the dictionary does not carry, and written
    names inside a document's leaves. That is the direction this has to fail
    in, and it is a narrowing of the net and not of the classifier:
    `internal/classify` still scores `LooksLikeName` at one word and `Prose` at
    one dictionary word, and still masks the column. **Owed:
    THREAT_MODEL.md T1's gap list still says the net has no `person_name` and
    no `free_text`; it now says the wrong thing in both directions and should
    name the dictionary rule instead** — that file is not in this task's paths
    and the edit is reported in T-0055's return value.
  - `TestTheDictionaryRule` pins every half of it without a database: the four
    columns that must not fail on a word (single dictionary words, two-word
    street names, compound colours, and business prose whose only dictionary
    word is an English one such as *may* or *price*), the two that must fail on
    the shape (written names, and prose naming another row's person), the two
    ways a dictionary hit is too thin to count (two distinct values, and one
    value repeated), and the masked `jsonb` column whose leaves carry that same
    failing prose and must pass. Every negative case there fails under the
    validators this rule replaced, which is what makes them regression guards
    and not decoration.
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
- **The JSON leaf walk,
  the value rendering, the type families and the chunk-join SQL are still copies
  of `internal/transform`'s and `internal/extract`'s.** Stage packages may not
  import each other (internal/CLAUDE.md) and every one of those is unexported
  where it lives. What was copied is the value-only half in each case — a
  canonical rendering, a family table, an identifier quoter — and never a rule
  pack: `internal/transform` refused a second rule pack for the same reason and
  this package refuses one too. **The fix is a shared home for each, not a third
  copy**; `internal/transform`'s CLAUDE.md already owes the type families one.
  T-0055 built the first of those homes and moved one copy into it: the
  validators and the name dictionary are `internal/textsig` now, and this
  package holds neither. The rest of the debt stands, and a change to
  `internal/transform`'s residual-filter contract still has to be made here in
  the same commit, with the unit tests in this package the thing that fails when
  it is not. `internal/textsig`'s own CLAUDE.md records what may follow it there:
  a value shape, never a rule pack, and never something that needs `pipeline`.
- **The first chunk of a key set is asked for by name** (T-0050).
  `Chunks(n)` materialises the *whole* set as typed arrays; the sample compare
  needs 100 keys, so `Chunks(100)` on a step at the `--row-budget` ceiling
  allocated a second copy of that step's key set, per table, at verify time —
  after `--memory-budget` (§8, exit 11) has been checked at plan and can no
  longer refuse anything. `internal/pipeline`'s `KeySet` now carries
  `FirstChunk(n) Chunk` beside `Chunks` and `EachChunk`, `internal/plan`'s two
  implementations have it, and `sample.go` calls it directly: the optional
  accessor it used to type-assert for, and the `Chunks` fallback behind it, are
  both gone. A `nil` chunk is an empty key set and the step is skipped, which is
  the same answer the fallback gave for a set with no chunks.
- **The per-table and per-column shapes name their table and column, and a name
  is arbitrary text.** `sampleShapeFor` and `probeShapesFor` quote a table (and,
  for the probes, a column) into the template precisely so the shape admits one
  relation instead of every relation. A table called `{ident}` used to defeat
  that — the template compiler read the placeholder inside the quoted name and
  compiled the shape into a table-agnostic one — and one called `{table}` failed
  to compile at all. The fix is in `internal/pg`'s `templateSegments` (T-0050),
  which treats a quoted identifier in a template as fixed text and matches it as
  Postgres reads a name — case-sensitively and space for space, where the rest
  of the template is matched case-insensitively with elastic whitespace; nothing
  in this package changed, and nothing in it should re-quote around the problem.
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
still an array, a number leaf is not a hit, and the second net's two thresholds
hold (`TestAColumnBelowMinValuesFailsOnAnyHit`, `TestTheDictionaryRule`).
`TestSecondNetReadsDocumentKeysAsWellAsValues` (2026-09-14 review round,
finding 3) pins the second net over a JSON object key and not only a value,
and its three `masked: true` cases (**T-0172**) pin the fix beside it: a
masked document column's key and leaf, each shaped like the masker's own
output for email, phone and the Luhn half of `financial_account`, pass the
net that used to refuse them.
The
`citext[]` fixture of `arrayliteral_test.go` is the T-0129 half: the grammar is
transform's on both case lists, a masked array that arrives as a literal is
tested one entry per element and never as one string
(`TestAMaskedArrayLiteralIsScannedElementWise` asserts the *bytes*, because a
scan that asks the wrong question passes for the wrong reason), an element the
filter holds reaches confirmation
(`TestAnElementOfAMaskedArrayLiteralIsFoundByTheScan`), a literal that will not
parse is exit 9 naming the column with no green tick beside it
(`TestAMaskedArrayValueThatIsNotALiteralIsRefused`), and the second net reads
the same literal element-wise
(`TestTheSecondNetReadsAnArrayLiteralElementWise`).

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

## The 2026-09-15 red team

Three changes here, all widening what the net and the catalog pass look at.

- **`famBytea` is in the second net, under a content guard** (A4a). `netText`
  named the character families plus uuid/inet/cidr/macaddr, and `bytea` was
  outside it for the same reason `internal/classify`'s `bestSignal` returned
  early for the family — a PNG rendered as a string carries a number and two
  words and is an address to `addressShape`. That reason is about *bytes*. A
  `bytea` holding printable UTF-8 with an address in it was read by nothing on
  either side, in a table with no `certain` column so the classifier's
  `byteaInPersonShapedTable` could not fire either. `netMode` now sets
  `bytea: true` for the family and `withDocuments` drops any value
  `textsig.PrintableText` rejects, so a column of images is still read by
  nothing and a column of documents is read like text. The guard is the same
  function `internal/classify` calls **and, since the T-REDFIX review's second
  finding, the same rule**: that package had a column-level 95%-readable ratio
  in front of the per-value question, so the two nets did disagree about a
  half-printable column — unmasked there, exit 9 here, no green path — for as
  long as both files claimed they could not. The ratio is gone; neither side has
  one.
- **A JSON document inside a `text` column is walked** (A5b). `document()`
  covers json, jsonb and hstore, so a three-level document in a column declared
  `text` — one of the commonest shapes in a real schema — was seen by neither
  this net's leaf walk nor `internal/classify`'s leaf signal, and its leaves
  were chosen so that no validator fires on the document read as one string.
  `withDocuments` now parses any character (or readable `bytea`) value and, when
  it is an object or an array, yields its leaves and keys **as well as** the
  value itself. A bare JSON scalar is not a document: reading `"12345"` as one
  would double-count every numeric-looking text column in the target.
  - The leaf-derived strings are returned **separately** from the column's own,
    and `count` skips the dictionary-backed validators on them. That is the
    same rule `applies` states for a json column, applied to the text column
    holding the same bytes, and for the same reason: `internal/classify` runs
    no dictionary signal over a document's leaves, so a `person_name` or
    `free_text` refusal there would fail an already-loaded target on evidence
    the classifier is structurally unable to have seen.
  - **They are scored separately too, and the first version of this was not**
    (the T-REDFIX review's high finding). `netColumn` counted every leaf into
    the column's own `nonNull`, which is both the ratio's denominator *and* the
    `proven` gate of the T-0058 branch — so putting a JSON document into a text
    column *raised* the denominator and flipped the column from "unproven, any
    hit fails" to "proven, ratio ≥ 0.8 required". A two-row column holding
    `221 Baker Street, London` and `ok` is exit 9 as `address`; the same column
    with the second row replaced by a four-key object recorded no failure at
    all, and a production address in the loaded target went from exit 9 to exit
    0. The dictionary rule was diluted the same way on any text column that also
    held a document. `netTally` is the fix: one denominator for the column's own
    values and one for the strings a document yielded, each scored on its own by
    `scoreHits`, either failing the column. The two cases in
    `TestRedTeamSecondNetReadsWhatItUsedToSkip` — the same address with and
    without a document beside it — are the guard, and the second of them passed
    before this.
  - **This is closed fail-closed and not closed properly.** Such a column is
    exit 9 here; `internal/classify` still cannot mask it, because
    `semi_structured`'s writable families are json/jsonb/hstore in the `mask`
    module. Tracker **T-0182**.
- **The catalog pass reads two more object classes and two more validators**
  (A4b, A11, A12, A20). `pg_enum`'s labels, which `internal/load/ddl` writes as
  string literals into `CREATE TYPE ... AS ENUM`, and `pg_type.typdefault`,
  where a domain's `DEFAULT` lives — `pg_attrdef` does not carry it, so the
  three reads could not see it, and it is the 2026-09-09 finding 5 mechanism one
  catalog table to the left. A label is the value rather than an expression, so
  `readLiteral` quotes it into the form `pipeline.Literals` scans; neither class
  is ever exempt, because neither is ever rewritten. And `strongCatalogHit` gained
  `national_id` and IBAN, which stay in step with `internal/plan`'s `strongHit`
  by hand as they always have.

**Never:** use the run's own (released) snapshot; treat an unconfirmable
residual hit as anything but exit 9; skip the cap on confirmation probes; treat
a source-changed mismatch as a pass; let a value reach a refusal, a check or an
event; scan a masked array column that arrived as a text literal as one string,
or let a literal this stage cannot split be anything but exit 9 naming the
column.

## The 2026-09-15 red team, round 2 (T-0187)

**`national_id` joined the second net's `validators` table.** R2-01
through R2-04 (`docs/reviews/2026-09-15-redteam/round2-still-leaking.json`,
attacks A2, A6, A7, A9b) found that `strongCatalogHit` above already carried
`national_id` — the note two paragraphs up — but nothing in *this file's*
table did, so a plain SSN or NI number in a column the rule pack's name
patterns miss reached the loaded target unmasked, was scanned by this net's
eleven entries, and matched none of them. Three entries close it, in
`validators.go` (the character-family half split in two by the review round
that followed, below, so this is three entries and not the two the round
originally landed):

- `{text: true, strong: true, ok: textsig.ValidNationalIDStructured}` — the
  six shape-constrained formats (a US SSN, a UK NINO, an Italian codice
  fiscale, a Spanish DNI or NIE, a French NIR), at the same footing as email:
  each also constrains the value's *shape* — a dash, a letter, or a fixed
  length under its own mod-97 check — so none matches a bare digit run at
  all, and any hit is one production identifier in the target, whatever the
  ratio.
- `{text: true, ok: textsig.ValidNationalIDChecksumOnly}` — the other six
  (PESEL, BSN, SIN, TFN, Aadhaar's Verhoeff check, CPF), at the ordinary
  ratio rather than `strong`: each is a mod-N sum over an otherwise
  unconstrained digit run and clears a meaningful fraction of a random string
  of the right length regardless of what it means (measured: 25.7% for a
  random 9-digit string), which is not a precise enough claim for "any hit
  fails". This is the text-family half of the split the T-0187 review round's
  finding 2 asked for; `internal/classify/CLAUDE.md`'s own note records the
  same split on that package's side, and why it is only a partial answer
  there.
- `{digits: true, minRatio: nationalIDDigitsThreshold, sequenceExempt: true,
  ok: textsig.ValidNationalIDDigits}` — A9b's own half, mirroring T-0136's
  Luhn text/digits split for the same reason: a numeric column drops an SSN's
  hyphens and, when the area starts with 0, its leading digit, so the
  structured entry's dashed regexp never matches a bigint's rendering of the
  same number. `ValidNationalIDDigits` is the function that recovers it, and
  it is deliberately **not** strong: an SSN carries no check digit at all, so
  a bare nine-digit number is "SSN-shaped" about as often as a random
  nine-digit number clears the SSA's exclusion ranges — a claim precise
  enough for the ordinary ratio rule and far too wide for "any hit fails".
  `ADR-010`'s numeric-family silencing never had to be touched for any of the
  three: `rules.yml`'s `national_id` category already accepted
  `bigint`/`integer`/`numeric` (`mask/gen_number.go`'s masker already emits
  digits into them), so the premise that would have needed exempting was
  already false. `minRatio` and `sequenceExempt` are the review round that
  followed T-0187's own fix, below, and `validators.go`'s own comment on the
  entry has the full account.

**This closes a gap `internal/classify` still has, and that is the accepted
asymmetry, not a second bug.** `internal/classify`'s own new `national_id`
entry (its own CLAUDE.md, T-0187) calls plain `ValidNationalID` uniformly
across every family, because that package's ordered validators list has no
`text`/`digits` split the way this file's does — so a `bigint` column with no
name hit is still decided `none` there and copied by `internal/transform`.
This net's digits entry is what catches it: `testdata/regressions/020-ssn-
stored-as-bigint.sql` runs such a column through end to end and asserts the
refusal, `expect: exit 9 verify.refused.second_net`, the same "refusal instead
of a mask" shape the dictionary rule and the Luhn split above already argue
for. Fixtures 018 and 019 pin the character-family half (a plain SSN in a
column called `code`, a `text[]` of NI numbers), each `expect: ok` with a
`not-copied:` key naming the column — the generic leak check every `expect:
ok` regression gets, `assertTortureNoLiteralSurvives`, only ever proves the
absence of the two shapes `internal/invariants/scan_test.go`'s own detectors
recognise, an email and a phone number, and a national identifier is neither
(`internal/invariants/CLAUDE.md`'s own note on the point).

## The review round that followed T-0187: the digits entry's ratio was not the answer, and neither was the first fix

`ValidNationalIDDigits` has no check digit at all — the SSA's own exclusion
ranges are the whole of the check — so a first review round (finding 1) found
it clearing an ordinary surrogate `id bigint PRIMARY KEY` and an ordinary
`booked_on integer` booking-date column at rates well over `validatorThreshold`
(0.8): a threshold raised to `nationalIDDigitsThreshold` (0.97) plus a skip
for a column `internal/classify` had already decided is a surrogate key
(`surrogateExempt`/`netMode.surrogateKey`, now removed) was the first fix, and
`testdata/regressions/021-ordinary-numeric-columns-clear-the-ssn-ratio.sql`
was its guard.

**A second review round found both halves of that fix were themselves wrong,
and the two findings are the two paragraphs below.**

**Finding 1: 0.97 was set above a figure that was itself an artefact.** The
entry's own comment justified the threshold with "~97% of YYYYMMDD integer
dates", averaged over 1950-2050 — a range where the only dates the padded SSA
check rejects are the two whose two-digit year suffix is itself the excluded
group `00` (1900, 2000). Over any realistic booking range the true clear rate
is 1.0, and the reviewer measured `ValidNationalIDDigits` at 1000/1000 on
YYYYMMDD dates across 2022-2024 and 10000/10000 on a sequential 9-digit
non-key business number (`400100000+i`) — the second of which the
decision-based exemption could never reach at all, because nothing about that
column's name or values ever told `internal/classify` it was a key.

**Finding 3: the decision-based exemption was also a regression, and a
serious one.** `surrogateKeyExempt` read `internal/classify`'s own "preserved
verbatim" reason, which classify grants a key column precisely when *its own*
name and value signals found nothing personal in it — the same miss this net
exists to catch a second time. Gating the digits entry's skip on that
decision meant a `citizen_no bigint PRIMARY KEY` (or an FK to one) holding
real SSNs, with a column name `rules.yml` misses, was exempted by classify,
skipped by this net, and crossed into the target verbatim at exit 0 — a
column that refused correctly before the first fix's exemption existed.

**The fix is now in three parts, and none of them is this entry's ratio
alone, or a second read of classify's decision.**

- `textsig.ValidNationalIDDigits` excludes a value that is also a real
  YYYYMMDD calendar date directly (`looksLikePlausibleDate`,
  `internal/textsig/nationalid.go`), rather than leaving it to this ratio: a
  column of real dates now scores zero hits from this entry regardless of the
  threshold, which is what actually answers finding 1's date case.
  `internal/textsig/CLAUDE.md` records the function on that package's side.
- `digitRange` (`secondnet.go`) reads the column's own values during the
  scan — never `internal/classify`'s decision — and marks a column *dense*
  when its values pack into a numeric range within a factor of two of their
  own count: a surrogate key's own values (`id`, `id+1`, `id+2`, ...) and an
  ordinary dense business-number block with no key at all are both dense:
  finding 1's own sequential-business-number case is answered the same way
  its own key case now is, by one signal read from the values rather than
  two, one of them a decision. A primary key of independently assigned SSNs
  is not dense — real identifiers are drawn from a space many orders of
  magnitude wider than the sample — so finding 3's attack still refuses,
  whatever `internal/classify` decided about the column being a key.
  `validator.sequenceExempt` (`validators.go`) is the field the digits entry
  alone sets; `netMode.surrogateKey` and `surrogateKeyExempt` are gone, not
  narrowed, because reading classify's decision at all was the mechanism
  finding 3 exploited.
  - **Order-independent on purpose.** `scanSQL` (`sql.go`) is `SELECT column
    FROM table` with no `ORDER BY`, so the order this net sees a column's
    values in is Postgres's own heap scan order and not a signal this net
    may read a *sequence* out of. `digitRange` asks only the *span* the
    observed values cover against how many there are, which answers the same
    question whatever order the scan delivers them in.
- `nationalIDDigitsThreshold` stays, at the same 0.97, for what is left once
  the two structural exclusions above run: a column that is neither a date
  nor a dense sequence and still clears the SSA exclusion ranges on more than
  97% of its values. It is no longer asked to carry the date case or the
  sequence case on its own, which is what made it insufficient the first
  time.

**The regressions.** `021` is rewritten rather than re-tuned: its own header
used to carry one hand-placed non-2024 row to hold the date column's ratio
just under the old threshold, which pinned the threshold's *number* rather
than the property that a column of real dates and a column of a real
generated sequence must both pass regardless of how many rows they hold or
what values they take. The rewritten file holds a `booked_on` column whose
every value is a 2024 date and a second, non-key `business_ref bigint` column
whose values are a dense run with no key or uniqueness constraint on it at
all, both `expect: ok`, so the fix cannot be satisfied by fixture
construction. `022-national-id-in-a-surrogate-key-column.sql` is finding 3's
own regression: a `bigint PRIMARY KEY` holding real-shaped, non-dense SSNs
chosen to also fail every checksum-only format, so `internal/classify` grants
its ordinary key exemption and this net is the only control left,
`expect: exit 9 verify.refused.second_net` — no `not-copied:` key, the same as
020, because the harness never checks an optional key on a non-zero exit
(`testdata/regressions/README.md`). `020-ssn-stored-as-bigint.sql` (a genuine
SSN, neither a date nor dense, ratio 1.0) is unaffected and still refuses.

## The review round that followed that one: no ratio is precise enough, so the entry now needs corroboration

**A third review round found a shape neither `nationalIDDigitsThreshold` nor
`digitRange` answers: a *sparse* numeric column with a fixed leading prefix.**
It is neither dense (`digitRange.dense()` needs the observed span within twice
the row count, and a sparse column's span is many orders of magnitude wider)
nor a date (`looksLikePlausibleDate` only ever excludes an eight-digit value),
so nothing the second review round's fix added reaches it, and it clears the
SSA's exclusion ranges at essentially 1.0 the same way a genuine leaked
identifier column does — the reviewer measured 494/500 for 500 account
numbers of the form `100000000+rand(1e8)` and 500/500 for 500 invoice numbers
of the form `202600000+7*rand(50000)`. An ordinary `account_no` or
`invoice_no` column of either shape refused an already-loaded run at exit 9
with no green path short of `--unmask`, and no threshold under 1.0 fixes it:
an assigned identifier and a fixed-prefix reference number clear the same
ranges at the same rate, so there is no ratio that admits one and still
refuses the other.

**The fix is not a fourth exclusion rule; it is a gate in front of the ratio.**
`validator.requiresCorroboration` (`validators.go`) is true for the digits
entry alone, and `netColumn` (`secondnet.go`) skips the entry entirely —
never scores it, whatever the ratio — for a column `corroborated` answers
false for:

```go
func (s *state) corroborated(col ref.ColumnRef) bool {
	d, has := s.decision(col)
	return has && (d.NameMatchedNationalID || d.TableHasLikelyPersonalColumn)
}
```

Both signals are read off `pipeline.Decision`, never re-derived: this package
may not import `internal/classify` (`internal/CLAUDE.md`'s import graph), and
a second copy of the rule pack's name-matching or of the neighbouring-column
rule's own count here would be exactly the drift this file's own opening
paragraphs already warn a hand copy of the classifier's validators risks.

- **`Decision.NameMatchedNationalID`** is `rules.yml`'s `national_id` name
  pattern (priority 75) matching the column's own name, set in
  `internal/classify/classify.go`'s `decide` independent of whether the type
  was accepted or which category the decision went on to record. In practice
  this can answer true for the digits entry only on a column `internal/classify`
  leaves unmasked for some other reason — a surrogate key or FK column, whose
  `neverMask` exemption is granted *before* a name hit would otherwise raise
  it to `possible` — because an ordinary bigint column with an accepted-type
  name hit is masked outright by `decide` and never reaches this net at all.
- **`Decision.TableHasLikelyPersonalColumn`** is `internal/classify`'s own
  neighbouring-column rule's `likely` count (`neighbouringColumns`), carried
  onto every decision in the table rather than only the ones that pass gets
  raised — "another column of this table was decided at `likely` or
  `certain`" — computed with the column's own confidence excluded, since a
  digits-family column that reaches this net is never itself at `likely` or
  above (it would be masked and excluded before `netMode` runs).

**Without either, the ratio is never asked at all** — not scored against
`nationalIDDigitsThreshold`, not checked for denseness or a date, skipped
outright — which is the direction that has to fail safe here: a column that
*is* corroborated still goes through every check the second review round
added, at the same thresholds, so a genuine leak beside a proven personal
column or under a matched name is caught exactly as before.

**Picking a corroborating column for a fixture is not free, and 020 is the
proof.** `TableHasLikelyPersonalColumn` is computed by the *same* pass
(`neighbouringColumns`) that already raises an ordinary `low`-confidence
column to `possible` beside a `likely` one — an existing rule, unrelated to
this finding — so adding a personal column to a table that already has a
digits-family column sitting at `low` (not `none`) masks that column outright
via the pre-existing rule instead of corroborating it. `020-ssn-stored-as-
bigint.sql`'s original five values were exactly that trap: three of the five
coincidentally cleared one of `internal/classify`'s own checksum-only formats
(BSN's 11-proef, or Australia's TFN) on the bare digit string, which put
`taxref` at `low` even though the file's own prose always said `none`. Adding
`email` for corroboration would have raised `taxref` to `possible` and masked
it, defeating the regression a different way than the one it exists to catch.
The file's own header now explains this and its five values are replaced with
ones confirmed, by direct computation, to clear none of
`internal/classify`'s national_id formats.

**The regressions.** `023-sparse-fixed-prefix-reference-block-is-not-
national-id.sql` is the finding's own probe, reduced: a ten-row `account_no
bigint` column with a fixed leading digit and eight further digits spread
across a range many orders of magnitude wider than the row count (span
about 92.5 million against ten rows, nowhere near `digitRange`'s
within-twice-the-count test), all ten clearing the SSA's exclusion ranges,
`expect: ok`, in a table with no other column and a name matching no
`rules.yml` pattern. `020` and `022` both needed a corroborating `email`
column added for the same reason `023` exists — each `taxref` and
`citizen_no` had neither signal on its own — and both still refuse on the
same ratio, dense-range and date logic as before, now that the table gives
them a `likely` neighbour to read.

## Which sequence to read (T-TORTURE)

`sequenceNameSQL` asks the **target** which sequence backs a column, with the
source's name as the fallback, and `counts.go` resolves it before reading
`last_value`. An identity column's sequence is created and named by the target
(§11.1 recreates the column as `GENERATED ... AS IDENTITY`), and the two names
part company the moment the source's table has been renamed — Postgres does not
rename an owned sequence with its table. Metabase's `sandboxes` still owns
`group_table_access_policy_id_seq`; reading that name against a target that has
`sandboxes_id_seq` is `42P01` against a target that is correct
(`testdata/regressions/006-identity-sequence-renamed-table.sql`).

The fallback is not decoration: pagila declares no sequence ownership at all, so
`pg_get_serial_sequence` answers NULL for every one of its sequences and the
source's name is the right one. `internal/load/ddl`'s `Setvals` makes the same
choice the same way, and the two have to keep making it together.

**When neither name resolves, the check fails.** `sequenceNameSQL` coalesces to
the empty string, and that answer means the target has no such relation — a
sequence §11.1's DDL did not create. It raises `verify.refused.sequence` with
`reasonNoSequence` at exit 7, exactly as the `42P01` it replaced used to. It is
deliberately **not** `verify.sequence.unowned`: that code is §6 item 5's report
for "the loader could not attribute this sequence to a column, so it did not
reset it", which is a state a correct run reaches (pagila reaches it twelve
times). Reusing it here would turn the loudest evidence of THREAT_MODEL.md T8 —
a target that looks complete with its sequences at 1 — into a passing note, and
`sequences()` counts the sequence as checked before the call, so the pass would
have counted it too.

## The catalog pass (T-0134)

`catalog.go` is the eighth check and the first whose subject is not a row. Every
other check here reads what the target holds in a *row*; this reads what it
holds in its *schema* — `pg_attrdef`, which carries both the column defaults and
the generated-column expressions `internal/load/ddl` wrote, `pg_constraint`,
which carries the `CHECK` and exclusion definitions (a domain's `CHECK` with
them, since `conrelid` is 0 there rather than absent), and `pg_index`, which is
where a partial index's predicate and an expression index's key expressions
live. Every string literal in each text goes through the three strong validators
and a hit is **exit 9** naming the table and the object
(`verify.refused.catalog_literal`, `checkCatalog`, between the second net and the
FK check in `order`).

Why it exists: the 2026-09-09 review's finding 5 put `DEFAULT
'ddl.canary@example.org'` on a masked email column, and every control in §6
passed — the rows *were* masked, the filter held their digests, the second net
found nothing — while the address sat in the target's `pg_attrdef` waiting for
the application's next `INSERT` to put it back into a row. A scan of the rows
cannot establish that the database artefact holds no sensitive literal.

- **It is the second look, not the first.** `internal/plan/ddlliteral.go`
  refuses or rewrites before anything is dropped (ARCHITECTURE.md §11.1's
  2026-09-14 amendment). The two are deliberately not one code path: the planner
  reads the source's `*pipeline.Schema` and this reads the target's catalog, so
  a literal that reached the target by a route the planner does not walk — an
  index predicate, a domain's `CHECK`, an object somebody added to the target by
  hand — is still found here.
  - **The index read is not decoration, and the third statement exists because
    the sentence above was false without it** (T-0134's review round).
    `internal/plan` never reads `t.Indexes`, and `catalogConstraintsSQL` reads
    `pg_constraint` with `contype IN ('c','x')` — a partial index's predicate
    lives in `pg_index.indpred` and has no `pg_constraint` row — so
    `CREATE UNIQUE INDEX ... WHERE email = 'x@y.test'` crossed into the target
    unseen by *both* halves while the file comment claimed the route was covered.
    `catalogIndexesSQL` reads `pg_get_expr(indpred, indrelid)` and
    `pg_get_expr(indexprs, indrelid)` as two arms of one `UNION ALL`, so the kind
    the refusal names is `index predicate` or `index expression` rather than one
    word covering both. **T-0163's plan-side half is closed as of T-0189
    (2026-09-15):** `internal/plan`'s `tableDDLLiterals` now walks `t.Indexes`
    the same way it walks `t.Constraints`, so an index predicate is refused at
    plan — exit 12 or 13, before anything is dropped — and this pass's own
    index read is a genuine second look at the same object now, not the only
    look there ever was.
- **A type the operator opted out of with `--allow-type-literal TYPE=REASON` is
  exempt for every object that belongs to it** — its enum labels, its `DEFAULT`
  and its `CHECK` (`allowedTypeLiteral`, read off `Plan.AllowedTypeLiterals`).
  `internal/plan`'s type-literal refusal (exit 13) honours the same opt-out, and
  before the T-REDFIX review's fourth finding it had no escape at all: it named
  `--skip-table`, which drops a table to *schema only* and still recreates every
  type. A pass here that ignored the opt-out would load the target and then
  refuse at exit 9 over the object the operator was told they had allowed, which
  is worse than no escape — §8's rule that an escape means the same thing at
  both ends, applied to the one object class that is not a column. A domain's
  `CHECK` arrives in the same `pg_constraint` read a table's does, so
  `catalogConstraintsSQL` now spells its own kind (`domain constraint`) and
  carries the domain's name in the relation position: without that this pass
  could not attribute a domain `CHECK` to the type the operator named, and a
  *table's* `CHECK` is never exempt whatever type names it carries.
- **One exemption, and it is about provenance rather than shape**
  (`catalogExempt`). A column this run **masked** is exempt for its own
  `DEFAULT` **when the planner actually rewrote that default**, and a column
  carrying an `--unmask` opt-out is exempt for its
  `DEFAULT` and its generated expression. Without the first arm this pass made
  §11.1's central case unreachable: `internal/plan` masks a masked column's
  default through that column's own masker, and an email masker's output is a
  valid address by construction (`mask.Apply` over the review's
  `ddl.canary@example.org` gives another working address), so a correctly masked
  default was exit 9 on every run. This stage cannot tell the masker's output
  from the source's value by inspection — it holds no key, and the residual
  filter holds cells the transformer masked, never a default — so the
  classification is the only thing that can answer, which is exactly what
  `netMode` does on the row side (`case has && d.Masked: return netMode{},
  false`). The second arm is ARCHITECTURE.md §8's escape meaning the same thing
  at both ends: `internal/plan`'s `optedOut` does not refuse that column's DDL
  either.
  - **The masked arm asks the rewrite, not the decision** (`rewroteDefault`,
    T-0134's review round). `d.Masked` says the *rows* were masked; it does not
    say the masker's output is what stands in `pg_attrdef`. The planner records
    the catalog's own text on `pipeline.Column.DefaultOriginal` when it rewrites
    a default, `internal/core` hands one `*pipeline.Schema` to both stages, and
    this arm reads that field — so it is closed on a column whose default was
    left exactly as the source wrote it. That distinction is not academic while
    **T-0161** is open: `internal/core` does not fill
    `pipeline.PlanRequest.Key`, the planner therefore rewrites *no* default on
    any real run, and an arm keyed on `d.Masked` alone would exempt an object
    nothing ever rewrote — the whole masked-default class, unjudged, for a
    control that exists because a masked column's default was the leak.
    `catalog_test.go` pins both: the rewritten default passes and the
    unrewritten one is exit 9.
  - **It is per object class, not per column.** A `CHECK`, an exclusion
    constraint, a generated expression on a *masked* column and an index
    predicate are never rewritten by anything, so a strong hit in one of them is
    the source's own literal whatever the classification says about the columns
    it names — and this pass is a second look at the artefact, not a re-run of
    the planner's judgement. `catalog_test.go` pins both directions: the masked
    column's masked default passes, and a `CHECK` on the same masked column does
    not.
- **Every validator that is a parse or a shape, not only the original five
  (T-0189, 2026-09-15 round-2 red team, R2-07 and R2-09).** The old narrowing
  — this text is SQL and not data, so a `CHECK` is full of English words and a
  default is full of identifiers, and the dictionary-backed validators would
  fail an already-loaded target over a column named after a street — is an
  argument about *identifiers*, and `strongCatalogHit` never sees one:
  `pipeline.Literals` returns only the quoted string constants a deparsed
  expression carries. Run over a literal rather than over the whole
  expression, the argument does not reach `person_name`, `address` or
  `free_text`, and R2-07 (a table `CHECK`) and R2-09 (an enum label, a domain
  `CHECK`, a domain `DEFAULT`, a generated expression) both crossed under
  exit 0 before this task. `network_id` and `online_id` join for the same
  reason, on the same footing as email or phone: a parse, not a guess.
  `credential` (`LooksSecret`) is the one category that does not join — see
  `strongCatalogHit`'s own comment for the measured, unrelated reason
  (`make torture` found three real schemas and a regression refusing over an
  ordinary sequence name once it was included). THREAT_MODEL.md T1 states the
  amendment. **R2-09's special-category sentence is not closed by this list,
  only narrowed by accident** (T-0189 fix round, 2026-09-15 review, finding
  4): `pipeline.CatSpecial` has no entry here either, so the canary is caught
  only because its date happens to supply `AddressShape`'s digit — strip the
  date and a health/special-category sentence with no name pair and no digit
  still crosses at exit 9's own green tick. Tracker **T-0198** carries a
  validator for it.
- **`address`'s validator needed corroboration for a one-hit refusal**
  (T-0189 fix round, finding 2). `textsig.AddressShape` is calibrated for
  this package's *own* second net (`validators.go`'s `minValues`/
  `validatorThreshold`), which fails only once many rows agree — it is
  explicitly marked not strong there for that reason — and this pass asks it
  of one literal in an already-loaded target. Measured against ordinary
  `CHECK` value-list and enum-label text, the bare shape also hits pricing
  tiers and priority labels ("Basic 1 user", "P1 High Priority", "Top 10
  sellers") and refuses the run at exit 9 over a value that was never a
  person's, with no escape but `--allow-type-literal` on an object that
  carries no address at all. `addressLiteralShape` (this file) corroborates
  it with a street-type suffix word — `addressSuffixWords` — the same
  shape `internal/plan/ddlliteral.go` carries under the identical two names,
  for the reason every other entry in this list is already a duplicate: the
  two packages may not import each other. `textsig.AddressShape` itself is
  untouched; `internal/textsig` was outside this task's paths, and this
  package's own second net still wants the loose shape it already has.
- **`strongCatalogHit` is the second copy of `internal/plan`'s `strongHit`**,
  for the reason `textOf` and the identifier quoting are copies: a stage package
  may not import another. What is shared is the *scanner* —
  `pipeline.Literals` — because that is the part that decides what is inside the
  data boundary, and two answers to that question is the failure this file's own
  validator note records. The three-validator list is a shorter contract and is
  stated in both places; **T-0162** gives the scanner a leaf home and is the
  task that should take the list with it.
- **It reads no parameter and needs no shape.** All three statements are
  constants in `sql.go` and go to the target, which this package reads directly;
  the source allowlist is not involved.
- `catalog_test.go` holds it against a double that answers the three statements
  separately, so a test can say which object class carried the literal, and
  asserts the refusal names no literal (THREAT_MODEL.md T4) and that no passing
  catalog check sits beside it.
- **Guards for T-0189's own three findings.** `TestTheCatalogPassFindsALiteral\
  NoRowScanCanSee` gained a table `CHECK` carrying a person's full name, a
  partial index predicate carrying a genuine postal address (not an email, to
  tell it apart from the pre-existing index case, which already passed under
  the old five-validator set), a pattern operand carrying the red team's own
  anchored, dot-escaped regex, and its control — the same pattern with no
  value hiding inside it, which must still pass or every ordinary
  `LIKE`/regex `CHECK` in a real schema would refuse.
  `TestRedTeamCatalogReadsEnumLabelsAndDomainDefaults` gained a person's full
  name in an enum label. None of the pre-existing cases in either table moved,
  which is the guard against the widened set refusing what it already passed.
  As in `internal/plan`, `credential` never reached a unit test here — the
  fixtures this task wrote all use unambiguous name/address values, and
  `make torture` is what found the sequence-name false positive that
  motivated leaving it out; see `internal/plan/CLAUDE.md`'s own T-0189 entry
  for the measured detail, since the finding and the fix are identical in
  both packages. The T-0189 fix round added two more cases to
  `TestTheCatalogPassFindsALiteralNoRowScanCanSee`: a pricing-tier enum label
  is not a hit, and a genuine address in the same object class still is —
  the `addressLiteralShape` guard, above.

## T-0221 (2026-09-16): the phone entry reads a configured region, and only that

The phone entry in `validators.go` still calls `textsig.ValidPhone` (the
international-only, `PhoneRegionHint` "ZZ" reading) as its `ok`; the region
widening lives in `secondnet.go`'s `count`, now a method on `*state` rather
than a free function so it can read `s.opts.PhoneRegion`. When a value fails
`val.ok` and the entry is `pipeline.CatPhone` and `Options.PhoneRegion != ""`,
`count` also asks `textsig.ValidPhoneRegion(text, s.opts.PhoneRegion)`, on the
entry's own `strong`, any-hit-fails footing — no new entry, no new threshold.
`Options.PhoneRegion` is `internal/core`'s resolved value (the
`--phone-region` flag if given, else the committed yml's `phone_region`),
the same one `internal/classify` classified the row with.

**It is deliberately never `internal/classify`'s own guessed-region list.**
That package tries a short, fixed set of common regions when no region is
configured, and masks a hit only with corroboration — a column's name
matching the phone pattern, or a proven personal neighbour in the same table
(`internal/classify/CLAUDE.md`'s own T-0221 section has the full account,
including the collision the first landing had with this package's own
national_id digits entry before the guess was scoped to character families
only). Reading that same guessed list here, over an already-loaded target,
would be the row-path corroboration argument won with the opposite hand: a
column this net refuses is a refusal an operator has to act on, with
`--unmask` the only way past it, so a ten-digit account column that happened
to clear one of fifteen guessed regions would cost a correct run its green
tick on no real evidence. Staying on the configured region and the
international form only is what keeps this net answering the question §6
item 4 asks — does the loaded target hold a value of this shape — rather
than repeating a guess this package has no way to corroborate against a row
scan the way `internal/classify` can against a column's neighbours.
