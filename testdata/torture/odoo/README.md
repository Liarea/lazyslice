# odoo

The ERP. 204 tables from `base`, `mail` and `contacts`, and the most-connected
table in the whole set: `public.res_users` has **330** incoming foreign keys,
because every model Odoo defines carries `create_uid` and `write_uid`.

| | |
|---|---|
| Upstream | https://github.com/odoo/odoo |
| Pin | the official image `odoo:18`, digest `sha256:259fa933bf3ee7f3e375bd74d1e0bc28bd75955159723be477359e0fdb8acf67` |
| Modules | `base,mail,contacts`, `--without-demo=all` |
| Built with | that image against `postgres:16` |
| `schema.sql` | 204 tables, 622 foreign keys, 2,048 columns |
| Root | `public.res_users` |

**Why an image digest and not a commit.** Odoo has no schema file. Its tables
exist only as Python model classes, and the DDL is emitted by the ORM when a
database is initialised. So the pin is the thing that emits it:
`odoo -d odoo -i base,mail,contacts --without-demo=all --stop-after-init` against
an empty database, then `pg_dump --schema-only --no-owner --no-privileges`. The
image digest fixes the Odoo version exactly, which a branch name would not.

**Why a subset.** `base,mail,contacts` is three of Odoo's several hundred
modules. Installing all of them is an hour and forty thousand tables' worth of
ERP; these three are the ones every Odoo database has, and they carry the parts
this fixture is for.

**The schema is schema-only.** Odoo seeds its own reference data — currencies,
countries, languages, the admin user — when it initialises a database, and none
of that is in `schema.sql`. `generate.sql` therefore generates the currencies and
countries too. Shipping Odoo's own data would be ten times the file for nothing:
the fixture's subject is the shape of the schema.

## What it is here for

* **One table referenced by everything.** A slice rooted at `res_users` makes
  almost every table in the database a depth-1 child. It is the worst case for
  the child walk and the reason `--depth` and `--cap` exist.
* **Translatable fields are `jsonb`.** `res_country.name` holds
  `{"en_US": "Belgium"}`, not text, and dozens of columns are like it — so a
  classifier that only looks at `text`-typed columns sees no names anywhere.
* **The personal data is in `res_partner`**, a table whose name says nothing:
  Odoo models a person and a company as the same row, and the addresses, phone
  numbers, VAT numbers and bank accounts hang off it.
* **`res_country.phone_code integer`** is why this schema needs an `--unmask`: a
  name hit puts it in the `phone` category and E.164 does not fit in an integer,
  so the plan refuses (exit 12) rather than dying in the loader. See
  docs/TORTURE.md.
