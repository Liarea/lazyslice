---
id: T-0147
title: "internal/classify/CLAUDE.md array-literal section is stale after T-0118/T-0129/T-0127"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-14
started: ""
closed: ""
outcome: ""
---

# T-0147 · internal/classify/CLAUDE.md array-literal section is stale after T-0118/T-0129/T-0127

## Goal

internal/classify/CLAUDE.md's 'An array whose sample arrives as one string' section (around line 473) still says the transform half is owed (T-0118) and that internal/plan/writeback.go's arrayArrivesAsLiteral refuses such a column at exit 12 with 'When T-0118 lands, that refusal goes.' T-0118, T-0129 and T-0127 have all landed and the refusal is deleted, so this file documents a plan-time guard that no longer exists and an owed transform half that is done. Repoint the section at the landed state (transform masks the literal element-wise, verify's arrayHits/arrayliteral.go splits it for the residual scan, plan no longer refuses it) the same way internal/transform/CLAUDE.md and internal/plan/CLAUDE.md were updated for T-0127's review findings. Outside this task's authorized paths (internal/plan/, testdata/, internal/invariants/, internal/transform/CLAUDE.md), so filed rather than edited.

## Acceptance



## Log

- 2026-09-14 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
