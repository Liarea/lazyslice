# Runbook

How to drive this project, for the orchestrator session or a human picking it up cold.

## Where am I

```
cat CLAUDE.md | sed -n '/Current phase/,/^## Rules/p'   # phase and gate
python3 tools/tracker.py list --status open             # what is unfinished
python3 tools/tracker.py list --status blocked          # what needs a decision
cat tracker/BOARD.md                                    # the board
```

## Phase order and the workflow that runs it

| Phase | Workflow | Runs | Human needed |
|---|---|---|---|
| 0 Frame | written by hand | CONCEPT.md, OPERATING_MODEL.md, tracker | no |
| 1 Research | `.claude/workflows/research.js` | 8 docs, critique, revise, 5 guides, synthesis | no |
| 2 Architecture | `.claude/workflows/architecture.js` | benchmark, 3 proposals, 3 judges, ADRs, review | no |
| 3 Foundations | `.claude/workflows/foundations.js` with `{step}` | `scaffold`, `fixtures` (fixtures, ADR-008), `docs` (CLAUDE.md files, roadmap, ADR-008 sync), `invariants`, `review`; one step per window | no |
| 4 Vertical slice | `.claude/workflows/slice.js` | ten packages in dependency order through implement.js, stops at first block; resume by run id | no |
| 5 Hardening | `.claude/workflows/hardening.js` with `{step}` | `features` (TUI, provisioning, polymorphic, CI matrix), `harden` (torture, failure UX, perf), `redteam` | no |
| 6 Launch | drafts only | README, GIF, posts | yes: create GitHub repo, post |
| 7 Breadth | implement.js per adapter, worktrees | MySQL, SQLite, SQL Server | no |
| 8 Client-facing | research only | interviews, hosted design, pricing | yes: everything commercial |

Run a phase from the orchestrator session with the Workflow tool and `scriptPath` pointing at the script. Each run reports a run id; to resume after an edit or interruption, pass `resumeFromRunId` and unchanged agents return cached results.

`implement.js` is the unit of work everywhere from phase 3 on. It takes `args {id, title, brief, model, paths, stage, integration}` and returns `merged` or `blocked` with findings. `slice.js` accepts `args {from: n}` to resume from stage n.

## After every workflow

1. Read the returned summaries, not the files, unless a decision depends on file contents.
2. Record outcomes: `python3 tools/tracker.py close T-xxxx --outcome ... --postmortem "..."`, or `block` with the findings.
3. Update the Current phase line in CLAUDE.md when a gate is met.
4. Commit: `git add -A && git commit -m "phase N: ..."`.
5. Read the gate in docs/BUILD_PLAN.md before starting the next phase. A gate item without evidence is not ticked.

## When a run dies on a usage limit

Agents fail with "You've hit your session limit" and the workflow returns with a failures list. Nothing is lost: files already written stay on disk and finished agents are cached. After the limit resets, relaunch with the same `scriptPath` and `resumeFromRunId`; only the failed agents run. Commit the partial output first so a later edit to the script cannot orphan it.

## When a task comes back blocked

Read the findings. Decide one of: split the task and rerun; write an ADR if the block is a design flaw; or cancel with a reason in the tracker. Never weaken a test or an invariant to unblock.

## Model routing

See docs/OPERATING_MODEL.md. Do not downgrade reviewers to save tokens.

## What only the human can do

Create the GitHub repository and push. Publish posts. Buy a domain. Anything involving an account, payment, or legal step. Leave these as open tasks in the tracker with owner "human".
