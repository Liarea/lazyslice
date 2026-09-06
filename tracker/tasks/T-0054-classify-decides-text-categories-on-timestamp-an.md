---
id: T-0054
title: "Classify decides text categories on timestamp and tsvector columns (pagila last_update credential, film.fulltext address); transform then refuses at exit 7 mid-run"
epic: E4
phase: 4
status: done
owner: opus
created: 2026-09-06
started: 2026-09-06
closed: 2026-09-06
outcome: "done: classify silences value signals on non-accepting families, derived_text for tsvector, plan-time write-back refusal, cross-stage integration test over both fixtures; ADR-010 records the rule"
---

# T-0054 · Classify decides text categories on timestamp and tsvector columns (pagila last_update credential, film.fulltext address); transform then refuses at exit 7 mid-run

## Goal

Blocks T-CORE: every full run over pagila dies. Value validators must run only on families the category accepts; a masker whose output cannot be written into the column type is refused at plan (exit 12), never at transform; tsvector gets a writable masker (empty tsvector).

## Acceptance



## Log

- 2026-09-06 created

- 2026-09-06 started

- 2026-09-06 closed: done: classify silences value signals on non-accepting families, derived_text for tsvector, plan-time write-back refusal, cross-stage integration test over both fixtures; ADR-010 records the rule

## Post-mortem

Went well: the fix closed the signal at its source and added the plan-time check core relies on. Went badly: the developer died once on a limit, and three follow-ups sat outside its paths (verify workaround, CatDerivedText home, the §4 sentence). Change: ADR-010 written by the orchestrator; a small task moves the constant and deletes the workaround before core.
