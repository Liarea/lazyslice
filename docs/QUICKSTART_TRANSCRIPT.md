<!-- This is a transcript, not a guide: every command below was actually run,
     against two disposable Postgres containers holding invented data, and
     removed afterwards. README.md's "The run" section is a trimmed excerpt
     of it. Nothing here is a real database or a real person. -->

# Quickstart transcript

Recorded 2026-09-17 against `lazyslice` built from this commit
(`make build`), Docker 28.5.2, two `postgres:16` containers.

## Setup

```sh
$ docker run -d --name shop-db  -e POSTGRES_USER=ls -e POSTGRES_PASSWORD=pw \
    -e POSTGRES_DB=shop     -p 55701:5432 postgres:16
$ docker run -d --name shop-dev -e POSTGRES_USER=ls -e POSTGRES_PASSWORD=pw \
    -e POSTGRES_DB=shop_dev -p 55702:5432 postgres:16
```

`schema.sql`, loaded into the source with `psql postgres://ls:pw@127.0.0.1:55701/shop -f schema.sql`:

```sql
create table customers (
  id serial primary key,
  email text not null,
  full_name text not null,
  phone text,
  created_at timestamptz not null default now()
);

create table products (
  id serial primary key,
  sku text not null unique,
  title text not null,
  price_cents integer not null
);

create table orders (
  id serial primary key,
  customer_id integer not null references customers(id),
  placed_at timestamptz not null default now(),
  status text not null default 'placed'
);

create table order_items (
  id serial primary key,
  order_id integer not null references orders(id),
  product_id integer not null references products(id),
  qty integer not null
);

insert into products (sku, title, price_cents)
select 'SKU-' || i, 'Invented Widget ' || i, 500 + i * 10
from generate_series(1, 20) i;

insert into customers (email, full_name, phone)
select
  'customer' || i || '@invented-example.test',
  (array['Priya Narayanan','Mateo Alvarez','Ingrid Solberg','Kwame Boateng','Yuki Tanaka'])[1 + (i % 5)] || ' ' || i,
  '+1-555-01' || lpad(i::text, 2, '0')
from generate_series(1, 400) i;

insert into orders (customer_id, placed_at, status)
select (i % 400) + 1, now() - (i || ' hours')::interval, (array['placed','shipped','delivered'])[1 + (i % 3)]
from generate_series(1, 900) i;

insert into order_items (order_id, product_id, qty)
select (i % 900) + 1, (i % 20) + 1, 1 + (i % 4)
from generate_series(1, 1800) i;
```

## The run README.md's "The run" section trims

```sh
$ export LAZYSLICE_SECRET=$(openssl rand -hex 32)   # a throwaway key for this transcript
$ lazyslice --source postgres://ls:pw@127.0.0.1:55701/shop?sslmode=disable \
            --target postgres://ls:pw@127.0.0.1:55702/shop_dev?sslmode=disable \
            --root customers -n 50 --no-config
  source shop on 127.0.0.1 as ls (flag) — --source
! the role ls can write to 4 table(s) in shop — recommend a read-only role: CREATE ROLE lazyslice_ro LOGIN PASSWORD '…'; GRANT CONNECT ON DATABASE shop TO lazyslice_ro; GRANT USAGE ON SCHEMA public TO lazyslice_ro; GRANT SELECT ON ALL TABLES IN SCHEMA public TO lazyslice_ro; GRANT EXECUTE ON FUNCTION pg_control_system() TO lazyslice_ro;
  target shop_dev on 127.0.0.1 — --target
  4 tables on Postgres 160015: 17 columns, 3 foreign keys
  public.customers.created_at: 200/200 samples look like secrets; timestamp is not an accepted type for credential; no name signal
  public.customers.email: name matches email; 200/200 samples parse as addresses
  public.customers.full_name: name matches person_name; 200/200 samples mixed digits and words
  public.customers.id: no name or value signal; surrogate key: preserved verbatim
  public.customers.phone: name matches phone
  public.order_items.id: no name or value signal; surrogate key: preserved verbatim
  public.order_items.order_id: no name or value signal; foreign key to public.orders: preserved verbatim
  public.order_items.product_id: no name or value signal; foreign key to public.products: preserved verbatim
  public.order_items.qty: no name or value signal
  public.orders.customer_id: no name or value signal; foreign key to public.customers: preserved verbatim
  public.orders.id: no name or value signal; surrogate key: preserved verbatim
  public.orders.placed_at: 200/200 samples look like secrets; timestamp is not an accepted type for credential; no name signal
  public.orders.status: nothing recognised in 200 samples, not proof the column is impersonal
  public.products.id: no name or value signal; surrogate key: preserved verbatim
  public.products.price_cents: no name or value signal
  public.products.sku: nothing recognised in 20 samples, not proof the column is impersonal
  public.products.title: 20/20 samples mixed digits and words; no name signal
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
$ echo $?
0
```

