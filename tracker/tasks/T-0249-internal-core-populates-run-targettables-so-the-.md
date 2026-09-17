---
id: T-0249
title: "internal/core populates Run.TargetTables so the loader's extra-tables path is not dead"
epic: E9
phase: ""
status: open
owner: sonnet
created: 2026-09-17
started: ""
closed: ""
outcome: ""
---

# T-0249 · internal/core populates Run.TargetTables so the loader's extra-tables path is not dead

## Goal

T-0242's developer: Run.TargetTables, the extra tables a caller can add to the drop loop, is never populated by internal/core/run.go, a pre-existing gap the loader's own doc comment notes. Decide whether the field is needed; populate it from the gate's enumeration or remove it and the dead branch. Files: internal/core/run.go, internal/load/load.go.

## Acceptance



## Log

- 2026-09-17 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
