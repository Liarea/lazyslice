---
id: T-0021
title: "Golden fixtures: Pagila and nasty.sql"
epic: E3
phase: 3
status: done
owner: opus
created: 2026-09-05
started: 2026-09-05
closed: 2026-09-05
outcome: "done: 31aa82e; Pagila pinned v3.1.0 with checksums, nasty.sql 21 tables, 22 traps documented, 2M-row generator, testutil loaders"
---

# T-0021 · Golden fixtures: Pagila and nasty.sql

## Goal



## Acceptance



## Log

- 2026-09-05 created

- 2026-09-05 started

- 2026-09-05 closed: done: 31aa82e; Pagila pinned v3.1.0 with checksums, nasty.sql 21 tables, 22 traps documented, 2M-row generator, testutil loaders

## Post-mortem

Went well: reviewers forced the loader from a 309-line mini-psql to 60 lines and added six tables for untested branches. Went badly: the developer had to edit fixtures_test.go outside its paths to satisfy findings; the .gitattributes LF pin was missed. Change: give fixture tasks the test file in paths; orchestrator adds .gitattributes at scaffold.
