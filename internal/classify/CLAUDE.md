# internal/classify

Decides what every column holds: name/type/value signals, dictionaries,
scoring, and `reasons.go`'s reason templates. Pure — no SQL, ever; samples
arrive only through `pipeline.Sampler`, taken by `internal/introspect`. The
embedded rule pack (YAML) lives here.

**Contract.** ARCHITECTURE.md §4 and §2 "classify": `Classifier.Classify(schema,
Sampler, prior *Config) (*Classification, error)`.

**Rules.**
- Biased to recall, not precision: after the neighbouring-column rule and FK
  propagation, `possible` and above is masked, always — this is
  THREAT_MODEL.md T1's primary control; a change here that could lower recall
  needs a T1 review, not just a test pass.
- No exemption by type. An enum, a partition key, a `varchar(2)` — all
  classified like anything else (ARCHITECTURE.md §4; the earlier enum
  exemption was removed).
- Every `Decision.Reason` is rendered from the fixed template set in
  `reasons.go`; a template's placeholders admit only identifiers and counts.
  `TestReasonGrammar` parses every emitted reason back against the set — never
  add a template that could interpolate a sample value.
- The classifier is not pluggable (ADR-006): no runtime rule loading, no
  registration hook for a caller-supplied rule. A pattern in `lazyslice.yml`
  may only add a category or raise a confidence, never lower one
  (`TestConfigCannotLowerConfidence`).
- Every category that a column can be *masked* under has a registered masker
  in `mask`, and a test walks both lists (ADR-006 "Consequences"). The
  exception is the one category that names the absence of a signal: `CatNone`
  is what a copied column carries, it reaches no masker, and it is excluded
  from that walk by name rather than by a wildcard.

**Test.** `go test ./internal/classify/...` — no database needed, this package
is pure.

**Never:** issue SQL; let a reason string carry a sample value; add a type
exemption; add a way for a category's confidence to be lowered by config.

## Files

- `rules.yml` — the embedded rule pack: the categories with their maskers and
  accepted type families, the name patterns with their priorities, the
  table-scoped patterns (T-0119: a name rule gated by a second regexp over the
  table, for a column name that means something different depending which
  table it is in), and the log-shaped table rule. Changing it changes
  `Classification.Fingerprint`.
- `literal.go` — reading a Postgres array or composite output literal back into
  the values inside it (T-0103, T-0094). A reader, not a parser: liberal in what
  it accepts, and what it cannot read stays one opaque value.
- `rulepack.go` — loading and compiling it.
- `types.go` — `pg_catalog.format_type` output to a type family, resolving
  domains and enums.
- `validators.go` — the thresholds and the sample-to-string conversion. The
  validators themselves, and the name dictionary they read, are
  `internal/textsig` (T-0055): `internal/verify`'s second net imports the same
  package, so there is one copy of a value shape and not two.
- `reasons.go` — the reason fragment set and `ParseReason`.
- `classify.go` — the six passes: base signals, bytea in a person-shaped table,
  the neighbouring-column rule, the two foreign-key passes (`keyChildren`, then
  FK propagation and shared names), the yml prior, then the threshold.
- `codes.go` — the five `event.Code`s the classify stage renders; each has a row
  in `internal/event/catalogue.yml`.

## Decisions made during implementation

ARCHITECTURE.md §4 is silent on each of these; the simplest correct behaviour
was chosen and is recorded here rather than only in a comment.
- **Phone region hint is `ZZ`, the unknown region.** §4 says "libphonenumber
  `IsValidNumber` with region hint" and §10's example reason says "region hint
  from country", but a per-table hint would mean reading a sibling column's
  production values to decide how to read this one, which is a second value
  dependency for a signal that only ever moves a column from `possible` to
  `certain`. Under `ZZ` only an international-format number validates; a
  national-format phone column still reaches `possible` on its name and is
  masked. Reversal condition: when `mask` grows the region-aware canonicaliser
  §5 needs, the same hint can be shared with it.
- **A partial value signal is `low`.** §4 scores "≥80% of non-null samples
  validating" and says nothing about 60%. A validator between 50% and 80% with
  no name hit records the category at `low`, which is below the mask threshold
  on its own and is what the neighbouring-column rule then raises. Without it
  that rule has almost nothing left to raise, because T-0033 excluded the
  type-conflicting decisions that were its other source of `low`.
