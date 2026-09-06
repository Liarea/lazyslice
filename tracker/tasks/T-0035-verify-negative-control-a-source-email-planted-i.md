---
id: T-0035
title: "Verify negative control: a source email planted in a masked target column makes lazyslice verify exit 9 naming table and column"
epic: E4
phase: 4
status: done
owner: opus
created: 2026-09-05
started: ""
closed: 2026-09-06
outcome: "done: implemented inside T-VERIFY's integration suite"
---

# T-0035 · Verify negative control: a source email planted in a masked target column makes lazyslice verify exit 9 naming table and column

## Goal

Section 6 items 1 to 3; closes the gap that a verify stage returning nil passes all six invariants

## Acceptance

Integration test in internal/invariants or internal/verify; referenced from the comment above flaggedCodePrefixes

## Log

- 2026-09-05 created

- 2026-09-06 closed: done: implemented inside T-VERIFY's integration suite

## Post-mortem

Went well: planted source email makes verify exit 9 naming table and column. Went badly: nothing. Change: none.
