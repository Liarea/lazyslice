---
id: T-0046
title: "Fixture: deferrable unique on a partitioned root, leaf-local key, and an edge referencing the leaf"
epic: E5
phase: 5
status: done
owner: sonnet
created: 2026-09-06
started: ""
closed: 2026-09-08
outcome: "done: merged in the backlog run wf_e6bd43ea-99d (see git log for the hash)"
---

# T-0046 · Fixture: deferrable unique on a partitioned root, leaf-local key, and an edge referencing the leaf

## Goal

Makes ForeignKey.NotRecreatable and the planner's exit-13 refusal CI-verified; today only a unit test holds the seam

## Acceptance



## Log

- 2026-09-06 created

- 2026-09-08 closed: done: merged in the backlog run wf_e6bd43ea-99d (see git log for the hash)

## Post-mortem

Went well: merged through the review loop. Went badly: see the run's journal for the per-task lows carried into CLAUDE.md notes. Change: none.
