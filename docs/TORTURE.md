# Torture testing

Ten real open-source PostgreSQL schemas, at a pinned commit or image digest,
each filled with generated rows, sliced from its most-connected table into a
second container, and put through the invariants.

```
make torture
```

Nine of the ten snapshot cleanly **with twenty-seven flags between them —
nineteen `--unmask`, seven `--skip-table` and one `--key`**. That whole sentence
is the result, and the split is part of it rather than a footnote: `--unmask`
copies a column of personal data into the target verbatim and `--skip-table`
drops a table, so the two are not interchangeable evidence and the total is
never quoted here without them. A stranger pointing lazyslice at GitLab types
three `--unmask` flags before it runs. The tenth, Mastodon, refuses at exit 13
for a reason ARCHITECTURE.md §11.1 states, and the suite asserts that refusal by
name. Between them the ten found **twelve defects**, every one of which is now a
fix in `internal/`, `mask/` or ARCHITECTURE.md §5, and **five more findings**
that were filed rather than fixed because the change belonged somewhere
T-TORTURE could not reach. Four of those five have since been fixed as well
(T-0100, T-0103, T-0104, T-0105) and are described where their fixes are; the
one still open is T-0102.

It was forty-five flags — thirty-seven `--unmask` — when this file was first
written. Eighteen of those `--unmask` flags were one defect, T-0098, and the
`credential_unique` masker (`mask/gen_credential.go`) removed the need for every
one of them; T-0112 stripped them one schema at a time and re-ran the suite,
which is the measurement above. **`make torture` exits 0** (T-HARD-C,
2026-09-09), which it did not between T-0098's fix and T-0113: two of the eight
regressions, `004` and `007`, still asserted the exit-12 refusal that fix
removed. Both are re-cut to `expect: ok` plus a `unique-masked:` assertion that
reads the columns out of the target, so what they pin is the masking and not
merely the exit code.

One thing the nine clean runs do **not** say, measured below: supabase-auth's
classifier misses **one** of the fifty columns the hand-labelling calls personal
— `refresh_tokens.parent`, a quarter populated in this fixture — and it is
copied into the target in cleartext under exit 0 (recall 0.980). It missed **ten** at recall
0.800 when this file was written; T-0104's name rules took eight of them and
T-0121's `public_key` decision the ninth, and the re-measurement is in the truth
sets below. A second finding used to stand beside it — `credential`'s only
masker had a domain of one — and that is the T-0098 defect the count above no
longer carries.

The fixtures are `testdata/torture/`; the catalogue that runs them is
`internal/invariants/torture_catalogue_test.go`; the reduced defects are
`testdata/regressions/`.

## The ten

Times are one `make torture` run on a 2024 MacBook Pro, Docker Desktop 28.5.2,
`postgres:16` (and `pgvector/pgvector:pg16` for Discourse), each schema on its
own pair of containers. They are dominated by container start and schema load,
not by the snapshot: Discourse's 370 tables take about six seconds to create and
the slice itself takes under one.

**The result is the same every run, and it was not always.** The suite writes a
fixed masking key into each run's working directory (`fixedSecret`,
`internal/invariants/harness_test.go`) instead of letting every invocation fall
through to `mask.NewKey()`. Under a random key the residual scan (§6 item 3) is
a probability rather than an answer — a masked value collides with a value the
source still holds in that column now and then — and `TestTortureSchemas/calcom`
failed about one run in five on `public."Attendee".name`, exit 9,
`verify.refused.residual`. Measured before the fix: two of three whole-suite
runs failed, both on calcom, and calcom on its own failed two of eleven.
Measured after it: six consecutive `make torture` runs, six passes, and the
same table below. (Those six were measured before T-0112 removed eighteen
flags. The table below is one run of the tree as it now stands; a second run of
the same tree gave the same result and the same flag counts, with each schema's
time moving by up to half a second, so read the times as the shape of the run
rather than as a benchmark.) The fixtures carry the other half of
the fix — a generated name may not be a word in the maskers' own lists, which is
the same rule `testdata/torture/README.md` already stated for `example.com`,
RFC 5737 and `555-01XX` — so the collision is impossible rather than merely
unlucky under this key.

