# Lazysnap Build Plan

Zero to one hundred · working name: lazysnap

One command that points at a production database, pulls out a small coherent slice, masks the personal data, and loads it into a local database you can safely develop against. This plan runs from a blank folder to a paid, hosted product, with the Claude Code prompts for each step.

```
# the whole pitch, in one line
$ lazysnap --from prod --root users --take 500
  detected 3 docker databases · 41 tables · 17 columns look like PII · ready in 38s
```

### Zero config
First run asks at most one question. Config is emitted after a run, never required before it.

### Safe by default
Anything that might be personal data is masked unless you say otherwise. The tool never writes to the source.

### Terminal first
One binary, one-line install, works headless in CI with the same command a human typed.

## How to use this plan

Each phase has a goal, a gate, and prompts. The gate is a list of things that must be true before the next phase starts. If you cannot tick it, you are not done, however good the code feels. The prompts are written for Claude Code with the repo open. Paste them as written, then argue with the output. Every prompt asks for a file, so decisions survive the session.

Timeline assumes one person at roughly fifteen hours a week. Halve the weeks if this is full time.

## Phase 0: Frame

**Days 1 to 2** | Output: CONCEPT.md, NAME.md

Write down what the thing is before researching what exists. Research done first turns into a feature list of everyone else's product.

### Prompts

#### PROMPT 0.1 · concept

```PROMPT 0.1 · concept
Create CONCEPT.md for a new open-source CLI called lazysnap. It snapshots a production SQL database into a local dev database: subset by a root table and row count, follow foreign keys in both directions so the slice is referentially complete, detect and mask personal data, load into a target. Model it on how sqlit and lazygit win: zero config, zero docs needed, magic first run.

Sections: one-line pitch; the user (a backend dev on a 3 to 30 person team); the moment they reach for it; the three principles (zero config, safe by default, terminal first) with one sentence each on what we refuse to ship because of it; explicit non-goals for v1 (synthetic data generation, scheduling, web UI, NoSQL, schema migration); and a "what a delighted first run looks like" transcript. Keep it under 700 words. Do not list features. Do not mention competitors.
```

#### PROMPT 0.2 · name

```PROMPT 0.2 · name
Working name is lazysnap. Check it: search GitHub, PyPI, npm, crates.io, Homebrew, and the USPTO trademark database for lazysnap and close variants. Report collisions with links. Then propose five alternative names that keep the "lazy" family association, are pronounceable, and have a free .dev or .sh domain (check with whois or a DNS lookup). Write findings to NAME.md with a recommendation and its reasoning.
```

### Gate 0

- All six research documents exist and cite sources.
- You can name, from memory, the top three user complaints and the top failure of each dead competitor.
- CONCEPT.md has been revised against SYNTHESIS.md and non-goals grew, not shrank.

## Phase 1: Research

**Week 1** | Output: research/ folder, six documents

Four questions. What did the dead companies get wrong? What do the living tools make hard? What is technically difficult about subsetting and masking? What do users actually complain about, in their own words?

### Prompts

#### PROMPT 1.1 · competitor teardown

```PROMPT 1.1 · competitor teardown
Research these tools and write research/COMPETITORS.md: Greenmask, Basecut, Neosync (archived after the Grow Therapy acquisition), Snaplet snapshot (open-sourced 2024, dormant), PostgreSQL Anonymizer, Jailer, condenser, replibyte, pgsubset, Tonic Structural, and Redgate Data Masker. For each: language, supported databases, how subsetting works (algorithm, direction of FK traversal, handling of cycles), how masking is configured, install path, licence, last commit date, star count, and the three most-upvoted open issues. Then a table: "time from install to first usable snapshot" estimated by reading their quickstarts. End with a section "what nobody does well" citing evidence. Include source links for every claim.
```

#### PROMPT 1.2 · post-mortems

```PROMPT 1.2 · post-mortems
Snaplet shut down in August 2024 and Neosync was acquired in August 2025. Read their archived docs, GitHub issues, changelogs, HN threads, founder posts, and any interviews. Write research/POSTMORTEMS.md answering: what did users love, what did users abandon them over, what did the product become that it did not start as, where did the complexity accumulate, and what pricing did they try. Finish with "five things we will not copy" and "three things we must copy". Quote users directly, with links.
```

