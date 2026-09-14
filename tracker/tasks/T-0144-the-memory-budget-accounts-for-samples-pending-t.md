---
id: T-0144
title: "The memory budget accounts for samples, pending traversal, channels and batch bytes; rename the flag help to what it measures"
epic: E9
phase: ""
status: open
owner: sonnet
created: 2026-09-14
started: ""
closed: ""
outcome: ""
---

# T-0144 · The memory budget accounts for samples, pending traversal, channels and batch bytes; rename the flag help to what it measures

## Goal

docs/reviews/2026-09-09/REVIEW.md finding 9: internal/plan/plan.go:801 budgets selected keys plus the filter only; internal/extract/extract.go:307 batches 2000 rows regardless of width. T-PERF adds byte-limited batches; the accounting model and the help text are this task. internal/plan, internal/extract, cmd/lazyslice.

## Acceptance



## Log

- 2026-09-14 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
