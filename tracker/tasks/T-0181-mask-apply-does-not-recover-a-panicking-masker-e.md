---
id: T-0181
title: "mask.Apply does not recover: a panicking masker escapes the module"
epic: E5
phase: 5
status: done
owner: ""
created: 2026-09-15
started: 2026-09-15
closed: 2026-09-15
outcome: done
---

# T-0181 · mask.Apply does not recover: a panicking masker escapes the module

## Goal

The 2026-09-15 red team's thrower masker panicked with the offending value in its message and the panic escaped mask.Apply (mask/mask.go) onto core's transform goroutine. internal/core and cmd/lazyslice now redact the recovered value (core.PanicSummary, THREAT_MODEL.md T4), but the module that owns the masker contract should recover in Apply and convert the panic into an error naming the masker id and the category and nothing else. mask/ is its own module and was outside T-REDFIX's paths.

## Acceptance



## Log

- 2026-09-15 created

- 2026-09-15 moved to E5 phase 5

- 2026-09-15 started

- 2026-09-15 closed: done

## Post-mortem

went well: implemented inside T-0191 (f5ec705), masker error messages never carry the value | went badly: as T-0180 | change next time: as T-0180
