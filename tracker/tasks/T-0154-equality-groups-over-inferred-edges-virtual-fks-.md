---
id: T-0154
title: "Equality groups over inferred edges: virtual_fks and polymorphic pairs"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-14
started: ""
closed: ""
outcome: ""
---

# T-0154 · Equality groups over inferred edges: virtual_fks and polymorphic pairs

## Goal

internal/plan/equality.go's equalityGroups unions only the declared foreign keys in p.fks. The inferred edges of ARCHITECTURE.md 3.2 - virtual_fks: from the config and the polymorphic thing_id/thing_type pairs - join columns the target never carries as a constraint, so two ends of such a join can still mask to different values and nothing refuses. Decide whether a masked value's equality is owed across an inferred edge and, if it is, feed p.inferred and p.virtual into equalityGroups.

## Acceptance



## Log

- 2026-09-14 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
