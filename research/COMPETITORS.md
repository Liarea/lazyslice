# COMPETITORS

Teardown of every tool that does some part of "point at a production SQL database, get a small, referentially complete, anonymised copy". Researched 4 September 2026. Every claim links to its source. Where a fact could not be verified it says **unverified**.

Star counts, commit dates and issue counts were read from the GitHub REST API and the GitLab API on 4 September 2026 and will drift.

---

## Orientation

| Tool | Lang | Subsets? | Masks? | Detects PII? | Alive? |
|---|---|---|---|---|---|
| [Greenmask](#greenmask) | Go | yes | yes | no | yes, active |
| [Basecut](#basecut) | closed binary | yes | yes | yes | yes, launched 2026 |
| [Neosync](#neosync) | Go | yes (SQL query) | yes | yes | **dead** — archived 30 Aug 2025 |
| [Snaplet Snapshot](#snaplet-snapshot) | TypeScript | yes | yes | partial | **dead** — archived |
| [PostgreSQL Anonymizer](#postgresql-anonymizer) | Rust (pgrx) | no | yes | yes (`anon.detect`) | yes, very active |
| [Jailer](#jailer) | Java | yes | filters only | no | yes, very active |
| [condenser](#condenser) | Python | yes | no | no | **stale** — last commit 2023 |
| [db-condenser](#db-condenser-fork-of-condenser) | Python | yes | no (open issue) | no | yes, new 2025/26 |
| [Replibyte](#replibyte) | Rust | yes | yes | no | **dead** — last commit 2024 |
| [pgsubset](#pgsubset) | Rust | yes | no | no | **dead** — one commit, 2022 |
| [pg_sample](#pg_sample) | Perl | yes | no | no | low-activity |
| [Tonic Structural](#tonic-structural) | closed | yes | yes | yes | yes, commercial |
| [Redgate Data Masker / TDM](#redgate-data-masker-and-redgate-test-data-manager) | closed | TDM only | yes | yes (classify) | yes, commercial |
| [fixturize](#fixturize-2026) | Go | yes | yes | yes | yes, new 2026 |
| [pgEdge Anonymizer](#pgedge-anonymizer-2025) | Go | no | yes | no | yes, new 2025 |
| [DBSnapper](#dbsnapper) | closed binary | yes | SQL script | no | yes, commercial |
| [msg555/subsetter](#msg555subsetter) | Python | yes | yes (faker) | no | yes, low profile |
| [Seedfast](#seedfast-2025) | closed | n/a | n/a | n/a | yes — synthetic, not subsetting |
| [VeilStream](#veilstream-2025) | closed | no | yes (proxy) | no | yes, new 2025 |

---

## Greenmask

**Repo:** [GreenmaskIO/greenmask](https://github.com/GreenmaskIO/greenmask) · **Site:** [greenmask.io](https://www.greenmask.io/) · **Docs:** [docs.greenmask.io](https://docs.greenmask.io/)

- **Language:** Go, distributed as a single static binary ([README](https://github.com/GreenmaskIO/greenmask#key-features): "Cross-Platform: Single binary, runs anywhere").
- **Databases:** PostgreSQL production-ready; MySQL is a work-in-progress beta tracked in [issue #222](https://github.com/GreenmaskIO/greenmask/issues/222). The project's own FAQ, as of 2026, still says ["Currently, Greenmask supports PostgreSQL and S3. Support for MySQL, MongoDB, and other databases is actively in development"](https://www.greenmask.io/).
- **Subsetting:** Declared as `subset_conds` on a table in the dump config. Greenmask builds a table dependency graph from introspection and generates SQL with joins, traversing **both directions** — downstream to children that reference filtered parents, and upstream to parents needed to satisfy FKs ([Database subset docs](https://docs.greenmask.io/latest/database_subset/)).
- **Cycle handling:** Recursive SQL queries with integrity checks for circular references. Documented limitation: it handles multiple cycles inside one strongly connected component but not "2 groups of vertexes" in a single SCC, tracked as [issue #197](https://github.com/GreenmaskIO/greenmask/issues/197). Polymorphic references are supported through `polymorphic_exprs` virtual references.
- **Masking config:** YAML. Every transformer is written by hand against a named schema, table and column — see the [playground config](https://github.com/GreenmaskIO/greenmask/blob/main/playground/config.yml). Deterministic transformation is available through a hash engine. **There is no PII detection**: nothing in the tool tells you which columns look personal.
- **Install:** `curl -fsSL https://greenmask.io/install.sh | sh`, `brew install greenmask`, Docker, or build from source ([installation docs](https://docs.greenmask.io/latest/installation/)). Requires PostgreSQL client utilities whose major version matches the destination server.
- **Licence:** Apache-2.0 (GitHub API, repo `GreenmaskIO/greenmask`).
- **Last commit:** 2026-08-22, `docs: release notes v0.2.23`. Latest release **v0.2.23**, 2026-08-22. Still pre-1.0 after nearly three years; [issue #358 "[EPIC] Greenmask V1"](https://github.com/GreenmaskIO/greenmask/issues/358) was opened 2025-11-04.
- **Stars:** 1,757. **Open issues:** 45.

**Three most-upvoted open issues**

1. [#222 epic: MySQL support](https://github.com/GreenmaskIO/greenmask/issues/222) — +8, 33 comments, open since 2024-10-15.
2. [#104 Bug: --data-only flag interfere with --schema-only](https://github.com/GreenmaskIO/greenmask/issues/104) — +3, open since 2024-05-08, zero comments.
3. [#111 feat: unique transformations](https://github.com/GreenmaskIO/greenmask/issues/111) — +2, open since 2024-05-12, zero comments.

Also worth reading for the failure modes: [#329 "greenmask does not work if there is not cycle in the database"](https://github.com/GreenmaskIO/greenmask/issues/329) (18 comments, open since 2025-08-13 — the subset component errors when the graph has *no* cycles), [#444 "Tables with over 128 columns fail due to out of index range"](https://github.com/GreenmaskIO/greenmask/issues/444) (a fixed `[128]` array in the COPY decoder panics on a 129-column table), and [#396](https://github.com/GreenmaskIO/greenmask/issues/396), where polymorphic subset predicates "produce ~0 rows".

---

## Basecut

**Site:** [basecut.dev](https://basecut.dev/) · **Docs:** [docs.basecut.dev](https://docs.basecut.dev/) · **Release repo:** [basecuthq/cli](https://github.com/basecuthq/cli)

This is the closest thing on the market to what CONCEPT.md describes, and it launched this year. Treat it as the primary competitor.

- **Language:** Not stated publicly. The [Homebrew formula](https://github.com/basecuthq/homebrew-cli/blob/main/Formula/basecut.rb) ships single prebuilt binaries for `darwin-amd64`, `darwin-arm64`, `linux-amd64`, `linux-arm64` at version 0.1.21 — the shape of a Go or Rust build. **Unverified** which.
- **Databases:** PostgreSQL 12+ only. ["What databases are supported? PostgreSQL today. MySQL and SQL Server are on the roadmap."](https://basecut.dev/pricing)
- **Subsetting:** Root tables with a `WHERE` clause and named params, then recursive traversal in **both directions** with per-direction depth budgets (`traverse: {parents: 5, children: 10}`). Missing parents are pulled in even beyond the depth limit to keep integrity. Virtual (application-level) foreign keys are declared in config and traversed as real edges. Limits are `rows.per_table` and `rows.total` ([How It Works](https://docs.basecut.dev/core-concepts/how-it-works.md)).
- **Cycle handling:** `basecut init` claims "Cycle detection: Finds and handles circular relationships" ([Quick Start](https://docs.basecut.dev/getting-started/quick-start.md)). The docs do not say what "handles" means. **Unverified.**
- **Masking config:** `anonymize: auto` is a one-line default with built-in PII detection, plus `manual` (explicit rules only) and **`off`** (no anonymisation at all). 30+ strategies including `hash` (deterministic), `fake_email`, `email_preserve_domain`, `date_shift`, `partial_mask`, `numeric_noise`. Masking runs *during* extraction so the snapshot never holds real values. Org-level "snapshot rules" can force strategies on Team/Enterprise plans ([Anonymization](https://docs.basecut.dev/configuration/anonymization.md)).
- **Install:** `brew install basecuthq/cli/basecut` or `curl -fsSL https://basecut.dev/install.sh | sh`. **Then `basecut login`, which opens a browser and requires a Basecut account** before the first snapshot.
- **Licence:** Proprietary. The Homebrew formula declares `license :cannot_represent`. Free tier ($0, 3 members, 20 snapshots/month, 30-day retention), Team $99/month, Enterprise custom ([pricing](https://basecut.dev/pricing)).
- **Last commit / activity:** The `basecuthq` GitHub org was created 2026-02-11; the CLI release repo was last pushed 2026-03-26. Show HN on [2026-03-31 scored 3 points](https://news.ycombinator.com/item?id=47586925).
- **Stars:** 0 across all public Basecut repos (they are release/tap repos only, not source).
- **Open issues:** none public — issues are not accepted on the release repos.

**Where they differ from us:** the account requirement before first snapshot, the closed source, the metadata-in-their-cloud model, and `anonymize: off` — which CONCEPT.md explicitly refuses to ship.

---

## Neosync

**Repo:** [nucleuscloud/neosync](https://github.com/nucleuscloud/neosync) — **ARCHIVED, read-only.**

Status verified as of September 2026: the repository is archived, the last commit is 2025-08-30 with the message `adds acquired disclaimer (#3568)`, and the README carries the line ["Neosync has been acquired by Grow Therapy. As a result, this repository is no longer actively maintained."](https://github.com/nucleuscloud/neosync)

- **Acquisition:** Grow Therapy acquired Neosync. Directly verified: the last commit is 2025-08-30 (GitHub API) and the [PR Newswire announcement](https://www.prnewswire.com/news-releases/grow-therapy-raises-the-privacy-bar-in-mental-health-302567153.html) is dated 25 September 2025 — so the repo was frozen almost a month *before* the deal was announced. The press release talks only about bringing the technology into Grow's mental-health platform and says nothing about the future of the open-source project or the hosted service. The [Crunchbase acquisition record](https://www.crunchbase.com/acquisition/grow-therapy-acquires-neosync-cd81--00632527) puts the deal itself at 1 August 2025; Crunchbase blocks automated fetches, so that specific date is **reported via search results, not directly verified**. HN noticed on [22 September 2025](https://news.ycombinator.com/item?id=45331127) — one point, one comment.
- **Language:** Go. **Databases:** PostgreSQL, MySQL, S3 (README).
- **Subsetting:** by arbitrary SQL query, with FK constraints maintained automatically across tables (README).
- **Masking config:** pre-built transformers plus custom transformers "using javascript or LLMs" (README).
- **Install:** Docker required — clone the repo and `make compose/up`, then a web UI on `localhost:3000` (README). Kubernetes-native with a Temporal dependency, per [Xata's 2026 round-up](https://xata.io/blog/the-5-best-data-anonymization-tools-for-development-teams-in-2026).
- **Licence:** MIT Expat with an `ee/` enterprise carve-out ([LICENSE.md](https://github.com/nucleuscloud/neosync/blob/main/LICENSE.md)). GitHub reports the licence as `NOASSERTION` because of the carve-out.
- **Last commit:** 2025-08-30. **Stars:** 4,141. **Open issues:** 35, frozen.

**Three most-upvoted open issues** (all now unactionable):

1. [#3411 Error Encountered When Setting Up MySQL 5.7 Connection](https://github.com/nucleuscloud/neosync/issues/3411) — +2, 2025-03-26.
2. [#1968 Any option to define destination db's schema?](https://github.com/nucleuscloud/neosync/issues/1968) — +1, 2024-05-20.
3. [#2009 Support Snowflake](https://github.com/nucleuscloud/neosync/issues/2009) — +0, 2024-05-23.

Neosync was the loudest tool in this space — its [Show HN in May 2024 scored 246 points with 44 comments](https://news.ycombinator.com/item?id=40443927) — and it is gone. That is the single most important market fact in this document.

---

## Snaplet Snapshot

**Repo:** [supabase-community/snapshot](https://github.com/supabase-community/snapshot) (redirected from `snaplet/snapshot`) — **ARCHIVED.**

- **History:** Snaplet shut down; [Supabase announced the open-sourcing on 14 August 2024](https://supabase.com/blog/snaplet-is-now-open-source) under MIT, covering three repos: `copycat` (deterministic fake data), `seed` (synthetic data from schema) and `snapshot` (capture, transform, restore). The team joined Supabase and the repos moved to the `supabase-community` org. HN [noted it](https://news.ycombinator.com/item?id=41244171) with four points.
- **Verified activity as of September 2026:** `supabase-community/snapshot` is **archived**; last commit 2025-11-05 (`try macos-15-intel`), latest release **v0.93.2** dated 2024-08-02, 328 stars, 4 open issues. Sibling repos: [`copycat`](https://github.com/supabase-community/copycat) not archived, last commit 2025-01-14, 1,057 stars; [`seed`](https://github.com/supabase-community/seed) not archived but its `main` branch last moved 2024-08-14, 790 stars, 20 open issues. In short: the snapshot product is dead, the fake-data library is dormant, the seeder is on life support.
- **Language:** TypeScript, distributed on npm.
- **Databases:** PostgreSQL.
- **Subsetting:** "Snapshots can be subset (reduced in size), and the source data can be transformed" ([README](https://github.com/supabase-community/snapshot#introduction)). The algorithm is not described in the README and the linked doc site (`snaplet-snapshot.netlify.app`) is a Netlify deployment of the archived docs.
- **Masking config:** a `snaplet.config.ts` transform file, generated by `npx @snaplet/snapshot setup`.
- **Install:** `npx @snaplet/snapshot setup`, then `SNAPLET_SOURCE_DATABASE_URL=... npx @snaplet/snapshot snapshot capture`, `... snapshot restore`. Contributing requires PostgreSQL 15, `libpq`, `brotli`, Node 20+, yarn 3.5+.
- **Licence:** MIT.

**Three most-upvoted open issues**

1. [#15 npm install failure](https://github.com/supabase-community/snapshot/issues/15) — +4, 6 comments, 2024-10-18.
2. [#18 Outdated documentation on configuring select and transform](https://github.com/supabase-community/snapshot/issues/18) — +2, 2025-01-17.
3. [#20 NPM Install Failing related to libtool static on ARM Darwin architecture platforms](https://github.com/supabase-community/snapshot/issues/20) — +2, 2025-04-21.

Two of the three top issues are **the install failing on a Mac**. That is a native-dependency tax lazysnap avoids by shipping a static binary.

---

## PostgreSQL Anonymizer

**Repo:** [gitlab.com/dalibo/postgresql_anonymizer](https://gitlab.com/dalibo/postgresql_anonymizer) · **Docs:** [postgresql-anonymizer.readthedocs.io](https://postgresql-anonymizer.readthedocs.io/)

- **Language:** Rust, since the 2.0 rewrite on the PGRX framework. It is a **PostgreSQL extension**, not a CLI.
- **Databases:** PostgreSQL only, plus forks and DBaaS providers — [3.0 lists Alibaba Cloud, Crunchy Bridge, Google Cloud SQL, IBM Cloud, Azure Database, Neon, Yandex, EDB Advanced Postgres and Greenplum](https://www.postgresql.org/about/news/postgresql-anonymizer-30-parallel-static-masking-json-import-export-3236/).
- **Subsetting: none.** The docs never mention it. This is a masking tool that assumes you already have a copy.
- **Masking config:** declarative DDL via `SECURITY LABEL`, e.g. `SECURITY LABEL FOR anon ON COLUMN people.lastname IS 'MASKED WITH FUNCTION anon.dummy_last_name();'` ([docs](https://postgresql-anonymizer.readthedocs.io/en/latest/)). Six strategies: dynamic, static, replica, backup, masking views and masking data wrappers. Rules live *in the database*, which means someone with DDL rights on production must add them.
- **PII detection:** `SELECT anon.detect('en_US');` returns table, column, identifier category and whether it is a direct identifier, matched against a dictionary and categorised against HIPAA. The docs are candid that it produces both false positives and false negatives and that ["you still need to review the entire database model in search of hidden identifiers"](https://postgresql-anonymizer.readthedocs.io/en/latest/detection/).
- **Anonymous dumps:** `pg_dump_anon` and `pg_dump_anon.sh` are **deprecated**. The current path is to create a masked login role and run stock `pg_dump` as that role: `pg_dump foo --user anon_dumper --no-security-labels --exclude-extension="anon" --file=foo_anonymized.sql` ([anonymous dumps](https://postgresql-anonymizer.readthedocs.io/en/latest/anonymous_dumps/)).
- **Install:** Debian/RPM packages, Docker (`docker run ... registry.gitlab.com/dalibo/postgresql_anonymizer`), Ansible, PGXN. Superuser or extension-install rights required.
- **Licence:** The PostgreSQL License (BSD-like) — [LICENSE.md](https://gitlab.com/dalibo/postgresql_anonymizer/-/blob/master/LICENSE.md), "Copyright (c) 2018-2020, DALIBO SCOP".
- **Last commit / activity:** last activity 2026-09-04. Latest tag **3.1.3**, 2026-06-29. Version 3.0 was announced 11 February 2026 (posted to postgresql.org 2026-02-19) with parallel static masking and JSON policy import/export, and fixed two privilege-escalation CVEs, CVE-2026-2360 and CVE-2026-2361.
- **Stars:** 294 (GitLab). **Open issues:** 31.

**Three most-upvoted open issues** (GitLab upvotes are sparse; these are the four-way tie at +1, most recent first)

1. [#672 Code Reorg](https://gitlab.com/dalibo/postgresql_anonymizer/-/work_items/672) — +1, 2026-09-03.
2. [#667 Masked role cannot INSERT/UPDATE a table that has a masking rule](https://gitlab.com/dalibo/postgresql_anonymizer/-/work_items/667) — +1, 6 comments, 2026-08-24.
3. [#564 sequences not handled correctly when privacy by default is active](https://gitlab.com/dalibo/postgresql_anonymizer/-/work_items/564) — +1, 5 comments, 2025-09-03.

This is the most technically serious masking project in the field and it is not a competitor to lazysnap so much as a component we should be measured against on masking quality. It does not subset, and it requires production DDL.

---

## Jailer

**Repo:** [Wisser/Jailer](https://github.com/Wisser/Jailer) · **Site:** [wisser.github.io/Jailer](https://wisser.github.io/Jailer/)

- **Language:** Java, over JDBC. Claims support for 24+ database systems including PostgreSQL, Oracle, MySQL, SQL Server, SQLite, Redshift and Snowflake ([home](https://wisser.github.io/Jailer/)).
- **Subsetting:** You define an **extraction model** — a subject table plus a `WHERE` condition — and Jailer follows foreign keys or user-defined associations from there. Export formats: topologically sorted SQL DML, JSON, YAML, XML and DbUnit datasets ([Exporting Data](https://wisser.github.io/Jailer/exporting-data.htm)).
- **FK traversal direction:** both, and **unrestricted by default**, which is the tool's defining problem. The tutorial itself walks you through discovering that exporting employee "SCOTT" also drags in every employee in the same department and every employee on the same salary grade, then tells you to hand-define five *restrictions* to stop it: "To exclude subordinates, department-members and 'same salary-grade'-employees, we must restrict some associations."
- **Cycle handling:** it can "detect and break cycles in parent-child relationships" by "deferring the insertion of nullable foreign keys" ([home](https://wisser.github.io/Jailer/)).
- **Masking:** none. There is a [Filters](https://wisser.github.io/Jailer/filters.html) feature that rewrites column values with SQL expressions, which can be used for crude obfuscation, but there is no PII model, no detection and no deterministic-masking guarantee.
- **Install:** download `Jailer-database-tools-n.n.n.msi` (Windows) or a `.deb` (Linux) for the GUI; **for the CLI you must unzip `jailer_n.n.n.zip` and call `jailer.bat` / `jailer.sh`**, or `java -jar jailer.jar` ([Installation](https://wisser.github.io/Jailer/installation-2.htm)). There is also a Java API, `net.sf.jailer.api.Subsetter` ([API](https://wisser.github.io/Jailer/api.html)).
- **Licence:** Apache-2.0 (GitHub API).
- **Last commit:** 2026-09-04, `subset insight, ongoing work`. Latest release **v17.2.2**, 2026-08-19. Fifteen years old and still shipping — the most durable project in the field.
- **Stars:** 3,195. **Open issues: 0.** 80 issues have ever been filed and every one is closed.

**Three most-upvoted open issues:** there are none. The maintainer closes everything. As a substitute, the three most instructive recent issues:

1. [#126 Subject condition is violated due to reverse traversal (unexpected rows in subset)](https://github.com/Wisser/Jailer/issues/126) — filed 2025-08-05, closed 2026-04-14, eight months open. The reporter set `T.customer_id = 1` on a `customers` subject and got orders for customers 2–10: "This happens due to implicit reverse traversal, which is not documented clearly / Bypasses the subject condition unless explicitly restricted."
2. [#132 H2 generates incorrect DDL](https://github.com/Wisser/Jailer/issues/132) — filed 2026-07-16, closed 2026-08-11.
3. [#127 DBUnit export of an XML column from a DB2 database doesn't work](https://github.com/Wisser/Jailer/issues/127) — filed 2026-08-21, closed the next day.

Issue #126 is the sharpest evidence in this whole document for why a naive "follow every FK" subsetter is not usable out of the box.

---

## condenser

**Repo:** [TonicAI/condenser](https://github.com/TonicAI/condenser)

- **Language:** Python 3.5+. **Databases:** PostgreSQL and MySQL, shelling out to `pg_dump`/`psql` or `mysqldump`/`mysql`.
- **Subsetting:** config-driven. `initial_targets` seeds from a table with either a `percent` or a `where` clause; the tool then does "upstream subsetting" where it "greedily grabs as many rows from the database as it can, based on the rows already imported", tempered by optional `upstream_filters` ([README](https://github.com/TonicAI/condenser#config)).
- **Cycle handling:** **manual.** "The subsetting tool cannot operate on databases with cycles in their foreign key relationships... This field lets you tell the subsetter to ignore certain foreign keys... You'll have to know a bit about your database to use this field effectively." You hand-write `dependency_breaks` entries. Implicit FKs are hand-declared in `fk_augmentation`.
- **Masking:** none.
- **Install:** `pip install toposort psycopg2-binary mysql-connector-python`, install PostgreSQL/MySQL client tools on `$PATH`, clone the repo, write `config.json`, `python direct_subset.py`.
- **Scale limit, stated by the vendor:** "Our open-source tool can subset databases up to 10GB, but it will struggle with larger databases. Our premium database subsetter can... subset multi-TB databases with ease." This is a lead magnet for Tonic Structural.
- **Licence:** MIT. **Last commit:** 2023-05-18. **Stars:** 337. **Open issues:** 11.

**Three most-upvoted open issues**

1. [#21 Bundling as package?](https://github.com/TonicAI/condenser/issues/21) — +2, 2021-05-13. There is still no pip package.
2. [#30 Writing subset to SQL file instead of to destination DB](https://github.com/TonicAI/condenser/issues/30) — +2, 2022-07-07.
3. [#14 PostgreSQL sequences need to be reset after subsetter is completed](https://github.com/TonicAI/condenser/issues/14) — +1, 2019-10-09, still open seven years later. "after a subset has been generated, I need to run some SQL to reset DB sequences to their max value... Otherwise, the sequences are all reset to 1."

---

## db-condenser (fork of condenser)

**Repo:** [tkhuu01/db-condenser](https://github.com/tkhuu01/db-condenser) — a 2025/2026 arrival.

- Python, MIT, created 2025-11-02, last commit **2026-09-02**, 0 stars, 3 open issues.
- Claims over the original: subsets databases larger than 10 GB, runs against read-only replicas, supports incremental top-ups so an existing subset can grow without a rebuild, handles dense FK graphs, concurrent worker pools, PostgreSQL `COPY` protocol, **automatic sequence reset after subsetting** (fixing condenser #14), built on psycopg3 and `uv` ([README](https://github.com/tkhuu01/db-condenser)).
- **Masking: not implemented.** It is [issue #25, "Data masking"](https://github.com/tkhuu01/db-condenser/issues/25), opened 2026-08-31.

**Three open issues** (all +0, this is a one-person project):

1. [#1 Decouple Core Subsetting Algorithm](https://github.com/tkhuu01/db-condenser/issues/1) — 2025-12-17.
2. [#25 Data masking](https://github.com/tkhuu01/db-condenser/issues/25) — 2026-08-31.
3. [#24 Refactor documentation](https://github.com/tkhuu01/db-condenser/issues/24) — 2026-08-29.

---

## Replibyte

**Repo:** [Qovery/Replibyte](https://github.com/Qovery/Replibyte) · **Docs:** [replibyte.com](https://www.replibyte.com/)

- **Language:** Rust, stateless single binary. **Databases:** PostgreSQL, MySQL, MongoDB — but **subsetting is PostgreSQL-only**: "Only PostgreSQL supports Subsetting at the moment" ([subset docs](https://www.replibyte.com/docs/guides/subset-a-dump)).
- **Subsetting:** a `database_subset` block naming a table and a `strategy_name: random` with `percent`, plus `passthrough_tables`. The docs section titled "Subset Strategy" reads, in full, **"TODO"**. That section has been a TODO since the docs were last touched.
- **Masking config:** per-column `transformers` in `conf.yaml`, named by hand (`first-name`, `random`, `phone-number`, `email`). WASM custom transformers are supported. **No detection** — the roadmap item "Auto-detect sensitive fields" is unchecked in the README.
- **Install:** binary; then a `conf.yaml` that also needs a datastore (local or S3) because Replibyte's model is dump-to-datastore then restore-from-datastore, not source-to-target.
- **Licence:** The README badge says MIT, but the repository's [LICENSE file is GPL-3.0](https://github.com/Qovery/Replibyte/blob/main/LICENSE) and the GitHub API reports `GPL-3.0`. Anyone vendoring this should note the contradiction.
- **Last commit:** 2024-05-04. Latest release **v0.10.0**, 2022-10-14 — nearly four years without a release.
- **Stars:** 4,409, the highest star count in the field, on a dead project. **Open issues:** 103.

**Three most-upvoted open issues**

1. [#74 New subset strategy: create SELECT with WHERE clause strategy](https://github.com/Qovery/Replibyte/issues/74) — +18, opened 2022-04-30, never implemented. The most-wanted feature in the entire field, four years unshipped: users want to say "give me *this* customer", not "give me 10% at random".
2. [#105 Support for Microsoft SQL](https://github.com/Qovery/Replibyte/issues/105) — +12, 2022-05-14.
3. [#36 Support COPY query for PostgreSQL](https://github.com/Qovery/Replibyte/issues/36) — +7, 2022-03-31.

Its [Show HN in April 2022 drew 129 points and 78 comments](https://news.ycombinator.com/item?id=31165538); a [July 2022 repost drew 222](https://news.ycombinator.com/item?id=32047535). Enormous interest, no maintenance.

---

## pgsubset

**Repo:** [MaieuticalLabs/pgsubset](https://github.com/MaieuticalLabs/pgsubset)

- **Language:** Rust. **Databases:** PostgreSQL only.
- **Subsetting:** "it calculates all the objects needed for a target table to mantain referential integrity and it uses pg's `COPY` instructions to efficiently export them as csv files" ([README](https://github.com/MaieuticalLabs/pgsubset)). No description of traversal direction or cycle handling anywhere in the repo. **Unverified.**
- **Masking:** none.
- **Config:** a TOML file with, minimally, `database_url`, `target_table`, `target_dir`. Two modes: `--mode export` writes numbered CSVs, `--mode import` reads them into a database with the same schema.
- **Install:** install rustup, install the stable toolchain, `cargo install --path .` from a clone. No release binaries, not on crates.io.
- **Licence:** MIT. **Last commit:** 2022-10-13 — the *initial* commit, and the only one. **Stars:** 2. **Open issues:** 0.

**Three most-upvoted open issues:** none exist; nobody has ever filed one.

Included because the task named it. It is a proof of concept, not a competitor.

---

## pg_sample

**Repo:** [mla/pg_sample](https://github.com/mla/pg_sample)

- **Language:** Perl (DBI + DBD::Pg). **Databases:** PostgreSQL 8.1+.
- **Subsetting:** "The sample database produced includes all tables from the original, maintains referential integrity, and **supports circular dependencies**" ([README](https://github.com/mla/pg_sample)). Default is 100 rows per table, with the caveat that "sample tables may end up with significantly more rows in order to satisfy foreign key constraints" — that is, parents are followed to completeness. There is no root-table concept; you tune per-table rules with repeated `--limit` flags: `--limit="users = 1000"`, `--limit="users = 10%"`, `--limit="users = NOT deactivated"`, `--limit="forums.* = *"`.
- **Cycle handling:** claimed supported; mechanism undocumented. It builds a temporary sample schema in the source database (`--keep`, `--force` control its lifecycle), which means **it writes to the source** — disqualifying under our "never holds write access to the source" principle.
- **Masking:** none.
- **Install:** `sudo apt install perl libdbi-perl libdbd-pg-perl`, clone the repo, run `./pg_sample`. A Docker image `mla12/pg_sample` exists. `pg_dump` must be on `$PATH`.
- **Output:** plain-text SQL to stdout: `pg_sample mydb | psql -v ON_ERROR_STOP=1 sampledb`. This is the single cleanest first-run in the field.
- **Licence:** the Artistic License, stated in the README; there is **no LICENSE file**, so GitHub reports no licence.
- **Last commit:** 2025-03-30 (`simplify`). **Stars:** 355. **Open issues:** 15.

**Three most-upvoted open issues**

1. [#19 Add installation instructions](https://github.com/mla/pg_sample/issues/19) — +7, 2020-08-04. The body is two words: "How do I install this?" Six years open.
2. [#44 Is there a way to export a sample by a table row?](https://github.com/mla/pg_sample/issues/44) — +1, 2023-02-12. Again: root-table subsetting, requested and unbuilt.
3. [#5 DBD::Pg::db do failed: ERROR:](https://github.com/mla/pg_sample/issues/5) — +0 but **63 comments**, open since 2016-03-03. A ten-year-old error thread is the busiest place in the repo.

---

## Tonic Structural

**Site:** [tonic.ai](https://www.tonic.ai/) · **Docs:** [docs.tonic.ai](https://docs.tonic.ai/)

- **Language:** closed source. **Databases:** broad — the [Xata round-up](https://xata.io/blog/the-5-best-data-anonymization-tools-for-development-teams-in-2026) lists PostgreSQL, MySQL, SQL Server, Oracle, Db2, MongoDB, DynamoDB, Redshift, Snowflake, BigQuery, Databricks and Salesforce. Tonic's own docs note that Snowflake and Redshift support subsetting while Iceberg, Databricks and BigQuery do not and must use table filtering instead.
- **Subsetting algorithm**, from Tonic's docs (retrieved via their documentation query endpoint, sourcing [About subsetting](https://docs.tonic.ai/app/generation/subsetting/subsetting-about.md) and [Subsetting and foreign keys](https://docs.tonic.ai/app/generation/subsetting/subsetting-foreign-keys.md)):
  1. Seed from configured **target tables** using a percentage or `WHERE` clause.
  2. Traverse **"upstream"** — Tonic's word for tables whose FK column holds the target's PK, i.e. what most people call children — repeating until exhausted.
  3. Traverse **"downstream"** — tables whose PK is referenced by an upstream row — and it may re-enter the upstream phase to keep the graph connected.
  Note the inverted vocabulary: Tonic's "upstream" is everyone else's "children". This matters when reading their docs against ours.
- **Cycle handling:** break the cycle by finding a **nullable** FK column and setting values to `NULL`, applying the minimum number of such nullings. **If no FK in the cycle is nullable, subset generation fails.**
- **Masking config:** web UI, with sensitivity scanning and 50+ generators; "patented cross-database subsetting" across databases.
- **CLI:** subsetting is configurable through the [Structural API](https://docs.tonic.ai/app/api/quick-start-guide/tonic-api-subsetting-config.md); their docs describe **no CLI** for subset configuration.
- **Install:** SaaS, Docker Compose, Kubernetes Helm, or air-gapped.
- **Licence / pricing:** proprietary. Structural is **Professional (custom pricing, up to 10TB source, 10 users)** or **Enterprise (custom, unlimited)** — [no self-serve tier and no published price](https://www.tonic.ai/pricing). Only their synthetic-data product Fabricate has a $0 free tier. Self-hosting is Enterprise-only.
- **Last commit / stars / issues:** not applicable, closed source. Their open-source [condenser](#condenser) is covered above and last moved in 2023.

---

## Redgate Data Masker, and Redgate Test Data Manager

Two separate products; the task named the first, but only the second is relevant to PostgreSQL.

### Data Masker

- **Databases:** SQL Server and Oracle ([product page](https://www.red-gate.com/products/data-masker/)). Redgate's own forum thread [Data Masker and PostgreSQL](https://forum.red-gate.com/discussion/88368/data-masker-and-postgresql) records that there are **no plans to support PostgreSQL** with Data Masker.
- **Masking config:** a GUI-built "masking set" with an "Advanced UI for ease of maintenance" ([documentation](https://documentation.red-gate.com/dms/data-masker-help)).
- **Subsetting:** none.
- **CLI:** `DataMaskerCmdLine.exe` driven by a PARFILE options text file; automating via `DataMasker.exe` was removed in version 7.0.0 ([About Command Line Automation](https://documentation.red-gate.com/dms6/data-masker-help/general-topics/about-command-line-automation)).
- **Install / licence:** Windows installer, proprietary, trial download; pricing not published on the product page.

### Test Data Manager (`rgsubset` + `rganonymize`)

This is Redgate's actual competitor to us, and it does support PostgreSQL.

- **Databases:** SQL Server, PostgreSQL, MySQL/MariaDB and Oracle ([TDM CLI docs](https://documentation.red-gate.com/testdatamanager/command-line-interface-cli/subsetting)).
- **Subsetting:** `rgsubset run` "extracts a subset of data from a source database and transfers it to a target database while preserving referential integrity". Configured by an **options file in YAML or JSON** naming starting tables with a filter clause, static-data tables, excluded tables and manual relationships. From the worked example, [`rgsubset-options-autopilot.json`](https://github.com/red-gate/TDM-AutoPilot/blob/main/Setup_Files/Data_Treatments_Options_Files/rgsubset-options-autopilot.json), that is 41 lines of JSON for a Northwind-sized toy.
- **A gotcha worth stealing:** `includeTablesRowThreshold` defaults to **300** — any table with 300 rows or fewer is copied whole. Redgate's own README warns that "without this parameter the subsetting can seem to perform unexpectedly on very small databases... `dbo.Customers` should bring through around 12 rows aswell, but without this parameter rgsubset will populate the `dbo.Customers` table with all it's original data" ([TDM-AutoPilot README](https://github.com/red-gate/TDM-AutoPilot#subsetting-options-file-rgsubset-options-northwindjson-description)).
- **Cycle handling:** not documented in the pages retrieved. **Unverified.**
- **Masking:** a three-command pipeline — `rganonymize classify` writes a `classification.json`, `rganonymize map` turns it into a `masking.json`, `rganonymize mask` applies it ([CLI_Tutorials.md](https://github.com/red-gate/TDM-AutoPilot/blob/main/CLI_Tutorials.md)). Classification is automatic; that is the good part.
- **Install:** Windows (the worked example says "A Windows machine to run this script and the rgsubsetter/rganonymize CLIs on. (May also work on Linux, but not tested.)"), PowerShell execution-policy changes, admin rights for first install, and a licence activation that "may open a web browser".
- **Licence:** proprietary, trial-gated. Pricing not published.
- **Repo:** the sample project [red-gate/TDM-AutoPilot](https://github.com/red-gate/TDM-AutoPilot) is MIT, created 2025-01-13, last pushed 2025-06-24, 0 stars, 0 open issues.

---

## fixturize (2026)

**Repo:** [boringSQL/fixturize](https://github.com/boringSQL/fixturize) · **Site:** [boringsql.com/products/fixturize](https://boringsql.com/products/fixturize/)

The most philosophically similar open-source project to lazysnap, and it is seven months old.

- **Language:** Go. **Databases:** PostgreSQL only.
- **Subsetting:** four verbs — `extract`, `apply`, `inspect`, `analyze`. `--root "organizations WHERE id = 42"` takes any SQL fragment after the table name (`WHERE`, `ORDER BY`, `LIMIT`). It "follows foreign keys in both directions (parents and children)... Supports composite FKs, self-referencing tables, and circular dependencies". `--limit` caps child tables, `--filter "orders=status='completed'"` adds a per-table predicate, `--include` pulls whole lookup tables, `--exclude` skips tables and **warns if an excluded table is an FK parent** ([README](https://github.com/boringSQL/fixturize)).
- **Cycle handling:** "Circular dependencies handled automatically via deferred constraints" on load; inserts are topologically sorted ([product page](https://boringsql.com/products/fixturize/)).
- **PII detection:** `fixturize analyze` scans column names and types across 30+ PII categories with confidence scoring and prints **ready-to-paste `--mask` expressions**. Detection uses word-level name matching (`user_email` matches, `emailed_at` does not) combined with type checking, and auto-excludes boolean, timestamp, integer, PK and FK columns to cut false positives.
- **Masking config:** SQL expressions evaluated in the extraction `SELECT`, so the fixture never contains real values: `--mask "auth.users.email='user_' || id || '@test.com'"`. Masks are recorded in the fixture metadata as an audit trail. Defaults can live in `.fixturize.yaml`.
- **Install:** `go install github.com/boringSQL/fixturize/cmd/fixturize@latest`, or `go build` from a clone. **No prebuilt binaries and no releases** — you need a Go toolchain.
- **Licence:** BSD-2-Clause. **Created:** 2026-02-05. **Last commit:** 2026-05-23. **Stars:** 7. **Open issues:** 4.

**Three open issues** (all +0):

1. [#1 Support limit-per-parent](https://github.com/boringSQL/fixturize/issues/1) — 2026-02-22.
2. [#2 JSONB masking support](https://github.com/boringSQL/fixturize/issues/2) — 2026-02-22.
3. [#4 Deterministic fixture output](https://github.com/boringSQL/fixturize/issues/4) — 2026-05-09.

Issue #4 is the important one: **fixturize does not have deterministic masking yet.** Masking with `'user_' || id || '@test.com'` is deterministic per-row but there is no cross-table consistency guarantee for a hashed identifier. That is a gap we can be better at from day one.

---

## pgEdge Anonymizer (2025)

**Repo:** [pgEdge/pgedge-anonymizer](https://github.com/pgEdge/pgedge-anonymizer)

- **Language:** Go. **Databases:** PostgreSQL.
- **Subsetting: none.** It anonymises in place, in a single transaction, on a database you already copied.
- **Masking config:** a `pgedge-anonymizer.yaml` with a `database` section and a `columns` list naming every column as `schema.table.column` plus a pattern. 100+ built-in patterns across 19 countries, consistent replacement within a run, FK-CASCADE awareness, server-side cursors for large tables ([README](https://github.com/pgEdge/pgedge-anonymizer)).
- **PII detection:** none. You enumerate every column by hand.
- **Licence:** the PostgreSQL licence. **Created:** 2025-11-27. **Last commit:** 2026-08-24. **Stars:** 30. **Open issues:** 0.
- Show HN twice, to near-silence: [16 December 2025](https://news.ycombinator.com/item?id=46290635) (1 point, 0 comments) and [24 December 2025](https://news.ycombinator.com/item?id=46376812) (3 points, 0 comments).

---

## DBSnapper

**Site:** [dbsnapper.com](https://dbsnapper.com/) · **Docs:** [docs.dbsnapper.com](https://docs.dbsnapper.com/latest/)

- **Language:** the agent is a closed-source binary; the public repo `dbsnapper/dbsnapper` is the documentation site (HTML/MkDocs, 9 stars, latest tag v2.9.0 on 2025-07-23). Docs describe a v3.0 with an MCP server for AI assistants.
- **Databases:** PostgreSQL and MySQL, local or via Docker engines (`pgdocker://`, `mydocker://`).
- **Subsetting:** four phases — analyse the FK graph, process subset tables by `percent` or `where`, process **upstream** tables (their word for tables holding the FK), transfer whole `copy_tables`, then process **downstream** tables ([Subset introduction](https://github.com/dbsnapper/dbsnapper/blob/main/docs/subset/introduction.md)).
- **Cycle handling: fails and makes you fix it.** "If a circular reference is detected, the subsetting process will stop and an error will be generated. You must then identify where the circular reference should be broken and exclude the relationship." Excluded FK values are set to `NULL`. Self-referencing tables count as cycles.
- **Masking config:** **a hand-written SQL file.** `sanitize` locates a snapshot, **loads the full unsanitised snapshot into a destination database**, runs your `query_file` against it, then re-snapshots ([Sanitize introduction](https://github.com/dbsnapper/dbsnapper/blob/main/docs/sanitize/introduction.md)). Real data therefore lands in a real database before masking. Their own docs say "In the near future we will be adding more capabilities to the sanitization process".
- **Config:** a single `~/.config/dbsnapper/dbsnapper.yml` created by `dbsnapper config init`, holding an auth token, a secret key, and per-target `snapshot` / `sanitize` / `subset` blocks including `subset_tables`, `copy_tables`, `excluded_tables`, `added_relationships`, `excluded_relationships`.
- **Install:** Homebrew, Docker image `ghcr.io/dbsnapper/dbsnapper`, `.deb`/`.rpm`/`.apk`, or GitHub release binaries. Requires database client tools present. Cloud account needed for sharing and storage profiles.
- **Licence / pricing:** proprietary. Starter free; Pro $300/month (100 users); Enterprise $500/month (200 users) ([dbsnapper.com](https://dbsnapper.com/)).

---

## msg555/subsetter

**Repo:** [msg555/subsetter](https://github.com/msg555/subsetter)

- **Language:** Python, on PyPI (`pip install subsetter`) and Docker Hub. **Databases:** MySQL, PostgreSQL, SQLite.
- Explicitly positions against condenser: "This is meant to be a simple CLI tool that overcomes many of the difficulties in using `condenser`."
- **Subsetting:** two-phase — `subsetter plan > plan.yaml` produces an editable sampling syntax tree, then `subsetter sample --plan plan.yaml --create --truncate` executes it; `subsetter subset` does both. You can hand-edit or hand-write SQL into the plan.
- **Cycle handling: it cannot.** "The subsetter tool takes an approach of 'one table, one query'... It cannot support calculating a full transitive closure of foreign key relationships for schemas that contain cycles. In general, as long as your schema contains no foreign key cycles and no target is reachable from another target, the subsetter will be able to automatically generate a plan."
- **Masking:** built-in filters backed by [faker](https://faker.readthedocs.io/) for name, email, phone, address, location, plus custom filter plugins. Also does **identifier compaction** (renumber sampled PKs 1..N and rewrite every referencing FK) and **merge mode** (shift PKs above the destination's existing max). Neither of those exists anywhere else in this field and both are genuinely clever.
- **Config:** [`subsetter.example.yaml`](https://github.com/msg555/subsetter/blob/main/subsetter.example.yaml) is **337 lines**.
- **Licence:** BSD-3-Clause. **Last commit:** 2026-08-24. **Stars:** 10. **Open issues:** 0.

---

## Seedfast (2025)

**Site:** [seedfa.st](https://seedfa.st/)

Not a competitor — a category-adjacent tool that positions itself as the [Neosync alternative](https://seedfa.st/blog/neosync-alternative) and the [Snaplet Seed alternative](https://seedfa.st/compare/snaplet-seed-alternative). It reads your live schema and generates **synthetic** data from plain-English descriptions; the planning step runs server-side. Distributed via `brew install seedfast-ai/tap/seedfast` or `npm install -g seedfast`, closed source, credit-based pricing with a free tier. It explicitly does **not** subset and does **not** anonymise: "Seedfast does not anonymize — generating from schema is a different workflow." Worth tracking because it is capturing the "our seed data drifted" half of our user's pain with a completely different answer.

---

## VeilStream (2025)

[Show HN, 18 June 2025, 22 points](https://news.ycombinator.com/item?id=44310026) · [veilstream.com](https://www.veilstream.com/)

A PostgreSQL **proxy** that masks on the fly rather than producing a snapshot. Closed source ("Currently we don't have much publicly on the GitHub page," per the founder in-thread). Its limitations as of launch, from the founder's own answers: no connection pooling (a fresh connection per query), no PostGIS/TimescaleDB masking, no UUID masking, no array masking, partial `jsonb` support, and conditional masking rules that can only be `AND`ed. Different architecture, same anxiety.

---

## Also in the field, briefly

- **[Xata](https://xata.io/)** — a Postgres platform with copy-on-write branching and anonymisation, launched May 2025; it [acquired Privacy Dynamics](https://xata.io/blog/xata-acquires-privacy-dynamics) and says it "will begin open sourcing the Privacy Dynamics anonymization engine" over the following months. Watch this: an open-source anonymisation engine from a funded company would change the field.
- **[evilmartians/evil-seed](https://github.com/evilmartians/evil-seed)** — Ruby gem, MIT, 567 stars, last commit 2026-07-24. "Partial anonymized dumps of your database using your app model relations" — traverses ActiveRecord associations rather than database FKs. Rails-only, but the most-starred *maintained* subsetter after Jailer.
- **[nixys/nxs-data-anonymizer](https://github.com/nixys/nxs-data-anonymizer)** — Go, Apache-2.0, 294 stars, last commit 2025-09-22. Anonymises a *dump stream* for PostgreSQL and MySQL. No subsetting.
- **[bluerogue251/DBSubsetter](https://github.com/bluerogue251/DBSubsetter)** — Scala, MIT, 16 stars, last commit 2022-10-15. Dead.
- **[sqlsizer/sqlsizer-mssql](https://github.com/sqlsizer/sqlsizer-mssql)** — PowerShell, MIT, SQL Server/Azure SQL only, 1 star.
- **[CuriousLearner/django-postgres-anonymizer](https://github.com/CuriousLearner/django-postgres-anonymizer)** — Django integration for PostgreSQL Anonymizer, BSD-3, 29 stars, created 2025-09-20, last commit 2025-10-05.
- **[rap2hpoutre/pg-anonymizer](https://github.com/rap2hpoutre/pg-anonymizer)** — Node CLI that anonymises a `pg_dump`, 239 stars, last commit 2024-08-30.
- **Datanymizer** — repeatedly cited by developers in the [Neosync HN thread](https://news.ycombinator.com/item?id=40443927) as "now unmaintained". Not independently verified here.

---

## Time from install to first usable snapshot

"Usable" means: a referentially complete subset of a real 40-ish-table schema, with personal data masked, sitting in a local database. Estimated by reading each quickstart end to end, not by running them — these are reading estimates and should be validated by an actual timed run before we quote them anywhere.

"Config lines" counts lines the user must have in a config file before the first successful run, distinguishing lines *written by hand* from lines *generated by the tool*.

| Tool | Install | Account needed? | Config lines before first run | Est. time to first usable snapshot | What eats the time |
|---|---|---|---|---|---|
| **pg_sample** | apt + clone | no | **0** | **5–15 min** | Perl/DBD::Pg deps. But it does not mask, so the result is not "usable" under our definition — it never gets there. |
| **Basecut** | `brew install` | **yes — `basecut login`, browser** | ~20, **all generated** by `basecut init` | **10–20 min** (they claim "under 5") | Account creation and browser auth |
| **fixturize** | `go install` (needs Go) | no | **0** — everything is flags; `.fixturize.yaml` optional | **15–30 min** | Installing Go; running `analyze` and pasting its `--mask` expressions back in by hand |
| **db-condenser** | `uv` + clone | no | ~25 JSON, hand-written | 30–60 min | No masking at all — you never reach "usable" |
| **Greenmask** | `brew`/install script | no | **~18 minimum**, hand-written; realistically 100+ for 17 PII columns | **45–90 min** | Matching `pg_bin_path` to the destination major version; hand-writing one transformer block per PII column; adding `subset_conds` per table |
| **DBSnapper** | `brew` | yes, for cloud features | ~20 YAML hand-written **plus a hand-written SQL sanitize file** | 45–90 min | Writing the sanitisation SQL from scratch |
| **condenser** | pip + clone | no | **27** JSON (their own example) plus `dependency_breaks` you must derive | 45–90 min | Discovering your FK cycles by trial and error; no masking, so never fully "usable" |
| **msg555/subsetter** | `pip install` | no | **337-line example** to cut down, plus a generated `plan.yaml` | 60–90 min | Understanding the plan syntax tree; fails outright on cyclic schemas |
| **Replibyte** | binary | no (S3 optional) | ~25 YAML hand-written incl. a datastore | 60–90 min | Configuring a datastore you did not want; per-column transformers; docs section on subset strategy reads "TODO" |
| **PostgreSQL Anonymizer** | package/Docker + `CREATE EXTENSION` | no | **~1 SQL statement per masked column** (17 columns ⇒ 17 statements) plus 4 role statements | 60–120 min | Superuser install; writing SECURITY LABELs; **no subsetting at all**, so you still need a second tool |
| **Jailer** | download zip; unzip for CLI | no | an extraction model **plus one restriction per over-collecting association** | **60–120 min** | GUI-first workflow; discovering that the default export drags in half the database and hand-restricting associations ([issue #126](https://github.com/Wisser/Jailer/issues/126)) |
| **Snaplet Snapshot** | `npx` | no | `snaplet.config.ts`, generated by `setup` | 20 min **or never** | Native npm build failures on ARM Macs are two of its four open issues; and it is archived |
| **pgsubset** | rustup + `cargo install` | no | ~5 TOML | 30–45 min | Installing a Rust toolchain; no masking |
| **pgEdge Anonymizer** | Go binary | no | **~3 lines per column**, hand-written; no detection | 45–75 min | Enumerating every PII column by hand; no subsetting |
| **Redgate TDM** | Windows installer + licence activation | **yes — licence/trial activation, browser** | **41-line JSON** options file plus generated `classification.json` and `masking.json` | **2–4 hours** | Windows requirement, PowerShell execution policy, admin rights, four separate CLI invocations |
| **Tonic Structural** | SaaS or Docker Compose/K8s | **yes — sales contact, no public price** | web UI, not files | **days** | Procurement. No self-serve tier for Structural. |
| **Neosync** | clone + `make compose/up` | no | web UI | n/a — **archived** | Docker Compose stack with a Temporal dependency |

**The honest read of that table:** nothing that masks *and* subsets *and* detects PII gets a first-timer to a usable local database in under ten minutes without an account. Basecut is closest and charges for it. fixturize is closest among open source and makes you install Go and paste mask expressions by hand.

---

## What nobody does well

Six gaps, each with evidence.

### 1. The default subset is wrong, and every tool makes you fix it by hand

Jailer's own tutorial teaches this as a lesson: export employee SCOTT and you also get every employee in his department and every employee on his salary grade, until you hand-write five restrictions ([Exporting Data](https://wisser.github.io/Jailer/exporting-data.htm)). A user filed it as a bug in 2025 — [issue #126](https://github.com/Wisser/Jailer/issues/126), "Subject condition is violated due to reverse traversal": with `T.customer_id = 1` they got orders for customers 2 through 10, and observed that "This happens due to implicit reverse traversal, which is not documented clearly / Bypasses the subject condition unless explicitly restricted." It took eight months to close.

Redgate ships the same trap with a different default: `includeTablesRowThreshold` is 300, so tables under that size are copied whole, and their own worked example warns the behaviour "can seem to perform unexpectedly" ([TDM-AutoPilot README](https://github.com/red-gate/TDM-AutoPilot#subsetting-options-file-rgsubset-options-northwindjson-description)). condenser describes its own traversal as one that "greedily grabs as many rows from the database as it can" and offers `upstream_filters` as the escape hatch ([README](https://github.com/TonicAI/condenser#config)).

**Nobody explains the plan before executing it.** No tool in this survey shows you "these 23 tables, ~18k rows, here is why each table is in the slice" and lets you correct it before a single row moves.

### 2. Cycles are a cliff, not a feature

- condenser: "The subsetting tool cannot operate on databases with cycles in their foreign key relationships... You'll have to know a bit about your database to use this field effectively."
- msg555/subsetter: "It cannot support calculating a full transitive closure of foreign key relationships for schemas that contain cycles."
- DBSnapper: "If a circular reference is detected, the subsetting process will stop and an error will be generated. You must then identify where the circular reference should be broken."
- Tonic Structural: nulls the minimum number of nullable FKs, and **"If no FK values are `NULL`-able, subset generation fails"**.
- Greenmask does handle cycles with recursive queries — and then breaks the other way: [issue #329, "greenmask does not work if there is not cycle in the database"](https://github.com/GreenmaskIO/greenmask/issues/329), 18 comments, open since August 2025, where the SCC component code errors on an acyclic graph.

### 3. Masking is opt-in, per-column, and hand-written — so it silently rots

Greenmask, Replibyte and pgEdge Anonymizer all require you to name every column. None of the three detects anything. A security practitioner made the consequence explicit in the [Replibyte HN thread](https://news.ycombinator.com/item?id=31165538): "people use them properly, and then a database schema changes, but people don't update their script... sensitive data has made its way into staging." Another commenter in the same thread asked for exactly the missing feature: automatic identification of sensitive columns rather than manual configuration for databases with many tables.

The tools that *do* detect are candid about the limits. PostgreSQL Anonymizer's docs on `anon.detect()` name both false positives and false negatives and insist "you still need to review the entire database model in search of hidden identifiers" ([detection docs](https://postgresql-anonymizer.readthedocs.io/en/latest/detection/)). fixturize's detector prints confidence levels and excludes boolean/timestamp/integer/PK/FK columns to cut noise — and still asks you to paste the results back in as flags.

And several tools ship a switch that turns it all off: Basecut has `anonymize: off` ([Anonymization](https://docs.basecut.dev/configuration/anonymization.md)).

### 4. Masking happens too late, in a real database, on real rows

DBSnapper's `sanitize` "load[s] the snapshot into the `dst_url` database" and *then* "appl[ies] the sanitization query" ([Sanitize introduction](https://github.com/dbsnapper/dbsnapper/blob/main/docs/sanitize/introduction.md)). The unmasked production rows exist in a live database on the way through. PostgreSQL Anonymizer's static masking has the same shape by construction — it anonymises a copy you already made.

fixturize and Basecut get this right, masking inside the extraction `SELECT` so the artefact never contains real values. That should be table stakes, and mostly is not.

### 5. Install is where people actually give up

Two of the four open issues on Snaplet Snapshot are the npm install failing — [#15](https://github.com/supabase-community/snapshot/issues/15) and [#20 "NPM Install Failing related to libtool static on ARM Darwin"](https://github.com/supabase-community/snapshot/issues/20). The single most-upvoted issue on pg_sample, [#19](https://github.com/mla/pg_sample/issues/19), has a two-word body — "How do I install this?" — and has been open since 2020. condenser's [#21 "Bundling as package?"](https://github.com/TonicAI/condenser/issues/21) has been open since 2021; you still clone a repo and run `python direct_subset.py`. fixturize has no release binaries at all. Greenmask makes you match your local `pg_dump` major version to the destination server's.

### 6. The commercially-backed tools keep dying, and the surviving free ones are one-maintainer projects

Neosync — 4,141 stars, YC-backed, 246-point Show HN — was archived on 30 August 2025 after a [Grow Therapy acqui-hire](https://www.crunchbase.com/acquisition/grow-therapy-acquires-neosync-cd81--00632527); the [press release](https://www.prnewswire.com/news-releases/grow-therapy-raises-the-privacy-bar-in-mental-health-302567153.html) does not mention the open-source project or the hosted service at all. Snaplet shut down and its snapshot repo is archived. Replibyte — 4,409 stars — last shipped a release in October 2022. Meanwhile Jailer has zero open issues because one person closes all of them, and pg_sample's busiest thread is a ten-year-old error report with 63 comments.

There is a real, repeated, well-upvoted demand here and no durable free answer to it. That is the opportunity, and it is also the warning: this is a hard problem that has eaten several funded teams.

---

## What we must match to be credible

Ranked. Items 1–7 are the credibility floor: ship without them and an informed reviewer dismisses lazysnap in one paragraph.

1. **Bidirectional FK traversal with parents to completeness and children capped.** Greenmask, Basecut, Tonic, DBSnapper and fixturize all do both directions. A one-direction subsetter is not in the conversation. CONCEPT.md already specifies this — hold the line.

2. **Cycles must not stop the run.** condenser, DBSnapper and msg555/subsetter all error out and hand the problem back; Tonic fails outright when no FK in the cycle is nullable. Greenmask handles cycles and then [breaks on acyclic schemas](https://github.com/GreenmaskIO/greenmask/issues/329). Handle both cases, and say in the plan output which edge was deferred or nulled and why.

3. **Verified referential integrity in the target, reported.** Basecut advertises "zero broken references" and prints "Verified referential integrity"; nobody else proves it. CONCEPT.md's `✓ foreign keys verified` line is a differentiator only if it is a real post-load check, not a claim.

4. **PII detection that says why, with confidence.** fixturize prints category and confidence per column; PostgreSQL Anonymizer's `anon.detect` returns an identifier category and a HIPAA classification. Our `17 columns look like personal data (press ? to see why)` must be at least as good and must state its false-negative risk out loud, the way [the anon docs do](https://postgresql-anonymizer.readthedocs.io/en/latest/detection/).

5. **Deterministic masking that survives joins.** Greenmask has a hash engine; Basecut has a `hash` strategy and `email_preserve_domain`; fixturize does **not** have it yet ([issue #4](https://github.com/boringSQL/fixturize/issues/4)). Same input, same output, across tables and across runs, or foreign keys on masked natural keys break.

6. **Mask during extraction, never after load.** fixturize masks in the `SELECT`; Basecut masks during extraction; DBSnapper and PostgreSQL Anonymizer's static masking do not. Real rows must never touch the target.

7. **One-line install of a single static binary, no account, no language toolchain.** Greenmask (`brew install greenmask`) and Basecut (`brew install basecuthq/cli/basecut`) clear this; fixturize (`go install`), condenser (clone + pip), pgsubset (rustup) and Snaplet (npm native builds) do not. Basecut then spends its advantage on `basecut login`. **Not requiring an account is our sharpest wedge against the only tool that otherwise matches us.**

Then, to be *better* rather than merely credible:

8. **Show the plan before moving a row.** Nobody does this. "23 tables in slice, ~18k rows" with a per-table reason, correctable before execution, directly answers Jailer #126 and the condenser greedy-traversal complaint.

9. **Zero hand-written config on the first run.** Basecut generates ~20 lines with `init` but demands an account first; every open-source tool demands hand-written config, from Greenmask's ~18 lines to msg555/subsetter's 337-line example. Emitting `lazysnap.yml` *after* the run, as CONCEPT.md specifies, is the correct inversion and nobody else has made it.

10. **Reset sequences after load.** condenser [#14](https://github.com/TonicAI/condenser/issues/14) has been open since 2019; db-condenser lists "Automatic sequence reset after subsetting" as a headline improvement over its parent. A subset whose sequences all start at 1 is not usable and users will not tell us politely.

11. **Root selection by a real predicate, not a percentage.** Replibyte [#74](https://github.com/Qovery/Replibyte/issues/74), the most-upvoted open issue in the field at +18, asks for `WHERE`-clause subsetting and was never built; pg_sample [#44](https://github.com/mla/pg_sample/issues/44) asks the same thing. "This customer and everything they touch" is the actual job.

12. **Virtual / implicit foreign keys.** condenser (`fk_augmentation`), DBSnapper (`added_relationships`), Redgate (`manualRelationships`), Basecut (`virtual_foreign_keys`) and Greenmask (virtual references with `polymorphic_exprs`) all support declaring FKs the database does not know about. Any schema with polymorphic associations — every Rails app, most Django apps — needs this, and it is the one place where post-run config is legitimately required.

13. **Refuse to ship what they shipped.** No `--no-mask`. Basecut's `anonymize: off` and Tonic's per-workspace toggles exist because enterprises asked; our differentiator is that we say no, and say why, in the tool's own output.

### What we should deliberately not chase

- **Database breadth.** Jailer claims 24+ engines, Tonic claims a dozen, and both are worse at PostgreSQL than a PostgreSQL-only tool can be. Greenmask has had [MySQL support open since October 2024](https://github.com/GreenmaskIO/greenmask/issues/222) with 33 comments and no release. CONCEPT.md's non-goal list is correct.
- **Synthetic generation from schema alone.** Seedfast and Snaplet Seed occupy that niche; it is a different workflow with different failure modes.
- **A hosted service or sharing layer.** That is where Basecut, DBSnapper and Tonic make money and where Snaplet and Neosync died. Stay a binary.