#### PROMPT 1.3 · hard problems

```PROMPT 1.3 · hard problems
Write research/HARD_PROBLEMS.md, a technical survey of the four hard problems in database subsetting and masking. (1) Subsetting: walking FK graphs with cycles, self-references, composite keys, polymorphic associations without constraints, and the choice between "children of root" versus "everything reachable". Cite Jailer's and Greenmask's approaches and academic work on referential subsetting. (2) Deterministic masking: keeping the same fake value for the same input across tables so joins on email still work, using keyed hashing; format-preserving options. (3) PII classification: column-name heuristics, regex on sampled values, entropy, and where ML is and is not worth it; list the categories a first release must catch (email, phone, name, address, DOB, IP, card, national ID, free text, JSON fields). (4) Streaming extraction and load at scale: COPY protocol, batching, ordering inserts to satisfy constraints, deferring constraints, sequences and identity columns. For each problem give the simplest approach that is correct, then the refinement.
```

#### PROMPT 1.4 · complaint mining

```PROMPT 1.4 · complaint mining
Mine real user complaints about getting realistic data into local and CI databases. Sources: Hacker News search, r/PostgreSQL, r/devops, r/webdev, r/ExperiencedDevs, dev.to, GitHub issues on Greenmask, Neosync, Snaplet, pg_anonymizer, and Stack Overflow questions tagged database-testing or anonymization. Write research/COMPLAINTS.md with at least 25 verbatim quotes, each with a link, tagged by theme (config burden, speed, broken FKs, leaked PII, wrong database, CI). Then rank the themes by frequency and add a short note on which theme our three principles address and which they do not.
```

#### PROMPT 1.5 · licence

```PROMPT 1.5 · licence
Write research/LICENSE_DECISION.md comparing Apache-2.0, MIT, AGPL-3.0, and BSL 1.1 for an open-core CLI where the hosted service is the paid product. Cover: contributor comfort, corporate adoption friction, protection against a cloud vendor hosting our tool, what Greenmask and Neosync chose and why, and how the licence interacts with a future closed hosted layer in the same repo versus a separate repo. Recommend one licence and one repo structure. Keep it to one page.
```

#### PROMPT 1.6 · synthesis

```PROMPT 1.6 · synthesis
Read everything in research/ and CONCEPT.md. Write research/SYNTHESIS.md: the ten facts that most change what we build, each with a link back to its source document; three risks that could kill the project with a mitigation for each; and a revised non-goals list. Then flag anything in CONCEPT.md that the research contradicts. Do not soften findings.
```

### Gate 1

- All six research documents exist and cite sources.
- You can name, from memory, the top three user complaints and the top failure of each dead competitor.
- CONCEPT.md has been revised against SYNTHESIS.md and non-goals grew, not shrank.

## Phase 2: Architecture

**Week 2** | Output: docs/adr/001 to 006, ARCHITECTURE.md, THREAT_MODEL.md

Six decisions, each recorded as an Architecture Decision Record so future-you and contributors can see why. My recommendation on each is below. Let the prompts try to argue you out of it.

### Recommended decisions

- **Language: Go.** Single static binary, first-class Docker SDK, the COPY protocol via pgx, and it is what lazygit, lazydocker, and Greenmask use. Python with Textual gets you thirty databases faster via SQLAlchemy, but distribution and streaming performance are worse. Breadth is a phase 7 problem, not a phase 4 one.
- **TUI: Bubble Tea and Lip Gloss.** The lazygit lineage. The TUI is one thin layer over a library that works headless.
- **Database order: PostgreSQL, then MySQL, SQLite, SQL Server.** Postgres is where the dead competitors' users are.
- **Config: emitted, not required.** First run writes `lazysnap.yml` describing exactly what it did. Committing that file makes CI reproducible.
- **Pipeline: introspect → classify → plan → extract → transform → load.** Each stage has one interface and can be run alone with a flag, which is what makes it testable.
- **Masking is pluggable, classification is not.** Users add maskers. The classifier is ours and conservative.

### Prompts

#### PROMPT 2.1 · language ADR

