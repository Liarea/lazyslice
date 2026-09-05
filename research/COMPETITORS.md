# COMPETITORS

A teardown of every tool that does some part of "point at a production SQL database, get a small, referentially complete, anonymised copy in a local database".

Researched 4–5 September 2026. Every factual claim links to its source. Where a fact could not be verified it says **unverified** rather than guessing. Star counts, commit dates, issue counts and reaction counts were read from the [GitHub REST API](https://docs.github.com/rest) and the [GitLab API](https://docs.gitlab.com/api/rest/) on 4–5 September 2026 and will drift.

Time-to-first-snapshot figures in the table near the end are **estimates derived from reading each quickstart**, not measured runs. They are flagged as such.

**A note on what this document cannot tell you.** No performance data is gathered anywhere below beyond condenser's own vendor statement that its open-source tool "can subset databases up to 10GB, but it will struggle with larger databases." There is no throughput, memory or wall-clock measurement for any tool in this document, from us or from a third party. The "time to first snapshot" table further down measures **setup cost**, not whether a tool actually finishes against a large source — that question is open for every row in it.

---

## Orientation

| Tool | Lang | Subsets? | Masks? | Detects PII? | Needs write access to source? | Alive? |
|---|---|---|---|---|---|---|
| [Greenmask](#greenmask) | Go | yes | yes | no | no evidence found — dumps via SELECT/joins, no CREATE step documented | active |
| [Basecut](#basecut) | closed binary | yes | yes | yes | closed source — **unverified** | launched 2026, releases paused since March |
| [dbslice](#dbslice-2026) | Python | yes | yes | yes | **unverified** — not documented | new 2026, most direct OSS rival |
| [Neosync](#neosync) | Go | yes (SQL query) | yes | yes | **unverified** — archived, not re-checked | **dead** — archived 30 Aug 2025 |
| [Snaplet Snapshot](#snaplet-snapshot) | TypeScript | yes | yes | partial | **unverified** — archived, not re-checked | **dead** — archived |
| [PostgreSQL Anonymizer](#postgresql-anonymizer) | Rust + PL/pgSQL | no | yes | yes (`anon.detect`) | **yes** — an extension, so superuser to install | very active |
| [Jailer](#jailer) | Java | yes | filters only | no | **no** — since 2016 rows are collected in an embedded H2 database precisely so it can run against read-only sources | very active, GUI-first |
| [condenser](#condenser) | Python | yes | no | no | **yes on MySQL** — creates and drops `tonic_subset_temp_db_*` on the source ([#25](https://github.com/TonicAI/condenser/issues/25)); Postgres path unverified | **stale** — last commit 2023 |
| [Replibyte](#replibyte) | Rust | yes | yes | no | **unverified** | **dead** — last commit 2024, won't build |
| [pgsubset](#pgsubset) | Rust | yes | no | no | **unverified** | **dead** — one day of commits, 2022 |
| [pg_subsetter](#pg_subsetter-teamniteo) | Go | yes (fraction) | no | no | **unverified** | low activity |
| [pg_sample](#pg_sample) | Perl | yes | no | no | **yes** — `CREATE SCHEMA _pg_sample` on the source, fails on read replicas ([#31](https://github.com/mla/pg_sample/issues/31)) | low activity |
| [Tonic Structural](#tonic-structural) | closed | yes | yes | yes | **unverified** — closed source | commercial, enterprise-priced |
| [Redgate TDM / Data Masker](#redgate-data-masker-and-redgate-test-data-manager) | closed | TDM only | yes | yes | **unverified** — closed source, no docs found | commercial, per-TB |
| [fixturize](#fixturize-2026) | Go | yes | yes | yes | **unverified** — not documented | new 2026, quiet since May |
| [pgEdge Anonymizer](#pgedge-anonymizer-2025) | Go | no | yes | no | n/a — masks in place, has write access by design | new 2025, active |
| [Brume](#brume-2026) | Java | yes | yes (pseudonymisation) | no | **unverified** — not documented | new 2026 |
| [pg_partialcopy](#pg_partialcopy-2025) | Go | yes (hand-written SQL) | yes (in SQL) | no | **unverified** — not documented | new 2025 |
| [slice-db](#slice-db) | Python | yes | yes | no | **unverified** — no mention in README; reads under "consistent snapshots" suggests read-only transactions | low activity |
| [msg555/subsetter](#msg555subsetter) | Python | yes | yes (faker) | no | **unverified** — not documented | active, low profile |
| [DBSnapper](#dbsnapper) | closed binary | yes | SQL script | no | **unverified** — closed source | commercial |
| [db-condenser](#db-condenser-fork-of-condenser) | Python | yes | no | no | inherits condenser's MySQL temp-db behaviour ([source](https://github.com/tkhuu01/db-condenser)) — **yes on MySQL, unverified for the rest** | fork, new 2025/26 |

**CONCEPT.md makes "it never holds write access to the source" a stated principle, and it is the one question this field's own documentation almost never answers.** Only two tools give a real, sourced answer either way: pg_sample requires it and fails on read replicas, and Jailer explicitly redesigned around not needing it in 2016. condenser needs it for MySQL specifically — a live, reproducible traceback, not a guess. Everything else is silence, which for a tool that will be pointed at production is itself a finding.

Adjacent but not competing: [Seedfast](#adjacent-not-competitors), [VeilStream](#adjacent-not-competitors), [Xata](#adjacent-not-competitors), [Ardent](#adjacent-not-competitors), [simple-anonymizer](#adjacent-not-competitors), [recordrelay](#recordrelay-2026).

---

## Greenmask

**Repo:** [GreenmaskIO/greenmask](https://github.com/GreenmaskIO/greenmask) · **Site:** [greenmask.io](https://www.greenmask.io/) · **Docs:** [docs.greenmask.io](https://docs.greenmask.io/)

- **Language:** Go. Described by its own docs as ["written in pure Go and includes ported PostgreSQL libraries, making it platform-independent"](https://docs.greenmask.io/latest/).
- **Databases:** PostgreSQL only. The documentation index covers no other engine; MySQL is an open epic ([#222](https://github.com/GreenmaskIO/greenmask/issues/222), opened 2024-10-15, still open).
- **Subsetting:** You write `subset_conds` on a table in the dump config. Greenmask introspects the schema, builds a table dependency graph, and — per the [Database subset docs](https://docs.greenmask.io/latest/database_subset/), which read "Greenmask genrates queries for subset conditions based on the introspected schema using joins and recursive queries" [sic] — generates queries using joins and recursive queries. Traversal is **bidirectional**: filtering a parent pulls in the children that reference it, and pulls up the parents needed to keep FKs satisfiable.
- **Cycle handling:** Recursive SQL. The docs state the limitation plainly, in the vendor's own wording, typos included: Greenmask "can resolve multi-cylces in one strogly connected component, but only for one group of vertexes" [sic] ([database_subset](https://docs.greenmask.io/latest/database_subset/)). The remaining case is tracked as [#197 "Advanced Subset: Multi-cycles with more than one group in one SCC"](https://github.com/GreenmaskIO/greenmask/issues/197). The same docs page also gives the mitigating claim section 2 below omits: "In practice this is quite rare situation and 99% of people will not face this issue." Missing FKs are declared by hand as `virtual_references`, and polymorphic associations via `polymorphic_exprs`.
- **Write access to source:** no evidence found that it needs any. The subset mechanism is SELECT-based joins and recursive CTEs read against the source during dump; nothing in the [database_subset docs](https://docs.greenmask.io/latest/database_subset/) or [architecture docs](https://docs.greenmask.io/latest/architecture/) describes a CREATE step on the source, in contrast to pg_sample and condenser below.
- **Masking config:** YAML, entirely manual. Each transformer names a schema, table, column and transformer type — see the shipped [playground/config.yml](https://github.com/GreenmaskIO/greenmask/blob/main/playground/config.yml) (35 lines, and it masks exactly one column). **There is no PII detection.** The [configuration docs](https://docs.greenmask.io/latest/configuration/) require every transformed column to be listed explicitly; nothing in the tool tells you which columns look personal.
- **Install:** `curl -fsSL https://greenmask.io/install.sh | sh`, `brew install greenmask`, `docker run greenmask/greenmask:latest`, or `git clone && make build` ([installation docs](https://docs.greenmask.io/latest/installation/)). It additionally requires PostgreSQL client binaries whose major version matches the destination server, and the config must point at them via `common.pg_bin_path`.
- **Licence:** Apache-2.0.
- **Last commit:** 2026-08-22, `d34cce1e` "docs: release notes v0.2.23". Latest release [v0.2.23](https://github.com/GreenmaskIO/greenmask/releases), 2026-08-22. Still pre-1.0 nearly three years after the repo was created (2023-12-01); [#358 "[EPIC] Greenmask V1"](https://github.com/GreenmaskIO/greenmask/issues/358) opened 2025-11-04.
- **Stars:** 1,757 · **Forks:** 69 · **Open issues:** 51 (includes PRs). Its [Show HN for 0.2 in October 2024](https://news.ycombinator.com/item?id=41863600) drew 94 points and 19 comments.

**Three most-upvoted open issues**

1. [#222 epic: MySQL support](https://github.com/GreenmaskIO/greenmask/issues/222) — 8 reactions (+7), open since 2024-10-15.
2. [#104 Bug: --data-only flag interfere with --schema-only](https://github.com/GreenmaskIO/greenmask/issues/104) — 3 reactions (+3), open since 2024-05-08.
3. [#111 feat: unique transformations](https://github.com/GreenmaskIO/greenmask/issues/111) — 2 reactions (+1), open since 2024-05-12.

Also load-bearing for our purposes: [#329 "greenmask does not work if there is not cycle in the database"](https://github.com/GreenmaskIO/greenmask/issues/329) — 18 comments, open since 2025-08-13; the reporter's schema has *no* cycles and the subset component errors anyway, pointing at [`component.go` L77–88](https://github.com/GreenmaskIO/greenmask/blob/8b714baf6d2e249cf271a76c21bec52d8cb77251/internal/db/postgres/subset/component.go#L77-L88). [#392 "epic: subset system revision"](https://github.com/GreenmaskIO/greenmask/issues/392) (2026-01-16) concedes the point in the maintainers' own words: "there are currently limitations that prevent this system from being used effectively in databases with cyclic references." [#338 "High collision rate in default company-name anonymizer"](https://github.com/GreenmaskIO/greenmask/issues/338) documents a 148 × 6 = 888-value name space sampled with replacement, producing duplicates that "can violate uniqueness constraints".

---

## Basecut

**Site:** [basecut.dev](https://basecut.dev/) · **Docs:** [docs.basecut.dev](https://docs.basecut.dev/) · **Release repo:** [basecuthq/cli](https://github.com/basecuthq/cli)

The closest commercial analogue to what lazysnap proposes. Same pitch, same audience, same verbs.

- **Language:** Closed source. The public repo `basecuthq/cli` contains only `install.sh`, a Homebrew `Formula` directory and a README; the releases carry prebuilt binaries for darwin/linux amd64+arm64 and windows amd64, which is consistent with Go, though the source is not published — **language unverified**.
- **Databases:** "PostgreSQL today. MySQL and SQL Server are on the roadmap." ([pricing FAQ](https://basecut.dev/pricing)). Quickstart requires PostgreSQL 12+.
- **Subsetting:** Root-table plus traversal budgets. The documented config declares `from:` with a table and a parameterised `where:`, then `traverse: parents: 5` / `children: 10` ([Welcome to Basecut](https://docs.basecut.dev/)). `basecut init` does "FK analysis", "root table suggestions" and "cycle detection" automatically ([Quick Start](https://docs.basecut.dev/getting-started/quick-start)). The cycle *algorithm* is not published — **unverified**.
- **Masking config:** `anonymize: auto` is the documented default, with `mode: auto | manual | off`, an `excluded_domains` list, and table- or wildcard-keyed `rules` such as `'*.email': fake_email` ([Anonymization](https://docs.basecut.dev/configuration/anonymization)). Masking is applied during extraction, "before data is written to a snapshot", so a restore cannot skip it. 43 strategies are listed on that page (`redact`, `hash`, `fake_email`, `email_preserve_domain`, `fake_credit_card`, `date_shift`, `partial_mask`, `numeric_noise`, …). Team and Enterprise plans can enforce org-wide rules.
- **Write access to source:** closed source — **unverified**.
- **Install:** `brew install basecuthq/cli/basecut` or `curl -fsSL https://basecut.dev/install.sh | sh` ([Installation](https://docs.basecut.dev/getting-started/installation)).
- **Licence:** Proprietary. Free tier: up to 3 team members, 20 snapshots/month, 30-day retention. Team: $99/month, up to 20 members. Enterprise: custom ([pricing](https://basecut.dev/pricing)).
- **Last release:** `v0.1.21`, 2026-03-26. No public release in the five months to September 2026, and the release repo's last push is the same date. **Development status, checked directly rather than left as a guess:** the marketing site (`basecut.dev`, HTTP 200), the docs site (`docs.basecut.dev`, redirects 200 to `/introduction`) and the app's sign-in page (`app.basecut.dev/sign-in`, HTTP 200) all still serve as of 2026-09-05 — this is not a shutdown, it is a binary release freeze on a product whose site and login are otherwise fully alive. Whether any commercial development continues privately, or whether it is one person maintaining infrastructure on a dead product, remains **unverified** — there is no changelog, blog post, or public statement addressing the five-month release gap.
- **Stars:** 0 on the release repo (it is a binary drop, not the source). Public [Show HN, 2026-03-31](https://news.ycombinator.com/item?id=47586925) got 3 points and 0 comments.

**Open issues:** the release repo has issues enabled and 0 open. There is no public bug tracker for the product, so there is no upvoted-issue list to read. This is itself a finding: with Basecut you cannot see what is broken before you adopt it.

**The seam we can attack.** Step 2 of the quickstart is `basecut login`, which "opens your browser for authentication" and requires a Basecut account before any snapshot exists. Even in "local execution" mode the CLI is account-gated, and the company stores "only metadata — names, timestamps, config" ([homepage](https://basecut.dev/)). A tool that requires signup, a browser round-trip and a hosted control plane is not the same product as one static binary that works offline.

---

## dbslice (2026)

**Repo:** [nabroleonx/dbslice](https://github.com/nabroleonx/dbslice) · **Docs:** [nabroleonx.github.io/dbslice](https://nabroleonx.github.io/dbslice/) · **PyPI:** [dbslice](https://pypi.org/project/dbslice/)

The single most direct open-source competitor, and it shipped after this project's concept was written. Its README uses our own words: "Zero-config start", "Safe by default".

- **Language:** Python 3.10+.
- **Databases:** PostgreSQL fully supported; MySQL and SQLite listed as "Planned (not yet implemented)" ([README](https://github.com/nabroleonx/dbslice#database-support)).
- **Subsetting:** Seed-record driven. `dbslice extract postgres://localhost/myapp --seed "orders.id=12345"`. Its documented pipeline is introspect → traverse → extract → topologically sort → output; traversal is "Starting from seed record(s), follows FK relationships via **BFS**". Direction is a flag: `--direction up|down|both` (default `both`), with `--depth` defaulting to 3. Virtual foreign keys for Django `GenericForeignKey` and other implicit relationships are declared in `dbslice.yaml`.
- **Cycle handling:** its own comparison table claims "Automatic" (versus "Manual config" for Jailer). The mechanism is not described in the README — **algorithm unverified**.
- **Write access to source:** not documented either way — **unverified**.
- **Masking config:** `--anonymize` turns on auto-detection of sensitive fields; `--redact "audit_logs.ip_address"` adds fields; `--compliance gdpr|hipaa|pci-dss` applies profiles, with `--compliance-strict` for HIPAA Safe Harbor's 18 identifier types, and a `.manifest.json` audit artefact. There is also `dbslice map`, a local browser UI on `127.0.0.1:9473` behind a one-time session token for mapping columns to rules and exporting YAML.
- **Install:** `uv tool install dbslice`, `pip install dbslice`, or `uvx dbslice` with no install at all. Output goes to stdout as SQL: `dbslice extract ... > subset.sql && psql -d localdb < subset.sql`.
- **Licence:** MIT, declared in [`pyproject.toml`](https://github.com/nabroleonx/dbslice/blob/main/pyproject.toml) (`license = "MIT"`). **There is no `LICENSE` file in the repository root**, so GitHub's licence API returns none — a real defect for anyone whose legal review greps for one.
- **Last commit:** 2026-06-23. Latest release `v1.0.1`, 2026-06-23. Repo created 2026-02-15.
- **Stars:** 143 · **Open issues:** 1 (a docs PR, [#10](https://github.com/nabroleonx/dbslice/pull/10)).

**Three most-upvoted open issues:** none — the tracker has one open item and it is a pull request. The closed issues are the useful signal: [#8 `--stream` + `--out-file` produces empty output after successful extraction](https://github.com/nabroleonx/dbslice/issues/8) (opened 2026-05-20, fixed 2026-06-23 — a month open, not the same day) and [#1 Schema selection option](https://github.com/nabroleonx/dbslice/issues/1) (fixed 2026-02-28). `pyproject.toml` still classifies it "Development Status :: 3 - Alpha".

**Usage signal beyond stars.** [PyPI download stats](https://pypistats.org/packages/dbslice) show 33 downloads in the last 30 days (checked 2026-09-05) — 143 GitHub stars have not yet translated into real installs, which matters when weighing how "direct" a rival it actually is today.

**Where it is weaker than our concept.** It extracts to a file for you to pipe into `psql` rather than loading and then verifying the target; there is no documented post-load foreign-key verification step; there is no container discovery or "found 2 postgres containers" first-run; and a Python tool installed via pip/uv is not one static binary. Its default `--depth 3` also means the default answer is *not* referential completeness — a parent five hops up is silently dropped.

---

## Neosync

**Repo:** [nucleuscloud/neosync](https://github.com/nucleuscloud/neosync) — **archived**

**Status as of September 2026: dead.** Verified rather than assumed.

- The repository is archived (GitHub API `archived: true`), last commit `8101c42d` **"adds acquired disclaimer"**, 2025-08-30. The README carries the disclaimer: "Neosync has been acquired by Grow Therapy. As a result, this repository is no longer actively maintained."
- The acquisition was announced 2025-09-25 ([Grow Therapy press release](https://www.prnewswire.com/news-releases/grow-therapy-raises-the-privacy-bar-in-mental-health-302567153.html), [Pulse 2](https://pulse2.com/grow-therapy-acquires-data-privacy-company-neosync/)). A 2025-08-01 close date is reported by a [Crunchbase acquisition profile](https://www.crunchbase.com/acquisition/grow-therapy-acquires-neosync-cd81--00632527), but that page returns HTTP 403 to an unauthenticated fetch (checked 2026-09-05) and cannot be verified directly — treat the close date as **unverified**, resting on a paywalled source, while the announcement date is confirmed.
- Infrastructure has been wound down: `docs.neosync.dev` no longer resolves in DNS, and `https://neosync.dev` resolves to a Vercel address but returns HTTP 404 (checked 2026-09-05).
- **No active community fork exists.** A GitHub repository search for `neosync` returns only unrelated projects; none is a maintained continuation.

Historical detail, for the record:

- **Language:** Go (backend), TypeScript (app). **Licence:** MIT Expat, with an `ee/` directory carve-out under a separate enterprise licence ([LICENSE.md](https://github.com/nucleuscloud/neosync/blob/main/LICENSE.md)).
- **Databases:** PostgreSQL, MySQL, S3.
- **Subsetting:** "Subset your production database for local and CI testing using any SQL query" — filters expressed as SQL predicates per table, not a root-table graph walk.
- **Cycle handling:** searched the archived README and the repo's docs (`.mdx`) tree for "cycle" and "circular" — no hits. Its SQL-predicate-per-table model has no traversal step to resolve, which suggests cycles were simply never a concept the tool needed to handle — **mechanism unverified, absence of documentation confirmed by search**.
- **Write access to source:** **unverified** — archived and not a live concern for adoption purposes.
- **Install:** `git clone` then `make compose/up`, landing a web app on `localhost:3000`. A full Docker Compose stack, not a binary.
- **Stars:** 4,141 · **Forks:** 232. Its [Show HN in May 2024](https://news.ycombinator.com/item?id=40443927) drew 246 points and 44 comments — the biggest single burst of attention this category has had. The archiving was noticed on HN in [September 2025](https://news.ycombinator.com/item?id=45331127) and drew 1 point and 1 comment.

**Three most-upvoted open issues** (frozen at archive time)

1. [#3411 Error Encountered When Setting Up MySQL 5.7 Connection](https://github.com/nucleuscloud/neosync/issues/3411) — 2 reactions, 2025-03-26.
2. [#1968 Any option to define destination db's schema?](https://github.com/nucleuscloud/neosync/issues/1968) — 1 reaction, 2024-05-20.
3. [#2067 [NEOS-1138] Add ability to drop tables prior to initialization](https://github.com/nucleuscloud/neosync/issues/2067) — 0 reactions, 2024-05-30.

**The lesson.** 4,141 stars and a YC badge did not produce a business; the outcome was an acqui-hire into a mental-health company that wanted the privacy engineers. The category has now killed two well-funded startups (Neosync, Snaplet) in about eighteen months.

---

## Snaplet Snapshot

**Repo:** [supabase-community/snapshot](https://github.com/supabase-community/snapshot) (redirects from `snaplet/snapshot`) — **archived**

**Status as of September 2026: dead.**

- Snaplet the company shut down, open-sourced its tooling and the team joined Supabase ([Supabase: "Snaplet is now open source"](https://supabase.com/blog/snaplet-is-now-open-source), [HN discussion, 2024](https://news.ycombinator.com/item?id=41244171)). The 31 August 2024 shutdown date comes from the [Wayback Machine capture of the original announcement](http://web.archive.org/web/20240716025550/https://www.snaplet.dev/post/snaplet-is-shutting-down), since the live post is gone. **Corrected from an earlier draft of this document:** `snaplet.dev` has **not** lapsed — `dig snaplet.dev NS` returns live nameservers (`dns1`/`dns2.registrar-servers.com`), `host snaplet.dev MX` returns five active mail-forwarding records, and `whois snaplet.dev` reports `status: ACTIVE` (checked 2026-09-05). Only the web server is gone: there is no A/AAAA record, so the site itself does not load, but the domain registration is current and mail still routes. The docs-unreachable argument in section 8 stands on that narrower fact alone.
- The snapshot repo is archived. Last commit `288e0a26` "try macos-15-intel", 2025-11-05 — a CI fix, not a feature. Last release `v0.93.2`, 2024-08-02, i.e. **no release in over two years**.
- The sibling seeding tool lives on at [supabase-community/seed](https://github.com/supabase-community/seed) but is community-maintained and receives occasional fixes only.
- **Language:** TypeScript · **Licence:** MIT · **Stars:** 328 · **Forks:** 29 · **Open issues:** 12.
- **Databases:** PostgreSQL.
- **Subsetting and masking:** "Snapshots can be subset (reduced in size), and the source data can be transformed to meet your requirements (for example, obfuscating personally identifiable information)" ([README](https://github.com/supabase-community/snapshot#introduction)). Configured in a `snaplet.config.ts` written by `npx @snaplet/snapshot setup`. **Subsetting algorithm, FK traversal direction and cycle handling are all unverified** — the archived README describes the feature only at the level of the sentence quoted above, and the source (still readable on GitHub) was not reverse-engineered for this document; digging into `packages/*/src` for the actual traversal code is the obvious next step if this tool's approach ever matters to us.
- **Write access to source:** **unverified**.
- **Install:** `npx @snaplet/snapshot setup`, then `SNAPLET_SOURCE_DATABASE_URL=… npx @snaplet/snapshot snapshot capture`.
- **Usage signal beyond stars:** despite being archived, `@snaplet/snapshot` still logged 29,631 npm downloads in the 30 days to 2026-08-29 ([npm download stats](https://api.npmjs.org/downloads/point/last-month/@snaplet/snapshot)) — likely legacy CI pipelines that have not been migrated off it, which is itself a small data point on how long a dead tool keeps running unattended.

**Three most-upvoted open issues**

1. [#15 npm install failure](https://github.com/supabase-community/snapshot/issues/15) — 4 reactions (+4), 6 comments, open since 2024-10-18. The user cannot get past `npx @snaplet/snapshot setup`.
2. [#18 Outdated documentation on configuring select and transform](https://github.com/supabase-community/snapshot/issues/18) — 2 reactions, 2025-01-17.
3. [#20 NPM Install Failing related to libtool static on ARM Darwin architecture platforms](https://github.com/supabase-community/snapshot/issues/20) — 2 reactions, 2025-04-21. "No prebuilt binaries found (target=20.19.0 runtime=node arch=arm64 … platform=darwin)".

**The lesson.** The two highest-signal open issues on a database-snapshot tool are both **"I cannot install it"** — on an Apple Silicon Mac, which is what its target user actually owns. Native npm dependencies are a first-run tax we must never pay.

---

## PostgreSQL Anonymizer

**Repo:** [gitlab.com/dalibo/postgresql_anonymizer](https://gitlab.com/dalibo/postgresql_anonymizer) · **Docs:** [postgresql-anonymizer.readthedocs.io](https://postgresql-anonymizer.readthedocs.io/en/stable/)

Not a competitor to the subsetting half at all, and the best-run project in the field on the masking half.

- **Language:** Rust 53.6%, PL/pgSQL 42.2% (GitLab languages API). It is a PostgreSQL **extension**, not a CLI.
- **Databases:** PostgreSQL, plus forks and managed services — the README lists Aiven, Alibaba, Azure, Crunchy, EDB, Google, Greenplum, IBM, Neon, PostgresPro, Yandex and YugabyteDB among others.
- **Subsetting:** **None.** There is no subsetting feature; it masks in place or on export.
- **Masking config:** Declarative DDL. Rules live in the schema itself via `SECURITY LABEL`:
  ```sql
  SECURITY LABEL FOR anon ON COLUMN people.lastname
    IS 'MASKED WITH FUNCTION anon.dummy_last_name()';
  ```
  Six application methods: [Anonymous Dumps](https://postgresql-anonymizer.readthedocs.io/en/stable/anonymous_dumps/), [Static Masking](https://postgresql-anonymizer.readthedocs.io/en/stable/static_masking/), [Dynamic Masking](https://postgresql-anonymizer.readthedocs.io/en/stable/dynamic_masking/), [Replica Masking](https://postgresql-anonymizer.readthedocs.io/en/stable/replica_masking/), [Masking Views](https://postgresql-anonymizer.readthedocs.io/en/stable/masking_views/) and [Masking Data Wrappers](https://postgresql-anonymizer.readthedocs.io/en/stable/masking_data_wrappers/).
- **PII detection:** `SELECT anon.detect('en_US');` returns `table_name`, `column_name`, `identifiers_category`, `direct`, categorised against the HIPAA classification, with `en_US` and `fr_FR` dictionaries ([detection docs](https://postgresql-anonymizer.readthedocs.io/en/stable/detection/)). The docs are refreshingly honest about its limits: it produces false positives, and "false negatives … is the most problematic issue", so "you still need to review the entire database model in search of hidden identifiers".
- **Install:** distribution packages (Debian, Ubuntu, RHEL, Rocky, SUSE), Docker image `registry.gitlab.com/dalibo/postgresql_anonymizer`, or build from source. Installing an extension typically needs superuser on the source — a non-starter for our "never hold write access to the source" principle.
- **Write access to source:** **yes** — see above; the one tool in this document where the write-access question has a definitive, docs-confirmed answer.
- **Licence:** [The PostgreSQL License](https://gitlab.com/dalibo/postgresql_anonymizer/-/blob/latest/LICENSE.md), © 2018–2026 DALIBO SCOP — a permissive, BSD-style licence with no copyleft obligation.
- **Last commit:** 2026-09-05. **Refreshed at time of writing, since the field moved during research:** version **3.2 released 2026-09-05** ([Release 3.2](https://gitlab.com/dalibo/postgresql_anonymizer/-/tags/3.2), superseding 3.1.3 of 2026-06-29); work item [#674 "Release 3.2"](https://gitlab.com/dalibo/postgresql_anonymizer/-/work_items/674) is now **closed** (closed 2026-09-05T09:25Z), not open. The 2026-09-04 commit batch that triggered this research included a fix for **CVE-2026-19633** ("Escalation via custom types, operators and rangevars") — worth knowing given we'd be citing this extension as the masking-detection tool to imitate.
- **Stars:** 294 · **Forks:** 110 (re-checked via the [GitLab API](https://gitlab.com/api/v4/projects/dalibo%2Fpostgresql_anonymizer) 2026-09-05).

**Three most-upvoted open issues.** Upvotes on this tracker barely differentiate — the top four all sit at 1 upvote, so ranking is close to arbitrary. The substantive ones:

1. [#667 Masked role cannot INSERT/UPDATE a table that has a masking rule, even when the statement does not touch any masked column](https://gitlab.com/dalibo/postgresql_anonymizer/-/work_items/667) — 1 upvote, 2026-08-24.
2. [#564 sequences not handled correctly when privacy by default is active](https://gitlab.com/dalibo/postgresql_anonymizer/-/work_items/564) — 1 upvote, 2025-09-03.
3. [#583 Debian Trixie support](https://gitlab.com/dalibo/postgresql_anonymizer/-/work_items/583) — 1 upvote, 2025-11-03.

Worth noting for our own correctness: [#670 "Parallel static masking can still report success when a worker aborts at run time"](https://gitlab.com/dalibo/postgresql_anonymizer/-/work_items/670) (2026-08-26) — a masking tool that reports success while a worker silently died is the exact failure mode a compliance-facing tool cannot have.

**What to steal:** `anon.detect()` and its documented honesty about false negatives. **What to avoid:** requiring an extension, and therefore superuser, on the production database.

---

## Jailer

**Repo:** [Wisser/Jailer](https://github.com/Wisser/Jailer) · **Site:** [wisser.github.io/Jailer](https://wisser.github.io/Jailer/)

The oldest and most capable subsetter here, and the one with the worst fit for a terminal-first workflow.

- **Language:** Java · **Licence:** Apache-2.0 (© 2007–2026 Ralf Wisser).
- **Databases:** any JDBC source in principle; specific support for PostgreSQL, Oracle, MySQL, MariaDB, SQL Server, IBM Db2, SQLite, Sybase, Amazon Redshift, Firebird, Informix, H2 and Exasol ([README](https://github.com/Wisser/Jailer#supported-databases)).
- **Subsetting:** an *extraction model* — a subject table plus a WHERE condition plus association restrictions — walked bidirectionally over foreign-key and user-defined associations. Output is topologically sorted SQL DML, or DbUnit / JSON / YAML / XML datasets. "Subset by Example" lets you browse rows in the GUI and have Jailer synthesise the model from what you collected.
- **Cycle handling:** since 2021-02-04, "Cycles in parent-child relationships will be detected and broken. Thus, such data can be exported by deferring the insertion of nullable foreign keys" ([README news](https://github.com/Wisser/Jailer#news)). This is the most graceful cycle story in the field.
- **Masking config:** none in the anonymisation sense. Jailer has column *filters* (SQL expressions applied on export, propagated from primary keys to the matching foreign keys) but no PII detection, no faker library, and no deterministic masking guarantee. **Anonymisation is not a Jailer feature.**
- **Install:** MSI installer for Windows, `.deb` for Linux, or unzip `jailer_n.n.n.zip`. **The CLI ships only in the zip** — the installers do not include it ([README installation](https://github.com/Wisser/Jailer#installation)).
- **CLI:** the usage string is emitted from source at [`CommandLineParser.java` L122–143](https://github.com/Wisser/Jailer/blob/master/src/main/engine/net/sf/jailer/CommandLineParser.java#L122-L143):
  ```
  sh jailer.sh export [options] <extraction-model> <jdbc-driver-class> <db-URL> <db-user> <db-password> -jdbcjar <JDBC driver jar file>
  ```
  Note the first positional argument: **`<extraction-model>`**. The CLI cannot subset anything without a model file, and the shipped [`extractionmodel/`](https://github.com/Wisser/Jailer/tree/master/extractionmodel) directory contains only two `.jm` demo models. The tool for authoring one is the GUI editor. `jailer.sh` itself is [a 28-line classpath wrapper around `java -cp … net.sf.jailer.Jailer`](https://github.com/Wisser/Jailer/blob/master/jailer.sh) — a shebang, an install-dir preamble, a commented-out JDBC line, eleven `CP=` assignments for bundled jars, and one `java` invocation — and you also supply your own JDBC driver jar.
- **Write access to source:** **no.** Since [2016-10-23](https://github.com/Wisser/Jailer#news), "Rows can alternatively be collected in a separate embedded database. This allows exporting data from read-only databases" — Jailer's working tables live in a local embedded H2 database by default, not on the source. PostgreSQL, Oracle and Db2 can opt into session-scoped temporary tables instead, which need no persistent schema change either. Of every tool in this document, Jailer is the one that explicitly designed around the write-access question years before it was our stated principle.
- **Last commit:** 2026-09-04, `6f1531a4` "subset insight, ongoing work". Latest release `v17.2.2`, 2026-08-19. Nineteen years of continuous maintenance by one person.

  **What "subset insight" actually is, investigated rather than taken on faith.** The 25 most recent commits (2026-08-27 to 2026-09-04, [commit history](https://github.com/Wisser/Jailer/commits/master)) show twelve consecutive commits titled "subset insight, ongoing work" or "project subset insight", interleaved with `store RowOriginChain`, `reversed originPath`, and, on 2026-08-26 to 08-27, a **"New feature: Discover Associations"** with an "improved association discovery dialog". Row-origin chains are per-row provenance — why is this specific row in the subset — and "Discover Associations" is implicit foreign-key discovery via a dialog, i.e. exactly the automatic virtual-FK detection this document elsewhere claims (§9, §632) nobody has. This work is unreleased (the last tagged release, `v17.2.2`, predates it) and its final shape — plan preview, provenance report, or something else — is not yet documented anywhere Jailer publishes, so we cannot say it *is* a plan-preview feature, only that its building blocks (origin chains, association discovery) point directly at the two differentiators this document claims as open ground. **The "nobody does this" claims about per-row provenance and a plan preview should be read as "nobody ships this today," not "nobody is building it" — the field's most mature incumbent is actively working in exactly this direction as of the week this document was written.**
- **Stars:** 3,195 · **Forks:** 143 · **Open issues: 0.**

**Three most-upvoted open issues:** there are none — the tracker is empty, which after nineteen years is a maintenance signal, not a quality gap. The recent closed issues are the useful evidence:

- [#126 Subject condition is violated due to reverse traversal (unexpected rows in subset)](https://github.com/Wisser/Jailer/issues/126) — opened 2025-08-05, fixed 2026-04-14: **over eight months open**. Reverse traversal pulled in rows the subject's WHERE clause excluded. This is the "child expansion blows up the slice" failure, in the most mature tool in the field, and it took eight months to close.
- [#132 H2 generates incorrect DDL](https://github.com/Wisser/Jailer/issues/132) — 2026-07-16.
- [#131 Add an extra statement after insert blocks](https://github.com/Wisser/Jailer/issues/131) — 2025-10-30.

**The first question in Jailer's own FAQ is the blow-up problem.** [faq.html](https://wisser.github.io/Jailer/faq.html) opens with "Why am I getting so much data back?" and answers: "Each association will be traversed in both directions, unless there is a restriction defined. If, for example, the subject table is `employee`, and the association from `department` table to `employee` table is enabled, you will not only get all departments associated with any subject employee, but also all employees associated with one of these departments." Its recommended remedy is to open the GUI, use "Edit→Disable all associations", then re-enable them one at a time via the Closure view, "check each table of this list from top to bottom", prioritising "the tables with the highest degree". That is a manual graph-pruning session before your first useful export.

Jailer also added an **AI Subsetting Assistant** on 2026-06-25 that turns a natural-language description into a subject table, WHERE condition and association restrictions, plus an AI Query Assistant using Anthropic or OpenAI-compatible APIs. Even the nineteen-year incumbent has concluded that writing the extraction model by hand is the barrier.

---

## condenser

**Repo:** [TonicAI/condenser](https://github.com/TonicAI/condenser)

Tonic.ai's open-source loss leader. It exists to sell Tonic, and the README says so.

- **Language:** Python 3.5+ · **Licence:** MIT.
- **Databases:** PostgreSQL and MySQL.
- **Subsetting:** config-driven. `initial_targets` names starting tables with either a `percent` or a `where`; the tool then walks foreign keys in both directions — "Upstream subsetting happens when a row is imported, and there are rows with foreign keys to that row. The subsetter then **greedily grabs as many rows from the database as it can**" ([README](https://github.com/TonicAI/condenser#config)). `upstream_filters` exists specifically to restrain that greed, and the README calls it "an advanced feature, you probably won't need for your first subsets".
- **Cycle handling:** manual. "The subsetting tool cannot operate on databases with cycles in their foreign key relationships." You must enumerate `dependency_breaks` yourself, and "You'll have to know a bit about your database to use this field effectively." Implicit FKs go in `fk_augmentation`.
- **Write access to source:** **yes, on MySQL.** [`mysql_database_helper.py`](https://github.com/TonicAI/condenser/blob/master/mysql_database_helper.py) runs `DROP DATABASE IF EXISTS tonic_subset_temp_db_*` and creates it on the **source** connection before subsetting; [#25 "access denied on 'tonic_subset_temp_db_...'"](https://github.com/TonicAI/condenser/issues/25) (2021-10-23) is a user hitting exactly this because their source account lacks CREATE/DROP privileges. The Postgres path was not independently re-verified for a matching step.
- **Masking config:** **none.** Condenser does not anonymise anything. The README positions PII removal as something you do "in tandem" with a different tool.
- **Install:** five manual steps — `pip install toposort psycopg2-binary mysql-connector-python`, install `pg_dump`/`psql` (or `mysqldump`/`mysql`) and put them on `$PATH`, clone or zip-download the repo, write `config.json`, run `python direct_subset.py`.
- **Config size:** the shipped [`config.json.example`](https://github.com/TonicAI/condenser/blob/master/config.json.example) is 27 lines and covers only the skeleton; cycles and implicit FKs add more.
- **Scale limit, stated by the vendor:** "Our open-source tool can subset databases up to 10GB, but it will struggle with larger databases. Our premium database subsetter can … subset multi-TB databases with ease. If you're interested find us at hello@tonic.ai."
- **Last commit on the default branch:** 2023-05-18, `14ff4077`. (The repo's `pushed_at` reads 2025-07-28 because of activity on a side branch; the default branch has not moved in over three years.)
- **Stars:** 337 · **Forks:** 53 · **Open issues:** 12.

**Three most-upvoted open issues**

1. [#21 Bundling as package?](https://github.com/TonicAI/condenser/issues/21) — 2 reactions (+2), open since 2021-05-13. Five years of "please make this installable".
2. [#30 Writing subset to SQL file instead of to destination DB](https://github.com/TonicAI/condenser/issues/30) — 2 reactions (+2), open since 2022-07-07.
3. [#14 PostgreSQL sequences need to be reset after subsetter is completed](https://github.com/TonicAI/condenser/issues/14) — 1 reaction (+1), open since 2019-10-09. "after a subset has been generated, I need to run some SQL to reset DB sequences to their max value in the resulting table before I can generate a backup via pg_dump. Otherwise, the sequences are all reset to 1."

That last one is seven years old and is a **correctness bug in the produced snapshot**: the target looks fine until the first `INSERT` collides with an existing primary key.

---

## db-condenser (fork of condenser)

**Repo:** [tkhuu01/db-condenser](https://github.com/tkhuu01/db-condenser) — "Modernized fork of Tonic's Condenser database subsetting tool"

- **Language:** Python · **Licence:** MIT · **Created:** 2025-11-02 · **Last commit:** 2026-09-02 · **Stars:** 0 · **Open issues:** 3.
- **Write access to source:** inherits condenser's MySQL temp-database behaviour (same `mysql_database_helper.py` mechanism) unless it has been patched out — **not independently re-verified for this fork**.
- Evidence that condenser's abandonment is being felt, but with zero stars and one contributor it is not yet a competitor. Inherits condenser's design, including the absence of masking. Given zero stars and no adoption signal, it does not get its own row in the time-to-first-snapshot table below — its install path and config burden are the same as condenser's, since it has not yet diverged from the parent's design.

---

## Replibyte

**Repo:** [Qovery/Replibyte](https://github.com/Qovery/Replibyte)

4,409 stars and it does not compile.

- **Language:** Rust · **Databases:** PostgreSQL, MySQL, MongoDB.
- **Licence:** **GPL-3.0** per the [LICENSE file](https://github.com/Qovery/Replibyte/blob/main/LICENSE) and the GitHub API — note the README badge still claims MIT. A licence contradiction in the repo itself.
- **Subsetting:** a `database_subset` block naming a `table` and a `strategy_name` (`random`) with `strategy_options: percent: 50`, plus `passthrough_tables`. The most-requested feature is precisely the thing it cannot do: a WHERE-clause strategy.
- **Masking config:** `transformers` per database/table/column, e.g. `transformer_name: redacted` with `transformer_options`. Custom transformers via WebAssembly. No PII detection — the README lists "Auto-detect sensitive fields" as a planned, unchecked feature.
- **Install:** documented on `replibyte.com`; in practice `cargo install --git`. Requires an object-store "datastore" (S3, GCS, or local disk) between dump and restore. The shipped [`with-subset-and-transformer.yaml`](https://github.com/Qovery/Replibyte/blob/main/examples/with-subset-and-transformer.yaml) is 30 lines including S3 credentials.
- **Last commit on `main`:** 2024-05-04, `5504db91`. Latest release **`v0.10.0`, 2022-10-14** — nearly four years ago.
- **Stars:** 4,409 · **Forks:** 138 · **Open issues:** 118.

**Three most-upvoted open issues**

1. [#74 New subset strategy: create SELECT with WHERE clause strategy](https://github.com/Qovery/Replibyte/issues/74) — 18 reactions (+9), open since 2022-04-30. Random-percent subsetting is not what anyone wants.
2. [#105 Support for Microsoft SQL](https://github.com/Qovery/Replibyte/issues/105) — 12 reactions (+12), open since 2022-05-14.
3. [#36 Support COPY query for PostgreSQL](https://github.com/Qovery/Replibyte/issues/36) — 7 reactions (+7), open since 2022-03-31.

The decay is documented in public. [#277 "Maintained?"](https://github.com/Qovery/Replibyte/issues/277), open since 2023-07-21: the maintainer replied "I will have the opportunity to work again on Replibyte… I have no ETA yet", then again in October 2023 "I'm allocating some time to improve Replibyte". Users kept asking through 2025 — one wrote "Currently can't get `subset` working at all, so a little discouraged but I love the approach." It now fails at the first step: [#310 "Cannot build from source"](https://github.com/Qovery/Replibyte/issues/310) (2025-09-25) and [#307 "Build fails with Rust 1.86.0"](https://github.com/Qovery/Replibyte/issues/307) (2025-04-07), alongside [#308 "Please make a new release"](https://github.com/Qovery/Replibyte/issues/308).

**The lesson.** 4,409 stars, a [129-point Show HN](https://news.ycombinator.com/item?id=31165538) with 78 comments, a [222-point front-page run](https://news.ycombinator.com/item?id=32047535) three months later, a ROSS Index award — and a tool that a new user in 2026 cannot install. Stars measure the pitch, not the product.

---

## pgsubset

**Repo:** [MaieuticalLabs/pgsubset](https://github.com/MaieuticalLabs/pgsubset)

- **Language:** Rust · **Licence:** MIT · **Stars:** 2.
- **Created and last committed 2022-10-13** — the entire history is one day. Version 0.1.0.
- Exports "all the objects needed for a target table to maintain referential integrity" via `COPY` to CSV, with an `import` mode for the other side. `pgsubset --config <CONFIG> --mode <export|import>`. **Cycle handling and write access to source are both unverified** — the entire history is one day of commits and the README does not address either; not worth reverse-engineering the source for a one-day weekend prototype.
- Install is `rustup` then build from source. No masking, no PII detection, no tests, no releases.

Included because the task named it. It is not a competitor; it is a weekend prototype. If someone means a *maintained* tool by that name they probably mean [pg_subsetter](#pg_subsetter-teamniteo) below.

---

## pg_subsetter (teamniteo)

**Repo:** [teamniteo/pg_subsetter](https://github.com/teamniteo/pg_subsetter)

- **Language:** Go · **Licence:** MIT · **Stars:** 4 · **Open issues:** 0 · **Last commit:** 2025-07-14.
- **Databases:** PostgreSQL → PostgreSQL only, on the fly, no intermediate file.
- **Subsetting:** a global fraction, not a root table. `-f 0.05` copies 5% of rows; `-include "user: id=1"` forces specific rows in; `-exclude "domains: all"` drops tables. It "ensures that all foreign keys (one-to-one, one-to-many, many-to-many) are handled correctly during the synchronization process". Uses PostgreSQL's native `COPY`. Cycle handling is not documented — **unverified**.
- **Write access to source:** not documented — **unverified**.
- **Masking:** none.
- **Install:** one-line tarball fetch from GitHub releases. **But it explicitly does not copy the schema** — the documented workflow requires you to `pg_dump --schema-only` and `psql` it into the target first, so the real install path is three commands and a schema round-trip. This is the one tool in the field that is explicit about the target-schema-must-pre-exist requirement; every other entry in this document is silent on whether it creates the target schema for you, which materially changes the real install-path length and belongs in a future revision's config-lines accounting.
- Zero config lines, which is the one thing it gets right, and no masking, which is the one thing that makes it unusable for our user.

---

## pg_sample

**Repo:** [mla/pg_sample](https://github.com/mla/pg_sample)

The sixteen-year-old baseline. Every "just use X" answer eventually lands here.

- **Language:** Perl · **Licence:** the Artistic License, declared only in the script's POD (`This code is released under the Artistic License.` at [`pg_sample` L173–175](https://github.com/mla/pg_sample/blob/master/pg_sample#L173)). **There is no `LICENSE` file**, so GitHub reports no licence.
- **Databases:** PostgreSQL 8.1 or later.
- **Subsetting:** `--limit` (default 100 rows per table), `--random`, `--ordered`, `--schema`, `--data-only`. Options deliberately mirror `pg_dump`. Per the README the sample "includes all tables from the original, maintains referential integrity, and supports circular dependencies". The traversal direction and the cycle mechanism are not described in the README, but the source is short enough to check: [`pg_sample` L488](https://github.com/mla/pg_sample/blob/master/pg_sample#L488) is the `CREATE SCHEMA` that [#31](https://github.com/mla/pg_sample/issues/31) complains about, and the surrounding code builds its working set inside that on-source schema rather than an external database — so "supports circular dependencies" and "needs write access" are the same design decision, not two independent facts.
- **Write access to source:** **yes.** `CREATE SCHEMA $opt{sample_schema}` (default `_pg_sample`) runs directly against the source connection ([`pg_sample` L626](https://github.com/mla/pg_sample/blob/master/pg_sample#L626), dropped again at L592/L954). [#31 "Can't run on a read-only DB"](https://github.com/mla/pg_sample/issues/31), open since 2021-10-14, is a user asking exactly why: "I'm attempting to run this on a read-only replica of a database and I noticed that this is failing... Would you be able to give an explanation as to why I would need write / creating access to complete this task." Five years open, no fix, no workaround offered by the maintainer.
- **Masking:** none. No mention of anonymisation anywhere in the documentation.
- **Install:** clone the repo, `apt install perl libdbi-perl libdbd-pg-perl`, run `./pg_sample`. A Dockerfile is provided. Zero config lines: `pg_sample mydb | psql -v ON_ERROR_STOP=1 sampledb`.
- **Last commit:** 2025-03-30, `9fd58807` "simplify".
- **Stars:** 355 · **Forks:** 51 · **Open issues:** 19.

**Three most-upvoted open issues**

1. [#19 Add installation instructions](https://github.com/mla/pg_sample/issues/19) — 7 reactions (+7), 5 comments, open since **2020-08-04**. The body is two words: "How do I install this?" Six years open, and it is the single most-upvoted issue on the tool.
2. [#44 Is there a way to export a sample by a table row?](https://github.com/mla/pg_sample/issues/44) — 1 reaction (+1), open since 2023-02-12. That is a request for root-table subsetting.
3. [#13 Using --schema flag for having dump schema level granularity like in pg_dump](https://github.com/mla/pg_sample/issues/13) — 0 reactions, open since 2020-01-24.

**The lesson.** The most-wanted feature of the incumbent baseline is *installation*, and the second is *"start from a row I care about"*. Those are lazysnap's first two design decisions.

---

## Tonic Structural

**Product:** [tonic.ai/products/tonic-structural](https://www.tonic.ai/products/tonic-structural) · **Docs:** [docs.tonic.ai](https://docs.tonic.ai/app/generation/subsetting)

- **Language:** closed source, delivered as a hosted web application or a self-hosted deployment.
- **Databases:** broad. PostgreSQL 10–16, MySQL and MariaDB, Oracle 12c+, SQL Server and Azure SQL, Db2 for LUW, MongoDB, Amazon Redshift, Databricks, BigQuery, Spark on EMR, plus CSV/TSV/JSON/XML/Parquet/Avro files ([data connector summary](https://docs.tonic.ai/app/setting-up-your-database/data-connector-summary)).
- **Subsetting:** target tables plus a percentage or WHERE clause, then bidirectional traversal — **upstream** pulls the rows that reference the targets, **downstream** pulls the rows the targets reference ([About subsetting](https://docs.tonic.ai/app/generation/subsetting/subsetting-about)). Missing constraints are supplied by a "virtual foreign key" tool. The docs are unusually candid about the blow-up: 5% of one target table produced "approximately 36% of total row count across all related tables".
- **Cycle handling:** "To break a circular dependency, Structural identifies a foreign key column that is NULLable, and sets its values to NULL. When the process reaches a NULL value, it stops looking for additional related records." ([Foreign keys and circular dependencies](https://docs.tonic.ai/app/generation/subsetting/subsetting-foreign-keys) — corrected citation; this quote is not on the "About subsetting" page it was previously attributed to). The same deferral trick Jailer uses, done automatically.
- **Write access to source:** **unverified** — closed source, and the docs describe reading via connectors, not a write step, but this was not independently confirmed.
- **Masking config:** in the web workspace, per column, with sensitivity scanning and a large generator library. Automation is via an API; **a first-class CLI is unverified** — Tonic's own docs describe API-driven automation, not a CLI binary.
- **Install:** sign up for a [14-day Structural Cloud trial](https://docs.tonic.ai/app/quick-start-guide); self-hosted deployment is "only available to customers who purchased Structural or are undergoing a formal evaluation" ([on-premise deployment](https://docs.tonic.ai/app/admin/on-premise-deployment)).
- **Licence and price:** proprietary. [Vendr's pricing page for Tonic.ai](https://www.vendr.com/marketplace/tonicai) reports a **median annual contract value of $45,525** across 43 recorded purchases (buyers save 16% on average versus list); by deployment size it gives **$18,000–$32,000/year for 1–3 data sources**, $45,000–$85,000 for 4–8, and $95,000–$220,000 for 10+; the overall observed range across the dataset is **$24,000 to $184,560**. (An earlier draft of this document understated this as "$15,000–$30,000/year for one or two data sources," below the source's own floor and roughly a third of the reported median — corrected here.) Treat all of it as indicative, not a published list price.

There is no public issue tracker, so there is no upvoted-issue list. Tonic's open-source [condenser](#condenser) is the visible proxy, and its README explicitly routes anyone with more than 10GB to sales.

---

## Redgate Data Masker, and Redgate Test Data Manager

**Products:** [Data Masker](https://www.red-gate.com/products/data-masker/) · [Test Data Manager](https://www.red-gate.com/products/test-data-manager/) · **Docs:** [documentation.red-gate.com/testdatamanager](https://documentation.red-gate.com/testdatamanager/command-line-interface-cli/subsetting)

- **Data Masker** proper: SQL Server and Oracle, configured through an "Advanced UI" of masking rule sets ([product page](https://www.red-gate.com/products/data-masker/)). No subsetting. Windows-centric tooling.
- **Test Data Manager** is the successor umbrella and the thing to compare against. It ships two CLIs, `rgsubset` and `rganonymize`, with **Windows and Linux** builds distributed as zips you extract and add to `PATH` ([installing the CLIs](https://documentation.red-gate.com/testdatamanager/getting-started/autopilot-for-tdm-standard-a-beginners-guide/2-installing-the-clis)).
- **Subsetting:** two modes. Give a **desired size** and it uses "statistical sampling to select representative data across all tables"; or give a **starting table** with optional filters and it "follows foreign key relationships from this starting point, collecting all related data from dependent tables". Default with no options: **10% of the source, capped at 1GB** ([Subsetting](https://documentation.red-gate.com/testdatamanager/command-line-interface-cli/subsetting)). Multiple starting tables, static-data tables and manual relationships go in an options file. **Cycle handling is not documented publicly** — the subsetting docs page was searched directly for "cycle" and "circular" and contains neither word — **unverified, confirmed absent rather than merely unchecked**.
- **Write access to source:** **unverified** — closed source, no docs found addressing it.
- **Databases:** SQL Server, Oracle, PostgreSQL and MySQL across the TDM suite ([Test Data Manager](https://www.red-gate.com/products/test-data-manager/)).
- **Auth:** `rgsubset auth`, or `--start-trial` to run without a licence.
- **Licence and price:** proprietary, licensed on a **capacity model priced per TB of production data**, purchased "in increments of 1TB" ([capacity model](https://www.red-gate.com/support/license/capacity-model/), which gives tiers of 1TB, 3TB, 5TB, 10TB, 20TB, 50TB, 100TB but **states no dollar figure at all**). The commonly repeated **$9,600 per TB per year** figure could not be confirmed on any page that actually renders it: the official pricing URL now 301-redirects to the unrelated SQL Provision product page (checked via `curl -L`, 2026-09-05, zero occurrences of "9,600" or "per TB" in the rendered text), ComponentSource's pricing page returned no accessible content, and the AWS Marketplace listing for Test Data Manager (available since [October 2024](https://www.red-gate.com/our-company/newsroom/press-releases/redgate-launches-presence-on-cloud-marketplace-portfolio-offerings-now-available-to-buy-via-aws-marketplace/)) does not surface pricing to an unauthenticated fetch either. **Treat $9,600/TB/year as unverified** — it is widely repeated in secondary pricing-research summaries but no primary source checked for this document renders it.

No public issue tracker; no upvoted-issue list available.

**The shape of the incumbent.** Redgate is the honest picture of what "enterprise-grade" costs: two separate binaries, an options file, an auth step, and a price that scales with the size of the database you are trying to *shrink*.

---

## fixturize (2026)

**Repo:** [boringSQL/fixturize](https://github.com/boringSQL/fixturize)

- **Language:** Go · **Licence:** BSD-2-Clause · **Created:** 2026-02-05 · **Last commit:** 2026-05-23 · **Stars:** 7 · **Open issues:** 4.
- **Databases:** PostgreSQL.
- **Subsetting:** `fixturize extract --root "organizations WHERE id = 42"`. Per its [README](https://github.com/boringSQL/fixturize/blob/master/README.md), it "follows foreign keys **in both directions** (parents and children) to collect a referentially-intact subset. Supports composite FKs, self-referencing tables, and circular dependencies." `--limit 500` caps child tables, `--filter "orders=status='completed'"` constrains a specific child, `--include` pulls whole lookup tables with no FK path, `--exclude` drops tables (with a warning if the excluded table is an FK parent).
- **Load:** `fixturize apply` inserts "in FK-dependency order, constraints are deferred".
- **Write access to source:** not documented — **unverified**.
- **PII detection:** `fixturize analyze` scans column names and types across "emails, names, phones, addresses, financial data, API keys" and **outputs ready-to-use `--mask` expressions** — the closest thing in the open-source field to explaining its classification and handing you the fix.
- **Install:** `go build -o ~/bin/fixturize ./cmd/fixturize`. **No binary releases, no package manager**, so the install path requires a Go toolchain.
- **Config lines:** zero. Everything is flags.

**Three most-upvoted open issues** — all four open issues have 0 reactions; by age:

1. [#1 Support limit-per-parent](https://github.com/boringSQL/fixturize/issues/1) — 2026-02-22.
2. [#2 JSONB masking support](https://github.com/boringSQL/fixturize/issues/2) — 2026-02-22.
3. [#4 Deterministic fixture output](https://github.com/boringSQL/fixturize/issues/4) — 2026-05-09.

Note what #4 implies: fixturize's masking is **not** deterministic yet, so joins across separately-masked runs will not line up. That is a first-class requirement for us, not a backlog item.

---

## pgEdge Anonymizer (2025)

**Repo:** [pgEdge/pgedge-anonymizer](https://github.com/pgEdge/pgedge-anonymizer)

- **Language:** Go · **Licence:** The PostgreSQL License · **Created:** 2025-11-27 · **Last commit:** 2026-08-24 · **Stars:** 30 · **Open issues:** 0.
- **Databases:** PostgreSQL.
- **Subsetting:** **none.** It anonymises a database you have already copied, in place, in a single transaction.
- **Masking config:** `pgedge-anonymizer.yaml` with a `database` block and a `columns` list naming fully-qualified `schema.table.column` and a pattern. 100+ built-in patterns across 19 countries; "consistent replacement — same input produces same output within a run"; "foreign key awareness — automatically handles CASCADE relationships"; format preservation; server-side cursors for large tables ([README](https://github.com/pgEdge/pgedge-anonymizer#features)).
- **Write access to source:** n/a by design — it masks a database you have already copied, in place, so the target of its write access is the copy, not the source.
- **PII detection:** none — you list every column yourself.
- Announced twice on HN in December 2025 ([#46290635](https://news.ycombinator.com/item?id=46290635), [#46376812](https://news.ycombinator.com/item?id=46376812)), 1 and 3 points, no comments.

Useful as evidence that a credible vendor (pgEdge) entered this space in late 2025 and chose to solve only the masking half, in-place, with a hand-written column list.

---

## Brume (2026)

**Repo:** [brumeorg/Brume](https://github.com/brumeorg/Brume)

- **Language:** Java · **Licence:** Apache-2.0 · **Created:** 2026-05-04 · **Last commit:** 2026-06-23 · **Stars:** 7 · **Open issues:** 0.
- **Databases:** PostgreSQL 14–18, source and target.
- **Subsetting:** "Automatic FK parent and child traversal — if you extract `orders`, Brume automatically traverses the referenced `users`, up to `fk_depth` levels." Cycle handling is not documented — **unverified**.
- **Write access to source:** not documented — **unverified**.
- **Masking config:** `brume.yml`, per table and per column, with strategies `FAKE`, `HASH`, `MASK`, `NULLIFY`, `FPE_ID`, `FPE_UUID`, `KEEP`. `linked_columns` keeps the same real value mapping to the same fake value across tables with no FK between them. JSONB paths (`$.field.subfield`) are handled individually.
- **Determinism:** the README states this plainly, and it is the one competitor in this entire document that already ships cross-run determinism as advertised: "Strong deterministic pseudonymization — same input + same secret → same output. Two runs produce identical results, ensuring consistency across test environments." Combined with `linked_columns` for cross-table consistency with no FK path, this is stronger than what section 7 below credits it with.
- **Distinctive:** format-preserving encryption of IDs so foreign keys stay valid without disabling constraints, and a `brume audit --anonymity` subcommand producing a k-anonymity report for a DPO.
- **Legal framing worth borrowing:** the README states outright that Brume produces "a **pseudonymization** within the meaning of Art. 4.5, **not anonymization** within the meaning of recital 26. The target dataset remains **personal data** under the GDPR."
- **Install:** apt **and** yum/dnf repositories via Cloudsmith — the README documents both `curl -1sLf 'https://dl.cloudsmith.io/public/brume/brume/setup.deb.sh' | sudo -E bash` and an RPM equivalent (`setup.rpm.sh`), so "Debian/Ubuntu only" understates it. The real gap is platform, not package format: **no macOS path** is documented at all, which matters more for our target user than the Linux package manager they'd use.

---

## pg_partialcopy (2025)

**Repo:** [jackc/pg_partialcopy](https://github.com/jackc/pg_partialcopy) — by the author of `pgx`.

- **Language:** Go · **Licence:** MIT · **Created:** 2025-03-29 · **Last commit:** 2026-03-14 · **Stars:** 11 · **Open issues:** 0.
- **Databases:** PostgreSQL.
- **Subsetting:** there is **no FK graph walk**. `pg_partialcopy -init` inspects the source and writes a TOML file listing every table with an explicit `select_sql`, and you edit the SQL yourself — typically with a `before_transaction_sql` that populates a temp table via `tablesample bernoulli(10)` and then joins against it. Referential completeness is your problem.
- **Masking:** whatever you write into `select_sql`. No library, no detection.
- **Write access to source:** not documented — **unverified**.
- **Design idea worth stealing:** the `select_sql` per table names every column explicitly, on purpose — "This ensures that if a column is added to a source database, it is not automatically included. This is important to ensure sensitive data is not inadvertently copied." That is fail-closed on schema drift, and no other tool here does it.
- **Config size:** the generated TOML lists every table with every column. For a 41-table schema that is hundreds of lines before you have made a single decision.

---

## slice-db

**Repo:** [rivethealth/slice-db](https://github.com/rivethealth/slice-db)

- **Language:** Python · **Licence:** MIT · **Created:** 2021-03-09 · **Last commit:** 2026-02-05 · **Stars:** 6 · **Open issues:** 1.
- **Databases:** PostgreSQL.
- **Subsetting:** root table, then "queries the physical IDs of rows, adds them to existing lists, and for new IDs, processes each adjacent table" in parallel under consistent snapshots.
- **Cycle handling:** the strictest rule in the field — "Foreign keys may form a cycle only if at least one foreign key in the cycle is deferrable", which is then deferred during restore. If your schema has a non-deferrable cycle, slice-db refuses.
- **Write access to source:** not mentioned in the README — **unverified**; the "consistent snapshots" language points at read-only transactions, but this was not confirmed against the source code.
- **Masking:** scrubbing with deterministic replacements "for a given pepper, with the pepper randomly generated each run by default" — deterministic within a run, non-reproducible across runs unless you pin the pepper.
- **Install:** `pip3 install slice-db` or the `rivethealth/slicedb` Docker image.
- **Three most-upvoted open issues:** there is exactly 1 open issue on the tracker, so there is no top-three to rank — noted explicitly rather than left as a silent gap.
- **Usage signal beyond stars:** despite 6 stars and a last commit seven months old, [PyPI download stats](https://pypistats.org/packages/slice-db) show 13,633 downloads in the 30 days to 2026-09-05 — two orders of magnitude more than dbslice's 33/month despite dbslice having 24× the stars. Some of this may be CI mirrors or transitive dependents rather than direct human adoption; it was not decomposed further, but it is a reminder that GitHub stars and package-registry downloads can disagree sharply for tools in this category.

dbslice's own comparison table calls slice-db "Unmaintained"; the last commit is February 2026, which is stale but not abandoned.

---

## msg555/subsetter

**Repo:** [msg555/subsetter](https://github.com/msg555/subsetter)

- **Language:** Python · **Licence:** BSD-3-Clause · **Created:** 2023-08-07 · **Last commit:** 2026-08-24 · **Stars:** 10 · **Open issues:** 0.
- **Databases:** MySQL, PostgreSQL, SQLite.
- It exists explicitly as a reaction to condenser: "This is meant to be a simple CLI tool that overcomes many of the difficulties in using `condenser`" ([README](https://github.com/msg555/subsetter#subsetter)).
- **Subsetting:** two phases. `subsetter -c my-config.yaml plan > plan.yaml` emits an inspectable, hand-editable syntax tree ("If needed, you can even write direct SQL here"), then `subsetter ... sample` executes it. Splitting *plan* from *execute* is a genuinely good idea.
- **Cycle handling:** it does not have any. "The subsetter tool takes an approach of 'one table, one query' … It cannot support calculating a full transitive closure of foreign key relationships for schemas that contain cycles." It also requires that no target is reachable from another target.
- **Write access to source:** not documented — **unverified**.
- **Masking:** built-in filters to remove or anonymise columns, plus custom plugins.
- **Install:** `pip install subsetter` or `docker run msg555/subsetter`. Requires `--create` to build tables and `--truncate` to clear them, neither of which is the default.
- **Three most-upvoted open issues:** none — the tracker is empty.

---

## DBSnapper

**Site:** [dbsnapper.com](https://dbsnapper.com/) · **Docs:** [docs.dbsnapper.com](https://docs.dbsnapper.com/subset/introduction/)

- **Language:** closed source agent. **Licence:** the agent itself is proprietary, but the tooling around it is not uniformly closed — a 30-second check of the public [github.com/dbsnapper](https://github.com/dbsnapper) org resolves what "the pricing page" left unverified: `install-dbsnapper-agent-action` is MIT-licensed and `terraform-provider-dbsnapper` is MPL-2.0. The core `dbsnapper` and `vscode-dbsnapper` repos carry no licence file (closed).
- **Databases:** PostgreSQL and MySQL, local and in Docker.
- **Subsetting:** five explicit phases — analyse the FK graph; process subset tables using `percent` or `where`; process **upstream** tables (parents of copied rows); transfer whole `copy` tables; process **downstream** tables ([Database Subsetting](https://docs.dbsnapper.com/subset/introduction/)).
- **Cycle handling:** it stops. "Circular references … including self-referencing tables" cause subsetting to halt with an error; you then list the offending relationship under `excluded_relationships`, which nulls the FK values. Missing constraints go in `added_relationships`.
- **Write access to source:** closed source — **unverified**.
- **Masking:** via SQL you supply — sanitisation is a script, not a detection-plus-strategy library.
- **Price:** Starter free; Pro $300/month or $3,600/year (up to 100 users); Enterprise $500/month or $6,000/year ([pricing](https://dbsnapper.com/pricing)). Subsetting is behind Pro — "advanced subsetting" is a Pro-tier feature.
- **Container handling — the closest prior art to a differentiator we claim.** DBSnapper documents `pgdocker://` and `mydocker://` URL schemes ([Docker Integration](https://docs.dbsnapper.com/latest/database-engines/docker-integration/)) for running its tooling against a database inside a named Docker container, e.g. `pgdocker://user:pass@remote-db:5432/production`. This is naming a specific container in a connection string, not scanning the host for running Postgres containers and proposing a source/target pair unprompted — the "container discovery on first run" claim later in this document should be read as "goes further than DBSnapper does today," not "nobody has touched containers," because DBSnapper has.
- **CI story — the one documented example in this field.** DBSnapper ships a public GitHub Action (`dbsnapper/install-dbsnapper-agent-action@v1`) and two docs articles walking through GitHub Actions + Amazon ECS integration ([part 1](https://docs.dbsnapper.com/articles/dbsnapper-github-actions-amazon-ecs/), [part 2](https://docs.dbsnapper.com/latest/articles/dbsnapper-github-actions-ecs-simplified/)). No image size, cold-start time or unattended-run cost is published in either article — the CI *path* is documented, the CI *cost* is not.
- Integrates with VS Code, Terraform, GitHub Actions and Okta. Announced subsetting in [DBSnapper 2.0, Feb 2024](https://dbsnapper.com/blog/introducing-dbsnapper-v2) ([HN, 2 points, 3 comments](https://news.ycombinator.com/item?id=39525736)); its original [Show HN in Dec 2022](https://news.ycombinator.com/item?id=33968058) drew 3 points.

No public issue tracker; no upvoted-issue list available.

---

## Adjacent, not competitors

Tools that come up in the same searches but solve a different problem. Listed so we do not accidentally position against them.

- **Seedfast** — [seedfa.st](https://seedfa.st/). A CLI and MCP server that reads a live PostgreSQL schema and **generates synthetic relational data** from a plain-English scope, re-reading the schema on every `seedfast_run`. Explicitly Postgres-only. It is synthetic generation from a schema, which is a stated [non-goal for our v1](../CONCEPT.md). Closed source; pricing and licence **unverified**.
- **VeilStream** — [veilstream.com](https://www.veilstream.com/). A PostgreSQL **proxy** that masks on the wire: "looks like a normal PostgreSQL server to clients" while sensitive fields are removed or replaced as queries run. Deployed as a Docker container beside your database ([Docker Hub](https://hub.docker.com/r/veilstream/veilstream-postgres-proxy)). Show HN [June 2025](https://news.ycombinator.com/item?id=44310026), 22 points, 12 comments; a second [Feb 2026](https://news.ycombinator.com/item?id=46874014) for per-branch preview environments. No subsetting — the copy is the same size as production.
- **Xata** — [xataio/xata](https://github.com/xataio/xata). Apache-2.0, Go, 1,057 stars, last commit 2026-09-04. Solves the same *user problem* from the storage layer: copy-on-write Postgres branches with [PII anonymization](https://xata.io/postgres-data-masking) instead of a subset-and-copy pipeline. If a team can run Xata, they do not need a subsetter. Its highest-profile HN post reached [45 points](https://news.ycombinator.com/item?id=44016289).
- **Ardent (YC P26)** — [tryardent.com](https://www.tryardent.com/). "Postgres sandboxes in seconds with zero migration", [Launch HN May 2026](https://news.ycombinator.com/item?id=48124436), 99 points, 52 comments — the largest recent attention burst in the neighbourhood. Sandboxes, not subsets. Details **unverified** beyond the launch post.
- **django-postgres-anonymizer** — [CuriousLearner/django-postgres-anonymizer](https://github.com/CuriousLearner/django-postgres-anonymizer), 29 stars, a Django wrapper over the Dalibo extension. Framework glue, not a snapshot tool.
- **simple-anonymizer** — [io-github-nafg/simple-anonymizer](https://github.com/io-github-nafg/simple-anonymizer). A Scala library, not a CLI — you write a `DbCopier` and `TableSpec` in a JVM project and call it as code, so it competes with our library internals more than our product. Created 2026-01-28, 7 stars, 17 open issues, no `LICENSE` file, last pushed 2026-09-05 (active today). Its feature list is squarely our territory even so: "Deterministic anonymization — Same input always produces same output (using MD5 hash-based selection)", "FK-aware ordering", and "Filter propagation — WHERE clauses on parent tables automatically propagate to child tables via FK subqueries" — that last one is root-table subsetting with cascading filters, done in a library API rather than a CLI. Excluded from the main table because there is no command-line entry point for a non-Scala user to run.

---

## recordrelay (2026)

**Repo:** [MstroCA/recordrelay](https://github.com/MstroCA/recordrelay)

Found late in this research and worth a full entry rather than a one-line mention: its pitch is close to a paraphrase of ours. The README's Vision section reads "RecordRelay is not an ETL tool. It is a **business context reproduction** platform for developers and SREs: it reproduces a real entity — a customer, order, or user — along with all its relationships, from production to a local environment in minutes."

- **Language:** Java 21 · **Licence:** repository metadata reports `Other`/`NOASSERTION` despite a README badge claiming Apache-2.0 — a licence file exists but GitHub's detector did not resolve it to a known SPDX identifier; **treat the licence as unconfirmed** until someone reads `LICENSE` directly. · **Created:** 2026-06-23 · **Last commit:** 2026-07-06 · **Stars:** 3 · **Open issues:** 15.
- **Databases:** the CLI's `conn add --type POSTGRESQL` suggests Postgres-first, with an explicit `--schema` flag for non-default `search_path`; other engines are not evidenced in the README beyond that.
- **Subsetting:** root-entity driven — `rr clone --entity customer --id 12345 --from prod --target local --depth 3` — not a table-and-WHERE model but an entity-and-ID one, which is closer to Jailer's "Subset by Example" than to Greenmask/Tonic's config-file approach. **Satellite (companion) tables** reach into a *separate* database linked by a single logical column (its README's example: an event-sourced Kafka read-model table) and copy matching rows with the link column remapped — no other tool in this document crosses a database boundary like this.
- **Cycle handling:** not documented in the parts of the README read for this entry — **unverified**.
- **Write access to source:** not documented — **unverified**.
- **Masking:** "field overrides" and named **identity strategies** — `REGENERATE_IDENTITIES` (default, allocates new IDs above both source and target max), `ISOLATE_NAMESPACE`, `SKIP_EXISTING`, `FAIL_SAFE`, `SEQUENCE`, `START_AT` — this is a more developed conflict-resolution model than anything else in this document, though it is about ID collision on re-import rather than PII masking specifically; whether it detects or classifies personal data at all is **unverified**.
- **Install and surface area:** three deployment targets — a JavaFX desktop app (14 screens: cloning, discovery, ERD graph view, schema-drift diff, masking coverage, presets, scheduled sync), a CLI, and an IntelliJ plugin — plus a headless `rr serve` REST API for CI/CD. This is a much larger surface than our zero-config, terminal-first bet, and is itself a signal: a solo or small team building a desktop app, a CLI, *and* an IDE plugin at once is optimizing for reach over focus.
- **CI:** `rr serve --port 8080` exposes `/health`, `/clone`, `/preset/run/{name}`, `/sync/run/{name}` for pipeline use — one of the few tools in this document with a documented headless-server mode built specifically for CI, distinct from "the CLI also happens to be scriptable."

Too new (three months old, 3 stars, 15 open issues) to be a competitive threat today, but its scope and messaging are close enough to ours that it is worth re-checking in a future revision of this document.

---

## Time from install to first usable snapshot

**These are estimates from reading each quickstart end to end, not measured runs.** "Config lines" counts the YAML/JSON/TOML/SQL a user must author or edit before the first successful command, using the project's own shipped example as the floor. "Usable" means: a subset in a target database, with personal data masked. Tools that cannot mask are marked accordingly — they never reach "usable" by our definition.

| Tool | Install | Config lines before first run | Est. time to first snapshot | Blocking friction |
|---|---|---|---|---|
| [dbslice](#dbslice-2026) | `uv tool install dbslice` | **0** | **~5 min** | none; but `--depth 3` default silently truncates the graph |
| [pg_sample](#pg_sample) | clone + Perl DBI/DBD::Pg | **0** | ~10 min (~30 if Perl deps fight you) | **no masking — never usable by our definition**; [#19](https://github.com/mla/pg_sample/issues/19) is "how do I install this" |
| [pg_subsetter](#pg_subsetter-teamniteo) | tarball from releases | **0** | ~10 min | must `pg_dump --schema-only` into the target first; **no masking** |
| [fixturize](#fixturize-2026) | `go build` from source | **0** | ~10 min with Go installed, ~30 without | no binary releases; masking not deterministic ([#4](https://github.com/boringSQL/fixturize/issues/4)) |
| [Basecut](#basecut) | `brew install basecuthq/cli/basecut` | 0 authored (`init` generates ~18) | **~5 min, vendor-claimed** ([quickstart](https://docs.basecut.dev/getting-started/quick-start)) | requires an account and a browser `basecut login` before anything runs |
| [slice-db](#slice-db) | `pip3 install slice-db` | ~10 | ~20 min | refuses non-deferrable FK cycles |
| [msg555/subsetter](#msg555subsetter) | `pip install subsetter` | ~25 (`planner` section) | ~30 min | two-phase `plan` then `sample`; **cannot handle cycles at all** |
| [pgEdge Anonymizer](#pgedge-anonymizer-2025) | binary release | ~4 + 1 per column | ~20 min for a small schema | **no subsetting**; 17 personal columns = 17 hand-written entries |
| [Greenmask](#greenmask) | `brew install greenmask` | **~25–40** ([playground config](https://github.com/GreenmaskIO/greenmask/blob/main/playground/config.yml) is 35 lines and masks one column) | **~45–90 min** | needs matching `pg_bin_path`, a storage backend, and one hand-written transformer per personal column; **no PII detection** |
| [condenser](#condenser) | 5 manual steps, no package | **27+** ([config.json.example](https://github.com/TonicAI/condenser/blob/master/config.json.example)) | ~60 min, more with cycles | you must enumerate `dependency_breaks` yourself; **no masking**; [sequences left at 1](https://github.com/TonicAI/condenser/issues/14) |
| [pg_partialcopy](#pg_partialcopy-2025) | `go install` | **hundreds** (every table, every column) | ~90 min | `-init` generates the whole schema as TOML; no FK walk at all |
| [Brume](#brume-2026) | apt via Cloudsmith | ~20+ (`brume.yml` per table/column) | ~45 min | Debian/Ubuntu packaging only |
| [DBSnapper](#dbsnapper) | binary agent + account | ~10 + relationship overrides | ~30 min | subsetting is behind the $300/mo Pro tier; cycles halt the run |
| [Redgate TDM](#redgate-data-masker-and-redgate-test-data-manager) | download + unzip 2 CLIs + `auth` | options file | ~60 min | two separate CLIs (`rgsubset`, `rganonymize`); licence auth first |
| [Tonic Structural](#tonic-structural) | sign up for cloud trial | web workspace | ~60 min to a first generation job | SaaS onboarding; self-hosted gated behind sales |
| [Jailer](#jailer) | zip (installers omit the CLI) | GUI-built extraction model | **hours** (its own docs suggest building the model in the GUI and copying args out of `export.log`) | **no anonymisation at all** |
| [PostgreSQL Anonymizer](#postgresql-anonymizer) | distro package or Docker | 1 `SECURITY LABEL` per column + extension init | ~30 min | needs an **extension installed on the database**, i.e. superuser; **no subsetting** |
| [Replibyte](#replibyte) | `cargo install --git` | 30 incl. S3 credentials | **∞** | [does not build](https://github.com/Qovery/Replibyte/issues/310) on current Rust |
| [Neosync](#neosync) | `git clone && make compose/up` | 0 (web UI) | n/a | **archived; site returns 404, docs domain does not resolve** |
| [Snaplet Snapshot](#snaplet-snapshot) | `npx @snaplet/snapshot setup` | interactive | n/a | **archived**; top two issues are [install](https://github.com/supabase-community/snapshot/issues/15) [failures](https://github.com/supabase-community/snapshot/issues/20) |
| [pgsubset](#pgsubset) | rustup + build from source | a config file, undocumented | n/a | one day of commits in 2022 |

**Reading of the table.** Two tools plausibly get a developer to a masked, referentially complete subset in under ten minutes with zero authored config: **dbslice** and **Basecut**. Both launched in 2026. Basecut charges and requires an account; dbslice is MIT, Python, alpha-classified, has no `LICENSE` file, and defaults to a depth-3 traversal that is not referentially complete. Everything older either demands 25–40+ lines of YAML before the first run, or does not mask at all.

---

## What nobody does well

### 1. Nobody tells you which columns are personal — except three tools and one extension

Greenmask's configuration reference requires every transformed column to be named explicitly with a transformer; there is no detection pass ([configuration docs](https://docs.greenmask.io/latest/configuration/)). Replibyte lists "Auto-detect sensitive fields" as an unchecked, planned feature in its [README](https://github.com/Qovery/Replibyte#features) — and has not shipped a release since 2022. condenser, Jailer, pg_sample, pg_subsetter, pg_partialcopy and slice-db have no notion of personal data at all. pgEdge Anonymizer, launched November 2025 by a real Postgres vendor, ships 100+ patterns and still makes you write out every `schema.table.column` yourself. **Basecut also detects PII** (its `anonymize: auto` mode is a genuine detection pass, not just a rule library) — it is omitted from this section's list of exceptions only because its closed, account-gated product cannot be inspected the way the other three can, not because it fails to clear the bar; the orientation table at the top of this document and must-match item 11 both already count it among the tools that detect.

The exceptions prove the point about what "good" looks like: [PostgreSQL Anonymizer's `anon.detect()`](https://postgresql-anonymizer.readthedocs.io/en/stable/detection/) returns a category per column against the HIPAA classification *and documents its own false-negative rate*; [fixturize's `analyze`](https://github.com/boringSQL/fixturize/blob/master/README.md) prints ready-to-paste `--mask` expressions; dbslice offers GDPR/HIPAA/PCI-DSS profiles with a manifest. None of the three explains *why* a given column was classified in a way the user can argue with — and that explanation is what turns a guess into something a compliance reviewer will accept.

### 2. Cycles break everything, and the failure is usually silent or user-assigned

The evidence is unusually consistent:

- **condenser** puts the work on the user: "The subsetting tool cannot operate on databases with cycles… You'll have to know a bit about your database to use this field effectively" ([README](https://github.com/TonicAI/condenser#config)).
- **msg555/subsetter** simply cannot: "It cannot support calculating a full transitive closure of foreign key relationships for schemas that contain cycles" ([README](https://github.com/msg555/subsetter#limitations)).
- **slice-db** refuses unless at least one FK in the cycle is deferrable.
- **DBSnapper** halts with an error and asks you to list `excluded_relationships` ([docs](https://docs.dbsnapper.com/subset/introduction/)).
- **Greenmask** handles multi-cycles in one SCC "but only for one group of vertexes" ([docs](https://docs.greenmask.io/latest/database_subset/)), tracked as [#197](https://github.com/GreenmaskIO/greenmask/issues/197) — and its own epic [#392](https://github.com/GreenmaskIO/greenmask/issues/392) admits "there are currently limitations that prevent this system from being used effectively in databases with cyclic references". Worse, [#329](https://github.com/GreenmaskIO/greenmask/issues/329) reports the *opposite* failure: a schema with **no** cycles at all errors out in the subset component, 18 comments deep and open for over a year.
- Only **Jailer** and **Tonic Structural** resolve cycles automatically, both by the same trick — defer or null a nullable FK ([Jailer news 2021-02-04](https://github.com/Wisser/Jailer#news), [Tonic docs — Foreign keys and circular dependencies](https://docs.tonic.ai/app/generation/subsetting/subsetting-foreign-keys)).
- **Redgate TDM, Snaplet Snapshot, Neosync, pgsubset and pg_subsetter leave the question unanswered entirely.** Redgate's public [subsetting docs](https://documentation.red-gate.com/testdatamanager/command-line-interface-cli/subsetting) contain no mention of "cycle" or "circular" anywhere; Neosync's archived README and docs tree were searched for the same terms with no hits; Snaplet Snapshot's archived source was not reverse-engineered beyond its one-sentence README claim; pgsubset and pg_subsetter simply never raise the topic. Five of the twenty-two tools profiled in this document are silent on the single technical question this section is built around.

Cycles are not exotic. `users → organizations → users`, `orders → addresses → orders`, any self-referencing `parent_id` — these are in every real schema, and the field's answer is "tell us where to cut".

### 3. The slice blows up, and the tools tell you afterwards

This is the single most consistent complaint, and the tools document it against themselves:

- **Jailer's FAQ opens with it.** The first question on [faq.html](https://wisser.github.io/Jailer/faq.html) is "Why am I getting so much data back?", answered: "Each association will be traversed in both directions, unless there is a restriction defined… you will not only get all departments associated with any subject employee, but also all employees associated with one of these departments." The prescribed fix is a manual GUI session disabling every association and re-enabling them by hand.
- **Tonic admits the arithmetic.** 5% of a target table produced "approximately 36% of total row count across all related tables" ([About subsetting](https://docs.tonic.ai/app/generation/subsetting/subsetting-about)).
- **condenser names its own behaviour.** Its upstream pass "greedily grabs as many rows from the database as it can", with `upstream_filters` offered as an "advanced feature, you probably won't need for your first subsets" ([README](https://github.com/TonicAI/condenser#config)).
- **Jailer shipped a correctness bug from it.** [#126 "Subject condition is violated due to reverse traversal (unexpected rows in subset)"](https://github.com/Wisser/Jailer/issues/126), opened 2025-08-05 and fixed 2026-04-14 — over eight months open, nineteen years in, the mature tool still leaked rows the subject condition excluded.
- **dbslice's answer is to truncate.** Its `--depth` defaults to 3, which caps the blow-up by silently discarding anything further out — trading referential completeness for size.

Not one tool shows you the estimated slice size *before* it runs. Every one of them lets you discover that "500 customers" meant 40% of production after the extraction has already started.

### 4. Installation is the top-voted feature request in this category

- pg_sample's most-upvoted open issue, by a wide margin, is [#19 "Add installation instructions"](https://github.com/mla/pg_sample/issues/19) — 7 upvotes, open since 2020, body: "How do I install this?"
- Snaplet Snapshot's two most-upvoted open issues are [#15 "npm install failure"](https://github.com/supabase-community/snapshot/issues/15) and [#20 "NPM Install Failing … on ARM Darwin"](https://github.com/supabase-community/snapshot/issues/20) — the tool does not install on an Apple Silicon Mac.
- condenser's [#21 "Bundling as package?"](https://github.com/TonicAI/condenser/issues/21) has been open since 2021.
- Replibyte cannot be built at all: [#310 "Cannot build from source"](https://github.com/Qovery/Replibyte/issues/310) and [#307 "Build fails with Rust 1.86.0"](https://github.com/Qovery/Replibyte/issues/307), alongside [#308 "Please make a new release"](https://github.com/Qovery/Replibyte/issues/308).
- fixturize, the best-designed newcomer, ships **no binaries at all** — `go build` from source is the only documented path.
- Jailer's CLI is only in the zip; the MSI and `.deb` installers omit it ([README installation](https://github.com/Wisser/Jailer#installation)), and you must supply your own JDBC driver jar on top.

### 4a. Stars measure the pitch; downloads are a better proxy for use, and this document went and got some

Section 3's Replibyte lesson ("Stars measure the pitch, not the product") is asserted throughout this document but was, until this revision, never checked against an actual usage number. A few spot-checks, gathered 2026-09-05:

- **dbslice**, the tool this document calls "the single most direct open-source competitor," shows **33 PyPI downloads in the last 30 days** ([pypistats](https://pypistats.org/packages/dbslice)) against 143 GitHub stars.
- **slice-db**, tagged "low activity" and "Unmaintained" by dbslice's own comparison table, shows **13,633 PyPI downloads in the last 30 days** ([pypistats](https://pypistats.org/packages/slice-db)) against 6 GitHub stars — two orders of magnitude more real installs than the tool positioned as its direct, modern replacement.
- **Greenmask**'s official Docker image has **32,108 total pulls** on Docker Hub ([hub.docker.com/r/greenmask/greenmask](https://hub.docker.com/r/greenmask/greenmask/)), a real usage signal independent of its 1,757 stars.
- **`@snaplet/snapshot`**, archived and unmaintained, still logged **29,631 npm downloads** in the 30 days to 2026-08-29 ([npm](https://api.npmjs.org/downloads/point/last-month/@snaplet/snapshot)) — almost certainly legacy CI pipelines nobody has migrated off a dead tool yet.

The pattern that emerges is not "stars are always wrong," it is that **stars and downloads can disagree by orders of magnitude in either direction**, and a document that cites star counts as a liveness or credibility signal without also checking a download number is measuring attention, not adoption. Homebrew analytics (for Greenmask, Basecut) and crates.io downloads (Replibyte is not published under that name on crates.io, so this could not be checked) were attempted but not obtained for this revision — a gap for whoever updates this document next.

### 5. Everyone demands configuration before the first run, and then never writes it back

Greenmask's shipped playground config is 35 lines and masks exactly one column. condenser's skeleton is 27 lines of JSON. Replibyte's subset example is 30 lines including S3 credentials. pg_partialcopy generates a TOML listing every table and every column. Only Basecut's `basecut init` and dbslice's `dbslice init` generate config *from the live schema* — and Basecut requires an account before `init` will run.

Nobody, in either direction, emits a config **after** a successful run as a record of what actually happened. The reproducibility artefact does not exist in this field.

### 6. Nobody verifies the result

Across every tool read for this document, exactly one — Basecut — claims a post-load integrity check, and only in marketing copy on its [homepage](https://basecut.dev/) ("✓ Verified referential integrity"), not in a documented command. dbslice lists "Validation — Checks referential integrity of extracted data" in its feature list, on the extracted data, before load. fixturize defers constraints on apply, which means a violation surfaces as a deferred-constraint error at commit time rather than as a report.

condenser's [#14](https://github.com/TonicAI/condenser/issues/14) is the cautionary tale: sequences silently reset to 1 in the target, open since 2019, and you only find out when the first insert collides. A snapshot tool that does not check its own output is asking the user to discover the bug in their own test suite.

### 7. Determinism is an afterthought

Deterministic masking is what keeps joins working after the values change, and it is not equally unsolved across the field — it is solved unevenly, which is a different and more actionable problem. slice-db randomises its pepper on every run by default, so two runs disagree. fixturize has determinism as an [open issue](https://github.com/boringSQL/fixturize/issues/4). Greenmask has a hash engine but [#338](https://github.com/GreenmaskIO/greenmask/issues/338) shows its default company-name generator drawing from an 888-value space with replacement, producing duplicates that "can violate uniqueness constraints". pgEdge Anonymizer is explicit that consistency holds only "within a run". **Brume is the counter-example this section should not omit:** its README states "Two runs produce identical results, ensuring consistency across test environments," backed by `linked_columns` for cross-table consistency with no FK path between the tables (see the [Brume entry](#brume-2026)). The honest framing is: cross-run determinism is a shipped, documented feature in at least one 2026 competitor, so our bar is to match Brume and then go further — printing *why* a value got the guarantee it did — not to be first to solve determinism at all.

### 8. The category kills companies

Snaplet [shut down on 31 August 2024](https://supabase.com/blog/snaplet-is-now-open-source), a date confirmed via the [Wayback Machine capture](http://web.archive.org/web/20240716025550/https://www.snaplet.dev/post/snaplet-is-shutting-down) of the original announcement since the live post is gone; it open-sourced its tools and the team joined Supabase, and the snapshot repo is archived. **Correction from an earlier draft:** `snaplet.dev` itself has not lapsed — its nameservers, MX records and whois status are all live (checked 2026-09-05) — only the website is gone (no A/AAAA record), so the practical effect for a user is the same: there is nowhere to read the docs. Neosync — 4,141 stars, YC-backed — was [announced as acquired by Grow Therapy on 2025-09-25](https://www.prnewswire.com/news-releases/grow-therapy-raises-the-privacy-bar-in-mental-health-302567153.html) (a 2025-08-01 close date is reported by [Crunchbase](https://www.crunchbase.com/acquisition/grow-therapy-acquires-neosync-cd81--00632527) but that page 403s to an unauthenticated fetch and is **unverified** here), archived its repo on 30 August 2025, and `docs.neosync.dev` no longer resolves either. Replibyte had 4,409 stars and a fastest-growing-OSS award and has not merged a commit to `main` since May 2024.

Both dead products' documentation is now unreachable, which means **anyone still running them has no docs** — not because either domain's registration lapsed (Snaplet's did not; Neosync's was not checked), but because nobody kept the web server or the docs subdomain running once the bills stopped being worth paying. A tool whose entire behaviour is discoverable from `--help` and a generated config file degrades better than one whose manual lived on infrastructure the company stopped maintaining.

This is not a reason not to build. It is a reason to be a tool, not a company: one binary, no control plane, nothing that stops working when someone stops paying a hosting bill.

---

## What we must match to be credible

Everything below is either already table stakes in a shipping competitor or is the direct answer to an issue cited above. These are the floor, not the differentiators.

**Install and first run**

1. **A real install path that is not `go build` or `pip`.** `brew install`, a `curl | sh` script, and signed static binaries for darwin/linux on amd64 and arm64. Greenmask, Basecut, pg_subsetter and pgEdge Anonymizer all clear this bar; fixturize, condenser and pg_sample do not, and it is the most-upvoted complaint in the category ([pg_sample #19](https://github.com/mla/pg_sample/issues/19), [Snapshot #15](https://github.com/supabase-community/snapshot/issues/15), [Snapshot #20](https://github.com/supabase-community/snapshot/issues/20), [condenser #21](https://github.com/TonicAI/condenser/issues/21)).
2. **Zero authored config for the first run.** dbslice and Basecut both do it; Basecut charges an account for the privilege. If we require a YAML file before the first snapshot we are behind two 2026 entrants on day one.
3. **No account, no browser, no network call to us.** This is Basecut's one structural weakness and the clearest wedge: `basecut login` is step 2 of its quickstart.
4. **Works offline against localhost and in CI with the same command.** This claim is weaker on evidence than the others in this list: DBSnapper is the only tool in this document with a *documented* CI story — a published GitHub Action and two step-by-step GitHub-Actions-plus-ECS guides — and even there, neither guide states image size or cold-start cost, and subsetting through that path is gated behind its $300/month Pro tier. msg555/subsetter's `docker run` invocation is scriptable but not documented as a CI recipe. Everything else asserts "works in CI" without a worked example: no tool profiled here, ours included until we build one, has published a CI cost number. Treat "only the ones with no control plane actually deliver it unattended" as our hypothesis to prove, not a finding this document has evidence for yet.

**Subsetting correctness**

5. **Root table plus WHERE, not a global percentage.** [Replibyte #74](https://github.com/Qovery/Replibyte/issues/74) is the most-upvoted open issue in the entire category (+9) and it is exactly this request. [pg_sample #44](https://github.com/mla/pg_sample/issues/44) is the same request against the baseline. Random-percent sampling is the wrong primitive.
6. **Bidirectional traversal: parents to completeness, children with caps.** Greenmask, Tonic, DBSnapper, fixturize and dbslice all do both directions. Doing only one is not competitive. The cap on children is what stops the blow-up documented by [Tonic](https://docs.tonic.ai/app/generation/subsetting/subsetting-about) (5% target → 36% of rows) and [condenser](https://github.com/TonicAI/condenser#config) ("greedily grabs as many rows as it can").
7. **Cycles resolved automatically, with the resolution named in the output.** Jailer and Tonic defer or null a nullable FK; everyone else stops and hands the user a graph problem. Match Jailer, and print *which* edge was deferred and why — nobody does that part.
8. **Composite foreign keys, self-referencing tables, and multi-schema sources.** fixturize claims composite FKs and self-references; [dbslice #1](https://github.com/nabroleonx/dbslice/issues/1) had to add schema selection after launch. These are not edge cases.
9. **Virtual/implicit foreign keys declarable without a full config rewrite.** Greenmask has `virtual_references` with `polymorphic_exprs`; condenser has `fk_augmentation`; dbslice has `virtual_foreign_keys` for Django `GenericForeignKey`; Tonic has a virtual FK tool. Polymorphic associations are common enough that Greenmask has an open bug where they [produce ~0 rows](https://github.com/GreenmaskIO/greenmask/issues/396). **Caveat this one against Jailer's in-flight work:** its 2026-08-26/27 commits shipped a "Discover Associations" dialog for automatic implicit-FK discovery, and its 2026-08-31 to 09-04 "subset insight" commits add row-origin provenance (`RowOriginChain`) — see the investigation in the [Jailer entry](#jailer). Neither is released yet, but both point at exactly this item and item 7's "nobody names the resolution" gap, so the nineteen-year incumbent is closing ground on this list, not standing still.
10. **Show the plan and the estimated size before extracting.** Nobody does this. msg555/subsetter comes closest by splitting `plan` from `sample`, and it is the single best idea in the field.

**Masking**

11. **Detection with a stated reason per column.** `anon.detect()` gives a category; fixturize gives a suggested mask; neither gives a defensible justification. Our `press ? to see why` has no equivalent anywhere and is therefore the differentiator — but detection *at all* is now table stakes, matched by Basecut, dbslice, fixturize and PostgreSQL Anonymizer.
12. **Deterministic masking by default, reproducible across runs.** Same input plus same key produces the same output, so joins on masked natural keys survive. slice-db's random-per-run pepper and [fixturize #4](https://github.com/boringSQL/fixturize/issues/4) show what happens without it — but this is not an unsolved problem in the field, only an unevenly solved one. **Brume already ships it**: "Two runs produce identical results, ensuring consistency across test environments," plus `linked_columns` for cross-table consistency with no FK path (see the [Brume entry](#brume-2026)). The bar to match is Brume, not an abstract ideal — and printing *why* a column got its determinism guarantee, the way we intend for detection, is still open ground even against Brume.
13. **A generator space large enough not to violate unique constraints.** [Greenmask #338](https://github.com/GreenmaskIO/greenmask/issues/338): 148 names × 6 suffixes, sampled with replacement, collides inside 1,000 rows.
14. **Format and type preservation** — a masked email still parses as an email, a masked `varchar(20)` still fits. pgEdge Anonymizer and Basecut both advertise this.
15. **Fail closed on schema drift.** [pg_partialcopy's](https://github.com/jackc/pg_partialcopy) explicit-column `select_sql` exists precisely so "if a column is added to a source database, it is not automatically included". A new unclassified column must be masked or must stop the run — never silently copied.
16. **Correct legal language.** Brume's README distinguishes GDPR Art. 4.5 pseudonymisation from recital 26 anonymisation and says the output "remains personal data". We must not claim more than we deliver.

**Licence risk, for anyone reading a competitor's source for algorithm ideas.** This document recommends studying specific mechanisms from Jailer (its nullable-FK cycle deferral), Tonic Structural (the same trick, and its virtual-FK tool), and PostgreSQL Anonymizer (`anon.detect()`'s category model). All three are safe to learn from: Jailer is Apache-2.0, PostgreSQL Anonymizer uses the PostgreSQL License (permissive, BSD-style, no copyleft), and Tonic Structural's mechanism is described only in prose in its docs, not in code we'd be copying. Across the rest of the field, licences are overwhelmingly permissive — MIT (dbslice, Neosync, Snaplet Snapshot, slice-db, condenser, db-condenser, pgsubset, pg_subsetter, pg_partialcopy), BSD-3-Clause (msg555/subsetter), BSD-2-Clause (fixturize), Apache-2.0 (Greenmask, Brume) — with two exceptions worth flagging explicitly: **Replibyte is GPL-3.0** despite a README badge claiming MIT (already noted above; do not port code from it without observing the GPL), and **pg_sample uses the Artistic License**, declared only in the script's POD with no `LICENSE` file — a weak-copyleft-adjacent licence with its own attribution terms, worth a real read before reusing any of its Perl rather than assuming it behaves like MIT. recordrelay's licence is unresolved by GitHub's detector (`NOASSERTION`) despite an Apache-2.0 badge, so treat anything from it as unconfirmed until someone reads its `LICENSE` file directly.

**The result**

17. **Load into the target and then verify foreign keys, in-process, and report it.** Only Basecut claims this and only in marketing copy. This is cheap for us and closes the gap that produced [condenser #14](https://github.com/TonicAI/condenser/issues/14) — sequences silently reset to 1 in the target, unfixed since 2019.
18. **Reset sequences, and say so.** Directly from that issue. A snapshot whose sequences start at 1 is not usable.
19. **Emit `lazysnap.yml` after the run.** No tool in this field writes back a record of what it did. Basecut and dbslice generate config *before*; nobody produces the reproducibility artefact *after*.
20. **PostgreSQL depth before breadth.** Greenmask has had [MySQL support](https://github.com/GreenmaskIO/greenmask/issues/222) as its top-voted open issue since October 2024 and it is still open; Basecut ships Postgres only and puts MySQL on a roadmap; dbslice lists MySQL and SQLite as "Planned (not yet implemented)". The whole field is Postgres-first for a reason, and adding engines before the Postgres path is excellent is how Greenmask ended up with an [epic to rebuild its subset system](https://github.com/GreenmaskIO/greenmask/issues/392).

**Where we can beat everyone, on current evidence**

- **Container discovery on first run.** No tool auto-discovers running Postgres containers and proposes a source and target pair unprompted — every quickstart in this document starts with the user pasting two connection strings. The nearest prior art is DBSnapper's `pgdocker://`/`mydocker://` URL schemes, which let you *name* a container in a connection string once you already know which one you want; that is not discovery, and saying so explicitly is a stronger claim than not knowing DBSnapper's container support exists.
- **Explaining the classification.** Detection exists; justification does not.
- **The plan preview with an honest row estimate.** The blow-up is documented by the incumbents themselves and unaddressed by all of them.
- **Config as output, not input.** Nobody in this field emits the run as a committable artefact.
- **Verification as a step, not a claim.** One tool claims it in marketing; none documents a command for it.