| Schema | Tables | FKs | Columns | Root | Flags | Time | Result |
|---|---:|---:|---:|---|---:|---:|---|
| [rails-activestorage](../testdata/torture/rails-activestorage/README.md) | 7 | 3 | 36 | `public.active_storage_blobs` | 1 | 4.0 s | clean |
| [django](../testdata/torture/django/README.md) | 10 | 9 | 44 | `public.auth_user` | 1 | 2.3 s | clean |
| [supabase-auth](../testdata/torture/supabase-auth/README.md) | 27 | 24 | 271 | `auth.users` | 1 | 3.1 s | clean |
| [plausible](../testdata/torture/plausible/README.md) | 42 | 40 | 294 | `public.sites` | 2 | 2.7 s | clean |
| [gitlab](../testdata/torture/gitlab/README.md) | 43 | 116 | 968 | `public.namespaces` | 3 | 4.5 s | clean |
| [metabase](../testdata/torture/metabase/README.md) | 100 | 130 | 892 | `public.core_user` | 4 | 3.6 s | clean |
| [calcom](../testdata/torture/calcom/README.md) | 102 | 179 | 1,092 | `public.users` | 1 | 4.4 s | clean |
| [mastodon](../testdata/torture/mastodon/README.md) | 118 | 156 | 1,018 | `public.accounts` | 3 | 2.5 s | **exit 13** |
| [odoo](../testdata/torture/odoo/README.md) | 204 | 622 | 2,048 | `public.res_users` | 3 | 5.4 s | clean |
| [discourse](../testdata/torture/discourse/README.md) | 370 | 29 | 3,372 | `public.users` | 8 | 7.5 s | clean |
| **total** | **1,023** | **1,308** | **10,035** | | **27** | **40.0 s** | 9 clean, 1 refused |

The whole `make torture` target is 57 seconds: the 40.0 above, plus 15.6 for the
eight regressions and a second for the two catalogue guards and the image check.
That is the T-HARD-C run, the first one in which every one of the four
`TestTorture*` functions reported `--- PASS`.

"Clean" means exit 0 with every check in `TestTortureSchemas` passing: the
tables the slice had to reach hold rows and, where a correct run cannot reach
them all, fewer rows than the source; every foreign key in the target resolves
and every source edge between two present tables was recreated (I1); the source
is unchanged in rows and in catalog (I4); the counted root holds exactly
`--take` rows (I6); and no email address or phone number of the source's survives
anywhere in the target (the grep half of I2).

**"Flags" is the number a first run demanded and named**, counting every flag
of any kind: of the twenty-seven, nineteen are `--unmask`, seven are
`--skip-table` (plausible 1, metabase 2, discourse 4) and one is `--key`
(discourse). It is the honest headline number of this exercise: a stranger
pointing lazyslice at GitLab types three `--unmask` flags before it runs. The
longest reason of the nineteen is `auth.mfa_factors.friendly_name`: it is under
a *partial* composite unique index that admits none of its rows, and
`d_required` is computed over the whole table's row count anyway — the
over-estimate ADR-011 clause (b) states as the rule and ARCHITECTURE.md §5
carries. `internal/invariants/torture_catalogue_test.go` carries the whole
reason beside the flag, and `TestTortureCatalogueMatchesTheFixtures` counts the three kinds
so this paragraph cannot drift from them again.

**Re-measured after T-0136** (docs/reviews/2026-09-09/REVIEW.md finding 7:
`internal/classify`'s `bestSignal` now masks a proven column as `free_text` on
a single strong-validator hit below the category threshold, and
`internal/verify`'s second net now fails a strong hit at any column size, not
only below `minValues`). Neither change touched a name signal or a ratio at or
above `validatorThreshold`, so no torture schema's first run demanded a flag it
did not already carry: the count is still twenty-seven — nineteen `--unmask`,
seven `--skip-table`, one `--key` — `TestTortureCatalogueMatchesTheFixtures`
passed against the unchanged catalogue, and all ten schemas still resolve the
way the table above records. `testdata/regressions/012-single-strong-hit-in-a-
mostly-plain-text-column.sql` is finding 7's own reduction, checked in rather
than only measured here.

**It was forty-five, and eighteen of the thirty-seven `--unmask` flags went in
one change** (T-0112, after T-HARD-A landed `credential_unique`): supabase-auth
6, gitlab 8, mastodon 2, calcom 1, discourse 1. Per schema the flag count fell
7 → 1, 11 → 3, 5 → 3, 2 → 1 and 9 → 8. What the re-run then measured, column by
column, is worth stating exactly, because "the flag is gone" and "the column is
masked" are not the same claim:

* **Four columns now hold masked values that used to need a flag** —
  `auth.refresh_tokens.token`, `auth.users.confirmation_token`,
  `auth.users.recovery_token` and
  `public.user_security_keys.credential_id`. Every non-null value in the target
  carries `credential_unique`'s `lazyslice-invalid-` prefix, and the distinct
  count equals the row count: 136 of 136, 100 of 100, 100 of 100, 100 of 100,
  no collisions.
* **Two hold the empty string**, which `mask.Apply` passes through by §6 item 6:
  `auth.users.email_change_token_current` and
  `auth.users.reauthentication_token` are `''` in every row of the fixture, so
  there is nothing to mask and nothing that leaks.