```PROMPT 2.1 · language ADR
Write docs/adr/001-language.md deciding between Go and Python for lazysnap. Argue both sides honestly using research/HARD_PROBLEMS.md and COMPETITORS.md. Criteria in priority order: install experience (curl, brew, single binary), streaming throughput on a 50M row table, quality of Postgres, MySQL, SQLite and SQL Server drivers, TUI library maturity, ease of contribution, and how sqlit got to 30 databases in Python. Give a benchmark plan we could run in a day to settle the throughput question. End with a decision and the condition under which we would reverse it. Format: Context, Options, Decision, Consequences.
```

#### PROMPT 2.2 · pipeline

```PROMPT 2.2 · pipeline
Write ARCHITECTURE.md. Define the pipeline introspect → classify → plan → extract → transform → load as six Go interfaces with their inputs and outputs as types. Show how one run flows through them, how the TUI and the headless CLI share the same core, and how a run emits lazysnap.yml that reproduces it. Specify the subset planner: given a root table and N, produce an ordered list of (table, predicate) steps that pulls parents first, children after, handles cycles by deferring constraints, and caps runaway fan-out with a per-table limit. Include a Mermaid diagram of the pipeline and a second one of the FK walk on a schema with a cycle. Note which stage each hard problem from research/HARD_PROBLEMS.md lives in.
```

#### PROMPT 2.3 · classifier design

```PROMPT 2.3 · classifier design
Write docs/adr/003-classifier.md. Design the PII classifier as three layers: column name heuristics (a curated list with weights, multilingual), value sampling with regexes and validators (Luhn for cards, RFC 5322 email, E.164 phone), and a "free text and JSON" catch-all that masks the entire field. Define confidence levels and the rule: anything at or above "possible" is masked unless the user opts out per column. Specify how the classifier explains itself in the TUI ("masked because column name contains 'email' and 97% of samples matched"). List the categories for v1 and the maskers each maps to. State what we will not try to detect in v1 and why.
```

#### PROMPT 2.4 · deterministic masking

```PROMPT 2.4 · deterministic masking
Write docs/adr/004-masking.md. Specify the masker interface, the built-in maskers for each v1 PII category, and the determinism scheme: an HMAC of the original value with a per-run secret feeds a seeded generator so the same email maps to the same fake email everywhere in the snapshot, and a different run with a different secret cannot be correlated. Cover format preservation (keep the domain shape, keep phone country code), null and empty handling, unique constraints on masked columns, and how a user registers a custom masker without recompiling (decide: Go plugin, subprocess, or expression language; pick one).
```

#### PROMPT 2.5 · threat model

```PROMPT 2.5 · threat model
Write THREAT_MODEL.md for lazysnap. Assets: production data, credentials, the snapshot on disk. Threats: masking misses a PII column, a user points the target at production, credentials leak into lazysnap.yml or logs, a snapshot file lands in git, a malicious custom masker, and supply chain risk in our release pipeline. For each threat: likelihood, impact, and the concrete control we build (for example: refuse to load into any database whose name or host matches the source, or which has more than 10k rows in any table, unless --i-know-this-is-not-prod is passed; never write secrets to the emitted config; add snapshots/ to .gitignore on first run). Mark which controls are v1 blockers.
```

#### PROMPT 2.6 · sqlit study

```PROMPT 2.6 · sqlit study
Clone github.com/Maxteabag/sqlit and read it. Write docs/adr/006-first-run-experience.md describing exactly how sqlit achieves zero-config first run: the Docker container detection, connection saving, keyring use, and the keybinding discoverability. Then specify our equivalent for lazysnap step by step: what happens when a user types `lazysnap` with no arguments in a project folder that has a docker-compose.yml, a DATABASE_URL in .env, or nothing at all. Define every question the tool may ask and its default. Cap it at one question on the happy path.
```

### Gate 2

- Six ADRs, each with a stated reversal condition.
- You could sketch the pipeline and the FK walk on a whiteboard without notes.
- THREAT_MODEL.md lists at least three v1-blocking controls, and they appear in the phase 4 scope.

## Phase 3: Foundations

**Weeks 3 to 4** | Output: a repo that builds, tests, and releases nothing useful yet

Everything that keeps the project honest gets built before the product does. The test fixtures are the most important artifact in the whole plan, because they define what "correct" means before you have an opinion.

### Prompts

#### PROMPT 3.1 · scaffold

