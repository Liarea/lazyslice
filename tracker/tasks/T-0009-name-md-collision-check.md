---
id: T-0009
title: "NAME.md collision check"
epic: E1
phase: 1
status: done
owner: opus
created: 2026-09-04
started: 2026-09-05
closed: 2026-09-05
outcome: "done: lazysnap rejected (Go module, npm, GitHub account, DBSnapper adjacency, wrong semantics); renamed to lazyslice per ADR-000"
---

# T-0009 · NAME.md collision check

## Goal



## Acceptance



## Log

- 2026-09-04 created

- 2026-09-05 started

- 2026-09-05 first draft written 2026-09-04; critique and revision pending (session limit interrupted run wf_ac7e4ed9-869)

- 2026-09-05 closed: done: lazysnap rejected (Go module, npm, GitHub account, DBSnapper adjacency, wrong semantics); renamed to lazyslice per ADR-000

## Post-mortem

Went well: every registry check reproducible via direct API calls. Went badly: first draft had numeric errors the reviewer caught. Change: the semantic argument mattered more than the collision table; lead with it next time.
