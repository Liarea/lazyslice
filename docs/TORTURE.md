# Torture testing

Ten real open-source PostgreSQL schemas, at a pinned commit or image digest,
each filled with generated rows, sliced from its most-connected table into a
second container, and put through the invariants.

```
make torture
```

Nine of the ten snapshot cleanly **with twenty-eight flags between them —
twenty `--unmask`, seven `--skip-table` and one `--key`**. That whole sentence
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

**This is the settled count, and `make torture` reports it.** T-0198's own
section below records a landing that briefly moved it — metabase gained two
new `--unmask` flags, and four more schemas newly refused outright, pending
curation this file once left unfinished — and the fix round that followed,
which found curating the rest was either unnecessary or, twice, genuinely
unsafe, and narrowed the rule instead of curating through it. The count did
not move there in the end: metabase's own two new flags turned out not to be
needed either once the rule was narrowed, so twenty-seven flags, nineteen
`--unmask`, is where that section starts. **The T-0257 section below moved it
to twenty-eight flags, twenty `--unmask`**: `internal/plan` gained the
exit-12 refusal `internal/classify`'s own `Decision.Refused` had gone unread
since T-0253, and wiring it up cost metabase one more `--unmask` over a pair
the classifier's own later passes were always going to reconcile anyway.
**The T-0258 section, the last one below, moved it back to twenty-seven and
then, on review, back up again to twenty-eight — where it stays.** The first
cut of T-0258's narrowing treated a decision as "resolved" once it was
genuinely masked *or* left unmasked on the operator's own say-so, which
brought the count back to twenty-seven — but that cut also admitted the one
shape the fk-pair refusal exists to catch: a masked column whose validated-FK
partner is a different column, left with its production value copied
verbatim because an *unrelated* `--unmask` happens to sit on it. A review
round found that shape is exactly metabase's own
`core_session.id`/`login_history.session_id` pair, corrected the narrowing to
require both ends genuinely `Decision.Masked`, and metabase's second
`--unmask` is back — **twenty-eight flags, twenty `--unmask`, seven
`--skip-table`, one `--key`, is the count `make torture` reports today.**

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

**It did not, for a while, and does again.** A `make torture` run taken for
T-0119 (2026-09-14) failed `testdata/regressions/013-json-object-key-that-
parses-as-an-email.sql` at exit 9, `verify.refused.second_net`, instead of the
exit 0 the fixture expects with the key masked; T-0172 tracked the fix and
closed the same day. `TestTortureSchemas`, `TestTortureCatalogueMatchesTheFixtures`,
`TestTortureImagesAreReachable` and `TestTortureNegativeControl` all passed on
their own throughout, and every regression but 013 did too — the ten schemas
below and their flag counts were unaffected by the window either way.

**T-0187 (2026-09-15, round-2 red team) re-measured the ten against the twelve
national-identifier validators (`internal/textsig/nationalid.go`) newly wired
into both `internal/classify`'s ordered validators list and `internal/verify`'s
second net.** `make torture` exits 0 with the same twenty-seven flags, the same
nine-clean-one-refused split, and no schema needed a new one: none of the ten
real schemas' generated data happens to carry a plain, unseparated national
identifier in a column the rule pack's name patterns miss, which is the shape
that would have surfaced as a new `--unmask` here. What the round found is in
`testdata/regressions/018-plain-ssn-in-an-unrecognised-column-name.sql`,
`019-national-id-text-array-carrier.sql` and `020-ssn-stored-as-bigint.sql`
instead — reduced fixtures, not one of the ten, which is what "reduced to the
smallest schema that still fails" (`testdata/regressions/README.md`) means in
practice when the real schemas do not happen to exercise a gap.

**The review round that followed T-0187 found the digits-family entry 020
needed was itself a false-positive hazard, and a second review round found
the first fix was two bugs at once — `make torture` still exits 0 with the
same twenty-seven flags after both.** `ValidNationalIDDigits` has no check
digit — the SSA's own exclusion ranges are the whole of the check — so it
cleared ~91% of random 9-digit numbers, and (the second round's own
measurement) effectively 100% of YYYYMMDD-shaped integer dates over any
realistic booking range and of a dense run of assigned numbers, well over the
ordinary 0.8 ratio: an unmasked surrogate bigint id column, an ordinary
non-key dense business-number column, or an ordinary booking-date integer
column, had no green path short of `--unmask`. None of the ten real schemas
happened to surface either round's shape — the same "no schema needed a new
flag" reading as above, for the same reason: a schema needs an *unmasked,
ratio-scored* 8- or 9-digit numeric column with no personal data in it, and
none of the ten's generated data produces one by chance.
`testdata/regressions/021-ordinary-numeric-columns-clear-the-ssn-ratio.sql`
is the first reviewer's own probe schema, reduced.

**The first fix — a surrogate-key exemption read off `internal/classify`'s
own decision, plus a ratio threshold raised above the measured rates — was
itself wrong twice over, and the second review round's own probe
(`testdata/regressions/022-national-id-in-a-surrogate-key-column.sql`) is
what a third one reduces.** The 97%-of-dates figure the threshold was set
above was itself an artefact of the range it was measured over (over any
realistic range the true rate is 1.0), and gating the exemption on classify's
decision meant a primary key of real SSNs — a column classify's own signals
found nothing personal in — was exempted along with the ordinary keys the
exemption was written for, crossing into the target verbatim at exit 0. The
fix is now read from the column's own values during the scan rather than from
either a ratio alone or a second read of classify's decision:
`textsig.ValidNationalIDDigits` excludes a value that is also a real calendar
date directly, and `internal/verify`'s second net exempts a column whose own
values pack into a dense numeric range — a generated sequence, key or not —
which a primary key of independently assigned SSNs is not, whatever
`internal/classify` decided about the column being a key. 021 now pins a
dense surrogate id column, a dense non-key business-number column and an
all-2024-dated booking column together, none tuned to a threshold; 022 pins
the primary key of real SSNs still refusing; and 020's own SSN-as-bigint
column — ratio 1.0, neither a date nor dense — still refuses at exit 9 with
every change in place.