```PROMPT 3.1 · scaffold
Scaffold the Go repo per ARCHITECTURE.md: cmd/lazysnap, internal/{introspect,classify,plan,extract,transform,load,tui}, one interface file per stage with a doc comment and a no-op implementation, a Makefile with build, test, lint (golangci-lint), and integration targets, GitHub Actions running unit tests on push and integration tests with Postgres via testcontainers-go, goreleaser config producing darwin/linux/windows binaries plus a Homebrew tap, SECURITY.md, CONTRIBUTING.md, a docs/adr/README that explains the ADR format, and .gitignore containing snapshots/ and lazysnap.secret. Everything must pass green on an empty implementation before you stop.
```

#### PROMPT 3.2 · CLAUDE.md

```PROMPT 3.2 · CLAUDE.md
Write CLAUDE.md for this repo using the "Standing rules for the agent" block from the build plan as the base. Add: the pipeline stage names and the rule that a change touches one stage unless an ADR says otherwise; the commands to run before claiming anything works (make lint test integration); the location of ADRs and the rule that any change to a decision requires a new ADR, never an edit; the invariants that tests enforce; and the current phase with a pointer to ROADMAP.md. Keep it under 80 lines. Nothing aspirational, only rules you will actually enforce.
```

#### PROMPT 3.3 · golden fixtures

```PROMPT 3.3 · golden fixtures
Build testdata/ with two Postgres fixtures loaded by the integration suite. First, Pagila (the Postgres port of Sakila) as the friendly realistic schema. Second, a hostile schema you design, nasty.sql, that contains: a self-referencing table, a three-table FK cycle, a composite primary key, a polymorphic association with no FK constraint, a JSONB column containing emails and phone numbers, a free-text notes column with names in it, a column named "email_verified" that is a boolean (a false positive trap), a column named "ref" that holds emails (a false negative trap), a partitioned table, an enum, an array column, a generated column, a table with 2M rows for streaming tests generated on load, and identity columns with non-default sequences. Document every trap in testdata/README.md with the behaviour we expect from the tool.
```

#### PROMPT 3.4 · invariant tests

```PROMPT 3.4 · invariant tests
Write the invariant test suite in internal/invariants_test.go that runs against any snapshot the tool produces, using both fixtures. Invariants: (1) every foreign key in the target resolves; (2) no value in the target matches the classifier's own PII detectors for any column the classifier flagged; (3) running twice with the same secret produces byte-identical targets; (4) the source database's row counts and a checksum are unchanged after a run; (5) the emitted lazysnap.yml, fed back in, reproduces the snapshot; (6) the target row count for the root table equals --take. These tests are the definition of correct. Make them fail now, since nothing is implemented, and wire them into make integration.
```

#### PROMPT 3.5 · roadmap

```PROMPT 3.5 · roadmap
Write ROADMAP.md from the build plan phases 4 to 8. For each phase: the goal in one sentence, the gate as a checklist, and a "not in this phase" list. Add a top section "Current phase" that I will update by hand. Add a rule at the top: any feature request goes into a "Later" section at the bottom with a one-line reason, and nothing moves out of Later without the current phase gate being ticked.
```

### Gate 3

- CI is green on an empty implementation, and integration tests fail for the right reason.
- `brew install` from your tap installs a binary that prints its version.
- Both fixtures load, and testdata/README.md names every trap.

## Phase 4: Vertical slice

**Weeks 5 to 8** | Output: the one command works, Postgres to Postgres

Build the pipeline one stage at a time, in order, each behind a flag that runs it alone. The TUI comes last and only wraps what already works headless. Resist adding a second database. Resist adding a web anything.

### Prompts

#### PROMPT 4.1 · introspect

```PROMPT 4.1 · introspect
Implement internal/introspect for Postgres: tables, columns with types, primary keys, foreign keys including composite and self-referencing, unique constraints, sequences and identity columns, partitions, enums, approximate row counts from pg_class, and a 200-row sample per table taken with TABLESAMPLE where available. Output the Schema type from ARCHITECTURE.md. Add `lazysnap introspect --from URL --json` that prints it. Write tests against both fixtures that assert every trap in testdata/README.md is represented in the output. Do not touch any other stage.
```

#### PROMPT 4.2 · classify

