---
id: T-0076
title: "T-0076: source read-only setting per transaction, never a session GUC that leaks through a transaction-pooling PgBouncer; T9 reworded"
epic: E5
phase: 5
status: done
owner: opus
created: 2026-09-08
started: ""
closed: 2026-09-08
outcome: "done: 2d57b00; session GUC removed, SystemID in a read-only transaction, pgbouncer neighbour test proves a second client can CREATE TABLE after lazyslice exits, T9 reworded"
---

# T-0076 · T-0076: source read-only setting per transaction, never a session GUC that leaks through a transaction-pooling PgBouncer; T9 reworded

## Goal



## Acceptance



## Log

- 2026-09-08 created

- 2026-09-08 closed: done: 2d57b00; session GUC removed, SystemID in a read-only transaction, pgbouncer neighbour test proves a second client can CREATE TABLE after lazyslice exits, T9 reworded

## Post-mortem

Went well: the reviewers' pooler measurement made the decision unarguable, and the pgbouncer container from T-0050 made the proof cheap. Went badly: removing the rail exposed discover's three untransacted reads, filed by the developer to Later. Change: developer-filed tasks that restore a rail removed in the current phase are re-homed to the phase; tracker gained a move command.
