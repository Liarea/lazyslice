---
id: T-0158
title: "Export the closed-value label list from mask so plan compares labels, not CHECK text"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-14
started: ""
closed: ""
outcome: ""
---

# T-0158 · Export the closed-value label list from mask so plan compares labels, not CHECK text

## Goal

internal/plan/equality.go's sameClosedSet compares mask.Constraints.Checks verbatim because mask/domain.go's labels()/checkValues() are unexported and mask/ is a separate module outside T-0132's paths; that over-refuses an FK equality group whose two ends spell one value list differently. Export the parsed label list from mask and compare that.

## Acceptance



## Log

- 2026-09-14 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
