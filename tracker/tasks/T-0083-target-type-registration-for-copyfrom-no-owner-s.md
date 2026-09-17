---
id: T-0083
title: "Target type registration for CopyFrom: no owner since internal/load shipped"
epic: E5
phase: 5
status: done
owner: ""
created: 2026-09-08
started: 2026-09-08
closed: 2026-09-17
outcome: done
---

# T-0083 · Target type registration for CopyFrom: no owner since internal/load shipped

## Goal

ARCHITECTURE.md 11.1 and ADR-005 say load registers the source's user types on each target connection; internal/pg/CLAUDE.md deferred it to 'the task that builds internal/load', and internal/load/CLAUDE.md pointed at 'internal/pg's AfterConnect', which never existed on the target pool and is forbidden on the source pool since T-0076. internal/load is built, so the debt now has no owner: an enum array or a composite column fails CopyFrom mid-table (54000/42804, THREAT_MODEL.md T8). Work is in internal/pg (register the source's enum, domain, composite and user-defined array OIDs on each target connection from the catalog internal/introspect reads) plus the wiring that hands pg that catalog. Found while rewriting the stale AfterConnect prose for T-0081.

## Acceptance



## Log

- 2026-09-08 created

- 2026-09-08 moved to E5 phase 5

- 2026-09-08 started

- 2026-09-17 closed: done

## Post-mortem

went well: load registers the source's enum, domain, composite and user-defined array types on every target connection through Writer.RegisterTypes before the first CopyFrom (commit 1ae4a9e, recorded in ARCHITECTURE.md 11.1 and both package CLAUDE.md files) | went badly: the task landed on 2026-09-09 and was never closed, so the board showed phase-5 work in progress for eight days until the gate-5 audit on 2026-09-17 caught it | change next time: a by-hand landing closes its task in the same sitting as the commit
