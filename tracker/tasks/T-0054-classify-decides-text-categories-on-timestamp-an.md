---
id: T-0054
title: "Classify decides text categories on timestamp and tsvector columns (pagila last_update credential, film.fulltext address); transform then refuses at exit 7 mid-run"
epic: E4
phase: 4
status: in_progress
owner: opus
created: 2026-09-06
started: 2026-09-06
closed: ""
outcome: ""
---

# T-0054 · Classify decides text categories on timestamp and tsvector columns (pagila last_update credential, film.fulltext address); transform then refuses at exit 7 mid-run

## Goal

Blocks T-CORE: every full run over pagila dies. Value validators must run only on families the category accepts; a masker whose output cannot be written into the column type is refused at plan (exit 12), never at transform; tsvector gets a writable masker (empty tsvector).

## Acceptance



## Log

- 2026-09-06 created

- 2026-09-06 started

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
