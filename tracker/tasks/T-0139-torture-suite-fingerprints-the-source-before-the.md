---
id: T-0139
title: "Torture suite fingerprints the source before the run, with a negative control"
epic: E5
phase: 5
status: open
owner: sonnet
created: 2026-09-14
started: ""
closed: ""
outcome: ""
---

# T-0139 · Torture suite fingerprints the source before the run, with a negative control

## Goal

internal/invariants/torture_test.go runs the tool at line 56 and takes beforeRows and beforeCatalog at lines 72 and 73, after it, so a source mutation during the run cannot be detected and the I4 evidence the suite claims is weaker than documented: docs/reviews/2026-09-09/REVIEW.md finding 11. Move both fingerprints before runTool and add a negative control: a test that performs one UPDATE on the source between the baseline and the comparison and proves compareFingerprints fails.

## Acceptance



## Log

- 2026-09-14 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
