---
id: T-0252
title: "The run lease is re-asserted before the first drop and before each table drop; a lost lease refuses the load"
epic: E5
phase: 5
status: done
owner: sonnet
created: 2026-09-17
started: 2026-09-17
closed: 2026-09-17
outcome: done
---

# T-0252 · The run lease is re-asserted before the first drop and before each table drop; a lost lease refuses the load

## Goal

Round-5 replay (docs/reviews/2026-09-15-redteam/round5-still-leaking.json, the wrong-target attacker's lease attempt): the lease is a transaction-scoped advisory lock held open on one idle-in-transaction connection for the whole run and never looked at again, so a server-side reaper, a pooler restart or a dropped connection lets a second run take the target apart with the first; the first surfaces a raw SQLSTATE 23505 as an uncoded failure. Export a Lease.Alive that confirms the lease connection still holds classid and objid in pg_locks, call it in the loader before the whole-target recheck and before each dropOne, and refuse with load.refused.lease_lost, never a fall-through, the rule core already applies at acquisition; give the 23505 a code. Integration test: terminate the lease backend with pg_terminate_backend between gate and load, assert the refusal. Files: internal/pg/lease.go, internal/load, internal/core, internal/event/catalogue.yml, docs/ERRORS.md via make docs, THREAT_MODEL.md T2.

## Acceptance



## Log

- 2026-09-17 created

- 2026-09-17 started

- 2026-09-17 closed: done

## Post-mortem

went well: Lease.Alive confirms the lease connection still holds the lock in pg_locks and the loader asks before the marker write, the whole-target recheck and every drop; the Opus reviewer caught the first cut checking only after the marker row was written, while three documents claimed the opposite order, and caught a cancelled context surfacing as a phantom lease-lost refusal; the 23505 race now has a code; one fix round, reverify clean, integration test terminates the lease backend between gate and load | went badly: the workflow's verify agent started the gate in the background and returned before it finished, so the task blocked and the orchestrator ran the gate (check, pg, load and core integration, torture) and committed; the verify prompt now forbids background runs | change next time: a comment that claims an ordering (X before Y, so Z cannot happen) is verified against the call order in the same diff, and an integration test for a wrong-target race counts the marker table first
