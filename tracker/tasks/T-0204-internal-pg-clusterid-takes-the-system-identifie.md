---
id: T-0204
title: "internal/pg: ClusterID takes the system identifier core already read instead of reading pg_control_system a second time"
epic: E9
phase: ""
status: open
owner: sonnet
created: 2026-09-15
started: ""
closed: ""
outcome: ""
---

# T-0204 · internal/pg: ClusterID takes the system identifier core already read instead of reading pg_control_system a second time

## Goal

T-0190's reviewer (low): Source.ClusterID calls SystemID internally while internal/core/run.go reads it separately a few lines earlier, so a run issues pg_control_system twice in two read-only transactions, and ClusterID now fails whenever SystemID does. Pass the identifier in, as the target side already does, and state in the comment which failures propagate. Files: internal/pg/source.go, internal/core/run.go, and the trace invariants if the statement count is pinned.

## Acceptance



## Log

- 2026-09-15 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
