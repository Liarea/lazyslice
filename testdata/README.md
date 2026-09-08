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
traps themselves out of the catalogue — the two enums, the generated column,
the identity columns and their sequence positions, the partitioning, the two
composite foreign keys and their match types, the unique-index and no-identity
tables, the key column types, the array's four edges, the unique indexes and
`CHECK` on the masked columns, and that **every** table has a foreign-key path
to `public.people`. Strip `GENERATED ALWAYS` off a column, un-partition a table
or disconnect a component and that test fails, rather than a phase 4 test
failing later and saying something else. Change a fixture and change that test
with it, deliberately, in the same commit as this file.

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

25 tables, 5 people, and one trap per thing that goes wrong. It needs no
extension and no superuser, and it loads unchanged on PostgreSQL 14, 16 and 18.

**Every table has a foreign-key path to `public.people`, and that is a
requirement rather than an accident.** `public.people` is the root every
invariant run slices from, and a table with no path to it is unreachable, so
ARCHITECTURE.md §3 emits `Step{t, SchemaOnly}` for it: no chunked read, no
`COPY`, no masking, no residual scan, zero rows in the target. Its traps then
cost nothing to pass. Eight tables were in that state until the edges named in
traps 2, 4 and 6 and the quoted one in trap 9 were added, and
`assertEveryTableReachesPeople` in `internal/testutil/fixtures_test.go` is what
stops it happening again. A table added here connects itself or is not a trap.

**Every run over this fixture carries `--skip-table public.click_stream`.**
Trap 12's table has no row identity at all, which is its point, and §3's
identity ladder ends in exit 12 for it. It is a child of `public.people`
reached at depth 1, so the flag changes the slice rather than merely tidying
it, and it is child-only, which is the condition §3.6 puts on the flag. Read
trap 12 before assuming the refusal happens after `--skip-table` is applied; as
§3's pseudo-code is written today it happens before, and nothing else in the
file can run without the flag.

| Table | Rows | | Table | Rows |
|---|---:|---|---|---:|
| `billing.invoices` | 3 | | `public.people` | 5 |
| `public."LegacyCustomer"` | 3 | | `public.price_list_notes` | 2 |
| `public.attachments` | 5 | | `public.price_lists` | 3 |
| `public.audit_log` | 4 | | `public.price_lists_eu` | 2 |
| `public.click_stream` | 3 | | `public.price_lists_us` | 1 |
| `public.device_readings` | 4 | | `public.projects` | 2 |
| `public.devices` | 3 | | `public.sites` | 2 |
| `public.events` | 7 | | `public.stream_rows` | 0 (2,000,000 with `big`) |
| `public.events_2024` | 5 | | `public.teams` | 2 |
| `public.events_2025` | 2 | | `public.tenant_user_flags` | 3 |
| `public.order_items` | 7 | | `public.tenant_user_sessions` | 5 |
| `public.orders` | 5 | | `public.tenant_users` | 4 |
| `public.organisations` | 2 | | | |

### The traps

#### Structure

**1. Self-referencing foreign key** — `people.manager_id -> people.person_id`.

A walk with no visited set never terminates here. lazyslice must expand a
manager chain once per row, stop, and report `people` once in the plan rather
than once per level. `--root public.people --where person_id=90021 --take 1`
(Katherine Johnson) pulls Grace Hopper (90007) and then Ada Lovelace (90000) as
parents, and stops. `--root` names a table; the row is chosen by `--where`.

**2. Three-table foreign-key cycle** — `organisations -> teams -> projects ->
organisations`.

There is no valid table order for a loader that sorts by dependency, which is
exactly why ARCHITECTURE.md §11.1 loads data first and adds foreign keys after.
The planner must terminate; the loader must succeed; and the target must end
with all three constraints present and valid. `organisations.primary_team_id` is
`NOT NULL`, so "just leave the cycle-closing column null" is not an escape.

