---
id: T-0241
title: "A streaming standby as source, with the target on its own primary, is refused: a start-time disagreement without the system identifier is unknown, a standby source is announced, and a headless run with no --target refuses on a standby"
epic: E5
phase: 5
status: open
owner: sonnet
created: 2026-09-17
started: ""
closed: ""
outcome: ""
---

# T-0241 · A streaming standby as source, with the target on its own primary, is refused: a start-time disagreement without the system identifier is unknown, a standby source is announced, and a headless run with no --target refuses on a standby

## Goal

Round-4 replay (docs/reviews/2026-09-15-redteam/round4-still-leaking.json, the read-replica attempts): a primary and its hot standby share system_identifier but never the postmaster start time, and under the SELECT-only role the identifier is unreadable, so the gate reads 'different cluster' and writes to the production primary with a green exit; ADR-013's second escalation inherits that verdict. Three changes: a start-time disagreement with system_identifier missing on either side downgrades to unknown, which the gate already treats as possibly the same cluster; read pg_is_in_recovery() on the source (executable by PUBLIC) and, when true, print a header line saying the source is a standby with sender_host and sender_port from pg_stat_wal_receiver where readable, and make a headless run with no --target refuse outright naming --target; add GRANT EXECUTE ON FUNCTION pg_control_system() TO lazyslice_ro to ARCHITECTURE.md's recommended-role snippet and to the CodeRoleWritable message, keeping the statement that the identity degrades without it. Tests: cluster_test.go inputs for the verdict; an integration test with a primary and a streaming standby if internal/testutil can stand one up with pg_basebackup in a second container, otherwise a documented gap in THREAT_MODEL.md T2. Files: internal/pg, internal/core, internal/discover, ARCHITECTURE.md section 9, THREAT_MODEL.md T2, docs/adr/013 (proposed, editable), catalogue and docs via make docs.

## Acceptance



## Log

- 2026-09-17 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
