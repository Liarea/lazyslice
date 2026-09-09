---
id: T-0097
title: "ARCHITECTURE.md 11.1's not-recreatable refusal is still raised inside load.Load, not at plan"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-08
started: ""
closed: ""
outcome: ""
---

# T-0097 · ARCHITECTURE.md 11.1's not-recreatable refusal is still raised inside load.Load, not at plan

## Goal

11.1 says the exit-13 refusal for a recreated object depending on one v1 does not recreate is raised 'at plan - before the snapshot is used for keys and before anything in the target is dropped'. It is still raised by load.Load calling ddl.Recreatable as its first statement, which internal/load/CLAUDE.md has carried as owed since the loader landed ('the natural caller is core, which does not exist yet' - core exists now). T-TORTURE is the evidence that it matters: two of the ten real schemas reach it (mastodon's timestamp_id on nine primary keys, gitlab's gen_random_uuid_v7), so an operator with either schema pays for a full extract before being told the target cannot be built. core.asStop now maps *ddl.Refusal to its catalogued code and exit 13 (testdata/regressions/002); moving the call is the other half. The two exit-13 rows in internal/event/catalogue.yml already carry stage: plan.

## Acceptance



## Log

- 2026-09-08 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
