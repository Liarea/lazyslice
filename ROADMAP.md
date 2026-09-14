# Roadmap

Source of truth for phase sequencing: docs/BUILD_PLAN.md, phases 4 to 8. Where an architectural fact below (stage names and counts, flags, ADR numbers) conflicts with ARCHITECTURE.md or an ADR, the ADR wins — docs/BUILD_PLAN.md predates ADRs 001-008 and is not corrected when they supersede it. This file tracks where we are and what is explicitly not now.

**Rule:** any feature request goes into the Later section at the bottom with a one-line reason, into "Later, unscheduled" unless it is a permanent refusal, in which case it goes into "Refused". Nothing moves out of "Later, unscheduled" until the current phase's gate below is fully ticked. Nothing moves out of "Refused" by a gate tick at all — see that section for what it takes.

## Current phase: 5, Hardening

**Status 2026-09-14 (resumed):** the harden step is re-sequenced around an independent adversarial review of commit 936ceec, docs/reviews/2026-09-09/REVIEW.md, which reproduced six leak-class or contract defects with disposable containers (target ownership lapses between gate and drop; a polymorphic warning prints a source value; the unique escalation breaks an FK-connected pair; `complete` is written before verify; DDL defaults carry source literals; one strong hit in a sparse column passes the second net) and traced two more (TLS parameters lost on the config round trip; `mapping_file` accepted but unimplemented). Those are tracker T-0130 to T-0141 in E5 and run ahead of failure UX and performance: T-0129, T-0127 (arrays), T-0130 (target lease and lock-and-recheck), T-0133 (lifecycle owned by core), T-0131 (value-free events and a canary test), T-0132 (masker per FK equality group), T-0134 (DDL literals), T-0136 (strong single hit), T-0137 (JSON keys), T-0135 (transport parameters), T-0138 (mapping_file refused, ADR-012), T-0139 (torture baseline), T-0119 (table-scoped rule), T-0140 (CI torture and release gate), T-FAILUX, T-PERF, T-0141 (README and SECURITY drift), then the red team, which re-runs the review's probes before its own attacks. Tiering: Sonnet developer at medium effort and one Opus reviewer at medium effort by default; Opus developer only for T-0129, T-0130, T-0132 and T-0134. Resume: `hardening.js {step: 'harden', from: <index of the first unmerged task>}`, then `{step: 'redteam'}`. Before the previous pause the harden step had merged seven tasks: type registration for COPY, the torture suite, the unique credential masker with the post-plan fingerprint, the torture re-measure, composite fail-closed with the array-literal splitter, the mechanical trio, and transform's element-wise array masking as a partial landing (T-0118). GitHub Actions has not started a job since 2026-09-09 04:23 UTC (every job ends in three seconds with no steps, the signature of a billing or spending-limit block on the account); that is Gareth's to clear and gate 5 cannot close on local runs alone.

Phase 4 closed 2026-09-08. Evidence: all eleven packages merged; `make integration` green locally and in CI (run 34175303077) with every invariant I1 to I6 passing on both fixtures and no allowlist; the demo run, Pagila from one Docker container to another with `--root public.customer --take 200`, completed in 0.86 s real time, 15,920 rows across eleven tables, emails and names masked, source row count unchanged, `lazyslice.yml` emitted. Two gate items are open by nature and carried into phase 5 and 6: dogfood sessions against a real project need Gareth (tracker, owner human), and the 20-second GIF is launch material (tracker, E6). ADR-008 is accepted and frozen with this close. Phase 5 runs `.claude/workflows/hardening.js` with steps `features`, `harden`, `redteam`.

### Phase 4 gate, for the record

- [x] Pagila, 200 customers, container to container, under 60 seconds, one flag beyond the root.
- [x] All six invariants pass on both fixtures in CI.
- [ ] Two dogfood sessions logged (needs a real project; owner human).
- [ ] A 20-second GIF exists (E6, launch).

### Phase 3 gate, for the record

Gate 3 (this list is the definition; CLAUDE.md:7 currently restates it and has drifted — a follow-up task should reduce it to a pointer at this section):

