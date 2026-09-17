---
id: T-0228
title: "ARCHITECTURE.md's --phone-region flag row and the phone_region_guessed reason fragment describe the removed name-rule corroboration arm"
epic: E9
phase: ""
status: open
owner: sonnet
created: 2026-09-16
started: ""
closed: ""
outcome: ""
---

# T-0228 · ARCHITECTURE.md's --phone-region flag row and the phone_region_guessed reason fragment describe the removed name-rule corroboration arm

## Goal

T-0221's reverify (two lows): the flag table row at ARCHITECTURE.md section 8 still says the built-in list masks a column with corroboration by the phone name pattern, and internal/classify/reasons.go's phone_region_guessed fragment hard-codes the pre-fix wording; the name-rule arm was removed in fix round 2 because it was unreachable. Correct both to the personal-neighbour rule only. Files: ARCHITECTURE.md, internal/classify/reasons.go.

## Acceptance



## Log

- 2026-09-16 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
