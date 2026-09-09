# Torture testing

Ten real open-source PostgreSQL schemas, at a pinned commit or image digest,
each filled with generated rows, sliced from its most-connected table into a
second container, and put through the invariants.

```
make torture
```

Nine of the ten snapshot cleanly **with forty-five flags between them —
thirty-seven `--unmask`, seven `--skip-table` and one `--key` — twenty of the
thirty-seven being one unfixed defect (T-0098)**. That whole sentence is the
result, and the split is part of it rather than a footnote: `--unmask` copies a
column of personal data into the target verbatim and `--skip-table` drops a
table, so the two are not interchangeable evidence and the total is never
quoted here without them. A stranger pointing lazyslice at GitLab types eleven
`--unmask` flags before it runs. The tenth, Mastodon, refuses at exit 13 for a
reason ARCHITECTURE.md §11.1 states, and the suite asserts that refusal by name.
Between them the ten found **eight defects**, every one of which is now a file in
`testdata/regressions/` and a fix in `internal/`, and **nine more** that are
filed rather than fixed because the change belongs somewhere this task could not
reach.

Two things the nine clean runs do **not** say, both measured below and both
carrying a task: supabase-auth's classifier misses ten of the fifty columns the
hand-labelling calls personal, and each of those is copied into the target in
cleartext under exit 0 (recall 0.800, T-0104); and `credential`'s only masker
has a domain of one, which is where twenty of the thirty-seven `--unmask` flags
come from (T-0098).

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
same table below to a tenth of a second. The fixtures carry the other half of
the fix — a generated name may not be a word in the maskers' own lists, which is
the same rule `testdata/torture/README.md` already stated for `example.com`,
RFC 5737 and `555-01XX` — so the collision is impossible rather than merely
unlucky under this key.

| Schema | Tables | FKs | Columns | Root | Flags | Time | Result |
|---|---:|---:|---:|---|---:|---:|---|
| [rails-activestorage](../testdata/torture/rails-activestorage/README.md) | 7 | 3 | 36 | `public.active_storage_blobs` | 1 | 3.8 s | clean |
| [django](../testdata/torture/django/README.md) | 10 | 9 | 44 | `public.auth_user` | 1 | 2.3 s | clean |
| [supabase-auth](../testdata/torture/supabase-auth/README.md) | 27 | 24 | 271 | `auth.users` | 7 | 3.1 s | clean |
| [plausible](../testdata/torture/plausible/README.md) | 42 | 40 | 294 | `public.sites` | 2 | 2.7 s | clean |
| [gitlab](../testdata/torture/gitlab/README.md) | 43 | 116 | 968 | `public.namespaces` | 11 | 4.6 s | clean |
| [metabase](../testdata/torture/metabase/README.md) | 100 | 130 | 892 | `public.core_user` | 4 | 3.6 s | clean |
| [calcom](../testdata/torture/calcom/README.md) | 102 | 179 | 1,092 | `public.users` | 2 | 4.5 s | clean |
| [mastodon](../testdata/torture/mastodon/README.md) | 118 | 156 | 1,018 | `public.accounts` | 5 | 2.6 s | **exit 13** |
| [odoo](../testdata/torture/odoo/README.md) | 204 | 622 | 2,048 | `public.res_users` | 3 | 5.5 s | clean |
| [discourse](../testdata/torture/discourse/README.md) | 370 | 29 | 3,372 | `public.users` | 9 | 7.6 s | clean |
| **total** | **1,023** | **1,308** | **10,035** | | **45** | **40 s** | 9 clean, 1 refused |

The whole `make torture` target is 58 seconds: the 40 above, plus 16 for the
eight regressions and a second for the two catalogue guards.

"Clean" means exit 0 with every check in `TestTortureSchemas` passing: the
tables the slice had to reach hold rows and, where a correct run cannot reach
them all, fewer rows than the source; every foreign key in the target resolves
and every source edge between two present tables was recreated (I1); the source
is unchanged in rows and in catalog (I4); the counted root holds exactly
`--take` rows (I6); and no email address or phone number of the source's survives
anywhere in the target (the grep half of I2).

**"Flags" is the number a first run demanded and named**, counting every flag
of any kind: of the forty-five, thirty-seven are `--unmask`, seven are
`--skip-table` (plausible 1, metabase 2, discourse 4) and one is `--key`
(discourse). It is the honest headline number of this exercise: a stranger
pointing lazyslice at GitLab types eleven `--unmask` flags before it runs.
Twenty of the thirty-seven `--unmask` flags are one defect (T-0098), and the
table above is what will say by how much it improves when that lands. The
newest of the thirty-seven carries the longest reason:
`auth.mfa_factors.friendly_name` is under a *partial* composite unique index
that admits none of its rows, and `d_required` is computed over the whole
table's row count anyway — the over-estimate T-0099 owes an exact rule for.
`internal/invariants/torture_catalogue_test.go` carries the whole reason beside
the flag.

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

Eight. Each has a file in `testdata/regressions/` — the smallest schema that
still shows it — written before the fix, and each of those files is run by
`make torture`.

