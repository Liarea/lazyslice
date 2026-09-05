---
id: T-0002
title: "COMPETITORS.md teardown"
epic: E1
phase: 1
status: done
owner: opus
created: 2026-09-04
started: 2026-09-05
closed: 2026-09-05
outcome: "done: 636-line teardown of 23 tools, 156 sources, critiqued and revised"
---

# T-0002 · COMPETITORS.md teardown

## Goal



## Acceptance



## Log

- 2026-09-04 created

- 2026-09-05 started

- 2026-09-05 first draft written 2026-09-04; critique and revision pending (session limit interrupted run wf_ac7e4ed9-869)

- 2026-09-05 closed: done: 636-line teardown of 23 tools, 156 sources, critiqued and revised

## Post-mortem

Went well: authenticated gh api gave exact dates, licences, and reaction-sorted issues fast. Went badly: three HN item ids and a SourceForge thread were cited from inference and caught only by the link check. Change: verify every URL before writing it, never write an id that did not come back from a tool call.
