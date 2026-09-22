# Listings: places a PR adding lazyslice would be welcome

Draft only. Nothing here is opened or submitted; the maintainer reviews and
sends the PRs.

Ten repositories, each checked today (2026-09-22) by fetching its live
`README.md` from GitHub — via `raw.githubusercontent.com` at the branch
named below, cross-checked against `gh api repos/<owner>/<repo>` for
archived status, star count and last-push date, all run today — to confirm
the section still exists, is still active, and to copy its exact current
list-item format rather than guess at one. Repo description: "Subsets a
production PostgreSQL database by a root table across foreign keys, masks
personal data with a deterministic keyed hash, and loads a small local
copy — one command, no config." License: `Apache-2.0`. Language: `Go`.
Repo URL throughout: `https://github.com/Liarea/lazyslice`.

---

## 1. dhamaniasad/awesome-postgres

- **Repo:** https://github.com/dhamaniasad/awesome-postgres (not archived,
  ~8.9k stars, confirmed active today)
- **Section:** `### Utilities` (README.md, line 240 as fetched today) —
  already carries Greenmask, a comparable Postgres anonymization tool, at
  line 246, confirming the section's fit.
- **Exact line the PR adds**, inserted alphabetically after `ldap2pg` and
  before `migra` (matching the section's existing `* [name](url) -
  description.` style):

  ```
  * [lazyslice](https://github.com/Liarea/lazyslice) - Subset a production Postgres database by a root table, mask personal data deterministically, and load a small local copy — one command, no config.
  ```

## 2. mgramin/awesome-db-tools

- **Repo:** https://github.com/mgramin/awesome-db-tools (not archived,
  5,314 stars, `pushed_at` 2026-05-21 — confirmed via `gh api` today; not
  to be confused with the `pierrebrunelle/awesome-db-tools` fork, which has
  0 stars and is a fork of this one)
- **Section:** `### Generation/Masking/Subsetting` (README.md, line 458 as
  fetched today) — the single best-fitting section of the ten: it already
  lists Greenmask and Faker as neighbors.
- **Exact line the PR adds**, inserted alphabetically after `Faker` and
  before `Greenmask`:

  ```
  - [lazyslice](https://github.com/Liarea/lazyslice) - Subsets a Postgres database by root table across foreign keys, masks personal data deterministically, and loads a small local copy — one command, no config.
  ```

## 3. avelino/awesome-go

- **Repo:** https://github.com/avelino/awesome-go (the canonical Go
  awesome-list; not archived, actively maintained, confirmed today)
- **Section:** `### Database Tools`, under `## Database` (README.md, line
  874 as fetched today) — the right home because lazyslice is a Go binary,
  and the section already lists other single-purpose database CLIs (`dg`,
  a relational-data generator; `onedump`, a one-command database backup
  tool).
- **Exact line the PR adds**, inserted alphabetically after `hasql` and
  before `octillery` (matching the section's lowercase-name, no-trailing-
  period-on-name style):

  ```
  - [lazyslice](https://github.com/Liarea/lazyslice) - Subsets a production Postgres database by a root table across foreign keys, masks personal data deterministically, and loads a small local copy.
  ```

## 4. awesomelistsio/awesome-postgresql

- **Repo:** https://github.com/awesomelistsio/awesome-postgresql (not
  archived, confirmed active today)
- **Section:** `## Backup and Migration` (README.md, confirmed today) —
  sits next to `pg_dump`, `pgBackRest` and `WAL-G`; lazyslice is not a
  backup tool, but it is the closest existing category to "moves a
  Postgres database somewhere else," and the PR description should say so
  rather than imply lazyslice is a backup tool.
- **Exact line the PR adds**, appended after `Liquibase`:

  ```
  - [lazyslice](https://github.com/Liarea/lazyslice) - Subsets a production database by a root table, masks personal data deterministically, and loads a small local Postgres copy for development and CI.
  ```

## 5. TheJambo/awesome-testing

- **Repo:** https://github.com/TheJambo/awesome-testing (not archived,
  confirmed active today)
- **Section:** `### Test Data Management` (README.md, line 119 as fetched
  today) — already lists `dbmask` ("Masks sensitive data in SQL test
  databases with deterministic fakes and verifies the masking row by
  row"), the closest existing neighbor in the whole set of ten.
- **Exact line the PR adds**, appended after `MockJutsu` (matching the
  section's `- [name](url) - description.` style, ending each item with a
  short language/licence note as several neighbors do):

  ```
  - [lazyslice](https://github.com/Liarea/lazyslice) - Subsets a production Postgres database by root table, masks personal data deterministically so joins survive, and loads a small local copy for testing. Go, Apache-2.0.
  ```

## 6. agarrharr/awesome-cli-apps

- **Repo:** https://github.com/agarrharr/awesome-cli-apps (not archived,
  20,444 stars, confirmed active today)
