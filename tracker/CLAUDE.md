# tracker/

Epics (`epics/E*.md`), a closed-history archive (`tasks/T-NNNN-*.md`), and
`BOARD.md`. This is process state, not product code — nothing here is a type
in ARCHITECTURE.md; its contract is `tools/tracker.py`'s CLI, and the process
rules are in root `CLAUDE.md` and `docs/OPERATING_MODEL.md`.

Since T-0196, a task that is open, in progress or blocked is a GitHub issue on
Liarea/lazyslice plus a card on Project 3, not a file — `tasks/` holds only a
task that has closed or been cancelled, written once, at that moment, and
never touched again. `epics/` is unaffected; an epic is still a hand-managed
file here (there is no per-epic GitHub object).

**Contract.** `tools/tracker.py` is the only writer, and for anything GitHub
holds it runs over the `gh` CLI: `list`, `show T-0001`, `new --epic E1 --title
"..." --owner opus`, `close T-0001 --outcome done --postmortem "..."`. A
closed task's frontmatter (`id`, `title`, `epic`, `phase`, `status`, `owner`,
`created`, `started`, `closed`, `outcome`) is written once by `close` or
`cancel`, reconstructed from the issue's fields, body and comments at the
moment it closes; `BOARD.md`'s summary table is regenerated from GitHub plus
this archive on every mutating command, never hand-typed.

**Rules.**
- Written only via `tools/tracker.py`, by the orchestrator (root CLAUDE.md) —
  a subagent or task does not hand-edit an archived file's status, `BOARD.md`,
  or an epic file directly, even to "fix" something that looks wrong; it
  reports the discrepancy instead. An archived file, once written, is never
  edited again, by hand or by the tool.
- A task closes with a postmortem in the fixed shape: "went well | went badly
  | change next time" (root CLAUDE.md's `tracker` section).
- Out-of-phase work becomes a new task in a later epic with one line of
  reasoning, never built inline (root CLAUDE.md "Rules").
- An epic file records scope and gate, not a running log — task-level detail
  belongs in the task's own log, which for an open task is now its issue's
  comments (`tools/tracker.py show T-NNNN`), not a `## Log` section in a file.

**Test.** `python3 tools/tracker.py list` to check the tool still reads
GitHub and every archived file here without error; `python3 -m unittest
discover -s tools -p 'test_*.py'` (`make tools-test`) is the tool's own test
suite, against a fake `gh`.

**Never:** hand-edit an archived task's fields or `BOARD.md` outside
`tools/tracker.py`; write to a task file that has already closed; close a
task without a postmortem; build phase-N+1 work here instead of filing it as
a task.