**A third review round found a shape neither of those two fixes reaches, and
`make torture` still exits 0 with the same twenty-seven flags after this one
too.** A *sparse* numeric column with a fixed leading prefix and no check
digit is neither dense (`digitRange`'s own test) nor a date
(`looksLikePlausibleDate` only ever excludes an eight-digit value), and it
clears the SSA's exclusion ranges at essentially 1.0 regardless — the
reviewer measured 494/500 for 500 account numbers of the form
`100000000+rand(1e8)` and 500/500 for 500 invoice numbers of the form
`202600000+7*rand(50000)`, and no ratio under 1.0 tells that shape apart from
a real leaked identifier column, because assigned identifiers clear the same
ranges at the same rate. The fix is a gate in front of the ratio rather than a
fourth exclusion rule: the digits entry now refuses only when the column's own
name matches `rules.yml`'s national_id pattern or a certain-or-likely personal
column sits in the same table (`Decision.NameMatchedNationalID` and
`Decision.TableHasLikelyPersonalColumn`, both computed by `internal/classify`
and read by `internal/verify`'s `corroborated`, since the second package may
not re-run the first's rule pack). Without either, the ratio is never asked at
all. None of the ten real schemas needed a new flag for this round either, for
the same reason as the first two: none of them carries an unmasked, sparse,
fixed-prefix numeric column with no personal data and no name or neighbour
signal, which is the shape that would have surfaced here as one.
`testdata/regressions/023-sparse-fixed-prefix-reference-block-is-not-
national-id.sql` is the reviewer's own probe, reduced; `020` and `022` both
needed a corroborating column added to keep refusing under the new rule, and
their own headers say why.

**T-0221 (2026-09-16, the round-3 red team's kontaktnr/contact finding) re-measured the ten against `--phone-region` and the guessed-region corroboration gate, and `make torture` exits 0 with the same twenty-seven flags after this one too.** The phone validator on both nets parsed under `textsig.PhoneRegionHint` ("ZZ") only, which admits a number already written in international form and nothing else, so a plain national-format phone number — `07911 123456`, `020 7946 0958` — crossed unmasked whatever the ratio and whatever the column was called (`testdata/regressions/025-national-format-phone-region-kontaktnr.sql`, both a dictated and a plainly written UK number). `--phone-region REGION` fixes the case an operator can name; with none named, a short built-in list of common regions is tried instead, and a hit is masked only with corroboration — a proven personal neighbour in the same table, the same `Decision.TableHasLikelyPersonalColumn` signal `internal/verify`'s national_id digits entry already reads for its own corroboration gate (a name match needs no corroboration from this gate at all: `decide`'s ordinary name-match branch has already masked such a column before this pass would otherwise see it, and a since-removed second arm that read the name match again here could never fire, per a later T-0221 review round). **The first landing of that gate collided with the digits entry it borrowed the signal from**, found by re-running this suite rather than by a fixture written for the purpose: `020-ssn-stored-as-bigint.sql`'s own `taxref bigint`, corroborated by the file's own `email` neighbour, cleared one of the fifteen guessed regions on all five values and was masked as `phone` before `internal/verify`'s digits-family national_id entry ever saw the unmasked column, turning the file's own `expect: exit 9 verify.refused.second_net` into a quiet `ok` — a real leak of the *right* category caught for the *wrong* reason, and a different category from the one the row actually held. The guessed-region pass now runs over a character family only (`text`/`varchar`/`bpchar`/`citext`); a digits-family column is left for `internal/verify`'s own entry to decide exactly as it always has, which is what `020` still asserts and what `026-ten-digit-account-number-is-not-a-guessed-phone.sql` (the false-positive control this task's own brief asked for, a ten-digit account column with no name or neighbour signal, left unmasked) had to be written as `text` rather than `bigint` to test honestly. None of the ten real schemas needed a new flag for either half: none of their generated data carries a plain national-format phone number in a column `rules.yml`'s existing phone pattern misses (the shape `--phone-region` closes), and none carries a character column that both clears a guessed region and sits beside a personal neighbour with no real phone in it (the shape the corroboration gate exists to refuse).

One thing the nine clean runs do not say on their own, measured below:
supabase-auth's classifier now catches **all fifty** of the columns the
hand-labelling calls personal — recall 1.000 — where it missed **ten** at
recall 0.800 when this file was written. T-0104's name rules took eight of
them, T-0121's `public_key` decision the ninth, and T-0119's table-scoped rule
the tenth and last: `refresh_tokens.parent`, a quarter populated in this
fixture, held another refresh token in a column named after a tree edge and
reached the target in cleartext under exit 0 until now. The re-measurement is
in the truth sets below. A second finding used to stand beside it —
`credential`'s only masker had a domain of one — and that is the T-0098 defect
the count above no longer carries.

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

## T-0198: the special-category literal rule's measured cost

THREAT_MODEL.md T1's 2026-09-16 amendment carries the rule this section
measures: a masked column's own `CHECK`, generated expression, index
predicate or non-rewritable `DEFAULT` is refused on any literal none of the
eleven DDL-literal validators recognises, not only on a strong hit. The
tracker task's own decision (T-0198's log, 2026-09-16) asked for `make
torture` before and after, and for every schema the wider rule newly refuses
to be named here with the flag it needs, or a note that the flag is not yet
curated.

**This section is the history of that measurement, and it ends in a
different place than the landing it started by measuring: "the fix round"
below is the current state, and the four schemas the rest of this section
still marks "not yet curated" were never curated at all — the wider rule
stopped reaching the object classes that made curating them either
impractical or, twice, genuinely unsafe.** The measurement is kept in full
because it is what decided that, not because any of it is still owed.

**Before.** All ten schemas were at the baseline this file already
documents: nine clean with twenty-seven flags, mastodon refusing by design.

**After, first landing (no exemption for an empty collection literal).**
Seven of the ten newly refused, every one of them on `DEFAULT '{}'::jsonb`,
`DEFAULT '{}'::text[]` or `DEFAULT '[]'::jsonb` — the shape a `semi_structured`
or array-typed masked column's default takes in almost every schema that uses
one at all (`auth.custom_oauth_providers.scopes`, `public.oban_jobs.args`,
`public."OrganizationOnboarding".invitedMembers`,
`public.admin_dashboard_sections.settings`, and more beneath each once the
first is cleared). An empty collection holds nothing to be a person's — the
identical argument ARCHITECTURE.md §5 already makes for a masked column's
*row* value of `'{}'`/`'[]'` (`internal/invariants/i2_masking_test.go`'s
`preservedEmpty`), restated here for a `DEFAULT` rather than invented to pass
this measurement — so `unrewritableLiteral` in both
`internal/plan/ddlliteral.go` and `internal/verify/catalog.go` now excludes
it, the same way it already excludes a closed value list and a Pattern
operand.

**After, with that exemption.** Five of the ten still newly refuse:
supabase-auth, metabase, gitlab, odoo, discourse. calcom and plausible are
clean again — both were empty-collection-default cases and nothing else.
Metabase was fixed with two new flags, landed in `tortureSchemas` at the
time; the other four were not, and were named here rather than left for the
next person to rediscover. **Metabase's own two flags did not survive "the
fix round" below** — they closed an expression-index refusal, and once the
wider rule stopped reaching indexes at all metabase needed nothing beyond
the two flags it already had before this task.

- **metabase** (fixed at the time; not needed after "the fix round").
  `public.audit_log.idx_audit_log_entity_qualified_id` and
  `public.view_log.idx_view_log_entity_qualified_id` are an expression
  index whose `CASE` spells `'card_'`/`'Dataset'` — a type discriminator, not
  either table's own content. `view_log.model` is masked at all only because
  `generate.sql`'s generic filler writes `"view_log.model-1"` and the like
  into it, which `textsig.LooksSecret` reads as a credential (16+
  characters, two character classes) — a fixture artefact, not evidence
  about real Metabase data — and `audit_log.model`, which `generate.sql`
  never fills, inherits the same decision through classify's
  same-column-name rule. Two `--unmask` flags were landed in
  `tortureSchemas` to close it; both are gone again, not because the
  discriminator stopped being masked but because an expression index no
  longer carries the wider net that refused on it.
- **gitlab**. At least six more objects beyond the first: an expression
  index (`index_issues_on_description_trigram_non_latin`) whose predicate is
  a `SIMILAR TO` deparsed through `similar_escape(...)` — fixed separately,
  below, since the literal genuinely was a Unicode character-class shape and
  not a value; `index_members_on_user_id_created_at`
  (`members.source_type = 'GroupMember'`, another discriminator, masked via
  the same filler-reads-as-a-secret shape metabase's `model` had);
  `check_namespace_details_state_metadata_is_hash` and four sibling
  `namespace_settings` checks, all `jsonb_typeof(col) = 'object'::text` —
  the function's own three-or-so-word return vocabulary, not a person's, on
  a `semi_structured` column masked whole regardless of what this literal
  says; `index_groups_on_path_and_id` (`namespaces.path`, masked only via
  same-column-name propagation from `organizations.path`, already unmasked
  for an unrelated reason); `index_notes_for_cherry_picked_merge_requests`
  (`notes.noteable_type = 'MergeRequest'`, a Rails polymorphic-association
  discriminator). GitLab's schema is the largest of the ten by a wide
  margin, and this shape — a `_type` or `_id` discriminator compared to a
  fixed string in a partial index or a `CHECK`, beside a column this run
  masks for an unrelated reason — recurs; curating every instance is
  follow-up work, not something this landing forces through by narrowing
  the rule a third time. **Not curated in the end — see "the fix round"
  below: every one of these is a `CHECK` or an index, and the wider rule
  stopped reaching either object class.**
- **odoo**. `public.ir_filters.ir_filters_name_model_uid_unique_action_index`:
  `COALESCE(user_id, '-1'::integer)`, `COALESCE(action_id, '-1'::integer)` —
  the sentinel literal `-1`, quoted then cast, is not personal data by any
  reading. **Closed by the cast-to-non-text exemption below, not by
  curation** — and odoo turned out to need more than this one object before
  curation could finish it at all; see "the fix round".
- **discourse**. `public.categories.unique_index_categories_on_name`:
  `COALESCE(parent_category_id, '-1'::integer)`, the identical sentinel
  shape odoo's does. **Closed by the same cast-to-non-text exemption.**
  (`admin_dashboard_sections.settings`, the object named in an earlier draft
  of this measurement, is one of the seven the empty-collection exemption
  already closed.)
- **supabase-auth**.
  `auth.custom_oauth_providers.custom_oauth_providers_oauth2_requires_endpoints`:
  `CHECK (provider_type <> 'oauth2'::text OR authorization_url IS NOT NULL
  AND token_url IS NOT NULL AND userinfo_url IS NOT NULL)`. The literal
  `'oauth2'` is `provider_type`'s own discriminator value and `provider_type`
  itself is unmasked; the constraint is judged "on a masked column" only
  because it also names `authorization_url`/`userinfo_url` (`online_id`) and
  `token_url` (`credential`) — the pre-existing, unnarrowed scoping rule that
  a `CHECK` naming several columns is judged whole once any one of them is
  masked (`fixedExpression`'s `onMasked`, unchanged by this task). **Not
  curated in the end** — three masked columns named by one constraint is
  exactly the ambiguity "the fix round" below found unsafe to curate through
  in general (odoo's own two cases), and this object is a `CHECK`, so the
  wider rule no longer reaches it at all.

**A genuine second parsing gap, fixed rather than worked around.**
`pipeline.Literal.Pattern` never recognised `similar_escape('pattern',
escape)`, which is exactly what `pg_get_expr`/`pg_get_constraintdef` deparse
every `SIMILAR TO` into — so gitlab's trigram index predicate, a Unicode
codepoint-range character class and not a value, was scored as an ordinary
literal by every validator this task's rule and every validator before it
both ran. `afterPatternOperator` (`internal/pipeline/ddlliteral.go`) now
also marks a literal `Pattern` when it is the first argument of a
`similar_escape(` call, the deparser's own spelling of `SIMILAR TO`/`NOT
SIMILAR TO`, on the same footing the `~`/`!~`/`LIKE` branches already have.
This is not scoped to T-0198's own literals — it corrects what `strongHit`
and `strongCatalogHit` were already asking of such a literal, which is why
it is a fix and not an exemption.

**What was deliberately not done.** A third and a fourth structural
exemption — for a `jsonb_typeof(...)` comparison, for a bare-integer
`COALESCE` sentinel, for a polymorphic `_type` discriminator — would close
several of the remaining cases outright, and each is individually
defensible on the same "this shape cannot be a person's" argument the
empty-collection exemption already makes. They were not landed at the time
this measurement was taken. The empty-collection exemption restates a rule
ARCHITECTURE.md §5 already states elsewhere in this codebase; these would
each have been invented for the first time in direct response to a
measurement, which is the one thing the tracker task's own decision ruled
out ("land the rule behind the existing opt-out and say so in concerns
instead of weakening it"). One of the two structural exemptions that *did*
land afterwards — a literal cast to a non-text type, which closes both of
odoo's and discourse's `COALESCE(..., '-1'::integer)` sentinels outright — is
exactly this same kind of shape argument, and is not "weakening the rule a
third time" for the reason the empty-collection exemption already is not:
neither is invented to make a measurement pass, both restate that a
particular *shape* of literal cannot be a person's regardless of the column
it sits beside. What is below is not a third structural exemption on top of
that one; it is the finding that curating the rest by hand, the way this
section originally asked for, ran into two real schemas where the `--unmask`
escape itself could not be made to answer honestly — see "the fix round".

## The fix round (2026-09-16): why curation stopped, and what replaced it

The four schemas above stayed "not yet curated" for longer than a single
follow-up task, because finishing them the way this section asked for —
one `--unmask` per newly-refused object, the same way metabase's two were
closed above — ran into two real schemas where no `--unmask` could close the
refusal honestly. Both are odoo, and both are named here because a reduced
regression fixture cannot reproduce what made them dangerous: the reason is
that a real schema's own columns collide with the discriminators beside them
in a way a synthetic table built to test one rule does not.

- **`public.res_partner.res_partner_check_name`**:
  `CHECK ((type = 'contact' AND name IS NOT NULL) OR type <> 'contact')`.
  `res_partner` is Odoo's CRM contacts table, and `name` is not a technical
  label there — it is the actual person's or company's name the table exists
  to hold, correctly masked as `person_name`. The constraint's own literal,
  `'contact'`, is about `type`, a discriminator column unmasked or masked for
  an unrelated reason; it is never about `name`. But `fixedExpression`'s
  scoping judges the object whole once *any* named column is masked, so the
  only escape the rule could offer for this literal was `--unmask
  public.res_partner.name=REASON` — which does not say "this literal about
  `type` is not personal data", it says "this column, which manifestly is
  personal data, is not", and an operator who typed it would ship every
  contact's real name into the target.
- **`public.res_partner.res_partner_mobile_partial_gin_idx`**: a GIN trigram
  index over `regexp_replace(mobile::text, '[\s\\./\(\)\-]', '', 'g')` —
  `mobile` is a real phone number column, correctly masked, and the
  expression normalises it for fuzzy search. Its two non-empty literals are a
  punctuation character class and a `regexp_replace` flag, neither a value
  about the phone number at all. `mobile` is the *only* masked column the
  index names, so this is not even the multi-column ambiguity the first case
  is — the rule's escape was `--unmask public.res_partner.mobile=REASON`,
  the phone number itself, offered as the fix for two literals that were
  never about it.

Both are a `CHECK` and an index, and both crossed no bright line the first
landing's own exemptions (a closed value list, an empty collection, a
Pattern operand, a cast to a non-text type) could have been widened to
catch, because neither literal is any particular *shape* — one is an
arbitrary function argument, the other an arbitrary punctuation class — and
the object being refused is not one column but a `CHECK` or an index that
can name several. That is the property the fix does turn on: **the wider net
now runs only for a `DEFAULT` and for a generated expression, and stopped
running for a `CHECK`, an exclusion constraint or an index of any kind**
(`internal/plan/ddlliteral.go`'s `fixedExpression`, called with `broadNet`
false for a constraint or an index and true only for a generated expression;
`internal/verify/catalog.go`'s `unrewritableKind` answers `kindDefault`/
`kindGenerated` only). A `DEFAULT` and a generated expression belong to
exactly one column by construction — `internal/plan`'s `named` is always
`[]string{col.Name}` there — so the ambiguity both odoo cases turn on cannot
arise on that path, and a `CHECK` or an index still refuses on a strongHit
exactly as §11.1's original rule always has; only the newer, escape-free
net that refused on a literal no validator recognised at all is gone from
those two object classes.

**Re-measured against this scope, `make torture` needs none of the four
schemas' curation this section spent three paragraphs asking for — and
metabase's own two landed flags turned out to be unnecessary too.** Every
refusal gitlab, odoo, discourse and supabase-auth hit under the wider rule —
the `jsonb_typeof` checks, the polymorphic `_type` discriminators, the
`COALESCE(..., '-1'::integer)` sentinels, the oauth2 constraint naming three
masked columns at once — was a `CHECK` or an index, never a `DEFAULT`; so was
metabase's own expression index. None of them needed curating because none
of them refuses under this scope. The suite is back to the exact twenty-seven
flags this file opens with — the same nineteen `--unmask` and nothing more —
and gitlab and supabase-auth in particular needed no touching at all despite
being two of the four schemas this section spent longest asking someone to
curate. `special_category`'s own value validator is unaffected and
unconditional either way — it closes both round-3 canaries through the
ordinary strongHit path (below "Before"), which is why neither canary
regression (027, 028) needed re-cutting.

## T-0239: the A2b rail's declared-length floor, re-measured

The round-4 red team's native-script variant defeated `unknownColumnsBesideCertain`'s
own declared-length exclusion — `varchar(12)` was under the rail's old floor of
sixteen characters, so a name column that length beside a real `email` column
copied verbatim (THREAT_MODEL.md T1's A2b amendment, above, carries the full
account). The fix lowers `internal/classify/classify.go`'s `minUnknownLen`
from sixteen to two, so every character column from two characters up that
this rail would otherwise raise now is, unless one of the four remaining
exclusions applies.

**None of the ten schemas moved.** Re-running `make torture` after the change
passes all ten at the unchanged twenty-seven flags (nineteen `--unmask`, seven
`--skip-table`, one `--key`) `TestTortureCatalogueMatchesTheFixtures` already
pins — a lower floor could only ever mask *more* of a column this rail was
already built for, never force a new refusal, because the rail's own unique-
index exclusion is what stands between a narrow column and §5's domain rule,
and that exclusion is unchanged. Four of the ten (discourse, metabase, odoo,
plausible) declare at least one character column under sixteen characters and
still pass unchanged; the other six declare none at all under that width, so
the change reaches nothing in them either way.

**The three PII truth sets below are unaffected, provably rather than by
re-measurement.** `unknownColumnsBesideCertain` only ever reaches a character
column (`text`/`varchar`/`bpchar`/`citext`), and django, rails-activestorage
and supabase-auth — the three schemas with a hand-scored truth set in "PII
truth sets" below — declare **no** `varchar(n)` or `char(n)` column under
sixteen characters at all (`grep -Eio 'varchar\([0-9]+\)|character
varying\([0-9]+\)|char\([0-9]+\)'` over each `schema.sql`, filtered under 16,
is empty for all three): every character column in the three is either
unbounded `text` or declared sixteen or wider, so nothing in them ever fell
inside the floor this task moved. Their precision and recall figures below
stand as measured.

**Fix-round addendum.** The lowered floor above reached one shape it should
not have: a validated foreign key's character-family column, at either end,
beside a `certain` column in the same table — an ISO-style currency code
referenced by a same-width FK child was masked on one side of the join and
left verbatim on the other, which is not a leak but a new way to half-load a
target (THREAT_MODEL.md's own fix-round amendment carries the full account).
`unknownColumnsBesideCertain` now excludes such a column too
(`indexFKColumns`, `internal/classify/classify.go`), so the sentence above —
"unless one of the four remaining exclusions applies" — is stale: it is five
now, never-masked, type-conflicting, unique index, two-letter codes, and a
validated foreign key's character columns.
`testdata/regressions/031-fk-child-code-column-beside-a-certain-column.sql`
pins the reproduction under `make torture` (not re-run for this addendum —
Docker-gated and outside this fix round's own checks, `make check`). By the
same argument as the paragraph above, the new exclusion is expected to move
none of the ten schemas' flag counts: it only ever narrows what this one rail
alone can mask, never widens it, so any FK-linked column it now leaves alone
either was never reached by this rail in the first place or is still reached,
correctly, by some other pass (a name hit, a value validator, or FK
propagation from an already-masked parent). That expectation is unmeasured
and stated as one.

## T-0253 review round: fkPairs bounded to direct partners, and a shared lookup parent's own blast radius

T-0253's own review found two things `fkPairs` (the fix above's replacement
for the blanket FK exclusion) got wrong, neither reached by `make torture`'s
ten schemas — both are about a shape none of the ten declares, a table whose
only certain personal column sits beside an FK child of a lookup table that
other, unrelated tables also reference.

**First, a bug:** `fkPairs` walked the whole connected foreign-key component
reachable from the column being raised, not only its direct partner. A
column two hops away — in a table with no relationship at all to the
`certain` neighbour that justified the raise — could veto the whole pairing
on its own shape (a type conflict, a two-letter code), leaving the real
FK-linked column copied verbatim: the exact leak this rail exists to close,
triggered by a column that never should have had a vote. Fixed by bounding
`fkPairs` to direct partners only; a masked parent still reaches every other
table that references it through `propagateKeys` (a separate, later pass
that already does this unconditionally, per ARCHITECTURE.md §4), which
records a type conflict on one disagreeing child rather than vetoing the
parent's own masking. `internal/classify/CLAUDE.md`'s own section on this
amendment has the full account;
`TestFKPairIgnoresAnUnrelatedGrandchildOfASharedLookupParent`
(`internal/classify/redteam_test.go`) pins it at the unit level.

**Second, not a bug, but worth stating rather than leaving as a first-run
surprise: masking a shared lookup parent still cascades to every table that
references it, whatever those tables hold.** `currencies(code PK)`
referenced by both `members(email certain, currency)` and
`invoices(id, currency)` masks `members.currency` and `currencies.code`
together (the fix above), and `propagateKeys` then masks `invoices.currency`
too, because ARCHITECTURE.md §4's propagation is unconditional once a parent
is masked — `invoices` never had a `certain` column and has no relationship
to `members`, and is masked anyway, because leaving its copy of a masked
parent's values unmasked would break the join. Where the shared parent is
narrow and under a unique index — an ISO country or currency code, a status
enum stored as a lookup table — `free_text`'s fixed word list cannot meet
§5's `d_required` at more than a handful of rows, and the whole run refuses
at exit 12 rather than loading a mismatched join
(`testdata/regressions/031`, `035`). A production schema with a small,
widely-shared lookup table referenced from many otherwise-unrelated tables,
where only one of those tables happens to hold a `certain` personal column,
should expect the same: a full refusal naming the whole equality group, not
a partial mask of only the tables that look related. This is
ARCHITECTURE.md §4's own propagation rule working as specified, not
something `fkPairs`' own bound above changes — narrowing `fkPairs` to direct
partners stops an unrelated column from vetoing a raise, it does not, and
was never going to, stop a raise from cascading once it happens.

## T-0257: the fk-pair refusal wired into internal/plan, and metabase's new flag

`fkPairs` (both sections above) has refused a pair since T-0253 whenever a
direct partner already carries a decision this rail must not override — a
type-conflicting name hit, or measured two-letter-code evidence — and has
recorded that refusal on `Decision.Refused` and `Decision.RefusedPartner`
since the same task's own review round. Until this task, nothing read it:
`internal/plan` had no check over the field, so both ends of such a pair
still loaded copied verbatim under exit 0, the pre-T-0253 leak reopened for
this one shape. `internal/plan/fkpair.go`'s `checkFKPairRefusal` is the fix —
it runs beside `checkWriteBack`, before the first key is fetched, and stops
the run at exit 12 naming both columns of the pair with an `--unmask` escape
for each, unless the operator has already unmasked both.

**One of the ten needed the escape.** `metabase.core_session.id` is a
`character varying(254)` session token, decided `credential` on its own
value signal (200 of 200 samples look like secrets) before
`unknownColumnsBesideCertain` ever runs, and already carried an `--unmask`
of its own for an unrelated reason (its value is an opaque, regenerated
token, not a fixed secret worth refusing over). `login_history.session_id`,
its validated foreign-key child, has no samples of its own and sits beside
`login_history.ip_address` (a `certain` `network_id` column in the same
table) — exactly the shape `unknownColumnsBesideCertain` exists to raise as
`free_text`. `fkPairs` asks whether the parent, `core_session.id`, can be
raised the same way, finds it already carrying the `credential` decision
above, and refuses the pair: `plan.refused.fk_pair`, naming both columns.
The run now carries a second `--unmask`,
`public.login_history.session_id=an opaque session id, the same value as
the session it belongs to`
(`internal/invariants/torture_catalogue_test.go`), and the settled count at
the top of this file moves from twenty-seven flags to **twenty-eight —
twenty `--unmask`, seven `--skip-table`, one `--key`**.

**Worth naming rather than leaving as a silent cost: this one refusal looks
narrower than it is.** FK propagation (`propagateKeys`, a separate,
unconditional pass that runs after `unknownColumnsBesideCertain`) carries a
masked parent's category onto every child referencing it regardless of what
`fkPairs` decided, and `core_session.id` is masked — so `login_history.
session_id` ends up `credential` too, matching its parent, whether or not
`fkPairs` ever refused the pair. `Decision.Refused` is set at the point in
the pipeline where `fkPairs` runs, and nothing clears it when a later pass
(`propagateKeys`, `sameColumnName`) goes on to bring the two ends into
agreement anyway — so `internal/plan`'s new check refuses a pair here that
the classifier's own later passes were always going to resolve safely. That
is the correct default given root CLAUDE.md's "when in doubt, mask it" — a
refusal that costs an operator one `--unmask` is the safe direction, and a
check that tried to predict which later pass would run and agree is a wider
change than this task's brief asked for — but it is not free, and a task
that narrows `checkFKPairRefusal` to skip a pair once both ends are already
`Masked` under the same final `Category` would remove this one flag without
weakening what the refusal is for. Filed as **T-0258** rather than done here,
to stay inside this task's own paths.

## T-0258: the fk-pair refusal narrowed to skip a pair a later pass already reconciles

T-0257's own section above named the fix rather than taking it: `internal/plan`'s
`checkFKPairRefusal` reads `Decision.Refused`, which `fkPairs` sets at the point
in the pipeline where it runs and never clears — so it refuses a pair even when
a later, separate classify pass (`propagateKeys`, `sameColumnName`) goes on to
bring both ends into agreement anyway, the exact shape `metabase.core_session.id`/
`public.login_history.session_id` is. `checkFKPairRefusal` (`internal/plan/fkpair.go`)
now reads the classification's **final** state — the same map `checkWriteBack`
already reads, after every classify pass has run — and skips the refusal once
it finds `reconciledByALaterPass` true of both ends: the identical, non-`none`
`Category`, with each end either genuinely `Masked` or left unmasked on the
operator's own say-so (`Decision.Source`, the same distinction
`unmaskedByOperator` already draws for the "both explicitly unmasked" escape
just above it).

**Measuring the metabase pair itself found the first, simpler cut wrong.**
The task's own log entry describes the fix as "skip the refusal when both are
`Masked` and share the same final `Category`", and that was the first thing
tried — `reconciledByALaterPass(d, partner) = d.Masked && partner.Masked &&
d.Category == partner.Category && d.Category != CatNone`. Run against the
real fixture rather than a hand-built classification, it left the pair
refused: `core_session.id` already carries its own `--unmask` from before
T-0257, for the unrelated reason recorded on the flag itself ("an opaque
session id, regenerated on every login") — and that flag sets
`Decision.Source = pipeline.ByFlagUnmask` and `Decision.Masked = false` on
`core_session.id`, independently of anything `fkPairs` or `propagateKeys`
decided. `propagateKeys` still carries `credential` onto `session_id`
regardless, and `session_id` has no `--unmask` of its own, so it ends up
genuinely `Masked == true` under that category. The two ends therefore share
a `Category` but disagree on literal `Decision.Masked` — one true, one false
— and the literal reading of the log entry left this exact case, the one the
task measures against, still refusing. The fix is `resolved(d)`:
`d.Masked || unmaskedByOperator(d)`, applied to both ends rather than
`Masked` alone — a column an operator has explicitly, separately said is fine
to leave with its production value is a resolved end for this rail's purposes
exactly as a genuinely masked one is; `fkPairs`' own refusal predates that
flag and has nothing to do with it.

**What the narrowing does not touch.** A pair `fkPairs` refuses for a type
conflict (the partner's own name-matched category disagrees) or on measured
two-letter-code evidence (the partner is a lookup, not personal data) never
reaches a non-`none` `Category` on the blocked partner at all —
`fkPartnerRaisable` stops it there, and `propagateKeys`' own `type_conflict`
branch leaves the child's `Category` untouched — so `reconciledByALaterPass`
reports false on the `Category` check alone and the refusal still fires
exactly as T-0257 built it, whichever cut of "resolved" is used.
`TestFKPairIsRefusedAtPlan` and `TestFKPairRefusalIsClearedWhenBothEndsAreUnmasked`
(`internal/plan/fkpair_test.go`) are unchanged and still pass. Four new tests
hold the narrower shape: `TestFKPairRefusalStillFiresWhenOnlyOneEndIsMasked`
and `TestFKPairRefusalStillFiresWhenCategoriesDiffer` pin the category side;
`TestFKPairRefusalStillFiresWhenNeitherEndIsResolved` pins that a shared
category with neither end masked nor operator-unmasked still refuses — the
plain "nothing has moved" shape `fkPairs` itself produces. The plan-level
test for the reconciled pair is
`TestFKPairRefusalIsSkippedWhenALaterPassReconcilesThePair` (both ends
genuinely `Masked`, the simple case). The metabase shape itself, reduced —
one end `Source == pipeline.ByFlagUnmask`, `Masked == false`; the other
genuinely `Masked == true`; identical `Category` on both — was originally
held by a test of the same shape asserting the refusal was skipped; the
review round below renamed it
`TestFKPairRefusalStillFiresWhenOneEndIsOperatorUnmaskedAndTheOtherMasked`
and inverted the assertion, since that shape is exactly what the corrected
rule still refuses.

**Re-measured, metabase's second `--unmask` was gone and the suite settled at
twenty-seven — until a review round found the measurement was hiding an I1
gap rather than closing one, and put it back.** The paragraph above shipped
first with `reconciledByALaterPass(d, partner) = d.Category != CatNone &&
d.Category == partner.Category && resolved(d) && resolved(partner)`, where
`resolved(d) = d.Masked || unmaskedByOperator(d)`. That reading accepts a
pair where ONE end is genuinely masked and the other end is left unmasked —
not because a later pass reconciled anything, but because an *unrelated*
`--unmask` happens to sit on it, for a reason that has nothing to do with the
pair at all. `core_session.id`'s flag is exactly that: "an opaque session id,
regenerated on every login" says nothing about `session_id`'s side of the
join, and `session_id` is masked to `credential` regardless. Nothing else
catches the resulting shape — `equality.go`'s `maskedMembers` only groups
columns whose decision is `Masked`, so the unmasked `core_session.id` is
never in the group, and `keyChildren`'s exemption needs `isKeyFamily`, which
a `character varying` key does not satisfy — so a masked FK child whose
parent's identical values are copied verbatim reaches the target with
nothing having refused it: the load adds the edge `NOT VALID` and
`internal/verify/fk.go`'s own orphan count fails the run at **exit 8** for a
key the plan could have refused instead, at exit 12, before the first row
moved (I1, THREAT_MODEL.md T8; `internal/classify`'s own `markNeverMasked`
comment names this exact shape as broken).

The metabase fixture's own clean I1 pass never demonstrated the failure, and
that is a property of the fixture rather than of the shape: `login_history`
is nullable (`ON DELETE SET NULL`) and
`testdata/torture/_common/fill.sql`/`generate.sql`'s own `login_history`
UPDATE never touches `session_id`, so the column is NULL in every row the
fixture writes and `TestI1ForeignKeysResolve`'s anti-join has nothing to find
regardless of what the plan allowed through. `reconciledByALaterPass` now
requires `d.Masked && partner.Masked` on both ends, with no
`unmaskedByOperator` alternative — the literal reading the task's own log
entry for T-0258 asked for in the first place — and `session_id` needs its
`--unmask` back: `public.login_history.session_id=an opaque session id, the
same value as the session it belongs to`, restored in
`internal/invariants/torture_catalogue_test.go`'s metabase entry. The settled
count at the top of this file stays at **twenty-eight — twenty `--unmask`,
seven `--skip-table`, one `--key`**, where the T-0257 section above already
left it; this review round is a correction to the narrowing, not a second
move.

`ROADMAP.md`'s gate-5 line already names twenty-eight from T-0257's own
landing and needs no further correction; that file is outside this task's
paths regardless.

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

Two decisions were taken rather than a rule widened, and both are settled now.

**T-0121 — a `public_key` column is `credential`** (2026-09-09), masked to
the unusable literal, or `credential_unique` under a unique index. A public
key is published by design, which is the argument the other way, but it is a
stable identifier for exactly one person and nothing a development database does
needs the real one. T-0104 had named it one of two columns that deserve a
decision rather than a pattern.

**T-0119 — `refresh_tokens.parent` is `credential`, by a table-scoped rule**
(the other of the two). It holds another refresh token, in a column named
after a tree edge; a name rule matching `parents?` would have masked every
`parent_id` join key in every schema there is, at priority 80, which this
package's plain name rules cannot avoid because they see a column name with no
table beside it. `rules.yml` gained `table_patterns:` for exactly this shape —
a name rule with a second regexp, over the table, that gates whether the rule
is tried at all — and `refresh_token_parent` there is `credential` at
priority 80, scoped to tables named like `refresh_tokens`. It was the one
remaining false negative below; it is not any more.

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

Re-measured 2026-09-14 (T-0119), after T-0104's name rules, T-0121's
`public_key` decision and T-0119's table-scoped `refresh_token_parent` rule, by
the method under "Reproducing it" below: each schema loaded from its pin,
sliced with the catalogue's own root, `--take` and flags, and every entry of
the emitted `lazyslice.yml`'s `columns:` block scored against the labels at the
end of this file. Taken from the three per-schema reproductions
(`go test -tags 'integration torture' -run TestTortureSchemas/<django|rails-activestorage|supabase-auth> ./internal/invariants/`),
each green on its own, and not from a full `make torture` — the "It does not,
right now" note above records that the full suite fails elsewhere, on
regression `013`, a fixture none of these three schemas touches.

| Schema | Columns | Labelled personal | Predicted | TP | FP | FN | Precision | Recall |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| django | 44 | 9 | 11 | 9 | 2 | 0 | **0.818** | **1.000** |
| rails-activestorage | 36 | 7 | 13 | 7 | 6 | 0 | **0.538** | **1.000** |
| supabase-auth | 271 | 50 | 78 | 50 | 28 | 0 | **0.641** | **1.000** |
| all three | 351 | 66 | 102 | 66 | 36 | 0 | **0.647** | **1.000** |

**T-0315 (2026-09-24) moved rails-activestorage by one false positive and no
true positive**, and the table above carries it. `textsig.LooksSecret` no
longer reads a hex digest of exactly 32, 40 or 64 characters, a file name
carrying no dictionary word, a namespaced identifier or an environment
variable's name as a secret, the classifier does not ask it about a column
named `type`, `klass` or `component_name`, and it decides a column only over
five samples (ARCHITECTURE.md §4's T-0315 amendment).
`active_storage_blobs.checksum` (an MD5, labelled not-personal) is copied now:
14 → 13 predicted, precision 0.500 → 0.538; all three 103 → 102, 0.641 →
0.647; recall unchanged at 1.000. The one labelled-personal column the change
could have cost, `active_storage_blobs.filename` (`aoife-byrne-passport-3.pdf`),
is still masked at 200/200: a file name whose stem carries a dictionary word
is still read by the entropy check, and because the dictionary holds a name
in only 72 of every 100 of these, a column in which a fifth or more of the
file names carry one has every file name counted — a first measurement
without that rule copied this column, which is why the rule exists. django
and supabase-auth did not move. Measured the way T-0311 and T-0313 were, over
all ten schemas; the other seven moved 28 columns to copied and 3 to masked,
none of the 28 personal, and THREAT_MODEL.md T1's T-0315 amendment lists
them.

**T-0311 (2026-09-24) moved supabase-auth by three false positives and no
true positive**, and the table above carries it. The neighbouring-column
rule's second arm no longer sweeps a signal-less character column whose
samples are an enumeration or all one identifier shape (ARCHITECTURE.md §4's
T-0311 amendment), and three supabase-auth columns were exactly that:
`auth.users.aud` and `auth.users.role` (the single value `authenticated`)
and `auth.identities.provider` (three provider names), all labelled
not-personal — 81 → 78 predicted, 31 → 28 false positives, precision 0.617 →
0.641, recall unchanged at 1.000. django and rails-activestorage did not
move. This was measured as a before/after delta rather than re-scored from a
full run: each schema loaded from its four scripts and classified by
`lazyslice classify --json` with the binary before the change and after,
every column's masked-or-copied verdict compared. The other seven schemas,
which have no truth set, moved 24 columns between them and none is personal
data; THREAT_MODEL.md T1's T-0311 amendment lists them.

**T-0313 (2026-09-24) moved django and rails-activestorage by one false
positive each and no true positive**, and the table above carries it. A
column matched only by the rule pack's bare `name` word (`name`,
`display_name`, `<thing>_name`) now needs corroboration before it reaches
`possible` — a word for people in the table or column name, or at least a
fifth of its samples carrying a word from the name dictionary — and a
`*_file_name` column is outside the rule altogether (ARCHITECTURE.md §4's
T-0313 amendment). django's `auth_permission.name` ("Can add user", 0 of 200
samples a dictionary word) and rails-activestorage's
`active_storage_attachments.name` ("cover") are copied now: django 12 → 11
predicted, precision 0.750 → 0.818; rails-activestorage 15 → 14, 0.467 →
0.500; all three 105 → 103, 0.629 → 0.641; recall unchanged at 1.000.
supabase-auth did not move: its two labelled-personal bare names,
`mfa_factors.friendly_name` and `webauthn_credentials.friendly_name`, hold
no value in this fixture, so the name decides alone exactly as before (a
column with fewer than three samples is unproven, not clean); with values,
a device its owner named ("Grace's iPhone") carries the owner's name in a
possessive, which the dictionary now reads, and one named "YubiKey 5C" does
not, and is copied. Measured the way T-0311 was: each of the ten schemas
loaded from its four scripts and classified by `lazyslice classify --json`
with the binary before the change and after. Eighteen columns across the
corpus moved from masked to copied and none moved the other way; none is
personal data. Ten are bare names no word or sample corroborated —
django `auth_permission.name`, rails-activestorage
`active_storage_attachments.name`, plausible `funnels.name`,
`goals.display_name` and `goals.event_name`, calcom `Role.name` and
`Team.name`, discourse `groups.name`, `groups.imap_mailbox_name` and
`polls.name` — and eight are file names: mastodon's five Paperclip
`*_file_name` columns, discourse `user_exports.file_name`, odoo
`base_import_import.file_name`, and gitlab `push_rules.file_name_regex`,
which the file-name exclusion reaches as well (a regexp over file names, not
a person's). 237 bare-name columns across the ten are still masked on the name
alone because this corpus's fill leaves their tables empty or their values
NULL; real data is where the rule moves more, and dogfood session 1's 38
`name` columns are what it was written for.

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
recall 0.960. **T-0119 added the last one**: `refresh_tokens.parent`, also a
true positive and the false negative that had been left standing — 73 → 74
predicted, 49 → 50 TP, 24 FP unchanged, 1 → 0 FN, precision 0.671 → 0.676,
recall 0.980 → 1.000. supabase-auth's recall is 1.000 for the first time this
file has measured it, and no false negative is open on any of the three
schemas.

**The table moved once more between that measurement and this one, and not
because of anything this file's own tasks did to a name or a type rule.**
Re-measuring supabase-auth today — before T-0188 touched anything — reads 81
predicted, 50 TP, 31 FP, precision 0.617, against the 74/50/24/0.676 this file
last wrote down: the schema itself is unchanged (still 271 columns), so all
seven are rule-pack widening, not a new column. `git log` over
`internal/classify/{classify.go,rules.yml}` since this table's own T-0119
measurement names the mechanism: commit `a712cbc` ("Red team round 1 fixes",
T-REDFIX, 2026-09-15) widened `credential`, `online_id` and `person_name`'s
name patterns by roughly sixty spellings across several categories, and seven
of `custom_oauth_providers`' and its neighbours' OAuth-plumbing columns —
`discovery_url`/`token_url`/`userinfo_url` as `online_id`,
`provider_type`/`token_endpoint_auth_method` as `credential`,
`client_name`/`name_id_format` as `person_name` — now clear a name-pattern
match that did not exist when this table was last written. T-0187 (national_id
on the row path) is not the cause: no column in this schema decides
`national_id`, checked directly against the emitted `columns:` block. This
file was not re-measured after `a712cbc` landed, so the drift sat unrecorded
until T-0188's own before/after pass surfaced it; the table above is the
corrected "before" for what follows, not a new regression.

**T-0188 (2026-09-15) sourced and re-measured names.txt's multilingual stock
under CC0 (THIRD_PARTY_NOTICES.md) and moved none of the three schemas'
numbers at all**: django, rails-activestorage and supabase-auth are
12/9/3/0.750, 15/7/8/0.467 and 81/50/31/0.617 both before and after — the
same predicted set, column for column, not merely the same counts. The
reason is `TestFiftyNamesFromThreeSchemas` (internal/classify/names_test.go)
first: that fixture classifies sixty-four held-out columns by *name and type
only*, through `mapSampler{}` with no sampled values at all, so a value-level
dictionary change cannot move it by construction, and it did not — 47/2/0/15,
precision 0.959, recall 1.000, identical to before. The torture truth sets do
sample real rows, and still did not move, because none of django's,
rails-activestorage's or supabase-auth's generated data happens to contain a
word from any of the twenty languages T-0188 added — unsurprising for three
English-language open-source schemas' own generated fixtures, and the reason
`testdata/regressions/024` rather than one of the ten torture schemas is what
proves the dictionary change actually works: ten names, one per newly-sourced
language, each verified false against a pre-T-0188 `names.txt` and true
after, masked in a live run (`not-copied: public.reg024_records.label`).

**One thing the widening did cost, and the fix for it is in the same
change.** `TestCompositeAddressAcrossFieldsFailsClosed`
(internal/classify/composite_type_test.go) failed on the first pull: `Milano`
is both an Italian surname and Italy's second city, and a composite address
whose fields spelled `(3,"Via Roma",Milano)` decided `person_name` instead of
`address` once `Milano` entered the surname section — a family name and a
place name collide across languages in a way the English-only list never
surfaced. The fix is the third filter THIRD_PARTY_NOTICES.md's T-0188 section
describes (no candidate that is also a country, a capital, or a city over
100,000 population, read from Wikidata in English and in each native-script
language), and the full suite — this file's fixtures, the composite tests,
`TestDictionaryKeepsItsPrecisionRules`, the three per-language truth sets and
`make torture` in full — passes with it in place.

### django — precision 0.818, recall 1.000

Nothing personal was missed. The two false positives are both the same shape: a
column called `name` that is not a person's.

* `auth_group.name` — "Editors". `varchar(150) UNIQUE`, decided `person_name`,
  and the one flag this schema needs. Since T-0313 a bare `name` needs
  corroboration, and this fixture's group names carry a dictionary word in 4
  of 20 samples, which is the threshold, so it is still flagged.
* `django_migrations.name` — "0001_initial".

A third, `auth_permission.name` ("Can add user"), was one until T-0313: none of
its samples carries a dictionary word, so the bare name stays at `low` and is
copied.

Both of the schema's *hidden* carriers were caught: `django_admin_log.object_repr`
("User: bjorn.haddad1@borealis-works.test") at `likely`, and
`django_admin_log.change_message` at `possible`. Neither column's name says
anything.

### rails-activestorage — precision 0.538, recall 1.000

The worst precision of the three, and it is one validator. Six false
positives, most of them `LooksSecret` firing on a value that is merely long
and mixed: a `variation_digest`, an ActiveStorage `key`, a
`content_type` of `application/pdf`, and `ar_internal_metadata.key` holding
"environment" (the checksum was a sixth until T-0315, which reads an MD5 as a
digest and copies it). The last is `person_name` on `service_name` ("local"), a bare
name no word or sample corroborates, raised to `possible` by the
neighbouring-column rule beside `filename`. `active_storage_attachments.name`
("cover") was an eighth until T-0313 and is copied now.

Nothing personal was missed, including the one that matters:
`active_storage_blobs.filename` — `ana-aluko-passport-3.pdf` — is caught at
`likely` with no name signal at all.

### supabase-auth — precision 0.676, recall 1.000

The schema with the false negative, and it is the reason this schema is in the
set — though recall is 1.000 now, and there is none left. It had ten, and nine
of them were names no rule covered:

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
| `refresh_tokens.parent` | 400, a quarter populated | another refresh token | **masked (T-0119)** |

**Seven of the ten were columns with nothing in them**, where only the name could
have decided — which is precisely the case a name rule exists for, and precisely
where the rule pack was thin: it matched `tokens?`, `secrets?`, `passwords?` and
`api_keys?`, and none of `code`, `verifier`, `credential_id` or `public_key`.
That was T-0104, and the `Now` column is the fix scored against the same table.

**The tenth was the sharpest of the ten and the only one with data in it**:
`refresh_tokens.parent` is a refresh token in a column named after a tree edge,
populated, and missed by name and by value alike. There was no *name* pattern to
write — `parents?` would mask the join keys of half a database at priority 80,
and this rule pack's plain name rules see a column name without its table — so
the fix is `rules.yml`'s `table_patterns:` (T-0119): a name rule gated by a
second regexp over the table, tried only within a table that regexp matches.
`refresh_token_parent` there is `credential` at priority 80, scoped to
`(^|_)refresh_?tokens?(_|$)`, and catches the column by name alone in a run with
nothing in it to read — `textsig.LooksSecret` would catch the value in
production, where the column holds real tokens, but it was empty in this
fixture, which is exactly the gap a name rule closes and a value signal cannot.
It is pinned by `TestSupabaseAuthMissesArePinned`.

The twenty-four false positives are mostly one table — `custom_oauth_providers`,
nine of them, where a deployment's OAuth endpoints (`token_url`, `discovery_url`,
`userinfo_url`) read as secrets or as an `online_id` — plus `provider_type` and
`authentication_method`, which are short enum-ish strings that clear the entropy
threshold, `sso_domains.domain`, which is an organisation's, four `jsonb`
columns masked on their type alone, and the two `code_challenge` columns T-0104
added. None of the twenty-four moved with T-0119: the table-scoped rule is
anchored to `refresh_tokens.parent` alone and touches nothing else in the
schema.

**What the recall number does not measure.** These are the classifier's
decisions, not the run's outcome. A column the classifier misses is copied
verbatim, so a miss here *is* a leak — which is why the grep half of I2 runs over
every torture target as well, and why defect 8 was found by that and not by
this table.

**So phase 5's gate evidence, re-measured, is zero leaks of the fifty labelled
columns on the ten schemas** — a claim this paragraph used to be unable to make.
It was ten when this file was written, and two of those ten were populated in
the fixture that I2's grep half cannot see on its own — `identities.provider_id`
(300 rows, the provider's subject id for the person) and `flow_state.auth_code`:
that half knows email addresses and phone numbers, and a subject id is neither.
`refresh_tokens.parent` (400 rows, a quarter populated) was the third, and the
last: I2's grep half does not know a refresh token by shape either, so the name
rule T-0119 added is what closed it, the same way the identity provider's
columns needed T-0104's rather than the grep.

All ten are still pinned, one by one, by `TestSupabaseAuthMissesArePinned`
(`internal/classify/supabase_misses_test.go`), which runs in `make test` on
every change: all ten now assert the masking. It still fails in **both**
directions — an eleventh miss, or any of these ten reverting to a leak —
without this table being re-measured, so the number cannot get quietly worse,
and cannot get better without the doc being updated with it.

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
