---
name: lazyslice-tracker
description: File, start, log, block, move, cancel and close tracker tasks with tools/tracker.py; how to word a goal a developer can act on and a post-mortem worth reading. Use whenever a task or epic changes state, and never edit tracker files by hand.
---

# Tracker operations

GitHub is the source of truth for a task that is open, in progress or blocked (T-0196): it lives as an issue on Liarea/lazyslice and a card on Project 3, and `tools/tracker.py` is the only thing that writes it, over the `gh` CLI. `tracker/tasks/` is a read-only archive — only a closed or cancelled task has a file there, written once at the moment it closes, and never edited again by hand or by the tool. Never edit an archived file, and never regenerate `tracker/BOARD.md` by other means; `tools/tracker.py` rewrites it, from GitHub plus the archive, on every mutating command. A task is no longer a file an agent can read — use `show`, below, in place of reading `tracker/tasks/T-NNNN-*.md`.

A mutating command needs `gh` installed and authenticated (`gh auth login`; the board also needs `gh auth refresh -s project`) and refuses, changing nothing, if it is not — one sentence naming which. `list` and `show` fall back to the archive plus the last `tracker/BOARD.md` instead, and say they did.

## Commands

```
python3 tools/tracker.py new --epic E5 --phase 5 --owner sonnet --title "..." --goal "..." [--accept "..."]
python3 tools/tracker.py start T-0187
python3 tools/tracker.py log T-0187 "2026-09-15 what changed and why"
python3 tools/tracker.py block T-0187 --reason "..."
python3 tools/tracker.py move T-0187 --epic E5 --phase 5
python3 tools/tracker.py cancel T-0187 --reason "..."
python3 tools/tracker.py close T-0187 --outcome done --postmortem "went well: ... | went badly: ... | change next time: ..."
python3 tools/tracker.py list [--status open|blocked|done|cancelled] [--epic E5]
python3 tools/tracker.py show T-0187
python3 tools/tracker.py validate
```

`start`, `log`, `block`, `move`, `close` and `cancel` print the issue's URL (or, for `close`/`cancel`, the archive file path they just wrote) rather than a tracker-file path — there usually is no file until a task closes. `log`'s note becomes an issue comment; `close`'s and `cancel`'s post-mortem or reason becomes the comment that closes the issue.

`new` prints the id it assigned; read it back before referencing it anywhere. Ids are assigned in creation order and a running developer may file a task between two of yours (this happened on 2026-09-14: three ids shifted), so never predict an id.

Epics: E0 Frame, E1 Research, E2 Architecture, E3 Foundations, E4 Vertical slice, E5 Hardening, E6 Launch, E9 Later. `--phase` is the roadmap phase the task counts toward; a task in E9 has no phase. Owner is the model tier expected to do it (`haiku`, `sonnet`, `opus`, `fable`) or `human`.

## Who may do what

- The orchestrator files, starts, logs, moves, blocks, cancels and closes.
- A developer or reviewer agent may only file, and only into E9 (`new --epic E9`), for work someone must do outside its paths. Root CLAUDE.md calls this the one write outside a task's paths. The orchestrator re-homes it (`move`) when triaging.
- Closing needs evidence: the commit hash, the check output, or the run that proved it. "Should work" closes nothing.

## Writing a goal

The goal is the brief a developer reads first, so it carries the decision, not a question:

- What is wrong, with the file and line or the review or red-team item that showed it, and the evidence path (`docs/reviews/...`).
- What the fix is, as behaviour: the flag, exit code, message, column kind or rule. Name the packages that change.
- What proves it: the regression fixture, the test, the torture run.
- The escape the operator keeps, if the fix refuses something.

Bad: "verify does not handle arrays". Good: "internal/verify/residual.go's arrayHits falls back to scalarHits on a text literal, so a masked citext[] column passes the residual scan green (regression 009). Split the literal with transform's grammar and scan element-wise, or refuse at exit 9 naming the column; record the rule in internal/verify/CLAUDE.md."

## Post-mortems

Three parts, separated by ` | `: `went well: ...` (what to repeat), `went badly: ...` (what it cost, in rounds, tokens or flakes, and what was missed), `change next time: ...` (one concrete change to a brief, a path list, a check or a script; "nothing" is an acceptable answer). Name commit hashes. A task that blocked and was landed by hand says so and says why it blocked.

## Sequencing notes

When a task must land before or with another, `log` the constraint on both tasks, dated, so whoever resumes after a usage window sees it without reading the conversation. When a finding changes a decision, `log` the decision on the task and amend the ADR or ARCHITECTURE section in the same landing.
