---
id: T-0163
title: "the plan-time DDL literal rule does not read index predicates or domain CHECKs"
epic: E5
phase: 5
status: done
owner: ""
created: 2026-09-14
started: ""
closed: 2026-09-15
outcome: done
---

# T-0163 · the plan-time DDL literal rule does not read index predicates or domain CHECKs

## Goal

internal/plan/ddlliteral.go (T-0134) walks every recreated column default, generated expression and table CHECK/exclusion constraint, and nothing else. ARCHITECTURE.md section 11.1 also recreates indexes verbatim (pg_get_indexdef, so a partial index's WHERE clause can carry a literal) and domains with their CHECK text (pipeline.Schema.Domains), and a literal in either reaches the target unexamined by the planner. internal/verify/catalog.go catches a domain CHECK, because pg_constraint holds it, and catches neither an index predicate nor anything at plan time, so the failure arrives at exit 9 after the target is loaded rather than at exit 12 or 13 before anything is dropped. Extend checkDDLLiterals over Table.Indexes and Schema.Domains, and add the index predicate to the verify pass by reading pg_index.indpred.

## Acceptance



## Log

- 2026-09-14 created

- 2026-09-14 Narrowed by T-0134's review round (2026-09-14): internal/verify/catalog.go now reads pg_index.indpred and pg_index.indexprs as well as pg_attrdef and pg_constraint, so the verify half of this task is done and an index predicate carrying a strong literal is exit 9 after the load. What remains is the plan half only: extend checkDDLLiterals over Table.Indexes and Schema.Domains so the same literal is exit 12 or 13 before anything is dropped.

- 2026-09-15 moved to E5 phase 5

- 2026-09-15 closed: done

## Post-mortem

went well: closed by T-0189, which reads partial-index predicates and expression keys at plan time and domain CHECKs in both passes | went badly: sat open in E5 for a week as a known gap | change next time: file the gap inside the task that owns the code path when one is already queued
