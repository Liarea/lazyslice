---
id: T-0070
title: "T-PIN: the run that writes the target is pinned to the reviewed snapshot and endpoints (core.Request.Reviewed, core.refused.reviewed_changed)"
epic: E5
phase: 5
status: done
owner: opus
created: 2026-09-08
started: ""
closed: 2026-09-08
outcome: "done: af911ac; core.Preview, core.Request.Reviewed, core.refused.reviewed_changed; structural test pins the wiring"
---

# T-0070 · T-PIN: the run that writes the target is pinned to the reviewed snapshot and endpoints (core.Request.Reviewed, core.refused.reviewed_changed)

## Goal



## Acceptance



## Log

- 2026-09-08 created

- 2026-09-08 closed: done: af911ac; core.Preview, core.Request.Reviewed, core.refused.reviewed_changed; structural test pins the wiring

## Post-mortem

Went well: the pin is structural and tested. Went badly: a third entry point widened core's surface; a stray binary nearly shipped. Change: §1 records the three entry points; /lazyslice ignored.
