# lazyslice

![lazyslice's first run against the Pagila fixture: one command names the source and the target, every column's masking decision prints with its reason, the plan and the load follow, and three customer rows read back from the target with masked names and email addresses: real, common names that are not the customers' own](docs/media/first-run.gif)

Point it at a production Postgres database and get a small, referentially
complete, pseudonymised copy in a local database — one command, no config.

## Status: v0.3.0, pre-release, PostgreSQL only

The pipeline runs end to end against PostgreSQL 14 to 18: it discovers a
source and a target, refuses a target that is not empty or not its own,
subsets from a root table across foreign keys, masks personal data
deterministically, loads, and verifies the copy (foreign keys, row counts, a
residual scan of the target against the source). Hardening is done: the
defects an independent review found on 2026-09-09 and six rounds of an
adversarial red team have landed in the open ([docs/reviews/](docs/reviews/)),
and what remains is tracked as
[issues](https://github.com/Liarea/lazyslice/issues). `v0.3.0` is a
pre-release, like `v0.1.0` (the first version a stranger may install) and
`v0.2.0` before it: the `lazyslice.yml` schema, the flags and the exit codes may
still change between `0.x` minors, with every such change named in the
release notes; a `0.x.y` patch never changes them.

Whatever the version, point it only at data you are already allowed to hold
on the machine that runs it. What a snapshot does not hide is listed below
and in [THREAT_MODEL.md](THREAT_MODEL.md).

- What it will do, and what it refuses to do: [CONCEPT.md](CONCEPT.md)
- How it is built: [ARCHITECTURE.md](ARCHITECTURE.md) and
  [docs/adr/](docs/adr/)
- What it protects against, and what it does not:
  [THREAT_MODEL.md](THREAT_MODEL.md)
- Reporting a masking miss, privately: [SECURITY.md](SECURITY.md)
- Working on it: [CONTRIBUTING.md](CONTRIBUTING.md)
- The ten third-party schemas under `testdata/torture/` and their licences:
  [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)

## Install

From the tap, on macOS:

```sh
brew install Liarea/tap/lazyslice
lazyslice --version
```

Or with `go install`, once Go's own module cache and `$GOPATH/bin` are on
your `PATH`:

```sh
go install github.com/Liarea/lazyslice/cmd/lazyslice@v0.3.0
lazyslice --version
```

`v0.1.0`'s tag predates this fix and still fails: its `go.mod` points the
nested `mask` module at a local `replace` directive, which Go refuses to
resolve for anyone outside this tree (verified 2026-09-22, from an empty
`GOMODCACHE`/`GOPATH`). From `v0.2.0` on, `go.mod` requires
`github.com/Liarea/lazyslice/mask` by its own tagged version instead, and
`go install` works.

Every release also carries macOS, Linux and Windows archives (six in total,
amd64 and arm64), each with an SBOM, and a `checksums.txt` signed keylessly
with cosign:

```sh
cosign verify-blob --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp '^https://github.com/Liarea/lazyslice/' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  checksums.txt
```

then check the archive's own line against the verified `checksums.txt`. The
full steps, and what to do if verification fails, are in
[docs/RUNBOOK.md](docs/RUNBOOK.md) under "Cutting a release".

## The run

This ran against two disposable Postgres containers holding invented data —
no real database, no real person. Nothing below is typed; it is what actually
happened, trimmed of the columns that print but add nothing to read here
(`created_at`, `placed_at`, `qty`, `price_cents`, and the per-column reasons
for `orders`, `order_items` and `products`, which are three tables of
generated order data with no personal columns in them). The full transcript,
schema included, is [docs/QUICKSTART_TRANSCRIPT.md](docs/QUICKSTART_TRANSCRIPT.md).

```sh
$ docker run -d --name shop-db  -e POSTGRES_USER=ls -e POSTGRES_PASSWORD=pw \
    -e POSTGRES_DB=shop     -p 55701:5432 postgres:16
$ docker run -d --name shop-dev -e POSTGRES_USER=ls -e POSTGRES_PASSWORD=pw \
    -e POSTGRES_DB=shop_dev -p 55702:5432 postgres:16
$ psql postgres://ls:pw@127.0.0.1:55701/shop -f schema.sql
# customers, products, orders, order_items — 400 invented customers, none real

$ export LAZYSLICE_SECRET=$(openssl rand -hex 32)   # a throwaway key for this transcript
$ lazyslice --source postgres://ls:pw@127.0.0.1:55701/shop?sslmode=disable \
            --target postgres://ls:pw@127.0.0.1:55702/shop_dev?sslmode=disable \
            --root customers -n 50 --no-config
  source shop on 127.0.0.1 as ls (flag) — --source
! the role ls can write to 4 table(s) in shop — recommend a read-only role: CREATE ROLE lazyslice_ro ...
  target shop_dev on 127.0.0.1 — --target
  4 tables on Postgres 160015: 17 columns, 3 foreign keys
  public.customers.email: name matches email; 200/200 samples parse as addresses
  public.customers.full_name: name matches person_name; 200/200 samples mixed digits and words
  public.customers.id: no name or value signal; surrogate key: preserved verbatim
  public.customers.phone: name matches phone
  root public.customers (named by --root) — --root
  public.customers: 50 rows, child_ok; root
  public.orders: 149 rows, child_ok; child of public.customers via public.orders.customer_id
  public.products: 20 rows, parent_only; parent of public.order_items via public.order_items.product_id
  public.order_items: 298 rows, child_ok; child of public.orders via public.order_items.order_id
  517 rows, 8 KiB of keys, 0 KiB of residual filter; the snapshot is held about 0.0s, assuming 20,000 rows/s
  dropping public.products in the target
  dropping public.orders in the target
  dropping public.order_items in the target
  dropping public.customers in the target
  public.customers: 50 rows
  public.orders: 149 rows
  public.products: 20 rows
  public.order_items: 298 rows
  verify: 3 foreign key(s) validated, 4 table row count(s) matched (517 rows total), the residual scan found nothing (170 value(s) tested), the second net scanned 11 column(s)
  target shop_dev on 127.0.0.1:55702 as ls — password: wherever you supplied it for --target ($PGPASSWORD, ~/.pgpass, or the connection string itself)
$ echo $?
0
```

The two lines marked `--source` and `--target` are lazyslice answering the
first question anyone pointing it at a database has to ask before anything
else happens: which database is about to be read, and which one is about to
be dropped and rewritten. Get either wrong and the run refuses instead of
guessing — see the last example below.

`--root customers` above answers the one question a run at a terminal would
otherwise ask: a run without `--root` (and with no root already recorded in
`lazyslice.yml`) prints `root table? [customers]` and waits for Enter, a
table name, or `?` for the ranked candidates — defaulting to the table with
the most incoming and fewest outgoing foreign keys, which is `customers`
here anyway. With `--yes` or no terminal to ask, it takes that default
without printing the question at all.

Every column lazyslice thought looked like personal data says why:
`customers.email`, `.full_name` and `.phone` are masked by name; `customers.id`
is a surrogate key and is kept, because a key with nothing in it to identify
is not personal data on its own (row identifiers are still preserved — see
"What a snapshot will not hide"). The masked columns hold different values in
the target than in the source:

```sh
$ psql postgres://ls:pw@127.0.0.1:55702/shop_dev \
    -c "select id, email, full_name, phone from customers order by id limit 3"
 id |          email           |      full_name      |    phone
----+--------------------------+---------------------+--------------
  1 | sami.gruber@example.net  | 9164 Juniper Street | +12055550183
  2 | paulo.zhang@example.org  | 7528 Laurel Drive   | +12045550193
  3 | freya.tanaka@example.net | 8314 Fern Place     | +16185550121
(3 rows)

$ psql postgres://ls:pw@127.0.0.1:55701/shop \
    -c "select id, email, full_name, phone from customers where id in (1,2,3) order by id"
 id |              email              |    full_name     |    phone
----+---------------------------------+------------------+-------------
  1 | customer1@invented-example.test | Mateo Alvarez 1  | +1-555-0101
  2 | customer2@invented-example.test | Ingrid Solberg 2 | +1-555-0102
  3 | customer3@invented-example.test | Kwame Boateng 3  | +1-555-0103
(3 rows)
```

The run above exited `0`, which is the green verify: `lazyslice_meta.status`
is written `complete` only once every check in ARCHITECTURE.md section 6 has
passed — foreign keys resolve, row counts and sequences match the plan, and a
residual scan finds no source value left in a masked column. A masked name
can equal some other row's real name, because masked names are real, common
names drawn from the 2020 U.S. Census lists; the scan counts those and checks
that no row kept its own.

```sh
$ psql postgres://ls:pw@127.0.0.1:55702/shop_dev -c "select status from lazyslice_meta"
  status
----------
 complete
```

The marker table keeps one row per run, so on a target that has been loaded
more than once ask for the latest, which is the row lazyslice itself reads:
`select status from lazyslice_meta order by started_at desc limit 1`.

A run against the same target a second time reloads it — truncate and rebuild,
not append — because the row above marks the target as lazyslice's own:

```
  target shop_dev on 127.0.0.1 — --target
  shop_dev carries lazyslice's own marker for this source: it will be truncated and reloaded
```

And naming the same database as both `--source` and `--target` is refused
before a single row is read, not silently pointed at whichever one lazyslice
guesses you meant:

```sh
$ lazyslice --source postgres://ls:pw@127.0.0.1:55701/shop?sslmode=disable \
            --target postgres://ls:pw@127.0.0.1:55701/shop?sslmode=disable \
            --root customers -n 5 --no-config
  source shop on 127.0.0.1 as ls (flag) — --source
! the role ls can write to 4 table(s) in shop — recommend a read-only role: ...
✗ target shop on 127.0.0.1 is the source database
lazyslice: target.refused.same_database: the target ls@127.0.0.1:55701/shop (+1 param) is not eligible
$ echo $?
2
```

```sh
$ docker rm -f shop-db shop-dev
```

### Flags

<!-- docgen:flags:start -->

The flags a first run meets. The full set, one row per registered flag grouped by stage, is [docs/FLAGS.md](docs/FLAGS.md).

| Flag | Type | Default | Description |
|---|---|---|---|
| `--source` | string | - | Names the source; a non-Postgres scheme or unsupported major exits 2 |
| `--target` | string | - | Names the target; never bypasses the gate |
| `--root` | string | - | Root table (default: computed from the foreign-key graph) |
| `--yes` | bool | - | Headless: ask nothing; questions with no safe default become hard failures naming their flag |
| `--create-target` | bool | - | Start postgres:&lt;source major&gt; as lazyslice-target-&lt;project&gt; instead of asking |
| `--unmask` | stringArray | - | Per-column opt-out, as TABLE.COL=REASON; the bare form is exit 2; repeatable |
| `--skip-table` | stringArray | - | Drop a child-only table to schema-only; repeatable |
| `--phone-region` | string | - | ISO 3166-1 alpha-2 region libphonenumber recognises (e.g. GB; anything else is exit 2) a national-format phone column is read under, alongside the guessed regions every run already tries; recorded as phone_region and shown in the reasons output |
| `--secret-file` | string | "./lazyslice.secret" | Masking key file; LAZYSLICE_SECRET overrides it |
| `--require-key` | bool | - | Exit 5 instead of using an ephemeral key |
| `--plan` | bool | - | Stop after printing the plan; touch nothing |
| `--json` | bool | - | NDJSON events on stdout |

<!-- docgen:flags:end -->

## Why

**Zero config.** A first run at a terminal asks only what it could not settle
on its own — which table to start from, and, when it found no database to
load into, whether to start one — and then works; a headless run asks none. Nothing above
needed a YAML file written before it could run: `lazyslice.yml` is what a run
*emits* once it has already worked, a record to commit for next time, never a
prerequisite for the first one. We refuse to ship any feature whose first-run
path is "write a YAML file."

**Safe by default.** Anything that might be personal data is masked unless a
person opts a specific column out and says why; the source above is opened
read-only with its role's privileges checked and printed, not assumed, and the
target has to be empty or a database lazyslice wrote before. The output is
pseudonymised, not anonymised, and the section below says exactly what that
means. We refuse to ship a flag, mode, or default that copies an unclassified
column as-is — there is no flag and no mode that turns masking off, enforced
by `cmd/lazyslice`'s `TestForbiddenFlagsDoNotExist`.

**Terminal first.** One static binary, one-line install, no native
dependencies, no call to anything we operate — the run above never left the
two databases it named. The same command a human types at a prompt works
headless in CI with `--yes`. We refuse to ship a capability that exists only
in the TUI.

## How it compares

Every cell about another tool is that tool's own documentation, fetched
2026-09-22; a cell nothing found could confirm says "not stated" instead of
guessing. lazyslice's own cells describe `v0.3.0` exactly as installed above.

| | lazyslice | Greenmask | PostgreSQL Anonymizer | Tonic Structural |
|---|---|---|---|---|
| Time to first snapshot | One command and no config step — the run above went from the command line to `complete` with nothing written beforehand | A config file is written first; the bundled playground's own quickstart edits its sample `config.yml` before the first `dump`[^gm-quick] | Six DDL/SQL statements before a masked read: create the extension, enable it, load a sample table, initialise masking, create a masked role, declare a rule[^pga-home] | Sign up, verify by email, create a workspace, then a sensitivity scan and a generation run — about seven to eight steps end to end[^tonic-quick] |
| Config required before first run | None — `--no-config` above wrote nothing; a run with no flags at a terminal asks at most two questions (whether to start a target container when none is found, then which table to start from) and a headless one that cannot settle the target stops naming the flag it needs | Yes — "a configuration file is mandatory for Greenmask functioning"[^gm-quick] | Yes — masking rules are declared as `SECURITY LABEL`s on each column, a policy stored in the database, before anything is masked[^pga-rules] | An account and a workspace, always; a bundled sample workspace needs no database connection, but masking your own data does[^tonic-quick] |
| Databases | PostgreSQL 14–18 only | PostgreSQL (full support); MySQL "in progress"[^gm-repo] | PostgreSQL only, plus the Postgres-compatible forks Greenplum and YugabyteDB[^pga-home] | Postgres, Oracle, Db2, MySQL, SQL Server, Redshift, Snowflake, BigQuery, MongoDB, Databricks, Spark, S3, Salesforce and flat files[^tonic-product] |
| Masking determinism (same input, same output across runs) | Deterministic under a local key by construction — the same value always masks the same way for the same key and category (see "How it decides what is personal data" below) | Opt-in, not the default: `engine` "by default is set to `random`"; the hash engine has to be chosen explicitly for the same input to always produce the same output[^gm-engine] | Opt-in, not the default: the built-in masking functions are random; the same input is deterministic only through the separate `pseudo_*`/`hash` functions, seeded by hand[^pga-funcs] | Stated as a feature — "automated, consistent transformations that preserve relationships and referential integrity"[^tonic-product] |
| Licence | Apache-2.0 | Apache-2.0[^gm-repo] | The PostgreSQL License[^pga-license] | Proprietary — no free or open-source tier; "Professional" and "Enterprise" are both custom-priced[^tonic-price] |

[^gm-quick]: [Greenmask — Playground](https://docs.greenmask.io/latest/playground/), fetched 2026-09-22.
[^gm-repo]: [github.com/GreenmaskIO/greenmask](https://github.com/GreenmaskIO/greenmask), fetched 2026-09-22 (Apache-2.0 licence badge; README: "Designed for PostgreSQL and MySQL (in progress)").
[^gm-engine]: [Greenmask — Transformation engines](https://docs.greenmask.io/latest/built_in_transformers/transformation_engines/), fetched 2026-09-22.
[^pga-home]: [PostgreSQL Anonymizer — documentation home](https://postgresql-anonymizer.readthedocs.io/en/stable/), fetched 2026-09-22.
[^pga-rules]: [PostgreSQL Anonymizer — Declare Masking Rules](https://postgresql-anonymizer.readthedocs.io/en/stable/declare_masking_rules/), fetched 2026-09-22.
[^pga-funcs]: [PostgreSQL Anonymizer — Masking Functions](https://postgresql-anonymizer.readthedocs.io/en/stable/masking_functions/), fetched 2026-09-22.
[^pga-license]: [gitlab.com/dalibo/postgresql_anonymizer — LICENSE.md](https://gitlab.com/dalibo/postgresql_anonymizer/-/blob/latest/LICENSE.md), fetched 2026-09-22.
[^tonic-product]: [Tonic Structural — product page](https://www.tonic.ai/products/tonic-structural), fetched 2026-09-22.
[^tonic-price]: [Tonic — pricing](https://www.tonic.ai/pricing), fetched 2026-09-22.
[^tonic-quick]: [Tonic Structural — Getting started with the free trial](https://docs.tonic.ai/app/quick-start-guide), fetched 2026-09-22.

## How it decides what is personal data

Three signals feed every decision, and the run above shows the first two in
its reason lines. A column's **name** is checked against a multilingual rule pack
(`email`, `phone`, `full_name`, and so on — `customers.email: name matches
email`). Its **sampled values** — about two hundred rows, never the whole
table — are run through validators built for the same categories: an email
parser, libphonenumber, a Luhn check for card numbers that also wants a
known issuer prefix (and, under an id/number/version/reference column name,
the issuer's own length), a name dictionary
(`200/200 samples parse as addresses`). And its **neighbours** matter, in two
ways. A column already at low confidence is raised to suspect the moment
another column in the same table is at likely or above, because a
personal-shaped table tends to be personal throughout. And a character column
with no name or value signal at all — nothing to raise, and not a unique or
key column — is still swept into free-text masking when it sits beside a
column the classifier is certain identifies a person, unless its samples say
plainly what it is: an enumeration (at most 20 distinct values in 10 or more
samples, each seen at least twice — a `role`, a `state`, a log level) or all
one identifier shape (a UUID, a hex digest, a version, a hostname, a path).
Such a column is copied, and its reason line says which; a value carrying a
dictionary name, a special-category term, a gender term, a blood group or
marital status, or reading as a date, a postcode or a phone-number-like run of digits is never
spared that way. A signal-less column
that isn't character-typed — an integer, numeric, date or uuid column, the
shape of a surrogate key like `customers.id` above — is copied verbatim
regardless of its neighbours; the sweep only ever reaches columns free-text
masking can apply to. A name hit alone is enough to mask; a value hit alone is
enough to mask. A column with no name signal, no recognised value shape and no
personal neighbour in its table is not "unsure" — it is copied; that gap is
residual 4 below. Nothing here calls out to a network or a model; it runs
entirely against the box being read.

Every decision earns one line explaining itself — the lines the run above
printed before a single row moved — and that same reason is what
`lazyslice.yml` records and what `--json` emits, so a decision that looks
wrong can be read, not just trusted.

### How to override it

A decision that's wrong can be told so, per column:

```sh
lazyslice --unmask public.film.description="product catalogue text, no personal data" ...
```

The bare flag with no reason is refused at exit 2 — "not personal data" is a
claim someone has to own, not a checkbox. The run records that reason in the
`lazyslice.yml` it emits, alongside every other decision it made:

```yaml
columns:
  public.film.description:
    category: free_text
    confidence: possible
    reason: "name matches description; neighbouring-column rule did not apply (no PII in film)"
    unmask:
      reason: "product catalogue text, no personal data"   # never empty; --unmask TABLE.COL=REASON
      by: flag                                             # or the person's name, for a hand-written entry
      type: 3e51a0c2                                        # this opt-out expires if the column's type changes
```

(`masker:` is omitted here — its presence on a column means the run masked
it, and an unmasked column never carries one.)

The other direction needs no reason, because it only ever masks more:
`--mask public.film.description` masks a column the classifier copied, as
`free_text`, or as the category you name with `=CATEGORY`. It is recorded
under the column's `mask:` block with `by: flag`, and it is the answer to a
`verify.refused.second_net` refusal, whose line names a `--mask` flag that
works — the category the check found when that column's type accepts it,
`semi_structured` for a json/jsonb/hstore column whatever category matched
one of its values (no other category accepts the json family), the bare
`--mask TABLE.COL` when neither applies, and `--skip-table` alone for a
column the check found already masked, since `--mask` cannot change a
column's category once one is recorded. A column that cannot be masked that
way (a key, or a type the category does not fit) is exit 2, never quietly
copied, and so is a `--mask` on a column already masked under another
category. Masking a key
masks every column that references it by foreign key as well, under the same
category, so the join still holds; a column with its own `--unmask` is the one
that stays copied. A referencing column whose type cannot take that category
is exit 2, named for a `--mask` of its own.

Commit that file and the next run — including CI's — needs no flag and asks
no question. A column the file has never seen is classified fresh, exactly as
any column is: masked when the classifier lands at `possible` confidence or
above, copied when it lands at `low` or `none`, and printed under `drift:`
either way (ADR-004). `--strict-schema` turns any drift into exit 10 instead.
An opt-out itself only ever narrows what gets copied, never widens it by
omission — it takes a recorded `unmask` with a reason, never silence.

## The safety model

Six sentences cover it. The source is opened read-only and its role's
privileges are checked and printed, never assumed. The target must be empty
or a database lazyslice itself wrote before, held under a lease for the
whole run so nothing else can take it apart at the same time. Masking is
deterministic under a local key that is never written to git, never printed,
and never sent anywhere — the same key produces the same masked value for
the same input every time, which is what keeps a join working after masking
and what a leaked key would let someone confirm a guess against. Every check
runs before the marker row is written `complete`: foreign keys, row counts,
sequences, and a residual scan of the target against the source. A failure in
that last group — a masked column still holding something the source has —
empties the tables this run loaded rather than leaving a suspected leak on
disk. When the classifier cannot decide whether a column is personal data, it
masks it; when a run cannot tell what it is looking at, it refuses rather
than guesses.

## What a snapshot will not hide

An exit `0` means the checks above passed, not that the snapshot is
anonymous. lazyslice pseudonymises; it does not anonymise. The full list of
stated false negatives, with the reasoning behind each, is in
[SECURITY.md](SECURITY.md) and, in more detail, [THREAT_MODEL.md](THREAT_MODEL.md).
After five rounds of an adversarial red team and a sixth that replayed
everything still open with no new variants
([docs/reviews/2026-09-15-redteam/](docs/reviews/2026-09-15-redteam/)), five
residuals are accepted rather than hidden:

1. **Row identifiers are preserved.** Surrogate keys and the foreign keys
   that reference them are copied verbatim, so anyone holding any other
   reference to a production record — an admin URL, a ticket, a log line, a
   payment or support system — can re-identify every row exactly.
2. **A bare national identifier with nothing to corroborate it** — for
   example a nine-digit number with no dashes — in a column whose name
   matches no rule and whose table holds no other column already decided
   personal, masked or not. One exception inside that: a contiguously
   issued block of national identifiers that is itself a table's own primary
   key reads as a surrogate key and is kept, even beside a column that is
   masked. A key of scattered identifiers is scored like any other column,
   and one whose name matches a rule (`ssn`, `tax_id`) is refused at plan.
3. **A name, or any other value, in a script the built-in dictionaries do
   not carry**, in a column also named in that script. The name, address,
   phone and email name-patterns and the name dictionary are Latin-script
   only, covering the given/surname stock of about twenty languages; a
   column named — and holding values — in Cyrillic, CJK, Arabic, Thai,
   Devanagari, Amharic, or any other non-Latin script defeats both the
   name-pattern match on the column and the value match on its contents at
   once. Extending the rule pack to non-Latin scripts is filed, not shipped
   (docs/reviews/2026-09-15-redteam/round5.json). A column called just
   `name` (or `display_name`, or `plan_name`) reaches the same place in any
   script: unless its table or its own name has a word for people in it
   (`users`, `customer_name`), it is copied when no column beside it is
   decided personal at `likely` or above (a neighbour masked on its name
   alone, such as a `phone` column, does not count), three or more of its
   values are sampled, and fewer than a fifth of them carry a name the
   dictionary holds — so a list of names the dictionary cannot carry, in
   such a column, is copied. A name whose own qualifier says it is a
   person's (`legal_name`, `billing_name`, `name_on_card`) is masked on
   the name alone, as before.
4. **A shape none of lazyslice's validators know, in a column no name rule
   names.** The validators parse or recognise about a dozen shapes —
   email, phone, national ID, IBAN, card number, IP or MAC address, a
   credential's entropy, a name, an address, ordinary prose, a
   special-category term. A card number is only recognised inside a known
   issuer's range and, under an id/number/version/reference column name,
   at that issuer's own length, so one outside the table or of an unlisted
   length under such a name is copied too. Under a version, build or
   release column name, IP or MAC addresses among the column's values are
   copied too when they are only a minority — that name no longer offers
   the address validators as a minority signal, though a genuine majority
   of addresses there still masks the column as before. Under a key,
   code, license, serial or token column name, a guessed-region phone
   number is copied too, when the table's best personal neighbour is only
   likely, not certain, personal. Anything else, in a column with no name signal and
   no personal neighbour in its table, is copied — and so is such a column
   beside a personal neighbour when its samples read as an enumeration or an
   identifier shape (a username repeated across a handful of staff rows, a
   hostname a device's owner chose), which the neighbour rule spares. The
   entropy check itself now passes four more shapes through, in a column no
   credential name rule matches: a secret that is a hex run of exactly 32,
   40 or 64 characters, a secret column of one to four non-NULL rows, a
   secret in a column named `type`, `klass` or `component_name`, and a file
   named after a person in a column where fewer than a fifth of the file
   names carry a word the dictionary holds.
5. **The marker-bound reload window.** The first load into a fresh target
   checks the whole target, under the run's lease, for a table that appeared
   after the plan was approved. A *reload* — the ordinary daily case, since
   any target lazyslice has written once stays marked as its own — does not
   repeat that whole-target check: a table an application creates in the
   target strictly after one run finishes and strictly before the next run's
   first drop is left alone and unmentioned.

## Exit codes at a glance

Every exit code above 1 names a stage and a reason; `1` is the one code that
by construction names no reason (an internal failure; run with `--debug`).
The full table, one row per code with its message template, is
[docs/ERRORS.md](docs/ERRORS.md). In one line:

`0` ok · `1` internal failure with no code of its own — run with `--debug` ·
`2` usage — a flag names something that cannot work (this is also the code a
same-database target is refused with: that check runs at the flag surface,
before discovery, not as one of the `4`s below) · `3` no source · `4` target
refused · `5` no usable source credential or masking key · `6` the source
role can write and `--require-read-only-role` was set · `7` extract or load
failed · `8` a foreign key does not hold · `9` a masked column still holds a
source value, or a value that could not be confirmed either way · `10` a
column the committed config has never seen, under `--strict-schema` · `11` a
row or memory budget was exceeded · `12` the plan was refused — a column
cannot be masked in place, or an identifier is missing · `13` the target's
schema cannot be recreated safely — an unrewritable literal, or an object
lazyslice does not recreate · `130` interrupted — the in-flight transaction
rolled back; tables already committed stay as they are, so the target may be
partly loaded and the next run will truncate it.

## Building

```sh
git clone https://github.com/Liarea/lazyslice
cd lazyslice
make build        # bin/lazyslice
export PATH="$PWD/bin:$PATH"   # the examples above call it as lazyslice
make check        # lint and unit tests; this is what CI runs
make integration  # container-backed tests; needs a Docker endpoint
make egress       # runs the binary with only its two databases reachable and counts every other packet
```

The masker is a nested Go module, `github.com/Liarea/lazyslice/mask`, so it can
be imported by a program that has never heard of lazyslice (ADR-006).

## Roadmap

What's next, and what's deliberately not yet: [ROADMAP.md](ROADMAP.md).

## Licence

Apache License 2.0. See [LICENSE](LICENSE). The name lazyslice is a trademark of the project; the licence does not grant permission to use it for a derived product (Apache-2.0 §6).
