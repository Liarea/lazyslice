---
id: T-0163
title: "the plan-time DDL literal rule does not read index predicates or domain CHECKs"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-14
started: ""
closed: ""
outcome: ""
---

# T-0163 · the plan-time DDL literal rule does not read index predicates or domain CHECKs

## Goal

internal/plan/ddlliteral.go (T-0134) walks every recreated column default, generated expression and table CHECK/exclusion constraint, and nothing else. ARCHITECTURE.md section 11.1 also recreates indexes verbatim (pg_get_indexdef, so a partial index's WHERE clause can carry a literal) and domains with their CHECK text (pipeline.Schema.Domains), and a literal in either reaches the target unexamined by the planner. internal/verify/catalog.go catches a domain CHECK, because pg_constraint holds it, and catches neither an index predicate nor anything at plan time, so the failure arrives at exit 9 after the target is loaded rather than at exit 12 or 13 before anything is dropped. Extend checkDDLLiterals over Table.Indexes and Schema.Domains, and add the index predicate to the verify pass by reading pg_index.indpred.

## Acceptance



## Log

- 2026-09-14 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
