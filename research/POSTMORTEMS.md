# Postmortems: Snaplet and Neosync

Two companies built almost exactly what lazysnap describes — point a tool at a production Postgres database, subset it, mask the personal data, load it somewhere safe. Both are gone. Snaplet shut down on 31 August 2024. Neosync's repository was archived on 30 August 2025 after the team was acquired by Grow Therapy.

This document reconstructs what happened from primary sources: archived homepages and pricing pages, shutdown posts, GitHub issues and commits, the npm registry, changelogs, Hacker News threads, and founder statements. Every claim links to its source. Where a source could not be reached or a claim could not be corroborated, it says **unverified**.

**Research date: 5 September 2026.** Every claim sourced to `snaplet.dev` or `neosync.dev` below is cited to the Wayback Machine, to GitHub, or to a surviving Netlify mirror, because the live web no longer holds it. DNS and HTTP state verified from this machine on the research date:

| Host | State |
|---|---|
| `www.snaplet.dev` | NXDOMAIN |
| `snaplet.dev` | MX records only, no A record — mail routes, the site does not |
| `docs.snaplet.dev` | NXDOMAIN |
| `www.neosync.dev`, `docs.neosync.dev`, `app.neosync.dev` | NXDOMAIN |
| `neosync.dev` | Resolves to Vercel (76.76.21.21), returns `DEPLOYMENT_NOT_FOUND` (HTTP 404) |
| `assets.nucleuscloud.com`, `docs.nucleuscloud.com` | NXDOMAIN |
| `snaplet-snapshot.netlify.app`, `snaplet-seed.netlify.app` | HTTP 200 — the docs survive only on Netlify |