```PROMPT 4.2 · classify
Implement internal/classify per docs/adr/003. Input: Schema with samples. Output: per column, a category, a confidence, and a human-readable reason. Add `lazysnap classify --from URL` that prints a table sorted by confidence. Tests: on nasty.sql, email_verified must not be flagged, ref must be flagged, the JSONB and notes columns must be flagged as free text, and precision and recall on Pagila must be reported as numbers in the test output. Add a fixture of 50 real-world column names from open-source schemas (Discourse, GitLab, Mastodon) with expected categories, and make the test print the confusion matrix.
```

#### PROMPT 4.3 · plan

```PROMPT 4.3 · plan
Implement internal/plan per ARCHITECTURE.md. Given Schema, root table, and N, produce an ordered Plan of steps. Walk parents to completeness, then children with a per-table cap defaulting to 10x N. Handle cycles by marking constraints for deferral. Tables unreachable from the root are excluded unless they are small lookup tables (under 1000 rows, no FKs out), which are copied whole. Add `lazysnap plan --from URL --root T --take N` that prints the plan with estimated row counts. Tests on nasty.sql must show the cycle handled and the polymorphic association reported as "not followed: no constraint" with a hint.
```

#### PROMPT 4.4 · extract and transform

```PROMPT 4.4 · extract and transform
Implement internal/extract and internal/transform. Extract streams each plan step with a server-side cursor and COPY TO where possible, never materialising a full table in memory; prove it with a test on the 2M row table under a memory cap. Transform applies maskers per docs/adr/004 row by row with the HMAC determinism scheme, and guarantees unique constraints on masked columns survive. Wire the invariant tests for PII absence and determinism. Add a progress event stream the TUI will consume later.
```

#### PROMPT 4.5 · load and safety rails

```PROMPT 4.5 · load and safety rails
Implement internal/load for Postgres: create schema in the target if missing, load with COPY FROM in plan order with constraints deferred, reset sequences to max+1, and verify FK integrity at the end. Then implement every v1-blocking control from THREAT_MODEL.md, at minimum: refuse a target that resembles the source, refuse a target with existing data unless --replace, never log or emit secrets, write snapshots/ to .gitignore. Now `lazysnap --from URL --to URL --root T --take N` must run end to end and pass all six invariants on both fixtures. Emit lazysnap.yml at the end of the run.
```

#### PROMPT 4.6 · first run and docker

```PROMPT 4.6 · first run and docker
Implement docs/adr/006: with no arguments, detect Postgres containers via the Docker socket, read DATABASE_URL from .env and docker-compose.yml, and offer them as source and target candidates. If exactly one plausible source and one plausible target exist, ask the single question (root table, default: the table with the most incoming FKs) and go. Implement the Bubble Tea TUI as a thin layer: a table picker showing the classifier's reasons, a plan preview, and a live progress view fed by the event stream. Every action must have a visible keybinding hint. No feature may exist in the TUI that is not reachable by a flag.
```

#### PROMPT 4.7 · dogfood

```PROMPT 4.7 · dogfood
I am going to run lazysnap for the first time on a real project, pretending I have never seen it. Before I do, write docs/DOGFOOD_LOG.md with a template: what I typed, what I expected, what happened, how long it took, what I had to look up. After each session I will paste my notes. Your job then is to turn every "had to look up" into either a default, a better prompt in the tool, or a Later item, and to never suggest documentation as the fix.
```

### Gate 4 · the demo gate

- Pagila, 200 customers, from one Docker container to another, in under 60 seconds, with zero flags beyond the root table.
- All six invariants pass on both fixtures in CI.
- Two dogfood sessions logged, and the second needed nothing looked up.
- A 20-second GIF exists that shows the whole thing. If it is not impressive, the product is not done.

## Phase 5: Hardening

**Weeks 9 to 10** | Output: it survives strangers' schemas

The demo works on your fixtures. Now find the schemas that break it before users do, and make the failure modes gentle.

### Prompts

#### PROMPT 5.1 · schema torture

```PROMPT 5.1 · schema torture
Collect ten real open-source Postgres schemas of different shapes (Discourse, GitLab, Mastodon, Odoo, Metabase, Supabase's auth schema, Cal.com, Plausible, a Django default, a Rails default with ActiveStorage). Add a make target that loads each with a small amount of generated data and runs a snapshot from its most-connected table. For every failure, write a minimal reproduction into testdata/regressions/ before fixing it. Report a table: schema, tables, run time, failures found, PII columns detected versus a hand-labelled truth set for three of them.
```

