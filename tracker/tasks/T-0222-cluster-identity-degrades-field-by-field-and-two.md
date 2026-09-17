---
id: T-0222
title: "Cluster identity degrades field by field, and two unknown identities are treated as possibly the same cluster"
epic: E5
phase: 5
status: done
owner: sonnet
created: 2026-09-16
started: 2026-09-17
closed: 2026-09-17
outcome: done
---

# T-0222 · Cluster identity degrades field by field, and two unknown identities are treated as possibly the same cluster

## Goal

Round-3 replay (R2-06 variant): sqlClusterID is one SELECT, so a role without EXECUTE on pg_postmaster_start_time() or pg_control_system() errors the whole row, the identity becomes empty, the gate reads unknown and falls back to endpoint spelling, which is exactly what an alias changes. Guard each field with has_function_privilege so an unreadable field is empty and the others still compare; when both identities are unknown, Target.Gate reports that the target may be on the source's own cluster, warns, and T-0184's headless refusal treats it as same-cluster rather than different. A false negative here is a production write. Files: internal/pg/source.go, internal/pg/target.go, internal/pg/cluster_test.go inputs, ARCHITECTURE.md section 9, THREAT_MODEL.md T2.

## Acceptance



## Log

- 2026-09-16 created

- 2026-09-17 started

- 2026-09-17 closed: done

## Post-mortem

went well: the developer proved with a real container that the round-3 suggestion of a has_function_privilege guard inside one SELECT cannot work, because Postgres checks EXECUTE at plan time, and split the read into a guard statement and a guarded statement under a savepoint; weak fields (oid, version) now prove difference only, never sameness; the Opus reviewer caught two tests that would pass against the broken shape, and the fixer verified each by reintroducing the bad shape; one fix round | went badly: the interrupted first attempt had left a debug line that disabled the savepoint path, found only by re-reading every diff against its rationale; the docs and two code comments still say pg_control_system is not granted to PUBLIC, which the task's own test disproved on PostgreSQL 16 (T-0224); three lows filed | change next time: a brief that asserts a Postgres privilege default cites the version it was measured on