* **Ten are NULL in every row of their fixture** — calcom's
  `"Booking".oneTimePassword`, all eight of gitlab's, and
  `auth.users.email_change_token_new`. Their flags were demanded because
  `d_required` is computed over the whole table's planned row count whatever the
  values are (the same over-estimate ADR-011 clause (b) states as the rule, in
  ARCHITECTURE.md §5), not because a credential was ever going to be copied.
* **Mastodon's two were never reached at all.** That run refuses at exit 13
  before the plan runs, so removing `public.users.confirmation_token` and
  `public.users.reset_password_token` leaves the refusal unchanged and proves
  nothing about masking; it is the count that is now honest, not a new
  measurement.

So the eighteen are gone and the number a stranger types drops by 40%, but the
end-to-end evidence that `credential_unique` masks a real column is four
columns, not eighteen. The rest of its evidence is unit-level:
`mask`'s `TestAUniqueCredentialColumnIsCarriedRatherThanRefused` (a
`varchar(255)` unique credential column at 20,000 rows, no collisions) and
`internal/plan`'s `TestUniqueCredentialColumnEscalates`.

### The tenth

```
✗ public.accounts.id depends on timestamp_id, a function lazyslice does not
  recreate in the target
lazyslice: target.schema.not_recreatable.function: public.accounts.id depends on
  timestamp_id, which lazyslice does not recreate
```

Exit 13. Mastodon's primary keys default to `timestamp_id('accounts')`, a
plpgsql function the application installs; ARCHITECTURE.md §11.1 does not
recreate functions and says why there is no flag that drops the default and
carries on ("the application's first `INSERT` is the point of the tool"). The
message names the table, the column and the dependency, the code is in
`docs/ERRORS.md`, and the exit code is one a CI job can branch on. Before this
work it was exit 1, `run.refused.internal`, "run with --debug", with the real
code buried in the message — which is
`testdata/regressions/002-function-default-refusal-uncoded.sql`.

GitLab reaches the same refusal on two objects, and its subset removes them
rather than spending a second of the ten on the same finding;
`testdata/torture/gitlab/README.md` names both and says so.

## Defects found and fixed

Twelve. Eight of them have a file in `testdata/regressions/` — the smallest
schema that still shows it — written before the fix, and each of those files is
run by `make torture`. The other four are below the table and none of them
reduces to a schema: T-0098's fix is in `mask/`, which is ADR-006's own module,
and has its unit tests there; T-0097, T-0099 and T-0101 stood in "Found and not
fixed" until T-HARD-A and ADR-011 reached the files this exercise could not
write.

