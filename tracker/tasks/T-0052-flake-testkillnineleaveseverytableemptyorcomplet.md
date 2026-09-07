---
id: T-0052
title: "Flake: TestKillNineLeavesEveryTableEmptyOrComplete races container teardown (port 5432/tcp not found)"
epic: E5
phase: 5
status: open
owner: sonnet
created: 2026-09-06
started: ""
closed: ""
outcome: ""
---

# T-0052 · Flake: TestKillNineLeavesEveryTableEmptyOrComplete races container teardown (port 5432/tcp not found)

## Goal



## Acceptance



## Log

- 2026-09-06 created

- 2026-09-07 Widened 2026-09-06: same root cause seen in TestI6RootHoldsTakeRows/pagila (resolving the mapped port of postgres:16: port 5432/tcp not found). Fix in internal/testutil.Postgres: retry or wait on PortEndpoint with a bounded backoff instead of t.Fatalf on the first miss, and say in a comment that it is a known startup race.

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
