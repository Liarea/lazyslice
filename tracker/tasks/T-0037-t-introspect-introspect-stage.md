---
id: T-0037
title: "T-INTROSPECT: introspect stage"
epic: E4
phase: 4
status: done
owner: opus
created: 2026-09-05
started: 2026-09-06
closed: 2026-09-06
outcome: "done: 2dd2102 (stage) after fix2; PG18 contype filter, tolerant sampling, bounded TABLESAMPLE, FK end filters, extension walk narrowed, partition edges re-pointed only when the root can carry them"
---

# T-0037 · T-INTROSPECT: introspect stage

## Goal



## Acceptance



## Log

- 2026-09-05 created

- 2026-09-06 started

- 2026-09-06 closed: done: 2dd2102 (stage) after fix2; PG18 contype filter, tolerant sampling, bounded TABLESAMPLE, FK end filters, extension walk narrowed, partition edges re-pointed only when the root can carry them

## Post-mortem

Went well: reviewers reproduced every finding on a live container, including two defects the fixes themselves introduced, and the developer withdrew a wrong 'checked' claim after testing it. Went badly: two fix rounds were not enough and a third targeted task was needed; ARCHITECTURE.md's type list did not anticipate indimmediate. Change: a developer concern that says 'checked' must name the command; the orchestrator should expect a third round on catalogue-heavy stages.
