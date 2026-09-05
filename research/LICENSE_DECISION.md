# Licence decision

**Question.** What licence for the lazysnap CLI, given that the CLI is the whole v1 product and the eventual paid product is a hosted service? And one repo or two?

**Shape of the thing being licensed.** A single static binary a developer runs on a laptop or in CI against their own database ([CONCEPT.md](../CONCEPT.md)). It is not a server, not a daemon, not something a customer's users reach over a network. That shape decides most of what follows.

## The four candidates

| | Apache-2.0 | MIT | AGPL-3.0 | BSL 1.1 |
|---|---|---|---|---|
| OSI open source | yes | yes | yes | **no** ([SPDX BUSL-1.1](https://spdx.org/licenses/BUSL-1.1.html); [FOSSA](https://fossa.com/blog/business-source-license-requirements-provisions-history/)) |
| Explicit patent grant | yes ([§3](https://www.apache.org/licenses/LICENSE-2.0)) | no (ambiguous) | yes | inherits from Change Licence only |
| Copyleft on hosted use | no | no | only on **modified** versions ([§13](https://www.gnu.org/licenses/agpl-3.0.en.html)) | n/a — bans production use outright |
| In homebrew-core | yes | yes | yes | **no** — core formulae must be DFSG-compatible ([Homebrew](https://docs.brew.sh/Acceptable-Formulae)) |
| Blanket-banned at large employers | no | no | yes, at Google ([policy](https://opensource.google/documentation/reference/using/agpl-policy)) | fails any policy that requires an OSI licence; "blanket ban" unverified |

### Contributor comfort

Apache-2.0 and MIT are the default expectation for a Go/Rust CLI; neither asks a drive-by contributor to think. AGPL-3.0 is fine for ideologically-motivated contributors but silently excludes anyone whose employer bans it — Google staff may not even install AGPL software on a work laptop ([Google AGPL policy](https://opensource.google/documentation/reference/using/agpl-policy)). BSL is the one that actually costs contributions: VictoriaMetrics, who sell a hosted product, argue that "switching an open source license to the BSL erodes trust in your product and your company" and that developers refuse to contribute to projects carrying that reputation ([VictoriaMetrics](https://victoriametrics.com/blog/bsl-is-short-term-fix-why-we-choose-open-source/)). The Terraform precedent is the empirical version: HashiCorp moved MPL-2.0 → BSL on 10 August 2023 and a manifesto formed around it five days later, followed by a fork accepted into the Linux Foundation as OpenTofu in September 2023 ([OpenTofu manifesto](https://opentofu.org/manifesto/), [fork announcement](https://opentofu.org/blog/opentofu-announces-fork-of-terraform/)).

### Corporate adoption friction

This is the deciding axis, because lazysnap's adoption path is one developer installing it, then their team, then a compliance conversation. Apache-2.0 is the lowest-friction option in regulated environments precisely because it makes the patent grant explicit where MIT leaves it ambiguous, and adds a trademark clause and a change-notice requirement ([Apache-2.0 §3, §6](https://www.apache.org/licenses/LICENSE-2.0); [FOSSA](https://fossa.com/blog/open-source-licenses-101-apache-license-2-0/)). AGPL is where legal review stalls: the ban is not usually about the licence being unreasonable but about review cost and derivative-work ambiguity at scale ([analysis](https://thebuild.com/blog/the-agpl-radioactive-by-design/)). BSL fails earlier and harder — it is not open source, so it cannot ship in homebrew-core ([Homebrew](https://docs.brew.sh/Acceptable-Formulae)), which directly contradicts the "one static binary, one-line install" principle in CONCEPT.md.

### Protection against a cloud vendor hosting the tool

Worth being honest about how weak this threat is for *this* product. The strip-mining fight was about databases and search engines — MongoDB, Elasticsearch, MariaDB, Redis — where the open source project *is* the hosted service ([TechTarget](https://www.techtarget.com/searchdatamanagement/news/252456939/Open-source-cloud-databases-battle-strip-mining-by-AWS)). A subsetting/masking CLI is not that: the moat of the eventual hosted product is scheduling, retention, sharing and audit, none of which is in the CLI.

- **Apache-2.0 / MIT:** no protection. A vendor may host it. Practically irrelevant until the tool is large enough to be worth hosting, at which point our own hosted layer is already built.
- **AGPL-3.0:** far less protection than it looks. §13 triggers on *modification* plus remote network interaction — "If you run stock upstream AGPL software as part of your service, Section 13 asks nothing of you" ([Pettus](https://thebuild.com/blog/the-agpl-radioactive-by-design/), consistent with the [licence text](https://www.gnu.org/licenses/agpl-3.0.en.html)). A vendor shelling out to an unmodified lazysnap binary from their own control plane triggers nothing. We would pay the full corporate-ban cost for close to zero defence.
- **BSL 1.1:** real protection — non-production use only, unless we write an Additional Use Grant ([SPDX](https://spdx.org/licenses/BUSL-1.1.html)). But the Change Licence is mandated to be "GPL Version 2.0 or any later version, or a license that is compatible with GPL Version 2.0", and the Change Date is capped at four years, so every release eventually becomes *copyleft*, not permissive — the opposite of where we want to end up. Sentry built FSL to avoid exactly the per-vendor ambiguity BSL's Additional Use Grant creates ([FSL](https://fsl.software/), [Sentry](https://blog.sentry.io/introducing-the-functional-source-license-freedom-without-free-riding/)); FSL converts to Apache-2.0 or MIT after two years and is the better choice *if* we ever conclude we need a source-available core.

## What comparable projects chose (verified September 2026)

| Project | Licence | Business model | Notes |
|---|---|---|---|
| [Greenmask](https://github.com/GreenmaskIO/greenmask) | **Apache-2.0** ([LICENSE](https://raw.githubusercontent.com/GreenmaskIO/greenmask/main/LICENSE)) | Greenmask, Inc., founded 2023; "We release our products under open-source licenses" ([about](https://www.greenmask.io/about)) | Closest analogue: Postgres dump + anonymise. All org repos Apache-2.0; no public pricing page (greenmask.io/pricing returns 404 as of 2026-09-04), so no verified paid product. 1,757 stars. |
| [Neosync](https://github.com/nucleuscloud/neosync) | **MIT Expat, with a reserved `ee/` carve-out** ([LICENSE.md](https://raw.githubusercontent.com/nucleuscloud/neosync/main/LICENSE.md)) | Neosync Cloud, hosted + "run our managed version and keep all of their data on their infra while we host the control plane" ([Show HN](https://news.ycombinator.com/item?id=40443927)) | The carve-out reads "All content that resides under any `ee/` directory of this repository, **if such directories exists**" — and no `ee/` directory ever existed (GitHub contents API returns 404). The option was reserved, never exercised. Repo is now archived (last push 2025-08-30) after the Grow Therapy acquisition ([Crunchbase](https://www.crunchbase.com/acquisition/grow-therapy-acquires-neosync-cd81--00632527)); 4,141 stars. |
| [Snaplet](https://supabase.com/blog/snaplet-is-now-open-source) | **MIT** | Was a proprietary hosted snapshot service; shut down in 2024 (hosted cutoff reported as 31 Aug 2024) and open-sourced on the way out | `copycat`, `seed` and `snapshot` all MIT, now under `supabase-community` (verified via GitHub API; `snapshot` archived). Some of the team joined Supabase, who took over maintenance. The closed hosted product died and the permissive core outlived it. |
| [lazygit](https://github.com/jesseduffield/lazygit) | **MIT** ([LICENSE](https://raw.githubusercontent.com/jesseduffield/lazygit/master/LICENSE)) | None — GitHub Sponsors only ("not my fulltime job but it is a hefty part time job") | 81,998 stars. The naming ancestor. Evidence that MIT + terminal-first is the ecosystem default. |
| [sqlit](https://github.com/Maxteabag/sqlit) | **MIT** | None visible | "lazygit of SQL databases", 4,801 stars, created Dec 2025. Same default again. |

Nobody in this space picked AGPL or BSL. The two that had hosted businesses (Neosync, Snaplet) kept the core permissive and monetised the service.

## A future closed hosted layer: same repo or separate

**Same repo, permissive root + proprietary subdirectory** is the GitLab pattern: a root `LICENSE` that says content under `ee/` is governed by `ee/LICENSE`, and an EE licence that requires a paid subscription for production use ([GitLab root LICENSE](https://gitlab.com/gitlab-org/gitlab/-/raw/master/LICENSE), [ee/LICENSE](https://gitlab.com/gitlab-org/gitlab/-/raw/master/ee/LICENSE)); Neosync copied the wording verbatim. It works but it costs:

- **Licence detection breaks.** GitHub classifies Neosync as `NOASSERTION` (verified via API) because Licensee cannot match the modified file; GitHub's own advice is to "simplify your *LICENSE* file and note the complexity somewhere else" ([GitHub docs](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/licensing-a-repository)). A scanner-driven procurement review then flags us as unlicensed — the exact audience we need to pass.
- **Every contributor must now reason about which directory they are in**, and a good-faith PR into the wrong directory is a legal problem rather than a review comment.
- It is also premature: a hosted service is an explicit v1 non-goal, and there is no hosted code to place.

**Separate repos** costs a shared-code decision (extract the reusable parts as an Apache-2.0 Go module the private service imports) and nothing else. It keeps the public repo one licence, one `LICENSE` file, correctly detected.

Interaction by licence, if the hosted layer ever lands:

- **Apache-2.0 / MIT core:** trivial in either layout. We can link the CLI into a closed service with no obligation.
- **AGPL core:** our *own* hosted service becomes the problem, not a competitor's. To keep the service closed we would need to hold all rights in the CLI, which means a CLA on every contributor — see below.
- **BSL core:** same repo is workable, but distribution channels are already closed off and the community core is gone.

## CLA or DCO

A CLA's real function is retained relicensing power: MongoDB in 2019 and Elasticsearch in January 2021 both used rights granted by their CLAs to move to non-open-source licences ([Wikipedia](https://en.wikipedia.org/wiki/Contributor_license_agreement)). Both later walked it back — Elastic added AGPL in August 2024 ([Elastic](https://www.elastic.co/blog/elasticsearch-is-open-source-again)) and Redis added AGPLv3 in May 2025 after its own SSPL move ([Redis](https://redis.io/blog/agplv3/)) — which is the clearest available evidence that the licence-tightening play does not pay for itself. The DCO explicitly does not do that — it "assumes license terms appear in every file" and provides no relicensing mechanism ([Mitchell](https://writing.kemitchell.com/2021/07/02/DCO-Not-CLA)); it is only a signed attestation, clauses (a)–(d), that the contributor had the right to submit ([DCO 1.1](https://developercertificate.org/)). The cost of a CLA is measured in the first contribution: a typo fix requires a signing workflow, and a corporate CLA drags in the contributor's own legal and HR ([opensource.com](https://opensource.com/article/19/2/cla-problems)). The CNCF's IP policy accepts either but encourages DCO, and most CNCF projects use it ([CLA/DCO audit issue](https://github.com/cncf/foundation/issues/130)).

We do not want relicensing power. Wanting it is what produces the Terraform outcome. A DCO with `git commit -s` and the GitHub DCO check is the honest signal that the licence is not going to change.

## Recommendation

1. **Licence the CLI Apache-2.0.** Permissive so it spreads without a legal conversation; Apache rather than MIT because the explicit patent grant and trademark clause are what a compliance reviewer at a thirty-person company's customer actually asks for, and because Greenmask — the nearest comparable — is already there. Put `Apache-2.0` in `LICENSE` at the repo root, unmodified, and add a `NOTICE` file.
2. **One public repo, one licence, no `ee/` directory.** When the hosted layer arrives it goes in a separate private repo that imports the CLI as a Go module. Revisit only if we ship a paid *local* feature, and even then prefer a separate repo over a carve-out.
3. **DCO, not a CLA.** Enable the DCO check on PRs; state in `CONTRIBUTING.md` that the project will not be relicensed.
4. **Protect the name, not the code.** Register and publish a trademark policy for "lazysnap" — the standard defence for a permissively-licensed project, as Rust and Grafana do ([Rust](https://rustfoundation.org/policy/rust-trademark-policy/), [Grafana](https://grafana.com/trademark-policy/)), and the one Apache-2.0 §6 already gestures at. A cloud vendor may host the code; they may not call their product lazysnap.
5. **Escape hatch, documented now so it is a decision rather than a panic.** If a vendor ever does host lazysnap at scale, the response is FSL 1.1 (converting to Apache-2.0 after two years) on *new* hosted-adjacent components in a separate repo — never a relicence of the shipped CLI. Without a CLA we could not relicence it anyway, which is the point.

## Unverified / gaps

- Greenmask's revenue model. The company exists and describes an enterprise-oriented roadmap, but no pricing or commercial page is published (`/pricing` 404s), so "Apache-2.0 plus paid services" is inference, not a sourced claim.
- Whether Snaplet's hosted snapshot service shared a repo with the open source pieces before the shutdown — the shutdown post at snaplet.dev was unreachable from this environment; the Supabase post is the working source.
- Reddit was not reachable from this environment, so developer sentiment is drawn from Hacker News, blogs and vendor posts only.
