---
id: T-0202
title: "internal/plan/ddlliteral.go: judge only the indexes the loader recreates, not the ones backing PRIMARY KEY, UNIQUE and EXCLUDE constraints"
epic: E9
phase: ""
status: open
owner: sonnet
created: 2026-09-15
started: ""
closed: ""
outcome: ""
---

# T-0202 · internal/plan/ddlliteral.go: judge only the indexes the loader recreates, not the ones backing PRIMARY KEY, UNIQUE and EXCLUDE constraints

## Goal

T-0189's reviewer (low): tableDDLLiterals walks every t.Index while internal/load/ddl's indexStatements skips constraint-backed ones, so a refusal can name an index object the run never issues, and an exclusion constraint is judged twice under two names. Mirror the byConstraint filter so plan and loader judge the same set; a unit test over a table with a UNIQUE constraint pins it.

## Acceptance



## Log

- 2026-09-15 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
