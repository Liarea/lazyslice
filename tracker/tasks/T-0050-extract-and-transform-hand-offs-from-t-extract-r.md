---
id: T-0050
title: "Extract and transform hand-offs from T-EXTRACT review (see T-0041 log): shape-template identifier escaping, KeySet chunk iterator, pgbouncer testcontainer, text-keyed big fixture"
epic: E5
phase: 5
status: open
owner: opus
created: 2026-09-06
started: ""
closed: ""
outcome: ""
---

# T-0050 · Extract and transform hand-offs from T-EXTRACT review (see T-0041 log): shape-template identifier escaping, KeySet chunk iterator, pgbouncer testcontainer, text-keyed big fixture

## Goal



## Acceptance



## Log

- 2026-09-06 created

- 2026-09-06 Widened: FirstChunk(n int) Chunk is owed on ARCHITECTURE.md §2's KeySet and on internal/plan's two implementations; internal/verify/sample.go type-asserts for it and falls back to materialising every chunk (a full second copy of the key set at verify time, after --memory-budget can no longer refuse). Also: tsvector and enum columns are skipped by the second net (famOther never read).

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