- **Section:** `### Database` (readme.md, line 230 as fetched today) —
  currently interactive database clients (`mycli`, `pgcli`, `pgxcli`); the
  PR description should be explicit that lazyslice is a one-shot pipeline
  CLI, not a REPL client, since that's the section's current pattern.
- **Exact line the PR adds**, appended after `pgxcli`:

  ```
  - [lazyslice](https://github.com/Liarea/lazyslice) - Snapshot a production Postgres database into a small, masked, referentially complete local copy in one command.
  ```

## 7. veggiemonk/awesome-docker

- **Repo:** https://github.com/veggiemonk/awesome-docker (not archived,
  36,865 stars, `pushed_at` 2026-09-12 — confirmed via `gh api` today)
- **Section:** `### Development Environment` (README.md, line 478 as
  fetched today) — sits next to Lando and Laradock, tools that spin up a
  local dev environment's services; lazyslice fills the "with real data in
  it" half those don't cover.
- **Exact line the PR adds**, inserted alphabetically after `Laradock` and
  before `uniget`:

  ```
  - [lazyslice](https://github.com/Liarea/lazyslice) - Snapshot a production Postgres container into a small, masked, referentially complete copy in another container — one command, no config.
  ```

## 8. bramaos/brama (comparison page, not an awesome-list)

- **Repo:** https://github.com/bramaos/brama — a pre-alpha CLI (not yet
  implemented; issue-tracker-only) that pulls anonymized production
  databases to a local environment, and whose own README already runs
  through the field: *"The tools that exist pick one half... Tonic.ai
  anonymizes at enterprise prices. Neon and PlanetScale branch databases
  they host for you. Neosync was archived in July 2025."* and, separately,
  *"Every comparable tool — Greenmask, PostgreSQL Anonymizer, Snaplet,
  Percona — passes columns it has no rule for straight through, unmasked."*
  (both quoted exactly as fetched today from
  `raw.githubusercontent.com/bramaos/brama/main/README.md`).
- **Section:** the unnamed comparison prose in the README's problem
  statement (around "The tools that exist pick one half").
- **Exact line the PR adds**, as a new sentence appended to that
  paragraph — this is a competitor's own README, so the PR should be
  framed as a correction (a shipped alternative exists) rather than
  self-promotion, and the maintainer should expect it might not be merged:

  ```
  lazyslice (https://github.com/Liarea/lazyslice) ships today, refuses to load an unclassified column instead of passing it through, and has no cloud component to be acquired or archived.
  ```

## 9. jaywcjlove/awesome-mac

- **Repo:** https://github.com/jaywcjlove/awesome-mac (not archived,
  114,381 stars, `pushed_at` today, 2026-09-22 — the most active list of
  the ten)
- **Section:** `### Developer Utilities` (README.md, line 406 as fetched
  today) — the list's `### Databases` section (line 591) is GUI database
  *clients* (DataGrip, DBeaver, Beekeeper Studio) and is the wrong fit for
  a CLI pipeline tool; `Developer Utilities` already mixes CLI tools
  (`AXe`, `Swifka`) with GUI ones, which is the right neighborhood.
- **Exact line the PR adds**, matching the section's `* [name](url) -
  description. [icon markup]` style used by other open-source CLI entries
  in the same section (e.g. `Swifka`):

  ```
  * [lazyslice](https://github.com/Liarea/lazyslice) - CLI that subsets a production Postgres database, masks personal data, and loads a small local copy. [![Open-Source Software][OSS Icon]](https://github.com/Liarea/lazyslice) ![Freeware][Freeware Icon]
  ```

## 10. awesome-selfhosted/awesome-selfhosted

- **Repo:** https://github.com/awesome-selfhosted/awesome-selfhosted (not
  archived, confirmed active today; by far the largest list here)
- **Section:** `### Software Development - Testing` (README.md, line 2064
  as fetched today) — the list's `### Database Management` section (line
  691) is explicitly scoped to *"web interfaces for database management"*
  (Adminer, Baserow, Bytebase) and is the wrong fit; `Software Development
  - Testing` ("Tools and software for software testing") already lists
  CLI/Docker tools like Bencher and WebHook Tester in the same
  `- [name](url) - description. ([Source Code](url)) \`license\`
  \`language\`` format lazyslice's own repo *is* its source code.
- **Exact line the PR adds**, appended after `WebHook Tester`:

  ```
  - [lazyslice](https://github.com/Liarea/lazyslice) - Subsets a production PostgreSQL database by a root table, masks personal data deterministically, and loads a small local copy for development and CI. `Apache-2.0` `Go`
  ```
