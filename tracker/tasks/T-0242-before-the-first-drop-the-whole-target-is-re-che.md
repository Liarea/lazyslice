---
id: T-0242
title: "Before the first drop, the whole target is re-checked for emptiness under the run lease, and a table that appeared since the gate refuses the load"
epic: E5
phase: 5
status: done
owner: sonnet
created: 2026-09-17
started: 2026-09-17
closed: 2026-09-17
outcome: done
---

# T-0242 · Before the first drop, the whole target is re-checked for emptiness under the run lease, and a table that appeared since the gate refuses the load

## Goal

Round-4 replay (docs/reviews/2026-09-15-redteam/round4-still-leaking.json, the lock-and-recheck attempt): a third party creates a table holding production rows in the approved target while the run is stalled between the gate and the load; the per-table lock-and-recheck covers only the plan's tables, so the run loads, exits 0 and writes complete over a database its own gate would refuse as not empty. Under the lease already held, before the first drop, re-run the gate's emptiness probe over the target's current user-table set and refuse with load.refused.target_changed naming the tables that appeared; one SELECT EXISTS sweep on a target the gate already sized. Integration test with a table created between gate and load. Correct THREAT_MODEL.md T2's residual bullet, which says the writer is blocked by the lock, true only of planned tables. Files: internal/load, internal/pg (the probe), internal/core, internal/event/catalogue.yml, docs/ERRORS.md via make docs, THREAT_MODEL.md T2.

## Acceptance



## Log

- 2026-09-17 created

- 2026-09-17 started

- 2026-09-17 closed: done

## Post-mortem

went well: one probe reusing the gate's own emptiness query, one loader pre-check and one refusal variant sharing the existing exit code; the Opus reviewer caught the pre-check inferring 'appeared after the gate' from 'occupied and not in the plan', which is false on a reload, and the probe running inside the run transaction where the gate's ran autocommit, both fixed; the reverify caught the threat model omitting that the sweep does not run on a marker-bound reload, now recorded as the residual; two fix rounds, then one missing unit test added by hand with a break-and-restore proof, gate green with load, core and pg integration | went badly: the second fix round cited a test that did not exist, so the task blocked on a phantom name; the sweep runs at one instant and the window to the first drop stays open by design; Run.TargetTables is still never populated (T-0249) | change next time: a Tests line in the threat model is written after grepping the name it cites
