---
id: T-0130
title: "Target ownership: a run lease on the target and a lock-and-recheck before every destructive DDL"
epic: E5
phase: 5
status: open
owner: opus
created: 2026-09-14
started: ""
closed: ""
outcome: ""
---

# T-0130 · Target ownership: a run lease on the target and a lock-and-recheck before every destructive DDL

## Goal

Target.Gate (internal/pg/target.go:198) checks emptiness and releases its connection; core then introspects, classifies and plans before load acquires a writer and drops tables (internal/core/run.go:622, 1314; internal/load/load.go:278). Nothing owns the target across that interval: docs/reviews/2026-09-09/REVIEW.md finding 1 inserted a row after the gate approved the target and the run deleted it and exited 0 (evidence/target_race.py, evidence/race.log). Two controls, both required. (1) A run lease: the gate decision is held by a dedicated target connection that takes pg_try_advisory_lock over a key derived from the target database name and keeps it until the marker is finished; a second lazyslice run against the same target is refused at exit 4 naming the run_id that holds it. (2) Lock-and-recheck: inside each table load transaction, before DROP, take LOCK TABLE IN ACCESS EXCLUSIVE MODE NOWAIT on the existing table when it exists, then re-verify what the gate approved (unmarked target: the table is still empty; bound marker: the marker row is unchanged); a change refuses at exit 4 naming the table and its row count, rolls back, and marks the run failed. Regression: turn evidence/target_race.py into an integration test (insert between gate and load; expected exit 4; the inserted row survives). THREAT_MODEL.md target-gate threat and ARCHITECTURE.md section 11.2 get dated amendments stating the controls and the residual: a writer that inserts after the drop is blocked by the lock until commit and then writes into the fresh table, which is the application behaving normally.

## Acceptance



## Log

- 2026-09-14 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