| # | Regression | Found by | What was wrong | Fixed in |
|---|---|---|---|---|
| 1 | `001-unique-index-masking-collision.sql` | django, rails-activestorage, supabase-auth | ARCHITECTURE.md §5's unique-index domain rule was never called. `mask.Pick` implemented it; nothing invoked it, and `internal/transform` used the rule pack's default masker whatever the column's constraints were. A masked column under a unique index collided **in the loader**, after every row had moved: `duplicate key value violates unique constraint "auth_group_name_key" (SQLSTATE 23505)`. | `internal/plan/unique.go` (new): after the walk, every masked column under a unique index is held to `d_required = n²/2ε`; the widest generator for the category is chosen and written back onto the decision, or the run refuses at exit 12 under the new code `plan.refused.unique_domain`, printing d, d_required and §5's three escapes. |
| 2 | `002-function-default-refusal-uncoded.sql` | mastodon, gitlab | §11.1's exit-13 refusal arrived as exit 1, `run.refused.internal`, "run with --debug", with its own code printed *inside* the message of the wrong one. `core.asStop` converts five stage refusal types and `*ddl.Refusal` was a sixth that nothing converted. | `internal/core/names.go`: the missing case, plus a message that does not print the code twice. |
| 3 | `003-composite-unique-index-is-not-a-unique-column.sql` | rails-activestorage, supabase-auth, calcom, gitlab, mastodon | `Decision.UniqueIndex` was set for **every** column of every unique index. Once defect 1's check started reading it, five schemas refused at plan over columns that cannot collide — ActiveStorage's `name`, the literal `'cover'`, one of four in `(record_type, record_id, name, blob_id)`. | `internal/classify/classify.go`, `indexKeys`: a column is raised when it carries the uniqueness *alone*. |
| 4 | `004-composite-unique-index-all-masked.sql` | django | The other side of 3. `django_content_type` is `UNIQUE (app_label, model)` with both columns masked, and nothing was left to hold the tuple apart: `duplicate key value violates unique constraint "django_content_type_app_label_model_76bd3d3b_uniq"`. | `internal/classify/classify.go`, `raiseCompositeUnique`: a composite index — total, partial or expression — raises its masked columns unless an unmasked key column's sample has no repeats, where "no repeats" requires the column to have been *sampled*: a key column with no samples in a table nothing could be sampled from holds nothing apart, while one with no samples in a table that *was* sampled is NULL in every row and holds the tuple apart on its own (calcom's `Role_name_teamId_key`), unless the index is `NULLS NOT DISTINCT`. §5 now states the composite rule, as ADR-011 clause (a); the approximation and its known imprecisions are recorded there. |
| 5 | `005-array-of-extension-type-not-registered.sql` | plausible | `monthly_reports.recipients citext[]` — the addresses a report is emailed to. `citext` is a **base** type an extension installs, so it is in none of `Schema.Enums`, `Domains` or `Composites`, nothing registered it, and pgx could not build a codec for `_citext`: the binary `COPY` wrote nonsense and the server answered `08P01`. A scalar `hstore` column, found in the same file, could never be loaded at all. | `internal/pg/types.go`: the extensions the source depends on are resolved to their own base types on the target and registered — string-category ones with `pgtype.TextCodec`, `hstore` with pgx's `HstoreCodec` — before `LoadTypes` builds the arrays over them. |
| 6 | `006-identity-sequence-renamed-table.sql` | metabase | Metabase renamed `group_table_access_policy` to `sandboxes` and Postgres left the identity sequence under the old name. The target's `GENERATED AS IDENTITY` creates `sandboxes_id_seq`; `setval` named the source's, and the run died at `42P01` **after every table had been copied** — the THREAT_MODEL.md T8 outcome the strict-NULL `setval` exists to prevent. `internal/verify` read the same wrong name. | `internal/load/ddl/ddl.go` and `internal/verify`: for an identity column the sequence is resolved on the target with `pg_get_serial_sequence`, with the source's name as the fallback for the ownerless case (pagila). |
| 7 | `007-partial-unique-index-masked-column.sql` | supabase-auth | `CREATE UNIQUE INDEX confirmation_token_idx ON auth.users (confirmation_token) WHERE confirmation_token::text !~ '^[0-9 ]*$'`. A partial unique index was excluded from §5's rule, the masked literal fell inside the predicate in every row, the rows went in and the index would not build over them. | `internal/classify/classify.go`: a single-column partial unique index raises the column too, with `d_required` over the whole table's row count — an over-estimate, and the direction that refuses at plan rather than failing in the loader. |
| 8 | `008-name-hit-on-an-unaccepted-type-drops-the-type-signal.sql` | supabase-auth | **The one that leaked.** `auth.users.raw_user_meta_data` is `jsonb` and holds the identity provider's profile: full name, address, phone number, postal address. Its *name* matched the `free_text` rule, `free_text` does not accept `jsonb`, and the type-conflict branch recorded `low` and never consulted the type signal that would have made it `semi_structured` — so it was copied into the target verbatim, under exit 0. `auth.identities.identity_data`, the same type with no name, was masked. **The name made the column less safe.** I2's grep half found 200 of the source's own addresses in the target. | `internal/classify/classify.go`, `decide`: when a name hit is rejected by type and the samples say nothing, the column's type decides, exactly as it would have with no name at all. `people.email_verified boolean` (`testdata/README.md` trap 19) is unaffected: boolean has no type signal. |

Three smaller changes came out of the review of the eight above rather than out
of a schema, and they are recorded here because they change behaviour: defect 4's
rule was extended to partial and expression composite indexes (Cal.com alone
carries 85 partial composite unique indexes, several over a masked column) and
given the sampled-versus-unsampled reading above; `internal/verify` stopped
reporting `verify.sequence.unowned` for a sequence the *target* has no relation
for and fails the run instead, which is what the `42P01` it replaced used to do
(THREAT_MODEL.md T8); and `make check` gained `vet-tagged`, so the code behind
the two build tags is type-checked and vetted by the release gate rather than
only by this target.

### The ninth: unique credential columns (T-0098)

**`credential`'s only masker had a domain of exactly 1**, so no column under a
unique index that classified as a credential could be masked at all: `d_required`
is n²/2ε (ARCHITECTURE.md §5) and `MaxRows(1)` is zero, so the run refused at
exit 12 under `plan.refused.unique_domain` at *every* row count — "lower
`--take`" was not an escape, and the operator's only two were `--unmask`, which
copies the credential into the target verbatim, and `mapping_file:`. **Eighteen
of the thirty-seven `--unmask` flags in the first measurement of this file were
this one defect**, six of them in supabase-auth alone. An authentication schema
is nothing but unique credentials.

