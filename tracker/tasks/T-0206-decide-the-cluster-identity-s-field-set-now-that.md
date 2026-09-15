---
id: T-0206
title: "Decide the cluster identity's field set now that the maintenance-database oid is the constant 5 on PostgreSQL 15 and newer"
epic: E9
phase: ""
status: open
owner: opus
created: 2026-09-15
started: ""
closed: ""
outcome: ""
---

# T-0206 · Decide the cluster identity's field set now that the maintenance-database oid is the constant 5 on PostgreSQL 15 and newer

## Goal

T-0190's fix round left field 1 of the positional identity in place because changing the field set is more than a fix round should do silently; on 15 and newer it carries no information, so under a SELECT-only source role the identity rests on the postmaster start time and the server version. Decide whether to replace it (the reviewer suggested min(oid) over the non-pinned databases) or append a new field. Both sides read the format positionally, so the change is an ARCHITECTURE.md section 9 amendment plus internal/pg/source.go and target.go, with cluster_test.go's inputs extended.

## Acceptance



## Log

- 2026-09-15 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
