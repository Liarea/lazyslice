# mastodon

The federated social server: 118 tables, 156 foreign keys, and one function that
lazyslice does not recreate. **This is the tenth schema of gate 5 — the one that
fails.**

| | |
|---|---|
| Upstream | https://github.com/mastodon/mastodon |
| Commit | `26fce0f2f9e36ea0e0c2b03b7f57d1b1ea58ed1c` |
| Artifacts | `db/schema.rb` and `lib/mastodon/snowflake.rb` |
| SHA-256 of `db/schema.rb` | `5eae120dee4d71e82fbeaa2bd49f25556b94916e200e1ae6f8c09ee9e2d71188` |
| SHA-256 of `lib/mastodon/snowflake.rb` | `6aa3884d6ec5f6b8a489a1dcf9989586f5049c383b2ece694865baefbe1fe032` |
| Built with | `ruby:3.3-slim` (digest `sha256:79f7a07363931fde1a5b312dee281fd62ddf56c33bdba0c2622af9b4621182fb`), activerecord ~> 8.1, pg, scenic; loaded into `postgres:16` |
| `schema.sql` | 118 tables, 156 foreign keys, 1,018 columns |
| Root | `public.accounts` (85 incoming foreign keys — the most connected table in the whole set after Odoo's `res_users`) |

**How `schema.sql` was derived.** Mastodon ships `db/schema.rb`, the Rails schema
DSL, not SQL. `../build.sh mastodon` installs ActiveRecord and Scenic in a
`ruby:3.3-slim` container, points ActiveRecord at an empty database and `load`s
`schema.rb` into it — no Rails, no application, no migrations — then dumps the
result with `pg_dump --schema-only --no-owner --no-privileges`. Scenic is needed
because `schema.rb` uses `create_view`, which is Scenic's extension to the DSL.

**The one deviation, and it is a salt.** Nine of Mastodon's primary keys default
to `timestamp_id('accounts')`, a plpgsql function the application installs from
`lib/mastodon/snowflake.rb`; `schema.rb` cannot be loaded without it. That
function hashes with `SecureRandom.hex(16)`, a *different* salt every time it is
installed, which would make `schema.sql` different on every build. The build
script installs it with a fixed salt of thirty-two zeroes instead, so the file is
reproducible. Everything else about the function is upstream's, character for
character.

## Why this one fails

```
✗ public.accounts.id depends on timestamp_id, a function lazyslice does not
  recreate in the target
lazyslice: target.schema.not_recreatable.function: public.accounts.id depends on
  timestamp_id, which lazyslice does not recreate
```

Exit 13. ARCHITECTURE.md §11.1 lists functions among the object classes v1 does
not recreate, and raises exit 13 when a recreated object depends on one: "There
is no flag that drops the default silently, because the application's first
`INSERT` is the point of the tool." A Mastodon target without `timestamp_id`
would accept lazyslice's rows and refuse the application's.

So the run cannot succeed, and the suite asserts the refusal rather than working
around it: `internal/invariants`'s `TestTortureSchemas` requires exit 13 and that
event code, by name. What T-TORTURE *did* fix is how the refusal arrived — it
used to be exit 1, `run.refused.internal`, "run with --debug", with the real code
buried inside the message
(`testdata/regressions/002-function-default-refusal-uncoded.sql`).

The three `--unmask` flags in the catalogue were the plan-time refusals that
came first, back when §11.1's refusal was raised inside `load.Load`; T-HARD-A
moved it ahead of the plan, so this run now reaches exit 13 whatever the flags
say. Two more flags stood here until T-0112: `users.confirmation_token` and
`users.reset_password_token`, unique credential columns nothing could mask
(T-0098). Removing them changes nothing about this run — it refuses before the
plan — which is why the columns of this schema are no evidence that
`credential_unique` works, and docs/TORTURE.md says so.

## What else it is here for

* **A person split across two tables.** `accounts` is the public identity and
  `users` the private one, one-to-one, and the root is `accounts` — so every
  address and password digest in the fixture is reached the long way round.
* **`notifications` (activity_type, activity_id)**: the third undeclared
  polymorphic pair in the set.
