---
id: T-0246
title: "internal/verify: sequence observation limited to digit-like values, struct comments updated, fixture headers claim only what a revert proves"
epic: E9
phase: ""
status: open
owner: sonnet
created: 2026-09-17
started: ""
closed: ""
outcome: ""
---

# T-0246 · internal/verify: sequence observation limited to digit-like values, struct comments updated, fixture headers claim only what a revert proves

## Goal

T-0240's reviewer (three lows): seq.observe now runs for every direct value of every family though digitRange has no length or magnitude guard; sequenceExempt and requiresCorroboration's doc comments still describe the pre-T-0240 logic; regressions 032 and 033's headers claim every part is needed without a recorded revert. Files: internal/verify/secondnet.go, internal/verify/validators.go, testdata/regressions/032 and 033.

## Acceptance



## Log

- 2026-09-17 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
