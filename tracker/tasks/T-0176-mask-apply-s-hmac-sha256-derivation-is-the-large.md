---
id: T-0176
title: "mask.Apply's HMAC-SHA256 derivation is the largest CPU cost in the extract/transform/load pipeline"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-14
started: ""
closed: ""
outcome: ""
---

# T-0176 · mask.Apply's HMAC-SHA256 derivation is the largest CPU cost in the extract/transform/load pipeline

## Goal

T-PERF (docs/PERF.md) profiled a 2,000,000-row narrow extraction with pprof: mask.Apply (mask/, the per-value HMAC key-schedule and generator dispatch) accounted for roughly a quarter to a third of CPU samples end to end, well above anything internal/transform itself controls (transform's own allocation hotspot in bloom.positions was fixed under T-PERF). mask/ is a separate Go module (ADR-006) and was not among T-PERF's authorized paths, so this was profiled and reported, not fixed. Candidates worth investigating there: whether the per-category HKDF subkey (K_cat) can be derived once per column per run instead of read fresh, and whether the HMAC state per value can be pooled the way T-PERF pooled internal/transform's residual-filter one. docs/PERF.md has the pprof commands and the flame summary that found this.

## Acceptance



## Log

- 2026-09-14 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
