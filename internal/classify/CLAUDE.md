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
  table it is in), the `unless` and `corroborated_by` fields a name rule may
  carry (T-0313, "A bare name needs corroboration" below), and the log-shaped
  table rule. Changing it changes `Classification.Fingerprint`.
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
  FK propagation and shared names), the yml prior, FK propagation once more
  when the prior raised anything (T-0364), then the threshold.
- `spare.go` — what spares a signal-less column from the neighbouring-column
  sweep (T-0311): the enumeration thresholds, the identifier shapes it reads
  from `internal/textsig`, and the dictionary, special-category and gender
  guard every spare gives way to. See the T-0311 section below.
- `generated.go` — the document keys a generated column's expression reads,
  raised to its category when its samples validate (T-0397; the section on
  the per-leaf half of a JSON decision, below).
- `codes.go` — the five `event.Code`s the classify stage renders; each has a row
  in `internal/event/catalogue.yml`.
- `framework.go` — no file of that name here; see the T-0314 section below for
  why the table-name list itself lives in `internal/pipeline`.

## Framework metadata tables are never masked (T-0314)

Dogfood session 1 found `schema_migrations.version` masked — its digit
strings pass the Luhn check — and planned `SchemaOnly` besides, because
nothing in `internal/plan`'s ordinary lookup rule reaches a table with no
incoming foreign key at all, which is the *ordinary* shape of migration
bookkeeping. `ar_internal_metadata` went the other way: copied verbatim,
`environment` row and all, which makes a fresh Rails checkout refuse a
destructive rake task against its own clone.

`internal/pipeline.IsFrameworkMetadataTable` (that package's own CLAUDE.md
has the full list and the reasoning for why it lives there rather than here
or in `internal/plan`: three stage packages need the identical answer and
none may import another) is checked first in `markNeverMasked`, ahead of the
generated-column and surrogate-key checks it already made: it is a property
of the *table*, not of any one column's signals, and it must win over
whatever either of those checks would otherwise have said about the same
column. It sets `neverMask` exactly as those two do — the column is still
classified (base's signal passes still run and the reason line still
records whatever they found, the same way a generated or surrogate-key
column's own signal is recorded before the exemption fragment is appended)
— but it can never be *masked*, whatever category or confidence the signals
land on. `TestFrameworkMetadataTableNeverMasked` proves the override wins
against a column shaped to mask under the ordinary rules (a national-id-
shaped name and checksum-valid values).

**`work.frameworkMetadata` is a second, separate flag from `neverMask`
itself, and — since the T-0314 review round — it is the one `neverMask` two
passes are allowed to lift.** `pipeline.IsFrameworkMetadataTable` matches a
bare table name in any schema, so "bookkeeping the tool itself wrote and
reads back, never end-user data" is a premise about the *usual* case, not a
guarantee: a same-named application table is exactly the shape the two
lifts below are for, and a real migration table loses nothing to either,
because it never carries a foreign key at all (the ordinary shape dogfood
session 1 found) and nothing in a real one's own values is worth a
lazyslice.yml entry.

- `propagateKeys` treats it exactly like the ordinary surrogate/FK key
  exemption: cleared when a validated FK parent turns out to be masked, the
  values converged on the parent's category, and the "framework metadata
  table" reason fragment blanked (`frameworkMetadataFrag`, the same
  bookkeeping `keyFrag` does). Before this round, `cw.generated ||
  cw.frameworkMetadata` stopped the clear dead, and the test built to prove
  it — a contrived `schema_migrations.version` FK-child of a masked
  `people.ssn` — asserted the child stayed **unmasked**, which is the
  THREAT_MODEL.md T1 recall hole and T8 join break the propagation sentence
  exists to close, turned into required behaviour by the test. Only
  `cw.generated` keeps the unconditional carve-out now (a generated column
  is not copied at all, so there is nothing to mask); the renamed
  `TestFrameworkMetadataTableFKChildIsMaskedWhenParentMasks` asserts the
  child **masks** and converges on the parent's category, and reverting the
  narrower `cw.generated`-only check fails it. Nothing in `testdata/`
  carries this shape for real — no real `schema_migrations` or
  `ar_internal_metadata` has a foreign key at all — so the test still builds
  it by hand.
- `applyPrior` lifts it too, from `raiseFromConfig`'s own success branch
  only, on an explicit `lazyslice.yml` pattern or column entry naming the
  column: ADR-004 lets a committed file only tighten, and an operator's own
  "mask this" for a column real values are copied into must not read back
  unmasked. `liftFrameworkMetadataExemption` is a no-op on every other
  `neverMask` reason (generated, the ordinary key exemption), and it only
  runs once `raiseFromConfig` has confirmed the raise leaves a usable
  category, so a refused raise (`yml_no_category`) never clears the
  exemption for nothing.
  `TestFrameworkMetadataTableIsMaskedByExplicitYmlPattern` and
  `TestFrameworkMetadataTableUnaffectedByARefusedYmlRaise` pin the two
  directions.

