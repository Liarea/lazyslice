---
id: T-0036
title: "T-PG: source and target connections, target gate, statement-shape allowlist, dsn"
epic: E4
phase: 4
status: done
owner: opus
created: 2026-09-05
started: 2026-09-05
closed: 2026-09-06
outcome: "done: connections, marker, target gate, statement-shape tracer, dsn; package integration tests green"
---

# T-0036 · T-PG: source and target connections, target gate, statement-shape allowlist, dsn

## Goal



## Acceptance



## Log

- 2026-09-05 created

- 2026-09-05 started

- 2026-09-06 closed: done: connections, marker, target gate, statement-shape tracer, dsn; package integration tests green

## Post-mortem

Went well: one medium finding (a backslash-escape leak in the tracer's literal elision) was reproduced and fixed with a stronger-than-suggested check. Went badly: the task was reported blocked because verify ran the whole invariant suite, which must fail until core lands; the catalogue.yml rows were outside its paths. Change: pre-core stages verify their own package's integration tests; every stage owns its rows in catalogue.yml.
