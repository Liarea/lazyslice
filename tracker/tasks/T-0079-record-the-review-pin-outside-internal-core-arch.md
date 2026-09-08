---
id: T-0079
title: "Record the review pin outside internal/core: ARCHITECTURE.md entry points, cmd/CLAUDE.md, and ADR-005's exit 12"
epic: E9
phase: ""
status: done
owner: ""
created: 2026-09-08
started: ""
closed: 2026-09-08
outcome: "done: ARCHITECTURE.md §1 records the three entry points and the review pin; ADR README errata carries the fifth exit-12 cause; cmd/CLAUDE.md names core.Preview"
---

# T-0079 · Record the review pin outside internal/core: ARCHITECTURE.md entry points, cmd/CLAUDE.md, and ADR-005's exit 12

## Goal

T-PIN added core.Preview (a third exported entry point beside Run and Introspect), core.Request.Reviewed, and the refusal core.refused.reviewed_changed at exit 12, so that --tui's second pass is pinned to the snapshot the operator reviewed. Three files outside T-PIN's paths now understate or contradict that. (1) ARCHITECTURE.md section 1 says core.Run drives the pipeline 'with no other entry point'; Introspect was already an exception and Preview is a second - state both. (2) cmd/CLAUDE.md line 5 says this file 'builds core.Request and calls core.Run' and its 'Decisions' list records the core.Introspect exception; add the same bullet for core.Preview, which runTUI now calls for the pass that fills the screens. (3) ADR-005's exit table lists four cases under exit 12 ('plan refused'), and this refusal is a fifth that happens before the planner runs; ADR-005 is accepted and frozen, so this needs a superseding ADR that either widens 12 or gives the review pin an exit of its own. internal/core/CLAUDE.md records all three as owed.

## Acceptance



## Log

- 2026-09-08 created

- 2026-09-08 closed: done: ARCHITECTURE.md §1 records the three entry points and the review pin; ADR README errata carries the fifth exit-12 cause; cmd/CLAUDE.md names core.Preview

## Post-mortem

Went well: documentation-only. Went badly: nothing. Change: none.
