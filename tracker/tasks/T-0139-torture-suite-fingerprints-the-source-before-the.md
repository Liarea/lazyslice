---
id: T-0139
title: "Torture suite fingerprints the source before the run, with a negative control"
epic: E5
phase: 5
status: done
owner: sonnet
created: 2026-09-14
started: 2026-09-14
closed: 2026-09-14
outcome: done
---

# T-0139 · Torture suite fingerprints the source before the run, with a negative control

## Goal

internal/invariants/torture_test.go runs the tool at line 56 and takes beforeRows and beforeCatalog at lines 72 and 73, after it, so a source mutation during the run cannot be detected and the I4 evidence the suite claims is weaker than documented: docs/reviews/2026-09-09/REVIEW.md finding 11. Move both fingerprints before runTool and add a negative control: a test that performs one UPDATE on the source between the baseline and the comparison and proves compareFingerprints fails.

## Acceptance



## Log

- 2026-09-14 created

- 2026-09-14 started

- 2026-09-14 closed: done

## Post-mortem

went well: baselines taken before the run, negative control on rows, I4 now held for refusing runs too (4487315, one fix round) | went badly: the Makefile's TORTURE_TESTS list did not include the negative control (T-0171, orchestrator fixes); comment overstated what the reorder buys and the catalog half has no negative control (low) | change next time: a negative control per comparison, not per test
