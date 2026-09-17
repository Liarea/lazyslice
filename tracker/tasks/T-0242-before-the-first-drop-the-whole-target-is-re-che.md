---
id: T-0242
title: "Before the first drop, the whole target is re-checked for emptiness under the run lease, and a table that appeared since the gate refuses the load"
epic: E5
phase: 5
status: open
owner: sonnet
created: 2026-09-17
started: ""
closed: ""
outcome: ""
---

# T-0242 · Before the first drop, the whole target is re-checked for emptiness under the run lease, and a table that appeared since the gate refuses the load

## Goal

Round-4 replay (docs/reviews/2026-09-15-redteam/round4-still-leaking.json, the lock-and-recheck attempt): a third party creates a table holding production rows in the approved target while the run is stalled between the gate and the load; the per-table lock-and-recheck covers only the plan's tables, so the run loads, exits 0 and writes complete over a database its own gate would refuse as not empty. Under the lease already held, before the first drop, re-run the gate's emptiness probe over the target's current user-table set and refuse with load.refused.target_changed naming the tables that appeared; one SELECT EXISTS sweep on a target the gate already sized. Integration test with a table created between gate and load. Correct THREAT_MODEL.md T2's residual bullet, which says the writer is blocked by the lock, true only of planned tables. Files: internal/load, internal/pg (the probe), internal/core, internal/event/catalogue.yml, docs/ERRORS.md via make docs, THREAT_MODEL.md T2.

## Acceptance



## Log

- 2026-09-17 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