#### PROMPT 5.2 · failure UX

```PROMPT 5.2 · failure UX
Audit every error path in the tool. For each, the message must say what went wrong, which table or column, and the exact flag or action that fixes it. No stack traces reach the user without --debug. Partial failures must leave the target either empty or complete, never half-loaded; implement that as a transaction or a drop-on-failure. Write the error catalogue to docs/ERRORS.md and add a test that every error type in the code appears in it.
```

#### PROMPT 5.3 · performance

```PROMPT 5.3 · performance
Profile a snapshot of 5,000 root rows from a source with a 20M row child table. Report where time goes per stage. Target: under 3 minutes on a laptop over a local network. Optimise only the top two hotspots, write the before and after numbers into docs/PERF.md, and add a benchmark to CI that fails if extract throughput drops more than 20% from the recorded baseline.
```

#### PROMPT 5.4 · red team

```PROMPT 5.4 · red team
Act as a security reviewer who wants to find a way for lazysnap to leak production data. Try: PII in column names we do not check, values that dodge every regex, PII inside array and JSON columns three levels deep, a masker that throws halfway, a target URL that is prod with a different hostname alias, secrets in the emitted YAML, snapshots written to a synced folder, and verbose logging. For each attempt, show the reproduction, whether it succeeded, and the fix. Update THREAT_MODEL.md with anything new. Do not fix anything until the whole list is written.
```

### Gate 5

- Nine of ten torture schemas snapshot cleanly, and the tenth fails with a message that says exactly why.
- Red team found nothing that leaks unmasked flagged data.
- Performance baseline is recorded and enforced in CI.

## Phase 6: Launch

**Weeks 11 to 12** | Output: v0.1.0 in public, first thousand stars

sqlit launched with a README GIF and a Reddit post, not a website. The README is the landing page. Spend a full week on it.

### Prompts

#### PROMPT 6.1 · README

```PROMPT 6.1 · README
Rewrite README.md as the landing page. Structure, in order: the GIF; one sentence on what it does; the install line for brew, curl, and go install; the one-command example with real output; "why" in three short paragraphs matching the three principles; a comparison table against Greenmask, pg_anonymizer, and Tonic on the axes users care about (time to first snapshot, config required, databases, masking determinism, licence); how it decides what is PII and how to override it; the safety rails; a Roadmap link; and a "why I built this" paragraph I will write. Study the sqlit, lazygit, and Bruno READMEs first and note what they do in the first screen.
```

#### PROMPT 6.2 · GIF and docs

```PROMPT 6.2 · GIF and docs
Write a VHS tape file (charmbracelet/vhs) that records the first-run flow on the Pagila fixture at 1200x700, under 25 seconds, with a pause on the classifier's reasons screen. Then generate a minimal docs site with mkdocs-material: install, first run, CI usage with a GitHub Actions example, masking overrides, custom maskers, every flag, and the error catalogue. Nothing on the site may contradict the tool's own --help output; add a test that checks that.
```

#### PROMPT 6.3 · launch posts

```PROMPT 6.3 · launch posts
Draft launch posts in docs/launch/: a Show HN title and first comment under 200 words that leads with the problem and the Snaplet and Neosync shutdowns; a r/PostgreSQL post; a r/devops post; a r/webdev post; and a short blog post "What I learned reading Snaplet's and Neosync's issue trackers" drawing on research/POSTMORTEMS.md. Each post must be honest about v1 limits (Postgres only) and end with one specific question to readers. Also list ten awesome-lists and comparison pages where a PR adding lazysnap would be welcome, with links.
```

#### PROMPT 6.4 · release

```PROMPT 6.4 · release
Cut v0.1.0: run the full gate 5 checklist, tag, let goreleaser publish binaries and the Homebrew formula, verify a fresh machine installs and runs the Pagila demo from the README with copy-paste alone, and write CHANGELOG.md. Then set up issue templates (bug with schema attachment instructions, PII miss with a private reporting route via SECURITY.md, feature request that must name which principle it serves) and labels including good-first-issue.
```

### Gate 6

