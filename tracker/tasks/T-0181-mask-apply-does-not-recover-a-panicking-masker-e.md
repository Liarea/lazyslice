---
id: T-0181
title: "mask.Apply does not recover: a panicking masker escapes the module"
epic: E5
phase: 5
status: open
owner: ""
created: 2026-09-15
started: ""
closed: ""
outcome: ""
---

# T-0181 · mask.Apply does not recover: a panicking masker escapes the module

## Goal

The 2026-09-15 red team's thrower masker panicked with the offending value in its message and the panic escaped mask.Apply (mask/mask.go) onto core's transform goroutine. internal/core and cmd/lazyslice now redact the recovered value (core.PanicSummary, THREAT_MODEL.md T4), but the module that owns the masker contract should recover in Apply and convert the panic into an error naming the masker id and the category and nothing else. mask/ is its own module and was outside T-REDFIX's paths.

## Acceptance



## Log

- 2026-09-15 created

- 2026-09-15 moved to E5 phase 5

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