- [ ] CI is green on an empty implementation, and integration tests fail for the right reason.
- [ ] `brew install` from your tap installs a binary that prints its version.
- [ ] Both fixtures load, and testdata/README.md names every trap.
- [ ] Per-directory CLAUDE.md files are in place (T-0023).
- [ ] This ROADMAP.md is in place (this task).
- [ ] ADR-008 first run is decided (T-0027).

## Phase 4: Vertical slice

Build the pipeline stage by stage, each behind its own flag, until one command runs a real snapshot from Postgres to Postgres.

Gate 4 · the demo gate:

- [ ] Pagila, 200 customers, from one Docker container to another, in under 60 seconds, with zero flags beyond the root table.
- [ ] All six invariants pass on both fixtures in CI.
- [ ] Two dogfood sessions logged, and the second needed nothing looked up.
- [ ] A 20-second GIF exists that shows the whole thing. If it is not impressive, the product is not done.

Not in this phase:

- A second database engine (MySQL, SQLite, SQL Server) — that is phase 7.
- Schema torture testing beyond the two fixtures, red-teaming, or performance tuning — that is phase 5.
- README rewrite, docs site, or any launch material — that is phase 6.
- A web UI or anything reachable only from the TUI — the TUI is a thin layer, and both are out of scope for a company anyway.
- The two Bubble Tea screens (ADR-002) — deferred to phase 5 by ARCHITECTURE.md section 14; `?` prints through the line printer and `$PAGER` until then.

## Phase 5: Hardening

Run the vertical slice against real open-source schemas until the failures that would embarrass a stranger are found and fixed, and every remaining failure explains itself. This phase also ships the items ARCHITECTURE.md section 14 assigns to "v1 after Gate 4 (phase 5), not deferred past v1": provisioning (`--create-target`, Q1) and discovery rung 4 (stopped containers); polymorphic inference (section 3.2); the two Bubble Tea screens (ADR-002); and the five-major CI matrix, ten torture schemas, 2M-row fixture, network-namespace test, SBOM and govulncheck.

Gate 5:

- [x] (re-ticked 2026-09-09, T-HARD-C: `make torture` exits 0 — all ten schemas, both catalogue guards and all eight regressions report `--- PASS`, 57 s) Nine of ten torture schemas snapshot cleanly and the tenth fails naming its cause (docs/TORTURE.md, T-TORTURE, 2026-09-08). The ten runs demand twenty-seven flags between them: nineteen `--unmask`, seven `--skip-table`, one `--key` — re-counted from that run, unchanged by T-0113, T-0121 and T-0122. It was forty-five and thirty-seven `--unmask` at T-TORTURE; eighteen of those were one defect, unique credential columns (T-0098), fixed in the harden step by `mask/gen_credential.go` and re-measured by T-0112 (2026-09-08). The two reductions in `testdata/regressions/` that still asserted the exit-12 refusal that fix removed are re-cut (T-0113): both now expect exit 0 and assert the masked column in the target, and `plan.refused.unique_domain` losing its only reduction is filed as T-0124. Classifier recall on the supabase-auth truth set is re-measured at 0.980 (precision 0.671), from 0.800/0.645 at T-TORTURE, after T-0104's name rules and T-0121's `public_key` decision — T-0115, T-0116.
- [ ] Red team, having first re-run the seven probes of docs/reviews/2026-09-09 against the fixed binary, finds nothing that leaks independently labelled sensitive data: credentials, identifiers, JSON keys, DDL literals, mixed-value columns, or values in any output sink (revised 2026-09-14; the earlier wording, "unmasked flagged data", excluded classifier misses by construction).
- [ ] Performance baseline is recorded and enforced in CI.
- [ ] Every item ARCHITECTURE.md section 14 lists under "v1 after Gate 4 (phase 5)" has shipped.

Not in this phase:

- A second database engine — that is phase 7.
- Public launch, README rewrite, docs site — that is phase 6.
- New pipeline capability that is neither in ARCHITECTURE.md section 14's phase-5 list nor a fix for something torture testing or the red team found.

## Versioning and releases (decided 2026-09-14)

Semantic versioning. Tags are `vX.Y.Z` for the tool and `mask/vX.Y.Z` for the mask module, which has its own module path. goreleaser builds the binaries and the Homebrew cask from a tag, and the release workflow refuses a tag whose commit has not passed CI (T-0140).

