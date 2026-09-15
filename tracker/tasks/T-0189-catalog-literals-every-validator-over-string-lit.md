---
id: T-0189
title: "Catalog literals: every validator over string literals in CHECK, domain, enum and generated expressions; pattern operands detected but not rewritten; plan reads partial-index predicates"
epic: E5
phase: 5
status: open
owner: sonnet
created: 2026-09-15
started: ""
closed: ""
outcome: ""
---

# T-0189 · Catalog literals: every validator over string literals in CHECK, domain, enum and generated expressions; pattern operands detected but not rewritten; plan reads partial-index predicates

## Goal

Red team 2026-09-15 R2-07, R2-08, R2-09, R2-10: internal/verify/catalog.go strongCatalogHit and internal/plan/ddlliteral.go apply only the five strong validators to catalog literals, so a person name or a postal address in a CHECK, a domain CHECK or DEFAULT, an enum label or a generated expression crosses; a value on the right-hand side of a pattern operator (LIKE, ~) is exempt from detection entirely; and internal/plan reads no pg_index, so a partial index predicate holding an address is only seen by the post-load catalog pass (T-0163). Fix: string literals in expressions go through every validator (the dictionary false-positive argument applies to identifiers, not literals); a pattern-operand literal is exempt from rewriting only, never from detection; plan reads pg_index predicates for every index it will recreate and refuses or rewrites before anything is dropped. Regression fixtures for each of the four. Promotes T-0163.

## Acceptance



## Log

- 2026-09-15 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
