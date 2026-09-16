---
id: T-0223
title: "mask.Apply never returns a masker's error verbatim: the module wraps it without the value"
epic: E5
phase: 5
status: open
owner: sonnet
created: 2026-09-16
started: ""
closed: ""
outcome: ""
---

# T-0223 · mask.Apply never returns a masker's error verbatim: the module wraps it without the value

## Goal

Round-3 replay (R2-13, the module half): maskCell passes a third-party masker's error straight out of the public module, and mask/ is importable on its own under ADR-006, so its contract cannot rely on the binary's redaction; 'cannot mask "victim.canary@bigcorp.example": unsupported shape' reaches any caller of Apply. Wrap it the way the panic already is: a sentinel error naming the masker id, the category and the error's type, the original not kept. A test registers a masker whose error quotes its input and asserts the returned error does not contain it. Files: mask/mask.go, mask/apply_guards_test.go, mask/CLAUDE.md.

## Acceptance



## Log

- 2026-09-16 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
