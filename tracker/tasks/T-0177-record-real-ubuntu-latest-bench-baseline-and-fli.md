---
id: T-0177
title: "Record real ubuntu-latest bench baseline and flip bench job to blocking"
epic: E5
phase: 5
status: open
owner: ""
created: 2026-09-14
started: ""
closed: ""
outcome: ""
---

# T-0177 · Record real ubuntu-latest bench baseline and flip bench job to blocking

## Goal

internal/extract/testdata/bench/baseline.json (15,000,000 rows/sec) was recorded on an Apple M4, never on the ubuntu-latest runner .github/workflows/ci.yml's bench job actually runs on; a GitHub-hosted x86 runner is typically 2-3x slower single-thread than the M4 this in-memory benchmark was measured on, so the job (wired as continue-on-error: true pending this task, T-PERF review) is likely to read well below the recorded baseline on every real run. Push a branch, read the bench job's own printed best-of-5 rows/sec from a real ubuntu-latest run, replace baseline.json's value with that number, set BENCH_REGRESSION_PCT's effective floor to a real 20% off real hardware, then remove continue-on-error: true so the job gates again.

## Acceptance



## Log

- 2026-09-14 created

- 2026-09-15 moved to E5 phase 5

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
