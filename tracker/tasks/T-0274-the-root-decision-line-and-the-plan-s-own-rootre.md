---
id: T-0274
title: "The root decision line and the plan's own RootReason give different reasons when a name preference breaks a tie"
epic: E9
phase: ""
status: open
owner: sonnet
created: 2026-09-17
started: ""
closed: ""
outcome: ""
---

# T-0274 · The root decision line and the plan's own RootReason give different reasons when a name preference breaks a tie

## Goal

T-0271 review, low: internal/core/root.go takes the silently chosen default's reason from RootCandidate.Reason() alone, while internal/plan/root.go's defaultRoot appends '; name preference' when the top pick won a score tie on a preferred name, which is exactly ADR-008's customers example. Two sentences for one decision. Export the reason-with-preference computation beside RankRoots and use it in both places.

## Acceptance



## Log

- 2026-09-17 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