- **v0.0.x** are throwaway tags whose only job is to prove the release pipeline: goreleaser runs, the tap receives a cask, and `brew install Liarea/tap/lazyslice` prints a version on a machine that never built it. Gate 3 defined that check and it has never been run, because the tap is empty. The first is cut the day the repository goes public (T-0155).
- **v0.1.0 closes gate 5**, not phase 6 as the build plan said: torture, red team, performance baseline and the 2026-09-09 review findings landed. Marked pre-release on GitHub. It is the first version a stranger may install. The yml schema, flags and exit codes may still change between minors.
- **v0.2.0 closes gate 6**, the launch: README as landing page, the GIF, the docs. Every further 0.x minor may break the yml, a flag or an exit code, with the change named in the release notes and, where the yml is concerned, a tightening or migration note in the tool's own output; 0.x.y patches never do.
- **v1.0.0 is a compatibility promise, not a feature count.** It is cut when the lazyslice.yml schema, exit codes and flag set have gone two consecutive minors without a breaking change; the torture and red-team suites run green in CI on every supported Postgres major; at least five schemas from people other than the author have run unattended with no leak report open for thirty days; and a written stability policy says what a breaking change is and how long a deprecation lasts. A second engine, `mapping_file`, keyed remap and TUI polish are 1.x minors, not 1.0 requirements. Cutting 1.0 earlier would freeze a config schema nobody outside has used; cutting it only when everything imaginable works would never ship it, and 0.x would never signal that the tool can be trusted.

Release notes come from goreleaser's changelog grouped by the commit prefix (`stage: title (T-id)`); CHANGELOG.md is a pointer at GitHub releases until 1.0. The steps are in docs/RUNBOOK.md, "Cutting a release".

### Go-public checklist (inside phase 5, because gate 5 needs public CI minutes)

- [x] Tree and history scanned 2026-09-14: no tokens, personal emails, password-bearing DSNs or non-synthetic data; `gitleaks git .` over all 147 commits reports none after allowlisting four secret-shaped test fixtures (`.gitleaks.toml`).
- [x] README and SECURITY state the real status (T-0141, 2026-09-14).
- [x] git-crypt cancelled (T-0029, Gareth 2026-09-14): the AI-specific files stay public; nothing in them is a secret, and the operating model is part of what the project shows.
- [x] The tap repository has an initial commit (README, 2026-09-14), so goreleaser's first cask push has a branch to land on.
- [x] Visibility flipped to public 2026-09-14, with THIRD_PARTY_NOTICES.md for the ten torture schemas committed first (f4f37d7).
- [x] Branch protection on main 2026-09-14: no force pushes, no deletion, linear history. Required status checks are deliberately not set: they would reject the orchestrator's direct pushes (every new commit has unfinished checks at push time); the enforced gate is the release workflow, which refuses a tag whose commit has not passed `ci` (T-0140). DCO runs on pull requests, which is where outside commits arrive.
- [ ] v0.0.1 tagged and the pipeline proven (T-0155). The human steps above are T-0156.

## Phase 6: Launch