| # | Regression | Found by | What was wrong | Fixed in |
|---|---|---|---|---|
| 1 | `001-unique-index-masking-collision.sql` | django, rails-activestorage, supabase-auth | ARCHITECTURE.md §5's unique-index domain rule was never called. `mask.Pick` implemented it; nothing invoked it, and `internal/transform` used the rule pack's default masker whatever the column's constraints were. A masked column under a unique index collided **in the loader**, after every row had moved: `duplicate key value violates unique constraint "auth_group_name_key" (SQLSTATE 23505)`. | `internal/plan/unique.go` (new): after the walk, every masked column under a unique index is held to `d_required = n²/2ε`; the widest generator for the category is chosen and written back onto the decision, or the run refuses at exit 12 under the new code `plan.refused.unique_domain`, printing d, d_required and §5's three escapes. |
| 2 | `002-function-default-refusal-uncoded.sql` | mastodon, gitlab | §11.1's exit-13 refusal arrived as exit 1, `run.refused.internal`, "run with --debug", with its own code printed *inside* the message of the wrong one. `core.asStop` converts five stage refusal types and `*ddl.Refusal` was a sixth that nothing converted. | `internal/core/names.go`: the missing case, plus a message that does not print the code twice. |
| 3 | `003-composite-unique-index-is-not-a-unique-column.sql` | rails-activestorage, supabase-auth, calcom, gitlab, mastodon | `Decision.UniqueIndex` was set for **every** column of every unique index. Once defect 1's check started reading it, five schemas refused at plan over columns that cannot collide — ActiveStorage's `name`, the literal `'cover'`, one of four in `(record_type, record_id, name, blob_id)`. | `internal/classify/classify.go`, `indexKeys`: a column is raised when it carries the uniqueness *alone*. |
| 4 | `004-composite-unique-index-all-masked.sql` | django | The other side of 3. `django_content_type` is `UNIQUE (app_label, model)` with both columns masked, and nothing was left to hold the tuple apart: `duplicate key value violates unique constraint "django_content_type_app_label_model_76bd3d3b_uniq"`. | `internal/classify/classify.go`, `raiseCompositeUnique`: a composite index — total, partial or expression — raises its masked columns unless an unmasked key column's sample has no repeats, where "no repeats" requires the column to have been *sampled*: a key column with no samples in a table nothing could be sampled from holds nothing apart, while one with no samples in a table that *was* sampled is NULL in every row and holds the tuple apart on its own (calcom's `Role_name_teamId_key`), unless the index is `NULLS NOT DISTINCT`. §5 does not state the composite rule; the approximation and its known imprecisions are T-0099. |
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

One more change is not a defect in the pipeline but in the harness, and it is
here because the suite is what measured it: `internal/testutil`'s wait for
Docker's port table was ten attempts over five seconds, which `make torture`'s
forty container starts exhausted about once a run, failing a different schema
each time with a message about a port. It is thirty seconds now, and costs
nothing when the port is there.

## Found and not fixed

Nine, each with a tracker task, because the change belongs in a file this task
could not write (`mask/` is ADR-006's own module; ARCHITECTURE.md is the
architecture; `.golangci.yml` is neither) or is a decision rather than a repair.

| Task | Finding |
|---|---|
| **T-0098** | **`credential`'s only masker has a domain of exactly 1**, so no column under a unique index that classifies as a credential can be masked at all — `MaxRows(1)` is zero, so "lower `--take`" is not an escape either. **Twenty of the thirty-seven `--unmask` flags in the table above are this**, six of them in supabase-auth alone. An authentication schema is nothing but unique credentials. The fix is a second `CatCredential` generator in `mask/`, the shape `phone_unique` and `ip_unique` already have. |
| T-0099 | §5 states the unique-index rule for *a column* and says nothing about composite or partial indexes. Defects 3, 4 and 7 are all approximations of a rule the architecture has not written. |
| T-0097 | §11.1 says the not-recreatable refusal is raised **at plan**; it is still raised inside `load.Load`, so mastodon and gitlab pay for a full extract before being told the target cannot be built. |
| T-0100 | `textsig.LooksSecret` classifies a **URL** as a credential — `https://home.social.test/users/bea_donnelly1` clears its entropy test — which is safe but wrong, and under a unique index becomes a flag the operator has to type. Two of mastodon's five. |
| T-0101 | `internal/plan` now writes the escalated masker back onto the decision, and `Classification.Fingerprint` was computed before that, so "classification changed — masked values will differ" would not print for a column that became unique. |
| T-0102 | A **text column holding a JSON document** is invisible to §4's JSON rule: the scalar validators do not match a document and the column is copied. |
| T-0103 | An **array of an extension type** is sampled as one opaque string, so the classifier never sees the addresses inside a `citext[]`; and if it does decide to mask it, the masked scalar cannot be encoded into the array column at all. |
| T-0105 | `.golangci.yml`'s `run.build-tags` lists `integration` alone, so the 821 lines behind `integration && torture` are linted by nothing. `make check` now runs `vet-tagged`, which type-checks and vets them under both tag sets; linting them is one line in a file outside this task's paths. |
| T-0104 | The `credential` and `online_id` name rules miss the spellings an auth schema uses — `auth_code`, `otp_code`, `code_verifier`, `credential_id`, `public_key`, `external_id`, `provider_id`. That is every one of the ten misses in the supabase-auth truth set below. |

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

