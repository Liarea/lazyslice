---
id: T-0129
title: "internal/verify: an array column that arrives as a text literal is not residual-scanned element-wise"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-09
started: ""
closed: ""
outcome: ""
---

# T-0129 · internal/verify: an array column that arrives as a text literal is not residual-scanned element-wise

## Goal

internal/transform (T-0118) masks such a column element-wise and records one residual filter entry per element under the column's empty path, but internal/verify/residual.go's arrayHits falls back to scalarHits on the whole value when it is not a []any -- and the target read hands a citext[] back as the single literal string {a,b,c}. Canonicalising the whole literal matches no per-element entry, so every entry is untestable and the residual scan reports a green pass for exactly the column class T-0118 enables. Demonstrated by making the masker write one element through unmasked while still recording it: the run exits 0, the residual check passes, and only the torture harness's external I2 grep sees the addresses in the target. ARCHITECTURE.md section 6 item 1 is the only control THREAT_MODEL.md T12 has against a masker that fails open, and it is inert here. Fix in internal/verify (outside T-0118's paths): split an array literal with the same grammar internal/transform/array.go uses, or refuse/flag a masked array column whose target value does not decode to []any, so the blindness is loud rather than a green tick.

## Acceptance



## Log

- 2026-09-09 created

- 2026-09-09 Ordered ahead of T-0127 (T-0118 re-review). T-0127 removes internal/plan's arrayArrivesAsLiteral refusal, which is the only thing keeping a masked array column that arrives as a text literal out of the target today; while it stands, the untestable per-element filter entries this task is about cost nothing. The commit that removes it is the commit that turns them into a green residual tick over a column class that actually loads. So this task lands first, or in the same change as T-0127 -- T-0127's log carries the matching constraint. Landing this one alone is inert and safe.

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
