# plausible

The analytics tool's account database: 42 tables. The events live in ClickHouse,
so this schema is all identity and no analytics.

| | |
|---|---|
| Upstream | https://github.com/plausible/analytics |
| Commit | `e74d6fb214b76664442d6a6bb8d96e74807ccfbf` |
| Artifact | `priv/repo/structure.sql` (Ecto's own `mix ecto.dump` output) |
| SHA-256 of the artifact | `175f8b5af1935edcc9094bfbdfb53395ab479c382ce54203906dd998342c15c4` |
| Built with | `postgres:16` (digest `sha256:f1c3376c26f2609ab9f29f71f824103fe2fcd8ee0346485cb6122a4f93df6f94`) |
| `schema.sql` | 42 tables, 40 foreign keys, 294 columns |
| Root | `public.sites` (20 incoming foreign keys) |

**How `schema.sql` was derived.** Loaded and dumped back with
`pg_dump --schema-only --no-owner --no-privileges`, `\restrict` lines removed.
Nothing else changed.

## What it is here for

* **`citext`.** `users.email` and `invitations.email` are case-insensitive text —
  an extension's own base type, which is neither an enum nor a domain nor a
  composite, so nothing named it and nothing registered it. And
  `monthly_reports.recipients` is `citext[]`, which is
  `testdata/regressions/005-array-of-extension-type-not-registered.sql`: the load
  wrote nonsense down a binary `COPY` and the server answered 08P01.
* **A two-sided CHECK over two nullable columns.** `goals` is
  `CHECK (event_name IS NOT NULL AND page_path IS NULL OR event_name IS NULL AND
  page_path IS NOT NULL)`, which no generator that only fills NOT NULL columns
  can satisfy; `generate.sql` writes that table by hand and says so.
* **Personal data about non-users.** `invitations.email` is the address of
  somebody who does not have an account yet.
* **The root is not the people.** `public.sites` is the most-connected table, so
  the run reaches people upwards through `site_memberships`.