Ship v0.2.0 with a README that works as the landing page, and get it in front of strangers. (v0.1.0 is cut at gate 5, not here; see "Versioning and releases" below. docs/BUILD_PLAN.md's "v0.1.0 in public" at weeks 11 to 12 is superseded by this.)

Gate 6:

- [ ] Posted on HN and two subreddits. Every comment answered within a day for a week.
- [ ] A stranger has opened an issue with a schema you have never seen.
- [ ] The first ten issues are triaged into this ROADMAP, and at least half went to Later.

Not in this phase:

- A second database engine — that is phase 7.
- Any hosted-product work — deferred indefinitely by docs/adr/007-tool-not-company.md.
- Building a feature requested during launch before it's triaged into a phase or Later.

## Phase 7: Breadth and community

Add MySQL, SQLite, and SQL Server behind the same nine stage interfaces (ARCHITECTURE.md section 1 / ADR-005: discover, introspect, classify, plan, extract, transform, load, verify, emit) and the same invariant suite, and grow contributors.

Gate 7:

- [ ] Four databases pass the invariant suite.
- [ ] At least three merged PRs from people you have never met.
- [ ] Someone you do not know has written about the tool without being asked.

Not in this phase:

- Anything hosted, billed, or requiring infrastructure we run — deferred indefinitely by docs/adr/007-tool-not-company.md.
- A fifth database engine not named in Gate 7.

## Phase 8: Client-facing product

> Deferred indefinitely by docs/adr/007-tool-not-company.md. Only the design-partner interviews may run, as user research; the rest stays on this list for the record.

Find out, from five real teams, whether a hosted tier is worth building at all before building any of it.

Gate 8:

- [ ] Five interviews scored, and at least three named the same hosted feature.
- [ ] One design partner runs the hosted tier in anger for a month.
- [ ] First invoice paid.

Not in this phase:

- Building the hosted architecture, billing, or pricing paperwork — blocked on ADR-007 being superseded, which the interviews alone cannot do.
- Anything beyond the interview script and outreach.

## Later

Seeded from CONCEPT.md's v1 non-goals, split into two lists. Ticking a phase gate above never by itself makes anything in either list eligible to build; the two lists differ in what *can* move them.

### Refused

Permanent refusals stated as absolutes by a CONCEPT.md principle ("we refuse to ship...") or by ADR-007. These do not move because a gate was ticked — a gate tick is a schedule event, not a decision. The only way one of these ships is a new ADR that supersedes the principle or the ADR behind it (CLAUDE.md: "to change a decision, add a new ADR that supersedes the old one"), and until that ADR exists it is not built even if requested.

- A web UI, a hosted service, or any control plane between the developer and their database — the terminal-first principle refuses it outright (CONCEPT.md "Terminal first"); ADR-007 defers anything hosted indefinitely.
- Any unsafe or pass-through mode — the safe-by-default principle refuses it outright (CONCEPT.md "Safe by default": "We refuse to ship a flag, mode, or default that copies an unclassified column as-is"). ARCHITECTURE.md section 8 fixes the forbidden flag names (`no-mask|disable-mask|skip-mask|unsafe`, and others) and `make forbidden` (part of `make check`) fails the build on them; CLAUDE.md: "Never add a flag that disables masking wholesale."
- Anonymisation, k-anonymity, or any compliance guarantee — this tool pseudonymises and says so (CONCEPT.md "Safe by default"); it makes no compliance claim.
- A cloud or LLM classifier over production values — would send production data off the user's box; the classifier stays local and conservative (CONCEPT.md "Terminal first": "no call to anything we operate").

### Later, unscheduled

Genuine deferrals: not refused on principle, just not scheduled. Nothing here is scheduled into a phase without a new decision, and nothing moves out until the current phase's gate above is fully ticked.

- Synthetic data generation from a schema alone — not what this tool does; the point is real data shapes, not invented ones.
- Scheduling, retention, or sharing snapshots between people — needs infrastructure we run; a hosted-tier question, and that question is deferred by ADR-007.
- NoSQL and document stores — v1 is Postgres-only; other engines are phase 7 and SQL-only at that.
- Schema migration or diffing — a different problem from subsetting and masking.
- Databases other than PostgreSQL — phase 7 work, and it waits until the invariant suite passes on every Postgres fixture.
- Detecting personal data inside binary blobs — text and JSON are masked whole instead; blobs stay unaddressed.
- Format-preserving encryption — the masking scheme is keyed hashing, not FPE.
- Percent-based sampling or traversal knobs in config beyond the plan-stage flags in ARCHITECTURE.md section 8 (`--root`, `--take`, `--cap`, `--depth`, `--where`, `--row-budget`, `--memory-budget`, `--key`, `--skip-table`) — the zero-config principle refuses percent-based sampling and further knobs beyond that set (CONCEPT.md non-goals).
- Parsing docker-compose.yml for connection strings — ruled out in the first-run design, docs/adr/008-first-run.md section 2 ("Compose YAML is a naming source and nothing else (ADR-004) ... It is never parsed for a host, a port, a user, a password or a database name that becomes part of a DSN").
- Multi-terabyte sources and resumable extracts — a run finishes in minutes or aborts on its budget; no resumability is designed.
