---
id: T-0272
title: "Per-leaf categories for JSON documents on the Decision, so an email leaf is replaced by a fake email rather than filler"
epic: E9
phase: ""
status: open
owner: opus
created: 2026-09-17
started: ""
closed: ""
outcome: ""
---

# T-0272 · Per-leaf categories for JSON documents on the Decision, so an email leaf is replaced by a fake email rather than filler

## Goal

ARCHITECTURE.md section 14 listed 'the one-level JSON key collection' under v1 after Gate 4. The classifier half shipped (jsonLeaves and jsonLeafIsPersonal in internal/classify/validators.go walk a document's leaves, match keys against the rule pack and values against the validators, and a hit decides the column); the half that puts a category per leaf on the Decision, so internal/transform/json.go stops masking every leaf as free_text (its leafCategory constant) and verify can read the same map, did not. It touches no control: every string leaf of a masked JSON column is masked either way. It waits on the maintainer's arbitrary-JSON policy decision (T-0143): under whole-document replacement there are no leaves to categorise. Moved past v1 by the section 14 amendment of 2026-09-17; T-0087, T-0102 and T-0182 defer to it by name.

## Acceptance



## Log

- 2026-09-17 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
