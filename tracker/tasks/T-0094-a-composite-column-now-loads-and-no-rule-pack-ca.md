---
id: T-0094
title: "A composite column now loads, and no rule pack category accepts its type family: decide refuse or mask field-wise (THREAT_MODEL.md T1)"
epic: E5
phase: 5
status: open
owner: ""
created: 2026-09-08
started: ""
closed: ""
outcome: ""
---

# T-0094 · A composite column now loads, and no rule pack category accepts its type family: decide refuse or mask field-wise (THREAT_MODEL.md T1)

## Goal

Before T-0083 a source table with a composite column failed CopyFrom at 42804, so such a column could never reach a target. It loads now, and nothing in the masking chain covers it: mask.TypeTag returns ok=false for a composite, internal/transform/constraints.go gives it famOther, internal/plan/writeback.go declines to judge it, internal/classify/rules.yml accepts famOther for no category but special_category - so a name hit or a value-validator hit on a composite column drops to low with a type_conflict reason and the column is copied verbatim. internal/verify's second net skips famOther by construction (columns.go netText) and the residual scan only walks masked columns, so neither net sees it either. That converts a loud failure into a silent copy of personal data inside a composite field, which is THREAT_MODEL.md T1's territory, and T1's list of known phase-4/5 gaps does not mention it (THREAT_MODEL.md and internal/classify, internal/transform, internal/plan were all outside the paths of the task that made composites loadable). Two things owed: record the gap in THREAT_MODEL.md T1, and take the plan-time decision for a column whose type family no rule-pack category accepts but whose name or samples reach possible - refuse it (the section 11.1 exit-13 direction, which is what root CLAUDE.md's 'when in doubt, mask it' points at) or teach internal/transform to mask a composite field-wise. testdata/nasty.sql trap 27 deliberately steers around this: neither of its composite columns is personal data.

## Acceptance



## Log

- 2026-09-08 created

- 2026-09-08 moved to E5 phase 5

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
