# Postmortems: Snaplet and Neosync

Two companies built almost exactly what lazysnap describes — point a tool at a production Postgres database, subset it, mask the personal data, load it somewhere safe. Both are dead. Snaplet shut down on 31 August 2024; Neosync was acquired by Grow Therapy and its repository archived in August/September 2025.

This document reconstructs what happened from primary sources: archived shutdown posts and pricing pages, GitHub issues, changelogs, Hacker News threads, npm registry data, and founder statements. Every claim links to its source. Where a source could not be reached or a claim could not be corroborated, it says "unverified".

**Research date:** 4 September 2026. Both `www.snaplet.dev` and `www.neosync.dev` return `NXDOMAIN` as of this date — verified by DNS lookup from this machine — as do `docs.neosync.dev` and `app.neosync.dev`. Every claim sourced to those sites below is cited to the Wayback Machine or to GitHub, because the live web no longer holds them.

---

## 1. Timeline

### Snaplet

| Date | Event | Source |
|---|---|---|
| Jan 2021 | Public beta waitlist. Tagline: "Get a safe, minimized version of your production database on your laptop in minutes" | [Wayback, 18 Jan 2021](http://web.archive.org/web/20210118202109/https://www.snaplet.dev/) |
| Sep 2021 | Open beta, Postgres-specific | [Wayback, 22 Sep 2021](http://web.archive.org/web/20210922220231/https://www.snaplet.dev/) |
| Jun 2022 | "Say goodbye to seed scripts" — transform-and-copy positioning | [Wayback, 1 Jun 2022](http://web.archive.org/web/20220601160539/https://www.snaplet.dev/) |
| Apr 2023 | Preview databases + Netlify integration added to the homepage | [Wayback, 11 Apr 2023](http://web.archive.org/web/20230411000318/https://www.snaplet.dev/) |
| Jan 2024 | "Snaplet uses generative ai to give you realistic, production-like data" | [Wayback, 25 Jan 2024](http://web.archive.org/web/20240125104719/https://www.snaplet.dev/) |
| 6 May 2024 | Snaplet Seed launched on Product Hunt, placed #2 | [Show HN](https://news.ycombinator.com/item?id=40275677), [Product Hunt](https://www.producthunt.com/products/snaplet-seed?launch=snaplet-seed) |
| Jun 2024 | Homepage headline is now "Instant seed data for your relational database. Ditch the seed script!" — Snapshot demoted to a secondary tab | [Wayback, 24 Jun 2024](http://web.archive.org/web/20240624152127/https://www.snaplet.dev/) |
| 1 Jul 2024 | Shutdown announced | [Wayback of shutdown post](http://web.archive.org/web/20240716025550/https://www.snaplet.dev/post/snaplet-is-shutting-down) |
| 30 Jul / 2 Aug 2024 | Last stable npm publishes: `@snaplet/seed@0.98.0`, `@snaplet/snapshot@0.93.2` | [npm registry](https://registry.npmjs.org/@snaplet/seed) |
| 14 Aug 2024 | Copycat, Seed and Snapshot open-sourced under MIT; team members join Supabase | [Supabase blog](https://supabase.com/blog/snaplet-is-now-open-source) |
| 31 Aug 2024 | Service off | [shutdown post](http://web.archive.org/web/20240716025550/https://www.snaplet.dev/post/snaplet-is-shutting-down) |

### Neosync

| Date | Event | Source |
|---|---|---|
| Jun 2023 | The same founders launch **Nucleus** (YC S22), "a Kubernetes platform for both devs and ops", priced at ~$35k/licence | [Launch HN](https://news.ycombinator.com/item?id=36197880) |
| 7 Dec 2023 | Neosync v0.1.0 launched as "open source data replication and anonymization" | [Show HN](https://news.ycombinator.com/item?id=38560030), [changelog entry](https://github.com/nucleuscloud/neosync/blob/main/docs/blog/2023-12-04-neosync-init.md) |
| 20 Dec 2023 | "Introducing Neosync" explains the pivot away from cloud infrastructure | [Wayback](http://web.archive.org/web/20250803043832/https://www.neosync.dev/blog/introducing-neosync) |
| 22 May 2024 | Show HN, 246 points, 44 comments — the high-water mark of public attention | [HN](https://news.ycombinator.com/item?id=40443927) |
| 17 Sep 2024 | Second Show HN announcing DynamoDB, MongoDB, SQL Server, and Neosync Cloud | [HN](https://news.ycombinator.com/item?id=41569240) |
| 11 Jul 2025 | Last tagged release `v0.5.41`; a commit titled "license" adds a `ValidLicense` type that always returns `true`, disabling enterprise-edition gating | [releases](https://github.com/nucleuscloud/neosync/releases), [commit 4a2c971](https://github.com/nucleuscloud/neosync/commit/4a2c971c81b098ec8cb6661a1b7883f1f56b40f3) |
| 25 Aug 2025 | A user opens "What is the post-acquisition plan?"; a founder answers "There are no current plans for any continued maintenance." | [issue #3565](https://github.com/nucleuscloud/neosync/issues/3565) |
| 30 Aug 2025 | Final commit: "adds acquired disclaimer"; repo archived | [commit 8101c42](https://github.com/nucleuscloud/neosync/commit/8101c42dcdb0ac6f67558d4fabb41fefad97a101) |
| 25 Sep 2025 | Grow Therapy publicly announces the acquisition | [PR Newswire](https://www.prnewswire.com/news-releases/grow-therapy-raises-the-privacy-bar-in-mental-health-302567153.html) |

Note on the brief: the task described the acquisition as August 2025. The repository work (licence unlock, archival disclaimer, "no plans for continued maintenance") all happened in July–August 2025; the *public announcement* was 25 September 2025. Both dates are real and refer to different things.

---

## 2. What users loved

### Snaplet: the promise, stated plainly, and delivered on the first run

The 2021 tagline is nearly word-for-word what lazysnap's CONCEPT.md promises: *"Get a safe, minimized version of your production database on your laptop in minutes"* ([Wayback, Jan 2021](http://web.archive.org/web/20210118202109/https://www.snaplet.dev/)). It resonated hard. Tom Preston-Werner, quoted on Snaplet's own homepage from a public tweet:

> "Get a safe, minimized version of your production database on your laptop in minutes." I've needed this on every serious project I've worked on and it's always been hard.
> — Tom Preston-Werner ([@mojombo, 17 May 2021, as displayed on snaplet.dev](http://web.archive.org/web/20220601160539/https://www.snaplet.dev/))

> Just gave @_snaplet a whirl and outside of not understanding some syntax stuff...first experiment worked well for anonymizing data from a Heroku-hosted Rails app database
> — Robby Russell ([@robbyrussell, 3 Feb 2022, as displayed on snaplet.dev](http://web.archive.org/web/20220601160539/https://www.snaplet.dev/))

That "outside of not understanding some syntax stuff" is the earliest recorded friction signal and it is about configuration syntax, not about the core job.

On Product Hunt, users described the pain being removed rather than the feature being added:

> I have used Snapplet to seed our dev database with anonymized prod data to run complex data migration...It was 🔥
> — Thibault Le Ouay, openstatus ([Product Hunt](https://www.producthunt.com/products/snaplet-seed?launch=snaplet-seed))

> I spent hours, days, probably weeks crafting our seed data.
> — Dominik Hackl, fynk ([Product Hunt](https://www.producthunt.com/products/snaplet-seed?launch=snaplet-seed))

### Snaplet: the proxy, and deterministic fake values

A year after the shutdown, an HN user explaining what he missed named two specific things, neither of which was the cloud product:

> I liked similar thing, snaplet, unfortunately they're dead now. One thing I liked was the option to run proxy to which you could connect with any tool you like (psql, dbeaver, ...) and see preview of your transformations. Also they had some good (stable) generators for names, emails, etc...
> — muhehe, on a Greenmask thread ([HN, 17 Oct 2024](https://news.ycombinator.com/item?id=41867092))

"Stable generators" is Copycat: deterministic fake data, so the same input always yields the same output. Supabase's write-up describes it as "like faker.js, but deterministic: for any given input it'll always produce the same output" ([Supabase blog](https://supabase.com/blog/snaplet-is-now-open-source)).

**The adoption data confirms this is what people actually kept.** Weekly npm downloads for the week of 23–29 August 2026 — two years after the company died:

| Package | Weekly downloads |
|---|---|
| [`@snaplet/copycat`](https://registry.npmjs.org/@snaplet/copycat) (deterministic fake values) | 121,478 |
| [`@snaplet/seed`](https://registry.npmjs.org/@snaplet/seed) (schema-based generation) | 41,159 |
| [`@snaplet/snapshot`](https://registry.npmjs.org/@snaplet/snapshot) (the actual snapshot tool) | 9,808 |
| [`snaplet`](https://registry.npmjs.org/snaplet) (the original CLI) | 3,511 |

The smallest, most boring, least differentiated component — a deterministic faker with no service behind it — outlives the platform by 12×. The thing the company was actually built to sell is bottom of the list.

### Snaplet: a genuinely good `setup` command

The documented first run introspects the database and *proposes* a config rather than demanding one:

```
✔ Target database connection string … postgresql://postgres@localhost:5432/postgres
📡 Connected to database with "postgresql://postgres@localhost:5432/postgres"
Introspecting database...
Generated transform type definitions: snaplet.d.ts
⠋ Transform: Detecting PII fields...
ℹ Transform: Generate transform config...
Generated transform config: snaplet.config.ts

😽 Snaplet has introspected your database structure and generated
suggested transformations for your data in snaplet.config.ts
Please review them.
```
([transform docs](https://github.com/snaplet/docs-old/blob/main/docs/04-references/data-operations/02-transform.md))

The same docs also say, in a tip: *"A user account is **not required**, you only need a user account if you want to share snapshots with your team."* That is the right default and they had it. See §4 for what they did with it.

### Neosync: referential integrity, and being open source

The feature users praised most was the one nobody else got right. From the Show HN thread, a developer who had been fighting the same problem with ClickHouse's obfuscator:

> The problem I'm running into is referential integrity, as importing the anonymized data is raising unique and foreign key violations. The obfuscator tool is pretty minimal and has few knobs to tweak its output... Your tool looks interesting, and it seems that you directly address the referential integrity issue, which is great.
> — imiric ([HN, 22 May 2024](https://news.ycombinator.com/item?id=40446843))

Another, comparing it to the unmaintained `datanymizer`:

> Great to see such a project. We are using datanymizer right now but it has gone unmaintained and we are using my patched version and it is working pretty well for us.
> — gregwebs ([HN, 23 May 2024](https://news.ycombinator.com/item?id=40449655))

Neosync's own founder identified referential integrity as the hard part and the reason people picked them:

> yeah the referential integrity and constraints part is usually the most complicated part and everyone does things differently which adds another layer of complexity on it
> — edrenova ([HN, 22 May 2024](https://news.ycombinator.com/item?id=40446813))

And the open-source-first stance was the actual go-to-market:

> Open source is a great way to get adoption from mid-size and enterprise sized companies who have long procurement cycles and stringent data privacy and security programs. A developer can fork your repo and run it locally in an hour if your project is open source versus if it's not, well, then you're in procurement hell for the next 6-9 months.
> — Evis Drenova, "Introducing Neosync" ([Wayback](http://web.archive.org/web/20250803043832/https://www.neosync.dev/blog/introducing-neosync))

Neosync reached [4,141 GitHub stars](https://github.com/nucleuscloud/neosync) — roughly 12× Snaplet Snapshot's 328 — which is a decent proxy for how much developers wanted this to exist.

---

## 3. What users abandoned them over

### The cloud was the product, and users said out loud they would not use it

The single most direct piece of evidence in the whole corpus. On the Neosync Show HN, a prospective user works through the value proposition and rejects the business model in the same comment:

> As a side note, I'm not sure I understand what the value proposition of your Cloud service would be. If the original data needs to be exported and sent to your Cloud for anonymization, it defeats the entire purpose of this process, and only adds more risk. I don't think that most companies looking for a solution like this would choose to rely on an external service. Thanks for releasing it as open source, but **I can't say that I trust your business model to sustain a company around this product.**
> — imiric ([HN, 22 May 2024](https://news.ycombinator.com/item?id=40446843)) (emphasis added)

The founder's reply concedes the segmentation problem:

> Folks use the cloud service because they don't have the resource or time to deploy/run the OSS offering themselves. These are usually startups who are okay with us streaming their data and anonymizing it and sending it back to them.
> — edrenova ([HN](https://news.ycombinator.com/item?id=40447065))

So: the people willing to pay for hosting are the ones least worried about data leaving their perimeter, and the people most worried — the regulated enterprises the whole product is aimed at — self-host for free. That is the trap. Fifteen months later the repo was archived.

### A loud contingent rejects the *category*, not the product

The top-voted critical comment on the Neosync launch is a flat refusal of anonymised production data as a practice:

> While you may be able to change or delete obvious PII, like names, every bit of real data in aggregate leads to revealing someone's identity... Just stick with fuzzing random data during development... **Keep production data in production.**
> — blopker ([HN, 22 May 2024](https://news.ycombinator.com/item?id=40445047))

He follows up with the operational objection, which is the more dangerous one commercially:

> Once production data is floating around different environments, it will be easy to lose track of. Then the first GDPR delete request comes in. Was this data synthetic? Was it real? I think Joe has a copy on his laptop, he's on vacation?
> — blopker ([HN](https://news.ycombinator.com/item?id=40445417))

And a practitioner with medical-records experience:

> Mid 2000s, I worked with electronic medical records. I eventually determined anon isn't worthwhile. For starters, deanon will always beat anon. This statement is unambiguously true, per the research. Including the differential privacy stuff.
> — specialist ([HN, 30 May 2024](https://news.ycombinator.com/item?id=40526241))

The thread argues both ways at length ([one reply](https://news.ycombinator.com/item?id=40446466) makes the case that random data cannot reproduce production's scale or distributions). The point for lazysnap is not who is right. It is that a meaningful slice of the exact target audience believes the category is a bad idea, and a tool in this space must answer that objection in its own defaults — by keeping the copy small, local, and disposable, and by never silently letting untransformed columns through.

### Subsetting hit a wall users could not get past

The clearest abandonment story in the Neosync tracker. A user wants to subset from a "starting point" table that is both a parent and a child:

> My 'Starting point' table has foreign keys to other tables as well as other tables have foreign keys to it. I want to apply a subset filter to this table, and all the tables related to it should apply that subset filter to it. But it looks like my subset filter is not propagated to the tables that have a foreign key to it
> — Yarn-e ([issue #3227, 7 Feb 2025](https://github.com/nucleuscloud/neosync/issues/3227))

The maintainer's answer is an architectural confession:

> Yeah it's a current design choice and system limitation. Foreign Key constraints are effectively directional graphs (DAGs) that follow a parent->child hierarchy. We've had a few different users (as well as a lot of internal discussions) ask about changing our subsetting logic to treat any table that has a filter applied to it as the _root_ node and ignoring the directionality of the foreign key constraints and instead treat them as graph edges. This is more complex in practice...
> — nickzelei ([issue #3227, 26 Feb 2025](https://github.com/nucleuscloud/neosync/issues/3227))

User's final reply:

> Sadly it isn't possible in our case... Thanks for giving this explanation! I'll keep an eye out on the roadmap in case it would come up there!

The feature never shipped; the issue is still open on an archived repo. **This is precisely lazysnap's core mechanic** — "a root table, a row count, parents to completeness, children with caps". Neosync could not do the parent-by-child direction and lost the user.

### The docs and the CLI drifted apart, repeatedly

Snaplet renamed a command's meaning mid-flight and broke the official Supabase guide:

```
npx snaplet generate --sql
  📢 Change notice:
  Were you expecting this command to generate data? It now does something different.
```
— reported in [supabase/supabase#21089](https://github.com/supabase/supabase/issues/21089), 7 Feb 2024

Config-file location drift cost a user a working install; the fix was to hand-edit a JSON file the tool was supposed to write:

> Ok, the `.snaplet/config.json` was created but only with the `projectId` key. ✅ Thank you, I manually added the `adapter` key and it fixed it
> — Aymericr ([snaplet/docs#118](https://github.com/snaplet/docs/issues/118), 31 May 2024)

Nine months after shutdown the docs were still wrong, and there was no longer anyone to fix them: [supabase-community/snapshot#18, "Outdated documentation on configuring select and transform"](https://github.com/supabase-community/snapshot/issues/18) (17 Jan 2025) is still open.

### Node/npm packaging rot killed installs

`@snaplet/snapshot` shipped a native `better-sqlite3` dependency pinned to a version that did not support Node 22 — meaning the tool broke on the LTS release without anyone touching it:

> `@snaplet/snapshot` depends on `better-sqlite3` version 8.5.0, but that library only added Node 22 support in version 10, so you will probably need to override that somehow if you want to use Node 22.
> — smcgivern ([supabase-community/snapshot#15](https://github.com/supabase-community/snapshot/issues/15), 20 Nov 2024)

The issue was opened Oct 2024 and is still open; a fix PR was raised by a stranger 13 months later. A related report for Bun: [#21](https://github.com/supabase-community/snapshot/issues/21).

### The real cost of abandonment, in one team's words

A UK food bank charity running Snaplet Seed:

> Snaplet has shut down. They've open sourced their "seed" code... **We can't generate new seed data until this is resolved**
> ...
> The build is still failing on seed.ts because of type changes - these need fixing before a fresh checkout builds. **Currently I rename seed.ts to be seed.ts.txt so that the build succeeds.**
> — stonelink, [LambethFoodbank/foodbankapp#688](https://github.com/LambethFoodbank/foodbankapp/issues/688), opened Jun 2025, quoting internal notes from Oct 2024 and Mar 2025

Nine months of a broken seed pipeline, resolved by commenting out the tool.

And downstream projects unwinding the dependency after Neosync's archival:

> Given the recent archival of the neosync project, we will want to stop supporting the library eventually.
> — eminano, [xataio/pgstream#551](https://github.com/xataio/pgstream/issues/551), 29 Sep 2025

### Neosync's AI dependency became a liability for Snaplet Seed's users

After Snaplet Seed went to the community, the most-upvoted open requests are all about escaping the hosted LLM it depends on: [Ollama support](https://github.com/supabase-community/seed/issues/198) (open since Sep 2024, "+1" four times, one user asking "How is this still open?!"), [custom OpenAI-compatible endpoint](https://github.com/supabase-community/seed/issues/204), [AI model rate limiting](https://github.com/supabase-community/seed/issues/210), and:

> The banner is very annoying as it pops up every time. As far as I can tell, there's currently no way to disable it. Please consider adding a config option to opt out. Or better yet, make it opt-in.
> — philipbel ([seed#211](https://github.com/supabase-community/seed/issues/211), Aug 2025)

A local dev tool that phones a third-party model on every run is a tool that stops working when the company does.

---

## 4. What the product became that it did not start as

### Snaplet: snapshot tool → cloud data platform → AI seed-script generator

| Era | Headline | Source |
|---|---|---|
| Jan 2021 | "Get a safe, minimized version of your production database on your laptop in minutes" | [Wayback](http://web.archive.org/web/20210118202109/https://www.snaplet.dev/) |
| Jun 2022 | "Say goodbye to seed scripts — Snaplet copies your Postgres database, transforming personal information" | [Wayback](http://web.archive.org/web/20220601160539/https://www.snaplet.dev/) |
| Apr 2023 | "Quit writing seed scripts, start shipping features. Get production-accurate data **and preview databases** to code against" | [Wayback](http://web.archive.org/web/20230411000318/https://www.snaplet.dev/) |
| Jan 2024 | "Snaplet uses **generative ai** to give you realistic, production-like data" | [Wayback](http://web.archive.org/web/20240125104719/https://www.snaplet.dev/) |
| Jun 2024 | "**Instant seed data** for your relational database. Ditch the seed script! ... AI-generated mock data for your local database" | [Wayback](http://web.archive.org/web/20240624152127/https://www.snaplet.dev/) |

Three years of drift from *copy production safely* to *generate fake data from a schema with an LLM*. Note that the destination — synthetic data generation from a schema alone — is the **first item on lazysnap's own non-goals list**.

Along the way it also acquired a hosted serverless database product ("preview databases", backed by Neon), a VS Code extension, a Netlify plugin, a Vercel action, a GitHub Action, and a local proxy:

- **Preview databases**: "instant serverless database... allows you to view that database without needing to download and install the Snaplet CLI" ([docs](https://github.com/snaplet/docs-old/blob/main/docs/04-references/preview-databases.md)). The same doc admits two incompatible database providers existed simultaneously and advises: "For now, we'd recommend using Snaplet Cloud to create preview databases."
- **VS Code extension**: by the time of the final quickstart, the recommended path was *sign in to Snaplet Cloud → capture via a web wizard → install a VS Code extension → point your local `DATABASE_URL` at a proxy on `localhost:2345` that fronts a cloud preview database* ([quickstart](https://github.com/snaplet/docs-old/blob/main/docs/02-quickstart.md)).

That final workflow is the inversion of the founding promise. The 2021 pitch was "on your laptop". The 2024 default was "connect your production database to our cloud, and point your app at our proxy."

### Neosync: Kubernetes PaaS → data sync tool → "Data Security Platform"

Neosync is a pivot from a *different* failed product. The founders' own account:

> When Nick and I started Nucleus Cloud Corp, our vision was to help developers build faster, more secure and resilient applications... We spent a year on the Nucleus Cloud Platform and were proud of the work that we had done. But at the end of the day, it was clear to us that cloud infrastructure is a tough game for startups. So we made the decision to work on other ideas.
> — [Introducing Neosync](http://web.archive.org/web/20250803043832/https://www.neosync.dev/blog/introducing-neosync), Dec 2023

Nucleus was "a Kubernetes platform for both devs and ops" sold at "around $35k/license" ([Launch HN, Jun 2023](https://news.ycombinator.com/item?id=36197880)). Six months later the same team, the same GitHub org (`nucleuscloud`), the same Helm-and-Kubernetes instincts, shipped a database anonymisation tool. The architectural weight of the first product is inherited wholesale by the second — see §5.

Then the scope kept widening. In Dec 2023 the description was "open source data replication and anonymization". The final repository description reads:

> **Open Source Data Security Platform** for Developers to Monitor and Detect PII, Anonymize Production Data and Sync it across environments.
> — [github.com/nucleuscloud/neosync](https://github.com/nucleuscloud/neosync)

"Monitor and Detect PII" is a security-posture product, not a developer tool. The launch post already contains the seed of the drift — a second audience:

> For ML engineers, they need access to high quality synthetic data to train and fine tune models... Imagine being able to generate a million rows of synthetic data that you can use to fine-tune your model with just a prompt.
> — [Introducing Neosync](http://web.archive.org/web/20250803043832/https://www.neosync.dev/blog/introducing-neosync)

Two audiences (app developers and ML engineers), two jobs (anonymise real data, generate fake data), from month one.

---

## 5. Where complexity accumulated

### Snaplet: in the subset config

The subsetting configuration surface, in the final documented form, has these knobs — for one operation:

`targets[]` (each with `table`, plus one or more of `percent`, `rowLimit`, `where`, and optionally `orderBy`), `enabled`, `keepDisconnectedTables`, `followNullableRelations`, `maxCyclesLoop`, `maxChildrenPerNode`, `eager`, `traversalMode`
([reduce docs](https://github.com/snaplet/docs-old/blob/main/docs/04-references/data-operations/04-reduce.md), [live capture docs](https://snaplet-snapshot.netlify.app/snapshot/core-concepts/capture))

And the recommended way to use them:

> When setting up Snaplet for the first time, we recommend setting this parameter to 0 and to gradually increment it until the subset of data you fetch through a relationship is enough for your use case.
> — [Snaplet subset docs, on `maxCyclesLoop`](https://github.com/snaplet/docs-old/blob/main/docs/04-references/data-operations/04-reduce.md)

That is a documented instruction to binary-search a tuning parameter by trial and error. It is also an admission that the tool cannot tell the user what a good value is, so the user must. Compare lazysnap's principle: *"Documentation is never the fix for a confusing first run. Change the default or the question."*

The docs also disclose that the primary control does not do what it says:

> Note that the `percent` / `rowLimit` specified in the subset config may not be exact... As such, a 5% subset specified against a specific table may ultimately include more than 5% of the actual database.
> — [subset docs](https://github.com/snaplet/docs-old/blob/main/docs/04-references/data-operations/04-reduce.md)

### Snaplet: in the default that was not safe

Three transform modes, and the default is the unsafe one:

> - **`unsafe`** (default): The data for columns not specified in the config is simply copied over as is without transformation.
> - **`strict`**: Fail the capture if any columns, tables or schemas have not been specified in the config.
> - **`auto`**: Automatically transform the data for any columns, tables or schemas that have not been specified in the config.
> — [transform docs](https://github.com/snaplet/docs-old/blob/main/docs/04-references/data-operations/02-transform.md)

A tool whose whole reason to exist is not putting customer emails on developer laptops shipped, as its default, "copy everything you did not explicitly name". And in their own documented example output, the PII detector fails:

```
⠋ Transform: Detecting PII fields...Smart shape prediciton failed
```
— literal text from the [transform docs setup transcript](https://github.com/snaplet/docs-old/blob/main/docs/04-references/data-operations/02-transform.md) (typo theirs)

This is the exact failure lazysnap's threat model forbids. lazysnap's stated rule — *"Anything that might be personal data is masked unless the user opts a column out"* — is `strict`+`auto` by default with no `unsafe` mode at all.

### Snaplet: in a breaking config migration

Version 0.50.0 restructured configuration and 0.60.0 removed the old form:

> Previously, you would have files such as `.snaplet/transform.ts`, `.snaplet/schema.json`, `.snaplet/structure.d.ts`. Now, you will have only one file `snaplet.config.ts`... **The "old" configuration will be removed and no longer working in version `0.60.0`.**
> — [migration guide](https://github.com/snaplet/docs-old/blob/main/docs/05-guides/migration-new-config.md)

The migration also removed a capability people were using — dynamically computed transform configs — because type-safety required static configs. And the migration tool for it lived *in the cloud dashboard*: "For our cloud users, we created a migration tool... available in your project under the `Data Editor` tab. However, it might have some issues depending on the complexity of your current configuration."

### Neosync: in the deployment footprint

Self-hosting Neosync means running, at minimum, the services in its own `compose.yml`: `app` (Next.js frontend), `api`, `worker`, `db` (Postgres for Neosync's own config), `redis`, plus `api-seed` and `temporal-seed` init containers — and **Temporal**, which the compose file seeds but the Kubernetes guide explicitly declines to cover:

> Deploying a Postgres database and Temporal instances are not covered under this guide. See the external dependency section below for more information regarding these resources.
> — [Kubernetes deploy docs](https://github.com/nucleuscloud/neosync/blob/main/docs/docs/deploy/kubernetes.md)

Four separate Helm charts were published (api, app, worker, umbrella). An independent 2026 review puts the full picture as Temporal "requiring its own database and Helm deployment... a hard dependency", plus PostgreSQL, optional Redis and optional Keycloak ([xata.io](https://xata.io/blog/the-5-best-data-anonymization-tools-for-development-teams-in-2026)) — corroborated by the compose file and deploy docs above. "Improve Temporal Self-Hosted Documentation" was filed as [issue #1141](https://github.com/nucleuscloud/neosync/issues/1141) in January 2024, one month after launch.

The maintainer's own account of choosing Temporal is candid about the size of the bet:

> Being a startup, going with Temporal is a big choice, but is a technology that can be used small or big. It's a technology that we will likely never grow out of
> — Nick Zelei, [Exploring How Neosync Leverages Temporal](http://web.archive.org/web/20250803053743/https://www.neosync.dev/blog/leveraging-temporal), Jul 2024

Users paid for it. A developer trying to sync from a local Postgres container:

> I was surprised to get snagged here given I'd expect a lot of users might have a DB running on their machine (docker or otherwise), but maybe that's not a popular use case.
> — patricktyndall ([issue #3420](https://github.com/nucleuscloud/neosync/issues/3420), 28 Mar 2025)

The cause was Neosync's own Docker network isolating it from the user's database — a self-inflicted wound from shipping a multi-container platform instead of a binary. Another user could not connect at all: ["Unable to connect Neosync to Temporal – context deadline exceeded on DescribeNamespace"](https://github.com/nucleuscloud/neosync/issues/3560).

### Neosync: in the changelog

The `docs/blog` directory is a fortnightly changelog and reads as a two-year record of surface-area growth ([full list](https://github.com/nucleuscloud/neosync/tree/main/docs/blog)):

> public APIs (Dec 2023) → custom code transformers → conditional transformers → circular dependencies → **Terraform provider** (Feb 2024) → foreign-key transformations → metrics → partial table syncs → Postgres permissions → **AI generate jobs** (May 2024) → flexible schema references → real-time validation → **MongoDB** (Jun 2024) → job cloning → JavaScript transformers → **DynamoDB** (Aug 2024) → **SQL Server** (Aug 2024) → default transformers → bulk transformers → **CLI sync** (Oct 2024) → anonymization API → column strategies → **EU infrastructure** (Nov 2024) → **job hooks** (Dec 2024) → **RBAC** (Dec 2024) → v5 → Python SDK → **webhooks** (Feb 2025) → connection security → **Slack integration** (Mar 2025) → **PII detection** (Mar 2025)

Five database backends, two object stores, a Terraform provider, a Python SDK, an LLM integration, RBAC, SSO, audit logs, webhooks, account hooks, Slack. Meanwhile the CLI — the thing a developer on a laptop actually touches — got its first substantial sync improvements ten months after launch.

Two things are conspicuous in that list. First, **the enterprise checklist arrives before the developer experience is finished**: RBAC (Dec 2024) lands before the parent-by-child subsetting problem raised two months later is ever addressed, and it never is. Second, **breadth beat depth**: DynamoDB and MongoDB and SQL Server all shipped in a nine-week window in mid-2024, while [issue #3227](https://github.com/nucleuscloud/neosync/issues/3227) shows the Postgres subsetting graph was still directional-only in 2025.

### Both: in transformer edge cases

Enumerating transformers is cheap; making each correct is not. Neosync's tracker in its final year is dominated by transformer bugs: [`transformEmail` crashes on a negative computed length](https://github.com/nucleuscloud/neosync/issues/3540); [fullname transformer wrong with `preserve_length = true`](https://github.com/nucleuscloud/neosync/issues/3536); [generating a UUID for a referenced primary key fails the whole job](https://github.com/nucleuscloud/neosync/issues/3433); [tables with one column break jobs](https://github.com/nucleuscloud/neosync/issues/3430); [`'null'` JSON converted to SQL NULL](https://github.com/nucleuscloud/neosync/issues/3304). "40+ transformers" was the launch headline in December 2023; by 2026 an outside review counts "50+" ([xata.io](https://xata.io/blog/the-5-best-data-anonymization-tools-for-development-teams-in-2026)). Each one is a maintenance liability with its own edge cases across five database dialects.

---

## 6. Pricing: what they tried, and how it changed

### Snaplet

| Date | Free | Paid | Model |
|---|---|---|---|
| Sep 2023 | 1 GB snapshot storage, 5 h snapshot compute, 2 GB transfer, 10 h preview-DB usage / month | **Pro $30/team/month** with 10 GB / 50 h / 20 GB / 100 h allowances; opt-in usage-based overage billing with a hard spend cap | Metered infra resale, per team not per seat |
| Feb 2024 | unchanged | Pro $30/team/month; **overage billing replaced by "we'll reach out to chat to you about a custom pricing plan"** | Same headline, manual overage handling |
| Aug 2024 | unchanged | Pro $30/team/month for Snapshot; **Seed pricing "TBC"**, free in beta | Two products, one unpriced |

Sources: [Sep 2023](http://web.archive.org/web/20230930182525/https://www.snaplet.dev/pricing), [Feb 2024](http://web.archive.org/web/20240228045459/https://www.snaplet.dev/pricing), [Aug 2024](http://web.archive.org/web/20240802130407/https://www.snaplet.dev/pricing)

Three observations.

**$30 per team per month for an unlimited-seat product is not a business.** They knew why they picked it and said so explicitly:

> No, we charge per team, irrespective of how many people on the team use Snaplet. We think per seat costs disincentivize teams to use tools like Snaplet, and we want your entire team to benefit
> — [pricing FAQ](http://web.archive.org/web/20230930182525/https://www.snaplet.dev/pricing)

That is a good product instinct and a fatal pricing one. A ten-person team paying $30/month yields $360/year against a cost base that includes S3 storage, S3 egress, Fargate compute per capture, and Neon compute for preview databases — all itemised by Snaplet themselves in the same FAQ.

**The paid tier's value was storage and hosting, not the tool.** Every metered unit — snapshot storage, snapshot compute, snapshot data transfer, preview database hours — is infrastructure resale. The capture and transform logic, the part that took years to build, was free and self-hostable. lazysnap's non-goals already exclude every one of those meters ("Scheduling, retention, or sharing snapshots between people", "A web UI or a hosted service"), which is the right call for a tool but means there is no version of Snaplet's revenue model available.

**They walked back automated overages.** Sep 2023: "you will either be billed pro rata on a per-usage basis" with a self-serve spend cap. Feb 2024: "If you're a Pro plan user and you exceed your allocations repeatedly, we'll reach out to chat to you." Aug 2024 adds: "You'll never be billed pro-rata by Snaplet." That is a company discovering that usage-based billing on a $30 plan costs more in support than it collects.

At the end, the last homepage before the shutdown made **Seed** — the free, unpriced, beta product — the primary call to action ("Try Seed"), while the only product with a price sat behind a secondary tab ([Aug 2024 pricing](http://web.archive.org/web/20240802130407/https://www.snaplet.dev/pricing)).

### Neosync

| Date | Free tier | Team | Model |
|---|---|---|---|
| May 2024 | 100k records/month, 1 user | **$299/mo**, 5M records ($60 per additional 1M), 5 users ($10/user after) | Flat + record overage + seats |
| Sep 2024 | 30k records/month | **"Contact Us"** — no public price | Price removed |
| Oct 2024 | 20k–30k records/month | **Pay-as-you-go**: $200/mo platform fee + $0.0005/record (100k–1M), $0.00025/record (1M–5M), custom above | Pure usage-based, with a calculator |
| Feb 2025 | **Free tier removed**, replaced by a 14-day trial | Pay-as-you-go | Trial-gated |
| May 2025 | 14-day trial | **$300/mo flat, unlimited records, unlimited users** | Back to flat |

Sources: [May 2024](http://web.archive.org/web/20240518130236/https://www.neosync.dev/pricing), [Sep 2024](http://web.archive.org/web/20240905232639/https://www.neosync.dev/pricing), [Oct 2024](http://web.archive.org/web/20241007140104/https://www.neosync.dev/pricing), [Feb 2025](http://web.archive.org/web/20250206123739/https://www.neosync.dev/pricing), [May 2025](http://web.archive.org/web/20250516010948/https://www.neosync.dev/pricing)

Five pricing pages in twelve months, and the journey is a loop: flat with overages → hidden → usage-based → usage-based with no free tier → flat again at almost exactly the original price. Along the way the free tier shrank 5× (100k → 20k records) and then vanished. The Oct 2024 page even shipped internally inconsistent numbers — the plan card says "20k records/mo" while the comparison table below says "30k/month" — the fingerprint of a team changing pricing faster than they can update a page.

Notably, "Simple, Transparent Pricing / Pricing shouldn't be complicated, so we made it easy" (May 2024) became a page with a pricing calculator and a four-band per-record tier table (Oct 2024). The enterprise tier absorbed everything a self-hoster would want: SSO, RBAC, webhooks, audit logs, EU region, streaming mode ([May 2025](http://web.archive.org/web/20250516010948/https://www.neosync.dev/pricing)) — until July 2025, when they [switched the licence check to always return true](https://github.com/nucleuscloud/neosync/commit/4a2c971c81b098ec8cb6661a1b7883f1f56b40f3) and gave it all away a month before archiving.

---

## 7. What the founders said about why

### Snaplet — Peter Pistorius

The shutdown post, in full on the relevant point:

> Snaplet will be shutting down on 31 August 2024. Since starting Snaplet in 2021, our mission has been to empower developers with safe and easy access to production-realistic data. **While we've helped many developers since then, we have not reached the necessary adoption levels to continue Snaplet.**
> — [Snaplet is shutting down](http://web.archive.org/web/20240716025550/https://www.snaplet.dev/post/snaplet-is-shutting-down), 1 Jul 2024

Not "we ran out of money", not "a competitor beat us" — *adoption*. Three years, real love from named developers, a Product Hunt #2, and not enough people using it.

On why the code lives on:

> I built Snaplet because I believe developers write better software when they have access to production-like data. Although the company is closing, my belief remains strong, so we are open-sourcing the tools we've built.
> — Peter Pistorius, quoted in [Supabase's announcement](https://supabase.com/blog/snaplet-is-now-open-source)

Correcting the record eighteen months later, on Hacker News:

> Hey! Snaplet founder here. Want to clarify that it was not acquired by Supabase; I shutdown the startup and found roles for some of the team at Supabase.
> — pistoriusp ([HN, 6 Jan 2026](https://news.ycombinator.com/item?id=46518012))

The reason he had to correct it, from the comment he was replying to:

> Reminds me a bit of Snaplet before it embarked on its incredible journey to get acquired by Supabase and shut down. I like the concept but the painpoint has never been around creating realistic looking emails and such like, but **creating data that is realistic in terms of the business domain and in terms of volume.**
> — ljm ([HN, 6 Jan 2026](https://news.ycombinator.com/item?id=46513101))

That is a customer telling you, after the fact, that you solved the easy half.

Earlier, on the Jamstack Radio podcast while Snaplet was still free, Pistorius described the business model as deferred: *"It's completely free... What we're trying to do is build the best product for individuals and once we get to teams or bigger companies, we'll start thinking about pricing"* ([Heavybit, Jamstack Radio ep. 102](https://www.heavybit.com/library/podcasts/jamstack-radio/ep-102-database-accessibility-with-peter-pistorius-of-snaplet)). Snaplet raised a seed round (investors reported as including Netlify's Jamstack Innovation Fund, boldstart, and basecase; totals reported by aggregators vary and are **unverified** here — the Netlify fund participation is attested by Snaplet's own site banner, [Wayback](http://web.archive.org/web/20230411000318/https://www.snaplet.dev/)).

Supabase's own framing of the ending is two words long:

> Startups are hard. One of our favorite startups, Snaplet, is shutting down.
> — Paul Copplestone, [Supabase blog](https://supabase.com/blog/snaplet-is-now-open-source)

### Neosync — Evis Drenova and Nick Zelei

On the pivot into the space (from a failed Kubernetes PaaS):

> We spent a year on the Nucleus Cloud Platform and were proud of the work that we had done. But at the end of the day, it was clear to us that cloud infrastructure is a tough game for startups. So we made the decision to work on other ideas. When we started thinking of other ideas, the first place we went was to think about what our customers had asked for.
> — [Introducing Neosync](http://web.archive.org/web/20250803043832/https://www.neosync.dev/blog/introducing-neosync)

On the open-source bet, reflecting publicly in mid-2024 — the pros he lists are adoption and enterprise trust; the cons are a direct description of what killed the business:

> **Pros:** Enterprises don't want to put their sensitive data in someone else's infra; community engagement helps word-of-mouth; it facilitated early enterprise adoption.
> **Cons:** Reduced focus on sales could create misleading signals about traction; limited visibility into actual user adoption metrics; administrative demands from community management; complexity managing both open source and commercial versions simultaneously; uncertainty around future monetization.
> — Evis Drenova, ["Neosync: The pros and cons of open source"](https://www.linkedin.com/posts/evisdrenova_when-we-first-started-working-on-neosync-activity-7224483405667745792-7TRm) (paraphrased from the post's own bullet list)

"Misleading signals about traction" and "limited visibility into actual user adoption" are the two sentences to remember. 4,141 stars, hundreds of self-hosting companies, and no way to tell whether any of them would pay.

On the ending, before any public announcement, in the issue tracker:

> There are no current plans for any continued maintenance. We are still working with the acquirer on handing them the repository but ultimately it will be up to them as to what they decide to do with the repository. **If you're using Neosync I'd recommend forking it at the current time and making any changes you wish.** A few weeks ago we removed any license restrictions that were in place behind pro features so everything is currently available in the latest release.
> — nickzelei ([issue #3565](https://github.com/nucleuscloud/neosync/issues/3565), 25 Aug 2025)

The acquirer's framing makes clear it was the team and the technique that were bought, for internal use in a HIPAA-scoped healthcare product — not the product:

> bringing on the Neosync team allows us to drive advancements that will reverberate across the mental health care sector.
> — Aaltan Ahmad, Head of Security, Grow Therapy ([PR Newswire, 25 Sep 2025](https://www.prnewswire.com/news-releases/grow-therapy-raises-the-privacy-bar-in-mental-health-302567153.html))

> This approach brings modern privacy engineering practices, like masking and synthetic data generation, direct to the healthcare workflow.
> — Evis Drenova, now Staff Product Manager at Grow ([PR Newswire](https://www.prnewswire.com/news-releases/grow-therapy-raises-the-privacy-bar-in-mental-health-302567153.html))

Y Combinator lists Neosync's status as **Acquired**, team size 3 ([YC](https://www.ycombinator.com/companies/neosync)). Drenova has since [left the earn-out](https://talent.substack.com/p/why-i-joined-evis-drenova-entire).

---

## 8. Five things we will not copy

**1. A default that copies untransformed data.**
Snaplet's transform mode defaulted to `unsafe` — "data for columns not specified in the config is simply copied over as is without transformation" ([docs](https://github.com/snaplet/docs-old/blob/main/docs/04-references/data-operations/02-transform.md)) — while the accompanying PII detector visibly failed in their own documented transcript ("Smart shape prediciton failed"). A tool that exists to keep customer emails off laptops shipped a default that puts customer emails on laptops. lazysnap's CONCEPT.md already forbids the wholesale-disable flag; this evidence says go further and forbid the *mode* — there should be no `unsafe`, only "masked" and "explicitly opted out, per column, recorded in `lazysnap.yml`". *Evidence: [transform docs](https://github.com/snaplet/docs-old/blob/main/docs/04-references/data-operations/02-transform.md).*

**2. Tuning knobs in place of a working default.**
Snaplet's subset config accumulated `percent`, `rowLimit`, `where`, `orderBy`, `keepDisconnectedTables`, `followNullableRelations`, `maxCyclesLoop`, `maxChildrenPerNode`, `eager`, `traversalMode` — and the docs instructed users to set `maxCyclesLoop` to 0 and "gradually increment it until the subset of data you fetch through a relationship is enough for your use case", while separately admitting a 5% subset "may ultimately include more than 5%". That is the whole hard problem handed back to the user as homework. lazysnap's plan step must decide these itself, report what it decided in plain language, and only then write them into `lazysnap.yml` as a record. *Evidence: [subset docs](https://github.com/snaplet/docs-old/blob/main/docs/04-references/data-operations/04-reduce.md).*

**3. A control plane between the developer and their own database.**
Neosync's minimum self-hosted footprint is app + api + worker + Postgres + Redis + Temporal, with Temporal explicitly out of scope of the deploy guide, four Helm charts, and a Docker network that stopped a user connecting to a Postgres container on his own machine ("I was surprised to get snagged here"). Snaplet ended up with a cloud wizard, a VS Code extension, a proxy on `localhost:2345` and a hosted preview database in the recommended path. Both drifted away from the terminal. lazysnap's "one static binary" and "the TUI is a thin layer" rules are the correct reaction, and they are load-bearing, not stylistic. *Evidence: [compose.yml](https://github.com/nucleuscloud/neosync/blob/main/compose.yml), [Kubernetes deploy docs](https://github.com/nucleuscloud/neosync/blob/main/docs/docs/deploy/kubernetes.md), [issue #3420](https://github.com/nucleuscloud/neosync/issues/3420), [Snaplet quickstart](https://github.com/snaplet/docs-old/blob/main/docs/02-quickstart.md).*

**4. Breadth before the first path is excellent.**
Neosync shipped MongoDB, DynamoDB, SQL Server, S3, GCS, a Terraform provider, a Python SDK, RBAC, webhooks and a Slack integration inside eighteen months, and *still* could not subset a parent table by a filter on its child — the feature a user asked for in Feb 2025 and never got ([#3227](https://github.com/nucleuscloud/neosync/issues/3227)). Snaplet added preview databases, a VS Code extension, Netlify and Vercel plugins while its config file format broke twice and its docs went stale. lazysnap's non-goal list ("Databases other than PostgreSQL... come after the Postgres path is excellent") is the right instinct; the failure mode here is that both companies wrote that sentence and then did not obey it. *Evidence: [Neosync changelog](https://github.com/nucleuscloud/neosync/tree/main/docs/blog), [#3227](https://github.com/nucleuscloud/neosync/issues/3227), [migration guide](https://github.com/snaplet/docs-old/blob/main/docs/05-guides/migration-new-config.md).*

**5. A runtime dependency on a service we operate.**
Snaplet Seed calls a hosted LLM, so when the company died the tool degraded into a nag banner and rate-limit errors, and the top community requests became [Ollama support](https://github.com/supabase-community/seed/issues/198), [a custom OpenAI-compatible endpoint](https://github.com/supabase-community/seed/issues/204) and [a way to turn the AI banner off](https://github.com/supabase-community/seed/issues/211). Snaplet Snapshot's *storage* was the paid product, so a shutdown meant customers had to go rebuild snapshot storage themselves. Neosync Cloud is gone and `app.neosync.dev` no longer resolves. lazysnap should have no runtime call to anything we operate — no telemetry gate, no model call, no licence check, no registry — such that the binary a user has today works identically in five years with us gone. *Evidence: [seed#198](https://github.com/supabase-community/seed/issues/198), [seed#211](https://github.com/supabase-community/seed/issues/211), [seed#210](https://github.com/supabase-community/seed/issues/210), DNS lookups above.*

---

## 9. Three things we must copy

**1. Deterministic masking as a separate, tiny, dependency-free library.**
Copycat — "like faker.js, but deterministic: for any given input it'll always produce the same output" — is the only piece of Snaplet that thrived. Two years after the company died it does **121,478 downloads a week**, twelve times the snapshot tool it was built to serve, and an HN user naming what he missed about Snaplet named it ("good (stable) generators for names, emails"). Determinism is also what makes joins survive masking, which is the technical crux of lazysnap's step 4. Build this as a standalone, importable, separately-usable unit with its own tests and its own name, so that if lazysnap fails, this outlives it. *Evidence: [npm](https://registry.npmjs.org/@snaplet/copycat) (121,478/wk vs 9,808/wk for snapshot), [Supabase blog](https://supabase.com/blog/snaplet-is-now-open-source), [HN comment](https://news.ycombinator.com/item?id=41867092).*

**2. Introspect first, propose a config, ask the human to review it.**
Snaplet's `setup` is the best-designed thing either company shipped: connect, introspect, detect PII candidates, *generate* a config with suggestions, and print "Snaplet has introspected your database structure and generated suggested transformations for your data... Please review them." Crucially the docs also state "A user account is **not required**". That is lazysnap's zero-config principle already validated in the field — emit configuration as a record of what happened, never demand it up front. The two things to fix are the ones Snaplet got wrong: make the detector's failure mode *safe* rather than pass-through, and never make the review step happen in a web dashboard. *Evidence: [transform docs setup transcript](https://github.com/snaplet/docs-old/blob/main/docs/04-references/data-operations/02-transform.md).*

**3. Referential integrity as the headline feature, and say so.**
It is the single thing users praised Neosync for, unprompted, in public, by name — the ClickHouse-obfuscator user switching because "it seems that you directly address the referential integrity issue, which is great", the datanymizer user shopping for a replacement, the founder confirming "the referential integrity and constraints part is usually the most complicated part". It is also where Neosync's design ran out of road (parent-by-child, [#3227](https://github.com/nucleuscloud/neosync/issues/3227)) and where Snaplet's precision claims wobbled ("a 5% subset... may ultimately include more than 5%"). lazysnap already plans to verify FK integrity in the target and print `✓ foreign keys verified` — that check should be non-optional, run on every load, and fail the run loudly, because it is the promise that differentiates this from `pg_dump | sed`. And the subsetting graph should be undirected from the start: treat any filtered table as a root and follow edges both ways, which is exactly what Neosync's maintainer said users kept asking for and they never built. *Evidence: [HN, imiric](https://news.ycombinator.com/item?id=40446843), [HN, gregwebs](https://news.ycombinator.com/item?id=40449655), [HN, edrenova](https://news.ycombinator.com/item?id=40446813), [Neosync #3227](https://github.com/nucleuscloud/neosync/issues/3227), [Snaplet subset docs](https://github.com/snaplet/docs-old/blob/main/docs/04-references/data-operations/04-reduce.md).*

---

## 10. Open questions and gaps

- **No formal Snaplet retrospective exists.** Searches for a Pistorius postmortem, "lessons learned" post, or founder interview after July 2024 turned up nothing beyond the shutdown post, the Supabase quote, and the one-line HN correction. His pre-shutdown [Jamstack Radio interview](https://www.heavybit.com/library/podcasts/jamstack-radio/ep-102-database-accessibility-with-peter-pistorius-of-snaplet) predates monetisation. If a longer account exists, it is likely on X/Twitter or in the (now closed) Snaplet Discord, neither reachable from this environment.
- **Snaplet's funding total is unverified.** Aggregators disagree (one reports $3.1M total, another reports a $100K seed). The only first-party evidence found is the site banner "We are proud to be part of the Netlify Jamstack Innovation Fund" ([Wayback](http://web.archive.org/web/20230411000318/https://www.snaplet.dev/)). Crunchbase and PitchBook are paywalled.
- **Neither company published revenue, customer counts, or churn.** "We have not reached the necessary adoption levels" and "misleading signals about traction" are as specific as the record gets.
- **Snaplet's Discord and Neosync's Discord are the missing corpus.** Both companies routed support to Discord ("our legendarily responsive Discord support"), so the richest record of user friction is in servers that are private or gone. Everything in §3 is what leaked into public trackers.
- **Reddit was not consulted** (unreachable from this environment, per the research brief). Some user sentiment likely lives in r/PostgreSQL and r/devops and is not represented here.
- **The qa.tech post on storing Snaplet snapshots in GCS** was fetched but contains only a migration how-to, no first-hand commentary on the shutdown; it is not cited above as user testimony.
- **Two competitor blogs** ([seedfa.st](https://seedfa.st/blog/neosync-alternative), [xata.io](https://xata.io/blog/the-5-best-data-anonymization-tools-for-development-teams-in-2026)) were used only for claims independently corroborated against GitHub or the archived sites. xata's claim that Neosync lacked MongoDB support is **wrong** — [MongoDB shipped in June 2024](https://github.com/nucleuscloud/neosync/blob/main/docs/blog/2024-06-20-mongodb.md) — which is a useful reminder of how quickly secondary sources rot.