The fix is `credential_unique` (`mask/gen_credential.go`), a second
`CatCredential` generator registered *after* the fixed literal, so `Pick` reaches
it only for a unique column and a non-null non-unique credential column still
becomes `$lazyslice$invalid`. What it emits is not a plausible token: the first
eighteen characters are the constant `lazyslice-invalid-` and only a base32
suffix varies, thirteen symbols where the column has room — 65 bits, which clears
the 2⁶⁴ §5 asks of a generator a unique column escalates to. A narrower column
shortens the suffix and `Domain()` shrinks with it; nothing is truncated.

Two consequences are recorded rather than hidden. The first is that
`testdata/regressions/004-composite-unique-index-all-masked.sql` and
`007-partial-unique-index-masked-column.sql` both reduce a credential column and
both said `expect: exit 12 plan.refused.unique_domain`; both exit 0 with the
column masked now, so `make torture` failed them until **T-0113** re-cut both to
`expect: ok` plus a `unique-masked:` header that reads the columns out of the
target — every value carrying `lazyslice-invalid-`, one distinct value per row —
which is the assertion an exit code alone cannot make. What that costs is
recorded too: `plan.refused.unique_domain` is a real refusal that nothing in
`testdata/regressions/` exercises any more, and **T-0124** owes a reduction that
does. The second is in the "Flags" section above: of the eighteen columns whose
flag this removed, four are actually
masked end to end, two are the empty string, ten are NULL in their fixture and
two are in the run that refuses before the plan.

One more change is not a defect in the pipeline but in the harness, and it is
here because the suite is what measured it: `internal/testutil`'s wait for
Docker's port table was ten attempts over five seconds, which `make torture`'s
forty container starts exhausted about once a run, failing a different schema
each time with a message about a port. It is thirty seconds now, and costs
nothing when the port is there.

### The other three: T-0097, T-0099 and T-0101

Each of these had a row in "Found and not fixed" when this file was written,
because the change belonged in ARCHITECTURE.md or in `internal/core` and
T-TORTURE's paths reached neither. All three have since landed, so they are
defects this exercise found *and* got fixed:

* **T-0099 — §5 said nothing about composite or partial unique indexes**, and
  defects 3, 4 and 7 were all approximations of a rule the architecture had not
  written. **ADR-011** (accepted 2026-09-08) writes it and ARCHITECTURE.md §5
  carries both clauses: (a) a composite unique index raises every masked column
  it covers unless an unmasked key column's sample has no repeats; (b) a
  single-column partial unique index raises its column, with `d_required` over
  the whole table's row count. Clause (b) is the over-estimate quoted twice
  above — now the specified behaviour and the safe direction, not an open gap.
  ADR-011 carries the reversal condition: the agreeing-group statistic that
  would make (a) exact.
* **T-0097 — §11.1's not-recreatable refusal was raised inside `load.Load`.**
  T-HARD-A (`ecc42ae`) moved `ddl.Recreatable` to the top of `internal/core`'s
  `planStage`, before the plan request and before the first key query, which is
  where §11.1 says it is raised. Mastodon and gitlab now pay one introspect
  rather than a whole extract before being told the target cannot be built, and
  `internal/core/recreatable_test.go` pins the ordering.
* **T-0101 — the classification fingerprint was computed before the plan's
  picks.** `internal/plan` writes the escalated masker back onto the decision,
  so a column that became unique changed what it was masked with and did not
  change the fingerprint, and §11.2's "classification changed — masked values
  will differ" would not have printed for it. T-HARD-A recomputes the
  fingerprint after the plan (`classify.Refingerprint`, `core.refingerprint`),
  and ARCHITECTURE.md §5's determinism scope states the consequence: the
  fingerprint now moves with `--take`, `--depth`, the root and `--skip-table`,
  which is the conservative direction.

## Found and not fixed

One, with a tracker task. It was eight when this file was written, and seven of
the eight are now fixes rather than findings: T-0097, T-0099 and T-0101 went
first (T-HARD-A and ADR-011), then T-0100, T-0103 and T-0104 (T-HARD-B), then
T-0105 (T-HARD-C). Each is described where its fix is, above and below.

| Task | Finding |
|---|---|
| T-0102 | A **text column holding a JSON document** is invisible to §4's JSON rule: the scalar validators do not match a document and the column is copied. |

The four that closed after the first writing of this file, in the order they
landed, because each changes a number quoted here:

