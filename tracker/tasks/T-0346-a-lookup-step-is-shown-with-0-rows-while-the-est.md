---
id: T-0346
title: "A lookup step is shown with 0 rows while the estimate counts its rows"
epic: E6
phase: 6
status: done
owner: sonnet
created: 2026-09-24
started: ""
closed: 2026-09-25
outcome: done
---

# T-0346 · A lookup step is shown with 0 rows while the estimate counts its rows

## Goal

Orchestrator's evaluation of --tui against Pagila (2026-09-24): public.country, a lookup step, shows '0 rows, lookup; lookup' in the line printer and 0 on the plan screen, while the estimate's 15920 rows is the sum of the shown rows (15811) plus country's 109: the lookup step is emitted with no keys, so stepRows gives 0 (internal/plan/plan.go:994, internal/core/run.go:1622) while the estimate adds lookupRows. Print the lookup's row count on its plan line and in the plan screen, and say 'copied whole' instead of the bare 'lookup' in the why; pin with a plan test over the Pagila shape (a parent reached only as a lookup).

## Acceptance

—

## Log

- 2026-09-24 2026-09-24 created
- 2026-09-25 closed: done

## Post-mortem

went well: landed as ea3d105 after one review round (one medium, three low): a lookup step's plan line shows the rows it copies rather than 0, and the fix round capped the packed count at the lookup ceiling so the number agrees with the adjacent 'only the first 1000' sentence | went badly: three lows filed as a follow-up (ParseLookupRows runs on every step's Why, not only lookups, and can cut a name ending in a rows suffix; no core test over stepRows and stepWhy; a stale assemble comment and the Step.Why contract in internal/pipeline); extract's lookup read bound is 1001 where the sentence says 1000, noted for T-0379 | change next time: when a count is packed into a string another package parses back, the brief names both ends' tests
