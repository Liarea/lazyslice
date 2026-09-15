---
id: T-0179
title: "Bench CI gate compares head against its parent on the same runner; absolute baseline becomes a catastrophic floor"
epic: E5
phase: 5
status: done
owner: sonnet
created: 2026-09-15
started: 2026-09-15
closed: 2026-09-15
outcome: done
---

# T-0179 · Bench CI gate compares head against its parent on the same runner; absolute baseline becomes a catastrophic floor

## Goal

The blocking bench job failed on identical code: CI run 34927703066 measured 10,865,118 rows/sec and run 34928141791 measured 6,741,072 rows/sec, a 38 percent swing between two ubuntu-latest runs of the same benchmark, so a 20 percent ceiling against an absolute baseline cannot hold on shared runners, and a flaky bench job would block the release gate (T-0140 requires a green ci run on main). Fix: make bench-compare benchmarks BENCH_BASE (default HEAD~1, the PR base on pull requests) in a temporary worktree and the working tree, interleaved, best of five each, and fails on a 20 percent drop relative to base; make bench keeps baseline.json as a 3,000,000 rows/sec catastrophic floor only. Makefile, .github/workflows/ci.yml, internal/extract/testdata/bench/, docs/PERF.md, docs/RUNBOOK.md.

## Acceptance



## Log

- 2026-09-15 created

- 2026-09-15 started

- 2026-09-15 closed: done

## Post-mortem

went well: relative gate (worktree at the base, interleaved best-of-five, 20% ceiling) and a 3,000,000 rows/sec catastrophic floor; parsing factored once; local run against HEAD~1 within 0.5% | went badly: the absolute gate went blocking for exactly one push before the runner disproved it; PERF.md's prose had promised a runner baseline would fix it, which was the wrong theory | change next time: on shared runners compare against a same-machine control, never an absolute number