* **T-0100 — `textsig.LooksSecret` classified a URL as a credential.**
  `https://home.social.test/users/bea_donnelly1` cleared its entropy test, so
  mastodon's `accounts.uri` and `statuses.uri` were `credential` on every row,
  masked to a fixed literal under a unique index and therefore two of that
  schema's three flags. T-HARD-B excluded a URL from `LooksSecret` and put
  `textsig.ValidURL` into `internal/classify`'s validator list **ahead of** the
  secrets one, so such a column is `online_id`, whose generator has a domain a
  unique column can use. **T-0122 is the other half and landed in T-HARD-C**:
  `internal/verify`'s second net had the credential entry and no URL entry at
  all, so between the two tasks a profile URI that reached the target unmasked
  was seen by neither net — the second net is one of THREAT_MODEL.md T1's two
  blocking controls on classifier recall, and the two packages score
  independently.
* **T-0103 — an array of an extension type.** Fixed in T-HARD-B; defect 5's row
  above carries the loader half.
* **T-0104 — the `credential` and `online_id` name rules missed the spellings an
  auth schema uses.** `auth_code`, `otp_code`, `code_hash`,
  `authorization_code`, `code_verifier`, `code_challenge`, `credential_id`,
  `external_id`, `provider_id`, `extern_uid`. That was every one of the ten
  misses in the supabase-auth truth set below; T-HARD-B widened the two rules
  and the truth set is re-measured against them here.
* **T-0105 — `.golangci.yml` did not lint the torture build tag.** `run.build-tags`
  listed `integration` alone, so the ~820 lines behind `integration && torture`
  were linted by nothing; `make check`'s `vet-tagged` type-checked and vetted
  them but ran no linter over them. T-HARD-C added `torture` to that list. The
  suite was clean under the full linter set on the first run.

One decision was taken rather than a rule widened, and it moves a hand label
below: **T-0121 — a `public_key` column is `credential`** (2026-09-09), masked
to the unusable literal, or `credential_unique` under a unique index. A public
key is published by design, which is the argument the other way, but it is a
stable identifier for exactly one person and nothing a development database does
needs the real one. T-0104 had named it one of two columns that deserve a
decision rather than a pattern; the other, `refresh_tokens.parent`, is still
open and is the one remaining false negative below.

## PII truth sets

Three schemas were labelled by hand, column by column, and the labels compared
against what the classifier decided.

**The labelling rule**, stated so a change can be scored against the same
standard: a column is **personal** if its value identifies or describes a natural
person, on its own or with the row's key — a name, a contact detail, a
credential or secret, an identifier issued to a person by anyone, free text
people write about people, or a document holding any of those. It is **not
personal** if it is a surrogate key, a foreign key, a boolean, a counter, a
timestamp, or the application's own metadata. Timestamps are `N` deliberately:
they are personal data in law when attached to a person, and §4 has no category
for them, so labelling them `P` would make every schema's recall a statement
about timestamps rather than about the classifier.

**The prediction** is the classifier's, not the run's: a column counts as
predicted-personal when its confidence reaches `possible`, which is §4's mask
threshold. An operator's `--unmask` does not change the prediction — it is a
decision about a column the classifier flagged, and folding it in would score the
flags rather than the classifier.

Re-measured 2026-09-09 (T-HARD-C), after T-0104's name rules and T-0121's
`public_key` decision, by the method under "Reproducing it" below: each schema
loaded from its pin, sliced with the catalogue's own root, `--take` and flags,
and every entry of the emitted `lazyslice.yml`'s `columns:` block scored against
the labels at the end of this file.

| Schema | Columns | Labelled personal | Predicted | TP | FP | FN | Precision | Recall |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| django | 44 | 9 | 12 | 9 | 3 | 0 | **0.750** | **1.000** |
| rails-activestorage | 36 | 7 | 15 | 7 | 8 | 0 | **0.467** | **1.000** |
| supabase-auth | 271 | 50 | 73 | 49 | 24 | 1 | **0.671** | **0.980** |
| all three | 351 | 66 | 100 | 65 | 35 | 1 | **0.650** | **0.985** |

The first measurement of this table, before T-0104, was supabase-auth 62
predicted, 40 TP, 22 FP, 10 FN — precision 0.645, recall 0.800 — and all three
89/56/33/10, precision 0.629, recall 0.848. django and rails-activestorage did
not move at all: neither schema has a column any of the widened names match.
Recall moved 0.800 → 0.980 on supabase-auth and precision moved *up* with it,
0.645 → 0.671, which is not what widening a rule usually does — the two new
false positives are `flow_state.code_challenge` and
`oauth_authorizations.code_challenge`, the public half of PKCE, matched by
T-0104's `code_?challenges?`, against nine columns that stopped being missed.
T-0121 added exactly one prediction, `webauthn_credentials.public_key`, and it
is a true positive: measured with the same fixture and the `public_key` rule
removed, supabase-auth is 72 predicted, 48 TP, 24 FP, 2 FN, precision 0.667,
recall 0.960.

### django — precision 0.750, recall 1.000

