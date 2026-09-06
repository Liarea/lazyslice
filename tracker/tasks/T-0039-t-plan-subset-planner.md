---
id: T-0039
title: "T-PLAN: subset planner"
epic: E4
phase: 4
status: done
owner: opus
created: 2026-09-05
started: 2026-09-06
closed: 2026-09-06
outcome: "done: 86c5ee1; FIFO worklist with provenance, caps, budgets, identity ladder with §3.4 pseudo-keys, unreadable tables, SCC order, not-recreatable refusal; unit and integration tests green"
---

# T-0039 · T-PLAN: subset planner

## Goal



## Acceptance



## Log

- 2026-09-05 created

- 2026-09-06 started

- 2026-09-06 closed: done: 86c5ee1; FIFO worklist with provenance, caps, budgets, identity ladder with §3.4 pseudo-keys, unreadable tables, SCC order, not-recreatable refusal; unit and integration tests green

## Post-mortem

Went well: the reviewer found the pseudo-key candidate admitting uncomparable json columns and the developer proved the refusal path with a fixture. Went badly: four findings were cross-package (pg grammar, PlanRequest zeros, privileges duplication, Plan.Unmapped) and the run was cut by a limit at verify. Change: a T-PGSHAPES stage now precedes extract; core owns the request-shape fixes; ARCHITECTURE.md records the decisions.
