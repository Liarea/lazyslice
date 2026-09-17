---
id: T-0244
title: "mask: rename the test that says a sentinel passes through unwrapped, since it no longer does"
epic: E9
phase: ""
status: open
owner: sonnet
created: 2026-09-17
started: ""
closed: ""
outcome: ""
---

# T-0244 · mask: rename the test that says a sentinel passes through unwrapped, since it no longer does

## Goal

T-0238's reverify (low): TestApplyPassesItsOwnSentinelThroughUnwrapped and its comment describe the behaviour T-0238 removed; after the rewrap a bare ErrNoRoom comes back as a fresh wrap, and the test passes only because errors.Is still holds. Rename to state the real guarantee (the sentinel stays answerable and is not additionally wrapped in ErrMaskerFailed) and rewrite the comment. File: mask/apply_guards_test.go.

## Acceptance



## Log

- 2026-09-17 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
