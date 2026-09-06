---
id: T-0041
title: "T-EXTRACT: extract and transform stages"
epic: E4
phase: 4
status: done
owner: opus
created: 2026-09-05
started: 2026-09-06
closed: 2026-09-06
outcome: "done: 4fd5c28; chunked typed unnest extract with bounded memory (2M rows at 19 MiB growth), transform with JSON leaf masking and per-leaf residual digests, source pool read-only by session SET, extract shapes moved home"
---

# T-0041 · T-EXTRACT: extract and transform stages

## Goal



## Acceptance



## Log

- 2026-09-05 created

- 2026-09-06 started

- 2026-09-06 closed: done: 4fd5c28; chunked typed unnest extract with bounded memory (2M rows at 19 MiB growth), transform with JSON leaf masking and per-leaf residual digests, source pool read-only by session SET, extract shapes moved home

- 2026-09-06 Hand-offs: internal/plan/CLAUDE.md composed-allowlist bullet still names pg.ExtractShapes (deleted); pipeline.KeySet needs a chunk-at-a-time iterator to bound peak memory (§2 change); lookupShapeFor interpolates a quoted table name into a shape template so a table named with a {token} widens or breaks the allowlist (escape or pre-quoted literal segment); maskDocument ignores Decision.Masker; a pgbouncer testcontainer is owed to test the pooler-safe startup list; a big text- or uuid-keyed fixture table is owed for the memory test.

## Post-mortem

Went well: nine findings fixed in two rounds; the 2M-row memory test is real. Went badly: the developer had to edit plan/shapes_test.go outside its paths to keep the build green, and plan/CLAUDE.md is now stale; the JSON string-leaf rule deviates from §4 (all string leaves are free_text) for a stated reason. Change: record the deviation in ARCHITECTURE.md; let stage tasks write sibling test files that their deletions break.