Nothing personal was missed. The three false positives are all the same shape: a
column called `name` that is not a person's.

* `auth_group.name` — "Editors". `varchar(150) UNIQUE`, decided `person_name`,
  and the one flag this schema needs.
* `auth_permission.name` — "Can add user".
* `django_migrations.name` — "0001_initial".

Both of the schema's *hidden* carriers were caught: `django_admin_log.object_repr`
("User: bjorn.haddad1@borealis-works.test") at `likely`, and
`django_admin_log.change_message` at `possible`. Neither column's name says
anything.

### rails-activestorage — precision 0.467, recall 1.000

The worst precision of the three, and it is one validator. Eight false
positives, of which six are `LooksSecret` firing on a value that is merely long
and mixed: a checksum, a `variation_digest`, an ActiveStorage `key`, a
`content_type` of `application/pdf`, and `ar_internal_metadata.key` holding
"environment". The other two are `person_name` on `service_name` ("local") and
on `active_storage_attachments.name` ("cover").

Nothing personal was missed, including the one that matters:
`active_storage_blobs.filename` — `ana-aluko-passport-3.pdf` — is caught at
`likely` with no name signal at all.

### supabase-auth — precision 0.671, recall 0.980

The one with a false negative, and it is the reason this schema is in the set.
It had ten, and nine of them were names no rule covered:

| Was missed | Rows in the fixture | What it holds | Now |
|---|---|---|---|
| `flow_state.auth_code` | 100, column NULL | the OAuth authorization code | masked (T-0104) |
| `identities.provider_id` | 300, populated | the provider's subject id for the person | masked (T-0104) |
| `mfa_challenges.otp_code` | 0 | the one-time code | masked (T-0104) |
| `mfa_recovery_codes.code_hash` | 0 | a recovery code | masked (T-0104) |
| `oauth_authorizations.authorization_code` | 0 | the authorization code | masked (T-0104) |
| `oauth_client_states.code_verifier` | 0 | the secret half of PKCE | masked (T-0104) |
| `scim_users.external_id` | 0 | the IdP's id for the person | masked (T-0104) |
| `webauthn_credentials.credential_id` | 0 | the authenticator's credential id | masked (T-0104) |
| `webauthn_credentials.public_key` | 0 | a stable per-person identifier | masked (T-0121) |
| `refresh_tokens.parent` | 400, a quarter populated | another refresh token | **still copied** |

**Seven of the ten were columns with nothing in them**, where only the name could
have decided — which is precisely the case a name rule exists for, and precisely
where the rule pack was thin: it matched `tokens?`, `secrets?`, `passwords?` and
`api_keys?`, and none of `code`, `verifier`, `credential_id` or `public_key`.
That was T-0104, and the `Now` column is the fix scored against the same table.

The one that remains is the sharpest of the ten and the only one with data in
it: `refresh_tokens.parent` is a refresh token in a column named after a tree
edge, populated, and missed by name and by value alike. There is no pattern to
write — `parents?` would mask the join keys of half a database at priority 80,
and this rule pack sees a column name without its table — so the fix is a
table-scoped pattern, which is a rule-pack feature and not a rule. It is pinned
by `TestSupabaseAuthMissesArePinned`.

The twenty-four false positives are mostly one table — `custom_oauth_providers`,
nine of them, where a deployment's OAuth endpoints (`token_url`, `discovery_url`,
`userinfo_url`) read as secrets or as an `online_id` — plus `provider_type` and
`authentication_method`, which are short enum-ish strings that clear the entropy
threshold, `sso_domains.domain`, which is an organisation's, four `jsonb`
columns masked on their type alone, and the two `code_challenge` columns T-0104
added.

**What the recall number does not measure.** These are the classifier's
decisions, not the run's outcome. A column the classifier misses is copied
verbatim, so a miss here *is* a leak — which is why the grep half of I2 runs over
every torture target as well, and why defect 8 was found by that and not by
this table.

**So phase 5 closes with a measured, reproducible leak of one of the fifty
labelled columns on one of the ten schemas**, and that belongs in the gate's
evidence rather than in this paragraph alone. It was ten when this file was
written, and two of those ten were populated in the fixture that I2's grep half
cannot see — `identities.provider_id` (300 rows, the provider's subject id for
the person) and `flow_state.auth_code`: that half knows email addresses and
phone numbers, and a subject id is neither. Both are masked now. The one that
remains, `refresh_tokens.parent`, is a quarter populated and needs a
table-scoped pattern rather than a name.

