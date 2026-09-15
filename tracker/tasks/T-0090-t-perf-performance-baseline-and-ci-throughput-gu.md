---
id: T-0090
title: "T-PERF: performance baseline and CI throughput guard"
epic: E5
phase: 5
status: done
owner: opus
created: 2026-09-08
started: 2026-09-15
closed: 2026-09-15
outcome: done
---

# T-0090 · T-PERF: performance baseline and CI throughput guard

## Goal



## Acceptance



## Log

- 2026-09-08 created

- 2026-09-14 2026-09-14 trimmed for v0.1.0: 2,000,000-row child, 60-second target, byte-limited batches kept; the 20M-row run moved to T-0155 (0.2)

- 2026-09-14 2026-09-14 correction: the 20M-row follow-up is T-0157, not T-0155

- 2026-09-15 started

- 2026-09-15 closed: done

## Post-mortem

went well: 5,000 roots over a 2,000,000-row child profiled with pprof; two hotspots fixed (bloom positions allocation, pooled HMAC with cached column bytes); byte-capped batches so 2,000 one-MiB values no longer form one batch, with peak RSS measured for wide text and jsonb rows; make bench with a recorded baseline and a CI job; docs/PERF.md carries before and after per fix | went badly: the independent verify failed on a fixed-port collision in an unrelated discover test (5433) while make integration ran alongside other containers; re-run alone it passes; the CI bench job is non-blocking until a baseline is recorded on the runner itself (T-0177) | change next time: tests that bind fixed host ports pick a free one (filed)