**The match is case-insensitive and on the bare table name alone**
(`pipeline.IsFrameworkMetadataTable`'s own doc comment has the schema half).
`TestFrameworkMetadataTableIsCaseInsensitive` pins the one entry on the list
whose real catalogue spelling is mixed case, EF Core's
`__EFMigrationsHistory`.

**What this does not do.** It does not change what the column's signals
decide — `Decision.Category` and `Decision.Confidence` are whatever `decide`
found, unmasked reasons and all — only whether the column is masked, and (since
the review round below) only for a column that is also on the table's own
bookkeeping allowlist. The two lifts above are the escape for the case where
even an allowlisted column's premise is wrong about one particular database (a
same-named application table, an operator's own `lazyslice.yml` entry), not a
widening of either list.

**The exemption is narrower than the table match: only a column on
`pipeline.IsFrameworkMetadataColumn`'s own per-table allowlist is exempt (the
T-0314 review round's second finding).** The first landing exempted every
column of a recognised table by name alone, on the premise that all of it is
"bookkeeping the tool itself wrote and reads back, never end-user data" — true
of a migration timestamp or a checksum, and not true of every column a real
instance of one of these tools actually ships: Liquibase's `DATABASECHANGELOG`
carries `AUTHOR` (the developer who ran the changeset) and Flyway's
`flyway_schema_history` carries `INSTALLED_BY` (the database role or OS user
that applied it), and either can hold a real name, a real username or an email
address. `pipeline.frameworkMetadataColumns` (`internal/pipeline/framework.go`,
that package's own CLAUDE.md has the reasoning for why the map lives there) is
the well-known column list each tool's own migration schema ships — a version
string, a checksum, a timestamp, a boolean flag — and `markNeverMasked` now
checks `pipeline.IsFrameworkMetadataColumn(t.Ref.Name, col.Name)` alongside the
table match before it sets `neverMask`. A column that clears the table check
but not the column one falls straight through to the generated-column and
surrogate-key checks, and from there to the ordinary passes below, exactly as
if its table were never on the list at all — an `AUTHOR` column of email
addresses masks as `email` like any other. `internal/plan` still forces the
whole table to a `Lookup` step regardless of what any column here decides, so
the row count and reachability guarantee T-0314 exists for is unaffected;
`TestFrameworkMetadataTableBookkeepingColumnStaysExempt` and
`TestFrameworkMetadataTableColumnNotOnAllowlistIsMasked`
(`framework_test.go`) pin both directions, and
`internal/pipeline`'s own `TestIsFrameworkMetadataColumn` pins the map.

**Owed: `internal/verify`'s second net does not know about this exemption at
all, beyond the narrow dense-sequence case (T-0348).** `Decision.NeverMasked`
only gates `secondnet.go`'s `sequenceExempt` branch, which the two
`requiresCorroboration`-gated national-id-digits validator entries use; every
other entry there — including the Luhn/`financial_account` one dogfood
session 1 actually hit — scans an unmasked column unconditionally, whatever
`NeverMasked` says. `testdata/regressions/042`'s own header has the
measurement: a framework table whose real values happen to validate strongly
(a Luhn-valid migration timestamp, the exact dogfood shape) refuses the whole
run at exit 9 even though this package correctly leaves it unmasked. Out of
this package's reach — `internal/verify` is a different stage package — and
filed rather than fixed here.

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
      column with a Luhn minority hit is left for `sig.weak`/`none` exactly
      as before T-0136 (T-0269 later removed the `sig.refused` field this
      sentence originally named; see the T-0054/T-0269 bullet above), and
      `internal/verify`'s second net
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
  The gate reads the rule pack's own `accepts:` list, not a hard-coded list of
  families, so `person_date` still decides a `date` column on its values and
  only the categories whose maskers emit text are shut out of a timestamp.
  - **T-0269 corrects the sentence above this bullet's original wording.**
    From T-0054 until T-0269 the hit was still *scored* and *recorded* at
    `low` with `typeConflict` set, and the reason named the conflict
    (`4/5 samples look like secrets; timestamp is not an accepted type for
    credential`) exactly as a type-conflicting name hit is. That line reads as
    an alarm over a column that was never going to be masked either way — the
    decision (`low`, unmasked) was already right, only the reason was noise —
    so `bestSignal` no longer keeps the refused hit's category, phrase or
    sample count for anything a reason could name: the column falls through to
    whatever else has a signal, or to the same `no name or value signal` every
    signal-free column gets. `TestTimestampCredentialEntropyIsNeverScored`
    (`classify_test.go`) and `TestPagilaValueSignalsRespectAcceptedTypes`
    (`pagila_test.go`) pin this.
    - **T-0269's first landing dropped the whole result, not only the noisy
      half of it, and a review round on that same task found the difference
      is not cosmetic.** `signals.refused` used to do two things at once: name
      the conflict in the reason (the alarm this fix is about), and set
      `w.typeConflict = true` so `sameColumnName`, `guessedPhoneColumns` and
      `fkPairs` — three raising passes that key on nothing but a column's own
      name, region guess or FK partner — stayed off a column no category was
      ever going to be decided under. Dropping the field entirely kept the
      first half fixed and reopened the second: a column whose only signal was
      a silenced hit at or above `validatorThreshold` was indistinguishable,
      to every later pass, from a column the validators never looked at, so a
      same-named column elsewhere in the schema could raise it to `possible`
      on evidence that was never about it. `signals.silencedStrong` (a bare
      `bool`, set only when a silenced validator reaches `validatorThreshold`)
      and `decide`'s own `case sig.silencedStrong` are the fix: `w.typeConflict
      = true` and nothing else — no category, no phrase, no count, and the
      case sits before `default` in the switch so a family with its own
      `typeSignals` entry (inet, cidr, macaddr) is not masked on that entry
      either, the same pre-emption `signals.refused` gave it before T-0269.
      `TestSameColumnNameDoesNotRaiseASilencedTypeConflict` (`classify_test.go`)
      pins the reproduction: `births.birth timestamp` is a genuine
      `person_date` name hit, `a.stamp timestamp` is its FK child and is
      propagated to `person_date`, and `b.stamp timestamp` — no FK to
      `births`, no name signal, only the same high-entropy timestamp values
      `TestTimestampCredentialEntropyIsNeverScored` uses — must stay unmasked
      because `a.stamp` and `b.stamp` share a column name and nothing else.
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
- **`decodeSampleDocument` decodes a `[]byte` or `string` sample with
  `encoding/json`'s `Decoder` and `UseNumber`, not plain `Unmarshal`**
  (`validators.go`, T-0402, the 2026-09-25 JSON red team's A11,
  `docs/reviews/2026-09-25-redteam-json/round1.json` entry 14). A json or
  jsonb column now reaches this package as the source's own raw text
  (`internal/pg`'s `jsonTextRows`), the same text a domain over jsonb has
  always arrived as, so this is the one place both are decoded — read by
  `jsonKeyCategories` (the per-leaf key map, T-0272) and by
  `guessedPhoneLeafKeys` (the value half, T-0394). Plain `Unmarshal` decodes a
  number into a `float64`, which rounds an integer past 2^53; nothing this
  function's own two callers read a number leaf's value for, so the recall
  hole T-0402 fixes was never here directly — it was `internal/transform`'s
  own copy of this function, handed the same already-decoded, already-rounded
  value pgx used to produce. This one is fixed anyway, for the same reason
  `internal/transform`'s and `internal/verify`'s copies are: a number leaf
  keeps every digit wherever this package decodes one, not only where a bug
  report happened to find the loss.
  - **`jsonSignal` (`classify.go`) has the matching gap and is not fixed
    here.** It asks `scalars()`/`asText()` to render each sample into a
    string before `jsonLeaves` (a second, undeduplicated copy of this
    decoding, `validators.go`) ever runs, and `asText` renders neither a
    `map[string]any` nor a `[]any` — so a real object- or array-shaped
    document, sampled through the ordinary path, contributes nothing to the
    column's *value* signal at all; only a document whose sample happens to
    already be a string (a domain over jsonb, or a test fixture written that
    way) reaches `jsonLeaves`. Measured directly against a real
    `postgres:16`: `bestSignal`'s json branch never sees an object document's
    leaves. It costs nothing here that T-0402 needs — every jsonb column
    still gets its baseline type-signal `possible` confidence regardless of
    what `jsonSignal` finds, so the column is still walked leaf by leaf, and
    the per-leaf recall T-0402 fixes is `internal/transform`'s
    `leafValueCategory`, not this — but it is a real, separate recall gap
    (`jsonSignal` can never raise a real object document's confidence from
    `possible` to `likely` on its own values, only on its keys through
    `jsonKeyCategories`), reported rather than folded into this task, which
    named only `decodeSampleDocument`. Filed as **T-0415**.

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

## A composite holding a document field fails closed on the type alone (T-0399, 2026-09-25 JSON red team round 1, entry 14)

`compositeSignal` above is a value scan: it reads the composite's own sampled
record text and runs every validator over each field and over the whole
literal. That misses a whole shape of personal data, found by the round's own
probe schema: a `jsonb` field's value is itself a small document, and a
validator asks whether the *whole* field matches its shape, never whether a
value is embedded somewhere inside it — so a value hit read over the *field's
own text* is asking a different, narrower question than reading the
*document's* own contents would be. (The record's own quoting is not the
obstacle: `splitCompositeLiteral` already unquotes a field and undoes its
doubled quotes before any validator sees it, so the review round that
followed this fix corrected an earlier draft of this paragraph, and of
`THREAT_MODEL.md` T1's composite row, the reason string in `reasons.go`, and
several other code comments, that had all said so.) `reg051_widgets.w` (a
`(tag text, doc jsonb)` composite whose `doc` field held nothing but an email
address inside a JSON object) never scored a hit from any field or from the
whole literal and was decided `none` — copied, with the address inside it,
under exit 0. THREAT_MODEL.md T1's composite row promised a refusal on a
value hit at `possible` or above and said nothing about this case; the fix
closes it, not by teaching `compositeSignal` to decode a document field (that
would need a second leaf walk keyed by the field's own type, the way
`jsonSignal` already walks a plain `json`/`jsonb` column, and is a feature
rather than the fail-closed answer this row needs), but by refusing the
column structurally, before any sample is read.

- **`compositeDocumentField` (types.go) parses the composite's own `CREATE
  TYPE ... AS (...)` definition text** — `Schema.Composites`, the same catalog
  read `isComposite` already resolves a type name against — rather than a
  sample: `compositeFields` splits the field list on top-level commas
  (`splitTopLevelCommas`, quote- and paren-aware, so a field's own `numeric(12,2)`
  typmod does not look like a second field) and `splitFieldNameType` reads each
  field's `quote_ident`-rendered name and `pg_catalog.format_type` type text the
  way `sqlComposites` (`internal/introspect/sql.go`) wrote them. A field whose
  type, after its array suffix and typmod are stripped, resolves to `json`,
  `jsonb` or `hstore` (`documentFamilies`, the row-side document families
  ARCHITECTURE.md §4 and THREAT_MODEL.md T1 already name) is a hit; a field
  that is itself a composite is walked one level down, with a `seen` set
  against two composites that reference each other. It is a reader and not a
  parser, `literal.go`'s own words for `splitCompositeLiteral`: a definition
  text it cannot make sense of is skipped rather than guessed at, because the
  only cost of missing a shape here is the ordinary sample-based check still
  running underneath it.
- **`decideComposite` calls it first, ahead of the name and value checks, and
  it wins over both** — the same way a value hit from `compositeSignal`
  already wins over the copy branches. A hit sets `Category = semi_structured`
  (there is a document in the record, which is the closest existing category
  to what was found) and `Confidence = possible`, with a new reason fragment,
  `composite_document_field` (reasons.go): `composite type %s has %s field
  %s, whose document text a validator cannot read through the record's own
  quoting`. `possible` is above §4's mask threshold and reaches
  `internal/plan`'s existing composite refusal exactly as a value hit does —
  there is no second refusal path, only a second route into the one that
  already existed.
- **The return carries the type the field is actually declared on, not only
  the column's own composite type** — `holderType` in `compositeDocumentField`'s
  signature. For a direct field this is the column's own type; for a nested
  one it is the inner composite's name, never the outer wrapper's: a message
  naming `public.outer_wrap` for a field that is actually
  `public.wrap.doc` sends an operator looking at the wrong `CREATE TYPE`.
  `TestCompositeHoldingANestedDocumentFieldFailsClosed` pins this.
- **`internal/plan`'s `checkWriteBack` names the field too, not only the
  type** (`writeback.go`, `compositedoc.go`). Its composite branch already
  refused any masked composite before this landed, with a message naming only
  the type ("its type %s is a composite, which no masker can write into");
  this task added `documentField` — the same walk, over the same
  `Schema.Composites`, duplicated here because a stage package may not import
  another (`internal/CLAUDE.md`) — so that when the reason the composite is
  masked is this structural one, the message instead reads "its type %s is a
  composite whose %s field %s cannot be read through the record's own
  quoting", and the `{reason}` argument names the field the same way. The two
  escapes are unchanged: `--skip-table` and a reasoned `--unmask`.
- **`internal/verify` carries the identical structural check as a second,
  independent look** (`compositedoc.go`, `CodeRefusedCompositeDocument`, exit
  9), the same "second net" shape every other row-side control in that
  package already has (the catalog pass beside the second net). It reads only
  `Schema.Composites` — the same catalog ARCHITECTURE.md §11.1 recreates
  verbatim, so the target's own type holds the identical fields — over every
  table the run actually loaded, and it honours an operator's own `--unmask`
  for the column (`optedOut`): that is the accepted risk the escape exists
  for, not a hole this net should close behind their back. Nothing in
  `internal/verify` reached a composite type at all before this; there was no
  narrower check to preserve.
- **The v1 cut line is unchanged: field-wise masking of a composite is still
  later work.** This closes a way a composite's own record text could hide
  personal data from the *scan*, not the standing decision that a composite
  can only be refused or copied whole, never masked in place.
  `testdata/regressions/051-composite-holding-a-json-field.sql` is the reduced
  fixture, `expect: exit 12 plan.refused.unwritable`; none of the optional
  regression header keys apply, because a refused plan never reaches a target
  to check.

**Fix-round findings (T-0399, same day).** Three review findings landed
alongside the original fix:

- **`internal/verify`'s own copy skipped nothing for a `SchemaOnly` step**
  (a table `--skip-table` dropped, or one the run could not reach or read).
  Such a step recreates the type but copies no row of it, the same as
  `shapes.go` and `counts.go` already assume, but `compositeDocuments`
  walked every step regardless and refused a run that had correctly skipped
  the table it would otherwise have refused on — exit 9 for following the
  plan's own advice. `compositeDocuments` now skips a `pipeline.SchemaOnly`
  step first, before it looks the table up.
- **A field's own type can be a domain** — `CREATE DOMAIN docdom AS jsonb`
  used as a composite field's type, or a domain over a composite that holds
  a document field — and none of the three copies of `documentFieldWalk`
  resolved it, so a domain one level down defeated the whole check. Each
  copy now resolves a field's type through `Schema.Domains` (the same
  lookup `typeOf` already does for a column's own declared type) before the
  family and composite checks, the way `compositeType`/`p.domainBase` and
  `s.domainBase` already do at the column level.
- **The root-cause text above, `THREAT_MODEL.md` T1's composite row, the
  `051` fixture header and several code comments all blamed the record's own
  quoting.** They were wrong: `splitCompositeLiteral` already undoes a
  field's doubled quotes, so the document reaches a validator as bare text.
  The actual gap, corrected above and in every one of those places, is that
  `compositeSignal` (and its plan/verify mirrors) run each validator over a
  field's whole text and never walk inside it. **This still only closes the
  json/jsonb/hstore case.** A nested composite with no document field of its
  own — a plain `email` field two levels down, read only by
  `compositeSignal`'s whole-value scan the same way any embedded value is —
  is a real, separate gap, filed as **T-0413** (`tracker/epics/E9`) rather
  than folded into this fix.

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

## The 2026-09-15 red team

Four changes, and every one of them widens recall — so every one is a
THREAT_MODEL.md T1 question, and the precision floors in `pagila_test.go` and
`names_test.go` are what keep them honest. Pagila reads precision 0.762 /
recall 1.000 after them, against 0.70's floor.

- **The rule pack's name rules are multilingual now, and that is a patch**
  (`rules.yml`, A1). A `klienci` table whose `nazwisko`, `imie`, `pesel`,
  `komorka`, `cognome` and `achternaam` columns held real-shaped Polish,
  Italian and Dutch values crossed into the target verbatim: the rules were
  only *accidentally* multilingual, carrying `correo`, `courriel`, `nachname`,
  `prenom` and nothing else. About sixty spellings joined `person_name`,
  `phone`, `national_id`, `email` and `address`, and the abbreviations
  (`fname`, `lname`, `mob`, `eml`) with them. **No rule pack is ever complete**,
  which is why the comment in `rules.yml` says so at the rule: the control that
  closes the *class* is the multilingual dictionary in
  `internal/textsig/names.txt`, which both nets read.
- **A `bytea` is judged on its content, not silenced by its family**
  (`byteaTextSignal`, A4a). The note further up this file — "a bytea column is
  never decided by the text validators" — gave the right reason (a PNG reads as
  an address) for a rule that did not cover the case that leaked: a `bytea`
  holding printable UTF-8 with an address in it, in a table with no `certain`
  column so `byteaInPersonShapedTable` could not fire. `bestSignal` now offers a
  sample set that `textsig.PrintableText` accepts to the validators and answers
  **`binary_personal`** on a hit — never the hit's own category, because
  `binary_personal` is the one category `rules.yml` accepts on the family and
  its masker (NULL) is already registered, so there is no new masker and no
  route back to the contradiction the old rule was about. The gate is
  `textsig.PrintableText` **per value** — valid UTF-8 and 95% printable runes —
  and the hit is then either a *strong* validator on any readable sample or any
  validator at `validatorThreshold` across them. A PNG fails the first, so a
  column of images is still decided by nothing here.
  - **The column-level ratio is gone** (the T-REDFIX review's second finding).
    It required 95% of the *sample set* to be readable before any validator
    ran, where `internal/verify`'s second net asks `PrintableText` per value
    with no column ratio — so the two nets were not the same rule, which both
    files claimed they were. A `bytea` column of half documents and half images
    passed neither: unmasked here, exit 9 there, and `binary_personal` is
    unreachable for it by any other route, so the run had no green path short
    of `--unmask` on a column that really does hold documents. One question per
    value on both sides now. The cost is the case the ratio was written for — a
    column of images with one readable blob a validator hits is masked rather
    than left alone — and that is the direction it has to fail in, since
    `internal/verify` refuses that same column today.
    `TestRedTeamA4aHalfPrintableByteaIsMasked` is the guard.
- **The neighbouring-column rule reaches a column with no signal at all**
  (`unknownColumnsBesideCertain`, A2b). The existing arm raises `low` to
  `possible`; a character column with no name hit, no value hit and no type
  signal sat at `none` beside a `certain` personal column and was copied, with
  the report printing "no name or value signal" — which reads as a clean bill of
  health for a column nobody looked inside. Such a column is now masked as
  `free_text`. Two things make it safe to run:
  - **The neighbour must be `certain` under a category that identifies a
    person** (`identifiesAPerson`). `free_text`, `semi_structured`,
    `binary_personal` and `derived_text` are categories a column reaches by its
    *type* alone, so counting them made pagila's `film` table person-shaped on
    the strength of its own `fulltext` search index and masked `film.title` and
    `film.special_features` with it — precision 0.696, under the floor.
  - **Five exclusions, each a run this must not break**: never-masked, a
    type-conflicting decision, a column under a unique index (`free_text` would
    then owe §5's `d_required` and `internal/plan` would refuse at exit 12 over
    a column masked on no evidence), a column of two-letter codes (an ISO
    country column has a domain of two characters), and a declared length under
    sixteen.
  - It does **not** reach A2b's own table, which has no `certain` column at all.
    It is the rail under the next value shape neither net recognises, and the
    attack's own text says so.
- **The validators de-obfuscate** (`internal/textsig/candidates.go`, A2/A3).
  Nothing in this package changed for it, which is the point of `textsig`
  existing: an address written `grace.hopper AT realcorp DOT example` is now an
  address to `bestSignal` and to `internal/verify`'s second net in one change.

## The 2026-09-15 red team, round 2 (T-0187)

**`national_id` joined the ordered `validators` list.** R2-01 through R2-03
(attacks A2, A6, A7 in
`docs/reviews/2026-09-15-redteam/round2-still-leaking.json`) were the same
finding three ways: `textsig.ValidNationalID` was correct and recognised every
attack value, but no entry in this file's `validators` list ever called it, so
a plain SSN in a column called `code` — a name no `rules.yml` pattern matches,
in a table with no other personal column for the neighbouring-column rule to
key on — reached `decide` with the reason `no name or value signal` and was
copied verbatim under exit 0. The fix is one entry, `{pipeline.CatNationalID,
phraseNationalID, true, ...textsig.ValidNationalID}`, at the same `strong`
footing as email: every one of the twelve formats `internal/textsig` now
recognises (nationalid.go, T-0187) is a checksum or an issuing authority's own
exclusion range, the same class of precise parse. Nothing about `decide`'s
scoring changed — the entry runs through the same generic loop every other
validator does, gated by the same `silencedByType` accepted-types check, and
`rules.yml`'s `national_id` category already accepted every family
(`text`/`varchar`/`bpchar`/`citext`/`bigint`/`integer`/`numeric`) the fix
needed, so ADR-010's numeric-family silencing never had to be touched.

**What this package still cannot reach, and why that is the accepted
asymmetry and not a second bug.** R2-04 (A9b) planted the same SSN stored as
`bigint`: a numeric column can hold no hyphen and drops a leading zero, so
`078-05-1001` renders as the eight-digit `78051001`, and
`textsig.ValidNationalID`'s dashed regexp — the only validator this package's
`national_id` entry calls — never matches it, whatever spelling
`anyCandidate` offers. `internal/textsig.ValidNationalIDDigits` recovers that
padding, and it is deliberately *not* wired into this package's
`validators` list: this file's own generic loop applies one `ok` function
across every family uniformly, with no `text`/`digits` split the way
`internal/verify/validators.go`'s does, so there is no way to scope the wider,
ratio-only-safe function to numeric columns alone without either widening
recall on *every* family (an eight-digit code in a `text` column would also
validate) or adding that split here, which is a wider change than this task's
brief asked for. So `taxref bigint` with no name hit still decides `none` here
and is copied — the same "refusal instead of a mask" shape
`internal/verify/CLAUDE.md`'s T-0136 notes already accept for a `bigint`
column with a Luhn minority hit: `internal/verify`'s second net (its own
`digits: true`, non-strong, ratio-scored `national_id` entry, mirroring
Luhn's text/digits split) is what catches the value on the family this
package's entry cannot reach, refusing the already-loaded target at exit 9
rather than leaving a silent leak. `testdata/regressions/020-ssn-stored-as-
bigint.sql` is the regression that pins the refusal.

## The T-0187 review round: national_id split by evidence quality, partially (2026-09-15)

The review that followed T-0187 (finding 2) found the entry above still
calling `textsig.ValidNationalID` — the twelve-format union — uniformly at
`strong`, the same shape `internal/verify/validators.go`'s own national_id
entry had before that file's three-way split. `validators` (above) now
carries two entries instead of one: `textsig.ValidNationalIDStructured` (the
six shape-constrained formats — a US SSN, a UK NINO, an Italian codice
fiscale, a Spanish DNI or NIE, a French NIR) at `strong`, and
`textsig.ValidNationalIDChecksumOnly` (PESEL, BSN, SIN, TFN, Aadhaar's
Verhoeff check, CPF — a mod-N sum over an otherwise unconstrained digit run,
clearing 9%-26% of a random string of the right length by chance) at the
ordinary ratio.

**What the split closes.** `bestSignal`'s `sig.strongHit` (T-0136, finding 7)
masks a *proven* column outright as `free_text` at `ConfPossible` on a single
hit from a `strong` validator, anywhere in the sample, with no neighbouring
column needed. With the union at `strong`, an ordinary business-key or
reference-code column could be masked that way on one coincidental
checksum-only hit — the exposure the split closes: only the six
shape-constrained formats can set `sig.strongHit` now, so a bare checksum
coincidence no longer masks a column by itself.

**What it does not close, and why that is T-0195 and not a second bug.**
`sig.weak` (`bestSignal`) is set by ratio alone —
`proven && ratio >= weakThreshold` — and never reads `v.strong`, so a business
key whose values clear a checksum-only format at or above `weakThreshold`
(0.5) still records `national_id` at `low`, and the neighbouring-column rule
can still raise that to `possible`/masked beside a `likely` personal column in
the same table. That is finding 2's own "consequence (1)", and it needs
either a per-validator type-family gate this package does not have (the way
`rules.yml`'s `accepts:` gates a *name* hit, not a value hit) or a materially
higher within-column threshold scoped to this one entry — both recall-
affecting scoring changes that this file's own rule says need a T1 review and
a `TestPagilaPrecisionAndRecall` measurement, not a quiet edit riding on an
unrelated task. **T-0195** carries it; this paragraph is the deferral pinned
here as well as in the tracker, per the review round's own request.

This package also still has no `text`/`digits` split the way
`internal/verify/validators.go`'s does (`internal/textsig/CLAUDE.md`'s own
note on the point), so an SSN stored as `bigint` with no name hit is still
`none` here — `internal/verify`'s digits-family entry is what catches it
instead, at exit 9 rather than a mask. T-0195 owes that split too.

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
signal has to be at least as safe as the branch with no signal at all.**
`bestSignal`'s `silencedByType` gate (see the T-0054/T-0269 bullet above) is
the same shape with a value signal instead of a name, and as of T-0269 it does
not even reach `decide` as a distinct branch any more — the validator is never
scored, so a rejected value signal and no signal at all are now the same
branch by construction, not two branches kept equally safe by measurement.

## Two decision fields carried for internal/verify (T-0187 third review round, finding 1)

`internal/verify`'s digits-family national_id entry (a numeric column, ratio
scored, no check digit — `internal/verify/CLAUDE.md` has the full account) can
no longer refuse on ratio alone: a sparse column with a fixed leading prefix
clears the SSA's exclusion ranges at essentially 1.0 the same way a genuine
leaked identifier column does, so the entry now refuses only with
corroboration. `internal/verify` may not import this package or re-run
`rules.yml`'s patterns or the neighbouring-column rule itself
(`internal/CLAUDE.md`'s import graph, and this package's own "no rule pack
elsewhere" argument run in reverse), so this package carries the two signals
on `pipeline.Decision` instead of leaving `internal/verify` to guess at them a
second, drifting way:

- **`Decision.NameMatchedNationalID`** is `hit.Category ==
  pipeline.CatNationalID` from `decide`'s own `st.pack.matchColumn`, recorded
  right after the match and independent of `nameAccepted` or of which branch
  the switch below it takes — a plain fact about the column's name, not about
  what the decision did with it. It is deliberately **not** gated on
  acceptance: `internal/verify`'s digits family is exactly `rules.yml`'s
  `national_id` accepts list (`bigint`/`integer`/`numeric`), so a name hit on
  an accepted numeric column is already masked by one of the branches above
  and never reaches this net's digits entry at all — the field can answer
  true in practice only for a column `markNeverMasked` exempts regardless of
  confidence (a surrogate key or an FK column), because that exemption is
  granted *after* `decide` has already run. This is stated rather than
  papered over: the signal exists for the case it can reach, not for a wider
  promise about ordinary columns.
- **`Decision.TableHasLikelyPersonalColumn`** is `neighbouringColumns`'s own
  `likely` count (pass 3, above), carried onto *every* column of the table —
  not only the ones the pass goes on to raise — with the column's own
  confidence excluded from the count: "another column of this table was
  decided at `likely` or `certain`". It costs nothing extra to compute: the
  count already exists per table before the raise loop runs, and this is one
  more assignment inside the same loop.

**Neither field changes what this package masks.** Both are read-only
metadata about a decision this package already made, computed after `decide`
and `markNeverMasked` have run for the column in question (`base`, then
`neighbouringColumns`), so setting them cannot move `Masked`, `Confidence` or
`Category` for any column — a change here that could would need the T1 review
this file's own top rule requires, and neither of these does one.

**A trap for the next fixture that adds a corroborating column: check the
digits-family column's own baseline confidence first.**
`neighbouringColumns`'s pre-existing raise (pass 3's first arm, unrelated to
this finding) promotes any column sitting at exactly `low` to `possible`
— masked — the moment the table gains a `likely` column, whatever put it at
`low`. A digits-family column whose bare digit string happens to clear one of
this package's own checksum-only national_id formats (a coincidence, not a
leak) is at `low` already, and adding a personal neighbour for
`TableHasLikelyPersonalColumn` corroboration then masks it outright instead
of corroborating it — `testdata/regressions/020-ssn-stored-as-bigint.sql`'s
original five values were exactly that trap (three of five coincidentally
cleared BSN's or Australia's TFN's checksum on the bare digit string), and its
own header and `internal/verify/CLAUDE.md`'s matching note explain how the
replacement values were chosen: confirmed, by direct computation against
`ValidNationalIDStructured` and `ValidNationalIDChecksumOnly`, to clear none
of this package's twelve formats.

## A third decision field, for a masked neighbour the second one's floor missed (T-0240, the 2026-09-15 round-4 red team)

`Decision.TableHasMaskedPersonalColumn` joins the two fields above
(`internal/pipeline/classify.go`'s own comment has the full account): another
column of the table decided at `ConfPossible` or above, not exempt from
masking, under a category `identifiesAPerson` names. It is computed in the
same `neighbouringColumns` loop as `TableHasLikelyPersonalColumn`, by the new
`maskedPersonalNeighbour` helper, and carried onto every column of the table
the identical way.

**Why a third field and not a lower floor on the second one.** Three A9b
replays in the round-4 red team (`docs/reviews/2026-09-15-redteam/
round4-still-leaking.json`) planted a nine-digit national identifier — bigint
dense, bigint sparse, and `varchar(9)` dense — beside an `msisdn`
numeric/text column of real UK-shaped phone numbers. `msisdn` is one of
`rules.yml`'s own phone abbreviations (`(^|_)(...|msisdn|...)(_|$)`), so it
is `hasName && nameAccepted` in `decide` on its name alone; with no
`--phone-region` configured the numbers are not in the international `ZZ`
form `textsig.ValidPhone` reads, so `best` (the value signal) is nil and
`decide`'s own default arm for that branch — a name match with nothing from
the values to raise it further — records `ConfPossible`, not `ConfLikely` or
`ConfCertain`. `TableHasLikelyPersonalColumn`'s own `ConfLikely` floor never
saw it, so `internal/verify`'s `corroborated` gate read the table as holding
no personal column at all, over a column the run had already decided to
mask. Lowering `TableHasLikelyPersonalColumn`'s own floor to `ConfPossible`
was considered and rejected: that field is also `guessedPhoneColumns`'s own
corroboration signal (T-0221, below) for an entirely different, *value-only*
phone reading (no name hit at all), and widening what corroborates a guess
with no name evidence behind it was not this task's brief — a name-matched
column at `ConfPossible` and a value-guessed column with no name signal are
different claims that happened to share one field only by accident of both
being written against T-0187's corroboration pattern.

**`internal/verify/validators.go` also gained a character-family twin of the
digits-family national_id entry, on the same task.** The `varchar(9)`
replay is not answered by this field alone: nothing on the row-scanning
side had ever called `ValidNationalIDDigits` over a *character* column,
structured/checksum-only being the only two text-family national_id entries
there, and neither matches a bare zero-padded digit run. `internal/verify/
CLAUDE.md`'s own T-0240 section has the account, including the ordering fix
this task made to `secondnet.go`'s dense-sequence exemption: a dense,
contiguously issued identifier block — a payroll or benefits import — has
exactly the shape the exemption was written to allow through, so the
exemption must not outrank corroboration once corroboration exists to ask.

**A same-day review round found that rule too wide for an ordinary
foreign-key child, and `finalise` now carries a fourth field for the same
reason the three above exist.** `TableHasMaskedPersonalColumn` (and
`TableHasLikelyPersonalColumn` before it) answer "does something *else* in
this table look personal", which is the wrong question for a column whose
own values `internal/classify` has already decided ARE a surrogate key's —
`order_id`'s are `id`'s, copied verbatim across the foreign key — and an
unrelated genuine `email` column in the same table should no more cancel
that than it should invent a national identifier out of nothing.
`Decision.NeverMasked` (`internal/pipeline/classify.go`) is `work.neverMask`
carried onto the decision by `finalise`, after `keyChildren` and
`foreignKeys` have both had their say — the same final value that decides
`Masked`, never an earlier snapshot — so `internal/verify` can ask "is this
column exempt as a key" as a question separate from "does this table hold
other personal data", the same way it already asks the corroboration
question by three other carried fields rather than re-deriving `rules.yml`.
`internal/verify/CLAUDE.md`'s own review-round section has the fixture that
found it (`testdata/regressions/021`, restored to its original shape rather
than edited a second time) and the two fixtures that still refuse without
it (`032`, `033`, neither a key or an FK column).

`textsig.ValidPhone` (the single `phone` entry in `baseValidators`) parses
under `textsig.PhoneRegionHint` ("ZZ") only, which admits a number already
written in international form and nothing else. A national-format number —
`07911 123456`, `020 7946 0958` — scored zero however many rows agreed,
whatever the column was named, and whatever it was dictated in words rather
than digits: `docs/reviews/2026-09-15-redteam/round3-still-leaking.json`'s
attack:1:r3 put exactly that shape, with no obfuscation at all for the
`kontaktnr` half, into a table whose name no `rules.yml` pattern matches, and
300 rows crossed under exit 0 reporting "no name or value signal".

**With a region configured, the row path parses a second candidate under it,
on the same strong footing.** `--phone-region REGION` / the yml's own
`classify.phone_region` (`pipeline.Config.PhoneRegion`) is resolved once, in
`Classify`, into `state.region`. `buildValidators(region)` returns
`baseValidators` unchanged when `region == ""`; otherwise it splices a second
phone entry — `ok: func(_, s) bool { return textsig.ValidPhoneRegion(s,
region) }`, `strong: true` — immediately after the international-only one, so
the two sit together in the precedence order §4 states rather than at the
end of the list. `state.validators` (built once per `Classify` call, since
the entry closes over a per-run value) replaces every direct read of the old
package `var validators`; `bestSignal`, `byteaTextSignal` and
`compositeSignal` all take it as a parameter now (`vs []validatorEntry`)
instead of reading a global. An operator-named region is trusted evidence on
the same footing the international-only entry already has: the entry is
`strong`, decided through the same generic ratio loop, no corroboration
gate. `decide`'s `regionAssumed` appends a `phone_region_configured` reason
fragment naming the region whenever `sig.strong` or `sig.strongHit` is this
entry, so the reasons output states the assumption rather than leaving the
operator to infer it from a flag they may not have typed on this run — the
yml's own `phone_region:` carries the value forward, on the same
"flags, last, so they win" rule `internal/core`'s `--allow-type-literal`
merge already follows.

**Alongside whatever region is configured, a short, fixed list of regions is
also tried, and a hit decides nothing without corroboration.**
`phoneGuessRegions` (fifteen large calling-code populations) is not offered to
`bestSignal`'s ratio loop at all — trying it unconditionally would mask a
column the moment some region's numbering plan fit by chance, the same
false-positive shape `internal/verify/validators.go`'s `requiresCorroboration`
note measures for a sparse, fixed-prefix `national_id` digits column (T-0187's
third review round). Instead, `base` computes `guessedPhoneHit` (an OR across
the list,
≥`validatorThreshold`) separately, stores it on `work.guessedPhone`, and a new
pass — `guessedPhoneColumns`, called from inside `neighbouringColumns` after
the loop that fills `Decision.TableHasLikelyPersonalColumn` for every column
and before its `unknownColumnsBesideCertain` arm (which would otherwise sweep
the same unsignalled character column into `free_text` first) — masks it only
when `Decision.TableHasLikelyPersonalColumn` is true: the identical neighbour
signal `internal/verify`'s national_id digits entry already reads for its own
corroboration gate, the "T-0187 pattern" this task's brief names. Without it,
`guessedPhone` is computed and simply never acted on — the column is left
exactly as every earlier pass decided.

A first landing also tried to corroborate off the column's own name matching
`rules.yml`'s phone pattern (`work.nameMatchedPhone`, set in `decide` beside
`NameMatchedNationalID`), but a review round proved that arm could never
fire and it was removed rather than kept undocumented-dead: `guessedPhone`
is only ever set while the column's confidence is still below
`ConfPossible`, and on the character families this whole feature runs over,
`rules.yml`'s phone pattern accepts every one of them (`text`/`varchar`/
`bpchar`/`citext`) — so a name match always takes `decide`'s ordinary
`hasName && nameAccepted` branch to `ConfPossible` or above before
`guessedPhoneColumns` could ever see the column with `guessedPhone` set.
`TestNameMatchedPhoneColumnMasksOnNameAlone` pins the branch that actually
masks such a column instead.

**The guessed-region pass runs over a character family only, and that
restriction is not incidental — it is what keeps this feature from
colliding with `internal/verify`'s own national_id digits entry.** The first
landing gated `guessedPhoneHit` on `!silencedByType(..., CatPhone, family)`
alone, which `rules.yml`'s `phone` category satisfies for
`bigint`/`integer`/`numeric` too (the same families `phone`'s masker already
writes into) — and `make torture` caught the collision this introduced
before any fixture was written for it:
`testdata/regressions/020-ssn-stored-as-bigint.sql`'s own `taxref bigint`,
corroborated by the file's own `email` neighbour for the same coincidental
reason its header already explains for `national_id`'s checksum-only
formats, cleared one of the fifteen guessed regions on all five values and
was masked `phone` before `internal/verify`'s digits-family `national_id`
entry ever saw the unmasked column — a real mask, of the wrong category,
that turned the file's pinned `exit 9 verify.refused.second_net` into a
quiet `ok`. The gate is now `isCharacterFamily(ct.Family)`
(`text`/`varchar`/`bpchar`/`citext`) in addition to the type-conflict check:
a digits-family column is left exactly as it was for `internal/verify`'s own
entry to decide, which is what `020` still asserts and what
`testdata/regressions/026-ten-digit-account-number-is-not-a-guessed-phone.sql`
(the false-positive control, an ordinary ten-digit account column with no
name or neighbour signal) had to be written as `text` rather than `bigint`
to test honestly — a `bigint` column would have raced the identical
coincidence a second time.

**What this still misses, named rather than hidden.** A plain national-format
phone number, with no configured region, in a column whose name matches
nothing and whose table holds no other `likely`-or-above column, still
crosses unmasked — `kontaktnr` alone in a single-column table would. That is
narrower than the miss this task closes (an operator now has a flag that
closes it completely, and the yml carries the answer forward once used), and
it is the same shape of residual gap the multilingual-dictionary amendment
already accepts for a name in a language nobody wrote a pattern for: the
class is closed by an operator's word or a corroborating neighbour, never by
a guess alone.

**Owed elsewhere.** `internal/verify/validators.go`'s own phone entry gained
the identical region reading (its `count`, T-0221) — never the guessed list,
which stays here only — and its own comment records the asymmetry.
THREAT_MODEL.md T1 carries this amendment in its own terms; ARCHITECTURE.md
§4 and §8's flag table were both in this task's paths and are corrected
directly rather than filed.

## The T-0221 review round (2026-09-16): an either/or that widened nothing, and an unvalidated flag that silently narrowed everything

Two high findings, both about the same premise: a configured region was
treated as *the* answer for phone numbers rather than as *one more* piece of
trusted evidence.

**A configured `--phone-region` used to turn the guessed-region fallback off
entirely** (`base`, above): the first landing of this section's pass gated
`guessedPhoneHit`'s computation on `st.region == ""`, so naming a region did
not add a fifteenth trusted region to the fifteen already guessed at — it
*replaced* all fifteen with the one named. A varchar(15) column of US-format
numbers beside a `certain` email neighbour was `masked=true cat=phone` with
no flag and `masked=false cat=none`, "no name or value signal", the moment
`--phone-region GB` was named: exactly backwards for the common case this
flag exists for, a multi-country database, where naming the operator's own
region is a reason to trust *that* region without corroboration and never a
reason to stop looking at the other fourteen. The gate is gone: `base` now
computes `guessedPhoneHit` whenever nothing else has already decided the
column, whatever `--phone-region` holds, so the configured region is folded
into the same footing as every other trusted-without-corroboration read
(`buildValidators`'s spliced entry) while the guessed, corroboration-gated
fallback keeps covering everything else.

**`--phone-region` reached this package and `internal/verify` with no check
that libphonenumber could read it at all.** `textsig.ValidPhoneRegion` parses
under an exact, upper-case, libphonenumber-recognised code, so `"gb"`, `"UK"`
and any typo all parse zero phone numbers — silently, with no distinction
from a real region that genuinely has no match in a given sample. Before this
fix, that meant a single-character typo on the flag took a column that would
have masked correctly with **no flag at all** down to unmasked, under exit 0,
because the old `st.region == ""` gate above read only "is this string
non-empty", never "is this string usable" — a single-character typo turned
off the fifteen-region fallback exactly as a real region name would have. The
belt-and-braces half of the fix is the same code change as the paragraph
above: with the gate gone, an unusable `--phone-region` value can no longer
disable the fallback, because nothing in `base` reads `st.region` to decide
whether to compute `guessedPhoneHit` any more. The primary fix is
**`cmd/lazyslice`'s own flag-surface validation** (`checkPhoneRegion`,
`main.go`): `--phone-region` is normalised to upper case and checked against
`phonenumbers.GetSupportedRegions()` via the new
`textsig.SupportedPhoneRegion`, exit 2 on anything it does not recognise, the
way a misspelled `--memory-budget` already is — before this package or
`internal/verify` ever sees the value. `internal/textsig/CLAUDE.md` records
`SupportedPhoneRegion` on that package's side.

**Test coverage.** Neither half of this section's own behaviour was pinned by
a test that runs in `make check` before this round: `TestGuessedRegionPhone
Corroboration` exercised only the `TableHasLikelyPersonalColumn` arm of
`guessedPhoneColumns`, and the configured-region row path (`buildValidators`,
the strong entry, the "phone region assumed" reason) was covered only by
`testdata/regressions/025` under `make torture` (Docker-gated). Two tests
close the gap: `TestConfiguredPhoneRegionMasksNationalFormatColumn` classifies
a national-format column with `prior.PhoneRegion` set directly and asserts
both the mask and the `"phone region assumed: GB"` reason fragment, with no
database and no `make torture` run needed; `TestNameMatchedPhoneColumnMasksOnNameAlone`
(renamed from `TestGuessedRegionPhoneCorroborationByNameAlone` by the review
round that found the `nameMatchedPhone` arm unreachable) pins `decide`'s
ordinary name-match branch on a column whose name matches `rules.yml`'s
phone pattern, holding guessed-region-shaped values, in a table with no
personal neighbour at all — asserting it is still masked as phone, on the
name alone, with no corroboration from `guessedPhoneColumns` needed.

## A validated foreign key's columns are raised together, or not at all (T-0239, T-0253)

`unknownColumnsBesideCertain` (the "2026-09-15 red team" section above, A2b)
used to exclude a validated, non-virtual foreign key's character-family
columns outright, at either end, the moment `minUnknownLen`'s floor dropped
from sixteen characters to two (T-0239's fix-round review;
`testdata/regressions/031-fk-child-code-column-beside-a-certain-column.sql`).
The reasoning was that "a genuinely personal FK-linked column is still
reached by every other pass — a name hit, a value validator, or FK
propagation once one end is masked on real evidence."

**The 2026-09-17 round-5 red team disproved that claim**
(`docs/reviews/2026-09-15-redteam/round5-still-leaking.json`, the classifier
attacker's FK variant, tracker T-0253). A validated foreign key's
character-family child can carry the exact shape this rail exists for — a
native-script name, no name rule, no value hit — beside a `certain` email
neighbour, with a parent whose own table holds no `certain` column for any
other pass to key on. Nothing else in this package reaches such a child
either: `keyChildren` only reconciles the integer/uuid key case
(`isKeyFamily`), and `propagateKeys` only ever propagates a masked **parent**
forward, never a masked child back. The blanket exclusion copied real
personal data verbatim on both ends of the join under exit 0
(THREAT_MODEL.md T1) — worse than the half-loaded target (T-0132's failure
mode, an exit-8 `VALIDATE` after every row has moved) it was written to
avoid, which is a refusal that costs a rerun rather than a leak that costs
nothing at all.

**The fix is `fkPairs`, called from `unknownColumnsBesideCertain` once a
column has already passed `raisableUnknown` on its own signals.** It asks
whether every column directly paired to it across `indexFKColumns`'s
`fkPartners` map — `cref`'s own edges, not the wider connected component
those partners may themselves sit in — can be raised the same way
(`fkPartnerRaisable`): character family, no name or value signal of its own,
above `minUnknownLen`. If every direct partner qualifies, all of them are
raised together with `cref`, under the same category (`free_text`) and the
same confidence, so the join stays in agreement and `internal/plan`'s
equality-group masker choice (T-0132, `internal/plan/equality.go`) has one
category on both sides to work from rather than a masked child and a copied
parent. If any direct partner does not qualify, **neither `cref` nor that
partner is raised**: masking `cref` alone would still copy the pair, which is
the one outcome this rule must never produce.

A lookup table referenced by two children, or a chain of keys, is still
brought into agreement once one end of it is masked — but by `propagateKeys`
(below, in "columns are decided in a fixed order"), a separate pass that runs
after this one and already propagates a masked parent to *every* column
referencing it, unconditionally, per ARCHITECTURE.md §4's own "a masked PK or
unique column's decision overrides the decision on every column referencing
it". `fkPairs` walked that whole component itself until this task's own fix
round: a partner two hops from `cref`, in a table with no relationship at all
to the `certain` neighbour that justified raising `cref`, could veto the
pairing on its own shape (a two-letter code, a type conflict) and leave
`cref` copied verbatim — the exact leak this rail exists to close, reached by
a column that never should have had a vote on it. `propagateKeys` does not
have that failure mode: it records `type_conflict` on the one child that
cannot accept the propagated category and masks everyone else regardless, so
letting it own everything beyond `cref`'s direct edge closes the vote-by-
proxy hole without changing what testdata/regressions/031 and 035 mask (both
are a single direct edge; see their own headers).
`TestFKPairIgnoresAnUnrelatedGrandchildOfASharedLookupParent`
(`internal/classify/redteam_test.go`) pins the fix: an `invoices` table that
shares `currencies` as a lookup parent with `members`, and whose own FK
column is not a character type at all, no longer blocks `members.currency`
and `currencies.code` from being raised.

**This narrows, but does not remove, the blast radius a shared lookup parent
carries.** Masking a parent PK still cascades to every table that references
it, including one with no relationship to the `certain` column that started
the raise (`invoices` above is left alone only because its own column type
refuses `propagateKeys`' category, not because the cascade skips unrelated
tables in general) — that cascade is ARCHITECTURE.md §4's own rule, not
`fkPairs`' choice, and a schema where the shared parent is a small table
under a unique index refuses the whole run at exit 12 rather than loading a
mismatched join (the paragraph below). A real schema with a widely-shared,
narrow lookup table (a status code, a country code) referenced from many
otherwise-unrelated tables, where one of those tables happens to hold a
`certain` personal column, is expected to refuse in full for the same
reason `testdata/regressions/031` does — measured, not assumed, and named
here rather than left as a surprise on a first run.

A member under a unique index is not excluded from the pairing the way a
column's own uniqueness already excludes it from firing at all
(`raisableUnknown`'s pre-existing check, unchanged): it is a partner
*because* the certain neighbour supplies the evidence the standalone
exclusion says it has none of, and `internal/plan`'s own unique-index domain
check (`checkUniqueDomain`, unmodified) is what admits or refuses the masked
result on its own existing terms once both ends carry a decision — the same
backstop a lone unique column already relies on. **Measured against both
regressions below, a small lookup table refuses rather than loads**:
`free_text`'s generator draws from a fixed word list, not an unbounded
alphabet, so a narrow unique column's domain (3 distinct values for a
three-character column, 635 for a twelve-character one) is nowhere near
ARCHITECTURE.md §5's `d_required` for even a handful of rows, and
`internal/plan` refuses at exit 12, naming both ends of the pair together
with `--unmask` for each — the same message T-0132's equality-group
mechanism already prints, unmodified by this task. That refusal is the
correct answer named in the round-5 attack's own fix text ("refuse the run
at exit 12 ... the way `internal/plan` already refuses a unique-indexed
column free_text cannot fill"), reached with no new code in `internal/plan`
at all: once both ends carry the same category, the existing mechanism
already asks the right question.
`testdata/regressions/031` now pins that refusal (its own schema is
unchanged since the T-0239 review; its header moved from both ends unmasked
to `exit 12 plan.refused.unique_domain`);
`testdata/regressions/035-native-script-fk-child-beside-a-certain-column.sql`
pins the round-5 attack's own schema, the identical refusal. Both
`TestRedTeamRound5FKNativeScriptChildBesideCertainColumn` and
`TestRedTeamRound4A2bValidatedFKColumnsRaisedTogether`
(`internal/classify/redteam_test.go`) pin the classifier's own half of this
— both ends raised, under the same category — at the unit level, where
`internal/plan` does not run; the refusal itself is what `make torture`
proves.

**The direct-partner bound above had its own leak, found and fixed in
T-0253's second review round.** Bounding `fkPairs` to `cref`'s direct
partners is safe *downward* — a masked parent still reaches every other
child through `propagateKeys`, unconditionally — but it is not safe
*upward*: when a raised partner is itself the **child** end of a further
validated foreign key, that further parent is never reached by anything.
`propagateKeys` only ever pushes a decision from a masked parent down to its
children; nothing in this package ever raises an unmasked parent because one
of its children just got masked, which is the same asymmetry T-0253 exists
to fix in the first place, moved one hop further out. A chain — a natural
key, a child referencing it, a grandchild referencing the child — used to
raise the grandchild and the child together and leave the natural key itself
copied verbatim, holding the identical values (THREAT_MODEL.md T1), and also
left the load with a masked child and an unmasked parent across that further
edge (the T-0132 half-loaded-target shape, T8) — the very failure mode this
rail exists to prevent, reintroduced by the fix that closed the sibling
finding above.

Closed by walking `fkParents` — `indexFKColumns`'s new directional half of
`fkPartners`, a child's own parent column(s) and nothing else — transitively
from `cref` and from every column `fkPairs` has already collected, gating
each further parent through `fkPartnerRaisable` exactly as a direct one, until
nothing new is found. A disqualified ancestor, however many hops up, still
refuses the whole set: masking the columns below it and leaving it copied
would be the same leak. The *downward* bound is unchanged — this never walks
from a raised column to a further child of its own, only to its own
parents — because that direction is `propagateKeys`'s to cover, and walking
it here transitively would resurrect the medium finding the direct-partner
bound was written to close.
`testdata/regressions/036-chained-fk-native-script-names-three-tables-deep.sql`
pins a three-table chain with a distinct column name at every hop
(`slug_root`/`slug_mid`/`slug_leaf`) — identical names would let the
same-name pass (`sameColumnName`, below) mask the grandparent for an
unrelated reason and hide the bug — and refuses at exit 12 the same way 031
and 035 do, since every table in the chain is a small table under a unique
index. `TestFKPairWalksUpwardThroughAChainedForeignKey`
(`internal/classify/redteam_test.go`) pins the same shape at the unit level.

**A direct partner that already carries a decision this rail must not
override, and internal/plan is what refuses the run over it (T-0257,
closed alongside this task).** A type-conflicting name hit (ARCHITECTURE.md
§4 never lets a raising pass move one) or a measured two-letter-code value
shape (real evidence the column is a code lookup, not personal data) blocks
the whole pairing, and `Classify` is pure and pluggable-refusal-free, so
this package cannot make a run stop by itself: it can only leave both ends
unmasked and record why. `Decision.Refused` and `Decision.RefusedPartner`
(`internal/pipeline/classify.go`) are set on both ends the moment `fkPairs`
blocks a pair, naming the reason and the other column, and
`internal/plan/fkpair.go`'s `checkFKPairRefusal` is what reads that signal
and refuses the run at exit 12, naming both columns of the pair, with an
`--unmask` escape for each — the correct answer named in the round-5
attack's own fix text, in the style `internal/plan/unique.go` already uses
for a unique-indexed column `free_text` cannot fill. Before that check
existed, both ends stayed unmasked and loaded copied verbatim under exit 0,
the pre-T-0253 leak reopened for this one shape; `TestFKPairRefusedWhenPartnerIsTwoLetterCodes`
and `TestFKPairRefusedWhenPartnerHasTypeConflict` still hold this package's
own half of it — neither column masked, both `Refused` naming the other —
and `internal/plan/fkpair_test.go`'s `TestFKPairIsRefusedAtPlan` holds the
exit-12 refusal that Decision now drives.

## The sweep spares enumerations and identifier shapes (T-0311)

Dogfood session 1 ran a production Rails schema of 143 tables, and
`unknownColumnsBesideCertain` swept every signal-less character column of any
table with a `certain` email, IP or search-token column into `free_text`:
`role`, `ui_mode`, `os_type`, `state`, `log_level`, `timezone`, a text uuid,
colours, asset paths. The copy held filler where the application expects
`admin` or `linux` and did not boot. Those columns are not what the rail was
written for — a free-text column nobody could look inside — because their
samples say what they are.

`base` now records `work.spare` (`sparedBy`, `spare.go`) for every character
column with no decision of its own. Three things spare a column, each with its
own reason fragment (`spared_unique`, `spared_identifier`, `spared_enum` in
`reasons.go`), and a spared column is left at `CatNone` and copied. The order
in the sweep is load-bearing: the unique-index skip runs first, where
`raisableUnknown` always ran it; then `fkPairs`; and only when `fkPairs`
agrees does `spared` get asked about the two sample shapes. A first draft
asked `spared` before `fkPairs`, and `make torture` caught what that cost:
`testdata/regressions/037`'s child is an enumeration of placeholder strings
whose FK parent is a name-matched `dob` carrying a type conflict, and sparing
the child skipped the T-0257 pair refusal and copied both ends under exit 0.
A partner's own decision is evidence about the values on both sides of the
join, so it outranks the child's sample shape. `031` went the other way and
correctly: its child is four ISO codes repeated, its partner is
signal-less, so the pair is spared, both ends copied in agreement, and the
file moved from `exit 12` to `ok` with `not-masked:` and `equal-masked:`.

- **A unique index.** `raisableUnknown` always skipped it; it was the one
  silent skip, and the check moved into `spared` so the line says so.
- **Every sample one identifier shape**, at `minSamples` or more:
  `textsig.ValidUUID`, `HexDigest`, `SemanticVersion`, `HostnameShape`,
  `PathShape`, tried in that order. The two made of words (hostname, path)
  are refused on any value carrying a dictionary name or a special-category
  term (`carriesAPerson`): `/home/grace/...` is still swept.
- **An enumeration**: `enumMinSamples` (10) non-NULL samples or more, at
  most `enumMaxDistinct` (20) distinct values, every one seen at least twice,
  every value an `enumTokenRE` token (ASCII, no whitespace, at most 64
  characters), and none carrying a person (`carriesAPerson`), a gender or
  title term (`genderTerms`), a blood group or marital status
  (`attributeTerms`), a run of four digits or more, hyphens, dots, slashes and `+` allowed between them (`digitRun`: a postcode, a ZIP+4, a local phone number), or a date
  (`datelike`).

**The T-0311 review round.** The first landing was probed with personal
columns under neutral names beside a certain email, and each was spared and
copied where it had been swept: unpadded dotted dates of birth
(`5.3.1985`) as semantic versions, `05/Mar/1985` and `/home/jsmith` as paths,
and blood groups, marital status, ZIP codes, ISO dates, logins and CamelCase
handles as enumerations. `textsig.SemanticVersion` now refuses a dotted date
with a four-digit year and `textsig.PathShape` wants an application-path
marker (its doc comment has them); here, `attributeTerms`, the four-digit
floor and `datelike` guard the enumeration, and `carriesAPerson` splits a
CamelCase value at its case boundaries before the dictionary reads it, which
guards the hostname and path shapes too. A second review round found the
four-digit floor read only a bare digit run, so a ZIP+4 (`94105-1234`) or a
local phone number (`555-1234`), two parts where `datelike` wants three, was
still an enumeration at `sparedBy`; `digitRun` now counts digits across the
separators. End to end those two parsed as phone numbers under a guessed
region and were masked anyway, so the control that proves the guard uses
ZIP+4 codes no guessed region parses (`00501-0001`). `TestSweepSparesEnumIdentifierAndUniqueColumns`
has one control per guard, each confirmed by removing the guard and watching
the control be copied. Logins (`jsmith`) and attributes outside the closed
lists stay THREAT_MODEL.md T1's stated residual, with the page-clustered
sampling that makes "each seen twice" easy for them to meet.

**A hex column is spared only at a digest's length (T-0354).** The review of
T-0311 found `HexDigest` accepted 8 to 128 characters, and `LooksSecret`
claims hex only from 32, so a 15-character hex token column (an API key, a
reset token, an invite code) beside a certain email was copied as "hex
digests". `textsig.HexDigest` now takes 8 to 12 characters or exactly 32, 40,
64 or 128 where `LooksSecret` does not claim the value; everything else is
swept. `TestSweepSparesEnumIdentifierAndUniqueColumns` carries the probe
(`devices.ext_b`, swept) and a SHA-256 `serial` (spared); the spared column
is SHA-256 rather than SHA-1 because `net.ParseMAC` reads 40 (and 12 and 16)
bare hex characters as a MAC address, so a SHA-1 column is masked
`network_id` before the sweep is asked — safe, wrong category, filed as
T-0363. THREAT_MODEL.md T1's T-0354 amendment states the residual: a hex
token of exactly a digest's length.

**Every threshold is a claim about all the samples, never a ratio**, and the
guards are what keep it from being the T1 hole: the ASCII-token rule is what
keeps a repeated native-script name (regressions `030`, `035`; the control in
`043` and in `TestSweepSparesEnumIdentifierAndUniqueColumns`) swept however
often it repeats, and the dictionary guard is what keeps a colour enumeration
with `green` in it swept. Removing either guard fails that test's controls —
checked by reverting them, not assumed.

**What it moves beyond the rail.** `sameColumnName` shares a category between
same-named columns, and the category it shared from a swept column was
`free_text` reached on no evidence; with the swept column spared, a
same-named column elsewhere that had nothing of its own is copied too.
Measured over the torture corpus that is twenty columns (metabase's `type`,
odoo's `model`, calcom's `weekStart`), none personal. The reverse still
holds: a spared column that a same-named column *with* evidence reaches is
raised by `sameColumnName` as before, and its line carries both fragments.
THREAT_MODEL.md T1's T-0311 amendment has the whole measurement and the
residual; ARCHITECTURE.md §4's T-0311 amendment states the rule.

**A swept column that was NOT spared is still not a source (T-0312).** The
paragraph above only closed the gap for a column T-0311's own three guards
took out of the sweep entirely; a column the sweep actually raised — genuinely
signal-less, correctly masked `free_text` beside its own table's `certain`
neighbour — was still an ordinary `sameColumnName` source until this fix,
and dogfood session 1's own count (25 propagations of this shape in one
run, four of them plan refusals over a unique-indexed text `uuid` column
with no evidence behind it at all; ARCHITECTURE.md §4's T-0312 amendment has
the breakdown) is what this closes. `work.sweptNoSignal`
(`unknownColumnsBesideCertain`, set on the column it raises and on every
`fkPairs` partner raised alongside it) is the mark; `sameColumnName`'s
source-building loop skips a column carrying it, the same way it already
skips `neverMask` and `CatNone`. A swept column is still an ordinary
*target*: a same-named column elsewhere with real evidence still raises it,
unaffected. `TestSameColumnNameDoesNotPropagateASweptDecision`
(`classify_test.go`) pins it: a source table's signal-less column is swept
beside its own `certain` email neighbour, and an unrelated table's
same-named column — sampled, examined, and with no `certain` neighbour of
its own — stays unmasked rather than inheriting the swept category.

**The review round found the bit outlived the sweep's own guess
(T-0312).** `sweptNoSignal` was set and never cleared, so a swept column that
a *later* pass — `propagateKeys`, which walks every foreign-key edge
including an unvalidated one `fkPairs` never sees — then overwrote with real
evidence (the parent's category at the parent's confidence,
`Source=ByFKPropagation`) still carried the stale bit into `sameColumnName`,
which kept an evidenced decision out of the source set for no reason: the
column is no longer a guess once propagation has spoken for it.
`propagateKeys` now clears `cw.sweptNoSignal` in the branch that gives `cw`
the parent's category, the same place it already clears `cw.neverMask`,
`cw.typeConflict` and `cw.frameworkMetadata` for the same reason.
`TestSameColumnNameUsesASweptDecisionOnceFKPropagationBacksIt` pins the
reproduction: an unvalidated FK from the swept column to a certain,
unrelated table's key lets both passes reach it, and a same-named column
elsewhere is masked once the sweep's guess becomes real evidence. **Owed:**
the residual recall change from the exclusion itself — a genuinely swept,
never-FK-propagated column no longer exported as a source — is unmeasured
over `testdata/torture/`; the two DB-free truth sets this package carries
(`TestPagilaPrecisionAndRecall`, `TestFiftyNamesFromThreeSchemas`, the
latter including the supabase-auth truth set) show no change in precision or
recall from this diff, but neither contains the shape it affects. Filed as
T-0351.

**A committed lazyslice.yml keeps the old mask.** The yml records each
column's confidence, and `applyPrior` raises a column back to what the file
says (`yml_column`; ADR-004 lets a committed file only tighten). So a project
whose yml was written before T-0311 still masks a column this rule now
spares, until the file is regenerated with `--reconfigure` or the entry is
edited — the safe direction, and deliberately not special-cased here.

**Owed outside this package (T-0350).** `testdata/regressions/043` pins the
spared columns with the `not-masked:` key, whose failure message in
`internal/invariants/torture_test.go` was written for T-0221 and always says a
guessed-region phone hit was masked. A regression of this rule would fail
there correctly but name the wrong cause; the harness is outside this task's
paths.

## A bare name needs corroboration (T-0313)

Dogfood session 1 found 38 columns called just `name` — tags, folders,
playlists, widgets, roles, languages, AI models, triggers — and every
Paperclip `*_file_name` column masked as `person_name` by the rule pack's
`(^|_)names?(_|$)` word, and two under unique indexes refused the plan. The
word says a column holds *a* name, not *a person's*.

**The rule pack.** `person_name`'s row keeps every specific spelling
(`first_name`, `surname`, `nazwisko`...) and loses `names?` and
`display_?names?`, which are `bare_name` now, one priority below it (34), so a
specific spelling still wins a tie. A qualifier that itself says the name is a
person's — `legal_name`, `preferred_name`, `nick_name`, `birth_name`,
`married_name`, `card_holder_name`/`cardholder_name`, `billing_name`,
`shipping_name`, `name_on_card`, `name_given`/`name_family`/`name_first`/
`name_last`/`name_middle` — was added to `person_name`'s row by the review
round, so it keeps `possible` on the name alone and never waits on
corroboration; each spelling carries the underscore, so the row masks exactly
what the old bare word masked there and nothing new (`bare_name_test.go`'s
`kyc_checks` pins them with samples the dictionary does not hold). The
run-together spellings (`nickname`, `legalname`) match no name rule at all,
before T-0313 and since; that recall gap is T-0353. `bare_name` carries the two fields
`rulepack.go` learnt for it: `unless` (`(^|_)file_?names?(_|$)`) takes a file
name out of the rule entirely, in `match` and `matchColumn` alike, and
`corroborated_by` is a regexp of words for people — about eighty,
singular and plural, erring wide (`clients` and `agents` are here though they
also name OAuth clients and AI agents) — read against the normalised table name and the normalised column
name. `decodePack` refuses `corroborated_by` on any category but
`person_name`, because the name dictionary is the only value evidence that can
corroborate it (`TestCorroboratedByIsReadOnAPersonNameRuleOnly`).

**`decide`**, on a name hit from a rule that carries `corroborated_by`, asks
`bareNameVerdict`, which answers in this order:

1. a word for people in the table or column name — corroborated, and the line
   names the word (`bare_name_by_word`);
2. fewer than `minSamples` samples — unproven, so the name decides alone as
   it always did (`bare_name_unproven`): an empty table's `name` is masked;
3. at least `nameCorroborationThreshold` (0.2) of the samples carrying a word
   from the dictionary (`textsig.Dict.ContainsName`, which reads a possessive
   as its name since this task) — corroborated (`bare_name_by_samples`);
4. otherwise uncorroborated (`bare_name_uncorroborated`).

Uncorroborated is not "no name signal", and the branches are ordered so that no
case ends less masked than it needs to: a value signal at
`validatorThreshold` decides the column as it does beside any name the values
disagree with; a strong minority hit (`sig.strongHit`) keeps the old name-alone
`possible` and the line names the hit; a lower rule that also matches
(`matchColumnAfter`, so `content_name` is `free_text`) decides the column; and
only with none of those is the column recorded at `low`, under `person_name` or
a sub-threshold value signal's own category, with `work.nameUncorroborated`
set. `low` is what the neighbouring-column rule's first arm raises beside a
`likely` column, and that is deliberate: a personal column in the same table is
evidence about this table. `sameColumnName` does not raise it
(`nameUncorroborated` is skipped as a target): `users.name` holding people is
no evidence about `tags.name`, and without the skip every uncorroborated
`name` column in a schema with one masked `name` came straight back —
`TestBareNameNeedsCorroboration`'s same-name check fails with the skip
removed.

**The threshold** is `validators.go`'s `nameCorroborationThreshold` and its
comment has the calibration: label lists 0% to 12% dictionary words, lists of
people 30% to 100%. It is far below `weakThreshold` because the name is half
the evidence already.

**Measured.** `TestBareNameTruthSetsWithSamples` reruns both DB-free truth
sets with samples on their bare-name columns (the plain runs above leave every
bare name unproven, so they cannot see this rule): pagila with its own
category and language names reads precision 0.842 / recall 1.000 (0.762 /
1.000 without samples), and the held-out names 0.979 / 1.000 (0.959 / 1.000);
every labelled-personal bare name there is corroborated by its table alone,
with samples the dictionary does not carry. Over the ten torture schemas,
classified before and after, 18 columns moved from masked to copied and none
the other way, none personal; docs/TORTURE.md and THREAT_MODEL.md T1's T-0313
amendment list them. `TestBareNameNeedsCorroboration` pins both directions,
including the residual: a bare `name` of a table not named for people, holding
names in a script the dictionary lacks, is copied — THREAT_MODEL.md T1's
stated residual, as a non-Latin name already was. It is pinned so a change
that closes it is noticed and the residual text moves with it.
`testdata/regressions/044` is the end-to-end half.

**What is not changed.** The multilingual bare words for "name" in the
`person_name` row — `nombres?`, `naam`, `navn` — still mask on the name alone:
the goal named the English word, and narrowing another language's is a recall
change of its own, filed rather than made here (T-0352). The JSON-leaf path
(`jsonLeafIsPersonal`) reads `match`, which honours `unless` but not
corroboration: a leaf has no table and no samples of its own, so a `name` key
still marks a document personal, and a `file_name` key no longer does (the
document is masked on its type either way).


## The entropy signal does not read application values as secrets (T-0315)

Dogfood session 1 masked file names, MD5 and file-fingerprint digests, Rails
single-table-inheritance `type` class names, a formatter, a component name, an
environment variable's name and two columns of one and four values as
`credential`, "N/N samples look like secrets", to the fixed literal, which in
a `type` column raises on every row the application loads. Four pieces, and
`internal/textsig`'s own T-0315 section has the value half:

- **`textsig.LooksSecret` refuses four shapes** (a file name carrying no
  dictionary word, a 32/40/64-character hex digest, a `::`/`.` namespaced
  identifier (a dotted one only when camelCase and name-free), an
  environment variable's name with no dictionary word). Nothing here is needed for
  them: the validator entry calls `LooksSecret` as before.
- **`entropyExemptNames`** (`validators.go`): a column whose normalised name is
  `type`, `klass` or `component_name` is scored with `withoutSecrets(vs)`, the
  list minus the entropy entry, in `state.base`. Exact names only: a
  `token_type` is decided by the credential name rule before this is read,
  and a polymorphic `commentable_type` is not in it (its values are class
  names too; namespaced ones are spared by the value guard, and an
  un-namespaced one is still read by the entropy check).
- **`minSecretSamples`** (5): in `bestSignal`, the credential entry reaches the
  strong branch only over five samples or more. Below that it falls through
  to the weak branch (at `minSamples` or more it may record `low`) and the
  validators after it are still asked. Every other validator keeps T-0058's
  fail-closed reading below `minSamples`.
- **`namedFileNames`** (`validators.go`): when at least
  `nameCorroborationThreshold` of a column's samples are file names whose stem
  carries a dictionary word, `bestSignal` swaps the credential entry's check
  for "`LooksSecret`, or any file name", with its own phrase
  (`phraseNamedFiles`, `reasons.go`). Without it, the first measurement copied
  rails-activestorage's `active_storage_blobs.filename` (a truth-set true
  positive) and mastodon's `media_attachments.file_file_name`, whose names
  the dictionary holds in 72% and 51% of rows: `LooksSecret` keeps a
  name-bearing file name, but the ratio over the column fell under
  `validatorThreshold`.

`rules.yml`'s credential row gains `key_?hash(es)?` in the same change:
plausible's `api_keys.key_hash` is SHA-256 hex with no name signal and was
masked on its values alone before the digest guard.

`internal/verify/validators.go`'s credential entry mirrors the three names
(`secretExemptColumns`, matched through `snakeColumnName`), the floor
(`secretMinNonNull`) and the file-name share (`namedFileShare`, `fileTally`);
the lists are kept in step by hand, like the rest of that file.

**Tests.** `secret_shapes_test.go`: `TestEntropyValidatorSparesApplicationShapes`
(each dogfood shape spared, five real secrets, a column of owner-named file
names and a neutrally named column of dotted handles still masked; each of the
last two sits in a table of its own so a neighbouring credential column cannot
be what masks it) and
`TestEntropyValidatorNeedsFiveSamples`. `testdata/regressions/045` pins both
directions through a real run.

**Measured.** THREAT_MODEL.md T1's T-0315 amendment: over the ten torture
schemas 29 columns moved to copied and 3 to masked, none of the 29 personal;
the three hand-labelled truth sets keep recall at 1.000 (precision 0.641 →
0.647, docs/TORTURE.md), and `TestPagilaPrecisionAndRecall`,
`TestFiftyNamesFromThreeSchemas` and `TestBareNameTruthSetsWithSamples` read
as before, since they sample no credential-shaped value.

## The card entry wants the shape of a card (T-0316)

Dogfood session 1 masked eight identifier columns as `free_text`, "a strong
validator hit below the category threshold", on values that passed only the
Luhn check digit. The card entry in `baseValidators` reads
`textsig.ValidCard` (a known issuer prefix as well as the check digit), and
`state.base` swaps it for `textsig.CardShape` (the issuer's own length too)
through `withCardShape` when `identifierNamed` says the normalised column name
ends in `id`, `number`, `num`, `no`, `nr`, `version`, `ref` or `reference`
(`identifierNameWords`, `validators.go`). Only the check changes: category,
phrase, strength and order are as before, so the reason line reads the same
when a column is masked. `internal/verify/validators.go` carries the same word
list as `cardIdentifierWords`, by hand, and splits both of its card entries on
it. `jsonLeafIsPersonal` still reads the bare `textsig.ValidLuhn`, unchanged.

`card_shape_test.go`'s `TestCardEntryNeedsTheShapeOfACard` pins it: a
`version` column of migration timestamps, a `customer_id` and a neutrally named
column each with one check-digit-only value are copied (each masked before the
change, checked by reverting it); an `invoice_number` holding the Visa test
card and a neutral column holding a Visa-prefixed value of an unissued length
are still masked. **Measured:** THREAT_MODEL.md T1's T-0316 amendment — no
column of the ten torture schemas moved, because none samples a value that
passes the check digit.

## A version is not a network address, and a key is not a phone number (T-0317)

Dogfood session 1 found two more coincidental value shapes. `last_player_version`
held dot-separated integers of the same shape an IPv4 address is ("1.2.3.4"):
93 of 168 samples passed `textsig.ValidIP`, below `validatorThreshold`, so
`bestSignal`'s `strongHit` branch (T-0136, above) masked the whole column
`free_text`, "a strong validator hit below the category threshold". Separately,
`license_key` held ten-digit samples, 8 of which cleared one of
`phoneGuessRegions`' numbering plans, and `guessedPhoneColumns` masked it
`phone` beside a personal neighbour (T-0221, above) — the wrong shape for a
key, whatever some numbering plan makes of its digits.

Both are name-based vetoes over the *value* signal, the same shape T-0316's
`withCardShape` already is, and both live in `validators.go` beside it:

- **`networkIDVetoed`** answers whether any underscore-separated token of a
  `normaliseName`'d column name is `version`, `build` or `release`. Unlike
  `identifierNamed` (T-0316), which asks only about the last token because a
  card check is about what the whole value is, this checks every token —
  `app_version_code` carries the word in the middle. Nothing about the
  column's other signals changes — a version column that also carries a
  genuine email or phone value is still decided by that. `bestSignal` takes
  the already-vetoed `vs` and passes it straight through to `compositeSignal`
  and `byteaTextSignal`, so a composite or bytea column named `version` gets
  the same treatment there too, by construction and not by a second edit;
  `jsonLeafIsPersonal` is the one caller this does not reach, because it
  reads `st.pack`'s bare `matchColumn` over a JSON leaf key, which has no
  column name of the *document's* own to veto against and never took a `vs`
  argument at all.
  - **`withoutNetworkID`'s first landing dropped the `network_id` entries (IP,
    MAC) from the validator list outright, unconditionally on the values, and
    a review round on this same task found that goes further than the goal:
    it also leaves a column unmasked when *all* of its values are genuine
    addresses** (finding 2, T-0317 review round). Probed with a `build_host`
    column of five real public IPv4 addresses in a table with no certain
    neighbour: masked `network_id` before this change, "nothing recognised"
    and unmasked after — any column with `version`, `build` or `release` as a
    token and genuine addresses in it (`build_agent_address`,
    `release_server`, `client_version_origin`) reached the target unmasked on
    a name coincidence alone. The fix is narrower: `withoutNetworkID` now
    clears the two `network_id` entries' `strong` field instead of removing
    them. `bestSignal`'s ratio branch (`ratio >= validatorThreshold`, the
    "this column really is mostly addresses" case) does not read `v.strong`
    at all, so it is unaffected — a column that genuinely clears the
    threshold on IP or MAC still masks `network_id`. Only the `strongHit`
    branch below `validatorThreshold` (T-0136 finding 7) reads `v.strong`, so
    only the coincidental-minority path — the dogfood shape, 93/168 — is
    withheld now, which is what the goal actually described. Neither of the
    T1 measurements moved (see Measured, below); `build_host`'s shape is not
    in either truth set either.
- **`phoneGuessVetoed`** answers the same question against `key`, `code`,
  `license`, `serial` and `token`. It gates `base`'s own computation of
  `w.guessedPhone` (the same block `isCharacterFamily` and `silencedByType`
  already gate, T-0221's own section above), so a vetoed column's values are
  never even offered to `guessedPhoneHit`, and `guessedPhoneColumns` has
  nothing to raise. It is deliberately **not** read anywhere near the
  configured-region entry `buildValidators` splices into `state.validators`
  (T-0221's `phraseE164Region`): a `--phone-region` an operator configured is
  trusted evidence on the same strong footing as the international-only
  entry, and this task's goal is about the *guessed*, corroboration-only
  fallback alone, not about an operator naming a region and handing the tool
  a `license_key` column of the phone numbers their own support line prints
  on printed licence cards. `decide`'s ordinary name-match branch (a column
  whose own name matches `rules.yml`'s phone pattern) is unaffected for the
  same reason `guessedPhoneColumns`' own comment already gives: that branch
  masks the column before `guessedPhoneColumns` ever sees it, veto or not.

**Neither veto removes masking outright.** A vetoed column beside a `certain`
personal neighbour, with no other signal, is still swept into `free_text` by
the neighbouring-column rule's second arm (T-0311, above) unless one of that
arm's own guards spares it — the veto withholds one specific, wrong category,
not the decision to mask at all. `t0317_test.go`'s
`TestKeyNamedColumnIsNotMaskedAsGuessedPhone` pins exactly that: `license_key`
and `serial_number` are still masked, `free_text`, beside their table's
`email` column, and the assertion is on `Category != CatPhone`, not on
`Masked`. `TestVersionNamedColumnIsNotMaskedAsNetworkID` pins the other
direction, where nothing else in the fixture masks the column at all, and
both tests carry a same-shaped control column with no veto word in its name
to prove the fixture would otherwise mask the way the dogfood report
describes.

**Measured:** `TestPagilaPrecisionAndRecall` and `TestFiftyNamesFromThreeSchemas`
are unchanged by this diff (precision 0.762/0.959, recall 1.000/1.000) —
neither hand-labelled truth set carries a version/build/release or a
key/code/license/serial/token column, so this is the floor showing no
personal column newly missed, not a claim the torture corpus was measured
(nothing under `testdata/` changed in this task).

**Owed:** THREAT_MODEL.md T1 is outside this task's paths and does not yet
carry this amendment — tracker T-0359 carries the edit, the way T-0357 and
T-0358 already do for T-0315 and T-0316's own owed sentences elsewhere.

**Owed: `internal/verify`'s second net has no matching veto** (finding 1, T-0317
review round). T-0315 and T-0316 each mirrored their name veto into
`internal/verify` (`exemptColumns`, the `columns` func) in the same landing;
this task's paths did not reach that package, so the mirror was not done and
no task filed either. `internal/verify/validators.go`'s `network_id` entry
(`ValidIP || ValidMAC`, `strong: true`) has no gate of its own, so a column
like `last_player_version` — which this change now leaves unmasked, with no
certain neighbour (`masked=false`, `cat=none`) — still carries its coincidental
IP-parsing minority into the loaded target, and `secondnet.go`'s `val.strong`
branch fails the column on that single hit, refusing the whole run at exit 9.
Not a data leak (fails closed), but the dogfood run goes from over-masked to a
post-load refusal with no green path short of `--unmask`, and the two nets
disagree in a way this package's own T-0055 note above argues against.
Tracker **T-0360** carries the mirror.

**This task's own review round (2026-09-24) reopened this same owed note as
finding 1 and asked to hold the merge until T-0360 lands, or move T-0360 out
of E9/Later into the current phase's epic (E6) so it is scheduled with this
change rather than after it.** Neither is something this task's own paths or
role can do: `internal/verify/` is outside `internal/classify/`,
`internal/textsig/`, `ARCHITECTURE.md`, `testdata/` and `docs/`, and
`lazyslice-tracker`'s "Who may do what" restricts a developer or reviewer
agent to filing into E9 — `move` (re-homing a task into another epic) and
holding a merge are the orchestrator's calls. T-0360 was filed with the fix
already spelled out (the `columns`-based gate on verify's `network_id` entry,
mirroring `networkIDVetoed`, plus the verify-level test) rather than only
"do the mirror"; the sequencing decision the review round asked for is
recorded here for whoever next holds the tracker.

## A yml raise on a key reaches its FK children (T-0364)

`applyPrior` runs after `keyChildren` and `foreignKeys`, so a raise — a
`lazyslice.yml` pattern or column entry, and what `internal/core` makes of
`--mask` and a committed `mask:` block — on a natural key used to leave its
children as the first sweep found them: the parent's values in clear on the
child side (THREAT_MODEL.md T1) and two maskers, or one and none, across one
join (T8). A uuid or integer child `markNeverMasked` had exempted as a key
column could not be raised either, so `internal/core`'s stop-gap refusal of
such a run (T-0319's review round) had no `--mask` that cleared it.

- **`Classify` runs `foreignKeys()` again after `applyPrior`, only when a raise
  applied** (`state.priorRaised`, set from `raiseFromConfig`'s new `bool`).
  Same edges, same rule: the child converges on the parent's category and
  masker, and a key exemption is lifted exactly as for a key the classifier
  masked itself. Gating on a raise keeps every run with no prior, or with one
  that raised nothing, byte-identical — including a cycle of disagreeing
  masked keys the first sweep's cap cut off, which a second sweep would move.
- **This was chosen over moving the raise ahead of `keyChildren`.** The raise
  was not moved because `keyChildren` is the one pass that masks less: run
  after a raise, or rerun after one, it hands the key exemption back to a
  column the file itself raised whenever that column's parent is an exempt
  key. So only propagation is rerun; `keyChildren` still runs once, before
  the prior, and a raise still skips a column that is `neverMask` (a key the
  yml cannot contradict), as `applyPrior`'s own comment says.
- **A child the prior raised under another category converges on the
  parent's.** Before, the later raise won and the join broke; now the
  child's category is the parent's, and `internal/core`'s `checkMasks`
  already refuses a `--mask` whose category did not come out as named.
- **A child with its own honoured opt-out is skipped** (`w.unmasked` in
  `propagateKeys`): it is copied whatever its category says, and its Source
  and reason keep naming the opt-out. `unmasked` is set only in
  `applyPrior`, so the first sweep is unchanged.
- **The second sweep names an edge once**: when a raise lifted only the
  parent's confidence, the child's `fk_propagation` fragment is not appended a
  second time.
- **`sameColumnName` is not rerun.** The task is FK children; a raise does
  not export to same-named columns elsewhere, as before.

`TestAYmlRaiseOnANaturalKeyPropagatesToItsChildren` (`t0364_test.go`) pins a
uuid key chain (the exempt child and a grandchild, grandchild edge listed
first), a text key and an opted-out child; it fails with the rerun removed.
`internal/core` keeps its `unmaskedChildren` refusal as the backstop for a
child propagation cannot mask (its type refuses the parent's category, or
the edge's parent column is not a key or unique): after the rerun it fires
only for those.

## The per-leaf half of a JSON decision (T-0272)

`base` sets `Decision.LeafKeys` on every `json`/`jsonb` column (never an
`hstore`, which transform collapses, nor an array of either) from
`jsonKeyCategories` (`validators.go`): every object key in the samples, at any
depth up to `jsonMaxDepth` and inside arrays of objects, mapped to `match`'s
category for its normalised name or `CatNone`. `internal/transform` masks or
**copies** each leaf from that map (ARCHITECTURE.md §4's T-0272 amendment,
the maintainer's T-0143 decision) and `internal/verify` reads it back, so
this package is where the name rules reach a leaf without either of them
importing this package.

- **It reads the raw samples, not `values`.** pgx hands a jsonb sample back
  decoded, as a `map[string]any` or `[]any`, and `scalars`/`asText` render
  neither, so the string path sees nothing of an object document from a real
  database. `TestAJSONColumnsDecisionCarriesItsLeafKeys` samples decoded maps
  for that reason.
- **A key that is itself an email, phone or Luhn-valid number is left out**
  (`strongKeyShape`, the same three textsig validators transform's
  `keyCategory` uses, the phone one under the run's `--phone-region` since
  T-0394): transform masks such a key, so it is a value, and leaving it out
  means every leaf beneath it is masked on both sides.
- **The map has a value half for guessed-region phone numbers** (T-0394, the
  JSON red team's A26). `guessedPhoneLeafKeys` (`validators.go`) finds, in
  `base`, the keys the map calls `none` whose string leaves across the
  samples clear `guessedPhoneHit`'s own test (at least `minSamples`, at
  `validatorThreshold`, some region in `phoneGuessRegions`), a leaf counting
  toward its nearest enclosing key; `guessedPhoneLeafColumns` (pass 3b,
  beside `guessedPhoneColumns` and gated the same way) gives them
  `CatPhone` when `Decision.TableHasLikelyPersonalColumn` holds or the map
  has a key `identifiesAPerson` names. Transform then masks every leaf under
  such a key through the phone masker, and verify's net, reading the same
  map, leaves them to the residual scan. It only turns `none` into `phone`,
  so it only masks more. `phoneGuessVetoed` (T-0317) is not applied to a
  leaf key: it keeps a key-shaped *column* from getting a phone number's
  shape, and a leaf key it would veto was copied before this pass, so
  applying it would only choose copying over masking. `t0394_test.go` pins both halves and
  testdata/regressions/047 and 048 end to end.
- **At most `jsonKeyLimit` (4096) keys**; a key past it is simply unknown, and
  a leaf beneath an unknown key is masked. Which keys are kept is decided over
  the *sorted* set of every key the samples showed (the T-0272 review round,
  finding 3): `collectJSONKeys` only gathers, and `jsonKeyCategories` sorts
  before it cuts, so neither a Go map's iteration order nor the order samples
  arrive in reaches the map
  (`TestJSONKeyCategoriesPastTheLimitIsTheSameEveryTime`).
- **The map is read only for a plain `semi_structured` decision.**
  `pipeline.Decision.LeafMap` returns nil when this package decided the column
  under any other category (a `jsonb` named `medical_history` is
  `special_category` on its name) or when a yml raise decided it
  (`ByYmlRaise`, which `--mask` and a `mask:` block become), so every leaf of
  such a column is masked. The map is still set on those decisions; nothing
  reads it there.
- **A generated column whose samples validate raises the keys it reads**
  (`generated.go`, T-0397, the 2026-09-25 JSON red team's round 1, entry
  24). `base` records `work.generatedValue` — the category of the validator
  that decided a generated column's samples (`bestSignal`'s strong branch,
  never a document family's `jsonSignal`) — and `generatedLeafKeys`, run
  after `sameColumnName` and before `applyPrior`, gives every key the
  expression reads through `->` or `->>` (`generatedReads`: identifiers as
  `internal/verify`'s `exprIdentifiers` reads them, plus the string literal
  right of each arrow) the generated column's own category (its
  `Category`, else the validator's) in the `LeafKeys` of every `json`/`jsonb`
  column of the table the expression names. Only a key the map calls `none`
  moves; a named key keeps its category, and a key the map does not hold —
  never sampled, or a `strongKeyShape` key transform masks as a value — is
  left out, because its leaves are masked already and entering it could put
  the two packages' spellings out of step. It only masks more, changes no
  column decision, and is not fingerprinted (`LeafKeys` is not). A key read
  through `#>`, `#>>` or `jsonb_extract_path` is not raised; `internal/verify`'s
  net still refuses the assembled value there, and tracker **T-0405** owes it. `t0397_test.go` pins the red
  team's table, the no-validation and ordinary-column controls, and the
  parser; testdata/regressions/049 runs it end to end.
- **A rejected name hit is carried on the decision** (`Decision.NameHit`,
  T-0393, the 2026-09-25 JSON red team). `decide`'s `hasName &&
  !nameAccepted` branch records the hit's category on every arm, so a `jsonb`
  `full_name` or `passwords` that its type decided `semi_structured` is not
  mistaken for a document nothing names: `LeafMap` is nil for it and
  `LeafNameCategory` tells transform and verify to mask every leaf under the
  name's category. It changes no column decision (`Category`, `Confidence`,
  `Masked` are what the branch always set) and is not emitted or
  fingerprinted, like `LeafKeys`. A bare `name` hit is carried as it stands,
  without T-0313's corroboration, because the column is masked by its type
  either way; whether a label table's document should need it is T-0396.
  `TestADocumentWhoseOwnNameIsPersonalCarriesTheNameHit` (`t0393_test.go`)
  pins the eight names over testdata/regressions/046's own documents.
- **The table-level log-shaped rule is carried on every column, not only the
  JSON ones** (`Decision.LogShaped`, T-0398, the 2026-09-25 JSON red team's
  round 1, entry 26). `finalise` sets it from `st.pack.logShapedTable(w.table.
  Name)` — the identical call `appendContext`'s "jsonb in a log-shaped table"
  reason fragment already makes, so the field and the fragment can never
  disagree — for every column of a table `rules.yml`'s `log_shaped` regex
  matches, whatever the column's own family. It is the fix for a mismatch
  entry 26 found: `internal/transform` used to keep its own copy of the rule
  (`logTableWords`), an eight-word list matched by splitting the table name on
  `_` alone, with no CamelCase normalisation and no `activity` or `trace` —
  so a CamelCase table (Prisma's default `AuditLog`) or an `*_activity`/
  `*_trace` table was reported "replaced whole" here and walked leaf by leaf
  there, copying every signal-free leaf once T-0272's per-leaf rule made a
  walked leaf's default anything but a mask. `internal/transform`'s
  `maskDocument` and `internal/verify`'s own restatement of the per-leaf rule
  both read this field now (each package's own `leafPolicy`/`policyOf`,
  `logShaped`), in place of `logTableWords`, which is gone.
  `TestLogShapedMatchesTheRulePackWhereverTheReasonLineDoes` (`t0398_test.go`)
  pins the field against the reason line over the CamelCase and `activity`/
  `trace` shapes `logTableWords` missed, and a table the rule does not match;
  `TestLogShapedIsSetOnEveryColumnOfTheTable` pins that it is a table-level
  fact and not gated on family. Like `NameHit`, it changes no column decision
  and is not emitted or fingerprinted; `testdata/regressions/050` runs the
  CamelCase and `*_activity` shapes end to end.
  - **`logShapedTable` ORs the regex against two folds of the table name, not
    one** (T-0398 review round, high finding). The first landing tried the
    regex only against `normaliseName`, whose acronym-boundary rule
    (`needsBreak`'s upper-upper-then-lower case, written for `IDToken` ->
    `id_token`) also splits a trailing lower-case plural off an all-caps
    word: `EVENTs` normalises to `even_ts`, `LOGs` to `lo_gs`, `AUDITs` to
    `audi_ts`, and the same split happens with the word embedded
    (`user_LOGs` -> `user_lo_gs`, `x_LOGs_y` -> `x_lo_gs_y`). None of those
    match the regex's whole-word test, so those tables' jsonb columns were
    walked leaf by leaf instead of replaced whole — the exact T1 leak this
    task exists to close, reopened for the all-caps-plural shape, and the
    deleted `logTableWords` copy (a plain lower-case-and-split-on-`_` word
    match, no case-boundary splitting at all) had caught every one of them.
    `logShapedTable` now also tries `lowerUnderscoreFold` — the same fold
    `logTableWords` effectively was, lower-case the letters and turn every
    other non-name rune into `_`, with no word-splitting on a case
    boundary — and ORs the two matches. Root CLAUDE.md's "when in doubt,
    mask it" makes the direction non-negotiable: a name either fold already
    caught (`AuditLog`, which has no underscore for `lowerUnderscoreFold` to
    split on and needs `normaliseName`'s CamelCase break) must keep matching,
    so the fix adds a second fold rather than replacing the first.
    `TestLogShapedMatchesTheRulePackWhereverTheReasonLineDoes` gained the six
    names the deleted `logTableWords` matched that the first landing's
    single-fold version did not (`EVENTs`, `LOGs`, `user_LOGs`,
    `AUDIT_LOG`, `Events`, `Order_HISTORIES`).
- **`Classification.PhoneRegion` is the region `Classify` ran under**
  (`prior.PhoneRegion`), set for `internal/transform`, whose per-leaf value
  half reads a phone number under it the way `internal/verify`'s net does.
- **It changes no column decision.** `Category`, `Confidence` and `Masked` are
  what they were; the map is read by transform only for a column already
  masked. `TestClassificationIsDeterministic` compares decisions with
  `reflect.DeepEqual` now, because a map makes `Decision` incomparable with
  `==`, and the map is part of what has to come out the same.
- **Not in `Classification.Fingerprint`, and stated so in ARCHITECTURE.md §5
  "Determinism scope"** (the T-0272 review round, finding 3). A change in the
  keys the samples show moves which leaves are copied without printing
  "classification changed". Covering the map would make the fingerprint move
  with the source's data rather than with the rule pack and the plan, and the
  value that reaches the yml is `internal/core`'s `refingerprint`, not
  `fingerprintOf`, so a change here alone would not reach it anyway.

- **A committed yml pins the copied keys** (`leafdrift.go`, T-0404, the
  JSON red team's round 1, entry 28). The bullet above is why this was
  needed: nothing about the map reached the file, so a re-run whose samples
  showed a new key copied its leaves silently and `--strict-schema` passed.
  `leafKeys` runs after `finalise` (so `LeafMap` reads the final `Category`
  and `Source`), and, for a column the prior carries an entry for, deletes
  from `LeafKeys` every key the map calls `none` that the entry's
  `LeafKeys` does not name (`leafKeyListed`: by fingerprint, or by the key
  itself when identifier-shaped), recording it on
  `Classification.LeafDrift`. Transform and verify then read it as a key
  the samples never showed. A column not in the prior is column drift and
  untouched; a listed key keeps this run's verdict, so a listing never
  copies a key this run masks. It only deletes, so it only masks more.
  `Decision.RecordedLeafKeys` is what a run from the written file may copy,
  spelled by `leafKeySpelling` (the key when `identifierShaped` and its
  column holds at most `leafKeyNameLimit` keys, a `sha256:` fingerprint
  otherwise, so a value used as a key never reaches the yml): a fresh
  column's copied keys; an entry's own list, carried forward as it stands,
  when it has one; every copied key, drift included, only when the entry has
  no `leaf_keys:` at all (a pre-T-0404 file). **Review round (2026-09-25):**
  the first draft recorded the drifted keys on every run, and every
  non-strict run rewrites `--config`, so the pin lasted one run; and
  `identifierShaped` let a UUID starting a-f, a dotted handle and a
  digit-laden token through as themselves. Now a key joins an existing list
  only by hand, digits are allowed only as a trailing run of one or two,
  dots never, all-hex keys of eight letters or more and keys past 32 bytes
  are fingerprinted, and a column of more than 64 keys (a map keyed by
  data, where `jsmith` is a value) is fingerprinted whole. Such a column's
  list is bounded only by `jsonKeyLimit`, and each new key is a drift line on
  every run; folding those lines and capping the list is **T-0422**. The
  spelling is this package's alone. `t0404_test.go` pins it.

**Found while here, not fixed (outside this task's brief):** `jsonSignal` —
the column-level leaf signal that raises a JSON column from `possible` to
`likely` — reads `values`, which is the string path above, so on a real
database it sees no object document at all and a JSON column is decided on
its type alone. The column is masked either way; what is lost is `likely`,
which the neighbouring-column rule and `byteaInPersonShapedTable` count.