All ten are still pinned, one by one, by `TestSupabaseAuthMissesArePinned`
(`internal/classify/supabase_misses_test.go`), which runs in `make test` on
every change: nine assert the masking and one asserts the leak. It fails in
**both** directions — an eleventh miss, or one of these ten changing side
without this table being re-measured — so the number cannot get quietly worse,
and cannot get better without the doc being updated with it. A test that asserts
a leak is an uncomfortable thing to write and a worse thing to lose track of;
that is why it names its own tracker task in its failure message.

## Reproducing it

```
make torture                                        # all ten and the regressions
go test -tags 'integration torture' -run TestTortureSchemas/gitlab ./internal/invariants/
testdata/torture/build.sh gitlab                    # rebuild one schema.sql from its pin
```

`testdata/torture/README.md` is the fixtures' spec, each schema's own
`README.md` is its pin and its deviations, and
`internal/invariants/torture_catalogue_test.go` is the catalogue of runs.

The truth sets above were produced by reading the `columns:` block of each run's
emitted `lazyslice.yml` and scoring it against the labels below. A column counts
as predicted-personal when its `confidence:` is `possible`, `likely` or
`certain`; anything else, and any column the block does not name, counts as
predicted not-personal. The three runs are the catalogue's own — root, `--take`
and flags exactly as `torture_catalogue_test.go` lists them, against a source
loaded with `schema.sql`, `_common/fill.sql`, `generate.sql` and
`_common/cleanup.sql` in that order — so the yml is the same one `make torture`
writes into a temporary directory and throws away. Score it by hand, or with the
run's own emitted file:

```
lazyslice --source ... --target ... --root auth.users --take 100 \
  --unmask 'auth.mfa_factors.friendly_name=...' --config ./lazyslice.yml --yes
```

## The labels

Every column not listed here is labelled not-personal, by the rule stated above.
These are the lists a re-measurement should score against.

**django** (9 of 44):

```
public.auth_user.email                  public.auth_user.first_name
public.auth_user.last_name              public.auth_user.password
public.auth_user.username               public.django_admin_log.change_message
public.django_admin_log.object_repr     public.django_session.session_data
public.django_session.session_key
```

**rails-activestorage** (7 of 36):

```
public.active_storage_blobs.filename    public.comments.author_email
public.comments.author_name             public.comments.body
public.posts.author_email               public.posts.body
public.posts.title
```

**supabase-auth** (50 of 271):

```
auth.audit_log_entries.ip_address              auth.audit_log_entries.payload
auth.custom_oauth_providers.client_secret      auth.flow_state.auth_code
auth.flow_state.invite_token                   auth.flow_state.provider_access_token
auth.flow_state.provider_refresh_token         auth.identities.email
auth.identities.identity_data                  auth.identities.provider_id
auth.mfa_challenges.ip_address                 auth.mfa_challenges.otp_code
auth.mfa_challenges.web_authn_session_data     auth.mfa_factors.friendly_name
auth.mfa_factors.phone                         auth.mfa_factors.secret
auth.mfa_factors.last_webauthn_challenge_data  auth.mfa_factors.web_authn_credential
auth.mfa_recovery_codes.code_hash              auth.oauth_authorizations.authorization_code
auth.oauth_client_states.code_verifier         auth.oauth_clients.client_secret_hash
auth.one_time_tokens.token_hash                auth.one_time_tokens.relates_to
auth.refresh_tokens.token                      auth.refresh_tokens.parent
auth.saml_relay_states.for_email               auth.scim_tokens.token_hash
auth.scim_users.external_id                    auth.scim_users.resource
auth.scim_users.user_name                      auth.sessions.ip
auth.sessions.user_agent                       auth.sessions.refresh_token_hmac_key
auth.users.confirmation_token                  auth.users.email
auth.users.email_change                        auth.users.email_change_token_current
auth.users.email_change_token_new              auth.users.encrypted_password
auth.users.phone                               auth.users.phone_change
auth.users.phone_change_token                  auth.users.raw_user_meta_data
auth.users.reauthentication_token              auth.users.recovery_token
auth.webauthn_challenges.session_data          auth.webauthn_credentials.credential_id
auth.webauthn_credentials.friendly_name        auth.webauthn_credentials.public_key
```

Three of the supabase labels are judgement calls worth naming, because a
different labeller would score them differently. `custom_oauth_providers.client_secret`
and `oauth_clients.client_secret_hash` are the deployment's secrets rather than a
person's, and are labelled personal on the reading that the truth set is "what
must not leave production" — the same reading `credential` is a category under.
`webauthn_credentials.public_key` is public by name and a stable per-person
identifier by use; **T-0121 settled it as `credential`** (2026-09-09) on the
second reading, and the label stayed where it had always been — personal in this
list, and now predicted personal too. `raw_app_meta_data` is labelled *not*
personal, because what GoTrue puts there is the provider list — but a deployment
that put anything else there would move it.
