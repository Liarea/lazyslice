---
id: T-0234
title: "internal/pg: two code comments still say EXECUTE on pg_control_system is not granted to PUBLIC"
epic: E9
phase: ""
status: open
owner: sonnet
created: 2026-09-17
started: ""
closed: ""
outcome: ""
---

# T-0234 · internal/pg: two code comments still say EXECUTE on pg_control_system is not granted to PUBLIC

## Goal

T-0222's reviewer (low): target.go near the SameCluster switch and source.go near sqlClusterID assert the premise the task's own integration test disproved on PostgreSQL 16; T-0224 covers the prose in ARCHITECTURE.md, THREAT_MODEL.md and internal/pg/CLAUDE.md. Correct the two comments or point them at T-0224. Files: internal/pg/target.go, internal/pg/source.go.

## Acceptance



## Log

- 2026-09-17 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
