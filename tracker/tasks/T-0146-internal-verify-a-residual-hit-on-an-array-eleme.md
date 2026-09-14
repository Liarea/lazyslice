---
id: T-0146
title: "internal/verify: a residual hit on an array element cannot be confirmed by either probe of section 6 item 3"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-14
started: ""
closed: ""
outcome: ""
---

# T-0146 · internal/verify: a residual hit on an array element cannot be confirmed by either probe of section 6 item 3

## Goal

Both probes in internal/verify/sql.go ask whether the *column* holds the candidate (probeSQL: t.col = $1; foldedProbeSQL: lower(t.col::text) = lower($1::text)), and a residual hit on an array column carries one *element*, not the array (residual.go, arrayHits, for both carriers: the []any path that has always existed and the array-literal path T-0129 added). Binding an element string against an array column is an encode-plan or malformed-array-literal error, so the probe fails, confirm() answers untestable and every element hit is exit 9 verify.refused.residual_unconfirmable with reasonProbeFailed -- never verify.refused.residual, even when the source does still hold the element. Fail-closed, so not a leak, but three costs: a Bloom false positive on any element of any masked array column refuses a correct run with a reason that names no action; a real leak is reported as 'could not be confirmed' rather than 'the source still holds this value'; and a probe that must error still spends the cap and puts the element in the source's log, which is exactly what the leaf rule (hit.leaf, reasonNoProbe) avoids for a value inside a document. Decide between a probe shape that can express it (t.col @> ARRAY[$1]::<type>[], which needs a new entry in shapes.go and the element type) and treating an array element like a leaf (untestable by construction, with a reason that says so and no probe spent). Files: internal/verify/residual.go, sql.go, shapes.go, reasons.go. Noted in internal/verify/CLAUDE.md under the T-0129 rule.

## Acceptance



## Log

- 2026-09-14 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
