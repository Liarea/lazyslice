---
id: T-0145
title: "A read-only verify command that checks the current target without dropping it"
epic: E9
phase: ""
status: open
owner: opus
created: 2026-09-14
started: ""
closed: ""
outcome: ""
---

# T-0145 · A read-only verify command that checks the current target without dropping it

## Goal

docs/reviews/2026-09-09/REVIEW.md: the verify command drops and reloads because the residual filter is ephemeral; a developer investigating a suspicious copy should not destroy the evidence. Rename the destructive form and add an explicitly limited observational check. cmd/lazyslice, internal/core.

## Acceptance



## Log

- 2026-09-14 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
