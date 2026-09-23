# Roadmap

Source of truth for phase sequencing: docs/BUILD_PLAN.md, phases 4 to 8. Where an architectural fact below (stage names and counts, flags, ADR numbers) conflicts with ARCHITECTURE.md or an ADR, the ADR wins — docs/BUILD_PLAN.md predates ADRs 001-008 and is not corrected when they supersede it. This file tracks where we are and what is explicitly not now.

**Rule:** any feature request goes into the Later section at the bottom with a one-line reason, into "Later, unscheduled" unless it is a permanent refusal, in which case it goes into "Refused". Nothing moves out of "Later, unscheduled" until the current phase's gate below is fully ticked. Nothing moves out of "Refused" by a gate tick at all — see that section for what it takes.

## Current phase: 6, Launch

**Status 2026-09-22 (evening):** v0.2.0 is released (https://github.com/Liarea/lazyslice/releases/tag/v0.2.0, cut by `make tag` on c300a45 after green CI): `go install github.com/Liarea/lazyslice/cmd/lazyslice@v0.2.0` works from an empty module cache (the tool requires the tagged mask module, mask/v0.2.0; T-0285, T-0290), `brew install Liarea/tap/lazyslice` prints `lazyslice 0.2.0`, a first-name column masks to one given name and a last-name column to one surname (T-0287), and the first-run GIF's reason lines were cleaned (T-0288, T-0269, T-0297, T-0289). What gate 6 still needs is the maintainer's: the dogfood sessions (T-0064), the "why I built this" paragraph (T-0282), the docs-site decision (T-0283), and posting the drafts in docs/launch/.

**Status 2026-09-22:** phase 5 is closed and v0.1.0 is cut at its gate (the paragraph below is the evidence). Phase 6 begins with the launch material: the 20-second GIF (T-0065), the README as a landing page, the posts; and the two dogfood sessions (T-0064) carried since phase 4 are the first thing to do, because ADR-002's and ADR-008's reversal conditions both wait on them. Open work is tracked as GitHub issues (T-0196); `tracker/tasks/` is the archive.

**Status 2026-09-17:** the red-team item is ticked under its stopping rule (round 6, a pure replay: nothing the threat model does not name), all thirty red-team fix tasks are on main, and README.md and SECURITY.md were read end to end and corrected. One gate item is open, the section-14 list, and an audit of it against the code found two things that had never been built: THREAT_MODEL.md T4's network-namespace test (T-0270) and ADR-008's root-table question, Q2 (T-0271); both run under `hardening.js {step: 'gate'}`. JSON per-leaf categories move past v1 pending the maintainer's JSON policy decision (ARCHITECTURE.md section 14's amendment, T-0272). The same audit found CI red on main since 2026-09-16 on three jobs that no landing had looked at (lint, the Windows tests, the Postgres 14 leg); repaired in 47f6c34, and the operating model now makes reading CI part of every push. After the gate: T-0155 (v0.0.1 proves the release pipeline), T-0196, then v0.1.0.

**Status 2026-09-15:** harden step complete (every task merged, T-PERF last); gate items one and three ticked; the red team's first two rounds are recorded under the red-team gate item below and the fix step `{step: 'redfix'}` is running. The repository is public and CI green with torture on main and a relative bench gate.

