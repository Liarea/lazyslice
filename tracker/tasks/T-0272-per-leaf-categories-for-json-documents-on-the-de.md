---
id: T-0272
title: "Per-leaf categories for JSON documents on the Decision, so an email leaf is replaced by a fake email rather than filler"
epic: E6
phase: 6
status: done
owner: opus
created: 2026-09-17
started: ""
closed: 2026-09-25
outcome: done
---

# T-0272 · Per-leaf categories for JSON documents on the Decision, so an email leaf is replaced by a fake email rather than filler

## Goal

ARCHITECTURE.md section 14 listed 'the one-level JSON key collection' under v1 after Gate 4. The classifier half shipped (jsonLeaves and jsonLeafIsPersonal in internal/classify/validators.go walk a document's leaves, match keys against the rule pack and values against the validators, and a hit decides the column); the half that puts a category per leaf on the Decision, so internal/transform/json.go stops masking every leaf as free_text (its leafCategory constant) and verify can read the same map, did not. It touches no control: every string leaf of a masked JSON column is masked either way. It waits on the maintainer's arbitrary-JSON policy decision (T-0143): under whole-document replacement there are no leaves to categorise. Moved past v1 by the section 14 amendment of 2026-09-17; T-0087, T-0102 and T-0182 defer to it by name.

## Acceptance

—

## Log

- 2026-09-23 2026-09-23 Dogfood session 1 (2026-09-23): the evidence for per-leaf categories is in T-0143's log entry of the same date; the configuration documents that broke the copy had no personal leaf at all, so the same design must also let a signal-free leaf be copied, not only route an email leaf to the email masker.
- 2026-09-24 2026-09-24 moved to E6 phase 6
- 2026-09-24 2026-09-24 Decision 2026-09-24 (T-0143, the maintainer): per-leaf classification. A leaf whose key or value carries a personal signal (the classifier's name rules on the key, its validators on the value) is masked by that category's masker; a leaf with no signal is COPIED, not filled; keys stay; the log-shaped-table rule (whole-document replacement) stays for audit and log tables. This is the phase-6 task now: dogfood showed an AI model's config, ad-server settings, trigger actions and deployment rules turned to word salad and a latitude of 573 (T-0324 covers the geo range). THREAT_MODEL T1 and SECURITY item 8 change with it; a red-team round on JSON documents follows the landing.
- 2026-09-25 closed: done

## Post-mortem

went well: landed as 1e061d3 by Opus after one review round with two reviewers: a json document is masked leaf by leaf as the maintainer decided (T-0143, 2026-09-24), a personal leaf by its own category and a signal-free leaf under sampled keys copied; the review caught the first draft ignoring the column's own decision, the phone region, and a key-order dependence above 4,096 keys, and the fix round closed all three with mutation-checked tests | went badly: this task masks less than before on purpose, and the JSON red-team round run the same night (docs/reviews/2026-09-25-redteam-json/round1.json: 37 attempts, 27 leaks, 11 outside the named residuals) found the first draft's promise broken in one place the review did not reach: a jsonb column whose own name is personal falls back to plain semi_structured and every leaf is copied; that and ten more became the eight jsonfix tasks filed 2026-09-25, replayed before v0.4.0; the workflow's verify step was forced out before make torture finished, so the orchestrator reran the gate and landed by hand; T-0390 was filed by the first attempt and then done by the fix round | change next time: a task that reverses a masking rule gets its red-team round in the same batch, before the close, not after
