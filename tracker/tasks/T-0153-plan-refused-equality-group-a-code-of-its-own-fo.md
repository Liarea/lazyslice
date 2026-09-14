---
id: T-0153
title: "plan.refused.equality_group: a code of its own for the FK equality-group refusal"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-14
started: ""
closed: ""
outcome: ""
---

# T-0153 · plan.refused.equality_group: a code of its own for the FK equality-group refusal

## Goal

T-0132 made the group refusal reuse plan.refused.unique_domain (internal/plan/equality.go refuseEquality) because a new code is a row in internal/event/catalogue.yml and therefore a regenerated docs/ERRORS.md, which was outside that task's paths. The message an operator reads still says 'is under a unique index', which is wrong when the group was refused for a non-unique member. Add the code to internal/event/catalogue.yml, point refuseEquality at it, and run make docs.

## Acceptance



## Log

- 2026-09-14 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