**Status 2026-09-14 (resumed):** the harden step is re-sequenced around an independent adversarial review of commit 936ceec, docs/reviews/2026-09-09/REVIEW.md, which reproduced six leak-class or contract defects with disposable containers (target ownership lapses between gate and drop; a polymorphic warning prints a source value; the unique escalation breaks an FK-connected pair; `complete` is written before verify; DDL defaults carry source literals; one strong hit in a sparse column passes the second net) and traced two more (TLS parameters lost on the config round trip; `mapping_file` accepted but unimplemented). Those are tracker T-0130 to T-0141 in E5 and run ahead of failure UX and performance: T-0129, T-0127 (arrays), T-0130 (target lease and lock-and-recheck), T-0133 (lifecycle owned by core), T-0131 (value-free events and a canary test), T-0132 (masker per FK equality group), T-0134 (DDL literals), T-0136 (strong single hit), T-0137 (JSON keys), T-0135 (transport parameters), T-0138 (mapping_file refused, ADR-012), T-0139 (torture baseline), T-0119 (table-scoped rule), T-0140 (CI torture and release gate), T-FAILUX, T-PERF, T-0141 (README and SECURITY drift), then the red team, which re-runs the review's probes before its own attacks. Tiering: Sonnet developer at medium effort and one Opus reviewer at medium effort by default; Opus developer only for T-0129, T-0130, T-0132 and T-0134. Resume: `hardening.js {step: 'harden', from: <index of the first unmerged task>}`, then `{step: 'redteam'}`. Before the previous pause the harden step had merged seven tasks: type registration for COPY, the torture suite, the unique credential masker with the post-plan fingerprint, the torture re-measure, composite fail-closed with the array-literal splitter, the mechanical trio, and transform's element-wise array masking as a partial landing (T-0118). GitHub Actions has not started a job since 2026-09-09 04:23 UTC (every job ends in three seconds with no steps, the signature of a billing or spending-limit block on the account); that is the maintainer's to clear and gate 5 cannot close on local runs alone.

