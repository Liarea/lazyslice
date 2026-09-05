# lazyslice

**One line:** Point it at a production SQL database and get a small, referentially complete, pseudonymised copy in a local database, in one command, with no config.

## The user

A backend developer on a three to thirty person team. They own a service with a real database and need realistic data on their laptop and in CI. Today they either maintain hand-written seed data that drifted from reality months ago, or they copy production and hope nobody notices the customer emails in their local Postgres.

## The moment they reach for it

A bug only reproduces with real data shapes. A new teammate needs a working local environment today. CI tests pass on seed data and fail in staging. Someone from compliance asks where the prod dump on the shared drive came from.

## Principles

**Zero config.** First run asks at most one blocking question and then works; headless runs ask none. Configuration is emitted after a run as a record of what happened, never demanded before it. A committed `lazyslice.yml` never excuses an unseen column: it is masked or the run stops. We refuse to ship any feature whose first-run path is "write a YAML file".

**Safe by default.** Anything that might be personal data is masked unless the user opts a column out, and the tool explains why it masked each one. Free text and JSON columns are masked whole. When the classifier is unsure it masks more, never less. The source is opened read-only and the role's privileges are checked and printed; the target must be empty or one we wrote before. The output is pseudonymised, not anonymised, and we say so. We refuse to ship a flag, mode, or default that copies an unclassified column as-is.

**Terminal first.** One static binary, one-line install, no native dependencies, no call to anything we operate. The same command a human types works headless in CI. The terminal UI is a thin layer over the CLI. We refuse to ship a capability that exists only in the TUI.

## What it does

1. Introspects the source schema: tables, keys, foreign keys, unique indexes, sequences, partitions, sampled values.
2. Classifies columns that look like personal data and says why.
3. Plans a subset: a root table, a row count, parents to completeness, children capped and followed only from the root's descendants, lookup tables whole, cycles named, and an honest row and memory estimate printed before extraction.
4. Streams the rows out of one consistent snapshot, masks them with a keyed hash so joins survive across tables and runs, and loads them in bounded memory.
5. Verifies the target: foreign keys resolve, masked columns hold no source value, sequences are reset. Any failed check is a non-zero exit. Then writes `lazyslice.yml` so the run is reproducible.

## What a delighted first run looks like

```
$ lazyslice
  found 2 postgres containers: shop-db (5432), shop-db-test (5433)
  source  shop-db       41 tables · read-only   --source
  target  shop-db-test  empty                   --target
  17 columns look like personal data (press ? to see why)
  root table? [customers]
  plan: 23 tables, ~18k rows, 3 child tables capped (? for details)
  extract ▓▓▓▓▓▓▓▓▓▓ 18,204 rows   mask 17 cols   load ▓▓▓▓▓▓▓▓▓▓
  ✓ 500 customers and their records, in 38s
  ✓ foreign keys verified · no source values in masked columns · sequences reset
  ✓ wrote lazyslice.yml (commit it for CI)
```

## Non-goals for v1

- Synthetic data generation from a schema alone.
- Scheduling, retention, or sharing snapshots between people.
- A web UI, a hosted service, or any control plane between the developer and their database.
- NoSQL and document stores.
- Schema migration or diffing.
- Databases other than PostgreSQL. A second engine waits until the invariant suite passes on every Postgres fixture.
- Detecting personal data inside binary blobs. Text and JSON are not exempt; they are masked whole.
- Any unsafe or pass-through mode.
- Anonymisation, k-anonymity, or any compliance guarantee.
- A cloud or LLM classifier over production values.
- Format-preserving encryption.
- Percent-based sampling or traversal knobs in config.
- Parsing `docker-compose.yml` for connection strings.
- Multi-terabyte sources and resumable extracts. A run finishes in minutes or aborts on its budget.