`created_at`, `placed_at`, `qty`, `price_cents`, `.status`, `.sku` and the
`order_items`/`orders`/`products` FK-column reasons are the lines the README
excerpt drops — they are honest, but they are not about personal data, and
"200/200 samples look like secrets; timestamp is not an accepted type for
credential" is the classifier trying a signal that Postgres's own type system
already rules out (ADR-010's recall boundary), not a decision that means
anything for a `timestamptz` column.

`root public.customers (named by --root) — --root` is the one line `--root`
answers outright: without it (and with no root already recorded in
`lazyslice.yml`), this run would have stopped at a terminal to ask
`root table? [customers]` first — ADR-008's one blocking question — and
printed the same decision line either way once it had an answer.

## Masked values differ from the source

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

`full_name` masks to something address-shaped rather than name-shaped in this
build — that is `mask`'s own generator choice for `person_name` on this
input, not a masking miss (the value is not the source's, and it is not
another row's either).

## Foreign keys hold, and the marker says the run finished clean

```sh
$ psql postgres://ls:pw@127.0.0.1:55702/shop_dev \
    -c "select count(*) from order_items oi left join orders o on oi.order_id = o.id where o.id is null"
 count
-------
     0
(1 row)

$ psql postgres://ls:pw@127.0.0.1:55702/shop_dev -c "select run_id, status from lazyslice_meta"
                run_id                |  status
--------------------------------------+----------
 c0fbda56-9c1c-4fff-81c8-42b46ff77bd2 | complete
(1 row)
```

`status` is written `complete` only after every check in ARCHITECTURE.md
section 6 has passed (foreign keys, row counts, sequences, the residual scan).
A failing run always leaves it `failed`, but only a residual-class failure
(exit 9 — a masked column still holding a source value, an unconfirmable
hit, or the second net) also empties the tables this run loaded, rather than
leaving a suspected leak on disk; an exit 7 (a row count or sequence) or exit
8 (a foreign key) failure leaves the loaded rows in place, because those are
what an operator diagnoses the failure against (internal/core's `closeRun`).
This build
prints no separate "✓ verified" line to the terminal or to `--json` on a
passing run — the marker row above is, today, the only place that says so
without reading the exit code. That gap is filed as tracker task **T-0265**.

## Re-running against the same target reloads it

```sh
$ lazyslice --source postgres://ls:pw@127.0.0.1:55701/shop?sslmode=disable \
            --target postgres://ls:pw@127.0.0.1:55702/shop_dev?sslmode=disable \
            --root customers -n 50 --no-config
  source shop on 127.0.0.1 as ls (flag) — --source
! the role ls can write to 4 table(s) in shop — recommend a read-only role: CREATE ROLE lazyslice_ro LOGIN PASSWORD '…'; GRANT CONNECT ON DATABASE shop TO lazyslice_ro; GRANT USAGE ON SCHEMA public TO lazyslice_ro; GRANT SELECT ON ALL TABLES IN SCHEMA public TO lazyslice_ro; GRANT EXECUTE ON FUNCTION pg_control_system() TO lazyslice_ro;
  target shop_dev on 127.0.0.1 — --target
  shop_dev carries lazyslice's own marker for this source: it will be truncated and reloaded
  4 tables on Postgres 160015: 17 columns, 3 foreign keys
  ...
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

## Naming the same database as both `--source` and `--target` is refused

```sh
$ lazyslice --source postgres://ls:pw@127.0.0.1:55701/shop?sslmode=disable \
            --target postgres://ls:pw@127.0.0.1:55701/shop?sslmode=disable \
            --root customers -n 5 --no-config
  source shop on 127.0.0.1 as ls (flag) — --source
! the role ls can write to 4 table(s) in shop — recommend a read-only role: CREATE ROLE lazyslice_ro LOGIN PASSWORD '…'; GRANT CONNECT ON DATABASE shop TO lazyslice_ro; GRANT USAGE ON SCHEMA public TO lazyslice_ro; GRANT SELECT ON ALL TABLES IN SCHEMA public TO lazyslice_ro; GRANT EXECUTE ON FUNCTION pg_control_system() TO lazyslice_ro;
✗ target shop on 127.0.0.1 is the source database
lazyslice: target.refused.same_database: the target ls@127.0.0.1:55701/shop (+1 param) is not eligible
$ echo $?
2
```

## Cleanup

```sh
$ docker rm -f shop-db shop-dev
```
