---
id: T-0050
title: "Extract and transform hand-offs from T-EXTRACT review (see T-0041 log): shape-template identifier escaping, KeySet chunk iterator, pgbouncer testcontainer, text-keyed big fixture"
epic: E5
phase: 5
status: done
owner: opus
created: 2026-09-06
started: ""
closed: 2026-09-08
outcome: "done: 2694a03; shape-template identifiers escaped, KeySet FirstChunk and EachChunk with extract and verify using them, pgbouncer testcontainer, text-keyed big fixture; §2 reconciled by the orchestrator"
---

# T-0050 · Extract and transform hand-offs from T-EXTRACT review (see T-0041 log): shape-template identifier escaping, KeySet chunk iterator, pgbouncer testcontainer, text-keyed big fixture

## Goal



## Acceptance



## Log

- 2026-09-06 created

- 2026-09-06 Widened: FirstChunk(n int) Chunk is owed on ARCHITECTURE.md §2's KeySet and on internal/plan's two implementations; internal/verify/sample.go type-asserts for it and falls back to materialising every chunk (a full second copy of the key set at verify time, after --memory-budget can no longer refuse). Also: tsvector and enum columns are skipped by the second net (famOther never read).

- 2026-09-08 closed: done: 2694a03; shape-template identifiers escaped, KeySet FirstChunk and EachChunk with extract and verify using them, pgbouncer testcontainer, text-keyed big fixture; §2 reconciled by the orchestrator

## Post-mortem

Went well: the developer consolidated every owed note into one place with a ready-to-paste fix. Went badly: three rounds were spent on findings whose only remedy was orchestrator work, and the review found a real pooler leak in passing. Change: developers may now file owed tasks themselves (root CLAUDE.md); the leak is T-0076.
