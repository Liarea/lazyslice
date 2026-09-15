---
id: T-0199
title: "internal/load's gate test holds its own copy of the cluster-identity SQL"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-15
started: ""
closed: ""
outcome: ""
---

# T-0199 · internal/load's gate test holds its own copy of the cluster-identity SQL

## Goal

internal/load/load_integration_test.go (TestLoadPagilaIntoAMarkedTarget, around line 505) builds the source cluster identity by re-typing the SQL internal/pg's sqlClusterID used before T-0190 replaced it: pg_postmaster_start_time with inet_server_addr and inet_server_port. The test still passes, because the two containers differ in the first field and the comparison is field-wise, but it is now asserting against a shape the product no longer produces and it will drift again. Have it call pg.Source.ClusterID (or an exported test helper in internal/pg) instead of its own SQL. internal/load is outside T-0190's paths.

## Acceptance



## Log

- 2026-09-15 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