- **Three or more non-NULL samples before a *weak* value signal counts, and a
  strong one decides at any number above zero.** One row that parses as an
  address is evidence about a row, not a column — so `low`, which the
  neighbouring-column rule raises, needs `minSamples` values under it. A column
  below that number is *unproven*, not clean, and an early return before the
  validators ran was a fail-open with only `internal/verify`'s second net above
  it, which had a floor of its own at the same number: `public.devices.owned_by`
  in `testdata/nasty.sql` is two email addresses and a NULL in a three-row table
  under a column name that says nothing, and it reached the target in cleartext
  under exit 0 (THREAT_MODEL.md T1). `TestI2NothingFlaggedSurvives/nasty` is
  what found it and tracker T-0058 is the fix: below `minSamples` the validators
  still run and a strong ratio — both of two values, or the only one — decides
  the column, which is "when in doubt, mask it" (CLAUDE.md). Nothing about the
  sample changed: `TABLESAMPLE SYSTEM` at 100% already returns every row of a
  small relation, and `internal/introspect`'s own integration suite asserts the
  three sample rows of `public."LegacyCustomer"`.
  `TestNastyOtherColumns/TwoValuesBelowMinSamplesStillDecide` is the unit pin,
  so restoring the early return fails `make test` and not only the Docker-gated
  invariant.
  - **The two thresholds are deliberately asymmetric, and the gap is a
    refusal.** Below its own `minValues`, `internal/verify`'s second net fails
    on *any* hit; below `minSamples`, this package masks only on a strong ratio.
    So a column with two non-NULL values of which one parses as an email is
    `none` here, copied by `internal/transform`, loaded, and then refused at
    exit 9 by verify — a run with no green path short of `--unmask`, over a
    column masking would have handled. That is the safe direction (before
    T-0058 the same run exited 0 with the address in the target) and it is not
    the same question asked twice: this package decides what to do to a column
    from a 200-row sample, and a minority hit in two samples is the noise
    `minSamples` was written about — raising it to `low` puts the
    neighbouring-column rule one hop from masking a column on the strength of a
    single row. The net decides whether a value that is *in the target* may
    stay there, where one address is one address. Closing the gap by masking on
    any hit below `minSamples` is a recall widening that needs a T1 review and a
    measurement against `TestPagilaPrecisionAndRecall`, not a quiet threshold
    change; until then the asymmetry is the record and verify's exit 9 is the
    answer to a minority hit.
    - **Above `minSamples`, a *strong* minority hit is masked here now too**
      (`signals.strongHit`, T-0136, docs/reviews/2026-09-09/REVIEW.md finding
      7, `evidence/sparse_email.log`). The paragraph above is about a column
      with too few samples for a ratio to mean anything; this is the other
      gap, over a *proven* column, and it was a leak rather than a refusal.
      One email address among nineteen ordinary strings in a proven column is
      a 5% ratio — under `weakThreshold` as well as `validatorThreshold` — so
      the column fell all the way through `best`, `weak` and `refused` to
      `none`, and `internal/transform` copied the address verbatim under exit
      0. `bestSignal` now records the first *strong* validator (a real parse:
      email, phone, network_id, the Luhn half of financial_account, online_id
      — the `validators` list's own `strong` field, mirroring
      `internal/verify/validators.go`'s) that matches at least one proven
      sample without reaching `validatorThreshold`, and `decide` masks the
      column as `free_text` on that alone rather than the hit's own category:
      the column is not reliably an email column, only mixed, and
      `free_text`'s masker replaces every value. `credential`, `address` and
      IBAN (financial_account's other half) are not strong and are
      unaffected — a slug or a room number is a shape guess, and IBAN is a
      mod-97 checksum over letters and digits rather than over a run of
      digits, so an ordinary all-caps string passes it about as often as any
      string of the right length does (five of pagila's own film titles do;
      `internal/verify/validators.go`'s own comment has the count). None of
      that is the same claim as a valid email address, and masking an
      ordinary column on one occurrence of any of them would be the wrong
      direction for a column that is not personal data at all. This narrows
      but does not close the asymmetry above: below `minSamples` every
      non-dict validator still decides only on the strong branch (a ratio
      over one or two values is either all of them or none), so a strong
      minority hit there is still `none` here and exit 9 at verify, which
      already fails any hit below its own `minValues` regardless of category.
      `internal/verify/secondnet.go` carries the matching fix for whatever
      this misses — a strong hit at *any* column size verify scans, not only
      below `minValues` as before — so a column this case does not reach is
      still not a silent leak, only a refusal instead of a mask.
    - **`strongHit` is gated by whether `free_text` itself is writable on
      this family, not by the hit's own category** (`bestSignal`, T-0136
      review round, finding 1). The first landing read only
      `silencedByType(p, hit.cat, family)` — the *hit's* category's own
      gate — but `decide` never assigns the hit's category to the column: it
      always assigns `free_text`, on the "not reliably that category, only
      mixed" reasoning two paragraphs up. So a proven `bigint`/`integer`/
      `numeric` column where a minority of samples pass `ValidLuhn` (roughly
      one in ten 12-to-19-digit runs does, by chance — snowflake IDs,
      epoch-millisecond timestamps, EAN-13 barcodes, order numbers) cleared
      the old gate, because `financial_account` accepts those families, and
      was decided `free_text`/`Masked=true` on a family `free_text`'s masker
      cannot write into (`rules.yml`'s `accepts:` for `free_text` is
      `text`/`varchar`/`bpchar`/`citext` only) — `mask.Writable` answers
      false and `internal/plan/writeback.go` refused the whole run at exit
      12, over a column that carried nothing worth refusing a run for. That
      is the T-0054 failure class `silencedByType` exists to prevent,
      reached by a branch that substitutes a category the gate never saw.
      The gate is now `silencedByType(p, pipeline.CatFreeText, family)` —
      the category `decide` is actually going to write — so a `bigint`
      column with a Luhn minority hit is left for `sig.weak`/`sig.refused`/
      `none` exactly as before T-0136, and `internal/verify`'s second net
      (the family-split Luhn entry in `internal/verify/validators.go`,
      T-0136 review finding 2) is what catches the value on the family this
      branch cannot reach — a refusal at exit 9 over an already-loaded
      target rather than a leak, which is the same "refusal instead of a
      mask" shape the paragraph above already accepts for the gap below
      `minSamples`.
- **`Decision.Domain`, `SmallDomain` and `Refused` are left at their zero
  values.** They are §5 quantities: `Domain` is `min(column domain,
  generator.Domain())` and no generator is registered in `mask` yet, and the
  refusal is made at plan against the planned row count, which the classifier
  does not have. `Decision.UniqueIndex` *is* filled here, because it comes from
  `Schema`. When the maskers land, the walk over categories and registered
  maskers that ADR-006 requires belongs in this package's tests.
- **The reason names the unique index by setting `UniqueIndex`, not in prose.**
  §10's example reason ends "unique index customer_email_key forces suffix",
  which is a statement about the generator the *plan* picked; this package sets
  the flag the plan reads.
- **A name-and-value disagreement is decided by the values, at `likely`.** Both
  readings are above the mask threshold, and the values are evidence about this
  column rather than about whoever named it.
- **The classification fingerprint is length-prefixed in decimal.** The claim is
  ARCHITECTURE.md §5's — that `b.c` in schema `a` must not hash like `c` in
  schema `a.b` — with no integer narrowing in the encoder.
- **`Refingerprint` is exported and `internal/core` calls it after the plan**
  (T-0101). §5 hashes each column's category and its **masker**, and the masker
  is not final when `Classify` returns: §5's unique-index rule has the *plan*
  pick the widest registered generator, and `internal/plan/unique.go` writes
  that pick back onto the decision. Computed here alone the fingerprint covered
  the *default* masker, so a column that became unique between two runs changed
  every masked value in it and changed no fingerprint, and §11.2's
  `classification changed` line did not print. The pick cannot move into this
  package — it needs the planned row count, which a pure classifier reading
  samples does not have — so the fingerprint moves instead. `Classify` and
  `Refingerprint` are one function over the decisions (`fingerprintOf`), so the
  two cannot drift, and with no escalation the value is byte-identical.
- **`internal/classify` imports `github.com/nyaruka/phonenumbers` directly.**
  ARCHITECTURE.md §13 lists it for "the `mask` module and classifier validator",
  and it is already required at the pinned version. The root `go.mod` still
  marks it `// indirect`, which is wrong now that a package in the root module
  imports it: `go mod tidy` moves the line into the direct block, and
  `.github/workflows/ci.yml`'s "go.mod is tidy" step fails until it is run.
  `make check` does not run tidy, so a green local check does not cover this.
  The classify task's paths did not include `go.mod`, so the one-line move is
  owed to a task whose paths do — nothing else about the dependency graph
  changes.
- **A key column is a surrogate key only below the mask threshold.**
  ARCHITECTURE.md §4 exempts "surrogate keys (`id bigint` and the FK columns
  that reference them)"; a natural key that carries personal data is not one.
  Exempting every integer or uuid key would copy `subscribers(msisdn bigint
  primary key)` and `people(nhs_number bigint)` verbatim under a green tick
  (THREAT_MODEL.md T1), and would make §4's own propagation sentence — a masked
  PK overrides the decision on every column referencing it — unreachable for the
  common integer and uuid case. The gate is the threshold rather than "has a
  category", so a key whose name hit was on a type its category refuses
  (`address_id integer`) keeps the exemption and its verbatim reason.
  Reversal condition: if a masked key ever costs more than it saves, the change
  is an ADR and a printed warning per key, not a quiet branch.
- **The key exemption is decided again after propagation; the generated-column
  exemption is not.** The gate above reads only the column's *own* signals, and
  an integer or uuid FK child usually has none: `visits.subject_ref bigint` with
  no name and no value hit is exempt when `markNeverMasked` runs, even where its
  parent `people.nhs_number` is masked. Leaving it there is the recall hole §4's
  propagation sentence exists to close — the parent's values ship in cleartext on
  the child side under exit 0 (T1) — and it breaks the load as well, because the
  parent is replaced and the child is not (T8). So `foreignKeys()` lifts the key
  exemption when the referenced column is masked, blanks the "preserved verbatim"
  fragment that would otherwise sit beside "propagated through foreign key", and
  converges the child on the parent's category rather than skipping whenever the
  child already scored higher: two categories across one edge are two maskers over
  one set of values. A generated column is never lifted — it is not copied at all,
  so there is nothing there to mask. The one case the sweep refuses is a parent
  category the child's type family does not accept, which would put a masker on a
  column that cannot hold what it returns; the child keeps its own decision and
  the conflict is named on its line.
- **A key child whose parent is copied takes the exemption back** (`keyChildren`,
  T-0120). The gate above reads the column's own signals in the other direction
  too: an integer or uuid FK child that reaches `possible` on a *name* hit alone
  loses the exemption while the primary key it references — the same values, no
  name hit — keeps it. `auth.identities.provider_id uuid` is the shape real auth
  schemas carry. Masking that end alone buys nothing, because a child's values
  are a subset of the parent key's and the parent shipped them verbatim, and it
  costs the edge: the load adds the constraint NOT VALID, `internal/verify/fk.go`
  counts the orphans and the run fails at exit 8 (T8). ARCHITECTURE.md §4's
  wording is the child's side of this — the exemption is for "surrogate keys
  (`id bigint` and the FK columns that reference them)", with no condition on
  what the child is called. So `keyChildren` runs **before** `foreignKeys()` and
  hands the exemption back, naming the parent column on the child's line and
  keeping the name signal's category there beside it; §4's direction still wins,
  because `propagateKeys` lifts and blanks what this granted through the same
  `keyFrag`, and a column with two parents, one copied and one masked, ends
  masked. Two gates keep the argument honest. The parent must be exempt by its
  own **key fragment**, not merely unmasked: a *generated* parent is `neverMask`
  for a different reason and is recomputed by the target from columns that may
  themselves be masked, so it is no evidence that the child's values are copied
  anywhere. And the edge must be **validated and real**: an unvalidated
  constraint is a hint over rows Postgres never checked and a virtual one is a
  line in the yml, so a child of either can hold a value the parent does not, and
  `internal/plan` does not follow it to fetch the parent row (`followsAsParent`).
  Propagation runs over every edge because it only ever masks more; this pass
  masks less, so it takes the narrow set.
- **FK propagation sweeps until nothing moves.** A key chain — a natural key, a
  child referencing it, a grandchild referencing the child — is one propagation
  per edge, and `Schema.FKs` is in no order relative to the chain, so a single
  pass leaves the grandchild verbatim whenever the edges arrive in the wrong one.
  The sweep is bounded by the number of edges, which also terminates the only
  shape that could oscillate: a cycle of masked keys that disagree about category.
- **A `lazyslice.yml` raise is applied only when it leaves a usable category.**
  `Decision.Masker` is chosen from the category, so a raise that leaves
  `CatNone` would say "mask this column" with nothing to mask it with, and a
  category the column's type family refuses would put (for instance) the JSON
  masker on a text column — the pairing `rules.yml`'s own `accepts:` list calls
  impossible. Both refusals are written into the column's reason
  (`yml_no_category`, `type_conflict`) rather than dropped, so §4's explanation
  line says the file did not take effect here. `pipeline.Pattern.Name` is
  documented as a "regex over column or table name", so a pattern is matched
  against the normalised table name as well. The gate is on the category the
  raise would *leave*, not only on the one the yml named: an entry that supplies
  a confidence and no category raises whatever the classifier already recorded,
  and that can be a name hit the type refuses (`people.email_verified` is `email`
  at `low` on a boolean, which §4 pins there and `raisable()` keeps every other
  pass off). Checking the incoming category alone would make `confidence:
  certain` with no category the one route to the email masker on a boolean.
- **An opt-out with no type fingerprint is treated as expired.**
  THREAT_MODEL.md T3's control is that an opt-out records the column's
  fingerprint and is ignored when it changes; one that records none is the
  fail-open version of that control, because nothing could ever revoke it. So is
  one with no reason, which ARCHITECTURE.md §2 says cannot happen. The `--unmask`
  flag's opt-out is made for one run and carries no fingerprint by construction,
  so the branch is on `Unmask.By` and not on the fingerprint being absent.
  `pipeline.Classification.Expired` still describes itself as "opt-outs ignored
  because TypeFP changed"; that comment is owed the same widening.
- **Every identifier in a reason is quoted when it leaves the bare class.**
  PostgreSQL identifiers are not `[A-Za-z0-9_$]+`: a schema may be called
  `my-app` and a table `user events`. Interpolating one verbatim produced a
  reason this package's own `ParseReason` rejected, which broke §10's "every
  `reason:` string parses against the template set" on an ordinary database
  rather than on a malformed one. `quoteIdent` writes anything outside the bare
  class as a Go-quoted string with semicolons escaped as well, so a name can
  neither end its own fragment nor forge the `"; "` that separates two.
- **A bytea column is never decided by the text validators.** A bytea sample is
  arbitrary binary and `asText` renders it as a Go string, so a PNG read as an
  address and a compressed blob read as a secret — decisions the rule pack
  contradicts, since `address` accepts no bytea column. §4 gives bytea its own
  rule; the name rules and `byteaInPersonShapedTable` are the only routes to a
  decision on one.
- **The columns of a unique expression index are read out of `Index.Def`.**
  §5 names expression indexes ("including expression indexes such as
  `lower(email)`") because they are the case that drives the generator choice,
  and introspect leaves `Index.Columns` empty for one. The parse over-reports
  rather than under-reports — a function whose name equals a column name marks
  that column unique — because the other direction is a load that fails on a
  unique violation. An expression index raises `Decision.UniqueIndex` only: it
  can never be the referenced side of a foreign key.
- **The type family is matched on the bare type name.** `Column.TypeName` is
  `pg_catalog.format_type` output, which qualifies any type not visible in the
  connection's `search_path`: `CREATE EXTENSION ... SCHEMA extensions`
  (§11.1 item 2) spells citext `extensions.citext`. Matching only the whole
  string made every such type `famOther`, which no category accepts, so an
  email column of a qualified citext was a false type conflict and a qualified
  hstore lost the §4 semi_structured type signal. Enums are resolved by both
  spellings for the same reason: `Schema.Enums` is keyed `nspname.typname` while
  format_type writes an in-search_path enum bare.
- **The per-column decision is two codes, not one.** `event.Args` carries no
  category and no confidence key (ARCHITECTURE.md §7), so the code is the only
  place the mask-or-copy verdict can land. `internal/invariants/i2_masking_test.go`
  sorts every column-scoped code into `flaggedCodePrefixes` or
  `copiedCodePrefixes` and calls `t.Fatalf` on one in neither;
  `classify.masked.column` and `classify.copied.column` are the spellings those
  lists already anticipate. `classify.column.drift` and
  `classify.column.opt_out_expired` are column-scoped too and are in neither
  list — they are not mask-or-copy verdicts, and they are unreachable without a
  committed yml, which I2's run does not have. If I2 ever runs with one, that
  list is what changes.
- **The JSON leaf walk stays here, and `internal/transform` will own a second
  one.** §12 assigns "JSON leaf walking" to transform, which needs a walker
  keyed by leaf name to pick a per-leaf masker; this one answers a narrower
  question — does the document carry personal data at all — and its only effect
  is `possible` to `likely`. That difference is not cosmetic: `likely` is what
  the neighbouring-column rule counts and what `byteaInPersonShapedTable` reads,
  so deleting the walk would lower recall in a table whose only strong signal is
  a jsonb document. It is recorded here because the duplication is real and the
  two walkers have to be kept in step.

- **The accepted-types gate runs over the value signals too** (`bestSignal`,
  `decide`, T-0054). ARCHITECTURE.md §4 says the gate is on the *name* signal
  only: "a column whose sampled **values** validate for a category is classified
  on those values whatever it is called". That sentence is now wrong about this
  package, deliberately, and §4 is owed the amendment — `ARCHITECTURE.md` line
  820, and `testdata/README.md` line 524 repeats it. This file is not where a
  recorded decision is reversed: root CLAUDE.md routes that through `docs/adr/`,
  so what is owed is an ADR superseding §4's sentence plus the two edits, from a
  task whose paths reach those files. Until then this note is the record of the
  drift, not the decision. What it cost: on pagila
  every `last_update timestamptz` renders as `2017-02-15T09:34:33Z`, which has
  no space, no `@`, two character classes and enough entropy, so `looksSecret`
  called it `credential` on 100% of its rows; `internal/transform` was then
  handed the fixed-literal masker for a timestamp, could not parse
  `$lazyslice$invalid` back into a time, and refused at exit 7 in the middle of
  every whole-pipeline run over pagila. A validator whose category the column's
  type family cannot hold now decides nothing: not `certain`, not `likely`, and
  not `low` either, because `low` is what the neighbouring-column rule raises and
  a raise would put the same unwritable masker on the column by a longer route.
  The hit is still *recorded* at `low` with `typeConflict` set and the reason
  names the conflict (`4/5 samples look like secrets; timestamp is not an
  accepted type for credential`), exactly as a type-conflicting name hit has
  been recorded since T-0033. The gate reads the rule pack's own `accepts:`
  list, not a hard-coded list of families, so `person_date` still decides a
  `date` column on its values and only the categories whose maskers emit text
  are shut out of a timestamp.
  - **It is narrower than `accepts:`, by three families** (`silencedByType`).
    `accepts:` answers which families a *name* hit may decide a column on, and
    §4 keeps that tight because a name is weak evidence. Silencing a *value*
    signal needs the stronger claim that the masking could not have been
    performed at all — CLAUDE.md's rule is "when in doubt, mask it" — so
    `enum`, `xml` and the catch-all `other` are outside the gate and are masked
    on their values as they were before T-0054. An enum is writable under every
    category (`mask.Writable` answers a labelled column before it looks at the
    family, and every generator begins with `labelValue`), and `xml`/`other`
    are not "a family that refuses the category" but "a type this package has
    no family for": an ltree, a PostGIS geometry, an extension type, a domain
    whose base `introspect` could not render. Nothing downstream is behind a
    silence on those — `internal/plan` does not refuse a type `mask` has no tag
    for, and `internal/verify`'s second net covers the character, uuid, inet,
    cidr and macaddr families only — so a silence there would be a cleartext
    copy under a green tick (THREAT_MODEL.md T1).
  - It is not the enum exemption T-0033 removed, and it is not masking less
    where masking was possible: a column it silences is one whose family this
    package recognises and whose maskers for that category emit a value the
    family cannot hold. `TestPagilaValueSignalsRespectAcceptedTypes` holds the
    gate, and its `conflicts == 0` guard fails if the samples ever stop
    tripping the validator, so the test cannot quietly stop testing it;
    `TestValueSignalSurvivesATypeNothingCanJudge` holds the three families that
    are outside it.
- **A `tsvector` is `derived_text`, decided by its type alone** (`decide`,
  `rules.yml`, T-0054). It has no name pattern and cannot be reached by one. A
  tsvector holds the lexemes of the text it was built from — `film.fulltext` is
  maintained by a trigger over `title` and `description` — so copying one ships
  the words of a column that may itself be masked, in cleartext, beside it
  (THREAT_MODEL.md T12); and there is no fake worth generating, because a
  tsvector of invented lexemes is a search index that matches nothing, which is
  what the empty one already honestly is. So: `certain`, always, with the reason
  `tsvector is derived from text that may be masked`, and `mask`'s
  `derived_text` generator empties it. The validators do not run over a tsvector
  at all, for the reason they do not run over a `bytea`: its text form reads as
  an address, and no answer they could give would change the decision.
- **The rule pack's `accepts:` lists are checked against `mask`, not merged with
  them** (`TestRulePackAgreesWithMaskAboutTypes`, T-0054). `mask` declares, per
  category, the type tags its generators can be written into
  (`mask/writable.go`), and `internal/plan`'s write-back check reads that rather
  than this file. The two declarations exist for different readers — this one
  gates a *decision* on its way in, that one gates the *plan* over the category
  a decision ended up with — and a test walks them against each other so neither
  can drift. If they disagreed, one gate would be answering a question about a
  column the other had already let through, which is the whole of T-0054.
- **The type-family table is still a second copy** (`types.go`). `mask.TypeTag`
  is the shared home T-0054 gave `internal/transform` and `internal/plan`; this
  package keeps its own because it maps a type onto more than a family (it
  carries `famEnum`, `famXML` and `famOther`, which `mask` has no tag for and
  `silencedByType` reads) and because it is the one that decides what a
  validator even runs on. The two are read side by side through the rule pack
  and through the test above; a third copy is what the shared home exists to
  prevent.
  - **The quoting is not copied any more.** `stripTypmod`, `unquoteType` and
    `bareTypeName` were a fourth copy of three string functions whose only job
    is to produce `mask.TypeTag`'s argument; they are now `mask.StripTypmod`,
    `mask.UnquoteType` and `mask.BareTypeName`, and this package, `internal/plan`
    and `internal/transform` all call them. What is still written three times is
    the *domain* resolution (`domainBase`), which needs `pipeline.Schema` and so
    cannot live in `mask`; the one home for it is `internal/pipeline`, whose
    file T-0054's paths did not include.

- **The validators and the name dictionary moved to `internal/textsig`**
  (T-0055). `names.txt` and `dict.go` were here, and `internal/verify`'s second
  net — which re-runs §4's value signals over the loaded target — could not
  import them, because a stage package may not import another stage package
  (internal/CLAUDE.md). So verify carried a hand copy of eight of the ten
  validators and was missing exactly the two that read the dictionary,
  `person_name` and `free_text`, which is the largest personal-data category and
  half of one of THREAT_MODEL.md T1's two v1-blocking controls. What moved is
  the value-only half — a `func(string) bool` and a word list. What did **not**
  move, and must not: the rule pack, the categories, the confidences, the
  thresholds (`validatorThreshold`, `weakThreshold`, `minSamples`) and the
  scoring, all of which are still this package's. The dictionary is scored
  differently on the two sides on purpose — `textsig.Dict.LooksLikeName` and
  `Dict.Prose` here, `Dict.NameShape` and `Dict.ProseName` in verify, because
  neither a single dictionary word (black, brown, hill, green, wood) nor a pair
  of them (green lane, hunter green) nor an English sentence carrying one ("the
  supplier may terminate...") may fail a loaded target at exit 9. Verify's two
  ask for a given name immediately followed by a surname; this package's two do
  not, and must not, because a `first_name` column holds one word per row.
  Nothing about this package's decisions changed: `names.txt` is byte for byte
  the file that was here, `Classification.Fingerprint` never covered it (it
  covers the rule pack's `version`), and the precision and recall floors below
  are unmoved. The dictionary's `Given` and `Surname` sets were deleted in
  T-0055 as unused and came back unexported in its review, because verify's two
  validators ask which section a word came from; nothing here reads them. The
  two sections of `names.txt` are still parsed, so a line outside a section is
  still ignored.
- **A table-scoped pattern is a second, merged pattern list, not a second match
  call site** (`rulepack.go`, T-0119). `matchColumn(table, column)` is what
  `decide` and `decideComposite` call now, and it walks `ColumnPatterns` — a
  copy of the ordinary `patterns:` list with every `table_patterns:` rule
  merged in and the whole thing resorted by the one priority line the two
  share, so a table-scoped rule and a name rule matching the same column at the
  same priority tie-break identically (priority, then rule name) whichever list
  either came from. `match(column)`, the plain lookup, is untouched and still
  reads `Patterns` alone: `jsonLeafIsPersonal` (`validators.go`) is its only
  other caller, over a JSON document's leaf keys, and a leaf has no table to
  test a `table:` regexp against — merging table-scoped rules into `Patterns`
  itself would have let `refresh_token_parent` fire on any JSON key named
  `parent` anywhere, table or not, which is a different and wider claim than
  the one this rule makes. A `table_patterns:` row without a matching entry in
  `categories:`, or with either regexp malformed, fails `loadPack` the same way
  a `patterns:` row does; its `name` is checked against the reason grammar the
  same way too, since it renders through the same `name_match` fragment. An
  *empty* `table:` or `match:` fails `loadPack` too, and by a check of its own
  (review round, T-0119): `regexp.Compile("")` succeeds and matches every
  string, so without it an omitted `table:` would silently compile into a rule
  that applies to every table — a table-scoped rule that is not actually
  scoped — and an omitted `match:` would mask every column of a matching table
  to the rule's fixed literal. Neither is "malformed" in the syntax-error sense
  the sentence above is about; both are rejected by name before
  `regexp.Compile` ever sees them.

## A composite is refused, not copied and not masked (T-0094, T-HARD-B)

`types.go` gives a composite its own family (`famComposite`, resolved against
`Schema.Composites`, which is the catalog's own list of `typtype` 'c' with
`relkind` 'c' and so excludes a view's row type), and `decideComposite` is the
whole of its decision. Before it, a composite was `famOther`: a name hit on it
was recorded at `low` and the column was **copied verbatim** with whatever was
in the record, and a value hit could decide a category whose masker
`internal/transform` then tried to write into a record, dying mid-load. Both
are THREAT_MODEL.md T1.

The decision has two outcomes and no third:

- **A hit is `possible`.** The name rules run as they do everywhere, and
  `compositeSignal` runs the validators over **the whole sample and over the
  record's fields**, taking a hit from either — `splitCompositeLiteral` in
  `literal.go` reads the server's output form, since the source pool registers
  no user types and a record arrives as the string `(1234.50,GBP)`. Any field of
  any sample that validates is a hit, which is `jsonSignal`'s shape and not the
  scalar ratio: a `(street, city, email)` record is one third addresses and one
  third email, both under the weak threshold, so ratio scoring would decide
  `none` and copy the address. **Scoring the fields alone was a fail-open of its
  own** and is the case `TestCompositeAddressAcrossFieldsFailsClosed` holds: a
  record can carry personal data that exists only as the concatenation of its
  fields, and `(9,"Rue de Rivoli",Paris)` is an address no field of which is one,
  because `AddressShape` wants a digit and two words in a single value and the
  house number lives in a field by itself.
  `possible` is above §4's mask threshold, and `internal/plan`'s write-back
  check turns it into exit 12 naming the column, `--skip-table` and a reasoned
  `--unmask` (`internal/plan/writeback.go`). **No masker can write a record**,
  so a refusal is the only fail-closed answer available; masking a composite
  field-wise would need a masker per field and a way to re-render the literal,
  which is a feature and not a fix.
- **No hit is a copy that says so.** The reason carries `composite type: its
  fields were read and none is personal data`, so a green run over a composite
  is a claim somebody can read rather than a silence. `testdata/nasty.sql` trap
  27's `money_amount` is this case and stays copied. **A copy reached with
  nothing to read says that instead** (`composite_no_sample`): the fragment above
  claims a check that ran, and on a composite in a table nothing could be
  sampled from — seven of supabase-auth's ten misses are columns with no rows at
  all — appending it beside `no samples` produced a line that contradicted
  itself on precisely the branch that leaks.

The category on a hit is the first validator in precedence order that matched.
It is what the refusal names and nothing masks with it.

## An array whose sample arrives as one string (T-0103, T-HARD-B)

`scalarsOf` splits a Postgres array literal when the column's type says array
and the sample is a string. `scalars` flattens an array only when the driver
handed back a slice, and pgx does that only for an array type its map knows:
the source pool runs in `QueryExecModeExec` and registers no user types
(T-0076), so a `citext[]` of addresses arrives as `{a@b.test,c@d.test}`, no
validator matched it, and the column was decided `none` and copied. Plausible's
`monthly_reports.recipients` is that column.

It is a fallback and never a replacement: a literal the splitter cannot read
falls through to `scalars`, which treats it as one opaque value — the behaviour
before it existed, and never worse than it. `rawSamples` is the *unsplit*
flattening, and the two callers that use it (`tableWasSampled`,
`sampleDistinct`) want it: a unique index is over the whole array, not over the
strings inside it.

**The transform half has landed (T-0118), and so has verify's (T-0129); plan no
longer refuses in front of them (T-0127).** `internal/transform`'s `maskArray`
used to fire only on a `[]any`, so such a column was masked as one scalar
string and `CopyFrom` failed with "cannot find encode plan" at exit 7 mid-load,
with the tables before it already committed — which is why
`internal/plan/writeback.go` (`arrayArrivesAsLiteral`) refused such a column at
exit 12 instead, with `--skip-table` and `--unmask`, asking the samples rather
than the type because the samples are the only place the driver's answer is
recorded. T-0118 taught `internal/transform` to parse this same literal
grammar (`internal/transform/array.go`) and mask it element-wise, writing back
a literal `array_in` accepts for the same element count and dimensions; T-0129
taught `internal/verify` to split the target's literal the same way for the
residual scan (`internal/verify/arrayliteral.go`), so a hit inside the braces
is not missed either. With both halves in place, T-0127 removed the
`arrayArrivesAsLiteral` refusal: the column this section used to describe as
refused at plan time is now planned, masked and verified like any other array.

## A URL is online_id and never credential (T-0100, T-HARD-B)

`textsig.ValidURL` is in the ordered validator list **ahead of** the secrets
one, under `CatOnlineID`, and `textsig.LooksSecret` now excludes a URL. The two
halves ship together on purpose: `LooksSecret` matched any 16-to-512-character
value with two character classes, entropy at or above 3.2, no space and no `@`,
so mastodon's `accounts.uri` was `credential` on every row — masked to the fixed
literal, which is safe and wrong, and a plan refusal under the unique index that
column carries. Excluding it from `LooksSecret` alone would drop a
username-bearing URL to `none` and copy it, which is T1's direction; `online_id`
catches it, and its generator's domain is wide enough for a unique column.

Order is precedence in that list, so the URL rule sits after the email, IBAN,
Luhn, phone, IP and MAC validators and before the secrets one: nothing that is
already a stronger shape becomes a URL.

**Only this package got the replacement — T-0122.** `internal/verify`'s second
net imports the same `textsig` and reads `LooksSecret`, and it has no
`online_id` or URL validator at all, so the narrowing took coverage away from a
THREAT_MODEL.md T1 blocking control that this package's new validator does not
give back: the two nets score independently. `internal/verify` was outside
T-HARD-B's paths; T-0122 carries the entry and the "all ten of
internal/classify's value validators" sentence there, which is eleven now.

## Measured

`TestPagilaPrecisionAndRecall` and `TestFiftyNamesFromThreeSchemas` print a
confusion matrix and hold the rates to a floor. Recall's floor is the strict
one — a false negative is cleartext in the target under a green tick
(THREAT_MODEL.md T1) — and precision's is deliberately loose, so that raising it
by deleting a name pattern fails the test that matters first. Run
`go test -v -run 'TestPagila|TestFiftyNames' ./internal/classify/` to read them.

`TestFiftyNamesFromThreeSchemas` keeps its name and is now 64 names from four
schemas: T-0104 added Supabase's auth columns and GitLab's `identities.extern_uid`
beside the original fifty, because every spelling the credential and online_id
rules gained has to be scored in the same matrix as everything else. It reads
precision 0.959 / recall 1.000 (T-0119; it was 0.958 / 0.979 after T-0121 with
`refresh_tokens.parent` still a false negative, and 0.957 / 0.978 before that
with both `public_key` columns copied and labelled not-personal).

**No hand label moves in the change that widens the rules scored against it.**
Relabelling a column turns a false positive into a true positive without the
classifier doing anything, so it is not the rule author's call to make in the
same commit — and this fixture's whole value is that the rates are over a set
nobody trimmed. Exactly one label has ever moved: both `public_key` columns,
from not-personal to personal, under **T-0121**'s own decision that a
`public_key` column is a credential, taken by the orchestrator and applied in a
task that wrote none of the T-0104 patterns it is scored against. The movement
it caused is the two lines above: 44 → 46 true positives, 17 → 15 true
negatives, and the false positives and the one false negative unchanged.
**T-0119 moved no label either** — `supabase.refresh_tokens.parent` was already
labelled personal, and only the classifier's decision moved, false negative to
true positive: 46 → 47 true positives, the one false negative gone, recall
0.979 → 1.000.

One rule was **not** widened, and the refusal is load-bearing:

- **A bare `codes?` is not in the credential pattern.** At priority 80 it would
  take `postal_code`, `country_code`, `currency_code` and `status_code` away
  from the address rule and mask them to the fixed literal. What is in the
  pattern is the compound spellings: `auth_code`, `otp_code`,
  `authorization_code`, `code_verifier`, `code_challenge`, `code_hash`.

**`parent` used to be the second of these, and it is now a table-scoped rule
and not a name rule** (`refresh_token_parent`, T-0119). `refresh_tokens.parent`
holds another refresh token, and a rule matching `parents?` would mask every
`parent_id` join key in every schema there is — this package's name rules see
the column name alone, so "parent, in a table called refresh_tokens" was not
expressible as one of `patterns:`. `rules.yml` gained `table_patterns:` for
exactly this shape: a name rule with a second regexp, over the *table*, that
gates whether the rule is tried at all — `refresh_token_parent` is `credential`
at priority 80 (the same line `patterns:`'s own `credential` rule sorts on),
scoped to `table: '(^|_)refresh_?tokens?(_|$)'`, `match: '^parents?$'`. It is
compiled into `compiledPack.ColumnPatterns`, a copy of `Patterns` with the
table-scoped rules merged in and resorted by the one priority line the two
share; `compiledPack.Patterns` itself, and `match`, are unchanged, because the
JSON-leaf path that calls `match` (`jsonLeafIsPersonal`) has no table to test a
table-scoped rule against. `decide` and `decideComposite` call the new
`matchColumn(table, column)` instead. `supabase_misses_test.go` and
`names_test.go` pin the column as masked now.

**`public_?keys?` was the third of the three T-0104 left open, and it is now in
the credential pattern** (T-0121, settled 2026-09-09). It is the other of the two columns
T-0104 said "deserve a decision rather than a pattern". A public key is
published by design, which is the argument for leaving it alone or for calling
it an `online_id`; what decided it is that a key identifying exactly one person
is a per-person identifier that outlives every other value in the row, and
nothing a development database does verifies a WebAuthn assertion or serves an
actor document, so the real value buys nothing there. Under a unique index it
escalates to `credential_unique` rather than refusing, which is what removed the
practical objection to a masker whose domain is one. The hand labels moved with
it, in the same task and by an author who wrote none of the rules they score.

**The IdP names are their own rule, and it is anchored** (`online_id_idp`,
T-0104). `external_id`, `provider_id` and `extern_uid` are the identifier an
identity provider issues for a person, so they are `online_id`; but every other
name rule in `rules.yml` is a *word* rule, `(^|_)word(_|$)`, and under that
spelling `provider_?ids?` also takes `sso_provider_id`, `oauth_provider_id` and
`identity_provider_id`, which are join keys. `testdata/torture/supabase-auth` has
five uuid `sso_provider_id` foreign keys, `online_id` accepts uuid (for
`device_id` and `cookie_id`), so a compound spelling reaches `possible` on the
name alone and the report calls a join key an `online_id`. Anchoring keeps the
three names T-0104 asked for and leaves the compound ones `none`.

Anchoring was a narrowing, not the orphan fix; the fix is `keyChildren` above
(**T-0120**). What the anchor is still for is the *category on the
line*: `sso_provider_id` is not an identifier an IdP issued for a person whether
or not it ends up masked, and a compound spelling on a column carrying no
foreign key at all is the case no reconciliation can reach.

## What `Decision.UniqueIndex` means (T-TORTURE)

`internal/plan/unique.go` holds every column this field raises to
ARCHITECTURE.md §5's `d_required = n²/2ε`, so what the field means is a claim
this package makes and the planner acts on. §5 states the rule for *a column*
and says nothing about composite or partial indexes, so two of the three answers
are approximations and both are argued at their site:

- `indexKeys` raises a single-column primary key, a single-column non-partial
  unique index, and a single-column *expression* unique index (§5 names
  `lower(email)`). It also raises a single-column **partial** one, where
  `internal/transform`'s `uniqueColumn` does not: Supabase's
  `confirmation_token_idx` is partial, every masked row falls inside its
  predicate, and the index would not build
  (`testdata/regressions/007-partial-unique-index-masked-column.sql`).
- `raiseCompositeUnique` runs after the threshold, because which columns are
  masked is part of its answer, and decides by which key column's sample has no
  repeats. Both directions were measured on real schemas: raising all of them
  refused runs that could not collide (regression 003), raising none of them
  loaded rows that did (regression 004). It judges **partial and expression**
  composites too: excluding them raised the field on no column of such an index
  at all, which was strictly less than the code it replaced did, and the ten
  torture schemas carry 85 composite partial unique indexes, several over a
  masked column.
- The unmasked key column that suppresses the raise has to have been
  **sampled**. A column with no scalar samples is one of two things and they
  mean opposite things: NULL in every sampled row, where the tuple cannot
  collide at all under a plain unique index (calcom's `Role_name_teamId_key`,
  `teamId` NULL in all three rows); or in a table nothing could be sampled from,
  where nothing is known and counting it distinct is a fail-open. They are told
  apart by whether any column of the table produced a sample, and the first is
  withdrawn for an index declared `NULLS NOT DISTINCT`.

Both err towards raising. Raising wrongly costs a plan refusal that prints three
escapes; not raising costs a loader that dies with every row already moved.

**These are behaviour rules, and they are decided.** `Decision.UniqueIndex`
feeds `internal/plan`'s exit-12 refusal, `internal/transform`'s
`Constraints.Unique` and the emitted `lazyslice.yml`, so what is written above
changes what every user's run does — and root CLAUDE.md says a decision lives in
`docs/adr/`. They shipped ahead of that: T-TORTURE's paths reached neither
`docs/adr/` nor ARCHITECTURE.md, and T-0099 was the record in the meantime.
**`docs/adr/011-unique-index-domain-rule.md` is the decision now** — accepted
2026-09-08, clause (a) the composite rule and clause (b) the partial one, with
T-0099's text as its Decision and ARCHITECTURE.md §5 amended to state both
(T-0099 and T-0107 are closed). The paragraphs above describe the code; **the
ADR is the specification**, and a change to either of these rules is a
superseding ADR and not an edit here. ADR-011's reversal condition is the one
open thread: when introspect collects the largest-agreeing-group statistic,
clause (a) becomes exact.

## A rejected name hit never leaves a column worse off than no name (T-TORTURE)

`decide`'s `hasName && !nameAccepted` branch used to record `low` and stop, and
`low` is below the mask threshold — so a `jsonb` column whose *name* matched a
rule that does not accept `jsonb` was copied verbatim, while the same column with
no name at all was `semi_structured` and masked. Supabase's
`auth.users.raw_user_meta_data` is the case: the identity provider's profile,
names and addresses and phone numbers, in the target in cleartext under exit 0
(THREAT_MODEL.md T1;
`testdata/regressions/008-name-hit-on-an-unaccepted-type-drops-the-type-signal.sql`).
That branch now falls back to `typeSignals[family]` when the samples say nothing.

The rule to keep in mind when touching `decide`: **every branch that rejects a
signal has to be at least as safe as the branch with no signal at all.** The
`sig.refused` branch below is the same shape with a value signal instead of a
name, and it has not been measured against a real schema; if one turns up, it
gets the same treatment.
