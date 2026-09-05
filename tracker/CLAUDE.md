# tracker/

Epics (`epics/E*.md`), tasks (`tasks/T-NNNN-*.md`), and `BOARD.md`. This is
process state, not product code — nothing here is a type in ARCHITECTURE.md;
its contract is `tools/tracker.py`'s CLI and file format, and the process
rules are in root `CLAUDE.md` and `docs/OPERATING_MODEL.md`.

**Contract.** `tools/tracker.py` is the only writer: `list`, `new --epic E1
--title "..." --owner opus`, `close T-0001 --outcome done --postmortem "..."`.
A task file's frontmatter (`id`, `title`, `epic`, `phase`, `status`, `owner`,
`created`, `started`, `closed`, `outcome`) and `BOARD.md`'s summary table are
both derived from that tool's writes, not hand-typed.

**Rules.**
- Written only via `tools/tracker.py`, by the orchestrator (root CLAUDE.md) —
  a subagent or task does not hand-edit a task file's status, `BOARD.md`, or
  an epic file directly, even to "fix" something that looks wrong; it reports
  the discrepancy instead.
- A task closes with a postmortem in the fixed shape: "went well | went badly
  | change next time" (root CLAUDE.md's `tracker` section).
- Out-of-phase work becomes a new task in a later epic with one line of
  reasoning, never built inline (root CLAUDE.md "Rules").
- An epic file records scope and gate, not a running log — task-level detail
  belongs in the task file's own `## Log`.

**Test.** `python3 tools/tracker.py list` to check the tool still reads every
file here without error; there is no other test suite for this directory.

**Never:** hand-edit a task's `status`/`outcome` fields or `BOARD.md` outside
`tools/tracker.py`; close a task without a postmortem; build phase-N+1 work
here instead of filing it as a task.
