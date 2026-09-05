# lazysnap

**One line:** Point it at a production SQL database and get a small, referentially complete, anonymised copy in a local database, in one command, with no config.

## The user

A backend developer on a three to thirty person team. They own a service with a real database and need realistic data on their laptop and in CI. Today they either maintain hand-written seed data that drifted from reality months ago, or they copy production and hope nobody notices the customer emails in their local Postgres.

## The moment they reach for it

A bug only reproduces with real data shapes. A new teammate needs a working local environment today. CI tests pass on seed data and fail in staging. Someone from compliance asks where the prod dump on the shared drive came from.

## Principles

**Zero config.** First run asks at most one question and then works. Configuration is emitted after a run as a record of what happened, never demanded before it. We refuse to ship any feature whose first-run path is "write a YAML file".

**Safe by default.** Anything that might be personal data is masked unless the user opts a column out, and the tool explains why it masked each one. It never holds write access to the source. We refuse to ship a flag that disables masking wholesale.

**Terminal first.** One static binary, one-line install, and the same command a human types works headless in CI. The terminal UI is a thin layer over the CLI. We refuse to ship a capability that exists only in the TUI.

## What it does

1. Introspects the source schema: tables, keys, foreign keys, samples.
2. Classifies columns that look like personal data and says why.
3. Plans a subset: a root table, a row count, parents to completeness, children with caps, cycles handled.
4. Streams the rows out, masks them deterministically so joins still work, and loads them into the target.
5. Verifies foreign-key integrity in the target and writes `lazysnap.yml` so the run is reproducible.

## What a delighted first run looks like

```
$ lazysnap
  found 2 postgres containers: shop-db (5432), shop-db-test (5433)
  source: shop-db   target: shop-db-test
  41 tables · 17 columns look like personal data (press ? to see why)
  root table? [customers]
  planning… 23 tables in slice, ~18k rows
  extract ▓▓▓▓▓▓▓▓▓▓ 18,204 rows   mask 17 cols   load ▓▓▓▓▓▓▓▓▓▓
  ✓ 500 customers and everything they touch, in 38s
  ✓ foreign keys verified · wrote lazysnap.yml (commit it for CI)
```

## Non-goals for v1

- Synthetic data generation from a schema alone.
- Scheduling, retention, or sharing snapshots between people.
- A web UI or a hosted service.
- NoSQL and document stores.
- Schema migration or diffing.
- Databases other than PostgreSQL. MySQL, SQLite and SQL Server come after the Postgres path is excellent.
- Detecting personal data inside binary blobs.