- Posted on HN and two subreddits. Every comment answered within a day for a week.
- A stranger has opened an issue with a schema you have never seen.
- The first ten issues are triaged into ROADMAP.md, and at least half went to Later.

## Phase 7: Breadth and community

**Months 4 to 6** | Output: MySQL, SQLite, SQL Server, a GitHub Action, contributors

Now the sqlit trick: breadth is where AI-assisted building pays off. Each new database is the same six interfaces with a different driver, and the invariant suite already knows what correct means.

### Prompts

#### PROMPT 7.1 · adapter playbook

```PROMPT 7.1 · adapter playbook
Write docs/ADDING_A_DATABASE.md from the Postgres implementation: which of the six stages need a driver-specific implementation, which are shared, the introspection queries needed, the bulk-load mechanism (LOAD DATA, BCP, bulk insert), how to port the fixtures, and the checklist for calling an adapter done (all six invariants, torture suite, docs page, README table row). Then implement MySQL following the playbook exactly, and improve the playbook wherever it was wrong.
```

#### PROMPT 7.2 · CI mode

```PROMPT 7.2 · CI mode
Build a GitHub Action, lazysnap-action, that runs a committed lazysnap.yml against a source in secrets and produces a target service container for the job's tests. Include caching of the snapshot keyed on the config hash and the source schema hash so unchanged schemas reuse the last snapshot. Write the example workflow into the docs and dogfood it on our own repo against a Pagila source.
```

#### PROMPT 7.3 · weekly triage

```PROMPT 7.3 · weekly triage
Run the weekly triage. Read all issues and PRs opened or updated in the last seven days. For each: label it, decide phase or Later per ROADMAP.md, draft a reply, and flag any report of unmasked PII as urgent. List PRs that pass CI and follow CONTRIBUTING.md and are safe for me to merge after reading, versus those that change an ADR decision and need a conversation. Output as a checklist I work through, not as actions taken.
```

### Gate 7

- Four databases pass the invariant suite.
- At least three merged PRs from people you have never met.
- Someone you do not know has written about the tool without being asked.

## Phase 8: Client-facing product

**Months 6 to 12** | Output: five design partners, a hosted tier, first invoice

The hosted product sells what a CLI cannot: shared team snapshots, schedules, access control, and a compliance record that proves what was masked. Do not build any of it until five real teams have told you which of those they would pay for.

### Prompts

#### PROMPT 8.1 · design partners

```PROMPT 8.1 · design partners
Write docs/business/DESIGN_PARTNERS.md: a 30-minute interview script for engineering leads at 5 to 50 person companies using lazysnap. Questions must uncover: how they get dev data today, who is responsible when PII leaks, whether they need snapshots shared across a team, how often schemas change, what they pay for adjacent tools, and what would make them pay for a hosted version. Include a scoring rubric and a target of five interviews. Then draft the outreach message for the GitHub users who opened the most detailed issues.
```

#### PROMPT 8.2 · open-core boundary

```PROMPT 8.2 · open-core boundary
Write docs/adr/010-open-core-boundary.md. List every capability we have or plan, and sort each into "always free in the CLI" or "hosted only" using one test: does it require infrastructure we run, or a guarantee only we can make? Scheduling, storage, team sharing, audit and compliance reports, SSO, and support go one way; every masker, every database, and CI mode stay free. Specify how the repo is structured so the boundary is visible in the code, and how the CLI talks to the hosted service through a documented API that a self-hoster could reimplement.
```

#### PROMPT 8.3 · hosted architecture

```PROMPT 8.3 · hosted architecture
Design the hosted service in docs/business/HOSTED_ARCHITECTURE.md for the first 50 paying teams, optimising for one person operating it. Components: auth via GitHub and Google, a runner that executes lazysnap.yml against customer sources on a schedule from a fixed egress IP they can allow-list, encrypted snapshot storage in S3-compatible object storage with per-team keys, a small web app for browsing snapshots and masking reports, and Stripe billing with flat per-team pricing. State the data we hold, the data we never hold, and the deletion guarantee. Choose boring technology and justify each choice in one line.
```

#### PROMPT 8.4 · pricing and paperwork

