---
id: T-0219
title: "--allow-type-literal is recorded only for a type that actually carried a literal a strong validator hit"
epic: E9
phase: ""
status: open
owner: sonnet
created: 2026-09-16
started: ""
closed: ""
outcome: ""
---

# T-0219 · --allow-type-literal is recorded only for a type that actually carried a literal a strong validator hit

## Goal

T-0186's reviewer (low): the flag stamps every named type into the run's allow map and the yml types: block whether or not the type carries a literal any validator hits, so a stale or misspelt opt-out is carried forward silently. Record and honour only names that matched, and warn on the rest. Files: internal/core/run.go, internal/core/typeallow_test.go.

## Acceptance



## Log

- 2026-09-16 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
