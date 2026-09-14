---
id: T-0133
title: "Core owns the run lifecycle: complete is written only after verify passes, and a residual failure empties the target"
epic: E5
phase: 5
status: open
owner: sonnet
created: 2026-09-14
started: ""
closed: ""
outcome: ""
---

# T-0133 · Core owns the run lifecycle: complete is written only after verify passes, and a residual failure empties the target

## Goal

load.Load writes status = complete (internal/load/load.go:184) before core runs verify (internal/core/run.go:1405), so a run that fails verification exits 9 with the leaked value still in the target and a marker that says complete: docs/reviews/2026-09-09/REVIEW.md finding 4, evidence/verify_failure.log. Fix: load leaves the marker at running (or a new status, loaded, if that is cleaner; pick one and say why in internal/pg/marker.go); core writes complete only after verify passes and failed after any verify failure. On an exit-9 class failure (residual scan or second net) core additionally drops every table it loaded before marking failed, because the target then holds personal data by definition and waiting for the next gate to truncate it leaves the leak on disk; on exit 8 (FK or row count) the data stays for diagnosis. THREAT_MODEL.md T8 gets a dated amendment stating both. Tests: the two-row column from the review reproduces exit 9 with an empty target and a marker at failed; TestKillNineLeavesRunningMarker still holds.

## Acceptance



## Log

- 2026-09-14 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
