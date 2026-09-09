# django

`django-admin startproject` plus `manage.py migrate`: ten tables, nine foreign
keys, and nothing else. The smallest schema in the set that anyone actually has.

| | |
|---|---|
| Upstream | https://github.com/django/django |
| Pin | Django 5.2.6 from PyPI, on `python:3.13-slim` (digest `sha256:9d2e5553305c7c7b0097999bb17187c69b921ccd6bc9d40e4bb5ebe652c00285`) |
| Built with | that image against `postgres:16` |
| `schema.sql` | 10 tables, 9 foreign keys, 44 columns |
| Root | `public.auth_user` |

**How `schema.sql` was derived.** `../build.sh django` runs, in a
`python:3.13-slim` container, exactly what a person starting a Django project
runs: `pip install django==5.2.6 psycopg[binary]`, `django-admin startproject`, a
`DATABASES` block pointing at an empty PostgreSQL database, `manage.py migrate`.
The `INSTALLED_APPS` are `startproject`'s own defaults, which is what puts
`django.contrib.auth` and `django.contrib.admin` in — the two the task names. The
result is dumped with `pg_dump --schema-only --no-owner --no-privileges`.

Pinning the Django version rather than a git commit is deliberate: nobody
deploys Django from `main`, and `5.2.6` is the thing a person would have.

## What it is here for

* **It is the schema every Python shop has**, and the one where a failure is
  least excusable.
* **`django_admin_log.object_repr`** is the string Django's admin logs for the
  object that changed, and in a real deployment it routinely carries the changed
  user's address — a column whose name says nothing carrying personal data. It is
  hand-labelled in docs/TORTURE.md's truth set for exactly that reason.
* **`django_content_type` is `UNIQUE (app_label, model)`** with both columns
  masked, which is
  `testdata/regressions/004-composite-unique-index-all-masked.sql`; and
  **`auth_permission` is `UNIQUE (content_type_id, codename)`** where the
  unmasked half repeats and the masked half is the discriminator, which is the
  other half of the same rule.
* **`auth_group.name`** is a `varchar(150) UNIQUE` that classifies as a person's
  name, which is
  `testdata/regressions/001-unique-index-masking-collision.sql`.
