# Show HN draft

Draft only. Nothing here is posted; the maintainer edits and posts it.

## Title (67 characters, under the 80-character limit)

Show HN: lazyslice – subset, mask and load a local Postgres snapshot

## First comment (190 words, under the 200-word limit)

The problem: getting realistic data on your laptop today means a raw
pg_dump of production, customer emails and all, or hand-written seed data
that drifted from reality months ago.

Two companies built almost exactly this and both are gone. Snaplet shut
down in 2024; its docs repo still opens with "Snaplet the company has shut
down" (https://github.com/snaplet/docs/blob/main/README.md, unchanged since
13 Aug 2024), and its last package release was 2 Aug 2024
(https://registry.npmjs.org/@snaplet/snapshot). Neosync's repository was
archived 30 Aug 2025 after Grow Therapy acquired the team
(https://github.com/nucleuscloud/neosync — "archived by the owner on Aug
30, 2025"); its own README still links to three dead domains.

lazyslice does the same job: subset a production Postgres database by a
root table, follow foreign keys, mask personal data deterministically so
joins survive, load a small local copy. One static binary, no account, no
cloud, nothing it calls home to. v0.1.0 is PostgreSQL only and
pseudonymises, not anonymises — the row identifiers it can't hide and the
rest of the residual list are documented, not glossed over.

What would make you actually trust a tool in this category again, given
two of them already died?

## Source verification, done today (2026-09-22)

Both shutdowns were re-checked today, not assumed from research/POSTMORTEMS.md's
5 September research date, because the first comment states them as current fact:

- **Snaplet.** `github.com/snaplet/docs` README still reads "Goodbye, world /
  Snaplet the company has shut down, but our tools are now open source" as of
  today's fetch; the file's last commit is 13 Aug 2024
  (`gh api repos/snaplet/docs/commits --jq '.[0].commit.author.date'` →
  `2024-08-13T08:44:59Z`, run today). `https://registry.npmjs.org/@snaplet/snapshot`,
  fetched today, shows `time.modified` of `2024-08-02T09:24:33.720Z` — no
  release since. `www.snaplet.dev` and `snaplet.dev` return no DNS A record as
  of today (`dig +short`, run today). The founder's own correction — "I
  shutdown the startup" — is at
  https://news.ycombinator.com/item?id=46518012 (6 Jan 2026), re-read today.
- **Neosync.** `gh api graphql` against `nucleuscloud/neosync`, run today,
  returns `"isArchived": true, "archivedAt": "2025-08-30T18:23:18Z"`. The
  live repository page, fetched today, shows the banner "This repository was
  archived by the owner on Aug 30, 2025. It is now read-only" and the
  disclaimer "Neosync has been acquired by Grow Therapy. As a result, this
  repository is no longer actively maintained." `neosync.dev` resolves
  (Vercel) but returns `DEPLOYMENT_NOT_FOUND` (HTTP 404) as of today's `curl`.

Both are still dead today, seventeen days after research/POSTMORTEMS.md's
research date, and the sources above are the ones the first comment links to.