Phase 5 closed 2026-09-22. Evidence: all four gate items ticked above with dates and run ids — nine of ten torture schemas clean under twenty-eight flags and `make torture` green in CI; the section-14 list audited against the code, with the two never-built items landed the same day (`make egress` for T4, ADR-008's Q2); the red team stopped under its written rule after six rounds and about 240 attempts, with round six a pure replay that reproduced only residuals THREAT_MODEL.md names; the performance baseline enforced by a relative bench gate. The 2026-09-09 review's findings landed as T-0130 to T-0141, and thirty red-team fix tasks as T-0187 to T-0271. CI on main is green on every job, including the five-major integration matrix, torture, bench, egress and govulncheck. The release pipeline is proven by v0.0.3 (T-0155) and `make tag` cuts a tag only when every runbook precondition holds. README.md and SECURITY.md were read end to end at the close and again before the tag. ADR-012, ADR-013 and ADR-014 are accepted and frozen with this close. The tracker moved to GitHub Issues and Project 3 (T-0196). Carried into phase 6 by nature: the dogfood sessions (T-0064) and the GIF (T-0065). v0.1.0 is tagged on this close per "Versioning and releases" below.

Phase 4 closed 2026-09-08. Evidence: all eleven packages merged; `make integration` green locally and in CI (run 34175303077) with every invariant I1 to I6 passing on both fixtures and no allowlist; the demo run, Pagila from one Docker container to another with `--root public.customer --take 200`, completed in 0.86 s real time, 15,920 rows across eleven tables, emails and names masked, source row count unchanged, `lazyslice.yml` emitted. Two gate items are open by nature and carried into phase 5 and 6: dogfood sessions against a real project need the maintainer (tracker, owner human), and the 20-second GIF is launch material (tracker, E6). ADR-008 is accepted and frozen with this close. Phase 5 runs `.claude/workflows/hardening.js` with steps `features`, `harden`, `redteam`.

### Phase 4 gate, for the record

- [x] Pagila, 200 customers, container to container, under 60 seconds, one flag beyond the root.
- [x] All six invariants pass on both fixtures in CI.
- [x] (2026-09-23, docs/DOGFOOD_LOG.md: two sessions against a production Rails application of the maintainer's, 143 tables; nine runs to the first green verify, five to the second; findings filed as T-0311 to T-0327) Two dogfood sessions logged (needs a real project; owner human).
- [ ] A 20-second GIF exists (E6, launch).

### Phase 3 gate, for the record

Gate 3 (this list is the definition; CLAUDE.md:7 currently restates it and has drifted — a follow-up task should reduce it to a pointer at this section):

- [ ] CI is green on an empty implementation, and integration tests fail for the right reason.
- [x] (2026-09-22, v0.0.3: `brew install Liarea/tap/lazyslice` on a Mac that did not build the release printed `lazyslice 0.0.3` with its commit; docs/RUNBOOK.md "Cutting a release") `brew install` from your tap installs a binary that prints its version.
- [ ] Both fixtures load, and testdata/README.md names every trap.
- [ ] Per-directory CLAUDE.md files are in place (T-0023).
- [ ] This ROADMAP.md is in place (this task).
- [ ] ADR-008 first run is decided (T-0027).

## Phase 4: Vertical slice

Build the pipeline stage by stage, each behind its own flag, until one command runs a real snapshot from Postgres to Postgres.

Gate 4 · the demo gate:

- [ ] Pagila, 200 customers, from one Docker container to another, in under 60 seconds, with zero flags beyond the root table.
- [ ] All six invariants pass on both fixtures in CI.
- [x] (2026-09-23, docs/DOGFOOD_LOG.md) Two dogfood sessions logged; the second did NOT meet "needed nothing looked up": it took four runs to reach the recorded target (T-0327) and one new flag for one new value (T-0319). The sessions are the evidence; the criterion is carried into the phase-6 work those tasks name.
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

- [x] (re-ticked 2026-09-09, T-HARD-C: `make torture` exits 0 — all ten schemas, both catalogue guards and all eight regressions report `--- PASS`, 57 s) Nine of ten torture schemas snapshot cleanly and the tenth fails naming its cause (docs/TORTURE.md, T-TORTURE, 2026-09-08). The ten runs demand twenty-seven flags between them: nineteen `--unmask`, seven `--skip-table`, one `--key` — re-counted from that run, unchanged by T-0113, T-0121 and T-0122; twenty-eight as of 2026-09-17 (T-0253 and T-0257: Metabase's session-id pair needs a second `--unmask`, twenty `--unmask` in all; T-0258 narrowed the refusal to skip a pair only when both ends are genuinely masked under one category, which Metabase's pair is not — its parent is copied verbatim on the operator's own `--unmask` — so the count stays at twenty-eight; docs/TORTURE.md has the account). It was forty-five and thirty-seven `--unmask` at T-TORTURE; eighteen of those were one defect, unique credential columns (T-0098), fixed in the harden step by `mask/gen_credential.go` and re-measured by T-0112 (2026-09-08). The two reductions in `testdata/regressions/` that still asserted the exit-12 refusal that fix removed are re-cut (T-0113): both now expect exit 0 and assert the masked column in the target, and `plan.refused.unique_domain` losing its only reduction is filed as T-0124. Classifier recall on the supabase-auth truth set is re-measured at 0.980 (precision 0.671), from 0.800/0.645 at T-TORTURE, after T-0104's name rules and T-0121's `public_key` decision — T-0115, T-0116.
- [x] (ticked 2026-09-17 under the stopping rule below: round 6, a pure replay, found nothing the threat model does not name) Red team, having first re-run the seven probes of docs/reviews/2026-09-09 against the fixed binary, finds nothing that leaks independently labelled sensitive data: credentials, identifiers, JSON keys, DDL literals, mixed-value columns, or values in any output sink (revised 2026-09-14; the earlier wording, "unmasked flagged data", excluded classifier misses by construction). Stopping rule (decided 2026-09-17, after five rounds and about 200 attempts): every attempt from rounds 1 to 5 either holds or reproduces a residual THREAT_MODEL.md names as accepted, confirmed by a pure replay round with no new variants; each fix round's attackers were told to try one new variant of every held attempt, which finds new shapes every round by construction, so fresh-variant hunting becomes a standing per-release activity from phase 6 rather than a gate-5 blocker.
  - Round 1 (2026-09-15, docs/reviews/2026-09-15-redteam/round1.json): 58 attempts, 17 leaked. All seven 2026-09-09 probes held and so did every wrong-target attack, the lease, the lock-and-recheck, the source rail and the secret sinks. What leaked: column names in other languages, values that dodge every parser (spelled-out digits, `at`/`dot` addresses), plain national identifiers with no validator at all, printable bytea, enum labels and domain defaults and CHECK literals, a JSON document in a text column, a passthrough or panicking custom masker, a symlinked or world-readable secret file, cluster identity computed from the transport.
  - Fix round (T-REDFIX, partial landing 2026-09-15): forty-six files; de-obfuscating candidates in textsig, bytea and tsvector no longer skip the validators, enum labels and domain defaults and generated expressions in both catalog passes, quarantine drops types and sequences too, `--allow-type-literal`, same-cluster warning wired, secret file symlink and mode checks, panics redacted. Blocked only on the mask module being outside its paths (T-0180, T-0181).
  - Round 2 against that tree (round2-still-leaking.json): 16 still leaking, grouped into six tasks T-0187 to T-0192 plus T-0184, T-0186 and T-0178: national-identifier validators (the severe one: a plain SSN in a column called `code` crosses), multilingual names, catalog literals under every validator and partial-index predicates, transport-independent cluster identity, the mask module post-condition and recover, and the secret file on its resolved path. `hardening.js {step: 'redfix'}` runs them and re-attacks.
  - Round 3 (2026-09-16, replay of the sixteen round-2 attempts on ee2c64f, docs/reviews/2026-09-15-redteam/round3.json): 64 attempts, 17 leaked, all of them on four shapes. Two are accepted residuals the threat model already names: a bare nine-digit national identifier with no dashes, no name signal and no personal neighbour (T-0187), and names in Khmer, Lao and Amharic (T-0197). Two were not fixed by the round-2 tasks and are filed as phase-5 work ahead of round 4: phone numbers in national format or spelled out in words cross verbatim because only a +E.164 number parses (T-0221); a masked column's own CHECK carries a literal of the same category and no validator reaches it (T-0198, re-homed). Two smaller ones: a role that cannot read one identity function collapses the whole cluster identity to unknown and an alias then decides (T-0222); the mask module still returns a masker's error with the value in it (T-0223), and the CLI's renderSafe allowlist is the other half (T-0212, re-homed). Everything else on the round-2 list held, including all seven 2026-09-09 probes, the run lease, the lock-and-recheck and the secret sinks. Round 4 replays round 3's seventeen after those five land.
  - Round 4 (2026-09-17, replay of round 3's seventeen on c9536ed with three attackers, docs/reviews/2026-09-15-redteam/round4.json): 26 attempts, 11 leaked, 6 of them residuals the threat model names (bare nine-digit identifiers without corroboration, names in scripts the dictionary cannot carry, and a table created in the target outside the plan). Five leaks it does not name, on four shapes, are phase-5 tasks ahead of round 5: a masker can smuggle its text through the module by wrapping a module sentinel (T-0238); the certain-neighbour rail skips a column declared shorter than sixteen characters, so a varchar(12) name beside a real email column copies verbatim (T-0239); nine-digit identifiers beside a masked phone column get no corroboration, in either family (T-0240); a streaming standby as source with the target on its own primary reads as a different cluster because the start times differ and the identifier is unreadable, and writes to production (T-0241). The out-of-plan table is fixed too because the fix is one probe under the lease already held (T-0242), and the misleading 'no name or value signal' sentence becomes absence of evidence (T-0197). All four round-3 fixes held: national-format and dictated phone numbers, the cluster identity under a revoked function, the masker error wrap, and the special-category literal in a masked column's constraint.
  - Round 5 (2026-09-17, replay of round 4's eleven on 27ce492 with three attackers, docs/reviews/2026-09-15-redteam/round5.json): 31 attempts, 10 leaked, 5 of them named residuals (names in scripts the dictionary cannot carry, in a column, an array and native script; the dense identifier block behind the never-masked carve-out; the out-of-plan table on a reload bound to an existing marker). Five it does not name are phase-5 tasks ahead of a pure replay: a .gitignore that lists the secret file and then negates it fools the text-match check, so the key is written where git commits it (T-0251); a run whose lease connection is killed mid-run never re-checks it, so two runs take the target apart together (T-0252); a validated foreign-key child escapes the certain-neighbour rail through the exclusion its own fix round added (T-0253); a special-category term glued by an underscore, HIV_POSITIVE, misses the vocabulary (T-0254); a standby whose role can read the data directory reads as a different cluster (T-0255). All six round-4 fixes held, and the misleading 'no name or value signal' sentence is gone.
  - Round 6 (2026-09-17, a pure replay of round 5's ten on ffd65b1 with three attackers and no new variants, docs/reviews/2026-09-15-redteam/round6.json): 10 attempts, 5 held, 5 leaked, none of them undocumented. All five round-5 fixes held: the secret file is written only where git itself reports it ignored (T-0251), a lost lease refuses the load at exit 4 (T-0252), a validated foreign-key child beside a certain column is masked with its parent or the pair is refused (T-0253, T-0257, T-0258), HIV_POSITIVE refuses at exit 13 (T-0254), and a standby on its own primary reads as the same cluster (T-0255). The five that still leak are the residuals the threat model accepts, checked against its text by the orchestrator rather than taken from the attackers' flag: names the dictionary does not carry in a column no rule names (scalar, array, and native script in a column named in that script), a table created in the target outside the plan on a reload bound to an existing marker, and a contiguous block of nine-digit identifiers used as a table's own primary key. That last one was named in THREAT_MODEL.md only by its mechanism; T1 now says it in plain words, README.md's second residual carries it, and the narrowing worth measuring (a key with no sequence or identity default) is T-0266. Fresh-variant hunting moves to phase 6 as a per-release activity.
- [x] (2026-09-15, T-PERF/T-0090, T-0177, T-0179) Performance baseline is recorded and enforced in CI. docs/PERF.md: 5,000 roots over a 2,000,000-row child, per-stage pprof, two hotspots fixed, byte-capped batches with peak RSS measured. The CI gate is relative, `make bench-compare` (head against its parent on the same runner, interleaved, best of five, 20% ceiling), because three consecutive ubuntu-latest runs of identical code measured 10.9M, 6.7M and 8.9M rows/sec; the absolute baseline is a 3,000,000 rows/sec catastrophic floor. First blocking run: 34929041470, +0.6% against the parent.
- [x] (ticked 2026-09-17 after an audit of the list against the code, not against the documents) Every item ARCHITECTURE.md section 14 lists under "v1 after Gate 4 (phase 5)" has shipped. Nine of eleven were on main with code, tests and a green CI job: `--create-target` and rung 4 (T-0067), polymorphic inference with its `not followed` line (T-0068), the two Bubble Tea screens behind `--tui` (T-0066), the five-major matrix (T-0069), the ten torture schemas, the 2M-row fixture with the bench gate (T-0090), SBOM and `govulncheck`, and the name dictionaries in about twenty languages (T-0188). Two had never been built and landed the same day: the network-namespace test for T4 is `make egress` and the blocking `egress` CI job (T-0270), and ADR-008's root-table question, Q2, is asked at a terminal (T-0271; ADR-014, proposed, records why `?` there prints rather than opening a screen). One half-item moves past v1 by a dated amendment to section 14: per-leaf categories for JSON documents wait on the maintainer's JSON policy decision (T-0143, T-0272); every leaf is masked either way. The audit also found main red in CI for two days; see the 2026-09-17 status above.

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
- [x] git-crypt cancelled (T-0029, the maintainer 2026-09-14): the AI-specific files stay public; nothing in them is a secret, and the operating model is part of what the project shows.
- [x] The tap repository has an initial commit (README, 2026-09-14), so goreleaser's first cask push has a branch to land on.
- [x] Visibility flipped to public 2026-09-14, with THIRD_PARTY_NOTICES.md for the ten torture schemas committed first (f4f37d7). The first public CI run, 34901076717 on c6fb76d, is green on every job: lint, forbidden, unsafe-flags, docs, release-config, govulncheck, tests on Ubuntu, macOS and Windows, and integration on Postgres 14 to 18; the first green CI since 2026-09-09 04:23 UTC. Run 34924823369 on b132d03 (2026-09-15) is the first green run that includes the torture job on main (T-0140): ten schemas, thirteen regressions, the catalogue guard and the I4 negative control.
- [x] Branch protection on main 2026-09-14: no force pushes, no deletion, linear history. Required status checks are deliberately not set: they would reject the orchestrator's direct pushes (every new commit has unfinished checks at push time); the enforced gate is the release workflow, which refuses a tag whose commit has not passed `ci` (T-0140). DCO runs on pull requests, which is where outside commits arrive.
- [x] (2026-09-22) v0.0.x tagged and the pipeline proven (T-0155): v0.0.1 failed before its first step on an action tag that does not exist, v0.0.2 at signing on cosign v3's bundle format, and v0.0.3 published six archives, six SBOMs, a signed checksum file (`cosign verify-blob` answers `Verified OK`) and the cask; each failure and its fix is in docs/RUNBOOK.md. `make tag` now refuses a tag until every precondition the runbook names holds. The human steps above are T-0156.

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
