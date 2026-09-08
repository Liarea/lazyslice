---
id: T-0052
title: "Flake: TestKillNineLeavesEveryTableEmptyOrComplete races container teardown (port 5432/tcp not found)"
epic: E5
phase: 5
status: done
owner: sonnet
created: 2026-09-06
started: ""
closed: 2026-09-08
outcome: "done: merged in the backlog run wf_e6bd43ea-99d (see git log for the hash)"
---

# T-0052 · Flake: TestKillNineLeavesEveryTableEmptyOrComplete races container teardown (port 5432/tcp not found)

## Goal



## Acceptance



## Log

- 2026-09-06 created

- 2026-09-07 Widened 2026-09-06: same root cause seen in TestI6RootHoldsTakeRows/pagila (resolving the mapped port of postgres:16: port 5432/tcp not found). Fix in internal/testutil.Postgres: retry or wait on PortEndpoint with a bounded backoff instead of t.Fatalf on the first miss, and say in a comment that it is a known startup race.

- 2026-09-08 closed: done: merged in the backlog run wf_e6bd43ea-99d (see git log for the hash)

## Post-mortem

Went well: merged through the review loop. Went badly: see the run's journal for the per-task lows carried into CLAUDE.md notes. Change: none.
