# Naming: is `lazysnap` the right name?

**Recommendation: change the name to `lazysubset`.** Runner-up: `lazyslice`.

`lazysnap` is not legally blocked and its domains are free, but it fails on three counts
that matter more than domains. A Go TUI called `lazysnap` — same language, same family,
explicitly "inspired by lazygit and lazydocker" — was published to the Go module proxy ten
weeks ago and owns the `go install` path. The word "snap" already means *packaging* and
*backup* to the exact user CONCEPT.md describes. And "snapshot" means the whole database at
a point in time, which is the opposite of this product's central claim. `lazysubset` has
zero collisions in any namespace checked, is the category's own search term, and has
`.dev`, `.sh`, `.io` **and** `.com` free.

**Nothing has been purchased or registered.**

---

## 0. Method and how far to trust it

All registry and DNS data below was collected **2026-09-05** by direct query. Every channel
was validated against known-registered and known-free controls first, because two of them
give false answers if used naively:

| Channel | Query | Control that passed |
|---|---|---|
| npm | `registry.npmjs.org/<name>` | `lazygit` 200 / `lazysubset` 404 |
| PyPI | `pypi.org/pypi/<name>/json` | `requests` 200 / `lazysubset` 404 |
| crates.io | `crates.io/api/v1/crates/<name>` | `serde` present / `lazysubset` "does not exist" |
| Homebrew | `formulae.brew.sh/api/formula/<name>.json` | `lazygit`, `lazysql`, `snap`, `tarsnap` all 200 |
| GitHub | `gh api /users/<name>`, `/search/repositories?q=<name>+in:name` | authenticated, live |
| Go proxy | `proxy.golang.org/<module>/@v/list` | `jesseduffield/lazygit` → v0.59.0; nonexistent module → 404 |
| `.dev` | `pubapi.registry.google/rdap/domain/<name>` | `web.dev` 200, `google.dev` 200, random string 404 |
| `.io` / `.sh` | `whois -h whois.nic.io` / `whois.nic.sh` | `fly.io` and `esm.sh` return records; free names return "Domain not found." |
| `.com` | `rdap.org/domain/<name>.com` | `github.com` 200 / free name 404 |

Two traps worth recording, because they would have produced wrong answers:

