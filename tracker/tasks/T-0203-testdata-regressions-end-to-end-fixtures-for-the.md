---
id: T-0203
title: "testdata/regressions: end-to-end fixtures for the round-2 DDL attempts (index predicate, pattern operand, enum label)"
epic: E9
phase: ""
status: open
owner: sonnet
created: 2026-09-15
started: ""
closed: ""
outcome: ""
---

# T-0203 · testdata/regressions: end-to-end fixtures for the round-2 DDL attempts (index predicate, pattern operand, enum label)

## Goal

T-0189's reviewer (low): its regression tests are database-free unit tests over hand-written definition strings; nothing exercises real pg_get_indexdef and pg_get_constraintdef text or the post-load catalog read under make torture. Add numbered fixtures for the R2-10 pattern operand and the R2-11 index predicate at least, in the style of 016 to 023, plus direct assertions for the network_id, online_id and free_text entries of the strong set in internal/plan/ddlliteral_test.go.

## Acceptance



## Log

- 2026-09-15 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
