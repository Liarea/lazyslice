---
id: T-0262
title: "The special-category rule pins the LIKE spelling at plan level, and its normalisation comment matches the code"
epic: E9
phase: ""
status: open
owner: sonnet
created: 2026-09-17
started: ""
closed: ""
outcome: ""
---

# T-0262 · The special-category rule pins the LIKE spelling at plan level, and its normalisation comment matches the code

## Goal

T-0254's reviewer (two lows): the pattern route the red team reported beside the equality one, CHECK (note NOT LIKE '%HIV_POSITIVE%'), is covered only by the StripPatternMeta string table, with no plan-level assertion that it refuses at exit 13, so a change to the pattern branch could re-open it with every test green; and normalizeSpecialCategoryCandidate's comment says a run of separators collapses to one space while the code writes one per rune. Add the plan-level case beside TestRedTeamR5SpecialCategoryUnderscoreGluingIsRefused, and collapse the run or reword the comment. Files: internal/plan/ddlliteral_test.go, internal/textsig/special.go.

## Acceptance



## Log

- 2026-09-17 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
