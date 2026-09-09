---
id: T-0096
title: "testdata/CLAUDE.md and testdata/README.md still say 'two fixtures and nothing else'"
epic: E5
phase: 5
status: done
owner: ""
created: 2026-09-08
started: ""
closed: 2026-09-09
outcome: "done: in T-HARD-C (4f9a186)"
---

# T-0096 · testdata/CLAUDE.md and testdata/README.md still say 'two fixtures and nothing else'

## Goal

T-TORTURE added testdata/torture/ (ten real schemas, 1,023 tables) and testdata/regressions/ (seven reduced defects), and testdata/CLAUDE.md's opening line — 'Two fixtures and nothing else: pagila/ and nasty.sql' — is now false, as is testdata/README.md's 'Two PostgreSQL fixtures'. Both files were outside T-TORTURE's paths (testdata/torture/ only). Owed: testdata/CLAUDE.md gains testdata/torture/ and testdata/regressions/ with a pointer to each directory's own CLAUDE.md/README.md and a line saying which of the three a new fixture belongs in; testdata/README.md's opening paragraph says the same. Do not restate the torture rules there — testdata/torture/README.md is the spec.

## Acceptance



## Log

- 2026-09-08 created

- 2026-09-08 moved to E5 phase 5

- 2026-09-09 closed: done: in T-HARD-C (4f9a186)

## Post-mortem

Went well: landed. Went badly: nothing. Change: none.
