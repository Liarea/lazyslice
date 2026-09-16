---
id: T-0222
title: "Cluster identity degrades field by field, and two unknown identities are treated as possibly the same cluster"
epic: E5
phase: 5
status: open
owner: sonnet
created: 2026-09-16
started: ""
closed: ""
outcome: ""
---

# T-0222 · Cluster identity degrades field by field, and two unknown identities are treated as possibly the same cluster

## Goal

Round-3 replay (R2-06 variant): sqlClusterID is one SELECT, so a role without EXECUTE on pg_postmaster_start_time() or pg_control_system() errors the whole row, the identity becomes empty, the gate reads unknown and falls back to endpoint spelling, which is exactly what an alias changes. Guard each field with has_function_privilege so an unreadable field is empty and the others still compare; when both identities are unknown, Target.Gate reports that the target may be on the source's own cluster, warns, and T-0184's headless refusal treats it as same-cluster rather than different. A false negative here is a production write. Files: internal/pg/source.go, internal/pg/target.go, internal/pg/cluster_test.go inputs, ARCHITECTURE.md section 9, THREAT_MODEL.md T2.

## Acceptance



## Log

- 2026-09-16 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
