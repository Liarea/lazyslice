---
id: T-0312
title: "The same-column-name rule does not propagate a decision that was itself only a neighbour sweep"
epic: E6
phase: 6
status: done
owner: sonnet
created: 2026-09-23
started: ""
closed: 2026-09-24
outcome: done
---

# T-0312 · The same-column-name rule does not propagate a decision that was itself only a neighbour sweep

## Goal

Dogfood session 1: 25 propagations of the form 'column name X is free_text in public.T' carried sweep decisions across tables: uuid and state from one devices table into five tables each, filename as credential into three, colour, country, role, ui_mode, type and title into others; four propagated text-uuid columns were under unique indexes and each became a plan refusal. A decision reached only by the certain-neighbour sweep (no name or value signal of its own) should not be evidence for another table's column; propagate only a decision with a signal. internal/classify's sameColumnName pass; pin with a fixture where the source column was swept and the target column has its own samples.

## Acceptance

—

## Log

- 2026-09-23 2026-09-23 created
- 2026-09-24 closed: done

## Post-mortem

went well: 291f198; a decision reached only by the sweep is no longer a source for the same-column-name rule, propagateKeys clears the swept mark, and the ARCHITECTURE amendment records the two truth sets showing no recall change (n=64, n=87) with the torture measurement deferred to T-0351 | went badly: three lows from review: a swept unique fkPairs partner can still spread its decision one hop through an unvalidated or virtual edge, the docs claim a swept column is still an ordinary target when sameColumnName never raises a column already at possible, and no test pins the source-ordering claim; filed together | change next time: when a pass gains a new state bit, list every other pass that copies decisions and say what each does with the bit
