---
id: T-0225
title: "internal/classify: phoneGuessRaisable applies the same exclusions as raisableUnknown"
epic: E9
phase: ""
status: open
owner: sonnet
created: 2026-09-16
started: ""
closed: ""
outcome: ""
---

# T-0225 · internal/classify: phoneGuessRaisable applies the same exclusions as raisableUnknown

## Goal

T-0221's reviewer (low): phoneGuessRaisable drops three of the exclusions raisableUnknown applies to the sibling pass that reaches the same shape of column, so the guessed-region phone pass can raise a column the unknown pass would leave alone. Align the two, with a table test naming each exclusion. File: internal/classify/classify.go.

## Acceptance



## Log

- 2026-09-16 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
