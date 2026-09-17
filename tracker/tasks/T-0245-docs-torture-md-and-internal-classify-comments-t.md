---
id: T-0245
title: "docs/TORTURE.md and internal/classify comments: the lowered floor can still force a refusal through an equality group's masker"
epic: E9
phase: ""
status: open
owner: sonnet
created: 2026-09-17
started: ""
closed: ""
outcome: ""
---

# T-0245 · docs/TORTURE.md and internal/classify comments: the lowered floor can still force a refusal through an equality group's masker

## Goal

T-0239's reviews (lows): the measurement section claims a lower floor can only mask more and never force a new refusal, but a narrow newly masked column that is FK-connected to a masked unique column joins its equality group and can make the group's masker not fit, a plan refusal at exit 12; the FK exclusion's rationale comment overclaims in the same way; THREAT_MODEL.md's first 2026-09-17 amendment still counts four remaining exclusions. Narrow the claims to what was measured. Files: docs/TORTURE.md, internal/classify/classify.go, THREAT_MODEL.md T1.

## Acceptance



## Log

- 2026-09-17 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
