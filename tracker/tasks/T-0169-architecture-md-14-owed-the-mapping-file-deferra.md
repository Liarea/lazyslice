---
id: T-0169
title: "ARCHITECTURE.md §14 owed the mapping_file deferral"
epic: E9
phase: ""
status: done
owner: ""
created: 2026-09-14
started: 2026-09-14
closed: 2026-09-14
outcome: done
---

# T-0169 · ARCHITECTURE.md §14 owed the mapping_file deferral

## Goal

§14's v1 cut line lists mapping_file inside v1 on ADR-006's word; ADR-012 (T-0138) defers it to T-0142 and §14 needs the same correction docs/reviews/2026-09-09/REVIEW.md finding 10 asked for. File: ARCHITECTURE.md section 14.

## Acceptance



## Log

- 2026-09-14 created

- 2026-09-14 Review of T-0138 (reviewers, 2026-09-14) found ARCHITECTURE.md drift wider than section 14: section 5 (line ~880) still states the unique refusal prints three escapes including mapping_file:, section 9 (line ~1070) still says lazyslice protects the file when it 'first reads a yml naming a mapping_file:', and the ColumnConfig listing (line ~684) still shows the removed MappingFile field. This task's scope is widened to cover all four: section 14's v1 cut line, section 5's three-escapes sentence, section 9's mapping_file clause, and the ColumnConfig listing — not section 14 alone.

- 2026-09-14 started

- 2026-09-14 closed: done

## Post-mortem

went well: ARCHITECTURE sections 5, 9 and 14 amended by the orchestrator with the landing | change next time: nothing
