---
id: T-0189
title: "Catalog literals: every validator over string literals in CHECK, domain, enum and generated expressions; pattern operands detected but not rewritten; plan reads partial-index predicates"
epic: E5
phase: 5
status: done
owner: sonnet
created: 2026-09-15
started: 2026-09-15
closed: 2026-09-15
outcome: done
---

# T-0189 · Catalog literals: every validator over string literals in CHECK, domain, enum and generated expressions; pattern operands detected but not rewritten; plan reads partial-index predicates

## Goal

Red team 2026-09-15 R2-07, R2-08, R2-09, R2-10: internal/verify/catalog.go strongCatalogHit and internal/plan/ddlliteral.go apply only the five strong validators to catalog literals, so a person name or a postal address in a CHECK, a domain CHECK or DEFAULT, an enum label or a generated expression crosses; a value on the right-hand side of a pattern operator (LIKE, ~) is exempt from detection entirely; and internal/plan reads no pg_index, so a partial index predicate holding an address is only seen by the post-load catalog pass (T-0163). Fix: string literals in expressions go through every validator (the dictionary false-positive argument applies to identifiers, not literals); a pattern-operand literal is exempt from rewriting only, never from detection; plan reads pg_index predicates for every index it will recreate and refuses or rewrites before anything is dropped. Regression fixtures for each of the four. Promotes T-0163.

## Acceptance



## Log

- 2026-09-15 created

- 2026-09-15 started

- 2026-09-15 closed: done

## Post-mortem

went well: the three rules composed cleanly and closed R2-09's type arm and the masked and unmasked arms without touching their callers; the Opus reviewer measured AddressShape against ordinary CHECK text (Basic 1 user, P1 High Priority) and found a one-hit refusal with no escape, and found the DEFAULT path still passing a pattern operand; both fixed with tests, one reverify round clean | went badly: the first cut put the credential validator in the strong set and only make torture caught the nextval() false positive; the fixer had to invent a street-suffix corroboration because textsig was outside its paths; the special-category sentence is still only caught by accident (T-0198) | change next time: a task that widens a strong set names the false-positive budget per validator up front, and a fix that needs a package outside the paths goes back to the orchestrator for a decision instead of a local duplicate
