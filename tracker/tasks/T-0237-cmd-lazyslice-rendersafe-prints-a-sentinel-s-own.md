---
id: T-0237
title: "cmd/lazyslice: renderSafe prints a sentinel's own text rather than its wrapping chain, and internal/load's refusal is value-free without a SQLSTATE"
epic: E9
phase: ""
status: open
owner: sonnet
created: 2026-09-17
started: ""
closed: ""
outcome: ""
---

# T-0237 · cmd/lazyslice: renderSafe prints a sentinel's own text rather than its wrapping chain, and internal/load's refusal is value-free without a SQLSTATE

## Goal

T-0212's reviewer (two lows): the errUsage and ErrNotImplemented allowlist arms print err.Error(), the whole chain, and errUsage wraps foreign errors in at least one place (os.Getwd); and load.Refusal.Error()'s no-SQLSTATE branch prints the wrapped error's text, contradicting renderSafe's documented guarantee that a claimed Stop's message is value-free. Print the sentinel's own text, and make the load branch value-free with the cause reachable through Unwrap. Files: cmd/lazyslice/main.go, internal/load/codes.go, with a test each.

## Acceptance



## Log

- 2026-09-17 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