`organisations.founded_by` is `NOT NULL REFERENCES public.people`, and it is
what makes any of the above happen at all: the cycle is pulled as a child of
the root, so the loader has rows to fail on. Without that edge the three tables
are `SchemaOnly`, the load is of zero rows, and "the loader must succeed" is
true of a loader that does nothing. The cycle itself is untouched by it.

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

`tenant_users.owner_person_id` is `NOT NULL REFERENCES public.people` and is
what puts this component in the slice at all; before it, traps 4, 5 and 18 were
exercised by nothing but I6's row count on a `SchemaOnly` table. It is also the
one outgoing edge `tenant_users` has, which is why I6 can still root at it: its
only incoming edges are from its own two children, so nothing reaches it as a
parent and it holds exactly `--take` rows.

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
`attachments.owner_type` (`'people'`, `'projects'`, or `'Person'`, the
Rails-spelled form) and `attachments.owner_id`.

PostgreSQL knows nothing about this pair, so lazyslice infers it instead of
following a declared constraint (§3.2, amended 2026-09-08, T-POLY): the
distinct `owner_type` values are sampled and each is mapped to a table —
`'Person'` through the Rails form (underscore and pluralise: `Person` →
`people`), `'people'` and `'projects'` through the raw-table-name fallback,
since a value no Rails form resolves and that names a table directly is
mapped to it rather than reported unmapped — and each mapping becomes one
virtual, parent-direction foreign key that the plan follows. Every followed
edge is printed under `plan.polymorphic.inferred`, one line per discriminator
column and parent table pair, before extraction starts:

```
owner_type: inferred public.attachments (owner_id) -> public.people, followed as a virtual parent edge
owner_type: inferred public.attachments (owner_id) -> public.projects, followed as a virtual parent edge
```