- **`rdap.org` is not reliable for `.dev`.** It returned `000` (connection failure) for a
  random string rather than a clean 404, so "404 = free" could not be distinguished from
  "request died". The [IANA RDAP bootstrap file](https://data.iana.org/rdap/dns.json) names
  `https://pubapi.registry.google/rdap/` as authoritative for `.dev`; that endpoint returns
  200 for `web.dev` and 404 for a random string, so it is trustworthy. A different Google
  host, `www.registry.google/rdap/`, returns 404 for `web.dev` — a false "available".
  All `.dev` results below come from `pubapi.registry.google`.
- **Hacker News Algolia search is typo-tolerant and word-splitting.** A raw query for
  `lazysnap` reports `nbHits: 459`, but every hit is a fuzzy match (`Lazyslop`, `lazy`,
  `snap`) and **zero** contain the literal string. All HN counts below are exact-substring
  filtered, not raw `nbHits`.

Trademark clearance could **not** be completed. See §6.

---

## 1. `lazysnap` collision audit

| Namespace | Status | Detail |
|---|---|---|
| npm | **TAKEN** | [`lazysnap` v0.0.1](https://registry.npmjs.org/lazysnap) — "Monorepo for `@lazysnap/*` — image loading with observability, retry, and blur-up" |
| npm `@lazysnap/*` scope | free | `@lazysnap/core`, `/react`, `/angular` all 404 — the monorepo names them but never published them |
| PyPI | free | 404 |
| crates.io | free | "crate `lazysnap` does not exist" |
| Homebrew formula | free | [404](https://formulae.brew.sh/api/formula/lazysnap.json) |
| Homebrew cask | free | 404 |
| GitHub account | **TAKEN** | [github.com/LazySnap](https://github.com/LazySnap) — User, 0 public repos, created 2024-04-13 |
| GitHub repos | **14 matches** | see below |
| Go module proxy | **TAKEN** | [`github.com/jpdarago/lazysnap`](https://proxy.golang.org/github.com/jpdarago/lazysnap/@v/list) — v0.1.0, v0.2.0; v0.2.0 tagged **2026-06-29**, commit `5013d7d` |
| `lazysnap.dev` | free | authoritative RDAP 404 |
| `lazysnap.io` | free | `whois.nic.io` → "Domain not found." |
| `lazysnap.sh` | free | `whois.nic.sh` → "Domain not found." |
| `lazysnap.com` | **REGISTERED** | [RDAP](https://rdap.org/domain/lazysnap.com) → handle `2622607671_DOMAIN_COM-VRSN`, Namecheap, Cloudflare nameservers |
| Hacker News | 0 exact hits | no goodwill attached to the name, and none to lose |
| USPTO | **unverified** | §6 |

### The one collision that actually hurts

[`jpdarago/lazysnap`](https://github.com/jpdarago/lazysnap) (Go, pushed 2026-06-29). Its
[README](https://raw.githubusercontent.com/jpdarago/lazysnap/main/README.md) opens:

> "A terminal UI for [tarsnap](https://www.tarsnap.com/), inspired by
> [lazygit](https://github.com/jesseduffield/lazygit) and
> [lazydocker](https://github.com/jesseduffield/lazydocker)."

Same name, same language, same TUI-over-a-CLI premise, same declared lineage, and it is
**live in the Go module proxy at v0.2.0** — so it owns `go install github.com/jpdarago/lazysnap@latest`.
It has 0 stars, so nothing is famous here; the problem is not fame, it is that any
announcement of "lazysnap, a lazy-family TUI in Go" now needs a disambiguating sentence,
forever. That is a tax on every README, every HN post, every conference slide.

The other thirteen repos are less serious but shape the search results:

- [`AlthafPattan/lazysnap`](https://github.com/AlthafPattan/lazysnap) (TypeScript, pushed
  2026-04-09) — the npm package above.
- [`jwdev42/lazysnapshotter`](https://github.com/jwdev42/lazysnapshotter) — a btrfs backup
  frontend.
- [`hugolevacher/lazySnapchat`](https://github.com/hugolevacher/lazySnapchat) — unrelated.
- **Eight implementations of the "Lazy Snapping" computer-vision algorithm**:
  [`zjxeditor/LazySnapping`](https://github.com/zjxeditor/LazySnapping) (10★),
  [`wuyongxiang/LazySnapping-Android`](https://github.com/wuyongxiang/LazySnapping-Android) (5★),
  [`vyerneni/LazySnapping`](https://github.com/vyerneni/LazySnapping) (3★),
  `liaoxl/LazySnappingWithGMM`, `matinJ/lazysnapping`, `namthse03439/LazySnapping`,
  `MaxtirError/LazySnaping`, `StefanoFochesatto/LazySnappingGraphFlow`.

That last group exists because **Lazy Snapping is a well-known SIGGRAPH 2004 paper** — an
interactive image cutout method by Li, Sun, Tang and Shum
([ACM DL](https://dl.acm.org/doi/10.1145/1015706.1015719),
[paper PDF](https://home.cse.ust.hk/~cktang/sample_pub/lazy_snapping.pdf),
[Microsoft Research](https://www.microsoft.com/en-us/research/publication/lazy-snapping/)).
Roughly half the name's existing search surface is image segmentation and will stay that way;
a twenty-two-year-old citation graph is not something a new project out-ranks.

---

## 2. The bigger problem is "snap", not the collision list

The collisions alone would be survivable. The semantics are the real objection, in three layers.

### "snap" already means package manager to a terminal user

Canonical's [snapd/snap](https://github.com/canonical/snapd) (2,046★) is the Linux packaging
format whose primary verb is literally `snap install`; [Snapcraft](https://snapcraft.io/) is
its store. Homebrew itself ships formulae named [`snap`](https://formulae.brew.sh/formula/snap)
and [`snapcraft`](https://formulae.brew.sh/formula/snapcraft). A tool distributed as
`brew install lazysnap` sits one prefix away from the thing that installs software.

### "snap" already means filesystem backup to a terminal user

Homebrew also carries [`tarsnap`](https://formulae.brew.sh/formula/tarsnap),
[`rsnapshot`](https://formulae.brew.sh/formula/rsnapshot) and
[`snapraid`](https://formulae.brew.sh/formula/snapraid). Add `jwdev42/lazysnapshotter`
(btrfs backups) and `jpdarago/lazysnap` (a *tarsnap* TUI), and the established reading of
"lazy" + "snap" in a terminal context is **backups**, not databases.

### "snap" is taken in this exact product category — twice

- **[DBSnapper](https://dbsnapper.com/)** is a live commercial product — "Automated Database
  Snapshotting and De-identification", with [flat-rate pricing at $0 / $300 / $500 per month](https://dbsnapper.com/).
  Its [v2 release (2024-02-27)](https://dbsnapper.com/blog/introducing-dbsnapper-v2) added
  Database Subsetting, letting teams "work with smaller, relationally complete, snapshots of
  their production databases", for "PostgreSQL and MySQL".

  Set that beside CONCEPT.md's one-liner — "a small, referentially complete, anonymised copy"
  — and the phrases are near-identical. Shipping `lazysnap` into this market invites exactly
  one reading: that we are DBSnapper's unofficial TUI. (Their own GitHub repo,
  [`dbsnapper/dbsnapper`](https://github.com/dbsnapper/dbsnapper), is only 9★ and is the docs
  site; the product, not the repo, is the brand we would be shadowing.)
- **Snaplet** ran "Snapshot" — "captures, transforms, and restores database snapshots with
  advanced subsetting" — then shut down in August 2024; Supabase took over the code on
  2024-08-14 under MIT ([Supabase: Snaplet is now open source](https://supabase.com/blog/snaplet-is-now-open-source),
  [supabase-community/seed](https://github.com/supabase-community/seed), 790★). HN carries the
  whole arc, from ["Show HN: Snaplet Seed"](https://hn.algolia.com/?query=snaplet) to "Snaplet
  Is Shutting Down". "Snap" in database anonymisation now trails a dead product behind it.

### And "snap" describes the wrong thing

CONCEPT.md promises "a **small**, referentially complete, anonymised copy", and its non-goals
rule out whole-database work. A snapshot is, by definition, *the whole thing at a point in
time*. The product's entire differentiator is that it is not the whole thing. The name argues
against the pitch on first contact — precisely the failure CLAUDE.md's rule
"documentation is never the fix for a confusing first run" tells us to fix at the source
rather than explain away.

### To be clear: the "lazy" prefix is fine — keep it

This is an objection to the stem, not the family. The convention is healthy and active:
[lazygit](https://github.com/jesseduffield/lazygit) (82.0k★),
[lazydocker](https://github.com/jesseduffield/lazydocker) (52.7k★) and
[lazynpm](https://github.com/jesseduffield/lazynpm) (862★) from Jesse Duffield
([five-year retrospective](https://jesseduffield.com/Lazygit-5-Years-On/)), plus
[lazysql](https://github.com/jorgerojas26/lazysql) (4.3k★), a cross-platform TUI database
tool. Homebrew currently carries **eleven** `lazy*` formulae:
[`lazygit`](https://formulae.brew.sh/formula/lazygit),
[`lazydocker`](https://formulae.brew.sh/formula/lazydocker),
[`lazysql`](https://formulae.brew.sh/formula/lazysql),
[`lazyjj`](https://formulae.brew.sh/formula/lazyjj),
[`lazyssh`](https://formulae.brew.sh/formula/lazyssh),
[`lazyjournal`](https://formulae.brew.sh/formula/lazyjournal),
[`lazymake`](https://formulae.brew.sh/formula/lazymake),
[`lazyrsync`](https://formulae.brew.sh/formula/lazyrsync),
[`lazycut`](https://formulae.brew.sh/formula/lazycut),
[`lazycontainer`](https://formulae.brew.sh/formula/lazycontainer),
[`lazy-tmux`](https://formulae.brew.sh/formula/lazy-tmux).
Note that `lazysql` is already a database TUI in the family — the prefix carries real,
relevant recognition here. Keep the prefix. Change the stem.

---

## 3. Five alternatives

Constraints applied: keeps the "lazy" family, pronounceable, short to type, and at least one
of `.dev` / `.sh` / `.io` free. All five clear the domain bar with all three free.

| Name | Chars | npm | PyPI | crates | brew | GH acct | GH repos | Go proxy | .dev | .sh | .io | .com |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| **lazysubset** | 10 | free | free | free | free | free | **0** | free | free | free | free | **free** |
| **lazyslice** | 9 | free | **taken** | free | free | **taken** | 2 | free | free | free | free | taken |
| **lazycarve** | 9 | free | free | free | free | free | **0** | free | free | free | free | **free** |
| **lazyfixture** | 11 | free | free | free | free | free | **0** | free | free | free | free | **free** |
| **lazyscoop** | 9 | free | free | free | free | free | **0** | free | free | free | free | **free** |
| *lazysnap (for comparison)* | 8 | **taken** | free | free | free | **taken** | **14** | **taken** | free | free | free | **taken** |

### lazysubset — *the recommendation*

```
$ lazysubset
```

**The only candidate clean in every namespace checked**, including `.com`: zero hits on npm,
PyPI, crates.io, both Homebrew taps, the GitHub account namespace, GitHub repo search
(`total_count: 0`), and the Go module proxy; `.dev`, `.sh`, `.io` and `.com` all unregistered.

More importantly, it is **the category's own search term**. Every serious tool in this space
calls the operation subsetting:

- [Jailer](https://github.com/Wisser/Jailer) (3,195★) — "Database Subsetting and Relational
  Data Browsing Tool"; the [project site](https://wisser.github.io/Jailer/) says it "creates
  small slices from your productive database and imports the data into your development and
  test environment (consistent and referentially intact)".
- [TonicAI/condenser](https://github.com/TonicAI/condenser) (337★) — "Condenser is a database
  subsetting tool".
- [Tonic Subset](https://tonic.ai/products/tonic-subset) — "reduces full production databases
  to targeted slices of the data your developers actually need, while keeping primary and
  foreign key relationships intact".
- [DBSnapper v2](https://dbsnapper.com/blog/introducing-dbsnapper-v2) — "Database Subsetting".
- [Neosync](https://github.com/nucleuscloud/neosync) (4,141★) — anonymise and sync production
  data across environments.

Hacker News carries **21 stories** matching "database subsetting", led by
["Jailer: A tool for database subsetting"](https://hn.algolia.com/?query=database%20subsetting&type=story)
at 128 points and ["Show HN: Condenser – A database subsetting project"](https://github.com/TonicAI/condenser)
at 26. For a project with no marketing budget, whose acquisition channel is somebody typing
the problem into a search box, matching the term of art is worth more than being evocative.

Costs, honestly: ten characters, and "subset" reads a little drier than the family's playful
register. Ten characters is not disqualifying — `lazydocker` is also ten and is the second
most-starred tool in the family. The dryness is real but it buys literal accuracy: the tool
subsets, and says so.

One further caveat: a descriptive name is a weak trademark. If a hosted service is ever
planned, `lazysubset` would be hard to register. CONCEPT.md lists "a web UI or a hosted
service" as a v1 non-goal, so this is a cost deferred rather than paid — but it is the single
best argument for `lazyslice` instead.

### lazyslice — *the runner-up*

```
$ lazyslice
```

The better-sounding name, and already the team's own vocabulary: CONCEPT.md's hero transcript
says `planning… 23 tables in slice, ~18k rows` without anyone having proposed the word. It is
also the word Jailer and Tonic reach for in prose ("small slices", "targeted slices"). Nine
characters, two syllables, sits naturally beside `lazygit` and `lazysql` in a `brew search`
listing.

Its collisions are all genuinely low-cost, and worth stating precisely so the trade is visible:

- [PyPI `lazyslice` 0.3.0](https://pypi.org/project/lazyslice/) — "Lazy slicing and transpose
  operations for h5py and zarr". Irrelevant channel: CONCEPT.md ships "one static binary,
  one-line install", so we never ask anyone to `pip install`.
- [github.com/lazyslice](https://github.com/lazyslice) — Organization, created 2020-08-12,
  one Haskell repo last pushed 2020-08-31. It holds the org name but publishes no Go module
  (`proxy.golang.org/github.com/lazyslice/lazyslice/@v/list` returns 200 with an **empty**
  body — repo present, zero versions), so `go install` is unaffected. Not owning the org costs
  little: `lazygit` lives at `jesseduffield/lazygit` and `lazysql` at `jorgerojas26/lazysql`.
  The family has never required a matching org.
- [catalystneuro/lazyslice](https://github.com/catalystneuro/lazyslice) — 3★, the same
  h5py/zarr project. Negligible.
- `lazyslice.com` registered; `.dev`, `.sh`, `.io` free.

The objection that decides it against `lazysubset`: **"lazy" + "slice" already means lazy
evaluation of a sequence**, and that is not hypothetical — the dormant `lazyslice/lazyslice`
org repo is *Haskell*, and both PyPI packages are lazy-evaluation libraries. If this tool ships
as a Go binary (the family convention, and what "one static binary" implies), `lazyslice` reads
to a Go developer as a lazy-slice library before it reads as a database tool. That is a
quieter version of the same mistake `lazysnap` makes.

### lazycarve

```
$ lazycarve
```

Clean in every namespace including `.com`, nine characters, and it names the mechanism with
some energy — you carve a piece off the whole. Web search finds no software using it.

The cost is discoverability: nobody searches for "database carving". It is a good brand and a
bad keyword, which is the reverse of what a tool with no distribution needs in year one.

### lazyfixture

```
$ lazyfixture
```

Clean everywhere including `.com`, and it names the **user's outcome** in the user's own
vocabulary — a backend developer calls the data in their local database "fixtures".

Two costs. Eleven characters is the longest of the five. And in most frameworks a "fixture"
means hand-authored static data — which is precisely the thing CONCEPT.md says these
developers are stuck maintaining ("hand-written seed data that drifted from reality months
ago"). The name risks describing the problem rather than the fix.

### lazyscoop

```
$ lazyscoop
```

Clean everywhere including `.com`, nine characters, and it carries the family's playful
register — scoop a cupful out of prod.

But it repeats `lazysnap`'s structural error. [Scoop](https://github.com/ScoopInstaller/Scoop)
(24,628★, [scoop.sh](https://scoop.sh)) is "a command-line installer for Windows" — a package
manager. A CLI tool named `lazyscoop`, distributed through package managers, reads as a TUI
for Scoop exactly as `lazysnap` reads as a TUI for snap. "Scoop" also says nothing about
referential completeness or masking; it describes a careless gesture, and this tool's whole
claim is care.

### Considered and rejected

| Name | Why not |
|---|---|
| `lazyclone` | npm and PyPI both taken, GitHub account taken, **134** repo matches; and "clone" means a full copy |
| `lazyprune` | [crates.io `lazyprune`](https://crates.io/api/v1/crates/lazyprune) exists with 12 published versions (updated 2026-05-15) |
| `lazyseed` | [github.com/lazyseed](https://github.com/lazyseed) org taken; worse, "seed" now means *generated* data here — [supabase-community/seed](https://github.com/supabase-community/seed) is a schema-driven generator, and "synthetic data generation from a schema alone" is an explicit CONCEPT.md non-goal |
| `lazysample` | "sampling" implies taking rows independently, which breaks referential integrity — the one thing this tool guarantees |
| `lazymask` | Clean registries, but `.com` taken and it names step 4 of 5; subsetting is the harder problem and the real differentiator. Minor noise: [`LazyMask` in spectral-cube](https://spectral-cube.readthedocs.io/en/latest/api/spectral_cube.masks.LazyMask.html) |
| `lazysnip` | GitHub org [lazysnip](https://github.com/lazysnip) taken, 4 repo matches, `.com` taken; and it stays inside `snap`'s phonetic neighbourhood |
| `lazycut` | Already a [Homebrew formula](https://formulae.brew.sh/formula/lazycut) — a video-trimming TUI |
| `lazyshrink`, `lazygraft` | Clean enough, but neither is category vocabulary and both sound like disk utilities |

---

## 4. Recommendation and reasoning

**Rename to `lazysubset`. Take `lazysubset.dev` as the canonical home; `.sh`, `.io` and `.com`
are also free if a hedge is wanted. Nothing has been bought.**

The reasoning, in the order it matters:

1. **It fixes the semantic error.** `snap` promises a copy of the whole database; the product
   delivers a small part of one. A name that contradicts the pitch costs an explanation in
   every README, every talk, and every first run — and CLAUDE.md says to fix that at the
   source, not in documentation.
2. **It is the only genuinely empty namespace.** `lazysnap` collides with a live Go TUI in
   the same family that owns the `go install` path, a published npm package, a taken GitHub
   account, a registered `.com`, and eight repos named after a 2004 SIGGRAPH algorithm.
   `lazysubset` collides with nothing, anywhere, in any of the eleven channels checked.
   Renaming to something *partly* occupied would repeat a softer version of the mistake we
   are correcting.
3. **It matches how people search for this problem.** "Database subsetting" is what Jailer,
   Condenser, Tonic and DBSnapper all call it, with 21 HN stories behind the phrase. A
   zero-budget CLI tool is found by keyword or not at all.
4. **It avoids a live competitor's brand.** [DBSnapper](https://dbsnapper.com/) sells
   snapshot + subset + de-identify for Postgres today, at $300–500/month, describing its
   output in almost exactly CONCEPT.md's words. `lazysnap` reads as its TUI wrapper.
5. **It types acceptably.** Ten characters, three syllables, no ambiguous letters, and the
   same length as `lazydocker`.

**If brandability is weighted above discoverability and namespace cleanliness, take
`lazyslice` instead.** It is the more memorable name, one character shorter, already the
team's own word, and its three collisions (a scientific PyPI package, a dormant 2020 org, a
registered `.com`) break nothing operationally. The trade you accept is that "lazy slice"
independently means lazy evaluation to Go and Haskell developers. Both choices are defensible.
`lazysubset` is the safer and more findable one; `lazyslice` is the prettier one.

**If the name stays `lazysnap`**, then at minimum: do not publish to npm under that name (it
is taken), choose a GitHub owner path that is not `lazysnap/*` (the account is taken), and
expect to disambiguate from `jpdarago/lazysnap` in every announcement. None of that repairs
the DBSnapper adjacency or the fact that "snapshot" describes the opposite of what the tool
produces.

**Before committing** the chosen name to a binary, a domain purchase, or a public
announcement, run it through [tmsearch.uspto.gov](https://tmsearch.uspto.gov/) — see §6.

---

## 5. Migration cost if the name changes now

Low, and it only rises from here. The name currently appears in the repository directory name,
`CONCEPT.md`, `CLAUDE.md`, the emitted config filename `lazysnap.yml`, and the hero transcript.
No code, no published package, no domain, no Homebrew formula, no GitHub repo under the name,
and [zero Hacker News mentions](https://hn.algolia.com/?query=lazysnap) — so there is no
audience to migrate and no redirect to maintain. Phase 1 is the cheapest moment this decision
will ever have.

---

## 6. What could not be verified

**USPTO trademark search: unverified. No name in this document has been cleared for
trademark.**

The task named TESS, which no longer exists — it was retired and replaced by the Trademark
Search system at [tmsearch.uspto.gov](https://tmsearch.uspto.gov/)
([USPTO: Search our trademark database](https://www.uspto.gov/trademarks/search)). The
replacement is a JavaScript single-page app behind an AWS WAF challenge — the page source
loads `https://a434627cf98f.edge.sdk.awswaf.com/.../challenge.js` — and exposes no documented
public search API. Every route available from this environment failed:

| Attempt | Result |
|---|---|
| `POST tmsearch.uspto.gov/api-v1-0-0/tmsearch` | 405 `MethodNotAllowed` from S3 (static hosting, not an API) |
| Grep `tmsearch.uspto.gov/main.js` (1.9 MB bundle) for an API base | no API host found; endpoints are in lazy-loaded chunks |
| `api.uspto.gov/api/v1/trademark/search` | 403 `Missing Authentication Token` (needs an ODP API key) |
| `tsdrapi.uspto.gov` | returns a notice that API keys are required from October 2 |
| `trademarks.justia.com`, `trademarkia.com` | 403 to curl with a browser user agent |
| TMview API (`tmdn.org`) | connection failed |
| Web search for a `lazysnap` mark | [no record surfaced](https://www.uspto.gov/trademarks/search) — but absence from web search is not clearance |

Someone with interactive browser access should search the shortlist at
[tmsearch.uspto.gov](https://tmsearch.uspto.gov/) in classes **9** (software) and **42**
(SaaS / software services) before the name is final.

Three secondary gaps:

- **Non-US and common-law marks** (EUIPO/TMview, UKIPO, CIPO) were not checked; TMview was
  unreachable.
- **Company and business-name registers** were not checked for any candidate.
- **Domain availability is not the same as purchasability.** A `.dev`/`.io`/`.sh` name absent
  from RDAP and WHOIS can still be premium-priced or on a registry reserved list. That was not
  checked, and no domain was bought.

Finally, `.io` and `.sh` have no entry in the [IANA RDAP bootstrap](https://data.iana.org/rdap/dns.json),
so those results rest on WHOIS alone. WHOIS controls passed (`fly.io`, `esm.sh` both return
full records), so the channel is sound, but it is a single source rather than two.

---

## 7. Reproducing this

```sh
curl -s https://registry.npmjs.org/<name>                       # 404 = free
curl -s https://pypi.org/pypi/<name>/json                       # 404 = free
curl -s -H "User-Agent: <contact>" https://crates.io/api/v1/crates/<name>
curl -s https://formulae.brew.sh/api/formula/<name>.json        # and /cask/
gh api /users/<name>                                            # 404 = free
gh api "/search/repositories?q=<name>+in:name" --jq '.total_count'
curl -s https://proxy.golang.org/<module>/@v/list               # 404 = no module
curl -s https://pubapi.registry.google/rdap/domain/<name>.dev   # 404 = free (NOT rdap.org)
whois -h whois.nic.io <name>.io                                 # "Domain not found." = free
whois -h whois.nic.sh <name>.sh
curl -sL https://rdap.org/domain/<name>.com                     # 404 = free
```

Cited sources:
[registry.npmjs.org/lazysnap](https://registry.npmjs.org/lazysnap) ·
[jpdarago/lazysnap](https://github.com/jpdarago/lazysnap) ·
[its Go proxy versions](https://proxy.golang.org/github.com/jpdarago/lazysnap/@v/list) ·
[its README](https://raw.githubusercontent.com/jpdarago/lazysnap/main/README.md) ·
[AlthafPattan/lazysnap](https://github.com/AlthafPattan/lazysnap) ·
[github.com/LazySnap](https://github.com/LazySnap) ·
[RDAP lazysnap.com](https://rdap.org/domain/lazysnap.com) ·
[jwdev42/lazysnapshotter](https://github.com/jwdev42/lazysnapshotter) ·
[Lazy snapping (ACM DL)](https://dl.acm.org/doi/10.1145/1015706.1015719) ·
[Lazy Snapping PDF](https://home.cse.ust.hk/~cktang/sample_pub/lazy_snapping.pdf) ·
[Lazy Snapping (Microsoft Research)](https://www.microsoft.com/en-us/research/publication/lazy-snapping/) ·
[zjxeditor/LazySnapping](https://github.com/zjxeditor/LazySnapping) ·
[vyerneni/LazySnapping](https://github.com/vyerneni/LazySnapping) ·
[canonical/snapd](https://github.com/canonical/snapd) ·
[Snapcraft](https://snapcraft.io/) ·
[brew: snap](https://formulae.brew.sh/formula/snap) ·
[brew: snapcraft](https://formulae.brew.sh/formula/snapcraft) ·
[brew: tarsnap](https://formulae.brew.sh/formula/tarsnap) ·
[brew: rsnapshot](https://formulae.brew.sh/formula/rsnapshot) ·
[brew: snapraid](https://formulae.brew.sh/formula/snapraid) ·
[tarsnap.com](https://www.tarsnap.com/) ·
[DBSnapper](https://dbsnapper.com/) ·
[DBSnapper v2 subsetting](https://dbsnapper.com/blog/introducing-dbsnapper-v2) ·
[dbsnapper/dbsnapper](https://github.com/dbsnapper/dbsnapper) ·
[Snaplet is now open source (Supabase)](https://supabase.com/blog/snaplet-is-now-open-source) ·
[supabase-community/seed](https://github.com/supabase-community/seed) ·
[lazygit](https://github.com/jesseduffield/lazygit) ·
[lazydocker](https://github.com/jesseduffield/lazydocker) ·
[lazynpm](https://github.com/jesseduffield/lazynpm) ·
[Lazygit Turns 5](https://jesseduffield.com/Lazygit-5-Years-On/) ·
[jorgerojas26/lazysql](https://github.com/jorgerojas26/lazysql) ·
[brew: lazysql](https://formulae.brew.sh/formula/lazysql) ·
[brew: lazycut](https://formulae.brew.sh/formula/lazycut) ·
[brew: lazyrsync](https://formulae.brew.sh/formula/lazyrsync) ·
[brew: lazycontainer](https://formulae.brew.sh/formula/lazycontainer) ·
[Wisser/Jailer](https://github.com/Wisser/Jailer) ·
[Jailer site](https://wisser.github.io/Jailer/) ·
[TonicAI/condenser](https://github.com/TonicAI/condenser) ·
[Tonic Subset](https://tonic.ai/products/tonic-subset) ·
[nucleuscloud/neosync](https://github.com/nucleuscloud/neosync) ·
[PyPI lazyslice](https://pypi.org/project/lazyslice/) ·
[github.com/lazyslice](https://github.com/lazyslice) ·
[catalystneuro/lazyslice](https://github.com/catalystneuro/lazyslice) ·
[github.com/lazyseed](https://github.com/lazyseed) ·
[github.com/lazysnip](https://github.com/lazysnip) ·
[crates.io lazyprune](https://crates.io/api/v1/crates/lazyprune) ·
[spectral-cube LazyMask](https://spectral-cube.readthedocs.io/en/latest/api/spectral_cube.masks.LazyMask.html) ·
[ScoopInstaller/Scoop](https://github.com/ScoopInstaller/Scoop) ·
[scoop.sh](https://scoop.sh) ·
[USPTO trademark search](https://www.uspto.gov/trademarks/search) ·
[tmsearch.uspto.gov](https://tmsearch.uspto.gov/) ·
[IANA RDAP bootstrap](https://data.iana.org/rdap/dns.json) ·
[HN: lazysnap](https://hn.algolia.com/?query=lazysnap) ·
[HN: database subsetting](https://hn.algolia.com/?query=database%20subsetting&type=story) ·
[HN: snaplet](https://hn.algolia.com/?query=snaplet) ·
[HN: dbsnapper](https://hn.algolia.com/?query=dbsnapper)
