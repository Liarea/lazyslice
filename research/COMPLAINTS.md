# COMPLAINTS

What people actually say, in their own words, about getting realistic data into a local or CI database. Every quote below is verbatim from a public source, with a link and a date.

Compiled 2026-09-05.

## Method and caveats

- **Where I looked.** Hacker News via the Algolia API (comment search plus full thread fetches, so replies are read in context); GitHub issues on [Greenmask](https://github.com/GreenmaskIO/greenmask/issues), [Neosync](https://github.com/nucleuscloud/neosync/issues), [Replibyte](https://github.com/Qovery/Replibyte/issues), [Jailer](https://github.com/Wisser/Jailer/issues), [Snaplet seed](https://github.com/supabase-community/seed/issues) and [dbslice](https://github.com/nabroleonx/dbslice/issues); GitLab issues on [PostgreSQL Anonymizer](https://gitlab.com/dalibo/postgresql_anonymizer) (which lives on GitLab, not GitHub) and on [gitlab-org/gitlab](https://gitlab.com/gitlab-org/gitlab) itself; Stack Overflow, Software Engineering SE and DBA SE via the Stack Exchange API; dev.to via its articles API; and vendor engineering blogs.
- **Every quote was re-fetched today.** Each HN comment was pulled by item id from `hn.algolia.com/api/v1/items/<id>`, each GitHub issue through the GitHub API, each GitLab issue through `gitlab.com/api/v4`, each Stack Exchange question through `api.stackexchange.com`. Nothing here is quoted from memory. Where a quote is a paraphrase or a second-hand report, it says so. (`news.ycombinator.com` returns HTTP 429 to scripted fetches from this environment, so the HN links were verified by item id through the Algolia API — which returns the author, timestamp and text for each id — rather than by fetching the pages. Stack Exchange pages likewise 403 to scripts and were verified through `api.stackexchange.com`.)
- **Where I could not look.** Reddit is not fetchable from this environment. One entry (TR-17) is a Reddit comment reproduced second-hand in a vendor's dev.to post; it is flagged as such and is weaker evidence than the rest.
- **Vendor and maintainer bias.** Sources written by someone selling a competing tool are marked **[vendor]**; sources written by a tool's own maintainer inside their own tracker are marked **[maintainer]**. Both are still useful — a maintainer writing "people often say X" is reporting user demand — but they are not neutral, and the frequency table breaks them out so the ranking can be re-checked without them.
- **Counting rule.** One numbered entry is one (source, theme) pair: one person, issue or post raising one theme. Where a single comment raises two themes, it is counted in both and the overlap is noted; where the same comment is merely useful context in a second theme, it appears as an unnumbered cross-reference and is not counted. Counts are reproducible from the file: `grep -cE '^\*\*FK-[0-9]+\*\*' COMPLAINTS.md`, and so on.
- **This is a convenience sample, not a survey.** It ranks how often a theme appears *in material I could reach*, weighted toward places where frustrated people write things down. Bug trackers over-represent crashes and under-represent "I gave up before filing anything". Treat the ordering as a strong hint, not a measurement.

## Tool state as of 2026-09-05

Context for the trust quotes. Verified today against the repos and release APIs, not from memory.

| Tool | State | Evidence |
|---|---|---|
| Greenmask | Active. v0.2.23 released 2026-08-22; last push 2026-08-25; 1,757 stars | [releases](https://github.com/GreenmaskIO/greenmask/releases/tag/v0.2.23) |
| PostgreSQL Anonymizer (Dalibo) | Active on GitLab. 3.1.3 released 2026-06-29; "Release 3.2" issue opened 2026-09-04, closed 2026-09-05 (today) | [repo](https://gitlab.com/dalibo/postgresql_anonymizer), [issue #674](https://gitlab.com/dalibo/postgresql_anonymizer/-/issues/674) |
| Jailer | Active. v17.2.2 released 2026-08-19; last push 2026-09-04; 3,195 stars | [releases](https://github.com/Wisser/Jailer/releases/tag/v17.2.2) |
| dbslice | New and quiet. Created 2026-02-15; last push 2026-06-23; 143 stars | [repo](https://github.com/nabroleonx/dbslice) |
| Datanymizer | Semi-dormant. No tagged release since v0.7.4; last commit 2026-03-11; 571 stars | [repo](https://github.com/datanymizer/datanymizer) |
| Neosync | **Archived** 2025-08-30 after acquisition. Last release v0.5.41, 2025-07-11; 4,141 stars | [repo](https://github.com/nucleuscloud/neosync), [issue #3565](https://github.com/nucleuscloud/neosync/issues/3565) |
| Replibyte | **Effectively dead.** Last release v0.10.0, 2022-10-14; last commit 2024-05-04; "Maintained?" issue open since 2023; 4,409 stars | [releases](https://github.com/Qovery/Replibyte/releases/tag/v0.10.0), [issue #277](https://github.com/Qovery/Replibyte/issues/277) |
| Snaplet snapshot | **Archived.** Company closed 2024; repo moved to `supabase-community/snapshot`, archived | [repo](https://github.com/supabase-community/snapshot), [Supabase post](https://supabase.com/blog/snaplet-is-now-open-source) |
| Snaplet seed | Drifting. Last release v0.98.0, 2024-07-30; 790 stars | [releases](https://github.com/supabase-community/seed/releases/tag/v0.98.0) |

Three of the nine tools above are archived or dead, and two more have not cut a release in over a year. That fact is itself the loudest complaint in the corpus, and it is the backdrop to every quote in Theme 4.

The five tools below are quoted for evidence elsewhere in this document (Themes 1, 5 and 6) but were not part of the original state-verification pass. Checked now, for the same reason as the table above:

| Tool | State | Evidence |
|---|---|---|
| VeilStream | Active, small. Postgres-proxy Docker image last pushed 2026-01-08 (tag `2026.01.08-002`); site live | [Docker Hub](https://hub.docker.com/r/veilstream/veilstream-postgres-proxy/tags), [veilstream.com](https://www.veilstream.com/) |
| GoMask | Active, commercial. Live product at gomask.ai with a free tier; [Product Hunt launch](https://www.producthunt.com/products/gomask-ai) March 2026; blog posts continuing through 2026 | [gomask.ai](https://gomask.ai/) |
| Xata / pgstream | Active. pgstream reached v1.0.0 in February 2026 (stateless schema-change replication) and [v1.4.2](https://github.com/xataio/pgstream/releases/tag/v1.4.2) on 2026-09-02; 1,186 stars; last push 2026-09-05. Xata open-sourced its core platform, folding pgstream in, in April 2026 | [repo](https://github.com/xataio/pgstream), [Xata open-source announcement](https://xata.io/blog/xata-is-now-open-source) |
| Basecut | Active, commercial, small. Proprietary CLI at basecut.dev, $79–99/month with a free tier; site live 2026-09-05 | [basecut.dev](https://basecut.dev/) |
| CloakDB | New and unproven, not comparable to the rest of this table. A single dev.to post's open-source Python script: repo created 2026-08-27, last push 2026-09-03, 1 star. This is one developer's week-old side project, not a maintained tool — its FK-21 quote should be read as "why someone wrote yet another one-off script", not as evidence about a live product | [repo](https://github.com/latryee/CloakDB) |

---

## Context: why they will not just generate fake data

Not counted in the ranking. These quotes establish the demand that every other complaint is downstream of — people are not copying production for fun.

- [Hacker News, 2022-04-28](https://news.ycombinator.com/item?id=31190245), `adamckay`:

  > Once you have data in production it's very possible for the fake data you generate in dev not really matching the sort of data you have in production (either in size or because of assumptions made whilst generating it or even bad app updates/schema migrations in the past causing duff data). It can then be useful for future development or debugging that the data is real(-ish).

- [Hacker News, 2022-04-28](https://news.ycombinator.com/item?id=31188907), `closeparen`, on why fuzzing is not a substitute:

  > Data that is tightly clustered on certain keys and widely dispersed on other keys can hit some "fun" interactions with sharding regimes, indexes, etc. that random data doesn't.

  and:

  > Sometimes you're wrong about the validation rules, i.e. you think you know the allowable enum values here but in fact production systems that really exist and have customers behind them are setting other values. Rejecting those would itself be a bug.

- [Hacker News, 2023-01-12](https://news.ycombinator.com/item?id=34351169), `alexvoda`:

  > Unless you have realistic data in the test database the performance behavior of a query can be unpredictable. SQL is very leaky abstraction.

- [Hacker News, 2022-04-28](https://news.ycombinator.com/item?id=31189176), `pyr0hu`:

  > There are tons of edgecases that only happens with the chaotic blob that we call production data.

- [Hacker News, 2022-04-28](https://news.ycombinator.com/item?id=31188369), `carterschonwald`, who built one of these at JPMorgan, on why the synthetic route is not cheap either:

  > There's a surprising amount of nuance needed just to do a decent job generating schematically valid fake data, let alone stuff that's statistically faithful to true data

---

## Theme 1 — Broken foreign keys and referential integrity

The most-repeated technical failure, and it arrives in three distinct shapes: masking that desynchronises keys, subsetting that drops parents, and cycle handling that panics or silently returns nothing.

**FK-1** — [Hacker News, 2024-05-22](https://news.ycombinator.com/item?id=40446843), `imiric`, on the Neosync Show HN:

> The problem I'm running into is referential integrity, as importing the anonymized data is raising unique and foreign key violations. The obfuscator tool is pretty minimal and has few knobs to tweak its output, so it's difficult to work around this, and I'm considering other options at this point.

He opened the same comment with: *"This topic is relevant to what I'm currently working on, and I'm finding it exhausting to be honest."*

**FK-2** — [Hacker News, 2024-05-22](https://news.ycombinator.com/item?id=40446813), `edrenova`, Neosync co-founder, replying in that thread **[vendor]**:

> yeah the referential integrity and constraints part is usually the most complicated part and everyone does things differently which adds another layer of complexity on it

**FK-3** — [Hacker News, 2026-03-05](https://news.ycombinator.com/item?id=47261678), `nickzelei`, Neosync co-founder, on the dbslice Show HN, writing freely because Neosync is now archived **[vendor]**:

> It was probably the hardest feature to fully solve for the customers that needed it the most, which were the ones with the most complex data sets and foreign key dependencies. To the point where it was almost impossible to do this, at least with syncing it directly to another Postgres database with everything in tact.

**FK-4** — [Hacker News, 2024-10-16](https://news.ycombinator.com/item?id=41865053), `imiric` again, five months after FK-1, having tried the dump-format-agnostic route:

> Unfortunately, the tool is quite barebones, and has issues maintaining referential integrity, so we had to abandon it. This is still an unsolved problem in our team, so I'll keep an eye on your tool.

**FK-5** — [Hacker News, 2025-05-16](https://news.ycombinator.com/item?id=44008738), `tudorg`, answering "Ask HN: Any good tools to pgdump multi tenant database?":

> The problem is most of these tools can walk foreign keys, but only in one directions.

**FK-6** — [Hacker News, 2022-04-28](https://news.ycombinator.com/item?id=31188127), `husainfazel`, reading Replibyte's subsetting config:

> Database Subsetting: Scale down a production database to a more reasonable size What about this? Does the database need foreign keys to prevent related rows in tables being lost and are they just randomly deleting rows as the config seems to indicate

**FK-7** — [Greenmask issue #403, 2026-02-19](https://github.com/GreenmaskIO/greenmask/issues/403), closed same day, a two-table repro:

> The restore operation generates an error because it is not able to recreate the foreign keys between the two tables above. The reason is there are no rows in the *demo_persons* table in the target database while there were rows with ID 3 and 9 in the source. So it seems the subset filter excludes all rows of the *demo_persons* table.

The reporter later found the cause himself — *"The problem is due to a circular reference which is not present in my scripts above. My intention was to simplify the test case. I did not realize that the circular reference was the cause of the issue."* — and the maintainer confirmed the shape of the gap: *"The circular references are not followed by Greenmask. That's why the demo_persons rows are not restored. […] The correct circular references subset implementation will be in the future V1 version."*

**FK-8** — [Greenmask issue #329, 2025-08-13](https://github.com/GreenmaskIO/greenmask/issues/329), open — a subset engine that fails on the *absence* of cycles:

> I was using greenmask, but it fails since there is no cycle on the database tables. It seems the code was written in a way to complain when there is more than one, but in our case we don't have any cycles

A second user, `nazriel`, [confirmed it on 2025-08-20](https://github.com/GreenmaskIO/greenmask/issues/329):

> I hit the panic `get one group cycle group is not allowed for multy cycles` and when building from source and debugging it turns out I have 0 cycles as well.

**FK-9** — [Greenmask issue #279, 2025-03-21](https://github.com/GreenmaskIO/greenmask/issues/279), open — `panic: runtime error: index out of range [0] with length 0` during subset. The maintainer diagnosed a self-reference (*"there's a circular dependency in the `public.posts` table, which references itself twice"*), and a second user, `joeljameswatson`, re-hit it on 2026-04-27 from a different direction:

> I hit this panic on a Postgres schema where a composite FK targets non-PK columns on the parent table (i.e. the FK references a column pair that isn't the parent's primary key). Same `index out of range` symptom as this issue, though the trigger is "FK targets non-PK columns" rather than self-reference specifically.

**FK-10** — [Greenmask issue #396, 2026-02-17](https://github.com/GreenmaskIO/greenmask/issues/396), open — polymorphic references, the Rails/Laravel-shaped schema, silently producing an empty slice:

> When `virtual_references` defines multiple polymorphic references on the same table […] the generated subset query silently drops nearly all rows. Only rows where the FK column is `NULL` survive.

**FK-11** — [Greenmask issue #392, 2026-01-16](https://github.com/GreenmaskIO/greenmask/issues/392), open, the maintainer's own epic **[maintainer]**:

> Greenmask includes a subset system that helps users consistently reduce database size based on specified conditions. However, there are currently limitations that prevent this system from being used effectively in databases with cyclic references.

The task list under it includes *"Research cycle resolution approaches for RDBMSs that support recursive queries"* and *"Implement a new version of the cycle resolver based on recursive queries"*. The most mature open-source tool in this space is rewriting its subset engine in 2026.

**FK-12** — [Greenmask issue #319, 2025-07-21](https://github.com/GreenmaskIO/greenmask/issues/319), open, **[maintainer]**, summarising what users keep reporting:

> People often say that they have legacy DB without primry keys. Need to implement virtual primary keys in order to define some virtual references on the schema where neither primary key nor foreign keys exists.

**FK-13** — [Greenmask issue #270, 2025-02-26](https://github.com/GreenmaskIO/greenmask/issues/270), open, `mikfreedman`, on relationships that do not point at a primary key:

> Users should be able to specify the primary key of the referenced table in a virtual reference so that they can handle non-standard relationships in greenmask.

**FK-14** — [Jailer issue #126, 2025-08-05](https://github.com/Wisser/Jailer/issues/126), closed 2026-04-14 — the subset pulled in strangers:

> When using an ExtractionModel with a subject table (customers) and a condition like: T.customer_id = 1 Jailer includes unrelated rows from the orders table where customer_id != 1. This violates the defined condition and leads to unintended data export, unless all reverse associations are explicitly blocked.

The reporter's own summary of the cause: *"This happens due to implicit reverse traversal, which is not documented clearly"*. A subsetter whose default traversal over-collects is a privacy bug as well as a correctness bug.

**FK-15** — [Jailer issue #120, 2025-03-21](https://github.com/Wisser/Jailer/issues/120), closed the next day, `patricktyndall`, on cycle handling emitting untyped NULLs:

> UUID Cols that are "deferred due to circular dependency" end up getting cast as `null::text`, when initially being set to null, which results in errors like `column "thing_id" is of type uuid but expression is of type text`.

**FK-16** — [Jailer issue #50, 2021-08-19](https://github.com/Wisser/Jailer/issues/50), closed 2022-02-23, `raresboza`, asking whether the advertised cycle handling is real:

> I saw the patch notes on the Jailer site stating that since the 4th of February one can submit a database containing circular references at it will break them. I was very curious as to how this would work, however upon a closer inspection of the code, I could only find the removal of reflexive cycles. Am I missing something?

**FK-17** — [Neosync issue #3227, 2025-02-06](https://github.com/nucleuscloud/neosync/issues/3227), open, `Yarn-e`:

> When subsetting, i have a table that is the source where most of my other tables rely on. But Neosync only recognises tables as root if they comply with the following rule: "This is a Root table that only has foreign key references to children tables."

The maintainer's answer names the design limit **[maintainer]**:

> Foreign Key constraints are effectively directional graphs (DAGs) that follow a parent->child hierarchy. […] If we need to actually filter the parent by the child, the run order doesn't necessarily change, but the query does, and it increases in complexity even further if you have multiple subsets that apply.

**FK-18** — [Neosync issue #3433, 2025-03-28](https://github.com/nucleuscloud/neosync/issues/3433), open, `patricktyndall`, on masking a key that other tables point at:

> Generating UUID for my table resulted in this error in UI / docker worker […] Turning this column back to passthrough fixes the issue.

"Turn the masking off and it works" is the failure mode that produces leaked PII by accident.

**FK-19** — [Snaplet seed issue #203, 2024-11-12](https://github.com/supabase-community/seed/issues/203), open, with a "did you ever find a way around this?" from a second user seven months later:

> I have a table which can reference itself. […] However, with the seed utility, i get the error `AssertionError: Node items forms circular dependency: items -> items` and am unable to proceed. Is this not a supported use case?

**FK-20** — [Stack Overflow, 2010-12-21](https://stackoverflow.com/questions/4504140/populate-tables-with-test-data-whilst-maintaining-relational-integrity) — the same request, sixteen years ago:

> I was going to write a script to populate the tables with test data (10-20k rows or more) but I thought I ought to ask if there's something already out there that can generate test data based on the field types but ensure relational integrity at the same time?

**FK-21** — [dev.to, 2026-08-27](https://dev.to/latryee/how-i-anonymized-relational-sql-dumps-without-breaking-foreign-key-relationships-24jm), `latryee`, explaining why they wrote yet another tool:

> Anonymizing a database dump sounds straightforward until the data is actually relational. You can replace names, emails, phone numbers, and other sensitive values quite easily. The difficult part is keeping the relationships between those values intact.

and:

> we have successfully hidden the original value, but we've also destroyed the relationship.

**FK-22** — [Xata blog, 2026-01-22 (approximate — the page carries no visible publication date and search-engine indexing puts it anywhere from 2026-01-22 to 2026-02-06; the quoted text itself is verbatim)](https://xata.io/blog/anonymization-the-missing-link-in-dev-workflows) **[vendor]**:

> Random masking breaks foreign keys and constraints and makes databases unusable for testing.

**FK-23** — [Hacker News, 2025-10-28](https://news.ycombinator.com/item?id=45732136), GoMask co-founder on his own Show HN **[vendor]**:

> Mock data isn't realistic enough to debug with. Snapshots go stale. Manual masking takes weeks and breaks referential integrity.

---

## Theme 2 — Leaked PII and masking that silently does nothing

The pattern is consistent and alarming: masking fails *quietly*. The command succeeds, the exit code is zero, and the cleartext is in the target.

**PII-1** — [Hacker News, 2022-04-28](https://news.ycombinator.com/item?id=31189199), `onion2k`, the single most-quotable complaint in the corpus:

> Another major problem with tools like replibyte is that people use them properly, and then a database schema changes, but people don't update their script to anonymize new tables or columns. Then a few months later someone notices sensitive data has made its way in to staging, and into the backups, and the database dumps devs made to debug things because "it's only staging data, who cares!"

**PII-2** — [Replibyte issue #269, 2023-03-28](https://github.com/Qovery/Replibyte/issues/269), open, three separate users:

> Hi, i try to anonymize data on dump but when i restore it on my dev environnement i have real data do i miss something ?

`TheKipmaster`, 2023-05-17:

> I have the same issue. Seemingly correctly configured .yml file outputs exactly the same data. No transformation takes place.

`sealed-rayboutotte` found the cause on 2024-01-12, ten months after the report:

> Its the dump format ... if it uses sql statements (i.e. insert) then it should transform the data. But if it uses `COPY public.table (column_a, colum_b) FROM stdin;` **_then data is NOT transformed_**.

A masker whose behaviour depends silently on the dump format is the worst possible failure mode: it looks like it worked.

**PII-3** — [PostgreSQL Anonymizer issue #531, 2025-05-09](https://gitlab.com/dalibo/postgresql_anonymizer/-/issues/531), closed 2025-05-22 — the same class of bug, opposite direction:

> If the "--inserts" option is used with `dump.sh`, no anonymization is done. […] Probably this is because `pg_dump` will use a Cursor instead of the COPY command when "--inserts" is specified.

**PII-4** — [PostgreSQL Anonymizer issue #668, 2026-08-25](https://gitlab.com/dalibo/postgresql_anonymizer/-/issues/668), closed 2026-08-27:

> `anon.anonymize_database_parallel()` fails silently: a perfectly maskable table in the same worker is left in cleartext, and the caller cannot tell the run was incomplete.

and:

> `t1` stays fully in cleartext: the worker's single transaction is rolled back by the `mv1` failure, undoing t1's masking too. Nothing is reported to the client; the failure only reaches the server log

**PII-5** — [PostgreSQL Anonymizer issue #670, 2026-08-26](https://gitlab.com/dalibo/postgresql_anonymizer/-/issues/670), open — the follow-up, because the first fix only covered the cases known before launch:

> A worker masks its tables in one transaction and the coordinator joins with `wait_for_shutdown()`, which returns `Ok(())` as soon as the worker process is gone — whether it exited cleanly or aborted.

**PII-6** — [Hacker News, 2026-04-20](https://news.ycombinator.com/item?id=47833744), `e7h4nz`, on why his team abandoned per-PR branches of the production database. This is the sharpest statement of the problem in the corpus:

> In principle you can define masking rules for sensitive columns, but in practice it's very hard to build a process that guarantees every new column, table, or JSON field added by any engineer is covered before it ever touches a branch. The rules drift, reviews miss things, and nothing in the workflow hard-fails when a new sensitive field slips through. Most of the time that's fine. But "most of the time" isn't the bar for customer data — a single oversight leaking PII into a developer environment is enough to do real damage to trust, and you can't un-leak it. Until masking can be enforced by construction rather than by convention, we'd rather pay the cost of synthetic data than accept that risk.

**PII-7** — [Hacker News, 2022-04-28](https://news.ycombinator.com/item?id=31190464), `tjpnz`, reviewing Replibyte's transformers:

> There's a transformer which appears to retain the first char on string fields. That's not safe if you're dealing with customer data.

Elaborated [the next day](https://news.ycombinator.com/item?id=31201089):

> It's not safe because I could potentially use that information to find a real customer in the DB. It becomes more problematic when working with data from Asian countries where it's possible (even common) for family and/or first names to consist of two or even a single character.

**PII-8** — [Hacker News, 2022-04-28](https://news.ycombinator.com/item?id=31190848), `micheljansen`, on why column-level masking is not sufficient under GDPR:

> In practice, this means that any realistic production-derived data is either very likely to be still considered PII (and therefore much more demanding to handle safely and securely) or has to be mangled so much that it is no longer representative of production data.

**PII-9** — [Hacker News, 2024-05-22](https://news.ycombinator.com/item?id=40445047), `blopker`, on the Neosync launch:

> I don't know exactly how this works, but I wanted to share my experience trying to anonymize data. Don't. While you may be able to change or delete obvious PII, like names, every bit of real data in aggregate leads to revealing someone's identity.

and, [in reply](https://news.ycombinator.com/item?id=40445417), on the operational tail:

> Once production data is floating around different environments, it will be easy to lose track of. Then the first GDPR delete request comes in. Was this data synthetic? Was it real? I think Joe has a copy on his laptop, he's on vacation? It gets messy.

**PII-10** — [Hacker News, 2022-04-28](https://news.ycombinator.com/item?id=31188966), `jlgaddis`:

> I'm (not) looking forward to the future data breach notifications / post-mortems that include something like "... our developers used a tool to copy the production database to a dev database on their laptop ..."
>
> Honestly, I'm kinda surprised by the lack of comments advocating against doing this.

**PII-11** — [Hacker News, 2014-08-02](https://news.ycombinator.com/item?id=8123705), `jzwinck`, writing about the MDN database disclosure — an actual leak with exactly this cause. He named the fix twelve years ago:

> But a better solution would be to write a system which makes it much less likely to leak private data. For example by copying only whitelisted columns (so if new sensitive columns are added to the system they are not dumped by default).

**PII-12** — [Hacker News, 2022-04-28](https://news.ycombinator.com/item?id=31187911), `gregwebs`, saying the same thing four years later as a purchase condition:

> I couldn't see myself using this unless there was a mode where only allowed fields are copied and non-id fields are first transformed in a lossy way.

**PII-13** — [Hacker News, 2019-03-23](https://news.ycombinator.com/item?id=19469128), `Raed667`, answering a "dumbest thing you've done on the job" thread:

> While inbording, i have been given shell script that is supposed to take a copy of the data in production, anonymize it, and set it up locally on my machine. It took 2 parameters, the first one is the IP address of the production database and the second one my IP address. I guess it was inevitable for those to get mixed up, and I guess no one did bother to prevent write access to production.

**PII-14** — [Hacker News, 2022-04-28](https://news.ycombinator.com/item?id=31189034), `onion2k`, on what a tool in this category owes its users:

> This project needs a giant heading box in the README stating 3 things; - staging databases that hold data generated from production databases should be considered production data, with the same level of consideration for security and access as production. - staging databases that hold production data are a GDPR violation waiting to happen.

When the maintainer answered that auto-detection of sensitive data and schema-change detection were planned, [the reply](https://news.ycombinator.com/item?id=31189231) (`onion2k`, 2022-04-28) was: *"Those are great features to have but they're not in the app yet."*

**PII-15** — [dev.to, 2026-03-24](https://dev.to/jakelaz/how-to-anonymize-pii-in-postgresql-for-development-hb2), Jake Laz **[vendor]**, listing the failure modes of the post-restore `UPDATE` script that most teams actually run:

> - Someone forgets to run the script, and real data ends up in a dev environment anyway.
> - The script is not versioned with the schema, so it breaks when new PII columns are added.
> - It replaces data inconsistently — the same customer gets a different fake email in `users` than in `audit_logs`, breaking join-based queries.
> - It runs after the fact, which means real data has already traveled through the restore pipeline.
> - It has no automated detection — every new PII column has to be added manually.

The same post, on why a hand-written column list is never complete:

> But in real production schemas, PII hides in less obvious places:
> - free-text fields like `notes`, `description`, `bio` that users fill in
> - `ip_address` columns in event logs and audit tables
> - `stripe_customer_id`, `paypal_email` — identifiers that link back to real people
> - JSONB columns that store user-submitted form data
> - `metadata` fields that accumulate whatever the app was logging at the time

**PII-16** — [gonymizer README](https://github.com/smithoss/gonymizer), retrieved 2026-09-05, the tool SmithRx built for HIPAA data, in its own words:

> **THERE IS ABSOLUTELY NO GUARANTEE THAT USING THIS SOFTWARE WILL COMPLETE A CORRECT ANONYMIZATION OF YOUR DATA SET FOR COMPLIANCE PURPOSES.**

The sentence immediately before it explains why they cannot promise more: *"Considering everyone's data set is completely different and the configuration of this application is very involved we cannot guarantee that this application will guarantee any compliance of any type."*

---

## Theme 3 — Config burden

Nobody complains about YAML in the abstract. They complain that the first run demands a complete inventory of a schema nobody in the building fully understands, and that the inventory rots.

**CB-1** — [Hacker News, 2022-04-27](https://news.ycombinator.com/item?id=31187000), `fedeb95`, the first reaction to Replibyte's Show HN:

> interesting, however couldn't it detect tables and columns automatically instead of having to specify them in the configuration file? If I understand correctly each table is to be specified by hand. Say I have nearly a hundred tables...

**CB-2** — [Hacker News, 2022-04-28](https://news.ycombinator.com/item?id=31190692), `nicoburns`, on whether the tool beats hand-rolling it:

> I suspect it's likely to take a couple of hours to set up this tool too!

He had [written the pg_dump/pg_restore script by hand the previous day](https://news.ycombinator.com/item?id=31186890): *"it only took a couple of hours to setup (and now I have a repeatable script), so this'll need to be implemented really well to provide value."* That is the bar a new tool has to clear.

**CB-3** — [Hacker News, 2024-05-22](https://news.ycombinator.com/item?id=40446590), `mathisd`, who worked on a commercial pseudonymisation toolchain:

> Referential constraint refer to ensuring some coherence / basic logic in the output data (ie. the anonymized street name must exist in the anonymized city). This was the most time consuming phase of the pseudonymization process.

and, on why the config cannot be written correctly even in principle:

> Also, a lot of the time client had no proper idea of what the field were and what they were truly containing (what format of phone number, we did find a lot of unusual things).

**CB-4** — [Hacker News, 2022-06-03](https://news.ycombinator.com/item?id=31605801), `2rsf`, on test data as a startup opportunity:

> A related problem related to data creation and subsetting is that one person (and practically nobody) knows the entire data relationships between subsystems, but you still need them to create your data.

**CB-5** — [Greenmask issue #321, 2025-07-28](https://github.com/GreenmaskIO/greenmask/issues/321), open — a user asking for deny-by-default:

> I would love the ability to define a transformer for columns that are not defined in the transformers list […] The use case is to avoid dumping data for columns that do not have a transformer defined so that we can ensure no sensitive information is dumped when new columns are added to tables.

**CB-6** — [Hacker News, 2022-04-28](https://news.ycombinator.com/item?id=31190131), `sverhagen`, proposing the same default three years earlier:

> Like an opt-out mode where you have to specify transformations for all columns unless indicated otherwise. Or at least for text columns.

**CB-7** — [Hacker News, 2024-05-23](https://news.ycombinator.com/item?id=40449655), `gregwebs`, describing the machinery a team builds when the tool will not enforce coverage:

> To ensure that we are marking columns as PII, we run a job that compares the anonymization configuration to a comment on the column- we have a comment on every column to mark it as PII (or not).

A comment on every column in the schema, plus a job to diff it against the config, is the cost of a tool that does not classify.

**CB-8** — [Snaplet seed issue #205, 2024-12-04](https://github.com/supabase-community/seed/issues/205), open, on a "quick start" that demanded an OpenAI account:

> Quick Start is not so quick if you have to sign up for an open ai account.

and:

> If the user does not want to use AI then in my opinion it would be better to just add a generic projectDescription or skip it rather than shitting the bed here.

**CB-9** — [Hacker News, 2026-01-22](https://news.ycombinator.com/item?id=46726140), `ofsen`, announcing yet another tool because of the setup cost of the existing ones:

> Dumping production data wasn't an option, and the tools I found were either non-deterministic, manual, or too intrusive.

**CB-10** — [Hacker News, 2022-04-28](https://news.ycombinator.com/item?id=31193683), `xwowsersx`, who wanted the basic case and could not find it in the docs:

> This looks very useful and I have an immediate need for this. I'm still not sure how to use replibyte to do the following: I want to take a snapshot of the DB from one of my environments and then seed a local DB with it. I see this is a basic use case of replibyte, but not sure exactly how to accomplish this.

**CB-11** — [Stack Overflow, 2015-04-20](https://stackoverflow.com/questions/29757797/expensive-maintenance-with-automated-test-data) — the hand-maintained fixture set, in its terminal stage:

> When there is a change in the model, we take a long time to correct all XML files (we have hundreds of XML files, a lot of them with redundancy). The complexity of creating an XML file manually discourages the programmer to explore different scenarios.

**CB-12** — [Software Engineering SE, 2011-10-10](https://softwareengineering.stackexchange.com/questions/113441/do-we-need-test-data-or-can-we-rely-on-unit-tests-and-manual-testing):

> We have problem with maintaining scripts for test data. The business logic is pretty complex and one "simple" change in the test data often produces several bugs in the application (which are not real bugs, just the product of invalid data). This has become big burden to the whole team because we are constantly creating and changing tables.

**CB-13** — [Neosync issue #3133, 2025-01-13](https://github.com/nucleuscloud/neosync/issues/3133), open, **[maintainer]**, filed by the maintainers against their own product:

> it's really troublesome right now to find what columns are no longer there in the mapped schema. if a column is configured but isn't there anymore, it's difficult to fix this error `unable to continue: job mappings contain schemas, tables, or columns that were not found in the source connection`

Config drift is not a hypothetical: the vendors ship a bug tracker entry for it.

**CB-14** — [Replibyte issue #261, 2023-01-30](https://github.com/Qovery/Replibyte/issues/261), open — the config schema itself is the limit:

> The database_subset.table is only allow one value.

**CB-15** — [Hacker News, 2019-06-16](https://news.ycombinator.com/item?id=20196993), `davismwfl`, describing the hand-rolled approach that "worked the best":

> This worked really well and yes, takes time initially to setup and takes some time to maintain, but it means you can reproduce a meaningful dataset into any environment for testing or development quickly and automated.

**CB-16** — [DBA Stack Exchange, 2017-03-23](https://dba.stackexchange.com/questions/168023/how-to-anonymize-pg-dump-output-before-it-leaves-server), asking for this exact product and finding nothing:

> For development purposes, we dump the production database to local. It's fine because the DB is small enough. The company's growing and we want to reduce risks. To that end, we'd like to anonymize the data before it leaves the database server.

and:

> Is there a ready-made solution for this? […] I searched for `postgresql anonymize data dump before download` and variations, but I didn't see anything highly relevant.

---

## Theme 4 — Trust: abandonment, silent success, dread and vetoes

Three flavours: distrust of the tool's future, distrust of the tool's output, and distrust of the practice itself.

**TR-1** — [Replibyte issue #277, "Maintained?", 2023-07-21](https://github.com/Qovery/Replibyte/issues/277), open for over three years:

> Is this project being maintained or alternatively is it just stable? I don't see any non-doc related changes in 8 months now?

The maintainer promised time twice ("I have no ETA yet, but it will probably happen before the end of the year", 2023-07-23). Follow-ups from three different users over the next two years. `macrozone`, 2025-01-24:

> hi @evoxmusic I would also be interested if you still plan to maintain it or hand it over to some other devs! I really like the approach and I don't see many alternatives that are as sophisticated as yours is

`patricktyndall`, 2025-03-19:

> Bumping on this. Would love to know if this is the right choice. Currently can't get `subset` working at all, so a little discouraged but I love the approach.

**TR-2** — [Neosync issue #3565, 2025-08-25](https://github.com/nucleuscloud/neosync/issues/3565), open:

> Hey, just wondering what the post acquisition plan is for this project? Is it going to continue to be maintained? Handed over to the community? Shuttered?

Maintainer `nickzelei`, the same day **[maintainer]**:

> There are no current plans for any continued maintenance. We are still working with the acquirer on handing them the repository but ultimately it will be up to them as to what they decide to do with the repository. If you're using Neosync I'd recommend forking it at the current time and making any changes you wish.

The first reply from a user: *"Thanks @nickzelei. I have already done so :)"*

**TR-3** — [Supabase blog, 2024-08-14](https://supabase.com/blog/snaplet-is-now-open-source), quoting Snaplet founder Peter Pistorius on the shutdown **[vendor]**:

> I built Snaplet because I believe developers write better software when they have access to production-like data. Although the company is closing, my belief remains strong, so we are open-sourcing the tools we've built.

Supabase said it would "pick up the ongoing maintenance". Two years on, `snapshot` is archived and `seed` has not had a release since 2024-07-30.

**TR-4** — [Hacker News, 2024-05-23](https://news.ycombinator.com/item?id=40449655), `gregwebs`, describing the normal end state for tools in this category:

> We are using datanymizer [1] right now but it has gone unmaintained and we are using my patched version [2] and it is working pretty well for us.

His comment lists five more tools he evaluated and rejected. Running a personal fork of an abandoned anonymiser against customer data is the status quo this project is competing with.

**TR-5** — [Hacker News, 2022-04-28](https://news.ycombinator.com/item?id=31189199), `onion2k` (same comment as PII-1, different claim):

> Protecting user data is something that you need to be extremely vigilant about. In my experience, the less access I have to production data the happier I am. Copying it and using it in staging, even if you're careful about it, fills me with dread.

**TR-6** — [Hacker News, 2022-04-28](https://news.ycombinator.com/item?id=31190734), `nicoburns`:

> If syncing prod data then I'd definitely want to have very thorough filtering. But then at that point I'm not sure I'd trust this tool!

**TR-7** — [Hacker News, 2022-04-28](https://news.ycombinator.com/item?id=31189830), `krageon`:

> Unless you can exhaustively guarantee your customer-data containing production data will definitely be transformed into something completely unrecognisable and irreversible (and let's face it, you can never do so - systems change all the time), using this is irresponsible.

**TR-8** — [ThoughtWorks Technology Radar, "Production data in test environments", **Hold**, last updated 2022-03-29](https://www.thoughtworks.com/radar/techniques/production-data-in-test-environments) — quoted into the Replibyte thread by `time4tea` [on 2022-04-28](https://news.ycombinator.com/item?id=31189350) as an argument against the whole category:

> There is little point in having elaborate controls around access to production data if that data is copied to a test database that can be accessed by every developer and QA. Although you can obfuscate the data, this tends to be applied only to specific fields, for example, credit card numbers.

**TR-9** — [Hacker News, 2026-03-06](https://news.ycombinator.com/item?id=47272366), `time4tea` again, four years later, on the dbslice thread:

> Copying production data to dev is widely regarded as being a bit of a bad idea, if the data contains any information that relates to a person or real life entity. Uncontrolled access, inability to comply with "right to be forgotten" legislation, visibility of personal information, including purchases, physical locations, etc etc. […] Attempts to anonymise are often incomplete, with various techniques to de-anonymise available.

**TR-10** — [Hacker News, 2026-03-05](https://news.ycombinator.com/item?id=47259769), `patpatpat` — the gate is social, not technical:

> I made one of these, however I still have to solve the PII issues convince the data custodians that it's safe to use.

[The dbslice maintainer's answer](https://news.ycombinator.com/item?id=47269176) (`nabroleonx`, 2026-03-06) describes the artefact he thinks unlocks that: compliance profiles that "auto-appl[y] masking rules, scan the output for residual PII, and generate an audit manifest your data custodian can review". [`patpatpat`'s reply](https://news.ycombinator.com/item?id=47284804) (2026-03-07): *"Sounds fantastic."*

**TR-11** — [Hacker News, 2026-03-05](https://news.ycombinator.com/item?id=47262861), `semiquaver`, on what the ban costs:

> When I moved to big tech the rules against doing this were honestly one of the biggest drivers of reduced velocity I encountered. Many, many bugs and customer issues are very data dependent and can't easily be reproduced without access to the actual customer data. […] I think it's under-appreciated how much it slows down the real world progress of fixing customer-reported issues.

**TR-12** — [Hacker News, 2022-04-29](https://news.ycombinator.com/item?id=31201089), `tjpnz`, on why an opt-out flag does not clear review:

> With regards to telemetry I'm aware that it can be disabled. But in my experience that would still result in a veto from the security teams I've worked with.

**TR-13** — [Hacker News, 2025-07-06](https://news.ycombinator.com/item?id=44476754), `throwaway7783`:

> Most security teams do not allow prod data in non prod environments, anonymized or not

**TR-14** — [Greenmask issue #465, 2026-07-10](https://github.com/GreenmaskIO/greenmask/issues/465), closed 2026-07-31, on `validate` passing and `restore` failing:

> The dump-succeeds/restore-fails failure mode is the most expensive kind: in a scheduled pipeline the bad artefact is produced and stored successfully, and the breakage is only discovered by a downstream consumer, potentially much later. A validate-time check would move the error to the earliest possible point.

The maintainer's answer was that `--strict` (shipped in v0.2.22, 2026-07-01) covers it.

**TR-15** — [Greenmask issue #469, 2026-07-28](https://github.com/GreenmaskIO/greenmask/issues/469), closed 2026-08-05 — a read-only source role producing a silently wrong target:

> When the role used by `greenmask dump` has `SELECT` on tables but no privilege on sequences (a very common "readonly" role setup), the dump does not fail, instead it silently records every sequence as `setval('<seq>', 1, false)`. `greenmask restore` then applies these wrong values, producing a database where all tables have data but every sequence is at 1.

The report notes the regression path: before PostgreSQL 18 the same call raised `permission denied` and the dump failed loudly. The safe behaviour was lost to an upstream change nobody re-tested.

**TR-16** — [dbslice issue #8, 2026-05-20](https://github.com/nabroleonx/dbslice/issues/8), closed 2026-06-23 — twenty minutes of extraction, then nothing:

> the file grows correctly during extraction (observed to several hundred MB / hundreds of thousands of INSERT statements over ~15-20 min), but the final file written to disk is a small (~400-byte) empty structured shell with header `-- Tables: 0, Rows: 0` […] dbslice's own log line reports `Wrote <N> rows to <path>` and the process exits with code 0, so the failure is silent.

**TR-17** — [dev.to, 2025-10-28](https://dev.to/alex_hayward_ff2d88ff331e/building-a-test-data-platform-after-watching-teams-secretly-use-production-for-years-31b6), GoMask co-founder **[vendor]**, quoting a Reddit reply he received. **Second-hand: the original Reddit comment could not be fetched from this environment:**

> Everywhere I've worked with sensitive data, everybody ended up secretly working off prod.

His own gloss:

> So everyone takes the path of least resistance. Quietly use production data and hope compliance doesn't dig too deep.

---

## Theme 5 — Speed and resource cost

Two different complaints get conflated: wall-clock time of a run, and memory blowing up on databases that are not large.

**SP-1** — [Replibyte issue #264, 2023-03-01](https://github.com/Qovery/Replibyte/issues/264), open, on a 15 GB source:

> I've observed network activity below 2MB/s while running replibyte. Eventually, the process is killed

Confirmed by a second user, `ikegentz`, 2023-08-22:

> Confirming the same issue, doesn't seem to matter where replibyte is running, I also get ~2MB/s and then the process is killed. FWIW I'm seeing this on mysql

**SP-2** — [Replibyte issue #293, 2024-01-30](https://github.com/Qovery/Replibyte/issues/293), open, restoring 8 million rows / 4 GB:

> I started yesterday around 1pm EST and it it's now 10am EST the following day and it only has about 1.7million records in the replica db

**SP-3** — [Replibyte issue #268, 2023-06-14](https://github.com/Qovery/Replibyte/issues/268), open, `maxleroy`:

> Same here, my database weighs less than 200Mb, and still I get OOM killed with 3GB of memory used by replibyte...

A third user added "+1 same issue" in 2024.

**SP-4** — [Replibyte issue #244, 2022-12-09](https://github.com/Qovery/Replibyte/issues/244), open, subsetting a ~250 MB database:

> the database is in total around 250mb (148mb if asking postgres with: `SELECT pg_size_pretty( pg_database_size('dbname') )`) and 51gb of memory taken up seems excessive.

**SP-5** — [Replibyte issue #289, 2023-12-24](https://github.com/Qovery/Replibyte/issues/289), open, on Pagila, a *sample* database:

> the dump takes hours while on linux takes few minutes

**SP-6** — [Jailer issue #93, 2022-08-07](https://github.com/Wisser/Jailer/issues/93), closed 2022-09-22:

> I have ran this operation twice at this point. The first run failed with this issue after running for 10+ hours and having 'collected' 35 million+ rows. The second run failed with this issue after running for 30+ hours and having 'collected' 50 million+ rows.

Thirty hours of work discarded by one dropped connection. A subsetter that cannot resume is a subsetter that cannot be trusted with a large source.

**SP-7** — [Hacker News, 2022-04-27](https://news.ycombinator.com/item?id=31186965), `tehlike`, replying to "it only took a couple of hours to set up":

> I am on the same boat but couple hours is terrible still. The best is probably copying the data directory straight which should cut it down to seconds, but i have yet to automate that + there are production credentials/sensitive data problems that needs to be tackled too...

**SP-8** — [PostgreSQL Anonymizer issue #507, 2025-01-20](https://gitlab.com/dalibo/postgresql_anonymizer/-/issues/507), open:

> We observed that the `anon.anonymize_table()` function runs sequentially, which increases memory and I/O load for large tables.

**SP-9** — [PostgreSQL Anonymizer issue #637, 2026-05-13](https://gitlab.com/dalibo/postgresql_anonymizer/-/issues/637), open, on the cost of doing FK-correct grouping — the price of Theme 1 shows up as Theme 5:

> as I saw `anon.dispatch` will look at all tables with reference recursively, this can be slow even on a small DB where almost all tables are linked to one another, and causes the parallel process to run on a single worker only

**SP-10** — [Hacker News, 2024-07-17](https://news.ycombinator.com/item?id=40988627), `bearjaws`:

> I've had to scrub a multi terabyte database of PII before moving to a staging environment, it hurts.

**SP-11** — [Neosync issue #3481, 2025-04-09](https://github.com/nucleuscloud/neosync/issues/3481), open, on time lost to a job that will not say what is wrong:

> When trying to debug a failing job and it hangs, it's impossible to debug from this tool. Opening the docker worker logs allows you to see the actual error coming from DB. In my case, data was violating a constraint. Meanwhile, the UI just continues to count up as if it's in progress. Discovering the source of this error after 10+ minutes of letting the job finish is frustrating.

**SP-12** — [Xata blog, 2026-01-22 (approximate — the page carries no visible publication date and search-engine indexing puts it anywhere from 2026-01-22 to 2026-02-06; the quoted text itself is verbatim)](https://xata.io/blog/anonymization-the-missing-link-in-dev-workflows) **[vendor]**, describing the pipeline it sells against:

> Production database snapshot (2-8 hours for 1TB+) Restore to staging environment (2-8 hours) Run Python scrubbing script (1-4 hours) Developer access (data already 12+ hours stale)

and:

> In reality, because the process takes so much time, its common for staging to lag production by weeks or months.

**SP-13** — [Hacker News, 2025-10-28](https://news.ycombinator.com/item?id=45732136), GoMask co-founder **[vendor]**, on the motivation:

> Built this because we were tired of waiting days for test data ourselves.

**SP-14** — [Hacker News, 2021-05-01](https://news.ycombinator.com/item?id=27007696), `john-tells-all`, in "Ask HN: How Long Is Your CI Process?":

> Speed up databases. Move from "install database and sample data interactively every time" to having a pre-baked Docker image with the database and seed data. Much faster: you get lower LAG and the same VALUE for the team.

---

## Theme 6 — Unsupported database, platform or type

Almost always the same shape: "this is exactly what I need, and then I found it doesn't do X."

**DB-1** — [Hacker News, 2022-04-28](https://news.ycombinator.com/item?id=31189054), `dvasdekis`:

> I was thinking "oh! this is awesome!", and then noticed it didn't support MSSQL. Not to worry, I'll just contribute a connector. Let's take a look at their existing connector code... Not a single comment to say what anything does. Sigh. It's the same for the other drivers too.

**DB-2** — [Replibyte issue #263, 2023-02-07](https://github.com/Qovery/Replibyte/issues/263), open:

> Your project is awesome, however our main database is Cassandra. Sadly I do not (yet) develop with Rust so I cannot create a PR for that.

**DB-3** — [Replibyte issue #260, 2023-01-26](https://github.com/Qovery/Replibyte/issues/260), open:

> I would like to see CockroachDB as a supported database. It's nearly pg compatible but pg_dump does not work afaik.

**DB-4** — [Greenmask issue #222, "epic: MySQL support", opened 2024-10-15](https://github.com/GreenmaskIO/greenmask/issues/222) — still open today, nearly two years later. Maintainer, 2025-04-15 **[maintainer]**:

> The work turned out to be more complex than we thought, so we've had some delays.

**DB-5** — [Neosync issue #3411, 2025-03-26](https://github.com/nucleuscloud/neosync/issues/3411), open, on a minor-version boundary:

> The issue is specific to MySQL 5.7. When attempting the same setup with MySQL 8.0.40, also on AWS RDS, the connection is established successfully without any errors.

**DB-6** — [Greenmask issue #377, 2025-12-12](https://github.com/GreenmaskIO/greenmask/issues/377), closed 2025-12-17:

> I see there are windows binaries, but is windows actually fully supported? The default tmp_dir doesn't exist on windows and when I set it to an existing directory I get file locking error.

**DB-7** — [Hacker News, 2025-06-18](https://news.ycombinator.com/item?id=44310202), `Brycee`, on the VeilStream Show HN:

> Also, does this work with all PostgreSQL extensions (PostGIS, timescaledb, etc.)?

[The founder's answer](https://news.ycombinator.com/item?id=44310262) (`joram87`, 2025-06-18):

> PostGIS and other extensions are on the radar, but currently are not supported. The proxy works with the extensions, but can't mask the data yet.

**DB-8** — [Hacker News, 2025-06-18](https://news.ycombinator.com/item?id=44311946), `Ksbt`, in the same thread, on types rather than engines:

> What data types can Veilstream handle? Like can I mask nested jsonb, uuids, IP addresses, arrays? Would be wild if adding new filters was fast enough to support weird internal schemas or bespoke pii.

[The founder's answers](https://news.ycombinator.com/item?id=44312147) (`joram87`, 2025-06-18), in order: "kinda", "no, but I should", "yes … ip4 and ip6", and "again, not yet".

**DB-9** — [Greenmask issue #444, 2026-05-11](https://github.com/GreenmaskIO/greenmask/issues/444), open:

> greenmask dump panics when processing the settings table (129 columns). Greenmask's COPY format decoder uses a fixed [128] array internally. Any table with ≥129 columns causes an index-out-of-range panic.

**DB-10** — [Greenmask issue #394, 2026-01-29](https://github.com/GreenmaskIO/greenmask/issues/394), closed same day — silent data corruption on a plain passthrough column:

> When dumping a PostgreSQL database containing VARCHAR fields with numeric-looking content (e.g., GTIN product codes like `00001402417161`), Greenmask appears to auto-detect these as numeric values and strips leading zeros.

**DB-11** — [Stack Overflow, 2023-01-19](https://stackoverflow.com/questions/75173243/anonymising-a-postgres-database-stored-in-amazon-rds) — the managed-Postgres wall:

> The preferred method for anonymising postgres dumps seems to be using Postgresql Anonymizer. This is unsupported by RDS, meaning you have to upload an SQL file manually which adds the extension. I've done that following the steps here However as the installation was not done on the postgres machine itself, there are various data folders missing.

An extension-based design is unavailable to anyone on RDS, Cloud SQL or most managed Postgres.

**DB-12** — [PostgreSQL Anonymizer issue #635, 2026-05-12](https://gitlab.com/dalibo/postgresql_anonymizer/-/issues/635), closed 2026-05-25 — the distribution channel lying about what it ships:

> The published images in the container registry are tagged with version numbers (e.g. `3.0.5, latest, stable, 2.1.0 ...`), but the `postgresql_anonymizer` binary _inside_ those images is still `2.0.0`.

**DB-13** — [Jailer issue #104, 2023-05-09](https://github.com/Wisser/Jailer/issues/104), closed 2023-05-11 — a user asking the best-maintained subsetter in the field whether it also masks:

> Does your solution comply with data privacy regulations by offering data anonymization features?

The maintainer:

> The tool has no explicit anonymization functions for data. You can define filters for it, or use other tools. The solution is then as secure as you make it.

Subsetting and masking are still two tools in 2026. The user has to be the integration.

**DB-14** — [Neosync issue #3420, 2025-03-27](https://github.com/nucleuscloud/neosync/issues/3420), closed the next day, on the most ordinary local setup there is:

> I can't connect to a locally running docker instance, which is running next to neosync in a different docker app. I have no issues connecting to said local instance using other tools (pgadmin/dbeaver for example).

**DB-15** — [Snaplet seed issue #193, 2024-08-07](https://github.com/supabase-community/seed/issues/193), open, with a macOS "+1" in 2025 — the very first command:

> I try to run `npx @snaplet/seed init` and got this error

**DB-16** — [Replibyte issue #310, 2025-09-25](https://github.com/Qovery/Replibyte/issues/310), open — the install path itself:

> Cannot build from source

---

## Theme 7 — CI integration

Thinner than the others, which is itself a finding: most people are not yet at the point of wiring this into CI, because they are still stuck on Themes 1 to 4. The CI complaints that exist are about exit codes, config discovery, install weight, and fixtures that rot between runs.

**CI-1** — [Greenmask issue #454, 2026-06-08](https://github.com/GreenmaskIO/greenmask/issues/454), closed 2026-06-26:

> For incorporating a greenmask configuration into a project, it would be helpful if CI could be configured to fail if there are any validation warnings at all - this would be functionally equivalent to `gcc -Werror`.

Shipped as `--strict` in [v0.2.22](https://github.com/GreenmaskIO/greenmask/releases/tag/v0.2.22), 2026-07-01.

**CI-2** — [Greenmask issue #455, 2026-06-09](https://github.com/GreenmaskIO/greenmask/issues/455), closed 2026-06-26, from the same user one day later:

> Supporting something like `GREENMASK_CONFIG` would make it easier to iterate on greenmask commands/configs without having to re-type or alias it locally. I would love to be able to automatically configure this for our developers' environments.

Both requests are from one person setting one tool up in one project's CI, and both were cheap to satisfy.

**CI-3** — [Greenmask issue #333, 2025-08-15](https://github.com/GreenmaskIO/greenmask/issues/333), closed 2025-09-20 — install friction, offered by a user as a PR:

> Users could then run: `curl -fsSL https://greenmask.io/install.sh | sh`

**CI-4** — [Replibyte issue #242, 2022-11-27](https://github.com/Qovery/Replibyte/issues/242), open:

> Is there any built-in (or a workaround) config to make the backups and restorations automatic? I want to run Replibyte on docker and assign a source and datasource to it, so it can periodically backup the postgres database and upload it to S3

Answered by another user, not a maintainer, 2023-01-16:

> I'm not a replibyte contributor, but I don't believe this feature is supported by replibyte. I would recommend using cron or a CI tool like Github actions or buildkite to accomplish this.

**CI-5** — [Neosync issue #3560, 2025-07-31](https://github.com/nucleuscloud/neosync/issues/3560), closed same day — what "install the tool" means when the tool is a platform:

> I am trying to connect a Neosync deployment in Kubernetes to a Temporal cluster, but Neosync fails with: `unable to verify account's temporal workspace. error: failed to describe namespace: context deadline exceeded`

Getting production-like data into CI should not require running a Temporal cluster.

**CI-6** — [Hacker News, 2020-04-16](https://news.ycombinator.com/item?id=22889726), `harrisonjackson`, after successfully moving his CI from in-memory SQLite to a real Postgres container:

> The tests are slower but the runners scale horizontally so we don't mind […] Now our biggest testing issue is keeping fixtures up to date.

[Asked to elaborate](https://news.ycombinator.com/item?id=22890096):

> Our issue with fixtures has more to do with changing application code and not having a great way to generate/regenerate the fixtures from live data. We've tried a few different libraries to do this but haven't found any that we love.

That is this project's thesis, written by a stranger in 2020.

**CI-7** — [Hacker News, 2024-06-17](https://news.ycombinator.com/item?id=40708192), `jakjak123`, giving the opposite advice from experience:

> usually my advice is to never even being trying to do write seed data in the database unless its very static. It just gets annoying to maintain and will often break.

**CI-8** — [GitLab issue #17211, 2017-02-14](https://gitlab.com/gitlab-org/gitlab/-/issues/17211), "Seed dev with production like data by default" — GitLab, about its own developers:

> The performance of dev instances is so far off of production that it makes it hard to properly consider performance. Having production-like data on dev isn't as good as the real thing, but allows for much quicker iteration on performance problems and makes some types of issue less likely to slip through.

The same issue records why generating it is not enough, quoting an earlier infrastructure discussion: *"it is also difficult to create a real-life data distribution in a seeded database. There will always be lots of outliers that may be difficult to reproduce."* And, in the considerations list, the line every seed-data project eventually writes down: *"Seeds must be kept up to date"*.

**CI-9** — [Hacker News, 2024-06-17](https://news.ycombinator.com/item?id=40707082), `MajimasEyepatch`, in "How to test without mocking":

> The test data is a harder problem to solve. […] However, this can get really nasty if there's a lot of dependencies between your tables. […] You can attempt to anonymize production data, but obviously that can go very wrong.

**CI-10** — [Hacker News, 2022-12-22](https://news.ycombinator.com/item?id=34095802), `btown`, describing a per-PR setup that works:

> within the specific preview namespace named after the PR ID, spins up and seeds with test data (in our case, a sanitized subset of production) a dedicated database statefulset

**CI-11** — [Hacker News, 2022-12-23](https://news.ycombinator.com/item?id=34103230), `nunez`, on why most teams do not get there:

> But what do you do when your most current tables have customer PII or other sensitive data in them and migrations are done manually during release? Now you need to audit the entire database for fields where PII might exist so that automation can be written to dump those databases and sanitize that data.

**CI-12** — [Hacker News, 2013-02-27](https://news.ycombinator.com/item?id=5295135), `Domenic_S`, describing the hand-built version everyone eventually gets to — thirteen years ago:

> Replace all personal user data with placeholders. This part can be tricky, because you have to find everywhere this lives (are form submissions stored and do they have PII?)

and the payoff, which is exactly the product being described:

> We've got a working, sanitized database dump ready and waiting every morning, and a fresh prod-like environment built for us when we log on. It's a beautiful thing.

Cross-reference: **PII-6** (`e7h4nz`, 2026-04-20) is also a CI complaint — the workflow he abandoned was a masked database branch per pull request — but it is counted under Theme 2 because its substance is masking enforcement.

---

## Frequency ranking

One entry is one (source, theme) pair. Counts are reproducible: `grep -cE '^\*\*FK-[0-9]+\*\*' COMPLAINTS.md` gives 23, and so on for `PII`, `CB`, `TR`, `SP`, `DB`, `CI`.

| Rank | Theme | Entries | Vendor-authored | Maintainer-authored | Sharpest single quote |
|---:|---|---:|---:|---:|---|
| 1 | Broken foreign keys / referential integrity | 23 | 4 | 3 | *"To the point where it was almost impossible to do this"* (FK-3) |
| 2 | Trust: abandonment, silent success, veto | 17 | 2 | 1 | *"Copying it and using it in staging, even if you're careful about it, fills me with dread."* (TR-5) |
| =3 | Leaked PII / masking silently no-ops | 16 | 1 | 0 | *"i try to anonymize data on dump but when i restore it on my dev environnement i have real data"* (PII-2) |
| =3 | Unsupported database / platform / type | 16 | 0 | 1 | *"and then noticed it didn't support MSSQL"* (DB-1) |
| =3 | Config burden | 16 | 0 | 1 | *"Say I have nearly a hundred tables..."* (CB-1) |
| 6 | Speed and memory | 14 | 2 | 0 | *"my database weighs less than 200Mb, and still I get OOM killed with 3GB"* (SP-3) |
| 7 | CI integration | 12 | 0 | 0 | *"not having a great way to generate/regenerate the fixtures from live data"* (CI-6) |

(Leaked PII, unsupported database and config burden are a genuine three-way tie at 16. Of the three, config burden is the weakest 16: two of its entries are 2011–2015 fixture-framework questions that are only adjacent to this product, so read it as third-of-three within the tie.)

Notes on the ranking, including where count and severity disagree:

- **Rank 1 and rank 3 (leaked PII) are the same complaint wearing different hats.** Both are "the tool did something to my data that I could not see and could not verify". FK breakage is loud — a constraint violation at load time. PII leakage is silent — a successful run. Frequency ranks FK higher; severity ranks PII higher, because you find out about the FK bug in minutes and about the PII bug in months. The design conclusion is the same for both: make the result *checkable after the run*, in the target, by a command that exits non-zero.
- **FK's lead is partly a sampling artefact and partly real.** Bug trackers reward failures that produce a stack trace, and I deliberately read the Greenmask and Jailer trackers, which are dominated by subset bugs. But the FK entries also come from HN, dev.to, Stack Overflow and two competing vendors' own founders, over sixteen years. The theme would rank first on a narrower sample too.
- **Trust ranks second on this pass by raw entry count — but that rank does not survive normalizing for people, and the table above reports raw counts, not the normalized ones.** See the person-normalized recount in the last bullet below: Trust ties for fourth once each repeat filer is counted once per theme. Independent of where it ranks, structural evidence supports treating durability as a top-tier concern: three of the nine tools in the state table are archived or dead and two more are stale. A developer choosing a tool in 2026 has watched Snaplet close, Neosync be acquired and archived, Replibyte go four years without a release and Datanymizer go quiet. "Why will you still be here in two years" is a fair question to ask a new entrant, and the honest answer is structural: an abandoned static binary keeps working; an abandoned hosted service does not.
- **Silent success is the sharpest sub-theme and it cuts across four themes — though four of the seven examples below are now fixed in their tools.** TR-15 (wrong sequences, exit 0 — [closed 2026-08-05](https://github.com/GreenmaskIO/greenmask/issues/469)), TR-16 (empty output file, exit 0 — [closed 2026-06-23](https://github.com/nabroleonx/dbslice/issues/8)), PII-2 (no transformation, exit 0 — still open), PII-4 (parallel masking reports success while leaving cleartext — [closed 2026-08-27](https://gitlab.com/dalibo/postgresql_anonymizer/-/issues/668)) and PII-5 (the same bug's follow-up — still open), FK-10 (subset silently returns ~0 rows — still open), DB-10 (leading zeros stripped on a passthrough column — [closed same day](https://github.com/GreenmaskIO/greenmask/issues/394)). Seven independent reports across five tools in which the tool said it succeeded and had not; four of the seven underlying bugs are now fixed, three (PII-2, PII-5, FK-10) are not. Read this as evidence that the *failure mode recurs across codebases and years*, not as a live indictment of any one tool today — and the recurrence, not the open count, is why the guarantee is worth designing for. If this project makes one guarantee, "the run either verifies itself or fails loudly" is the one with the most evidence behind it.
- **CI is last on count but not on importance.** It is under-represented because most complainants have not cleared Themes 1 to 4. The CI complaints that do exist are cheap to satisfy and mostly already were: a non-zero exit code on validation failure (CI-1, closed), config discoverable from an env var (CI-2, closed), a one-line install (CI-3, closed), no cluster required (CI-5, closed).
- **Overlaps are flagged, not hidden — and the within-theme duplicate-author census in an earlier draft of this file undercounted itself.** Five sources are counted in two *different* themes each: `onion2k` (PII-1/TR-5), `tjpnz` (PII-7/TR-12), `gregwebs`' 2024 comment (CB-7/TR-4), the GoMask Show HN (FK-23/SP-13) and the Xata post (FK-22/SP-12) — these are legitimate double-counts, one theme-pair each, and are not touched below. Separately, eight people file *twice within the same theme*, verified against GitHub/GitLab author logins rather than by name-matching prose: `imiric` (FK-1, FK-4), `wwoytenko` (FK-11, FK-12), `patricktyndall` (FK-15, FK-18), `onion2k` (PII-1, PII-14), `Maxwell2022` (TR-14, TR-15), `time4tea` (TR-8, TR-9, already noted inline as one person), `danlamanna` (CI-1, CI-2, already noted inline) and `akbarz` (SP-8, SP-9). Counting each *person* once per theme gives **FK 20, DB 16, CB 16, TR 15, PII 15, SP 13, CI 11** — corrected from an earlier, wrong count of FK 22, TR 14, PII 15, DB 16, CB 16, SP 12, CI 12, which had missed six of the eight duplicate filers above. This is **not the same ordering** as the raw-count table: FK still leads and CI still trails — that part is robust — but **Trust falls from rank 2 (17 raw entries) to a tie for fourth with PII (15 each), behind DB and CB (16 each)**. The raw table above counts *entries*, not *people*, and is left as-is because "how many separate reports exist" and "how many separate people are reporting" are both real questions with different answers; but any claim about Trust's rank should say which count it means, and the strongest honest claim is "second by distinct issues and posts, tied for fourth by distinct people."

---

## What the CONCEPT.md principles address, and what they do not

Read against [CONCEPT.md](../CONCEPT.md). The brief asked for a short section here; this one runs long on purpose — the gaps below are the most actionable output of the whole document, so the scope was widened deliberately rather than trimmed to fit. Quick-reference table first, full argument with citations below it.

| Principle | Addresses | Gap / not addressed | Strongest cited entry |
|---|---|---|---|
| Zero config, emit config after the run | Theme 3 (16 entries), partial Theme 2 | Committed `lazysnap.yml` becomes the stale config it was meant to replace | CB-1, CB-2 |
| Deny-by-default masking, explained | Theme 2 (PII), partial Theme 4 | Free-text/JSONB columns can't be classified by name | PII-6, PII-11 |
| No flag to disable masking wholesale | Theme 4 (TR-7, TR-12) | — | TR-12 |
| Verifies FK integrity in target | Theme 1 (loud half) | No equivalent verification that masking actually ran | TR-14, FK-7 |
| Plans a subset (root, count, parents, cycles) | Theme 1 (FK-7/8/9/14/15/16/19) | "Cycles handled" must include *no* cycles; over-collection is a privacy bug, not just a size one | FK-8, FK-14 |
| One static binary, one-line install | Theme 6 install/platform failures | Non-Postgres engines (Theme 6, ~31%) explicitly out of scope for v1 | DB-11, DB-4 |
| Same command works headless in CI | Theme 7 (cheap, already shipped upstream) | — | CI-1 |
| Never holds write access to source | PII-13 | A read-only role can itself produce a silently wrong target | TR-15 |
| Deterministic masking for joins | FK-21, PII-15 (partial) | Silent on determinism *across runs*, and on how a stable pseudonym interacts with erasure requests | CB-9, PII-9 |
| (unstated) | — | Verification of masking as a class; silent-success as a class; schema drift in a *committed* config; sequences/extensions as non-row state; the policy-blocked user; target-side restore behaviour | PII-1, TR-15 |

### Directly addressed

**"Zero config. First run asks at most one question and then works. Configuration is emitted after a run as a record of what happened, never demanded before it."**

A direct hit on Theme 3 (16 entries) and a partial hit on Theme 2. CB-1, CB-5, CB-6, CB-7, CB-10 and PII-15 are all asking for exactly this inversion: do not make me enumerate a schema nobody fully understands, and do not let an un-enumerated column ship in cleartext. Emitting `lazysnap.yml` after the run also answers CB-2 — the hand-rolled script took two hours and produced a repeatable artefact; this promises the artefact without the two hours.

**"Anything that might be personal data is masked unless the user opts a column out, and the tool explains why it masked each one."**

Deny-by-default is the fix PII-11 named in 2014 (`jzwinck`, on the MDN leak), PII-12 made a purchase condition in 2022, and CB-5 filed against Greenmask in 2025. It is the single most-requested behaviour in the corpus. PII-6 states the strongest version of the requirement — *"Until masking can be enforced by construction rather than by convention"* — and that is the sentence to design against. The "explains why" half also does work on Theme 4: TR-10's *"convince the data custodians that it's safe to use"* is a request for an artefact you can hand a person, and a per-column justification is the beginning of that artefact.

**"We refuse to ship a flag that disables masking wholesale."**

Answers TR-7 and TR-12. Note that TR-12 is specifically about a feature that *could* be disabled still failing security review. The absence of the flag, not its default, is the point.

**"Verifies foreign-key integrity in the target."**

The answer to the loud half of Theme 1, and the right shape: verify in the target after load, not by trusting the plan. FK-7, FK-8, FK-9, FK-10, FK-15 and FK-19 are all cases where a tool believed its own plan and produced an unloadable or empty artefact. TR-14 names why post-hoc verification beats pre-hoc validation.

**"Plans a subset: a root table, a row count, parents to completeness, children with caps, cycles handled."**

Names the four things that break in FK-7/8/9/14/15/16/19. Two cautions from the corpus. First, "cycles handled" must include *no cycles*: FK-8 is a tool that panicked on the absence of one. Second, FK-14 (Jailer's implicit reverse traversal pulling in other customers' orders) shows that "parents to completeness" has a privacy edge — over-collection is a leak, not just a size problem. Choosing a *root table* rather than a filter is also the direct answer to FK-17, where the tool would only accept a root it derived itself.

**"One static binary, one-line install."**

Answers CI-3, CI-5, DB-6, DB-12, DB-14, DB-15 and DB-16 — seven separate sources across six tools that failed at install, on an unexpected platform, or on the deployment weight of the tool itself. CI-5 (a Temporal cluster) and DB-11 (a Postgres extension that RDS will not let you install) are the two shapes of "you cannot even start". A static binary that speaks the wire protocol dodges both.

**"The same command a human types works headless in CI."**

Answers CI-1, CI-2 and CI-4. Cheap to satisfy, and already committed to.

**"It never holds write access to the source."**

Answers PII-13 — the developer who transposed the source and destination IP arguments. Structural, not a warning in the docs. But see the gap below: TR-15 shows that a least-privilege source role can *itself* produce a silently wrong target, so "read-only" is a precondition, not a guarantee.

### Partly addressed, with gaps

**Determinism: across tables versus across runs.** CONCEPT.md says "masks them deterministically so joins still work". That covers FK-21 and PII-15's third bullet. It does not say *deterministic across runs*, which is a different property and the one that makes a snapshot diffable and a bug report reproducible. CB-9 asks for it by name (*"the tools I found were either non-deterministic, manual, or too intrusive"*). Say which one is promised. [Greenmask #325](https://github.com/GreenmaskIO/greenmask/issues/325) was a real instance of the failure — filed 2025-07-31, the hash engine was not deterministic in the way users assumed, and the maintainer fixed it, closing the issue 2025-08-18 — which is exactly why the property is worth naming explicitly rather than assuming a mature tool already guarantees it.

**"Columns that look like personal data."** Column-name classification cannot catch PII inside a free-text `notes` field or a JSONB blob — PII-15 lists both, PII-8 argues that the aggregate itself is identifying, and PII-9 gives the re-identification arithmetic. The non-goals exclude binary blobs but are silent on text and JSONB. That silence is a gap, not a decision. At minimum the tool should say, per column, "I looked at this and could not classify it", and treat unclassifiable free text as maskable by default.

**Speed.** The concept's example run is 38 seconds for 500 customers, which is the right target, but no principle commits to bounded memory. Theme 5 shows that memory, not time, is what kills these runs: SP-3 (200 MB source, 3 GB RSS, OOM), SP-4 (250 MB source, 51 GB RSS). "Streams the rows out" implies constant memory; promise it explicitly. SP-6 adds a second requirement the concept does not mention: a long extraction must be resumable, or a dropped connection at hour 30 costs the whole run.

**Type and value fidelity.** DB-10 (leading zeros stripped from a VARCHAR that looked numeric) and FK-15 (a deferred UUID emitted as `null::text`) are both cases where the tool corrupted data it was not asked to transform. A snapshot tool's implicit promise is "untransformed columns arrive unchanged". Worth stating, and worth a check.

### Not addressed

**Verification of masking, as distinct from verification of foreign keys.** Step 5 verifies FK integrity in the target. Nothing verifies that no cleartext reached the target. PII-2, PII-3, PII-4 and PII-5 are four cases across three tools where masking silently no-opped and the run reported success. A residual-PII scan of the target is the exact analogue of the FK check and is absent from the concept. dbslice's maintainer is already advertising it ("scans the output for residual PII"); it is table stakes, not a differentiator.

**Silent success as a class.** Beyond masking: TR-15 (sequences at 1 because the read-only role lacked sequence privileges — and the loud failure was lost to a PostgreSQL 18 change), TR-16 (a zero-row output file after twenty minutes of writing, exit code 0), FK-10 (a subset that returns only the NULL-FK rows). The concept has no stated position on what the tool asserts about its own output before it claims success. Candidate rule: the run ends with a set of assertions — row counts per table non-zero where the plan said non-zero, FK check clean, sequence values sane, no residual matches for classified patterns — and any failed assertion is a non-zero exit.

**Schema drift over time.** PII-1 is the highest-signal single complaint in the corpus and it is a *longitudinal* failure: the config was right when written and wrong three months later. PII-6 is the 2026 restatement, and CB-13 shows the vendors filing it against themselves. CONCEPT.md's answer is "configuration is emitted after a run", which helps because each run re-derives classification. But if `lazysnap.yml` is committed and used in CI, as step 5 suggests, the committed file *is* the stale config and drift returns. There is no stated behaviour for "the schema has a column your committed config has never seen". The safe answer — mask it, and fail loudly rather than pass it through — is not written down anywhere. This remains the most important gap.

**Trust as a durability question (Theme 4 — second by raw issue count, tied for fourth once repeat filers are counted once; see the Frequency ranking section).** Nothing in CONCEPT.md speaks to "will this still be maintained". The static-binary, no-service, no-cloud-dependency choice is a real structural answer, but it is implicit. Three-of-nine-tools-archived is the strongest argument for the terminal-first, no-hosted-service shape; say so out loud, in the README, where TR-1 and TR-4's authors will read it.

**Theme 6, unsupported database, by explicit choice.** The non-goals list Postgres-only for v1. That is defensible, but it declines to serve 16 entries' worth of complaint, and DB-4 (Greenmask's MySQL epic, open two years) shows the second engine costs far more than it looks. One sub-case is worth pulling forward inside the Postgres scope: DB-11 (managed Postgres cannot install extensions) is a reason the extension-free approach is a feature, and DB-8's list — jsonb, uuid, inet, arrays — is the type coverage v1 will be judged on.

**Anything for the person who is not allowed to do this at all.** TR-11, TR-13 and TR-10 describe developers blocked by policy, not tooling — and TR-17's second-hand Reddit line ("everybody ended up secretly working off prod") describes what happens next. The concept has no story for producing an auditable artefact a compliance function signs off once, after which runs are self-service. dbslice is building exactly that and got *"Sounds fantastic"* in reply. Out of scope for v1; worth a tracker entry.

**Sequences, extensions and other non-row state.** TR-15 is a reminder that a "database copy" is not only rows. Nothing in the concept says what happens to sequences, identity columns, extensions or generated columns. Getting a target where every sequence is at 1 is a bug the user finds on their first `INSERT`.

---

## Questions the brief asked that the themes above don't answer directly

**Which of the tracker issues cited are still open, and which are fixed?** Every GitHub- and GitLab-sourced entry above now carries an inline open/closed marker, checked today via `gh api` and the GitLab v4 API rather than by re-reading each page. Of the 52 tracker-sourced entries (23 FK-adjacent through CI), **19 are now closed and 33 remain open** as of 2026-09-05. Closed: FK-7, FK-14, FK-15, FK-16, PII-3, PII-4, TR-14, TR-15, TR-16, SP-6, DB-6, DB-10, DB-12, DB-13, DB-14, CI-1, CI-2, CI-3, CI-5. The rest are open. Read a closed marker as "this specific bug was fixed", not as "this class of failure is gone" — TR-15's fix, for example, is one patched sequence-privilege check in one tool, not a general answer to "can a read-only role produce a silently wrong target".

**What share of the corpus is PostgreSQL-specific, given CONCEPT.md scopes v1 to Postgres only?** Exactly five of the corpus's 114 numbered entries name a non-Postgres engine explicitly: DB-1 (MSSQL), DB-2 (Cassandra), DB-3 (CockroachDB), DB-4 (MySQL), DB-5 (MySQL 5.7) — all five inside Theme 6, where they make up 5 of 16 entries (31%). Outside Theme 6, the only other explicit non-Postgres mention is the confirming reply under SP-1 ("FWIW I'm seeing this on mysql"); the root SP-1 report itself does not name an engine. Every other entry is either explicitly Postgres (pg_dump/pg_restore/COPY/sequence behaviour, the postgresql_anonymizer and dbslice trackers, RDS) or engine-agnostic (config burden, trust, masking leaks, the CloakDB and Xata posts). So Theme 1's lead is not an artefact of counting MySQL or Cassandra users lazysnap will not serve: at most 1 of its 23 entries (SP-1's confirmation, and that's a different theme) touches a non-Postgres engine, and DB-2/DB-3/DB-4/DB-5 are already isolated inside Theme 6, which CONCEPT.md's non-goals already decline to address.

**How big a slice do people actually want, against the concept's "500 customers … in 38s" example?** No entry in the corpus states a target row count, database size or table count a developer asked for by name — this is a genuine, unfilled gap, not a claim I can source around. The closest data points, none of them a direct answer: FK-20 (Stack Overflow, 2010) wanted "10-20k rows or more"; [Seedfast](https://seedfa.st/blog/small-data-big-lies) (`Mikhail Shytsko`, 2026-03-22) **[vendor]** claims "most test databases hold between 5 and 50 rows per table" today and recommends starting "at roughly 10× your current test volume"; the same vendor's [load-testing post](https://seedfa.st/blog/load-testing-data) reports generating "~1M FK-valid rows on a 20-table SaaS schema in about 3.5 minutes" as a synthetic (not extracted) benchmark. None of these is "developers asking for 500 customers' worth of data" — they are either decades-old (FK-20) or a competing vendor's marketing benchmark for a different technique (synthesis, not extraction). The 38-second, 500-customer example in CONCEPT.md remains unanchored to any stated user request; treat it as a design target chosen for the pitch, not a number this research supports.

**What do people say about the target side — loading into an existing local database (CONCEPT.md step 4)?** I searched HN, GitHub issues across all six tracked tools, and the general PostgreSQL mailing list for complaints about truncation, pre-existing rows, schema mismatch between source and target, or migrations already applied locally, and found no on-point complaint in any of the sources this document draws from — this is a real hole in the corpus, not a claim that the problem doesn't exist. The only tangential evidence is a [PostgreSQL mailing-list thread requesting `--truncate-tables` for `pg_restore`](https://www.postgresql.org/message-id/1348161889.25476.1%40mofo) (2012, still not in core `pg_restore` as of the versions cited elsewhere in this document), which shows the underlying operation — restoring into a target that already has rows or a slightly different shape — is a long-standing rough edge at the `pg_dump`/`pg_restore` level, not that any of the seven tools in scope have been reported as failing on it. Mark this "unverified" rather than inventing a tool-specific quote: nobody in the corpus has yet filed the bug CONCEPT.md's step 4 is most likely to hit.

**Is there evidence for or against the terminal-first, one-question, container-auto-discovery interaction bet?** I searched HN and dev.to for developer sentiment on CLI setup wizards versus flag-driven tools, and for opinions on Docker container auto-discovery specifically for database tools, and found nothing that bears on lazysnap's three most distinctive interaction choices, for or against. General CLI-design writing agrees wizards should fall back to flags for scripting, but nothing ties that to this product category. This is an absence worth stating plainly rather than filling with a tangential quote: the corpus has an opinion on config file burden (Theme 3) but not on the specific shape of "asks at most one question."

**Where did users of the dead tools (Theme 4) actually go?** TR-2's own first reply is a within-corpus data point — a Neosync user forking the repo the same day it was declared unmaintained — and TR-4 (`gregwebs`, running "my patched version" of an abandoned Datanymizer) shows the same pattern: fork, don't migrate. Beyond the corpus, [Seedfast's own "Neosync alternative" comparison post](https://seedfa.st/blog/neosync-alternative) (`Mikhail Shytsko`, 2026-04-23) **[vendor — read this as marketing positioning, not a survey]** claims former Neosync users split by function rather than converging on one tool: "Greenmask is the actively-maintained OSS option that does `pg_dump`-compatible masking" for anonymization, Tonic Structural for the enterprise tier, PostgreSQL Anonymizer for an in-database approach, and Seedfast itself for synthetic generation — concluding "no single tool in the current market covers both halves as cleanly as Neosync did." That is itself a claim in lazysnap's favor (a gap where a single tool would win) but it comes from a vendor selling into that exact gap, so treat the conclusion as directionally plausible and the framing as self-interested.

**What is the tolerated cost of a first run?** Still only two firsthand data points inside the corpus: CB-2 ("a couple of hours" to hand-roll a `pg_dump`/`pg_restore` script) and SP-7 ("couple hours is terrible still"). What has changed since those 2022 comments is the outside bar: GoMask's founder frames his own motivation as being "tired of waiting days for test data" (SP-13, 2025), and Seedfast's synthetic-generation benchmark above claims minutes, not hours, for a comparable-sized schema. None of this is a stated user *tolerance* — nobody says "I would accept N minutes but not N+1" — but the direction of travel across four years of vendor positioning (days → hours → minutes) means 38 seconds is competing against an already-falling bar, not a static one; it is aggressive relative to CB-2/SP-7 (2022) and merely competitive relative to what vendors claim in 2026.

**How does deterministic masking interact with the right-to-erasure problem PII-9 raises?** The two facts already in this document point in opposite directions and neither is resolved here. CB-9 asks for masking to be deterministic *by name*; PII-9 describes the GDPR delete-request scenario that a copied database creates ("the first GDPR delete request comes in. Was this data synthetic? Was it real?"). A single global deterministic transform (the same salt for every row, forever) makes the erasure problem *worse*: a stable pseudonym is itself a durable identifier, and once it exists in every snapshot ever taken, "delete this person" means finding and re-masking every copy — exactly the sprawl PII-9 describes, now with an extra fake identity to also track down. The alternative discussed in industry write-ups on this exact tension is **per-entity key isolation with crypto-shredding**: derive each person's deterministic pseudonym from a key unique to that person, so that destroying one key un-links all of that person's pseudonyms across every snapshot without touching anyone else's data or re-running a masking job ([Granit, "Crypto-Shredding: GDPR Erasure Without Deleting a Single Row"](https://granit-fx.dev/blog/crypto-shredding-gdpr-erasure-without-deleting-rows/); the EDPB, UK ICO and French CNIL are cited there as recognizing cryptographic erasure as a valid Article 17 mechanism). CONCEPT.md's "masks them deterministically so joins still work" does not say which of these two shapes it means, and the difference is exactly the one PII-9 is worried about: determinism *within* a run is what CB-9 is asking for and is cheap; determinism *across every run forever with one key* is what turns PII-9's scenario from bad to worse. This is a design question worth resolving explicitly, not a sourced fact — flag it as analysis built on two entries already in this document plus one external explainer, not as a claim any cited developer has made in these words.

---

## Gaps in this research

- **Reddit is unreachable from this environment.** r/ExperiencedDevs, r/dataengineering and r/PostgreSQL are where a lot of this complaining happens, and TR-17 is the only Reddit-origin material here, arriving second-hand through a vendor's blog. This is the biggest hole in the sample.
- **Paid-tool complaints were out of scope for this pass, and that was a real gap, not a correct call.** The premise in an earlier draft — that paid-tool customers only complain in unquotable support portals — was wrong: Gartner Peer Insights product-review pages are public and dated. [Tonic.ai carries 34 reviews](https://www.gartner.com/reviews/market/test-data-management/vendor/tonic-ai/product/tonic) (4.4/5) and [Perforce Delphix carries 64](https://www.gartner.com/reviews/market/test-data-management/vendor/perforce-software/product/perforce-delphix-1057886818) on Gartner's Test Data Management market page alone, and G2/TrustRadius carry more for both. None of those reviews were mined or quoted here — this was a time-boxing decision, not a sourcing dead end, and it should be done before treating "no paid-tool complaints" as a finding. What little is visible without logging in skews positive (Gartner's own "Voice of the Customer" framing selects for it), so the useful complaints are likely gated behind free-text review bodies that need to be read individually rather than summarized from a listing page.
- **Stack Overflow is a poor source for this topic.** The `anonymization` tag returns zero questions via the API; `database-testing` and `test-data` are dominated by 2008–2016 fixture-framework questions. The useful Stack Exchange material was on DBA SE and Software Engineering SE and there is very little of it. The corpus therefore skews toward HN and GitHub, i.e. toward people who evaluate open-source tools, and away from people who write a scrubbing SQL script and never mention it.
- **Survivorship bias in trackers.** Everyone here cared enough to file or post. The developer who ran `pg_dump | sed`, shrugged, and moved on is unrepresented, and is probably the modal case.
- **Vendor voices are load-bearing in two themes.** Four of 23 FK entries and two of 17 trust entries are vendor-authored. They are marked, and the ranking survives removing them, but the *framing* of Theme 1 — "referential integrity is the hard part" — is one that vendors have a commercial reason to repeat.
- **No quantitative baseline.** I found no public survey with a stated sample size on how teams get data into dev and CI databases. Any percentage claim in this space should be treated as unsourced until one exists.
