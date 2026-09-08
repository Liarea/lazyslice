---
id: T-0083
title: "Target type registration for CopyFrom: no owner since internal/load shipped"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-08
started: ""
closed: ""
outcome: ""
---

# T-0083 · Target type registration for CopyFrom: no owner since internal/load shipped

## Goal

ARCHITECTURE.md 11.1 and ADR-005 say load registers the source's user types on each target connection; internal/pg/CLAUDE.md deferred it to 'the task that builds internal/load', and internal/load/CLAUDE.md pointed at 'internal/pg's AfterConnect', which never existed on the target pool and is forbidden on the source pool since T-0076. internal/load is built, so the debt now has no owner: an enum array or a composite column fails CopyFrom mid-table (54000/42804, THREAT_MODEL.md T8). Work is in internal/pg (register the source's enum, domain, composite and user-defined array OIDs on each target connection from the catalog internal/introspect reads) plus the wiring that hands pg that catalog. Found while rewriting the stale AfterConnect prose for T-0081.

## Acceptance



## Log

- 2026-09-08 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
