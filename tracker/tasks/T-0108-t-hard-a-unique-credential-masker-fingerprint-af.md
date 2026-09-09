---
id: T-0108
title: "T-HARD-A: unique credential masker, fingerprint after plan, exit 13 at plan (T-0098, T-0101, T-0097)"
epic: E5
phase: 5
status: done
owner: opus
created: 2026-09-08
started: ""
closed: 2026-09-08
outcome: "done: ecc42ae; credential_unique (domain 2^65), fingerprint recomputed after the plan, exit 13 raised at plan"
---

# T-0108 · T-HARD-A: unique credential masker, fingerprint after plan, exit 13 at plan (T-0098, T-0101, T-0097)

## Goal



## Acceptance



## Log

- 2026-09-08 created

- 2026-09-08 closed: done: ecc42ae; credential_unique (domain 2^65), fingerprint recomputed after the plan, exit 13 raised at plan

## Post-mortem

Went well: the defect that forced the opt-outs is fixed in the masker module. Went badly: the torture catalogue still carries the opt-outs, so the evidence run does not yet exercise the fix; three §5 sentences were outside the paths. Change: T-0112 re-measures; orchestrator amended §5.
