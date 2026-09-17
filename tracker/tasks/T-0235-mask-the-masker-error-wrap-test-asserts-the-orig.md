---
id: T-0235
title: "mask: the masker-error wrap test asserts the original error is dropped, not only that the canary is absent"
epic: E9
phase: ""
status: open
owner: sonnet
created: 2026-09-17
started: ""
closed: ""
outcome: ""
---

# T-0235 · mask: the masker-error wrap test asserts the original error is dropped, not only that the canary is absent

## Goal

T-0223's reviewer (low): TestApplyWrapsAMaskersReturnedError checks the sentinel, the masker id, the category and that 'victim' is absent, but never that the original error is not reachable; have the masker return a sentinel of the test's own and assert errors.Is on it is false, and assert the full input string is absent. File: mask/apply_guards_test.go.

## Acceptance



## Log

- 2026-09-17 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
