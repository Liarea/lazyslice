# Blog draft: What I learned reading Snaplet's and Neosync's issue trackers

Draft only. Nothing here is posted; the maintainer edits and posts it.
Word count of the post body below: under 900 words. Every claim links to a
primary source — the same ones research/POSTMORTEMS.md cites, or a page
re-checked today (2026-09-22, noted inline where it matters).

---

## What I learned reading Snaplet's and Neosync's issue trackers

Two companies built almost exactly what I'm building: point a tool at a
production Postgres database, subset it, mask the personal data, load it
somewhere safe. Both are gone. Snaplet shut down on 31 August 2024
([shutdown post](http://web.archive.org/web/20240716025550/https://www.snaplet.dev/post/snaplet-is-shutting-down)).
Neosync's repository was archived on 30 August 2025 after the team was
acquired by Grow Therapy — still true as of today's check against
[GitHub's own archive banner](https://github.com/nucleuscloud/neosync)
("archived by the owner on Aug 30, 2025"). I read both issue trackers, both
shutdown threads, and the Hacker News comments under both launches, to
find out what to copy and what to refuse.

**The best thing either of them built wasn't the product.** Snaplet's
deterministic fake-value library, Copycat, still pulls more than thirteen
times the weekly downloads of the actual snapshot tool it was built to
serve
([copycat](https://api.npmjs.org/downloads/point/last-week/@snaplet/copycat),
[snapshot](https://api.npmjs.org/downloads/point/last-week/@snaplet/snapshot),
both checked today, 2026-09-22) — on a codebase whose last commit was
January 2025
([commits](https://github.com/supabase-community/copycat/commits/main)).
An HN user, seven weeks after the shutdown, named exactly this when asked
what he missed: *"good (stable) generators for names, emails"*
([HN](https://news.ycombinator.com/item?id=41867092)). The boring,
dependency-free, deterministic part outlived the company. That's why
lazyslice's masking has to survive lazyslice.

**Referential integrity is the actual product, not a feature.** Users
praised Neosync for it unprompted: *"it seems that you directly address
the referential integrity issue, which is great"*
([HN](https://news.ycombinator.com/item?id=40446843)); the founder agreed
it was *"usually the most complicated part"*
([HN](https://news.ycombinator.com/item?id=40446813)). And it's exactly
where Neosync ran out of road: a user asked to subset from a table that
was both parent and child, the maintainer confirmed the engine only
walked foreign keys in one direction, called an undirected version
*"definitely possible... we just haven't tackled it yet"*, and it never
shipped
([issue #3227](https://github.com/nucleuscloud/neosync/issues/3227)). The
issue is still open, on an archived repository, with nobody able to
merge a fix. lazyslice's answer to that is to treat the graph as
undirected from the start, not to bolt it on later.

**The cloud was the product, and the target users said so out loud.** On
Neosync's own launch thread, a prospective user rejected the business
model in the same breath as praising the tool: *"if the original data
needs to be exported and sent to your Cloud for anonymization, it
defeats the entire purpose... I can't say that I trust your business
model to sustain a company around this product"*
([HN](https://news.ycombinator.com/item?id=40446843)). Fifteen months
later, the repository was archived. Snaplet's final quickstart pushed the
same way — sign into Snaplet Cloud, install a VS Code extension, point
your app at a proxy on `localhost:2345` fronting a hosted database
([quickstart](https://github.com/snaplet/docs-old/blob/main/docs/02-quickstart.md))
— which is the inversion of its own 2021 pitch, *"on your laptop in
minutes"*
([Wayback, 2021](http://web.archive.org/web/20210118202109/https://snaplet.dev/)).
lazyslice has no cloud to invert into: it's a static binary, and there is
nothing it calls home to.

**An unsafe default did exactly the damage you'd expect.** Snaplet's
transform step shipped three modes, and the default — `unsafe` — was
*"the data for columns not specified in the config is simply copied over
as is without transformation"*
([docs](https://github.com/snaplet/docs-old/blob/main/docs/04-references/data-operations/02-transform.md)),
on the same documented screen where the PII detector's own transcript
reads `Smart shape prediciton failed`. A failing detector plus a
pass-through default is production data on a laptop, silently. lazyslice
has no `unsafe` mode: an unclassified column stops the run.

**Complexity leaked into the config instead of staying in the code.**
Snaplet's subset config reached nine top-level keys, with the docs
instructing users to set one (`maxCyclesLoop`) to zero and *"gradually
increment it"* by trial and error against their own database
([subset docs](https://github.com/snaplet/docs-old/blob/main/docs/04-references/data-operations/04-reduce.md)).
That's the hard problem, handed back as homework. lazyslice's plan step
decides this itself and prints what it decided in one line.

**And packaging rot killed the thing that outlived the shutdown.** A
native dependency pinned below the Node 22 floor broke installs in
October 2024; the fix sat as a mergeable, unmerged pull request for six
months before the repository was archived and closed to merges forever
([issue #15](https://github.com/supabase-community/snapshot/issues/15),
[PR #22](https://github.com/supabase-community/snapshot/pull/22)). A
tool's error messages kept telling users it had "notified" a support
channel for a company that no longer existed
([issue #17](https://github.com/supabase-community/snapshot/issues/17)).
lazyslice ships one static binary with no runtime dependency to rot out
from under it.

None of this is a guarantee lazyslice survives where two well-built,
well-liked tools didn't — both founders independently landed on the same
word for why they died: *demand*, not execution
([evis.dev](https://www.evis.dev/posts/is_the_market_stupid),
[HN](https://news.ycombinator.com/item?id=46518012)). But it's a free,
local, dependency-light CLI, not a company, so the bar it has to clear is
lower: not "enough demand to fund a team," just "less painful than a raw
`pg_dump` or nothing." The full research, with every source, is in
[research/POSTMORTEMS.md](../../research/POSTMORTEMS.md).
