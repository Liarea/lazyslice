# lazyslice

Snapshot a production SQL database into a safe local copy: subset by a root table, follow foreign keys, mask personal data, load. Read CONCEPT.md for what we are building and docs/OPERATING_MODEL.md for how.

## Current phase

Phase 3, Foundations. Gate: CI green on an empty implementation, integration tests fail for the right reason, both fixtures load with every trap documented, per-directory CLAUDE.md files, ROADMAP.md, ADR-008 first run. ARCHITECTURE.md section 14 is the v1 cut line; section 12 is the layout. See docs/BUILD_PLAN.md.

## Rules

Work only on the current phase. Anything else goes to the tracker as an open task in a later epic with one line of reasoning; do not build it.

Before saying something works: run the checks (lint, test, integration once they exist) and paste the result. "Should work" is not a status.

Decisions live in docs/adr/. An ADR is proposed until its phase gate closes, then accepted and frozen. To change an accepted decision, add a new ADR that supersedes it. Never edit an accepted ADR.

Write only the files your task names. Do not fix nearby code, extend behaviour the task did not mention, or add tests beyond the task. If you see something else wrong, report it in your return value.

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

## Tracker

```
python3 tools/tracker.py list
python3 tools/tracker.py new --epic E1 --title "..." --owner opus
python3 tools/tracker.py close T-0001 --outcome done --postmortem "went well | went badly | change next time"
```
