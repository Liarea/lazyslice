---
id: T-0175
title: "Give nasty.sql's stream fixtures a size parameter for perf profiling"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-14
started: ""
closed: ""
outcome: ""
---

# T-0175 · Give nasty.sql's stream fixtures a size parameter for perf profiling

## Goal

T-PERF (docs/PERF.md) needed a 5,000-row root table and a 2,000,000-row child table to profile the pipeline, but testdata/nasty.sql and internal/testutil (LoadNasty) are not among T-PERF's authorized paths (internal/extract/, internal/load/, internal/transform/, docs/PERF.md, Makefile, .github/), so it could not add a size parameter there. fill_stream_rows(n)/fill_stream_docs(n) already take n as a SQL parameter; only the psql gate (-v big=1) and testutil.LoadNasty hardcode 2,000,000/1,000,000. T-0157 (the 20,000,000-row run) will want the same parameter. T-PERF's own perf_profile_test.go seeds a throwaway schema of its own in internal/extract instead, which is a workaround and not a fix.

## Acceptance



## Log

- 2026-09-14 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
