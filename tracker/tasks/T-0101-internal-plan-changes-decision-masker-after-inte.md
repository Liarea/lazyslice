---
id: T-0101
title: "internal/plan changes Decision.Masker after internal/classify has computed the classification fingerprint"
epic: E5
phase: 5
status: done
owner: ""
created: 2026-09-08
started: ""
closed: 2026-09-08
outcome: "done: in T-HARD-A (ecc42ae)"
---

# T-0101 · internal/plan changes Decision.Masker after internal/classify has computed the classification fingerprint

## Goal

internal/plan/unique.go implements 5's 'the plan picks, within the column's category, the registered generator with the largest Domain()' by writing the chosen masker back onto pipeline.Decision, which internal/transform then masks with and internal/emit writes into the yml. Classification.Fingerprint is sha256 over (rule-pack version, and per column: category, masker) and is computed inside Classify, before the plan runs - so an escalation from phone to phone_unique, or network_id to ip_unique, changes the masked values and does not change the fingerprint. The consequence is narrow but real: 'classification changed - masked values will differ' would not print for a column that became unique between two runs. internal/core/domain.go already writes Domain and SmallDomain after the fingerprint for the same structural reason and internal/core/CLAUDE.md records that deviation. Owed: decide where the unique-index pick belongs (classify cannot, it has no row count; plan can, but then the fingerprint has to be computed after it) and make the fingerprint cover what is actually masked with.

## Acceptance



## Log

- 2026-09-08 created

- 2026-09-08 moved to E5 phase 5

- 2026-09-08 closed: done: in T-HARD-A (ecc42ae)

## Post-mortem

Went well: landed. Went badly: nothing. Change: none.
