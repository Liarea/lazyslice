# lazyslice

Snapshot a production SQL database into a safe local copy: subset by a root table, follow foreign keys, mask personal data, load. Read CONCEPT.md for what we are building and docs/OPERATING_MODEL.md for how.

## Current phase

See ROADMAP.md, section "Current phase", for the phase and its gate. ARCHITECTURE.md section 14 is the v1 cut line; section 12 is the layout.

## Rules

Work only on the current phase. Anything else goes to the tracker as an open task in a later epic with one line of reasoning; do not build it.

Before saying something works: run the checks (lint, test, integration once they exist) and paste the result. "Should work" is not a status.

Decisions live in docs/adr/. An ADR is proposed until its phase gate closes, then accepted and frozen. To change an accepted decision, add a new ADR that supersedes it. Never edit an accepted ADR.

Write only the files your task names. Do not fix nearby code, extend behaviour the task did not mention, or add tests beyond the task. If you see something else wrong, report it in your return value, and if it is work someone must do outside your paths, file it yourself with `python3 tools/tracker.py new --epic E9 --title "..." --goal "why, and which file"` (E9 is Later; the orchestrator re-homes it). That is the one write outside your paths a task allows; never edit tracker files by hand.

Every factual claim in a research document links to its source. Recognizing a tool's name is not knowing its current state; search for it and verify as of the current date.

Safety rails from THREAT_MODEL.md are not configurable away without a flag whose name says what it does. Never add a flag that disables masking wholesale. When in doubt, mask it.

The TUI is a thin layer. Every TUI action must be reachable by a CLI flag first.

Documentation is never the fix for a confusing first run. Change the default or the question.

Do not create git commits unless your task says to. The orchestrator commits.

## Layout

- CONCEPT.md, docs/OPERATING_MODEL.md, docs/BUILD_PLAN.md: what, how, and in what order.
- research/: phase 1 documents.
- docs/adr/: architecture decision records. docs/prompting/: per-model prompt cheat sheets.
- tracker/: epics, tasks, BOARD.md. Written only via tools/tracker.py by the orchestrator.
- .claude/workflows/: one resumable workflow per phase.

## Tracker and commits

Two skills carry the recurring chores: `lazyslice-tracker` (`.claude/skills/lazyslice-tracker/SKILL.md`: filing, starting, logging, moving, closing with a post-mortem; how to word a goal) and `lazyslice-commit` (`.claude/skills/lazyslice-commit/SKILL.md`: the headline-plus-bullets commit format release notes are generated from, and path-scoped staging while a workflow runs). Read the skill before doing either.

```
python3 tools/tracker.py list
python3 tools/tracker.py new --epic E5 --phase 5 --title "..." --owner sonnet --goal "..."
python3 tools/tracker.py close T-0001 --outcome done --postmortem "went well: ... | went badly: ... | change next time: ..."
make relnotes FROM=v0.1.0 TO=HEAD
```
