# Roadmap

Source of truth for phase sequencing: docs/BUILD_PLAN.md, phases 4 to 8. Where an architectural fact below (stage names and counts, flags, ADR numbers) conflicts with ARCHITECTURE.md or an ADR, the ADR wins — docs/BUILD_PLAN.md predates ADRs 001-008 and is not corrected when they supersede it. This file tracks where we are and what is explicitly not now.

**Rule:** any feature request goes into the Later section at the bottom with a one-line reason, into "Later, unscheduled" unless it is a permanent refusal, in which case it goes into "Refused". Nothing moves out of "Later, unscheduled" until the current phase's gate below is fully ticked. Nothing moves out of "Refused" by a gate tick at all — see that section for what it takes.

## Current phase: 3, Foundations

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

- [ ] Nine of ten torture schemas snapshot cleanly, and the tenth fails with a message that says exactly why.
- [ ] Red team found nothing that leaks unmasked flagged data.
- [ ] Performance baseline is recorded and enforced in CI.
- [ ] Every item ARCHITECTURE.md section 14 lists under "v1 after Gate 4 (phase 5)" has shipped.

Not in this phase:

- A second database engine — that is phase 7.
- Public launch, README rewrite, docs site — that is phase 6.
- New pipeline capability that is neither in ARCHITECTURE.md section 14's phase-5 list nor a fix for something torture testing or the red team found.

## Phase 6: Launch

Ship v0.1.0 in public with a README that works as the landing page, and get it in front of strangers.

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
