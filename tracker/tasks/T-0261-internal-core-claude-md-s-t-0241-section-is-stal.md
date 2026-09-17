---
id: T-0261
title: "internal/core/CLAUDE.md's T-0241 section is stale after T-0255"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-17
started: ""
closed: ""
outcome: ""
---

# T-0261 · internal/core/CLAUDE.md's T-0241 section is stale after T-0255

## Goal

T-0255's fix round stored discover's Source.Replica answer on r.replica (run.go:298,796,910) and made data_directory's disagreement conditional on it; internal/core/CLAUDE.md lines 703-705 still describe the old single-expression call shape and line 726-727 still says Eligibility.SameCluster can be fooled by a standby whose data_directory differs.

## Acceptance



## Log

- 2026-09-17 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