and the rows the edge reaches are in the slice, pulled in as parents alongside
the ones `attachments.uploaded_by_person_id` — a real declared edge into
`public.people` — already reaches. `attachments.uploaded_by_person_id` is what
makes the trap stronger even though every value resolves: four of the five
rows (800, 813, 826 and 845; 839's `uploaded_by_person_id` is `NULL`) are
already in the slice through the constraint PostgreSQL does know about, so
the virtual edge is shown reaching a row (`public.projects` 700, through
attachment 826) that no declared edge touches, not merely repeating what the
declared edge already selected.

A value no form resolves is never guessed; it is reported once as unmapped
(`plan.polymorphic.unmapped`, §3.2), which is the finding that survives from
before inference landed. research/COMPLAINTS.md FK-10 is the silently empty
slice both this trap and that finding exist to make impossible. Attachment
839 points at `people` 99999, which does not exist, and its
`uploaded_by_person_id` is `NULL`, so the dangling polymorphic owner sits on
the one row the declared edge does not reach: the virtual edge is followed in
the parent direction only, so it never pulls a dangling id into the slice as
a child, and the row stays out.

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
`"CustomerID"`, `"MigratedFromPersonID"`, `"EmailAddress"`, `"ContactNumber"`,
`"MobileNumber"`, `"Notes"`.

These objects exist only when quoted. Every statement lazyslice generates — the
count probe, the chunked read, the residual scan, the `COPY` target, the
emitted `lazyslice.yml` — must quote them. An identifier concatenated into SQL
without quoting fails here with `42P01` rather than quietly reading something
else, which is the point: this trap turns a class of silent bug into a loud one.

`"MigratedFromPersonID" integer REFERENCES public.people (person_id)` is what
makes the claim above testable. No table in the `people` component has a quoted
column of any kind, so before this edge existed the table was `SchemaOnly` and
not one of those five statements was ever generated for it. The column is
`integer` against a `bigint` primary key, which PostgreSQL allows, so a
widening key comparison is in the fixture too — and the `COPY` column list of a
loaded `"LegacyCustomer"` now contains quoted identifiers, which is where an
unquoted concatenation fails.

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

**Every other run over `nasty.sql` carries `--skip-table public.click_stream`,
and it has to.** `click_stream.person_id` references `public.people`, so it is
a **child of the root, reached at depth 1** by every run this fixture is
sliced by — the flag is what keeps its rows out of the slice, not a formality
for a table nobody selects. It is also **child-only**: nothing references
`click_stream`, so no selected row needs it as a parent, and that is the
condition §3.6 puts on the flag ("`--skip-table TABLE` … drops a child-only
table to `SchemaOnly` on request … it cannot skip a parent table"). Both halves
matter: the first says the flag changes the slice, the second says the flag is
allowed at all.

**This is where the fixture and ARCHITECTURE.md §3 do not agree, and the
disagreement is §3's.** §3's pseudo-code computes `identity[t]` over
`tables := sort(schema.Tables ...)` — every table in the catalogue — and
refuses on the first `nil`, *before* `unreadable(req, priv, root)`, which is
where §3.6 applies `--skip-table`. Read literally, `click_stream`'s identity is
computed and refused whatever flags are passed, and every run over this fixture
exits 12 at plan; `internal/invariants/harness_test.go` depends on the other
reading and all six invariants are unrunnable on this fixture without it.
**Decided in ARCHITECTURE.md §3 (tracker T-0032, landed 2026-09-06):** the identity ladder runs after `req.Skipped` is applied, and `--skip-table` clears an exit-12 identity refusal for a child-only table. Reachability is no part of it: `click_stream` is a depth-1 child of the root, and only the flag removes the refusal.

**The documented bypass, which must not be one.** §3.4 puts `--key` on the
first rung and says an explicit key always wins over a probed guess, so
`--key public.click_stream=person_id,url,clicked_at` is accepted today — and
the first two rows of this table are identical in every column, so that key
identifies two rows at once and produces exactly the silently-wrong-rows slice
this trap exists to prevent. An explicit key that is not unique has to be
probed and refused like any other candidate, with the same exit 12. Trap 12 has
a hole until it is.

#### Types

**13. Enum type, not flagged** — `public.account_status`, used by
`people.status`.

The type must exist in the target before the table that uses it, and the
classifier must not sample it as free text. **An enum is classified like any
other column**: ARCHITECTURE.md §4 is explicit that "there is no exemption by
type: an enum column, a partition-key column and a `varchar(2)` column are
classified like any other, and a masked enum emits a valid label (§5). The
earlier draft's enum exemption was a copy-as-is default by type and is
removed." An earlier version of this entry said the opposite — "leave it alone
unless the category says otherwise" — which was the removed exemption in
disguise.

What this column proves is therefore narrow, and that is the point of splitting
it from trap 24. `status` takes no name hit and no validator hit, so it
classifies at `none` and is copied. It proves the type reaches the target
before the table, and that nothing treats an enum as free text. It proves
nothing at all about masking an enum, because it is never masked. The other
half is trap 24, `public.marital_status` on `people.marital_status`, whose name
a special-category rule does hit.

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
same length and dimensions, with a `NULL` element preserved as `NULL` and an
empty array left empty. The five rows cover every edge exactly once:

| Row | Value | What it is for |
|---|---|---|
| Ada 90000 | two addresses | the ordinary case, and "same length" |
| Grace 90007 | one address | a one-element array |
| Alan 90014 | `ARRAY[NULL, 'a.turing@corp.invalid']` | a `NULL` **element**, which must stay `NULL` |
| Katherine 90021 | `NULL` | a `NULL` **column**, which is a different thing |
| Edsger 90028 | `'{}'` | an empty array, which must stay empty |

**Decided in ARCHITECTURE.md §4 and §5 (tracker T-0034, landed 2026-09-06):** arrays are classified on their element type and masked element-wise; length, dimensions and lower bounds are preserved; a `NULL` element stays `NULL`, an empty array stays empty, and a `NULL` column stays `NULL`. Every row of this table is covered by that rule.

**16a. JSONB with personal data nested two levels deep, masked leaf by leaf**
— `people.contact`.

```json
{"profile": {"contact": {"email": "ada.lovelace@fixture.test",
                         "phone": "+44 20 7946 0958"}, "locale": "en-GB"},
 "tags": ["founder"]}
```

The email and the phone are at `profile.contact.email` and
`profile.contact.phone`. At Gate 4 the rule pack collects **no** JSON keys at
all — §14 puts one-level key collection in phase 5 — so the required v1
behaviour is not "find them by key": it is that the column is classified as
carrying personal data on the strength of its sampled values, that every scalar
leaf is replaced (§4: string leaves through the category masker chosen by
running the leaf's key name through the name rules, defaulting to `free_text`;
numbers and booleans re-derived from `h`; `null` stays `null`; structure and key
names kept), and that the residual scan over the target finds no address from
the source in it. Each masked leaf is a separate Bloom-filter entry keyed by its
JSON path (§6 item 1), which is what makes one surviving leaf findable inside a
document that otherwise changed. Silently shipping the document unchanged is
the failure this trap catches.

**16b. JSONB in an event table, collapsed to `{}`** — `events.payload`.

Different required behaviour, from the same §4 paragraph: "Wildly varying keys,
or any `jsonb` in a table named like `audit|log|history|event`, replace the
document with `{}`." `public.events` matches `event`, so `payload` is **not**
walked leaf by leaf: the document is replaced with `{}`, no per-leaf masker
runs, and no per-leaf filter entries exist for it. A run that masks these
payloads leaf-wise instead has ignored the rule; a run that ships them
unchanged has ignored the section. Both are bugs, and they are different bugs,
which is why this is its own entry — an earlier version of trap 16 named both
columns and stated only the leaf behaviour, which would have had a phase 4
implementer assert leaf masking on the column the design collapses.

The `actor.contact.email` values in these payloads are the same addresses as
`people.contact`'s, so the I2 grep covers both columns whichever path is taken.

**17. Free text with full names in it** — `people.notes` and
`public."LegacyCustomer"."Notes"`.

`Ada Lovelace asked that Grace Hopper be copied on the renewal. Call back on
+44 20 7946 0958.` Names in prose are the case the English name dictionary and
the entropy check exist for. The column must be classified as free text, masked
as free text, and the residual scan must find no source name or phone in the
target — including names that belong to a *different* row than the one the note
is on, which is why the notes cross-reference each other.

**18. Type signals, with and without a name** — `tenant_user_sessions.origin`
(`inet`), `tenant_user_sessions.adapter` (`macaddr`), `audit_log.client_ip`
(`inet`).

Three columns, because §4 lists three separate things and an earlier version of
this entry conflated two of them. It named `tenant_user_sessions.client_ip` as
"a type signal on its own: the name says nothing", which was wrong: `client_ip`
is as strong a name hit as any column in the file, and under §4 it reaches
`certain` on the name plus `net.ParseIP` agreeing. The branch the entry claimed
to isolate had no fixture at all.

| Column | Type | Signal it proves |
|---|---|---|
| `tenant_user_sessions.origin` | `inet` | **Type alone.** No name rule in any language recognises `origin`, so only the type plus `net.ParseIP` over the samples can classify it — §4's `likely`, "values look like X" |
| `audit_log.client_ip` | `inet` | **Name and type agreeing**, which §4 scores `certain`. A run that classifies this and `origin` alike has collapsed two signals into one |
| `tenant_user_sessions.adapter` | `macaddr` | `macaddr` is named in §4's v1 type-signal list and had no column anywhere in `testdata/`. The name says nothing, so again the type is the only signal |

`2001:db8::1` is in `origin` and `2001:db8::7` in `client_ip`, so a masker that
only understands dotted quads fails visibly on either.

The IPv4 values in `origin` and `client_ip` are RFC 1918 private addresses
(`10.x`, `172.16.x`, `192.168.x`), chosen deliberately to stay outside the
three RFC 5737 documentation blocks `network_id` emits into (ARCHITECTURE.md
§5). A documentation-range IPv4 source value here would make the residual
scan report a false hit — see the invariant in
`internal/invariants/CLAUDE.md` and the CI check that pins it.

**`citext` is deliberately absent**, and this line is here so that its absence
is a decision rather than an oversight. §4 names it as a v1 type signal, but it
needs `CREATE EXTENSION citext`, and `nasty.sql` takes no extensions so that it
loads on 14, 16 and 18 with no superuser. The nearest cover is pagila's two
domains (`public.year`, `public."bıgınt"`), which exercise the "resolve the
underlying type of a domain" half of the same rule. A `citext` fixture needs a
third file with an extension in it, and that is a phase 5 question.

#### Names that lie

**19. False positive: `people.email_verified boolean`.**

The name matches every email rule anyone would write. The type says it cannot
be an address, and the values are `true`/`false`. Masking it produces either a
load failure (`22P02`, an `email` masker's output written to a `boolean`) or a
silently inverted flag, and either way the reason line would have said "email"
about a column that never held one.

**ARCHITECTURE.md as written masks it, and that is the bug this entry now
records.** §4 says "Name hit alone → `possible`", then "after the
neighbouring-column rule and FK propagation have run, `possible` and above is
masked", then "there is no exemption by type", then "the failure mode is mask
more, never less". Under those four sentences `email_verified` takes a name
hit, lands at `possible`, and is masked with the `email` category masker. The
neighbouring-column rule makes it worse rather than better: `people` carries
`ref` at `likely` and `notes` as free text, so every `low` column in the table
is raised to `possible` anyway. The word "boolean" appears in ARCHITECTURE.md
only inside the JSON-leaf rule.

**Decided in ARCHITECTURE.md §4 (tracker T-0033, landed 2026-09-06):** every category declares the types its maskers accept; a name hit on a type the category does not accept is recorded at `low` with a reason naming the conflict, and the neighbouring-column rule never raises it. It gates the name signal and, per ADR-010, silences a value signal on a recognised family the category does not accept; enum, xml and other families keep full recall, so trap 20 still classifies on values. `people.email_verified` therefore lands at `low` and is copied.

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

The source addresses across `nasty.sql` (`people.ref`'s and every other email
column's, including these two) use `fixture.test` and `corp.invalid`, chosen
deliberately to stay outside `example.com`/`example.net`/`example.org`, which
is the email masker's output space (ARCHITECTURE.md §5). A source address
already inside that output space here would make I2's third half and the
residual scan report a false hit — see the invariant in
`internal/invariants/CLAUDE.md` and the CI check that pins it.

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
| `price_list_notes.note_id` | `GENERATED BY DEFAULT`, start 1 | — |

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

Every row hangs off the lowest `person_id`, so a slice that selects Ada
Lovelace can reach all two million and a slice that selects anyone else reaches
none. That is the fixture for the claim that extract streams rather than
buffers: resident memory must not grow with the row count, the progress line
must move, and the run must not hold two million rows anywhere at once.

**The flags are part of the trap, and they were missing from this entry.**
Under the defaults §3 states — `--take 500`, per-parent-key cap `100`,
`--depth 3`, `--row-budget 1000000`, `--memory-budget 256MiB` — `stream_rows`
is a child of `people` and its step is capped at 100 rows per parent key, so a
default run pulls 100 rows, not 2,000,000. Even with the cap raised, the walk
hits `checkBudgets` and exits 11 naming `public.stream_rows` long before two
million. The streaming run is therefore:

```
lazyslice --root public.people --where person_id=90000 --take 1 \
          --cap 2000000 --row-budget 3000000 --memory-budget <what the key set needs>
```

and the default `--cap 100` and `--row-budget 1000000` are exactly what make
every other test over this fixture cheap. A budget refusal here is the correct
behaviour for the defaults and a bug for the run above; the entry has to name
which one it is talking about, and this one is talking about the run above.

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

#### Masking domains

**23. A unique index, a `varchar(n)` and a `CHECK`, all on masked columns** —
`public."LegacyCustomer"."EmailAddress"` and `"ContactNumber"`.

The whole of ARCHITECTURE.md §5's domain machinery had no fixture anywhere in
`testdata/` before these. No masked column carried a unique index; no column in
the file was `varchar(n)`; no column had a `CHECK`. The three unique indexes
that existed were all on `audit_log.entry_uid`, which is never masked and which
is there for trap 11's identity ladder. Pagila covers `varchar(n)` incidentally
and has no unique index on a personal column either. So `d_required = n²/2ε`,
the switch to a larger generator, the exit-12 refusal and "preserve what the
application checks" were four untested claims.

| Object | §5 sentence it proves |
|---|---|
| `CREATE UNIQUE INDEX "LegacyCustomer_EmailAddress_key" ON ... ("EmailAddress")` | "The plan picks, within the column's category, the registered generator with the largest `Domain()` that fits the column; the explanation says uniqueness chose it (`email` → hash-derived suffix, `alice.k7v2x@example.com`, domain ≥ 2⁶⁴)" |
| `"ContactNumber" character varying(15)` with `CREATE UNIQUE INDEX "LegacyCustomer_ContactNumber_key"` | §5's own worked example, word for word: "A unique `varchar(15)` phone column". `phone` has `Domain()` ≈ 8 × 10⁴ and would collide at load with a `PgError` whose `Detail` we drop, so the plan must choose `phone_unique` — saying that libphonenumber validity is not preserved — or refuse the column by name at plan with exit 12 printing `d`, `d_required` and the three escapes. Choosing `phone` and hoping is the bug |
| `CONSTRAINT "LegacyCustomer_EmailAddress_check" CHECK ("EmailAddress" LIKE '%@%.%')` | "Preserve what the application checks: `varchar(n)` length, `CHECK` shapes we can parse". A masked address in `example.com` satisfies it; a filler string does not, and the load fails with `23514` |

`"ContactNumber"` is `NULL` on customer 44, because a unique index permits
repeated `NULL`s and §5's "`NULL` stays `NULL`" has to hold under a
uniqueness-driven generator too.

Which of the two outcomes `"ContactNumber"` should produce depends on `n`, the
planned row count of the table, and `n` here is at most 3. The entry does not
prescribe one: it prescribes that the plan says which, prints the numbers, and
never silently picks a generator whose `Domain()` is below `d_required`.

**24. An enum the classifier flags** — `public.marital_status`, used by
`people.marital_status`.

Trap 13's `account_status` is copied, so it proves nothing about masking an
enum. This one is masked, and it is the only fixture for three separate §5
sentences:

- **"A masked enum emits a valid label."** The column's type has six labels and
  nothing else is insertable; a masker that emitted an arbitrary string fails
  the load with `22P02`. Before this column existed, a masker that did exactly
  that would have failed no test in this repository.
- **`small_domain:`.** Six admissible values is far below `2 × distinct(samples)`
  for any real column, so §5 requires the column to be listed under
  `small_domain:` in the yml and in the plan, and under "what the green tick
  does not prove".
- **The `special_category` collapse.** `marital_status` is a special category by
  name, which §4 scores `certain` by name alone. §5 says substitution over a
  small domain offers no protection for a special category, so the masker
  collapses the column to one fixed label — the first enum label, or `NULL`
  when nullable — and the explanation says `collapsed: substitution over 6
  values is not a mask`. The column is `NOT NULL`, so the collapse must be to a
  label.

The five rows use five different labels, so a collapse is visible as five
identical values and a substitution is visible as five different ones.

#### Target recreation

**25. A partitioned root's own key is `DEFERRABLE`, a leaf carries a key the
root cannot hold, and an edge references the leaf** —
`public.price_list_notes.list_id` into `public.price_lists_eu (list_id)`.

ARCHITECTURE.md §11.1 item 6 re-points a partition-referencing edge onto the
root when the root carries a key over the edge's columns; introspect's
`hasKeyOver` is what decides that. `public.price_lists` is partitioned by
`region`, and its own key, `UNIQUE (list_id, region) DEFERRABLE`, can never
back a foreign key at all — Postgres refuses a `DEFERRABLE` unique constraint
as a referenced key outright ("cannot use a deferrable unique constraint for
referenced table"), which is why `pipeline.Index` carries `Immediate`.
`public.price_lists_eu` additionally carries its own
`UNIQUE (list_id)`, over columns the root can never hold a key on at all: a
partitioned table's own unique constraint must include every partition key
column, and `region` is that column here, so no `(list_id)`-only key can ever
exist on `public.price_lists`, deferrable or not.
`public.price_list_notes.list_id` references `public.price_lists_eu (list_id)`
directly — legal, because a leaf partition is an ordinary table with its own
key — and that is the edge `hasKeyOver` must find no root key for.

That foreign key is added by a separate `ALTER TABLE` gated behind
`\if :{?notrecreatable}` at the end of the DDL, the same device nasty.sql uses
to gate the 2,000,000-row `stream_rows` fill. `checkRecreatable`
(`internal/plan/plan.go`) scans every edge in the schema before a root is even
chosen and refuses any `Plan` call over one carrying a `NotRecreatable` edge,
unconditionally — so a fixture that always declared it would refuse to plan
for every caller of `internal/testutil.LoadNasty`, not only the two tests this
trap is for. `LoadNasty` always cuts the gated block out;
`internal/testutil.LoadNastyNotRecreatable` puts it back, and only
`internal/introspect`'s `TestIntrospectNasty` and `internal/plan`'s
`TestPlanNastyNotRecreatable` use it.

`public.price_list_notes` also carries `person_id bigint NOT NULL REFERENCES
public.people (person_id)`, unconditionally. `internal/plan/plan.go`'s
`byRef` never holds a partition (a table with `Parent != nil` is skipped when
it is built), so an edge that lands on a partition — trap 25's, once it is
present — gives the planner no path to the table at all: reachability
computed the way the planner walks it is not the same question as
`assertEveryTableReachesPeople`'s raw `pg_constraint` walk, which follows the
inherited copy of the trap edge onto the leaf and calls that connected. The
`person_id` column is what actually connects `price_list_notes` to
`public.people` in the planner's own graph, independently of whether the trap
edge is loaded at all.

The required behaviour is two-sided, and each half has its own layer:

- **Introspect** must leave the edge pointed at the leaf rather than silently
  re-pointing it at a root that cannot carry it, and must set
  `ForeignKey.NotRecreatable`. This is the one case where an edge in
  `Schema.FKs` is allowed to name a partition at all.
- **The planner** must refuse at plan — exit 13, `target.schema.not_recreatable`,
  naming `public.price_list_notes`, `list_id` and `public.price_lists_eu` —
  before the snapshot is used for a single key and before anything in the
  target is dropped. `TestNotRecreatableForeignKeyIsRefusedBeforeAnyRead`
  (`internal/plan/plan_test.go`) is this seam over a hand-built schema, kept
  because it is the one case that cannot be a fixture on its own — proving
  no statement reaches the source needs a reader that fails the test if one
  is sent, which only a hand-built schema can hand the planner from nothing.
  This trap is what makes the same refusal CI-verified over a real catalog
  instead.

**This is a schema-wide refusal, not a per-table one.** `checkRecreatable`
runs before a root is even chosen and scans every edge in the schema, so a
schema carrying the constraint refuses to plan regardless of `--root`,
`--skip-table` or which tables the run would otherwise touch — §11.1
recreates the whole target schema's constraints before any row moves, so one
edge it cannot carry blocks every run, not only one that would have reached
`price_list_notes`. That is why the constraint is gated in the fixture itself
(above) rather than left for every caller to work around: `LoadNasty` cuts it
out, so `psql -f testdata/nasty.sql` and every Go caller of `LoadNasty` —
`internal/extract`, `internal/load`, `internal/invariants` and most of
`internal/plan`'s own suite — get a fixture that plans. Only
`internal/introspect`'s `TestIntrospectNasty` and `internal/plan`'s
`TestPlanNastyNotRecreatable` load the constraint too, through
`internal/testutil.LoadNastyNotRecreatable`; that is the one test that plans
the fixture with the constraint present, and it plans through
`refusingReader` precisely because §11.1 says no statement may reach the
source first.
