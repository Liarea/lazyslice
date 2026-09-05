# .claude/

`workflows/` only: one resumable workflow script per phase (`research.js`,
`architecture.js`, `foundations.js`, `slice.js`, `hardening.js`), plus
`implement.js`, the shared one-task runner every phase workflow calls into.
No product code, no docs content — this is orchestration only.

**Contract.** Each workflow exports `meta` (`name`, `description`, `phases`)
and reads `args.step` to resume at a named step, matching root CLAUDE.md's
phase list and `docs/BUILD_PLAN.md`'s gates. `implement.js` is the contract
every phase task in a workflow runs through — a task spec's shape
(`id`, `title`, `model`, `paths`, `brief`, optional `reviewers`/`checks`) is
defined by how `implement.js` consumes it, not restated per workflow.

**Rules.**
- One workflow file per phase; a phase's steps resume independently (`args:
  { step: '...' }`) so a usage-window interruption can restart at the last
  completed step, not phase 0.
- A task's `paths` list is the authorization boundary a spawned agent gets —
  it must match what root CLAUDE.md and the task's own brief say the agent may
  touch; do not widen it to make a task "easier."
- `implement.js` is shared infrastructure: a change to it affects every
  workflow's tasks, so treat it like a library API, not one phase's script.
- `implement.js` itself commits each task after independent verification
  succeeds (`git add -A && git commit`) — that is the orchestrator committing,
  per root CLAUDE.md, not an exception to it. A developer agent never commits
  itself; its brief says "Do not commit." The scaffold brief is the one place
  a developer agent is told to run git commands directly, and it says so
  explicitly.

**Test.** No automated test suite; verify a workflow change by resuming a
step in a scratch run and checking the tracker (`tools/tracker.py list`)
reflects what actually happened.

**Never:** let a task's `paths` exceed what its brief and root CLAUDE.md
authorize; hardcode a phase's steps in a way that can't resume after an
interruption; let a developer agent commit itself instead of leaving that to
`implement.js`'s own verify-phase commit step.