```PROMPT 8.4 · pricing and paperwork
Write docs/business/PRICING.md and a checklist for going commercial. Pricing: propose three flat team tiers with the value metric being snapshot schedules and retained snapshots, never seats; justify against PagerDuty-style per-seat resentment and Tonic's enterprise price. Paperwork checklist: legal entity options for a solo founder, a terms of service and privacy policy tailored to a tool that touches customer PII, a data processing agreement template, what SOC 2 actually requires and when to start, and cyber insurance. Mark which items need a lawyer versus a template.
```

### Gate 8

- Five interviews scored, and at least three named the same hosted feature.
- One design partner runs the hosted tier in anger for a month.
- First invoice paid.

## Guardrails

Every session, every phase

### Invariants the tests enforce

These are the definition of correct. A pull request that breaks one is wrong, whoever wrote it and however nice the feature.

| Key | Invariant |
|-----|-----------|
| I1 | Every foreign key in the target resolves. |
| I2 | No flagged column in the target contains a value the classifier would flag. |
| I3 | Same source, same secret, same config produces a byte-identical target. |
| I4 | The source is unchanged after a run. The tool holds no write privilege on it. |
| I5 | The emitted config reproduces the run. |
| I6 | The root table in the target has exactly the requested number of rows. |

### Standing rules for the agent

The core of CLAUDE.md. Prompt 3.2 expands it.

```CLAUDE.md · core block
# lazysnap rules

Read ROADMAP.md first. Work only on the current phase. Anything else goes to Later with one line of reasoning; do not build it.

Before saying something works: make lint test integration. Paste the result. "Should work" is not a status.

Invariants I1 to I6 in internal/invariants_test.go define correct. Never weaken a test to make it pass. If an invariant is wrong, write an ADR proposing the change and stop.

One stage per change. A change touching more than one pipeline stage needs a sentence in the PR explaining why.

Decisions live in docs/adr/. To change a decision, add a new ADR that supersedes the old one. Never edit an accepted ADR.

Safety rails from THREAT_MODEL.md are not configurable away without a flag whose name says what it does. Never add a flag that disables masking wholesale.

The TUI is a thin layer. Every TUI action must be reachable by a CLI flag first.

Do not add a database, a config option, or a dependency to fix a bug. Fix the bug.

Documentation is never the fix for a confusing first run. Change the default or the question.

When in doubt, mask it.
```

### Session prompts

#### Start of every session

```Start of every session
Read CLAUDE.md, ROADMAP.md, and the last entry in docs/DOGFOOD_LOG.md. Tell me the current phase, what the gate still needs, and the single most valuable thing to do in the next two hours. Then wait for me to agree before writing code.
```

#### Scope check, when a feature feels tempting

```Scope check, when a feature feels tempting
I want to add: [describe it]. Before anything else: which phase in ROADMAP.md does this belong to, which principle does it serve, and what in the current gate does it unblock? If the answer to the last is "nothing", write it into Later with my reasoning and tell me no. Be blunt.
```

#### Before merging anything

```Before merging anything
Review this diff as a maintainer who will be paged if it leaks production data. Check: does it weaken any invariant, does it add a way to skip masking, does it log or persist a secret, does it write to the source, does it touch more than one stage, and does the error path leave a half-loaded target? Then check the boring things: tests for new behaviour, docs updated, --help still matches docs. List findings by severity. Do not fix anything yet.
```

#### Friday review

```Friday review
Compare this week's commits against ROADMAP.md's current phase. What moved the gate forward, what did not, and what got built that was not in the phase? Update the "Current phase" section with what is left. Then give me one honest sentence about whether I am on track for the phase's week range, and if not, what to cut rather than what to add.
```

#### When stuck for more than an hour

```When stuck for more than an hour
I have been stuck on [problem] for over an hour. Do not propose a fix. First write down the three simplest explanations, the one-line experiment that would rule out each, and which ADR or research doc is relevant. Then run the cheapest experiment.
```

### What "done" means at every level

| Level | Done means |
|-------|-----------|
| A change | Lint, tests, and integration green. Invariants untouched. One stage. Docs match --help. |
| A phase | Every gate item ticked by evidence, not by feel. ROADMAP.md updated. |
| A release | Fresh machine, README copy-paste, demo runs. CHANGELOG written. Tag pushed. |
| The product | A stranger's team pays for the hosted tier and renews. |
