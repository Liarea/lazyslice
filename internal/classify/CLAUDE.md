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
  accepted type families, the name patterns with their priorities, and the
  log-shaped table rule. Changing it changes `Classification.Fingerprint`.
- `names.txt` — the English name dictionary, in a `# given` and a `# surname`
  section.
- `rulepack.go`, `dict.go` — loading and compiling those two.
- `types.go` — `pg_catalog.format_type` output to a type family, resolving
  domains and enums.
- `validators.go` — the §4 validators and the sample-to-string conversion.
- `reasons.go` — the reason fragment set and `ParseReason`.
- `classify.go` — the six passes: base signals, bytea in a person-shaped table,
  the neighbouring-column rule, FK propagation and shared names, the yml prior,
  then the threshold.
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
- **Three or more non-NULL samples before a value signal counts.** One row that
  parses as an address is evidence about a row, not a column.
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

## Measured

`TestPagilaPrecisionAndRecall` and `TestFiftyNamesFromThreeSchemas` print a
confusion matrix and hold the rates to a floor. Recall's floor is the strict
one — a false negative is cleartext in the target under a green tick
(THREAT_MODEL.md T1) — and precision's is deliberately loose, so that raising it
by deleting a name pattern fails the test that matters first. Run
`go test -v -run 'TestPagila|TestFiftyNames' ./internal/classify/` to read them.
