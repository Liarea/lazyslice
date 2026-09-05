# testdata

Two PostgreSQL fixtures. Between them they are the definition of correct for
this repository: `pagila/` is the friendly schema a person would plausibly have
designed, and `nasty.sql` is every shape that has ever broken a subsetting tool,
put in one place on purpose.

Nothing in `nasty.sql` is decoration. Every object in it appears in the table
below with the behaviour lazyslice must show for it, and a trap that stops
being interesting should be deleted from both files in the same commit.

```
psql -f testdata/pagila/pagila-schema.sql -f testdata/pagila/pagila-data.sql
psql -f testdata/nasty.sql              # fast: public.stream_rows stays empty
psql -v big=1 -f testdata/nasty.sql     # also fills stream_rows with 2,000,000 rows
```

From Go, `internal/testutil.LoadPagila(ctx, url)` and
`internal/testutil.LoadNasty(ctx, url, big)`.
`internal/testutil/fixtures_test.go` loads both under `make integration` and
asserts, in two passes: the table list and every row count below, and then the
traps themselves out of the catalogue — the enum, the generated column, the
identity columns and their sequence positions, the partitioning, the two
composite foreign keys and their match types, the unique-index and no-identity
tables, and the key column types. Strip `GENERATED ALWAYS` off a column or
un-partition a table and that test fails, rather than a phase 4 test failing
later and saying something else. Change a fixture and change that test with it,
deliberately, in the same commit as this file.

---

## pagila/