| Schema | Columns | Labelled personal | Predicted | TP | FP | FN | Precision | Recall |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| django | 44 | 9 | 12 | 9 | 3 | 0 | **0.750** | **1.000** |
| rails-activestorage | 36 | 7 | 15 | 7 | 8 | 0 | **0.467** | **1.000** |
| supabase-auth | 271 | 50 | 62 | 40 | 22 | 10 | **0.645** | **0.800** |
| all three | 351 | 66 | 89 | 56 | 33 | 10 | **0.629** | **0.848** |

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

### supabase-auth — precision 0.645, recall 0.800

The one with false negatives, and they are the reason it is in the set. All ten
are columns no name rule covers:

| Missed column | Rows in the fixture | What it holds |
|---|---|---|
| `flow_state.auth_code` | 100, column NULL | the OAuth authorization code |
| `identities.provider_id` | 300, populated | the provider's subject id for the person |
| `refresh_tokens.parent` | 400, a quarter populated | another refresh token |
| `mfa_challenges.otp_code` | 0 | the one-time code |
| `mfa_recovery_codes.code_hash` | 0 | a recovery code |
| `oauth_authorizations.authorization_code` | 0 | the authorization code |
| `oauth_client_states.code_verifier` | 0 | the secret half of PKCE |
| `scim_users.external_id` | 0 | the IdP's id for the person |
| `webauthn_credentials.credential_id` | 0 | the authenticator's credential id |
| `webauthn_credentials.public_key` | 0 | a stable per-person identifier |

**Seven of the ten are columns with nothing in them**, where only the name could
have decided — which is precisely the case a name rule exists for, and precisely
where the rule pack is thin: it matches `tokens?`, `secrets?`, `passwords?` and
`api_keys?`, and none of `code`, `verifier`, `credential_id` or `public_key`.
That is T-0104, and this table is the measurement a fix should be scored against.

The three with data are worth separating out. `refresh_tokens.parent` is the
sharpest: a refresh token in a column named after a tree edge, populated, and
missed by name and by value alike.

The twenty-two false positives are mostly one table — `custom_oauth_providers`,
nine of them, where a deployment's OAuth endpoints (`token_url`, `discovery_url`,
`userinfo_url`) read as secrets — plus `provider_type` and
`authentication_method`, which are short enum-ish strings that clear the entropy
threshold, and `sso_domains.domain`, which is an organisation's.

**What the recall number does not measure.** These are the classifier's
decisions, not the run's outcome. A column the classifier misses is copied
verbatim, so a miss here *is* a leak — which is why the grep half of I2 runs over
every torture target as well, and why defect 8 was found by that and not by
this table.

**So phase 5 closes with a measured, reproducible leak of ten of the fifty
labelled columns on one of the ten schemas**, and that belongs in the gate's
evidence rather than in this paragraph alone. Three of the ten are populated in
the fixture — `identities.provider_id` (300 rows, the provider's subject id for
the person), `refresh_tokens.parent`, `flow_state.auth_code` — and I2's grep
half cannot see any of them: it knows email addresses and phone numbers, and a
subject id is neither. The fix is T-0104 and it is a *scored* change: widening
`credential` and `online_id` to match `code`, `verifier`, `credential_id`,
`public_key`, `external_id` and `provider_id` will move precision as well, and
the number to beat is the row above.

Until it lands the ten are pinned by
`TestSupabaseAuthMissesArePinned` (`internal/classify/supabase_misses_test.go`),
which runs in `make test` on every change and asserts each of them exactly as it
is: a name-only classification, below the mask threshold. It fails in **both**
directions — an eleventh miss, or one of these ten becoming masked without this
table being re-measured — so the number cannot get quietly worse, and cannot get
better without the doc being updated with it. A test that asserts a leak is an
uncomfortable thing to write and a worse thing to lose track of; that is why it
names T-0104 in its own failure message.

## Reproducing it

```
make torture                                        # all ten and the regressions
go test -tags 'integration torture' -run TestTortureSchemas/gitlab ./internal/invariants/
testdata/torture/build.sh gitlab                    # rebuild one schema.sql from its pin
```

`testdata/torture/README.md` is the fixtures' spec, each schema's own
`README.md` is its pin and its deviations, and
`internal/invariants/torture_catalogue_test.go` is the catalogue of runs. The
truth sets above were produced by reading the `columns:` block of each run's
emitted `lazyslice.yml` and scoring it against the labels below.

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
identifier by use. `raw_app_meta_data` is labelled *not* personal, because what
GoTrue puts there is the provider list — but a deployment that put anything else
there would move it.
