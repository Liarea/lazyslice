---
id: T-0045
title: "T-DISCOVER: discovery rungs 0-3 and the first-run ladder"
epic: E4
phase: 4
status: done
owner: opus
created: 2026-09-05
started: 2026-09-07
closed: 2026-09-07
outcome: "done: 5381042; rungs 0 to 3, six-step Docker endpoint resolution, compose and .env as naming sources, one blocking question, headless asks nothing; unit tests green"
---

# T-0045 · T-DISCOVER: discovery rungs 0-3 and the first-run ladder

## Goal



## Acceptance



## Log

- 2026-09-05 created

- 2026-09-07 started

- 2026-09-07 closed: done: 5381042; rungs 0 to 3, six-step Docker endpoint resolution, compose and .env as naming sources, one blocking question, headless asks nothing; unit tests green

## Post-mortem

Went well: the ladder landed with the locality predicate and refusals from ADR-008; reviewers traced the provenance loss through core and emit before it reached a user. Went badly: firstRun landed in cmd/ because core was outside the paths, breaking cmd/CLAUDE.md's rule; the developer wrote tracker/ directly, flagged but still a boundary crossing. Change: discovery tasks get internal/core in paths; a fix task moves firstRun into core and threads provenance through.
