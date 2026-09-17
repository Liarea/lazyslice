# Operating model

How this project is built by one orchestrator and a fleet of agents, with a human who checks in rarely.

## Roles

| Role | Who | Does |
|---|---|---|
| Orchestrator | Claude Fable 5.1, the interactive session | Architect, PM, lead dev, lead ops. Makes decisions, writes ADRs, runs workflows, merges, owns the tracker. |
| Researcher | Fable or Opus agents | Web research, competitor teardowns, post-mortems. Writes one file, returns a summary. |
| Developer | Opus (core pipeline) or Sonnet (peripheral) agents | Implements one task in one stage. Runs lint and tests before returning. |
| Reviewer | Opus agents, three lenses in parallel | Correctness, security and invariants, scope. Returns findings, never fixes. |
| Mechanic | Haiku agents | Format conversions, boilerplate, renames, doc regeneration. |
| Human | the maintainer | Sets direction, unblocks anything needing accounts, money, or public posting. |

## Model tiers

| Work | Model | Effort | Why |
|---|---|---|---|
| Research synthesis, architecture decisions, ADRs, threat model | Fable | high | Judgment under uncertainty; the cost of a wrong decision compounds. |
| Individual research documents, sqlit study, name check | Opus | high | Long web research with judgment, no irreversible decision. |
| Core pipeline code: introspect, classify, plan, extract, transform, load | Opus | high | Streaming, constraints, and masking are where subtle bugs leak data. |
| Fixtures, CI, docs, tests scaffolding, TUI wiring, adapters after the first | Sonnet | medium | Well-specified work with a clear definition of done. |
| Reviews: correctness, security, scope | Opus | high | A cheap reviewer that misses a leak is worse than no reviewer. |
| Format conversion, boilerplate, tracker board regeneration | Haiku | low | Mechanical. |

Correction, 2026-09-09 (the maintainer): phase 5 ran an Opus developer plus three Opus reviewers on nearly every task and burned the weekly allowance. The defaults are now a Sonnet developer and one Opus reviewer; a task opts in to Opus or to three reviewers only for masking, verify, and source-safety logic. Mechanical work goes to Haiku or Sonnet. Any single run expected to exceed about one million tokens is confirmed with the maintainer first.

## Budget reality

The human's plan has session usage limits that reset at fixed times. A phase-1 research run of 29 agents cost about 3.5 million subagent tokens and was interrupted twice. Rules that follow from this: every workflow must be resumable by run id and commit partial output before resuming; observed windows are five hours long and have held between 0.6 and 3.6 million subagent tokens depending on what ran earlier in the window, so a single run should aim for under 12 agents or about 2 million tokens, and the orchestrator does not start a second heavy run in the same window; reviewers stay on Opus, but revision and fix rounds that apply a concrete finding list run on Sonnet; research agents that spot-check links do so on at most five links, not every one.

## Review policy

Every implementation task is followed, in the same workflow, by one Opus reviewer reading through a merged correctness-and-safety lens (three parallel reviewers with distinct lenses only when the task opts in; see the correction above), then at most two fix rounds by the original developer, then a final verify. A task that still fails after two rounds is returned to the orchestrator as blocked with the findings, not merged.

Tasks are capped at roughly 400 changed lines so review happens in real time. Bigger work is split before it starts.

## Parallelism

Work runs in parallel only when it touches disjoint directories. The six pipeline stages are built sequentially because each consumes the previous stage's types. Fixtures, CI, docs, and tests can run alongside any stage. Parallel agents whose write paths are disjoint share the working tree and may not touch go.mod, go.sum, or each other's paths. Only when two parallel tasks must touch the same files, as with adapters in phase 7, does each get its own git worktree and branch, and the orchestrator merges.

## Tracking

`tracker/` holds epics and tasks as markdown with frontmatter. `tools/tracker.py` is the only writer, and only the orchestrator runs it. Agents return structured results that include a one-line post-mortem; the orchestrator records it on close. `tracker/BOARD.md` is regenerated on every write. A task is never closed without a post-mortem; a cancelled task carries its reason.

## Commit discipline

Every commit that changes behaviour carries a body of release-note bullets under a `stage: title (T-id)` headline (the maintainer, 2026-09-15: one-line ticket references were unusable for release notes); `.claude/skills/lazyslice-commit` is the format, `implement.js` writes it from the developer's `changelog` field, and `make relnotes` generates the notes between two refs. While a workflow that commits is running, the orchestrator never runs `git add -A`. It stages the exact paths it changed (`git add tracker/ docs/RUNBOOK.md`) so an agent's half-written files are not swept into an unrelated commit. In parallel steps only the orchestrator commits, once, after the batch, so every task's commit is attributable. Observed failure, 2026-09-05: two tracker commits absorbed the invariants suite mid-task.

Documentation keeps step with the code in three tiers (the maintainer, 2026-09-17, after the README sat untouched for 81 commits). Reference documents change in the task that changes the behaviour: ARCHITECTURE.md, THREAT_MODEL.md, docs/TORTURE.md and the package CLAUDE.md files are in every task's paths for that reason, and docs/FLAGS.md, docs/ERRORS.md and the README's flag table are generated by `make docs` and compared by `make docs-check`, so a stale one fails CI. Reader-facing documents, README.md and SECURITY.md, change in the task that adds or changes a flag, an exit code or a first-run behaviour they describe, and are read end to end at every gate close and before every tag; the release workflow refuses a tag whose README does not name it. ROADMAP.md is the orchestrator's and changes at every landing that moves a gate item.

A push is not finished until CI has answered for it (2026-09-17, after main sat red for two days across twenty landings: a lint finding that only fires on Linux, five tests that only fail on Windows and one that only fails in the Postgres 14 leg, none of which a local `make check` on macOS can see). The orchestrator reads `gh run list --branch main --limit 1` before the next landing; a red main is the next task, ahead of whatever was queued, and the landing that broke it is named in the fix's commit. The release workflow already refuses a tag whose commit CI did not pass, so a red main is also a release that cannot be cut.

## Context discipline

Agents write their work to files and return a summary under 200 words plus structured fields. The orchestrator reads summaries, not whole documents, and reads a file only when making a decision that depends on its contents. Decisions live in `docs/adr/` so they survive compaction. If the session is compacted, the summary must keep: current phase and gate status, open task ids, unresolved review findings, and any ADR under discussion.

## Prompting

`docs/prompting/` holds one operational cheat sheet per model. Every agent prompt carries: a role, the repo path, the exact files it may write, the definition of done, the return schema, an instruction to touch nothing else, and an instruction to finish without asking. Prompts to Sonnet and Haiku are more literal and more explicit about output shape; prompts to Fable and Opus give the goal and the constraints and trust the method.

## Workflows

`.claude/workflows/` holds one script per phase. Each is resumable: an edited script re-runs only the changed agents. The orchestrator runs them in order and reads each result before starting the next, so a human can also run them one at a time.

## What needs the human

Creating the GitHub repository and pushing. Publishing posts. Buying a domain. Any account, payment, or legal step. Everything else is the orchestrator's call, recorded in an ADR or the tracker.