That table is itself a finding. Neosync's archived README still links to `www.neosync.dev`, `docs.neosync.dev` and an `assets.nucleuscloud.com` banner image ([README](https://github.com/nucleuscloud/neosync/blob/main/README.md)) — every one of them dead. Snaplet's `docs` repo, reduced to a farewell stub, points readers at `docs.snaplet.dev/seed` and `docs.snaplet.dev/snapshot` ([README](https://github.com/snaplet/docs/blob/main/README.md)) — also dead, though a `_redirects` file in the same repo reveals the surviving Netlify mirrors ([_redirects](https://github.com/snaplet/docs/blob/main/_redirects)). **When these companies died, their documentation died with the domain.** Only what lived in a git repository or on someone else's free tier survived.

---

## 1. Timeline

### Snaplet

| Date | Event | Source |
|---|---|---|
| Jan 2021 | Public beta. Tagline: *"Get a safe, minimized version of your production database on your laptop in minutes"* | [Wayback, 18 Jan 2021](http://web.archive.org/web/20210118202109/https://snaplet.dev/) |
| Jun 2022 | *"Say goodbye to seed scripts"* — transform-and-copy positioning | [Wayback, 1 Jun 2022](http://web.archive.org/web/20220601160539/https://www.snaplet.dev/) |
| 2 Jun 2022 | Founder on Jamstack Radio: *"It's completely free… once we get to teams or bigger companies, we'll start thinking about pricing"* | [Heavybit ep. 102](https://www.heavybit.com/library/podcasts/jamstack-radio/ep-102-database-accessibility-with-peter-pistorius-of-snaplet) |
| Apr 2023 | Preview databases + Netlify integration on the homepage; Netlify Jamstack Innovation Fund banner | [Wayback, 11 Apr 2023](http://web.archive.org/web/20230411000318/https://www.snaplet.dev/) |
| Sep 2023 | First archived pricing page: free tier + **Pro $30/team/month** with usage-based overages | [Wayback, 30 Sep 2023](http://web.archive.org/web/20230930182525/https://www.snaplet.dev/pricing) |
| Feb 2024 | Docs drift breaks the official Supabase guide | [supabase/supabase#21089](https://github.com/supabase/supabase/issues/21089) |
| Jan 2024 | Homepage: *"Snaplet uses generative ai to give you realistic, production-like data"* | [Wayback, 27 Jan 2024](http://web.archive.org/web/20240127164802/https://www.snaplet.dev/) |
| 6 May 2024 | Snaplet Seed launches on Product Hunt, #2 of the day | [Show HN](https://news.ycombinator.com/item?id=40275677), [Wayback capture, 8 May 2024](http://web.archive.org/web/20240508200605/https://www.producthunt.com/posts/snaplet-seed) (stats block: "Featured on May 6th, 2024", "Upvotes 424", "Comments 59", "Day rank #2", "Week rank #4") |
| Jun 2024 | Homepage headline is now *"Instant seed data for your relational database. Ditch the seed script!"*; Snapshot demoted to a secondary tab | [Wayback, 24 Jun 2024](http://web.archive.org/web/20240624152127/https://www.snaplet.dev/) |
| 1 Jul 2024 | Shutdown announced | [Snaplet is shutting down](http://web.archive.org/web/20240716025550/https://www.snaplet.dev/post/snaplet-is-shutting-down) |
| 30 Jul / 2 Aug 2024 | Final stable npm publishes: `@snaplet/seed@0.98.0` (30 Jul), `@snaplet/snapshot@0.93.2` (2 Aug) | [npm registry](https://registry.npmjs.org/@snaplet/seed), [npm registry](https://registry.npmjs.org/@snaplet/snapshot) |
| 14 Aug 2024 | Copycat, Seed and Snapshot open-sourced under MIT; Supabase says it will "pick up the ongoing maintenance" | [Supabase blog](https://supabase.com/blog/snaplet-is-now-open-source) |
| 31 Aug 2024 | Service off | [shutdown post](http://web.archive.org/web/20240716025550/https://www.snaplet.dev/post/snaplet-is-shutting-down) |
| 14 Aug 2024 | Last commit on `supabase-community/seed`'s `main` — the same day it was donated | [commits](https://github.com/supabase-community/seed/commits/main) |
| 14 Jan 2025 | Last Copycat release, v6.0.0 | [npm](https://registry.npmjs.org/@snaplet/copycat), [commits](https://github.com/supabase-community/copycat/commits/main) |
| 11 May 2026 | `supabase-community/snapshot` archived — six months after its last push (5 Nov 2025), which means PR #22 (below) sat open and mergeable the whole time | [GitHub GraphQL `archivedAt`](https://github.com/supabase-community/snapshot) (`gh api graphql -f query='{repository(owner:"supabase-community",name:"snapshot"){archivedAt}}'` → `2026-05-11T09:29:03Z`) |
| 6 Jan 2026 | Founder corrects the public record on HN: *"it was not acquired by Supabase; I shutdown the startup"* | [HN](https://news.ycombinator.com/item?id=46518012) |

### Neosync

| Date | Event | Source |
|---|---|---|
| 5 Jun 2023 | Same founders launch **Nucleus** (YC S22), "a Kubernetes platform for both devs and ops", priced "around $35k/license" | [Launch HN, 77 pts](https://news.ycombinator.com/item?id=36197880) |
| 7 Dec 2023 | Neosync launches as "open source data replication and anonymization", 40+ transformers | [Show HN](https://news.ycombinator.com/item?id=38560030), [changelog](https://github.com/nucleuscloud/neosync/blob/main/docs/blog/2023-12-04-neosync-init.md) |
| 20 Dec 2023 | "Introducing Neosync" explains the pivot away from cloud infrastructure | [Wayback](http://web.archive.org/web/20250803043832/https://www.neosync.dev/blog/introducing-neosync) |
| 19 Jan 2024 | "Improve Temporal Self-Hosted Documentation" filed, one month after launch | [#1141](https://github.com/nucleuscloud/neosync/issues/1141) |
| 18 May 2024 | First archived pricing: free 100k records; **Team $299/mo** | [Wayback](http://web.archive.org/web/20240518130236/https://www.neosync.dev/pricing) |
| 22 May 2024 | Show HN — 246 points, 44 comments, the high-water mark of public attention | [HN](https://news.ycombinator.com/item?id=40443927) |
| 7 Jul 2024 | "Exploring How Neosync Leverages Temporal" | [Wayback](http://web.archive.org/web/20250803053743/https://www.neosync.dev/blog/leveraging-temporal) |
| 17 Sep 2024 | Second Show HN: DynamoDB, MongoDB, SQL Server, and **Neosync Cloud** | [HN](https://news.ycombinator.com/item?id=41569240) |
| 6 Feb 2025 | "Mark a table as root yourself for subsetting" filed — never shipped | [#3227](https://github.com/nucleuscloud/neosync/issues/3227) |
| 11 Jul 2025 | Final release `v0.5.41`; the same day, a commit titled "license" adds a `ValidLicense` type whose `IsValid()` returns `true` unconditionally | [releases](https://github.com/nucleuscloud/neosync/releases), [commit 4a2c971](https://github.com/nucleuscloud/neosync/commit/4a2c971c81b098ec8cb6661a1b7883f1f56b40f3) |
| 25 Aug 2025 | "What is the post-acquisition plan?" — maintainer: *"There are no current plans for any continued maintenance."* | [#3565](https://github.com/nucleuscloud/neosync/issues/3565) |
| 30 Aug 2025 | Final commit: "adds acquired disclaimer"; repo archived | [commit 8101c42](https://github.com/nucleuscloud/neosync/commit/8101c42dcdb0ac6f67558d4fabb41fefad97a101) |
| 22 Sep 2025 | Archival noticed on HN | [HN](https://news.ycombinator.com/item?id=45331127) |
| 25 Sep 2025 | Grow Therapy publicly announces the acquisition | [PR Newswire](https://www.prnewswire.com/news-releases/grow-therapy-raises-the-privacy-bar-in-mental-health-302567153.html) |
| 29 Sep 2025 | A downstream project begins unwinding its dependency | [xataio/pgstream#551](https://github.com/xataio/pgstream/issues/551) |

**A note on the brief's dates.** The task described the Neosync acquisition as August 2025. The repository work — licence unlock, "no plans for continued maintenance", the archival disclaimer — all happened in July and August 2025; the *press announcement* was 25 September 2025. Both are real and refer to different things, and the founders themselves have since settled it in first-party writing rather than leaving it to inference from repo activity: co-founder Nick Zelei's own site states "We were acquired by Grow Therapy in July of 2025" ([nickzelei.com](https://www.nickzelei.com)), and co-founder Evis Drenova's shutdown post opens "Last Thursday, on August 14th, we officially shut down the last remnants of Neosync after our acquisition" ([evis.dev, 17 Aug 2025](https://www.evis.dev/posts/is_the_market_stupid)) — so the deal itself closed by July 2025, the product was actually turned off on 14 August 2025, the repository was archived 30 August 2025, and the press release followed on 25 September 2025. Four dates, four different events, and none of them contradicts another. Y Combinator lists Neosync's status as **Acquired**, batch Summer 2022, team size 3 ([YC](https://www.ycombinator.com/companies/neosync)).

---

## 2. What users loved

### The promise, stated plainly

Snaplet's 2021 tagline is nearly word-for-word what lazysnap's CONCEPT.md promises. It landed:

> "Get a safe, minimized version of your production database on your laptop in minutes." I've needed this on every serious project I've worked on and it's always been hard.
> — Tom Preston-Werner ([@mojombo, 17 May 2021, as displayed on snaplet.dev](http://web.archive.org/web/20230411000318/https://www.snaplet.dev/))

> I hope we get the chance to write up the tools we use to build @replayio soon, but in the meantime I'll just say that @_snaplet enabled us to pretty quickly build an excellent dev environment with a tiny (but growing!) engineering team.
> — Dan Miller, Replay ([@jazzdan, 22 Nov 2021, ibid.](http://web.archive.org/web/20230411000318/https://www.snaplet.dev/))

> Just gave @_snaplet a whirl and outside of not understanding some syntax stuff...first experiment worked well for anonymizing data from a Heroku-hosted Rails app database
> — Robby Russell ([@robbyrussell, 3 Feb 2022, ibid.](http://web.archive.org/web/20230411000318/https://www.snaplet.dev/))

That parenthetical — *"outside of not understanding some syntax stuff"* — is the earliest recorded friction signal in the whole corpus, and it is about **configuration syntax**, not about the job.

On Product Hunt, three comments (Thibault Le Ouay of openstatus, Dominik Hackl of fynk, and a Stockle user named Joonatan) reportedly described the pain being removed rather than the feature being added, in the vein of "this saved me hours/days/weeks of crafting seed data" — but this is **unverified**: `producthunt.com/posts/snaplet-seed` is bot-walled from this environment (it serves a Cloudflare "Just a moment…" interstitial to curl, to a real browser session, and to this session's browser tool), and the two Wayback captures of the page ([8 May 2024](http://web.archive.org/web/20240508200605/https://www.producthunt.com/posts/snaplet-seed), [18 May 2024](http://web.archive.org/web/20240518133022/https://www.producthunt.com/posts/snaplet-seed)) do not contain the comment text — Product Hunt renders comments client-side, after the crawl. The stats block those captures do carry ("Upvotes 424", "Comments 59") corroborates that 59 comments existed; their content could not be independently confirmed and none of it is load-bearing here — the homepage testimonials below and the npm download data carry the same point with sources that were fully verified.

Neosync's own launch post names the same status quo, and it is worth quoting because it is the exact scene lazysnap's CONCEPT.md describes:

> The status quo is that a developer will either manually PGDUMP from their production database and PGRESTORE locally or run a script that does the same thing. It's what almost every developer we've talked to does. That obviously needs to change. Not only is it ridiculously insecure but it's also wildly inefficient.
> — [Introducing Neosync](http://web.archive.org/web/20250803043832/https://www.neosync.dev/blog/introducing-neosync), Dec 2023

### The proxy, and deterministic fake values

Seven weeks after the shutdown (31 August 2024 to 17 October 2024), an HN user explaining what he missed named two specific things — neither of which was the cloud product:

> I liked similar thing, snaplet, unfortunately they're dead now. One thing I liked was the option to run proxy to which you could connect with any tool you like (psql, dbeaver, ...) and see preview of your transformations. Also they had some good (stable) generators for names, emails, etc...
> — muhehe, on a Greenmask thread ([HN, 17 Oct 2024](https://news.ycombinator.com/item?id=41867092))

"Stable generators" is Copycat: *"like faker.js, but deterministic: for any given input it'll always produce the same output"* ([Supabase blog](https://supabase.com/blog/snaplet-is-now-open-source)).

**The adoption data says this is what people actually kept.** Weekly npm downloads for the week of 23–29 August 2026, two years after the company died:

| Package | Weekly downloads | What it is |
|---|---|---|
| [`@snaplet/copycat`](https://api.npmjs.org/downloads/range/2026-08-23:2026-08-29/@snaplet/copycat) | **more than 12×** `@snaplet/snapshot`'s downloads (121,478) | Deterministic fake values. No service. ~1,000 lines of pure function. |
| [`@snaplet/seed`](https://registry.npmjs.org/@snaplet/seed) | 41,159 | Schema-based generation (calls a hosted LLM) |
| [`@snaplet/snapshot`](https://api.npmjs.org/downloads/range/2026-08-23:2026-08-29/@snaplet/snapshot) | 9,808 | **The actual snapshot tool** |
| [`snaplet`](https://registry.npmjs.org/snaplet) | 3,511 | The original CLI |

(Verified via `https://api.npmjs.org/downloads/range/2026-08-23:2026-08-29/<pkg>`, the week of 23–29 August 2026, the research date; a `point/last-week` link would drift as the calendar moves and no longer back these numbers.)

The smallest, most boring, least differentiated component — a deterministic faker with nothing behind it — outlives the platform by **12×**. GitHub stars agree: [copycat 1,057](https://github.com/supabase-community/copycat), [seed 790](https://github.com/supabase-community/seed), [snapshot 328](https://github.com/supabase-community/snapshot). The thing the company was built to sell is bottom of both lists.

**That single week is not a fluke — it is a two-year trend, and copycat and snapshot are moving in opposite directions.** Monthly download totals from the npm registry's range API (`https://api.npmjs.org/downloads/range/2024-09-01:2026-08-31/<pkg>`):

| Month | `@snaplet/copycat` | `@snaplet/snapshot` |
|---|---|---|
| Apr 2025 | 132,926 | 15,550 |
| Jul 2025 | 153,909 | 16,724 |
| Oct 2025 | 177,103 | 23,280 |
| Jan 2026 | 181,845 | 29,638 |
| Apr 2026 | 246,708 | 24,235 |
| Jul 2026 | 414,268 | 28,748 |

Copycat more than tripled its monthly download volume over two years with zero commits since January 2025 — pure-function libraries with no runtime dependency keep getting pulled in by new projects indefinitely. Snapshot grew too, roughly in line with copycat's early trajectory, right up to its archival in May 2026 — which means "rots" understates it: snapshot was not obviously dying by download volume, it was simply cut off while still growing, the moment nobody was left to merge PR #22 or fix the CVEs in `seed`. The lesson is not "unmaintained tools decline" — it's that **an unmaintained tool with no native dependencies can keep growing indefinitely, while one with any (`better-sqlite3`, a hosted LLM call) eventually hits a wall it cannot climb without a maintainer**, whatever its download curve looked like on the way there.

### A genuinely good `setup` command

Snaplet's documented first run introspects the database and *proposes* a config rather than demanding one:

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

The same page states, in a tip: *"A user account is **not required**, you only need a user account if you want to share snapshots with your team."* That is exactly lazysnap's zero-config principle, and they had it. §5 covers what they did with it.

And the final self-hosting guide — the underlying tool with the company stripped away — is three commands and no account:

```
SNAPLET_SOURCE_DATABASE_URL='postgres://…/production' npx snaplet config generate --type typedefs --type transform
SNAPLET_SOURCE_DATABASE_URL='postgres://…/production' npx snaplet snapshot capture /tmp/my-snapshot
SNAPLET_TARGET_DATABASE_URL='postgres://…/development' npx snaplet snapshot restore /tmp/my-snapshot
```
([final self-hosting docs](https://snaplet-snapshot.netlify.app/snapshot/guides/self-hosting))

**That is the good product, and it was never the one on the homepage.**

### Referential integrity, and being open source

The Neosync feature users praised most, unprompted, by name, was the one nobody else got right. From a developer fighting the same problem with ClickHouse's obfuscator:

> The problem I'm running into is referential integrity, as importing the anonymized data is raising unique and foreign key violations. The obfuscator tool is pretty minimal and has few knobs to tweak its output… Your tool looks interesting, and it seems that you directly address the referential integrity issue, which is great.
> — imiric ([HN, 22 May 2024](https://news.ycombinator.com/item?id=40446843))

> Great to see such a project. We are using datanymizer right now but it has gone unmaintained and we are using my patched version and it is working pretty well for us.
> — gregwebs ([HN, 23 May 2024](https://news.ycombinator.com/item?id=40449655))

> Congrats on the release! I should be able to switch from datanymizer (unmaintained) now.
> — gregwebs, five months later on a Greenmask thread ([HN, 17 Oct 2024](https://news.ycombinator.com/item?id=41866607))

That last one is the whole market in two lines: users of this category are perpetually migrating off a dead tool onto the next one that will die.

The founder identified referential integrity as the hard part and the reason people picked them:

> yeah the referential integrity and constraints part is usually the most complicated part and everyone does things differently which adds another layer of complexity on it
> — edrenova ([HN, 22 May 2024](https://news.ycombinator.com/item?id=40446813))

And an intern who had built a similar toolchain confirmed where the cost lives:

> Referential constraint refer to ensuring some coherence / basic logic in the output data (ie. the anonymized street name must exist in the anonymized city). **This was the most time consuming phase of the pseudonymization process.** … Also, a lot of the time client had no proper idea of what the field were and what they were truly containing (what format of phone number, we did find a lot of unusual things).
> — mathisd ([HN, 22 May 2024](https://news.ycombinator.com/item?id=40446590))

The open-source-first stance was the actual go-to-market, and the founders said so:

> Open source is a great way to get adoption from mid-size and enterprise sized companies who have long procurement cycles and stringent data privacy and security programs. A developer can fork your repo and run it locally in an hour if your project is open source versus if it's not, well, then you're in procurement hell for the next 6-9 months.
> — [Introducing Neosync](http://web.archive.org/web/20250803043832/https://www.neosync.dev/blog/introducing-neosync)

Neosync reached [4,141 GitHub stars](https://github.com/nucleuscloud/neosync) — about 12× Snapshot's 328 — which is a decent proxy for how much developers wanted this to exist.

---

## 3. What users abandoned them over

### The cloud was the product, and users said out loud they would not use it

The single most direct piece of evidence in the corpus. On the Neosync Show HN, a prospective user works through the value proposition and rejects the business model in the same comment:

> As a side note, I'm not sure I understand what the value proposition of your Cloud service would be. If the original data needs to be exported and sent to your Cloud for anonymization, it defeats the entire purpose of this process, and only adds more risk. I don't think that most companies looking for a solution like this would choose to rely on an external service. Thanks for releasing it as open source, but **I can't say that I trust your business model to sustain a company around this product.**
> — imiric ([HN, 22 May 2024](https://news.ycombinator.com/item?id=40446843)) (emphasis added)

The founder's reply concedes the segmentation problem exactly:

> Folks use the cloud service because they don't have the resource or time to deploy/run the OSS offering themselves. These are usually startups who are okay with us streaming their data and anonymizing it and sending it back to them.
> — edrenova ([HN](https://news.ycombinator.com/item?id=40447065))

So: the people willing to pay for hosting are the ones *least* worried about data leaving their perimeter, and the people most worried — the regulated enterprises the product is aimed at, the ones with the budget — self-host for free. Fifteen months later the repo was archived. imiric's closing line stands as the epitaph:

> Sounds good about your service then. I can't say that I would personally want to rely on it, but I can see how it would be useful for others.
> — imiric ([HN](https://news.ycombinator.com/item?id=40447728))

### A loud contingent rejects the *category*, not the product

The top-voted critical comment on the Neosync launch is a flat refusal of anonymised production data as a practice:

> I don't know exactly how this works, but I wanted to share my experience trying to anonymize data. **Don't.** While you may be able to change or delete obvious PII, like names, every bit of real data in aggregate leads to revealing someone's identity… Just stick with fuzzing random data during development… **Keep production data in production.**
> — blopker ([HN, 22 May 2024](https://news.ycombinator.com/item?id=40445047))

The follow-up is the more dangerous objection commercially, because it is operational rather than philosophical:

> Once production data is floating around different environments, it will be easy to lose track of. Then the first GDPR delete request comes in. Was this data synthetic? Was it real? I think Joe has a copy on his laptop, he's on vacation?
> — blopker ([HN](https://news.ycombinator.com/item?id=40445417))

And from someone with medical-records experience:

> Mid 2000s, I worked with electronic medical records. I eventually determined anon isn't worthwhile. For starters, deanon will always beat anon. This statement is unambiguously true, per the research. Including the differential privacy stuff.
> — specialist ([HN, 30 May 2024](https://news.ycombinator.com/item?id=40526241))

The thread argues both ways at length; [the strongest rebuttal](https://news.ycombinator.com/item?id=40446466) is that random data cannot reproduce production's scale or distributions. The point for lazysnap is not who is right. It is that **a meaningful slice of the exact target audience believes the category is a bad idea**, and a tool in this space has to answer that objection in its defaults — by keeping the copy small, local and disposable, by never letting an untransformed column through silently, and by making the run reproducible so the copy can be regenerated rather than hoarded.

### Subsetting hit a wall users could not get past

The clearest abandonment story in the Neosync tracker. A user wants to subset from a "starting point" table that is both a parent and a child:

> My 'Starting point' table has foreign keys to other tables as well as other tables have foreign keys to it. I want to apply a subset filter to this table, and all the tables related to it should apply that subset filter to it. But it looks like my subset filter is not propagated to the tables that have a foreign key to it
> — Yarn-e ([#3227, 7 Feb 2025](https://github.com/nucleuscloud/neosync/issues/3227))

The maintainer's answer is an architectural confession:

> Yeah it's a current design choice and system limitation. Foreign Key constraints are effectively directional graphs (DAGs) that follow a parent->child hierarchy. We've had a few different users (as well as a lot of internal discussions) ask about changing our subsetting logic to treat any table that has a filter applied to it as the _root_ node and ignoring the directionality of the foreign key constraints and instead treat them as graph edges. **This is more complex in practice** because today when a sync happens, we build the run order based on the foreign key constraints… **Long story short, it's definitely possible to update Neosync to subset in this way, we just haven't tackled it yet as we started with the simpler approach.**
> — nickzelei ([#3227, 26 Feb 2025](https://github.com/nucleuscloud/neosync/issues/3227))

The user's final reply:

> Sadly it isn't possible in our case… Thanks for giving this explanation! I'll keep an eye out on the roadmap in case it would come up there!
> — Yarn-e ([#3227, 27 Feb 2025](https://github.com/nucleuscloud/neosync/issues/3227))

The feature never shipped. The issue is still open on an archived repo. **This is precisely lazysnap's core mechanic** — "a root table, a row count, parents to completeness, children with caps". Neosync could not do the parent-by-child direction, said so in public, and lost the user.

Snaplet had the same shape of limitation in a different place: `percent` and `rowLimit` did not mean what they said.

> Note that the `percent` / `rowLimit` specified in the subset config may not be exact [precent, sic]. The actual row count of the data is affected by the relationships between the tables. **As such, a 5% subset specified against a specific table may ultimately include more than 5% of the actual database.**
> — [Snaplet subset docs](https://github.com/snaplet/docs-old/blob/main/docs/04-references/data-operations/04-reduce.md)

### The docs and the CLI drifted apart, repeatedly

Snaplet renamed a command's meaning mid-flight and broke the official Supabase guide:

```
npx snaplet generate --sql
  📢 Change notice:
  Were you expecting this command to generate data? It now does something different.
```
— reported in [supabase/supabase#21089, "Snaplet docs are out of date"](https://github.com/supabase/supabase/issues/21089), 7 Feb 2024

Config-file location drift cost a user a working install; the fix was to hand-edit a JSON file the tool was supposed to write:

> Ok, the `.snaplet/config.json` was created but only with the `projectId` key. ✅ Thank you, I manually added the `adapter` key and it fixed it
> — Aymericr ([snaplet/docs#118](https://github.com/snaplet/docs/issues/118), 31 May 2024)

Five months after the shutdown the docs were still wrong and there was no longer anyone to fix them: [supabase-community/snapshot#18, "Outdated documentation on configuring select and transform"](https://github.com/supabase-community/snapshot/issues/18) was filed on 17 January 2025 and is still open with zero comments.

### Node/npm packaging rot killed installs

`@snaplet/snapshot` shipped a native `better-sqlite3` dependency pinned to a version that did not support Node 22 — so the tool broke on an LTS release without anyone touching it:

> `@snaplet/snapshot` depends on `better-sqlite3` version 8.5.0, but that library only added Node 22 support in version 10, so you will probably need to override that somehow if you want to use Node 22.
> — smcgivern ([supabase-community/snapshot#15, "npm install failure"](https://github.com/supabase-community/snapshot/issues/15), 20 Nov 2024)

Issue #15 was opened 18 October 2024 and is **still open** — the most-reacted issue in that repository. Related reports: [#20, ARM Darwin build failure](https://github.com/supabase-community/snapshot/issues/20) (Apr 2025), [#21, `bun add @snaplet/snapshot` fails](https://github.com/supabase-community/snapshot/issues/21) (Oct 2025).

Thirteen months after the report, a stranger sent the fix: [PR #22, "fix(cli): npm install is broken, need to upgrade better-sqlite3"](https://github.com/supabase-community/snapshot/pull/22), opened 18 November 2025 by faisalburhanudin. The repository was still open and writable at the time — it was not archived until 11 May 2026 ([`archivedAt`](https://github.com/supabase-community/snapshot)) — so the one-line fix sat mergeable for nearly six months before anyone with write access acted on it, and now that the repo is archived **nobody can merge it, ever**. That is not an unlucky race between a PR and an archival; it is six months of maintainer silence on a working fix, followed by a door closing on it permanently.

And the tool could fail on the capture itself against a perfectly ordinary hosted Postgres:

> When I want to capture my remote database with snapshot I have this error … `Unhandled error: could not open file "base/5/526868": No such file or directory` … We have been notified, but if you need help now please contact us on Discord
> — [supabase-community/snapshot#17](https://github.com/supabase-community/snapshot/issues/17), 10 Jan 2025, against a Supabase pooler connection string

Note the error text: the tool tells the user it has "notified" a company that no longer exists, and directs them to a Discord that was the company's support channel. **A dead tool's error messages keep making promises the dead company can no longer keep.**

Two years on, the package is a supply-chain liability:

> `@snaplet/seed` version 0.98.0 has transitive dependencies with known high-severity security vulnerabilities. Running `npm audit` reports vulnerabilities in `@langchain/core`, `tar`, and `tmp`…
> — jmgunter ([supabase-community/seed#213](https://github.com/supabase-community/seed/issues/213), 22 Jan 2026)

41,159 weekly downloads, a package whose `main` branch has not moved since 14 August 2024, carrying known high-severity vulnerabilities.

### The real cost of abandonment, in one team's words

A UK food bank charity running Snaplet Seed:

> Snaplet has shut down. They've open sourced their "seed" code, which I believe is all we use, but double-check. … **We can't generate new seed data until this is resolved**, so if there are schema or other changes that should be reflected in the seed data, put them in comments on this ticket.
> — stonelink, [LambethFoodbank/foodbankapp#688](https://github.com/LambethFoodbank/foodbankapp/issues/688)

The ticket then accumulates eight schema changes the seed data can no longer reflect, and ends:

> The build is still failing on seed.ts because of type changes - these need fixing before a fresh checkout builds. **Currently I rename seed.ts to be seed.ts.txt so that the build suceeds [sic].**
> — stonelink, quoting internal notes of 29 March 2025, [#688](https://github.com/LambethFoodbank/foodbankapp/issues/688)

Roughly nine months of a broken seed pipeline at a volunteer-run charity, worked around by renaming the file so the build would stop failing. The ticket itself was not closed until [26 October 2025](https://github.com/LambethFoodbank/foodbankapp/issues/688) — about fifteen months after the shutdown announcement — which is the more honest measure of how long a dead dependency drags on a small team than the rename alone suggests.

Downstream libraries unwound their dependencies after Neosync's archival:

> Given the recent archival of the neosync project, we will want to stop supporting the library eventually. In order to do that, we should have equivalent transformers to the neosync ones pgstream currently supports, so that anyone using them can smoothly transition.
> — eminano, [xataio/pgstream#551](https://github.com/xataio/pgstream/issues/551), 29 Sep 2025

### A runtime dependency on a hosted model became the top complaint

After Snaplet Seed went to the community, the highest-signal open requests are all about escaping the hosted LLM it calls:

- [seed#198, "Ollama support"](https://github.com/supabase-community/seed/issues/198) — opened 11 Sep 2024, tied for the most-reacted open issue in the repo (5 reactions), three bare "+1" comments, and then:
  > How is this still open?!
  > — simmorsal ([seed#198](https://github.com/supabase-community/seed/issues/198), 18 Jan 2025)
- [seed#204, "Allow custom OpenAI API-compatible server endpoint"](https://github.com/supabase-community/seed/issues/204) — 28 Nov 2024, zero comments
- [seed#210, "AI Model rate limiting"](https://github.com/supabase-community/seed/issues/210) — 11 May 2025
- [seed#211, "Add a Configuration Option to Disable AI Banner"](https://github.com/supabase-community/seed/issues/211) — 15 Aug 2025:
  > The banner is very annoying as it pops up every time. As far as I can tell, there's currently no way to disable it. Please consider adding a config option to opt out. Or better yet, make it opt-in.
  > — philipbel

A local dev tool that phones a third-party model on every run is a tool that stops working, or starts nagging, when the company does.

---

## 4. What the product became that it did not start as

### Snaplet: snapshot tool → cloud data platform → AI seed-script generator

| Era | Headline | Source |
|---|---|---|
| Jan 2021 | "Work with your database as easily as your code — **Get a safe, minimized version of your production database on your laptop in minutes**" | [Wayback](http://web.archive.org/web/20210118202109/https://snaplet.dev/) |
| Jun 2022 | "**Say goodbye to seed scripts.** Snaplet copies your Postgres database, transforming personal information" | [Wayback](http://web.archive.org/web/20220601160539/https://www.snaplet.dev/) |
| Apr 2023 | "Quit writing seed scripts, start shipping features. Get production-accurate data **and preview databases** to code against" | [Wayback](http://web.archive.org/web/20230411000318/https://www.snaplet.dev/) |
| Jan 2024 | "Build faster, test better… **Snaplet uses generative ai** to give you realistic, production-like data" | [Wayback](http://web.archive.org/web/20240127164802/https://www.snaplet.dev/) |
| Jun 2024 | "**Instant seed data** for your relational database. Ditch the seed script! … AI-generated mock data for your local database" | [Wayback](http://web.archive.org/web/20240624152127/https://www.snaplet.dev/) |

Three and a half years of drift from *copy production safely* to *generate fake data from a schema with an LLM*. Note the destination — **synthetic data generation from a schema alone is the first item on lazysnap's own non-goals list.**

The homepage decision tree records the moment the audience changed. In January 2024 the question was *"Do I have access to production-like data?"* ([Wayback](http://web.archive.org/web/20240127164802/https://www.snaplet.dev/)). By June 2024 the same widget asked *"Am I authorized to use production credentials?"* ([Wayback](http://web.archive.org/web/20240624152127/https://www.snaplet.dev/)) — and the "NO" branch, which routes to Seed, is labelled *"↑ Easiest way to try out Snaplet!"*. The company had concluded most of its visitors could not use its original product at all.

Along the way it also acquired a hosted serverless database ("preview databases", backed by Neon), a VS Code extension, a Netlify plugin, a Vercel action, a GitHub Action, and a local proxy. By the final quickstart, the recommended path was:

1. *"Sign in to Snaplet Cloud"* — "First off, you'll need a free Snaplet account."
2. Run a web onboarding wizard to connect the source database and capture.
3. Install the **VS Code extension** — *"the fastest and easiest way to use Snaplet"*; it *"automatically restores your latest snapshot tagged `main` into a cloud preview database"*.
4. *"Change your local development database environment variable to point to `postgresql://snaplet@localhost:2345/snaplet`"* — a proxy fronting the cloud database.

([quickstart](https://github.com/snaplet/docs-old/blob/main/docs/02-quickstart.md))

**That workflow is the inversion of the founding promise.** The 2021 pitch was "on your laptop in minutes". The final default was "connect your production database to our cloud, install our editor extension, and point your app at our proxy". The clean three-command self-hosted path still existed ([self-hosting docs](https://snaplet-snapshot.netlify.app/snapshot/guides/self-hosting)) — it was just not what anyone was shown.

### Neosync: Kubernetes PaaS → data sync tool → "Data Security Platform"

Neosync is a pivot from a *different* failed product, and the founders say so plainly:

> When Nick and I started Nucleus Cloud Corp, our vision was to help developers build faster, more secure and resilient applications… We spent a year on the Nucleus Cloud Platform and were proud of the work that we had done. But at the end of the day, it was clear to us that cloud infrastructure is a tough game for startups. So we made the decision to work on other ideas.
> — [Introducing Neosync](http://web.archive.org/web/20250803043832/https://www.neosync.dev/blog/introducing-neosync), Dec 2023

Nucleus was "a Kubernetes platform for both devs and ops" sold at "around $35k/license, or about 10% of what it would cost you to build and maintain this yourself" ([Launch HN, Jun 2023](https://news.ycombinator.com/item?id=36197880)). Six months later the same two people, in the same GitHub org (`nucleuscloud`), with the same Helm-and-Kubernetes instincts, shipped a database anonymisation tool. **The architectural weight of the first product was inherited wholesale by the second** — see §5.

Then the scope widened. In December 2023 the description was "open source data replication and anonymization". The final repository description reads:

> **Open Source Data Security Platform** for Developers to Monitor and Detect PII, Anonymize Production Data and Sync it across environments.
> — [github.com/nucleuscloud/neosync](https://github.com/nucleuscloud/neosync)

"Monitor and Detect PII" is a security-posture product sold to a security team, not a developer tool. The seed of the drift is in the launch post — a second audience, from month one:

> For ML engineers, they need access to high quality synthetic data to train and fine tune models… Imagine being able to generate a million rows of synthetic data that you can use to fine-tune your model with just a prompt.
> — [Introducing Neosync](http://web.archive.org/web/20250803043832/https://www.neosync.dev/blog/introducing-neosync)

Two audiences (app developers, ML engineers) and two jobs (anonymise real data, generate fake data) before the first release. The "Why now?" section of that same post rests the timing on AI: *"It's clear that AI/ML is a major platform change and the best businesses are built ontop of platform changes."*

**Both companies ended up chasing synthetic generation from a schema, from opposite directions.** Snaplet arrived there by drift; Neosync started there by design. It is the single most attractive adjacent problem in this space, and it is a different product with a different buyer. lazysnap's non-goal list already names it first. That non-goal is the most load-bearing sentence in CONCEPT.md.

---

## 5. Where complexity accumulated

### Snaplet: in the subset config

The final documented subsetting surface, for one operation:

`enabled`, `targets[]` (each with `table` plus one or more of `percent`, `rowLimit`, `where`, and optionally `orderBy`), `keepDisconnectedTables`, `followNullableRelations`, `maxCyclesLoop`, `maxChildrenPerNode`, `taskSortAlgorithm`, `eager`, `traversalMode`
([final capture docs](https://snaplet-snapshot.netlify.app/snapshot/core-concepts/capture); earlier form in [reduce docs](https://github.com/snaplet/docs-old/blob/main/docs/04-references/data-operations/04-reduce.md))

Nine top-level keys, four more nested inside each `targets[]` entry, several of which require the user to understand graph traversal to set. And the documented way to use one of them:

> When setting up Snaplet for the first time, we recommend setting this parameter to 0 and to gradually increment it until the subset of data you fetch trough a relationship is enough for your use case.
> — [Snaplet subset docs, on `maxCyclesLoop`](https://github.com/snaplet/docs-old/blob/main/docs/04-references/data-operations/04-reduce.md)

That is a documented instruction to binary-search a tuning parameter by trial and error against a production database. It is also an admission that the tool cannot tell the user what a good value is, so the user must guess. Compare lazysnap's rule: *"Documentation is never the fix for a confusing first run. Change the default or the question."*

`traversalMode` is the purest example of complexity leaking into the interface. Its documentation:

> **together**: The traversal algorithm considers all targets collectively when determining the next steps. Traversal state, including counters used for checking thresholds (e.g. `maxCyclesLoop` or `maxChildrenPerNode`), is shared across all targets.
> **sequential**: Targets are traversed one after another… traversal state, including counters for threshold checks, is reset between targets, ensuring that these thresholds are applied on a per-target basis rather than collectively across all targets.
> — [capture docs](https://snaplet-snapshot.netlify.app/snapshot/core-concepts/capture)

No user can choose between those two options without a model of the traversal algorithm in their head. **This is the internal state machine, exposed as configuration.**

### Snaplet: in the default that was not safe

Three transform modes, and the default is the unsafe one:

> - **`unsafe`** (default): The data for columns not specified in the config is simply copied over as is without transformation.
> - **`strict`**: Fail the capture if any columns, tables or schemas have not been specified in the config.
> - **`auto`**: Automatically transform the data for any columns, tables or schemas that have not been specified in the config.
> — [transform docs](https://github.com/snaplet/docs-old/blob/main/docs/04-references/data-operations/02-transform.md)

A tool whose reason to exist is not putting customer emails on developer laptops shipped, as its default, "copy everything you did not explicitly name". Worse, in their own documented example transcript, the PII detector fails:

```
⠋ Transform: Detecting PII fields...Smart shape prediciton failed
```
— literal text from the [transform docs setup transcript](https://github.com/snaplet/docs-old/blob/main/docs/04-references/data-operations/02-transform.md) (typo theirs)

**A failing detector plus a pass-through default equals production data on a laptop, silently.** This is the exact failure lazysnap's threat model forbids.

The safe mode had its own honestly-documented limits, which are worth reading because lazysnap will hit all of them:

> Auto-transform mode is able to transform your database data into data that is still valid according to what your database is expecting, but it may not always be what your application's logic is expecting.
> … there are types that we're still working on adding support for. These include: Composite types, Geometric types, Network address types, Range types, User-defined types, Bit-string types.
> … We're still working on optimising auto-transform to be fast enough to be useful on large datasets… For this same reason, we also currently truncate text values at 1000 characters before transforming them.
> — [capture docs](https://snaplet-snapshot.netlify.app/snapshot/core-concepts/capture)

Three distinct hard problems in one section: application-level validity, exotic Postgres types, and performance on large values. Silently truncating text at 1,000 characters is a correctness bug dressed as a performance setting.

### Snaplet: in a breaking config migration

Version 0.50.0 restructured configuration and 0.60.0 removed the old form:

> Previously, you would have files such as `.snaplet/transform.ts`, `.snaplet/schema.json`, `.snaplet/structure.d.ts`. Now, you will have only one file `snaplet.config.ts`… **The "old" configuration will be removed and no longer working in version `0.60.0` of Snaplet. We recommend you to migrate your configuration as soon as possible.**
> — [migration guide](https://github.com/snaplet/docs-old/blob/main/docs/05-guides/migration-new-config.md)

The migration removed a capability people were using — dynamically computed transform configs — as the price of type-safety:

> This won't work anymore. You will need to define your configuration in a static way. This means that you can't use variables or functions to define your configuration. This is because in order to provide type-safety, we need to know the configuration at compile time. We think it's a good trade-off to have a better developer experience.
> — [migration guide](https://github.com/snaplet/docs-old/blob/main/docs/05-guides/migration-new-config.md)

And the migration tool lived in the cloud dashboard, with a disclaimer:

> For our cloud users, we created a migration tool that will attempt to automatically migrate your configuration to the new format. It's available in your project under the `Data Editor` tab. **However, it might have some issues depending on the complexity of your current configuration.**
> — [migration guide](https://github.com/snaplet/docs-old/blob/main/docs/05-guides/migration-new-config.md)

Self-hosters got no tool. The escape hatch from a breaking change in the CLI was in the SaaS.

### Neosync: in the deployment footprint

Self-hosting Neosync means running, at minimum, everything in its own `compose.yml`: `app` (Next.js frontend), `api`, `worker`, `db` (postgres:15, for Neosync's own config), `redis` (redis:7.2.4), plus `api-seed` and `temporal-seed` init containers using `temporalio/admin-tools` — and **Temporal itself**, which the compose file seeds but the Kubernetes guide explicitly declines to cover ([compose.yml](https://github.com/nucleuscloud/neosync/blob/main/compose.yml)):

> Deploying a Postgres database and Temporal instances are not covered under this guide. See the external dependency section below for more information regarding these resources.
> … We do not cover how to deploy a Postgres database for Neosync or the Temporal suite. … Temporal can also be deployed to Kubernetes via a Helm chart that can be found in their Github repo. … There is also a community maintained Temporal Operator that could be of use… **Use at your own risk.**
> — [Kubernetes deploy docs](https://github.com/nucleuscloud/neosync/blob/main/docs/docs/deploy/kubernetes.md)

Four separate Helm charts were published (api, app, worker, and an umbrella chart) — plus optional Istio and Datadog hooks, in the deployment documentation of a tool whose job is to copy a database. "Improve Temporal Self-Hosted Documentation" was filed as [#1141](https://github.com/nucleuscloud/neosync/issues/1141) in January 2024, **one month after launch**.

The maintainer was candid about the size of the bet:

> Being a startup, going with Temporal is a big choice, but is a technology that can be used small or big. It's a technology that we will likely never grow out of and will continue to grow with Neosync.
> — Nick Zelei, [Exploring How Neosync Leverages Temporal](http://web.archive.org/web/20250803053743/https://www.neosync.dev/blog/leveraging-temporal), Jul 2024

The same post shows the alternative that was rejected: the original proof of concept was a Kubernetes Operator with a custom `NeosyncJob` CRD, dropped because *"you had to be using Kubernetes if you wanted to install Neosync and run it yourself."* They correctly identified that a Kubernetes-only install was too heavy — and replaced it with a seven-container Docker Compose plus Temporal.

Users paid for it. A developer trying to sync from a local Postgres container:

> I was surprised to get snagged here given I'd expect a lot of users might have a DB running on their machine (docker or otherwise), but maybe that's not a popular use case.
> — patricktyndall ([#3420, "Can't connect to localhost database on Mac"](https://github.com/nucleuscloud/neosync/issues/3420), opened 27 Mar 2025, comment dated 28 Mar 2025)

The cause was Neosync's own Docker network isolating it from the user's database — a self-inflicted wound from shipping a multi-container platform instead of a binary. Another user could not get the platform to start at all: [#3560, "Unable to connect Neosync to Temporal – context deadline exceeded on DescribeNamespace"](https://github.com/nucleuscloud/neosync/issues/3560), July 2025 — filed by the same person who a month later asked what the post-acquisition plan was.

**And the CLI was never an escape hatch, because the CLI was a client of the control plane.** `neosync sync` takes `--api-key` (or `$NEOSYNC_API_KEY`) and `--connection-id`, and is documented as syncing "data from a neosync connection to a local destination" ([CLI sync docs](https://github.com/nucleuscloud/neosync/blob/main/docs/docs/cli/sync.md)). There was no path where a developer types one command against two connection strings and gets data. That path did not exist at any point in the product's life.

### Neosync: in the changelog

The `docs/blog` directory is a fortnightly changelog and reads as a two-year record of surface-area growth ([full list](https://github.com/nucleuscloud/neosync/tree/main/docs/blog)):

> public APIs (Dec 2023) → custom code transformers → conditional transformers → circular dependencies → **Terraform provider** (Feb 2024) → foreign-key transformations → metrics → partial table syncs → Postgres permissions → **AI generate jobs** (May 2024) → flexible schema references → real-time validation → **MongoDB** (Jun 2024) → job cloning → JavaScript transformers → **DynamoDB** (Aug 2024) → **SQL Server** (Aug 2024) → default transformers → bulk transformers → **CLI sync** (Oct 2024) → anonymization API → column strategies → **EU infrastructure** (Nov 2024) → **job hooks** (Dec 2024) → **RBAC** (Dec 2024) → v5 → Python SDK → **webhooks** (Feb 2025) → connection security → **Slack integration** (Mar 2025) → **PII detection batch job** (Mar 2025)

Five database backends, two object stores, a Terraform provider, a Python SDK, an LLM integration, RBAC, SSO, audit logs, webhooks, job hooks, Slack — in the sixteen months the changelog covers (Dec 2023 → Mar 2025). The changelog then goes silent: the repository ran for another five months, to the Aug 2025 archival, with no further entries — either nothing shippable was left to announce, or the team's attention had already left the product. Two things are conspicuous:

1. **The enterprise checklist arrives before the developer experience is finished.** RBAC lands in December 2024. The parent-by-child subsetting problem is raised two months later and never addressed.
2. **Breadth beat depth.** MongoDB, DynamoDB and SQL Server all shipped in a ten-week window in mid-2024 ([Jun](https://github.com/nucleuscloud/neosync/blob/main/docs/blog/2024-06-20-mongodb.md), [Aug](https://github.com/nucleuscloud/neosync/blob/main/docs/blog/2024-08-01-dynamodb.md), [Aug](https://github.com/nucleuscloud/neosync/blob/main/docs/blog/2024-08-29-sql-server-support.md)), while [#3227](https://github.com/nucleuscloud/neosync/issues/3227) shows the *Postgres* subsetting graph was still directional-only in 2025.

The CLI — the thing a developer on a laptop actually touches — got its first substantial sync improvements ten months after launch, and even then, item 1 of 4 in that release ([CLI Sync](https://github.com/nucleuscloud/neosync/blob/main/docs/blog/2024-10-10-cli-sync.md)).

### Both: in transformer edge cases

Enumerating transformers is cheap; making each correct across five database dialects is not. Neosync's tracker in its final year is dominated by transformer bugs:

- [`transformEmail` crashes when `maxLength` is too low and `preserveDomain` is true](https://github.com/nucleuscloud/neosync/issues/3540) (open at archival)
- [fullname transformer wrong with `preserve_length = true`](https://github.com/nucleuscloud/neosync/issues/3536)
- [generating a UUID for a referenced primary key fails the whole job](https://github.com/nucleuscloud/neosync/issues/3433) (open at archival)
- [tables with one column break jobs](https://github.com/nucleuscloud/neosync/issues/3430)
- [`'null'` JSON converted to SQL NULL](https://github.com/nucleuscloud/neosync/issues/3304)

"40+ transformers" was the launch headline in December 2023 ([Introducing Neosync](http://web.archive.org/web/20250803043832/https://www.neosync.dev/blog/introducing-neosync)); the May 2025 pricing page advertises "45+ Pre-built Transformers" ([Wayback](http://web.archive.org/web/20250516010948/https://www.neosync.dev/pricing)). Five more in eighteen months, and a bug queue for the ones already shipped. **The transformer count is a marketing number and a maintenance liability at the same time.**

---

## 6. Pricing: what they tried, and how it changed

### Snaplet

| Date | Free | Paid | Model |
|---|---|---|---|
| Sep 2023 | 1 GB snapshot storage, 5 h snapshot compute, 2 GB transfer, 10 h preview-DB usage / month | **Pro $30/team/month** with 10 GB / 50 h / 20 GB / 100 h; opt-in usage-based overage billing with a self-serve hard spend cap | Metered infra resale, per team not per seat |
| Feb 2024 | unchanged | Pro $30/team/month; **automated overage billing replaced by "we'll reach out to chat to you about a custom pricing plan"** | Same headline, manual overage handling |
| Aug 2024 | unchanged | Pro $30/team/month for Snapshot; **Seed pricing "TBC"**, free in beta; "You'll never be billed pro-rata by Snaplet" | Two products, one unpriced |

Sources: [Sep 2023](http://web.archive.org/web/20230930182525/https://www.snaplet.dev/pricing), [Feb 2024](http://web.archive.org/web/20240228045459/https://www.snaplet.dev/pricing), [Aug 2024](http://web.archive.org/web/20240802130407/https://www.snaplet.dev/pricing). Two further captures exist at [Dec 2023](http://web.archive.org/web/20231209202539/https://www.snaplet.dev/pricing) and [May 2024](http://web.archive.org/web/20240519040256/https://www.snaplet.dev/pricing).

Four observations.

**$30 per team per month for an unlimited-seat product is not a business.** They knew why they picked it and said so:

> No, we charge per team, irrespective of how many people on the team use Snaplet. We think per seat costs disincentivize teams to use tools like Snaplet, and we want your entire team to benefit from being able to code against realistic data.
> — [pricing FAQ](http://web.archive.org/web/20230930182525/https://www.snaplet.dev/pricing)

That is a good product instinct and a fatal pricing one. A ten-person team pays $360/year against a cost base the same FAQ itemises: S3 storage, S3 in/out transfer, a **Fargate worker per capture**, and Neon compute for preview databases.

**The paid tier's value was storage and hosting, not the tool.** Every metered unit is infrastructure resale:

> Snapshots are stored on an Amazon S3 instance, and as such, there's a cost for the absolute size of all snapshots (snapshot storage), and the cost for transferring data in and out of that S3 instance… When you capture a snapshot, we start up a Fargate worker to connect to your database and turn it into a snapshot. The time taken by that Fargate worker is the snapshot compute time. … Similarly, we use Neon.tech for preview databases, which also incur compute time, storage, and data transfer fees.
> — [pricing FAQ](http://web.archive.org/web/20230930182525/https://www.snaplet.dev/pricing)

The capture, transform and subset logic — the part that took three years to build — was free and self-hostable. lazysnap's non-goals already exclude every one of those meters ("Scheduling, retention, or sharing snapshots between people"; "A web UI or a hosted service"), which is the right call for a tool and means **there is no version of Snaplet's revenue model available to us.** That is a decision to make with open eyes, not a gap to paper over.

**They walked back automated overages.** Sep 2023: *"you will either be billed pro rata on a per-usage basis"*, with a self-serve global spend cap. Feb 2024: *"If you're a Pro plan user and you exceed your allocations repeatedly, we'll reach out to chat to you about a custom pricing plan."* Aug 2024 adds: *"You'll never be billed pro-rata by Snaplet."* That is a company discovering that usage-based billing on a $30 plan costs more in support and anxiety than it collects.

**At the end, the priced product was not the one being sold.** The last archived pricing page leads with **Seed** — free, in beta, "Pro: TBC" — and the CTA in the header is "Try Seed". The only product with a price sits behind a secondary tab ([Aug 2024](http://web.archive.org/web/20240802130407/https://www.snaplet.dev/pricing)).

### Neosync

| Date | Free tier | Team | Model |
|---|---|---|---|
| May 2024 | 100k records/month, 1 user | **$299/mo**, 5M records ($60 per extra 1M), 5 users ($10/user after) | Flat + record overage + seats |
| Sep 2024 | 30k records/month | **"Contact Us"** — price removed | Price hidden |
| Oct 2024 | 20k records/mo on the plan card, 30k in the comparison table | **Pay-as-you-go**: $200/mo platform fee + $0.0005/record (100k–1M), $0.00025/record (1M–5M), custom above, with a calculator | Pure usage-based |
| Feb 2025 | **Free tier removed**, replaced by a 14-day trial | Pay-as-you-go, platform fee now **$100/mo** | Trial-gated, halved platform fee |
| May 2025 | 14-day trial | **$300/mo flat**, unlimited records, unlimited users (comparison table below still says "Pay-as-you-go") | Back to flat |

Sources: [May 2024](http://web.archive.org/web/20240518130236/https://www.neosync.dev/pricing), [Sep 2024](http://web.archive.org/web/20240905232639/https://www.neosync.dev/pricing), [Oct 2024](http://web.archive.org/web/20241007140104/https://www.neosync.dev/pricing), [Feb 2025](http://web.archive.org/web/20250206123739/https://www.neosync.dev/pricing), [May 2025](http://web.archive.org/web/20250516010948/https://www.neosync.dev/pricing).

Five distinct pricing pages in twelve months, and the journey is a **loop**: flat with overages → hidden → usage-based → usage-based with the free tier removed → flat again at almost exactly the original price. Along the way the free tier shrank 5× (100k → 20k records) and then vanished entirely.

Two of those pages ship internally inconsistent numbers. The Oct 2024 plan card says "20k records/mo" while its own comparison table says "30k/month". The May 2025 page headlines "$300/mo … Unlimited Records" while the table below still lists Team records as "Pay-as-you-go". **That is the fingerprint of a team changing pricing faster than they can update a page** — which is itself the fingerprint of a team that does not know what its product is worth.

The tagline moved with the model. May 2024: *"Simple, Transparent Pricing / Pricing shouldn't be complicated, so we made it easy."* By October 2024 the same page carried a pricing calculator and a four-band per-record tier table.

The enterprise tier absorbed everything a self-hoster might want — SSO, RBAC, webhooks, audit logs, job hooks, EU region, streaming mode, free-form PII detection ([May 2025](http://web.archive.org/web/20250516010948/https://www.neosync.dev/pricing)) — until 11 July 2025, when they [made the licence check return `true` unconditionally](https://github.com/nucleuscloud/neosync/commit/4a2c971c81b098ec8cb6661a1b7883f1f56b40f3):

```go
type ValidLicense struct{}
func (v *ValidLicense) IsValid() bool { return true }
func (v *ValidLicense) ExpiresAt() time.Time { return time.Now().UTC().Add(time.Hour * 24 * 365 * 10) }
```

Everything behind the paywall was given away, with a ten-year expiry, seven weeks before the repository was archived.

The GitHub star count on those same pricing pages traces the attention curve: 2.9k (Sep 2024) → 3.1k (Oct 2024) → 3.7k (Feb 2025) → 3.8k (May 2025) → [4,141 today](https://github.com/nucleuscloud/neosync). **Growth had flattened well before the end.**

---

## 7. What the founders said about why

### Snaplet — Peter Pistorius

The shutdown post, on the relevant point:

> Snaplet will be shutting down on 31 August 2024. Since starting Snaplet in 2021, our mission has been to empower developers with safe and easy access to production-realistic data. **While we've helped many developers since then, we have not reached the necessary adoption levels to continue Snaplet.**
> — [Snaplet is shutting down](http://web.archive.org/web/20240716025550/https://www.snaplet.dev/post/snaplet-is-shutting-down), 1 Jul 2024

Not "we ran out of money", not "a competitor beat us" — **adoption**. Three years, named developers publicly enthusiastic, a Product Hunt #2, and not enough people using it.

On why the code lives on:

> I built Snaplet because I believe developers write better software when they have access to production-like data. Although the company is closing, my belief remains strong, so we are open-sourcing the tools we've built.
> — Peter Pistorius, quoted in [Supabase's announcement](https://supabase.com/blog/snaplet-is-now-open-source)

Supabase's framing of the ending is four words long:

> Startups are hard. One of our favorite startups, Snaplet, is shutting down. Despite that, they built an amazing team (some who now work at Supabase) and some incredible products.
> — Paul Copplestone, [Supabase blog](https://supabase.com/blog/snaplet-is-now-open-source), 14 Aug 2024

Eighteen months later, Pistorius corrected the record on Hacker News:

> Hey! Snaplet founder here. Want to clarify that it was not acquired by Supabase; I shutdown the startup and found roles for some of the team at Supabase.
> — pistoriusp ([HN, 6 Jan 2026](https://news.ycombinator.com/item?id=46518012))

> Thanks, but I am not at Supabase! I ended up going back to building RedwoodJS and took over the project, and now have a consultancy.
> — pistoriusp ([HN, 7 Jan 2026](https://news.ycombinator.com/item?id=46523330))

He had to correct it because of the comment he was replying to, on a Show HN for a new tool in the same space:

> Reminds me a bit of Snaplet before it embarked on its incredible journey to get acquired by Supabase and shut down. I like the concept but the painpoint has never been around creating realistic looking emails and such like, but **creating data that is realistic in terms of the business domain and in terms of volume.**
> — ljm ([HN, 6 Jan 2026](https://news.ycombinator.com/item?id=46513101), on [Show HN: DDL to Data](https://news.ycombinator.com/item?id=46511578))

That is a customer telling you, after the fact, that **you solved the easy half**. His follow-up sharpens it:

> The realistic cardinality is actually a good start (the problem with things like using Faker for DB seeds being that everything is entirely too random). … Considering a major issue with testing is that you can't accurately benchmark changes or migrations based on a staging environment that is 1% the size of your prod one, that would be a huge win I think even if the data is, for the most part, nonsensical. **As long as referential integrity is intact the specifics matter less.**
> — ljm ([HN, 6 Jan 2026](https://news.ycombinator.com/item?id=46514333))

For lazysnap this is a direct instruction about where to spend effort: **referential integrity and real cardinality beat pretty fake values.** It is also a warning, because "1% the size of your prod one" is exactly what lazysnap produces — the answer has to be that we are producing a *correct* slice for development and debugging, and saying so, not implying we solve load testing.

Earlier, while Snaplet was still free, Pistorius described the business model as deferred:

> No, no. It's completely free. What we're trying to do is build the best product for individuals and once we get to teams or bigger companies, we'll start thinking about pricing.
> — Peter Pistorius, [Jamstack Radio ep. 102](https://www.heavybit.com/library/podcasts/jamstack-radio/ep-102-database-accessibility-with-peter-pistorius-of-snaplet), 2 Jun 2022

Snaplet raised outside money — the Netlify Jamstack Innovation Fund participation is attested by the company's own site banner ([Wayback, Apr 2023](http://web.archive.org/web/20230411000318/https://www.snaplet.dev/)). Total funding figures reported by aggregators disagree and are **unverified** here; Crunchbase and PitchBook are paywalled from this environment.

### Neosync — Evis Drenova and Nick Zelei

On the pivot into the space, from a failed Kubernetes PaaS:

> We spent a year on the Nucleus Cloud Platform and were proud of the work that we had done. But at the end of the day, it was clear to us that cloud infrastructure is a tough game for startups. So we made the decision to work on other ideas. When we started thinking of other ideas, the first place we went was to think about what our customers had asked for.
> — [Introducing Neosync](http://web.archive.org/web/20250803043832/https://www.neosync.dev/blog/introducing-neosync)

On the open-source bet, reflecting publicly in mid-2024 — the pros are adoption and enterprise trust; **the cons are a description of what killed the business, written a year in advance**:

> When we first started working on Neosync, Nick Zelei and I spent a lot of time talking about whether or not we should open source. We actually listed out the pros and cons. Here they are:
>
> **OSS Pros:** Enterprises don't want to put their sensitive data in someone else's infra · Makes enterprise adoption easier in the early days · Community contribution and engagement will help spread the word · Will help us focus on building a great product
>
> **OSS Cons:** Lack of focus on sales early on might be a false signal · **Lack of insights into who is actually using Neosync** · Overhead with community, devrel, etc. · Overhead with managing an OSS product and eventually a hosted product · Commercialization risk
> — Evis Drenova, ["Neosync: The pros and cons of open source"](https://www.linkedin.com/posts/evisdrenova_when-we-first-started-working-on-neosync-activity-7224483405667745792-7TRm), Aug 2024

"Lack of focus on sales early on might be a false signal" and "Lack of insights into who is actually using Neosync" are the two lines to remember. 4,141 stars, "hundreds of companies" self-hosting by their own account ([Show HN](https://news.ycombinator.com/item?id=41569240)), and no way to tell whether any of them would pay.

**The contribution data says the community pro never materialised either.** Neosync's commit distribution ([contributors](https://github.com/nucleuscloud/neosync/graphs/contributors)): nickzelei 870, dependabot 628, evisdrenova 571 — then a cliff to the largest outside contributor at 4 commits, and 25 more people at 1–3 commits each. Two humans and a bot wrote the product. Across two years the repository accumulated 470 issues, only 39 of them opened in 2025, and a large share of the open ones are internal Linear tickets (`[NEOS-####]`) rather than user reports. GitHub Discussions was never enabled; feature requests were routed to a third-party roadmap tool ([#2596](https://github.com/nucleuscloud/neosync/issues/2596)), and support to Discord. **Open source bought stars and enterprise credibility. It did not buy maintainers, and it did not buy signal.**

On the ending, in the issue tracker, a month before any public announcement:

> There are no current plans for any continued maintenance. We are still working with the acquirer on handing them the repository but ultimately it will be up to them as to what they decide to do with the repository. **If you're using Neosync I'd recommend forking it at the current time and making any changes you wish.** A few weeks ago we removed any license restrictions that were in place behind pro features so everything is currently available in the latest release.
> — nickzelei ([#3565](https://github.com/nucleuscloud/neosync/issues/3565), 25 Aug 2025)

The user's reply is the entire lifecycle of an open-source dependency in one line: *"Thanks @nickzelei. I have already done so :)"*

The final README disclaimer, added five days later:

> **⚠️ Disclaimer:** Neosync has been acquired by Grow Therapy. As a result, this repository is no longer actively maintained. Thank you to all of our OSS and Cloud supporters over the years.
> — [commit 8101c42](https://github.com/nucleuscloud/neosync/commit/8101c42dcdb0ac6f67558d4fabb41fefad97a101)

The same commit deletes the repo-stats GitHub Action. Compare the promise made at launch:

> And our promise is that we will always have an open source version of Neosync that you can use. **That will never change.**
> — [Introducing Neosync](http://web.archive.org/web/20250803043832/https://www.neosync.dev/blog/introducing-neosync), Dec 2023

The code is still there under its licence, so the letter of the promise survives; the maintained project it named does not. **"Open source forever" and "maintained" are different promises, and users hear the second when you make the first.**

**And then, a year after the acquisition, Drenova answered the brief's question directly, in his own voice, with no PR gloss.** "It's the market, stupid" ([evis.dev, 17 Aug 2025](https://www.evis.dev/posts/is_the_market_stupid)) opens "Last Thursday, on August 14th, we officially shut down the last remnants of Neosync after our acquisition" and states the cause without hedging:

> Nothing matters more than the market you pick... if there is no existing demand for your product or category of products then you will not be successful. … In reality there just wasn't enough demand.
> — Evis Drenova, [evis.dev, 17 Aug 2025](https://www.evis.dev/posts/is_the_market_stupid)

His diagnostic test for a market is a checklist worth reading as a warning label for lazysnap's own category:

> If none of them are building or have been successful in your market then it's probably a bad market.
> — Evis Drenova, [evis.dev, 17 Aug 2025](https://www.evis.dev/posts/is_the_market_stupid)

Not execution, not a competitor, not funding — **demand**. That is the same word Pistorius used for Snaplet ("we have not reached the necessary adoption levels"), from a founder with every incentive, a year after a face-saving acquisition, to blame something else. Both founders independently converged on the identical diagnosis for two companies that built, by any technical measure, a good product. For a document whose purpose is to tell lazysnap whether to build in this market, that convergence is the single most load-bearing finding in the corpus, and it sharpens §11's "three things we must copy" into a genuine tension worth naming rather than resolving away: copying Snaplet's and Neosync's best engineering decisions does not manufacture the demand two sets of founders independently concluded was never there. lazysnap's answer to that has to be its non-goals and its refusal to build a company on top of the tool — a free, local, dependency-light CLI does not need "the market" to be large, only for the pain (ad hoc `pg_dump`/`PGRESTORE`, or nothing) to be worse than the tool, which is a lower bar than the one either startup had to clear to survive as a business.

A companion source exists but was not fully mined: Drenova gave a longer, less shutdown-focused account of himself as a founder on the **Ben Wolfson Podcast #5, "Lessons from Silicon Valley: Inside the Mind of a YC Founder"** (released 7 Dec 2024, [Apple Podcasts](https://podcasts.apple.com/us/podcast/lessons-from-silicon-valley-inside-the-mind-of/id1783893837?i=1000679541457)) — described by its own summary as covering "the hardest parts about being a founder" and "startup decisions" — before the shutdown but after Neosync's growth had already flattened (§6). Its audio could not be transcribed from this environment; it is listed here as a located, unopened source rather than a nonexistent one (see §9).

The acquirer's framing makes clear the team and the technique were bought for internal use in a HIPAA-scoped healthcare product, not the product as a product:

> bringing on the Neosync team allows us to drive advancements that will reverberate across the mental health care sector.
> — Aaltan Ahmad, Head of Security, Grow Therapy ([PR Newswire, 25 Sep 2025](https://www.prnewswire.com/news-releases/grow-therapy-raises-the-privacy-bar-in-mental-health-302567153.html))

> Joining Grow allows us to bring Neosync's data anonymization and synthetic data technology into mental health care at scale.
> — Evis Drenova, now Staff Product Manager at Grow ([PR Newswire](https://www.prnewswire.com/news-releases/grow-therapy-raises-the-privacy-bar-in-mental-health-302567153.html))

Drenova has since [moved on again](https://talent.substack.com/p/why-i-joined-evis-drenova-entire).

---

## 8. The afterlife: what open-sourcing actually bought

Both companies did the honourable thing on the way out. It is worth being precise about what that achieved, because "we'll open source it if it doesn't work" is a tempting way to avoid thinking about longevity.

Supabase's commitment on 14 August 2024:

> The Snaplet team who joined Supabase have been helping Peter to migrate these projects to open source. Over the next few weeks we'll move these into the Supabase GitHub org and **pick up the ongoing maintenance**.
> — [Supabase blog](https://supabase.com/blog/snaplet-is-now-open-source)

What actually happened, two years on:

| Project | Last commit on `main` | Last release | Open issues | Weekly downloads | State |
|---|---|---|---|---|---|
| [copycat](https://github.com/supabase-community/copycat) | 14 Jan 2025 | v6.0.0, 14 Jan 2025 | 4 | 121,478 | Alive but static |
| [seed](https://github.com/supabase-community/seed) | **14 Aug 2024** — the day it was donated | 0.98.0, 30 Jul 2024 | 25 | 41,159 | Unmaintained, [known CVEs](https://github.com/supabase-community/seed/issues/213) |
| [snapshot](https://github.com/supabase-community/snapshot) | 5 Nov 2025 | v0.93.2, 2 Aug 2024 | 12 | 9,808 | **Archived 11 May 2026** |

(Verified via the GitHub and npm registry APIs on the research date.)

Every contributor to all three repositories is either a former Snaplet employee or a drive-by of one or two commits. Copycat: justinvdm 160, peterp 12, khaya-zulu 7, avallete 6, then singles. Seed: jgoux 115, justinvdm 92, avallete 57, CarelFdeWaal 18 — nobody else. Snapshot: CarelFdeWaal 30, jgoux 9, khaya-zulu 4, bmbferreira 2, kiwicopple 1 ([contributors](https://github.com/supabase-community/snapshot/graphs/contributors)).

There is one genuinely hopeful signal in that table. In November 2025, fifteen months after the shutdown, a former Snaplet engineer spent a day trying to compile `snapshot` into **per-platform binaries** — commits titled "compile binary for each platform", "try to build mac binary on macos M1" ([commits](https://github.com/supabase-community/snapshot/commits/main)), 4–5 November 2025. Nothing followed: no more commits landed on `main` after that week, and the repository was eventually archived six months later, on [11 May 2026](https://github.com/supabase-community/snapshot). Somebody worked out that the thing standing between this tool and a long life was its Node packaging — and then, it seems, moved on; the archival did not follow within days, it followed a further half-year of silence.

**The lesson is not "don't open source". It is that open-sourcing at the end transfers the code but not the capacity to maintain it**, and that what survives is determined almost entirely by how easy the artefact is to keep alive: a pure-function library with no runtime dependencies (copycat) coasts for years; a CLI with a native `better-sqlite3` dependency ([#15](https://github.com/supabase-community/snapshot/issues/15)) and a hosted LLM call ([#198](https://github.com/supabase-community/seed/issues/198)) rots within months.

---

## 9. Open questions and gaps

- **No formal Snaplet retrospective exists — this was searched for, not assumed.** Searches for a Pistorius postmortem, a "lessons learned" post, or a founder interview after July 2024 turned up nothing beyond the shutdown post, the Supabase quote, and the January 2026 HN corrections. His pre-monetisation [Jamstack Radio interview](https://www.heavybit.com/library/podcasts/jamstack-radio/ep-102-database-accessibility-with-peter-pistorius-of-snaplet) is from June 2022. If a longer account exists it is likely on X or in the (now closed) Snaplet Discord, neither reachable from this environment. **Neosync's equivalent does exist and is used in §7**: Evis Drenova's [evis.dev, "It's the market, stupid"](https://www.evis.dev/posts/is_the_market_stupid) (17 Aug 2025) is exactly the founder retrospective Snaplet never produced.
- **One Neosync founder interview was located but not mined.** [Ben Wolfson Podcast #5 with Evis Drenova](https://podcasts.apple.com/us/podcast/lessons-from-silicon-valley-inside-the-mind-of/id1783893837?i=1000679541457) (7 Dec 2024) is a longer, less shutdown-focused conversation than the evis.dev post. It could not be transcribed from this environment — audio is out of reach here, not absent from the record — so §7 flags it as an unopened source. A future pass with audio transcription access should mine it.
- **Snaplet's funding total is unverified.** Aggregators disagree. The only first-party evidence found is the site banner "We are proud to be part of the Netlify Jamstack Innovation Fund" ([Wayback](http://web.archive.org/web/20230411000318/https://www.snaplet.dev/)). Crunchbase and PitchBook are paywalled.
- **Neither company published revenue, customer counts, or churn.** "We have not reached the necessary adoption levels" and "Lack of insights into who is actually using Neosync" are as specific as the record gets. Neosync's "hundreds of companies" self-hosting ([Show HN](https://news.ycombinator.com/item?id=41569240)) is a founder's estimate with no method attached.
- **Evis Drenova's acquisition announcement on X (post `1951401561760080121`, ~1 Aug 2025) could not be fetched** — x.com returns HTTP 402 from this environment. Its existence and approximate date are attested by a search index; its contents are **unverified**. The GitHub and PR Newswire dates in §1 do not depend on it.
- **Both Discords are the missing corpus.** Both companies routed support to Discord — Snaplet advertised *"our legendarily responsive Discord support"* ([pricing FAQ](http://web.archive.org/web/20230930182525/https://www.snaplet.dev/pricing)) — so the richest record of user friction is in servers that are private or gone. Everything in §3 is what leaked into public trackers, which biases this document toward bugs that were reproducible enough to file.
- **Reddit was not consulted** (unreachable from this environment, per the research brief). Some sentiment likely lives in r/PostgreSQL, r/devops and r/ExperiencedDevs and is not represented here.
- **The three Product Hunt user quotes in §2 could not be independently verified.** `producthunt.com/posts/snaplet-seed` is behind a Cloudflare bot check from this environment (curl, browser tool, and a real browser session all receive the same "Just a moment…" interstitial rather than the page), and neither Wayback capture of the page (comments render client-side, after the crawl) contains the comment text. §2 now states this explicitly rather than presenting the quotes as confirmed.
- **What is a lazysnap user supposed to use today, if not Snaplet or Neosync?** Both are dead, and §8 argues users in this category are "perpetually migrating off a dead tool onto the next one that will die" — so the live alternatives named in passing elsewhere in this document matter. As of September 2026: **Greenmask** ([GreenmaskIO/greenmask](https://github.com/greenmaskio)) is actively maintained, with commits as recent as March 2026 and a MySQL engine in development alongside its Postgres support. **pgstream** ([xataio/pgstream](https://github.com/xataio/pgstream)) is actively maintained by Xata, with issues and workflow runs from March 2026. **datanymizer** ([datanymizer/datanymizer](https://github.com/datanymizer/datanymizer)) shows commit activity as recent as March 2026 and is not archived, contradicting the "unmaintained" characterization two 2024 HN commenters gave it in §2 — it may have been revived, or those commenters may have meant a specific fork. **Tonic.ai** ([tonic.ai](https://www.tonic.ai/)) is the one still-standing venture-funded player, having raised a $35M Series B ([press release](https://www.tonic.ai/press-releases/series-b)) and grown to roughly 104 employees; it is closed-source and priced for enterprise, i.e. the Snaplet/Neosync cloud model rather than the self-hosted CLI model. Read against Drenova's "It's the market, stupid": the category has real, actively-developed open-source alternatives and one funded incumbent, which argues there is *some* demand — but Tonic is the only one that has found a durable business model in it, and it did so by selling to enterprises, not developers running a CLI. That is either the opportunity (open ground below Tonic's price point) or the warning (the developer-facing, free, self-hosted segment of this market has never sustained a company) depending on which side of Drenova's test lazysnap is willing to be judged by.
- **The Neosync roadmap tool (`neosync.productlane.com`) still resolves and returns HTTP 200**, but its content could not be extracted — the page renders client-side and the fetch returned only the Productlane shell. Whatever vote counts and unshipped items it holds are **unverified**.
- **Secondary sources were used only for corroborated claims.** Two competitor-authored comparison posts ([seedfa.st](https://seedfa.st/blog/neosync-alternative), [xata.io](https://xata.io/blog/the-5-best-data-anonymization-tools-for-development-teams-in-2026)) surfaced during research; nothing in this document rests on them. xata's claim that Neosync lacked MongoDB support is **wrong** — [MongoDB shipped in June 2024](https://github.com/nucleuscloud/neosync/blob/main/docs/blog/2024-06-20-mongodb.md) — which is a useful reminder of how fast secondary sources rot in this space.

---

## 10. Five things we will not copy

**1. A default that copies untransformed data.**
Snaplet's transform mode defaulted to `unsafe` — *"the data for columns not specified in the config is simply copied over as is without transformation"* — and their own documented transcript shows the PII detector failing (`Smart shape prediciton failed`) on the same screen where the config is generated. A tool that exists to keep customer emails off laptops shipped a default that puts customer emails on laptops. lazysnap's CONCEPT.md already forbids a wholesale-disable flag; this evidence says go further and forbid the *mode*. There should be no `unsafe`: only "masked", and "explicitly opted out, per column, recorded in `lazysnap.yml`". And the classifier's failure mode must be **mask more**, never **pass through**. *Evidence: [transform docs](https://github.com/snaplet/docs-old/blob/main/docs/04-references/data-operations/02-transform.md).*

**2. Tuning knobs in place of a working default.**
Snaplet's subset config reached nine top-level keys — `enabled`, `targets[]`, `keepDisconnectedTables`, `followNullableRelations`, `maxCyclesLoop`, `maxChildrenPerNode`, `taskSortAlgorithm`, `eager`, `traversalMode` — with four more (`percent`, `rowLimit`, `where`, `orderBy`) nested inside every entry of `targets[]`. Thirteen knobs in total, and the docs instructed users to set `maxCyclesLoop` to 0 and *"gradually increment it until the subset of data you fetch trough a relationship is enough for your use case"*, while separately admitting a 5% subset *"may ultimately include more than 5%"*. That is the whole hard problem handed back to the user as homework, plus a headline number that does not mean what it says. lazysnap's plan step must decide these itself, print what it decided in one line of plain language, report the honest resulting row count rather than the requested one, and only then write the decisions into `lazysnap.yml` as a record. *Evidence: [subset docs](https://github.com/snaplet/docs-old/blob/main/docs/04-references/data-operations/04-reduce.md), [final capture docs](https://snaplet-snapshot.netlify.app/snapshot/core-concepts/capture).*

**3. A control plane between the developer and their own database.**
Neosync's minimum self-hosted footprint is app + api + worker + Postgres + Redis + two init containers + Temporal, with Temporal explicitly out of scope of the deploy guide, four Helm charts, and Istio and Datadog hooks in the docs. Its Docker network stopped a user connecting to a Postgres container on his own laptop (*"I was surprised to get snagged here"*), and its CLI required an API key and a server-side connection id, so there was never a standalone path. Snaplet arrived at the same place from the other side: a cloud wizard, a VS Code extension, and a proxy on `localhost:2345` fronting a hosted preview database as the recommended quickstart. lazysnap's "one static binary" and "the TUI is a thin layer over the CLI" rules are the correct reaction, and they are **load-bearing, not stylistic**. A corollary: no native compiled dependency and no runtime that rots — `@snaplet/snapshot` was killed as much by `better-sqlite3` and Node 22 as by the shutdown. *Evidence: [compose.yml](https://github.com/nucleuscloud/neosync/blob/main/compose.yml), [Kubernetes deploy docs](https://github.com/nucleuscloud/neosync/blob/main/docs/docs/deploy/kubernetes.md), [CLI sync docs](https://github.com/nucleuscloud/neosync/blob/main/docs/docs/cli/sync.md), [#3420](https://github.com/nucleuscloud/neosync/issues/3420), [Snaplet quickstart](https://github.com/snaplet/docs-old/blob/main/docs/02-quickstart.md), [snapshot#15](https://github.com/supabase-community/snapshot/issues/15).*

**4. Breadth before the first path is excellent.**
Neosync shipped MongoDB, DynamoDB, SQL Server, S3, GCS, a Terraform provider, a Python SDK, RBAC, webhooks, job hooks and a Slack integration inside the sixteen months its changelog covers — and *still* could not subset a parent table by a filter on its child, the feature a user asked for in February 2025 and never got. Snaplet added preview databases, a VS Code extension, Netlify and Vercel plugins while its config format broke twice and its docs went stale enough to break Supabase's own guide. lazysnap's non-goals ("Databases other than PostgreSQL… come after the Postgres path is excellent") are the right instinct; the failure mode is that **both companies wrote that sentence and then did not obey it**. The test is not whether we say no to MySQL. It is whether we say no to the fifteenth transformer, the Terraform provider and the Slack integration while [#3227](https://github.com/nucleuscloud/neosync/issues/3227) is still open. *Evidence: [Neosync changelog](https://github.com/nucleuscloud/neosync/tree/main/docs/blog), [#3227](https://github.com/nucleuscloud/neosync/issues/3227), [migration guide](https://github.com/snaplet/docs-old/blob/main/docs/05-guides/migration-new-config.md), [supabase#21089](https://github.com/supabase/supabase/issues/21089).*

**5. A runtime dependency on anything we operate.**
Snaplet Seed calls a hosted LLM, so when the company died the tool degraded into a nag banner and rate-limit errors, and the top community requests became [Ollama support](https://github.com/supabase-community/seed/issues/198), [a custom OpenAI-compatible endpoint](https://github.com/supabase-community/seed/issues/204) and [a way to turn the AI banner off](https://github.com/supabase-community/seed/issues/211). Snapshot's error messages still tell users "we have been notified" and point at a dead Discord. Neosync's archived README links to three dead hostnames (`www.neosync.dev`, `docs.neosync.dev`, `assets.nucleuscloud.com` — the other non-GitHub hosts it links to, `neosync.productlane.com`, Discord, Grow Therapy's site and Codecov/Artifact Hub, are all still alive). lazysnap must have **no runtime call to anything we run** — no telemetry gate, no model call, no licence check, no registry, no shortlink, no image CDN in the terminal output — such that the binary a user has today behaves identically in five years with us gone, and every error message it prints names something that will still exist. *Evidence: [seed#198](https://github.com/supabase-community/seed/issues/198), [seed#211](https://github.com/supabase-community/seed/issues/211), [snapshot#17](https://github.com/supabase-community/snapshot/issues/17), [Neosync README](https://github.com/nucleuscloud/neosync/blob/main/README.md), DNS table in §0.*

---

## 11. Three things we must copy

**1. Deterministic masking as a separate, tiny, dependency-free library.**
Copycat — *"like faker.js, but deterministic: for any given input it'll always produce the same output"* — is the only piece of either company that thrived. Two years after Snaplet died it does **121,478 downloads a week**, twelve times the snapshot tool it was built to serve and more than seed and snapshot combined, on a codebase whose last commit was January 2025. An HN user naming what he missed about Snaplet named it by function (*"good (stable) generators for names, emails"*). Determinism is also what makes joins survive masking, which is the technical crux of lazysnap's step 4. Build it as a standalone, importable, separately-testable unit with its own name and no dependency on the rest of lazysnap, so that **if lazysnap fails, this outlives it**. *Evidence: [npm](https://registry.npmjs.org/@snaplet/copycat) (121,478/wk vs 9,808/wk for snapshot), [Supabase blog](https://supabase.com/blog/snaplet-is-now-open-source), [HN comment](https://news.ycombinator.com/item?id=41867092), [contributor graph](https://github.com/supabase-community/copycat/graphs/contributors).*

**2. Introspect first, propose a config, ask the human to review it — in the terminal, without an account.**
Snaplet's `setup` is the best-designed thing either company shipped: connect, introspect, detect PII candidates, *generate* a config with suggestions, and print *"Snaplet has introspected your database structure and generated suggested transformations for your data in snaplet.config.ts. Please review them."* Crucially the same page says *"A user account is **not required**"*, and the surviving self-hosting guide is three commands against two connection strings. That is lazysnap's zero-config principle already validated in the field — emit configuration as a record of what happened, never demand it up front. Two things to fix: make the detector's failure mode *safe* rather than pass-through, and **never move the review step into a web dashboard**, which is precisely how Snaplet lost the plot. *Evidence: [transform docs setup transcript](https://github.com/snaplet/docs-old/blob/main/docs/04-references/data-operations/02-transform.md), [self-hosting docs](https://snaplet-snapshot.netlify.app/snapshot/guides/self-hosting), [quickstart](https://github.com/snaplet/docs-old/blob/main/docs/02-quickstart.md) for the counter-example.*

**3. Referential integrity as the headline, verified on every run, with an undirected graph from day one.**
It is the single thing users praised Neosync for, unprompted, in public, by name: the ClickHouse-obfuscator user switching because *"it seems that you directly address the referential integrity issue, which is great"*; the datanymizer user shopping for a replacement; the intern who found it *"the most time consuming phase of the pseudonymization process"*; the founder confirming *"the referential integrity and constraints part is usually the most complicated part"*; and, eighteen months after Snaplet's death, a user saying *"as long as referential integrity is intact the specifics matter less."* It is also exactly where Neosync's design ran out of road (parent-by-child, [#3227](https://github.com/nucleuscloud/neosync/issues/3227)) and where Snaplet's precision claims wobbled (*"a 5% subset… may ultimately include more than 5%"*). So: lazysnap's `✓ foreign keys verified` check must be **non-optional, run on every load, and fail the run loudly** — it is the promise that distinguishes this from `pg_dump | sed`. And the subsetting graph should be **undirected from the start**: treat any filtered table as a root and follow edges both ways. That is the thing Neosync's maintainer said users kept asking for, said was "definitely possible", and never built. *Evidence: [HN, imiric](https://news.ycombinator.com/item?id=40446843), [HN, gregwebs](https://news.ycombinator.com/item?id=40449655), [HN, mathisd](https://news.ycombinator.com/item?id=40446590), [HN, edrenova](https://news.ycombinator.com/item?id=40446813), [HN, ljm](https://news.ycombinator.com/item?id=46514333), [Neosync #3227](https://github.com/nucleuscloud/neosync/issues/3227), [Snaplet subset docs](https://github.com/snaplet/docs-old/blob/main/docs/04-references/data-operations/04-reduce.md).*
