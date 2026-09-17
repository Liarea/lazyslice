# lazyslice

Snapshot a production SQL database into a safe local copy: subset by a root
table, follow foreign keys, mask personal data, load.

lazyslice reads a production Postgres database, follows one root table's
foreign keys outward to build a small, referentially complete subset, masks
every column that looks like personal data with a deterministic key so joins
still work, and loads the result into an empty local database you name. It
refuses to write anywhere but an empty database or one it created itself;
there is no flag and no mode that turns masking off
(`cmd/lazyslice`'s `TestForbiddenFlagsDoNotExist`); and when its classifier is
unsure whether a column is personal data, it masks it rather than guess it is
safe. A column with no name signal, no recognised value shape and no personal
neighbour in its table is not "unsure" — it is copied, and that is residual 4
below. What it produces is pseudonymised, not anonymised —
some things about the original rows survive on purpose, and the honest list
of what survives is below.

## Status: pre-release, PostgreSQL only

The pipeline runs end to end against PostgreSQL 14 to 18: it discovers a
source and a target, refuses a target that is not empty or not its own,
subsets from a root table across foreign keys, masks personal data
deterministically, loads, and verifies the copy (foreign keys, row counts, a
residual scan of the target against the source). It is in hardening: an
independent review on 2026-09-09 found leak-class defects that are being fixed
in the open ([docs/reviews/](docs/reviews/), tracked in [tracker/](tracker/)),
and there is no supported version until `v0.1.0` is tagged.

Until then, point it only at data you are already allowed to hold on the
machine that runs it. What a snapshot does not hide is listed below and in
[THREAT_MODEL.md](THREAT_MODEL.md).

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

From source, today — this is the only way to get it before `v0.1.0`:

```sh
git clone https://github.com/Liarea/lazyslice
cd lazyslice
make build                     # bin/lazyslice
export PATH="$PWD/bin:$PATH"   # the examples below call it as lazyslice
```

From the tap, once `v0.1.0` is tagged:

```sh
brew install Liarea/tap/lazyslice
```

There is nothing to install from the tap yet: no tag means no release, and
`brew install` against an empty tap fails rather than installing something
untagged.

## Quickstart

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
$ echo $?
0
```

The two lines marked `--source` and `--target` are lazyslice answering the
first question anyone pointing it at a database has to ask before anything
else happens: which database is about to be read, and which one is about to
be dropped and rewritten. Get either wrong and the run refuses instead of
guessing — see the last example below.

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
residual scan finds no source value left in a masked column.

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

## Flags

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
   (docs/reviews/2026-09-15-redteam/round5.json).
4. **A shape none of lazyslice's validators know, in a column no name rule
   names.** The validators parse or recognise about a dozen shapes —
   email, phone, national ID, IBAN, card number, IP or MAC address, a
   credential's entropy, a name, an address, ordinary prose, a
   special-category term. Anything else, in a column with no name signal and
   no personal neighbour in its table, is copied.
5. **The marker-bound reload window.** The first load into a fresh target
   checks the whole target, under the run's lease, for a table that appeared
   after the plan was approved. A *reload* — the ordinary daily case, since
   any target lazyslice has written once stays marked as its own — does not
   repeat that whole-target check: a table an application creates in the
   target strictly after one run finishes and strictly before the next run's
   first drop is left alone and unmentioned.

## Building

```sh
make check        # lint and unit tests; this is what CI runs
make build        # bin/lazyslice
make integration  # container-backed tests; needs a Docker endpoint
```

The masker is a nested Go module, `github.com/Liarea/lazyslice/mask`, so it can
be imported by a program that has never heard of lazyslice (ADR-006).

## Licence

Apache License 2.0. See [LICENSE](LICENSE). The name lazyslice is a trademark of the project; the licence does not grant permission to use it for a derived product (Apache-2.0 §6).
