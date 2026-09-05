# Naming: is `lazysnap` the right name?

**Recommendation: change the name to `lazyslice`.**

`lazysnap` is not legally blocked, but it is already occupied by a Go TUI of the same
name in the same "lazy" family, its word stem is saturated in exactly the neighbourhoods
this tool lives in (Linux packaging, filesystem backups, and a live commercial competitor
called DBSnapper), and its dominant search meaning is a 2004 computer-vision algorithm.
Worse, "snap" tells a user the tool makes a *copy of the whole database*, which is the
opposite of the product's central claim. `lazyslice` says the true thing, is one character
shorter, and has `.dev`, `.sh` and `.io` all free.

All availability data below was collected on **2026-09-04/05** by direct query to each
registry's API, `whois.nic.io` / `whois.nic.sh`, and RDAP. Methodology was validated
against known-registered controls (`esm.sh`, `fly.io`, `web.dev`, npm `lazygit`, PyPI
`requests`, crates `serde`, brew `lazygit`) — all returned "registered/taken" correctly.

---

## 1. `lazysnap` collision audit

| Namespace | Status | Detail |
|---|---|---|
| npm | **TAKEN** | [`lazysnap` v0.0.1](https://registry.npmjs.org/lazysnap) — "Monorepo for `@lazysnap/*` — image loading with observability, retry, and blur-up". Claims the `@lazysnap` scope too. |
| PyPI | free | `https://pypi.org/pypi/lazysnap/json` → 404 |
| crates.io | free | `https://crates.io/api/v1/crates/lazysnap` → "crate `lazysnap` does not exist" |
| Homebrew formula | free | [`formulae.brew.sh/api/formula/lazysnap.json`](https://formulae.brew.sh/api/formula/lazysnap.json) → 404 |
| Homebrew cask | free | `formulae.brew.sh/api/cask/lazysnap.json` → 404 |
| GitHub user/org | **TAKEN** | [github.com/LazySnap](https://github.com/LazySnap) — user account, 0 public repos, created 2024-04-13 |
| GitHub repos | **14 matches** | see below |
| Go module proxy | **TAKEN** | [`github.com/jpdarago/lazysnap`](https://proxy.golang.org/github.com/jpdarago/lazysnap/@v/list) — v0.1.0, v0.2.0; v0.2.0 tagged 2026-06-29 |
| `lazysnap.dev` | free | RDAP → 404 |
| `lazysnap.io` | free | `whois.nic.io` → "Domain not found." |
| `lazysnap.sh` | free | `whois.nic.sh` → "Domain not found." |
| `lazysnap.com` | **REGISTERED** | [RDAP](https://rdap.org/domain/lazysnap.com) → handle `2622607671_DOMAIN_COM-VRSN`, Namecheap, Cloudflare nameservers |
| USPTO | **unverified** | see §5 |

### The GitHub picture

Fourteen repositories match the name. They split into three groups, and none of them is us.

**a) A Go TUI already called lazysnap, in this exact family.**
[`jpdarago/lazysnap`](https://github.com/jpdarago/lazysnap) (Go, pushed 2026-06-29) describes
itself in its [README](https://raw.githubusercontent.com/jpdarago/lazysnap/main/README.md) as
"A terminal UI for [tarsnap](https://www.tarsnap.com/), inspired by
[lazygit](https://github.com/jesseduffield/lazygit) and
[lazydocker](https://github.com/jesseduffield/lazydocker)."

Same name, same language, same TUI-over-a-CLI premise, same stated lineage, and it is already
published in the Go module proxy at v0.2.0. It has 0 stars, so it is not famous — but it is
live, it owns the `go install github.com/jpdarago/lazysnap` path, and any post announcing
"lazysnap, a lazy-family TUI written in Go" now has to explain which lazysnap it is.

**b) The npm/TypeScript package.**
[`AlthafPattan/lazysnap`](https://github.com/AlthafPattan/lazysnap) — "Production-grade image
loading for React and Angular." This is the npm registration above.

**c) Seven implementations of the "Lazy Snapping" computer-vision algorithm.**
[`zjxeditor/LazySnapping`](https://github.com/zjxeditor/LazySnapping) (10★),
`vyerneni/LazySnapping`, `wuyongxiang/LazySnapping-Android`, `liaoxl/LazySnappingWithGMM`,
`matinJ/lazysnapping`, `namthse03439/LazySnapping`, `MaxtirError/LazySnaping`,
`StefanoFochesatto/LazySnappingGraphFlow`.

That group exists because **Lazy Snapping is a well-known SIGGRAPH 2004 paper** — an
interactive image cutout method by Li, Sun, Tang and Shum
([Microsoft Research](https://www.microsoft.com/en-us/research/publication/lazy-snapping/),
[paper PDF](https://home.cse.ust.hk/~cktang/sample_pub/lazy_snapping.pdf)). Half of the name's
existing search surface is image segmentation, permanently. There is nothing to be done about
a twenty-two-year-old citation graph.

Hacker News, by contrast, has **zero** stories or comments matching "lazysnap"
([hn.algolia.com](https://hn.algolia.com/api/v1/search?query=lazysnap) → `nbHits: 0`), so there
is no goodwill attached to the name to lose either.

---

## 2. The bigger problem is "snap", not the collisions

The collision list is survivable. The semantics are the real objection, and there are three
layers to it.

### "snap" already means package manager to a terminal user

Canonical's [snap/snapd/Snapcraft](https://snapcraft.io/) is the Linux packaging format whose
primary verb is literally `snap install`
([canonical/snapd](https://github.com/canonical/snapd)). Homebrew ships a formula named
[`snap`](https://formulae.brew.sh/formula/snap) ("Tool to work with .snap files") and
[`snapcraft`](https://formulae.brew.sh/formula/snapcraft). A CLI tool named `lazysnap`
distributed via Homebrew is one prefix away from the thing that installs software.

### "snap" already means filesystem backup to a terminal user

Homebrew also carries [`tarsnap`](https://formulae.brew.sh/formula/tarsnap), `rsnapshot`,
and `snapraid`. On GitHub, [`jwdev42/lazysnapshotter`](https://github.com/jwdev42/lazysnapshotter)
is a btrfs backup frontend. Combined with `jpdarago/lazysnap` being a tarsnap TUI, the
established reading of "lazy" + "snap" in a terminal context is *backups*, not *databases*.

### "snap" is taken in this exact product category — twice

- **DBSnapper** is a live commercial product that "makes it easy to snapshot, subset,
  sanitize and share your database" for PostgreSQL and MySQL
  ([dbsnapper.com](https://dbsnapper.com/),
  [github.com/dbsnapper/dbsnapper](https://github.com/dbsnapper/dbsnapper)), and
  [v2.0 shipped database subsetting](https://dbsnapper.com/blog/introducing-dbsnapper-v2)
  producing "smaller, relationally complete, snapshots of their production databases."
  That is CONCEPT.md's one-liner, almost word for word, from a product that already owns
  "snap" in the space. `lazysnap` reads as an unofficial lazy-TUI wrapper for DBSnapper.
- **Snaplet** ran "Snaplet Snapshot" for database snapshots and anonymisation, then wound
  down; the hosted service closed and the tooling was open-sourced under Supabase
  ([Supabase: "Snaplet is now open source"](https://supabase.com/blog/snaplet-is-now-open-source),
  [supabase-community/seed](https://github.com/supabase-community/seed)). "Snap" in the
  database-anonymisation category now carries a dead product's ghost.

### And "snap" describes the wrong thing

CONCEPT.md's promise is "a **small**, referentially complete, anonymised copy" and its
non-goals include full-database work. A snapshot means the whole thing, at a point in time.
The product's differentiator is that it is *not* the whole thing. The name argues against
the pitch on first contact — which is exactly the sort of confusion the CLAUDE.md rule
"documentation is never the fix for a confusing first run" says to fix at the source.

Note that this is a naming objection, not a family objection. The "lazy" prefix is a genuine,
healthy convention — [lazygit](https://github.com/jesseduffield/lazygit),
[lazydocker](https://github.com/jesseduffield/lazydocker),
[lazynpm](https://github.com/jesseduffield/lazynpm) from Jesse Duffield
([five-year retrospective](https://jesseduffield.com/Lazygit-5-Years-On/)), and Homebrew now
carries thirteen `lazy*` formulae including [`lazysql`](https://formulae.brew.sh/formula/lazysql)
([4,268★](https://github.com/jorgerojas26/lazysql), a cross-platform TUI database tool),
[`lazyrsync`](https://formulae.brew.sh/formula/lazyrsync),
[`lazycut`](https://formulae.brew.sh/formula/lazycut), `lazyjj`, `lazyssh`, `lazyjournal`,
`lazymake`, `lazycontainer`, `lazydocker`, `lazygit`, `lazy-tmux`. Keep the prefix. Change the stem.

---

## 3. Five alternatives

Constraints applied: keeps the "lazy" family, pronounceable, short to type, and at least one
of `.dev` / `.sh` / `.io` free.

| Name | Chars | npm | PyPI | crates | brew | GH org | GH repos | .dev | .sh | .io | .com |
|---|---|---|---|---|---|---|---|---|---|---|---|
| **lazyslice** | 9 | free | **taken** | free | free | **taken** | 2 | **free** | **free** | **free** | taken |
| **lazysubset** | 10 | free | free | free | free | free | **0** | **free** | **free** | **free** | free |
| **lazyscoop** | 9 | free | free | free | free | free | **0** | **free** | **free** | **free** | free |
| **lazyseed** | 8 | free | free | free | free | **taken** | **0** | **free** | **free** | **free** | taken |
| **lazymask** | 8 | free | free | free | free | free | **0** | **free** | **free** | **free** | taken |

*(`lazysnap` for comparison: npm **taken**, GH org **taken**, GH repos **14**, Go proxy **taken**, `.com` **taken**.)*

### lazyslice — *the recommendation*

`$ lazyslice`

The word names the mechanism. It is also the word this category already uses for the output:
Jailer "creates small slices from your database (consistent and referentially intact)"
([Wisser/Jailer](https://github.com/Wisser/Jailer), [project site](https://wisser.github.io/Jailer/)),
and Tonic describes reducing "full production databases to targeted slices"
([Tonic Subset](https://tonic.ai/products/tonic-subset)). CONCEPT.md's own hero transcript
already says `planning… 23 tables in slice, ~18k rows` — the team is thinking in slices before
anyone told it to.

Collisions, all weak:
- [PyPI `lazyslice` 0.3.0](https://pypi.org/project/lazyslice/) — "Lazy slicing and transpose
  operations for h5py and zarr", scientific Python. Different ecosystem; CONCEPT.md ships
  "one static binary, one-line install", so PyPI is not a distribution channel for this tool.
- [github.com/lazyslice](https://github.com/lazyslice) — organization, one Haskell repo, last
  pushed 2020-08-31. Dormant for six years but it holds the org name.
- [catalystneuro/lazyslice](https://github.com/catalystneuro/lazyslice) — 3★ Python, same
  h5py/zarr project. Negligible search noise.

Not owning `github.com/lazyslice` costs little: lazygit lives at `jesseduffield/lazygit` and
lazysql at `jorgerojas26/lazysql`. The family convention has never required a matching org.
`lazyslice.dev` is free and can be the canonical home.

### lazysubset

`$ lazysubset` — **the only candidate with a completely clean namespace.** Zero hits on npm,
PyPI, crates.io, Homebrew, GitHub repo search, and the GitHub user/org namespace; `.dev`,
`.sh`, `.io` *and* `.com` all free.

It is also the literal term of art: "database subsetting" is what
[Jailer](https://github.com/Wisser/Jailer), [Tonic's Condenser](https://github.com/TonicAI/condenser),
[Tonic Subset](https://tonic.ai/products/tonic-subset) and
[Neosync](https://github.com/nucleuscloud/neosync) all call this, and Hacker News has
[44 stories](https://hn.algolia.com/api/v1/search?query=database%20subsetting&tags=story) using
the phrase, led by
["Jailer: A tool for database subsetting"](https://wisser.github.io/Jailer/) at 128 points.

Costs: ten characters, and "subset" reads slightly academic next to `lazygit`. Ten is not
disqualifying — `lazydocker` is also ten — but this tool's pitch is a bare one-word command.

### lazyscoop

`$ lazyscoop` — clean everywhere, all four domains free, and it carries the family's playful
register ("scoop a cup out of prod"). Risks: [scoop.sh](https://scoop.sh) is the Windows
package manager, so the bare word already means something in dev tooling, and
`lazyscoop.wordpress.com` is an unrelated blog. Both are weak as compounds, but "scoop"
describes the gesture rather than the guarantee — it says nothing about referential
completeness or masking.

### lazyseed

`$ lazyseed` — shortest of the five, clean on every package registry. Its virtue is that it
names the *job*: the user's outcome is a seeded local database.

Two problems. [github.com/lazyseed](https://github.com/lazyseed) is an organization created
2026-05-08 with 0 repos — dormant, but it holds the name. More importantly "seed" now means
*generated* data in this exact category: `@snaplet/seed` and its successor
[supabase-community/seed](https://github.com/supabase-community/seed) are schema-driven
generators. CONCEPT.md lists "synthetic data generation from a schema alone" as an explicit
non-goal, so this name promises the one thing the product refuses to do.

### lazymask

`$ lazymask` — short, clean on every registry, `.dev`/`.sh`/`.io` free. Honest about the
safety half of the product. But masking is step 4 of 5; the name omits subsetting, which is
the harder engineering problem and the actual differentiator. Minor noise:
[`LazyMask` is a class in spectral-cube](https://spectral-cube.readthedocs.io/en/latest/api/spectral_cube.masks.LazyMask.html)
(astronomy) and "LazyMask" is a decoding method in [arXiv:2505.17938](https://arxiv.org/abs/2505.17938).

---

## 4. Recommendation and reasoning

**Rename to `lazyslice`. Take `lazyslice.dev` as the canonical domain; `lazyslice.sh` and
`lazyslice.io` are also free if a hedge is wanted.** (No domain has been purchased.)

The reasoning, in the order it matters:

1. **It fixes the semantic error.** `snap` says "copy of the whole database"; `slice` says
   "part of one, cut cleanly". The product's entire claim is the second thing. A name that
   argues with the pitch costs an explanation in every README, every talk, and every first run.
2. **It escapes an occupied namespace.** `lazysnap` collides with a live Go TUI of the same
   name in the same family, an npm package plus scope, a taken GitHub account, a registered
   `.com`, and a 2004 CV algorithm that owns half the search results. `lazyslice` collides
   with a scientific Python package we will never ship against and a repo dormant since 2020.
3. **It avoids a live competitor's brand.** [DBSnapper](https://dbsnapper.com/) sells
   snapshot + subset + sanitise for Postgres today. Shipping `lazysnap` into that market
   invites the reading that we are its TUI.
4. **It is already the team's own vocabulary.** CONCEPT.md says "23 tables in slice" without
   prompting.
5. **It types well.** Nine characters, two syllables, no ambiguous letters, and it sits
   naturally beside `lazygit` and `lazysql` in a Homebrew listing.

**If a zero-collision namespace is judged more important than the shorter, more evocative
name, take `lazysubset` instead.** It is the only candidate clean on every registry *and*
every domain, and it is the category's own term of art. The trade is one extra character and
a slightly drier word. Both choices are defensible; `lazyslice` is the better name, and
`lazysubset` is the safer one.

**If the name stays `lazysnap`**, do these two things: pick a GitHub owner path that is not
`lazysnap/*` (the account is taken), and never publish to npm under that name. Neither of
those repairs the semantic problem or the DBSnapper adjacency.

---

## 5. What could not be verified

**USPTO trademark search: unverified.** TESS was retired on 2023-11-30 and replaced by
Trademark Search at [tmsearch.uspto.gov](https://tmsearch.uspto.gov/)
([USPTO announcement](https://www.uspto.gov/subscription-center/2023/retiring-tess-what-know-about-new-trademark-search-system)),
so the TESS search named in the task no longer exists. The replacement is a JavaScript SPA
fronted by an AWS WAF challenge (`a434627cf98f.edge.sdk.awswaf.com/.../challenge.js` is
injected into the page source), and it has no documented public API. Every access route
available here failed:

- `POST tmsearch.uspto.gov/api-v1-0-0/tmsearch` → 405 from S3
- `api.uspto.gov/api/v1/trademark/search` → 403 (requires an ODP API key)
- `tsdrapi.uspto.gov` → 401 (requires an API key)
- `trademarks.justia.com`, `trademarkia.com`, `uspto.report` → 403 to both WebFetch and curl
  with a browser user agent
- browser navigation to `tmsearch.uspto.gov` → denied in this environment

A targeted web search for a `lazysnap` trademark returned no record of one, but absence from
web search is not a clearance opinion. **No name in this document has been cleared for
trademark.** Someone with browser access should run the shortlist through
[tmsearch.uspto.gov](https://tmsearch.uspto.gov/) before the name is committed to a binary,
a domain purchase, or a public announcement. Relevant classes are 9 (software) and 42
(SaaS/software services).

Two secondary gaps:

- **Common-law and non-US marks** (EUIPO/TMview, UKIPO, CIPO) were not checked; TMview's
  API was unreachable from here.
- **`lazyslice` as a small business name** was searched and nothing relevant surfaced — the
  results were unrelated pizza businesses (Lazy Moon, Freshslice, Miami Slice). Any such mark
  would sit in class 43 (restaurant services) and would not bar software use, but it was not
  confirmed either way.

---

## 6. Sources

Registry and DNS results in §1 and §3 were obtained by direct query and are reproducible:

```
curl -s https://registry.npmjs.org/<name>
curl -s https://pypi.org/pypi/<name>/json
curl -s -H "User-Agent: <contact>" https://crates.io/api/v1/crates/<name>
curl -s https://formulae.brew.sh/api/formula/<name>.json
gh api /users/<name>
gh api "/search/repositories?q=<name>+in:name"
curl -s https://proxy.golang.org/<module>/@v/list
curl -sL https://rdap.org/domain/<name>.dev      # 404 body = available
whois -h whois.nic.io <name>.io                   # "Domain not found." = available
whois -h whois.nic.sh <name>.sh                   # "Domain not found." = available
```

Cited pages:
[registry.npmjs.org/lazysnap](https://registry.npmjs.org/lazysnap) ·
[jpdarago/lazysnap](https://github.com/jpdarago/lazysnap) ·
[its Go proxy versions](https://proxy.golang.org/github.com/jpdarago/lazysnap/@v/list) ·
[its README](https://raw.githubusercontent.com/jpdarago/lazysnap/main/README.md) ·
[AlthafPattan/lazysnap](https://github.com/AlthafPattan/lazysnap) ·
[github.com/LazySnap](https://github.com/LazySnap) ·
[RDAP lazysnap.com](https://rdap.org/domain/lazysnap.com) ·
[Lazy Snapping (Microsoft Research)](https://www.microsoft.com/en-us/research/publication/lazy-snapping/) ·
[Lazy Snapping PDF](https://home.cse.ust.hk/~cktang/sample_pub/lazy_snapping.pdf) ·
[zjxeditor/LazySnapping](https://github.com/zjxeditor/LazySnapping) ·
[jwdev42/lazysnapshotter](https://github.com/jwdev42/lazysnapshotter) ·
[Snapcraft](https://snapcraft.io/) ·
[canonical/snapd](https://github.com/canonical/snapd) ·
[brew: snap](https://formulae.brew.sh/formula/snap) ·
[brew: snapcraft](https://formulae.brew.sh/formula/snapcraft) ·
[brew: tarsnap](https://formulae.brew.sh/formula/tarsnap) ·
[tarsnap.com](https://www.tarsnap.com/) ·
[DBSnapper](https://dbsnapper.com/) ·
[DBSnapper 2.0 subsetting](https://dbsnapper.com/blog/introducing-dbsnapper-v2) ·
[dbsnapper/dbsnapper](https://github.com/dbsnapper/dbsnapper) ·
[Snaplet is now open source (Supabase)](https://supabase.com/blog/snaplet-is-now-open-source) ·
[supabase-community/seed](https://github.com/supabase-community/seed) ·
[HN: snaplet](https://hn.algolia.com/api/v1/search?query=snaplet) ·
[HN: lazysnap (0 hits)](https://hn.algolia.com/api/v1/search?query=lazysnap) ·
[HN: database subsetting](https://hn.algolia.com/api/v1/search?query=database%20subsetting&tags=story) ·
[lazygit](https://github.com/jesseduffield/lazygit) ·
[lazydocker](https://github.com/jesseduffield/lazydocker) ·
[lazynpm](https://github.com/jesseduffield/lazynpm) ·
[Lazygit Turns 5](https://jesseduffield.com/Lazygit-5-Years-On/) ·
[jorgerojas26/lazysql](https://github.com/jorgerojas26/lazysql) ·
[brew: lazysql](https://formulae.brew.sh/formula/lazysql) ·
[brew: lazycut](https://formulae.brew.sh/formula/lazycut) ·
[brew: lazyrsync](https://formulae.brew.sh/formula/lazyrsync) ·
[Wisser/Jailer](https://github.com/Wisser/Jailer) ·
[Jailer site](https://wisser.github.io/Jailer/) ·
[TonicAI/condenser](https://github.com/TonicAI/condenser) ·
[Tonic Subset](https://tonic.ai/products/tonic-subset) ·
[nucleuscloud/neosync](https://github.com/nucleuscloud/neosync) ·
[PyPI lazyslice](https://pypi.org/project/lazyslice/) ·
[github.com/lazyslice](https://github.com/lazyslice) ·
[catalystneuro/lazyslice](https://github.com/catalystneuro/lazyslice) ·
[spectral-cube LazyMask](https://spectral-cube.readthedocs.io/en/latest/api/spectral_cube.masks.LazyMask.html) ·
[arXiv:2505.17938 (LazyMask decoding)](https://arxiv.org/abs/2505.17938) ·
[scoop.sh](https://scoop.sh) ·
[USPTO: Retiring TESS](https://www.uspto.gov/subscription-center/2023/retiring-tess-what-know-about-new-trademark-search-system) ·
[tmsearch.uspto.gov](https://tmsearch.uspto.gov/)
