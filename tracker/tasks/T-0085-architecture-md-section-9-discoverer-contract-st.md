---
id: T-0085
title: "ARCHITECTURE.md section 9 Discoverer contract still says three statements per dial"
epic: E9
phase: ""
status: done
owner: ""
created: 2026-09-08
started: ""
closed: 2026-09-08
outcome: "done: ARCHITECTURE.md §9 Discoverer contract says three reads inside one read-only transaction"
---

# T-0085 · ARCHITECTURE.md section 9 Discoverer contract still says three statements per dial

## Goal

ARCHITECTURE.md:108, inside the Discoverer interface contract, says the dial runs 'at most three statements per candidate: version, the pg_class count and hint, and to_regclass(lazyslice_meta)'. Since T-0081 the dial runs five statements on the wire: those three reads wrapped in BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY ... ROLLBACK (internal/discover/probe.go). Amend that sentence to 'at most three reads per candidate, inside one REPEATABLE READ READ ONLY transaction', so the count in the spec is a count of reads and the transaction is part of the contract rather than an undocumented extra statement. ARCHITECTURE.md was outside the paths of T-0081 and of its fix round; internal/discover/CLAUDE.md records this amendment as owed.

## Acceptance



## Log

- 2026-09-08 created

- 2026-09-08 closed: done: ARCHITECTURE.md §9 Discoverer contract says three reads inside one read-only transaction

## Post-mortem

Went well: doc. Went badly: nothing. Change: none.