Upstream: [devrimgunduz/pagila](https://github.com/devrimgunduz/pagila), tag
`pagila-v3.1.0`, commit `fef9675714cfba1756df4719b5e36075a7ddf90e`. The two SQL
files are byte-for-byte copies of upstream's, so the pin can be checked with
nothing but `curl` and `shasum`:

| File | SHA-256 |
|---|---|
| `pagila-schema.sql` | `8ce358e4c8014087b85296694a0893887bd7a4190e3ce407f2721b86b98e5707` |
| `pagila-data.sql` | `fb81bec377687c83e11d2a24916ae28656d85550bf0ada798305bf7e2af9823b` |
| `LICENSE.txt` | `516e7dac679ac1eeb62d5614b01c4e7318154e9a147377d6264954215997ff38` |

```
curl -sSL https://raw.githubusercontent.com/devrimgunduz/pagila/fef9675714cfba1756df4719b5e36075a7ddf90e/pagila-schema.sql | shasum -a 256
```

Pagila is under the same modified BSD licence as the Sakila database it is
ported from; `LICENSE.txt` is upstream's, copied alongside the data.

**Why the tag and not `master`.** ADR-003 puts the CI matrix on PostgreSQL 14
and 18 and `internal/testutil` defaults to 16. Pagila's current `master`
(`pagila-v4.1.0`) needs `CREATE EXTENSION vector`, `uuidv7()` and
`GENERATED ... AS (...) VIRTUAL`, so it loads on nothing before PostgreSQL 18
and on no stock image at all. `pagila-v3.1.0` needs no extension and loads
unchanged on 14, 16 and 18, which is the whole span this project supports.
Reversal condition: when the supported floor moves to 18 and the images carry
pgvector, re-pin to `master` and update the counts below.

**Two things `LoadPagila` does that the files do not.** The dump ends most
objects with `ALTER ... OWNER TO postgres` and the test container's superuser is
not called `postgres`, so the loader creates that role when it is missing; and
it runs `ANALYZE` at the end, because `pg_class.reltuples` is `-1` until
something does and both the classifier's partition choice and the planner's
estimates read it.

The role is created with no attributes: `CREATE ROLE postgres`, not
`CREATE ROLE postgres SUPERUSER`. Assigning ownership needs the *connecting*
user to be a superuser, never the target role, and pagila defines
`public.rewards_report` as `SECURITY DEFINER` with no `SET search_path` and
`EXECUTE` left to `PUBLIC`. A superuser `postgres` would therefore leave every
fixture database holding a superuser-owned `SECURITY DEFINER` function with a
mutable search path that any role in the database can call — created as a side
effect of a test helper, which is not a thing a test helper should create even
in a container that is thrown away.

### What Pagila brings

| Shape | Where | Why it matters |
|---|---|---|
| Partitioned table | `payment`, seven monthly leaves | The realistic version of the `nasty.sql` trap: a partitioned root with real data behind it |
| Materialised view | `rental_by_category` | Not a table. It must not be planned, extracted or loaded, and it must not appear in a table count |
| Views | `actor_info`, `customer_list`, `film_list`, `nicer_but_slower_film_list`, `sales_by_film_category`, `sales_by_store`, `staff_list` | Same |
| Enum | `mpaa_rating` | `film.rating` |
| Domains | `public.year`, `public."bıgınt"` | The second is a quoted identifier containing a dotless Turkish i. Any identifier lowercasing lazyslice does must be Unicode-correct, and any case-folding must not be locale-dependent |
| `tsvector` | `film.fulltext` | A derived column with a `BEFORE INSERT OR UPDATE` trigger behind it. It carries no personal data but it is not a type a masker may guess at |
| `text[]` | `film.special_features` | An array that is not personal data, next to `nasty.sql`'s array that is |
| Aggregate and functions | `group_concat`, `rewards_report`, ... | §11.1 does not recreate them in the target; they exist so that "did not recreate" is a checkable claim rather than an untested one |
| Trigger | `film_fulltext_trigger` | Must not fire during a load. The loaded rows are the source's rows, not re-derived ones |
| Two FKs from one child into one parent | `film.language_id` and `film.original_language_id`, both into `language` | An edge set keyed by `(child, parent)` instead of by constraint name silently loses one of these. Pagila v3.1.0 has **no** self-referencing foreign key; `people.manager_id` in `nasty.sql` is the only fixture for that shape |
| Sequences | one per table, `setval` at the end of the data file | Sequence state is part of the source, and the target's must not be left at 1 |

### Row counts

`internal/testutil/fixtures_test.go` asserts exactly this list: 22 tables
(`relkind` `r` or `p`), no more and no fewer, with these counts. `payment` is
the partitioned root, so its count is the sum of its seven leaves.

| Table | Rows | | Table | Rows |
|---|---:|---|---|---:|
| `public.actor` | 200 | | `public.payment` | 16049 |
| `public.address` | 603 | | `public.payment_p2022_01` | 723 |
| `public.category` | 16 | | `public.payment_p2022_02` | 2401 |
| `public.city` | 600 | | `public.payment_p2022_03` | 2713 |
| `public.country` | 109 | | `public.payment_p2022_04` | 2547 |
| `public.customer` | 599 | | `public.payment_p2022_05` | 2677 |
| `public.film` | 1000 | | `public.payment_p2022_06` | 2654 |
| `public.film_actor` | 5462 | | `public.payment_p2022_07` | 2334 |
| `public.film_category` | 1000 | | `public.rental` | 16044 |
| `public.inventory` | 4581 | | `public.staff` | 2 |
| `public.language` | 6 | | `public.store` | 2 |

---

## nasty.sql

21 tables, 5 people, and one trap per thing that goes wrong. It needs no
extension and no superuser, and it loads unchanged on PostgreSQL 14, 16 and 18.

| Table | Rows | | Table | Rows |
|---|---:|---|---|---:|
| `billing.invoices` | 3 | | `public.orders` | 5 |
| `public."LegacyCustomer"` | 3 | | `public.organisations` | 2 |
| `public.attachments` | 4 | | `public.people` | 5 |
| `public.audit_log` | 4 | | `public.projects` | 2 |
| `public.click_stream` | 3 | | `public.sites` | 2 |
| `public.device_readings` | 4 | | `public.stream_rows` | 0 (2,000,000 with `big`) |
| `public.devices` | 3 | | `public.teams` | 2 |
| `public.events` | 7 | | `public.tenant_user_flags` | 3 |
| `public.events_2024` | 5 | | `public.tenant_user_sessions` | 5 |
| `public.events_2025` | 2 | | `public.tenant_users` | 4 |
| `public.order_items` | 7 | | | |

### The traps

#### Structure

**1. Self-referencing foreign key** — `people.manager_id -> people.person_id`.

A walk with no visited set never terminates here. lazyslice must expand a
manager chain once per row, stop, and report `people` once in the plan rather
than once per level. `--take 1` rooted at Katherine Johnson (90021) pulls Grace
Hopper (90007) and then Ada Lovelace (90000) as parents, and stops.

**2. Three-table foreign-key cycle** — `organisations -> teams -> projects ->
organisations`.

There is no valid table order for a loader that sorts by dependency, which is
exactly why ARCHITECTURE.md §11.1 loads data first and adds foreign keys after.
The planner must terminate; the loader must succeed; and the target must end
with all three constraints present and valid. `organisations.primary_team_id` is
`NOT NULL`, so "just leave the cycle-closing column null" is not an escape.

**3. A second cycle, of length two, that is also a parent-and-child pair** —
`people.preferred_order_id -> orders` and `orders.person_id -> people`.

This is the case ARCHITECTURE.md §3 fixes the outcome of. `preferred_order_id`
pushes `orders` as `PARENT_ONLY`; `person_id` pushes the same rows as
`CHILD_OK`; under FIFO the parent batch pops first, so a person's preferred
order that is also one of their own orders (Ada's 200000) is expanded as a
parent and does **not** pull its `order_items` through that batch — the same
rows arrive in the child batch, are already selected, and are skipped. Two runs
over one snapshot must produce byte-identical `selected` sets.

**4. Composite primary key with a composite foreign key to it** —
`tenant_users (tenant_id, user_id)`, referenced by
`tenant_user_sessions (tenant_id, user_id)`.

Key sets here are pairs. Every chunked read, every `unnest` and every emitted
`where` has to carry both columns in the declared order. A one-column shortcut
anywhere gives a slice that is wrong rather than an error: tenant 1 user 1 and
tenant 2 user 1 are different people, and `tenant_users` contains both.

`tenant_user_sessions.user_id` is **nullable**, and session 5044 has a tenant
and no user. That row's parent edge must not be followed: the constraint is
`MATCH SIMPLE`, under which a composite foreign key with any `NULL` component
references nothing at all, which is why ARCHITECTURE.md §3's parent step reads
`WHERE ∀c ∈ fk.ChildCols: c IS NOT NULL`. An implementation that writes ANY
instead of ALL pulls a `tenant_users` row that session 5044 does not reference;
one that drops the predicate can miss a parent instead and break invariant I1.
Both failures are silent on a fixture where every composite child column is
`NOT NULL`, which is why this one is not.

**5. The same composite foreign key, `MATCH FULL`** —
`tenant_user_flags (tenant_id, user_id)`, also into `tenant_users`.

`ForeignKey.MatchFull` is a field in ARCHITECTURE.md §2, so it needs a fixture,
and the rule it selects is the opposite one: a row must have all of the
referencing columns `NULL` or none of them. Flag 9004 has both `NULL`, which is
legal and references nothing; a half-`NULL` row cannot be inserted at all.
Introspection must report `confmatchtype` `f` here and `s` on
`tenant_user_sessions` — two constraints over the same pair of columns with
different meanings.

**6. Polymorphic association with no constraint** —
`attachments.owner_type` (`'people'` or `'projects'`) and `attachments.owner_id`.

PostgreSQL knows nothing about this pair, so lazyslice must not follow it, and
must not be quiet about not following it. Until §3.2 lands the required output
is the line

```
polymorphic pair detected, not followed: no constraint
```

and the rows stay out of the slice. research/COMPLAINTS.md FK-10 is the
silently empty slice this trap exists to make impossible. Attachment 839 points
at `people` 99999, which does not exist, so an implementation that does follow
the pair one day still has to survive a dangling owner rather than fail the
load.

**7. Partitioned table with two partitions** — `events`, with `events_2024`
(5 rows) and `events_2025` (2 rows).

The root is `relkind` `p` and holds no rows of its own. `TABLESAMPLE` is refused
on it, so the classifier samples the largest leaf by `reltuples` — `events_2024`,
deliberately the larger — attributes the samples to the root, sets
`Table.SampledFrom` to the leaf and says `samples from partition events_2024` in
the explanation. The planner and the loader address `public.events`; nothing
addresses a leaf by name. The primary key is `(event_id, occurred_at)` because
it has to contain the partition key, which is a second composite key arrived at
the way real schemas arrive at one.

**8. A table outside `public`** — `billing.invoices`, with a foreign key to
`public.people`.

Nothing may assume `search_path`. Every table is named `(schema, name)` all the
way through, the edge carries the schema of both ends, and the target must have
schema `billing` created before the table that lives in it.

**9. Quoted, mixed-case identifier** — `public."LegacyCustomer"`, with columns
`"CustomerID"`, `"EmailAddress"`, `"MobileNumber"`, `"Notes"`.

These objects exist only when quoted. Every statement lazyslice generates — the
count probe, the chunked read, the residual scan, the `COPY` target, the
emitted `lazyslice.yml` — must quote them. An identifier concatenated into SQL
without quoting fails here with `42P01` rather than quietly reading something
else, which is the point: this trap turns a class of silent bug into a loud one.

**10. Keys that are not integers** — `sites.site_code text`,
`devices.device_id uuid`, and `device_readings (device_id uuid, taken_at
timestamptz)`.

ARCHITECTURE.md §2 encodes one chunk of key tuples as one typed array per
identity column: `[]int64` for `int2`/`int4`/`int8`, `[]string` for `text`,
`varchar`, `bpchar` and `citext`, `[]pgtype.UUID` for `uuid`, and the text form
with a cast for everything else. Every other key in `testdata/` is an integer or
a timestamp, so without these three tables the `[]string` and `[]pgtype.UUID`
branches and their `::text[]` / `::uuid[]` casts would ship untested — and a
wrong cast there is not loud: `unnest($1::text[])` joined against a `uuid`
column matches nothing and returns a chunk smaller than the one it was given,
which is research/COMPLAINTS.md FK-10's silently empty slice again.

Each has a child to follow the edge into (`sites` → `devices` →
`device_readings`), and `device_readings` puts a typed column and the text-form
fallback in one composite key. Both `sites` and `devices` have an outgoing
foreign key, so neither is lookup-shaped under §3 and both are read in chunks
rather than copied whole.

**11. No primary key, but one unique index that will do** — `audit_log`.

ARCHITECTURE.md §3.4's identity ladder is `--key` → primary key → unique index
(non-partial, non-expression) → probed pseudo-key → refuse. Every other table
here stops at the first rung; `audit_log` starts at the third. It carries three
unique indexes and only one of them is a legal identity:

| Index | Kind | Usable as identity |
|---|---|---|
| `audit_log_entry_uid_key` | plain, on `(entry_uid)` | yes — this is the answer |
| `audit_log_recent_action_key` | partial, `WHERE occurred_at >= '2025-01-01'` | no |
| `audit_log_lower_entry_uid_key` | on the expression `lower(entry_uid)` | no |

Identity for `audit_log` must come out as `IdentityUnique` over `(entry_uid)`.

**12. No row identity at all** — `click_stream`.

No primary key, no unique index, and two rows identical in every column, so a
pseudo-key probe has nothing to find either: no set of columns identifies a row.
§3.4 says lazyslice stops rather than guesses, and this is the one table in
`testdata/` whose required behaviour is a refusal —

```
exit 12, naming public.click_stream and --key public.click_stream=col,col
```

— because a guessed identity produces a slice whose rows are silently the wrong
ones. A regression that makes the planner guess shows up here and nowhere else.

#### Types

**13. Enum type** — `public.account_status`, used by `people.status`.

The type must exist in the target before the table that uses it. The classifier
must not treat it as free text. A masker that replaced a value with an
arbitrary string would fail the load with `22P02`, so the correct behaviour for
an enum is to leave it alone unless the category says otherwise.

**14. Generated column** — `people.display_name`, `GENERATED ALWAYS AS
(given_name || ' ' || family_name) STORED`.

Two rules at once. The loader must not name it in a `COPY` column list, because
PostgreSQL rejects a write to it (`428C9`). And the masker must not have to:
its inputs are masked, so the target's `display_name` is derived from the masked
names by the target itself. A residual scan that flags `display_name` while
`given_name` and `family_name` are clean has found a bug in the load, not in the
mask.

**15. Array of email addresses** — `people.alt_emails text[]`.

An array is not a scalar and not free text. The classifier must reach the
element type, and the masker must map each element and return an array of the
same length, with `NULL` preserved as `NULL` — three of the five rows have
values, two are `NULL`.

**16. JSONB with personal data nested two levels deep** — `people.contact` and
`events.payload`.

```json
{"profile": {"contact": {"email": "ada.lovelace@example.com",
                         "phone": "+44 20 7946 0958"}, "locale": "en-GB"},
 "tags": ["founder"]}
```

The email and the phone are at `profile.contact.email` and
`profile.contact.phone`, below the one level of JSON key collection the v1 rule
pack reaches (ARCHITECTURE.md §14). So the required v1 behaviour is not "find
them": it is that the column is classified as carrying personal data on the
strength of its sampled values, is masked as a whole, and that the residual scan
over the target finds no address from the source in it. Silently shipping the
document unchanged is the failure this trap catches.

**17. Free text with full names in it** — `people.notes` and
`public."LegacyCustomer"."Notes"`.

`Ada Lovelace asked that Grace Hopper be copied on the renewal. Call back on
+44 20 7946 0958.` Names in prose are the case the English name dictionary and
the entropy check exist for. The column must be classified as free text, masked
as free text, and the residual scan must find no source name or phone in the
target — including names that belong to a *different* row than the one the note
is on, which is why the notes cross-reference each other.

**18. `inet`** — `tenant_user_sessions.client_ip`, IPv4 and IPv6.

A type signal on its own: the name says nothing. `2001:db8::1` is in there so
that a masker that only understands dotted quads fails visibly.

#### Names that lie

**19. False positive: `people.email_verified boolean`.**

The name matches every email rule anyone would write. The type says it cannot
be an address, and the values are `true`/`false`. lazyslice must **not** mask
it: masking a boolean produces either a load failure or a silently inverted
flag, and either way the reason line would have said "email" about a column that
never held one. The type check has to beat the name check here.

**20. False negative: `people.ref text`, holding email addresses.**

The name says nothing at all. Only the sampled values say what it is, and
`net/mail.ParseAddress` accepts every one of them. lazyslice must mask it, and
the reason line must say the values decided it, not the name. This is the column
that makes "name rules only" indefensible.

`attachments.uploaded_by` is the same trap turned down: a name that hints at a
person without matching any email rule, over values that are unambiguous
addresses. `billing.invoices.bill_to_email` is the easy case both signals agree
on, and it is here so that "the name rule fired" and "the validator fired" can
be told apart in the reasons output.

#### Sequences and identity

**21. Identity columns with non-default starts and increments.**

| Column | Identity | Restarted at |
|---|---|---|
| `people.person_id` | `GENERATED ALWAYS` , start 90000, step 7 | 90035 |
| `public."LegacyCustomer"."CustomerID"` | `GENERATED ALWAYS`, start 42 | 45 |
| `orders.order_id` | `GENERATED BY DEFAULT`, start 200000, step 3 | 200015 |
| `tenant_user_sessions.session_id` | `GENERATED BY DEFAULT`, start 5000, step 11 | 5055 |
| `tenant_user_flags.flag_id` | `GENERATED BY DEFAULT`, start 9000, step 2 | 9006 |
| `organisations.organisation_id` | `GENERATED BY DEFAULT`, start 500, step 3 | 506 |
| `teams.team_id` | `GENERATED BY DEFAULT`, start 600, step 3 | 606 |
| `projects.project_id` | `GENERATED BY DEFAULT`, start 700, step 3 | 706 |
| `attachments.attachment_id` | `GENERATED BY DEFAULT`, start 800, step 13 | 852 |
| `billing.invoices.invoice_id` | `GENERATED BY DEFAULT`, start 3000, step 5 | 3015 |
| `stream_rows.stream_row_id` | `GENERATED BY DEFAULT`, start 1 | — |

Two behaviours. Writing a `GENERATED ALWAYS` identity requires
`OVERRIDING SYSTEM VALUE`; a loader without it fails with `428C9` on the very
first table. And the identity's own sequence is state: after a load, the
target's sequences must be advanced past the loaded rows, or the first insert
a developer makes into their local copy collides. The fixture's own inserts use
`OVERRIDING SYSTEM VALUE` and then `ALTER TABLE ... RESTART WITH`, which is the
same pair of moves lazyslice has to make.

#### Scale

**22. Two million rows, on demand** — `public.stream_rows`, filled by
`public.fill_stream_rows(n bigint)`.

Empty by default. It is filled only when the file is loaded with `-v big=1`
(`LoadNasty(ctx, url, true)` sets the same variable), because it costs a few
seconds and a couple of hundred megabytes and every test that is not about
streaming would pay for both.

Every row hangs off the lowest `person_id`, so a slice rooted at Ada Lovelace
pulls all two million and a slice rooted at anyone else pulls none. That is the
fixture for the claim that extract streams rather than buffers: resident memory
must not grow with the row count, the progress line must move, and the run must
not hold two million rows anywhere at once.

The gate is a psql conditional at the very end of the file:

```sql
\if :{?big}
SELECT public.fill_stream_rows(2000000);
ANALYZE public.stream_rows;
\endif
```

`internal/testutil` interprets none of that. It cuts the gate off the end of the
file and, when `big` is set, issues the two statements inside it directly; it
refuses to load `nasty.sql` at all if the gate is missing or no longer fills
`StreamRows` rows, so the psql path and the Go path cannot drift apart in
silence. The only psql construct it does implement is `COPY ... FROM stdin`,
which the wire protocol cannot carry as ordinary SQL and which is how
`pagila-data.sql` is written. Every other backslash command is an error, so a
fixture that grows a `\connect` fails loudly instead of loading half of itself.

`TestLoadNastyBig` runs the `big` path and asserts both the 2,000,000 rows and
the sequence position, because a gate stuck off is indistinguishable from a
working one if only the off state is ever tested.
