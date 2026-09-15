---
id: T-0209
title: "ARCHITECTURE.md describes the design as PostgreSQL-specific until a second engine is real"
epic: E9
phase: ""
status: open
owner: sonnet
created: 2026-09-15
started: ""
closed: ""
outcome: ""
---

# T-0209 · ARCHITECTURE.md describes the design as PostgreSQL-specific until a second engine is real

## Goal

docs/reviews/2026-09-09/REVIEW.md, 'The engine abstraction is aspirational': catalog SQL, key queries, casts, schema reconstruction and verification live across the stage packages, so a second engine is not another implementation of Source. Say so in ARCHITECTURE.md sections 2 and 9 and in an ADR that succeeds ADR-003's engine wording, instead of implying a backend boundary that does not exist; extract one only when a real second-engine requirement reveals it.

## Acceptance



## Log

- 2026-09-15 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
