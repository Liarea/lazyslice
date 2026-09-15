---
id: T-0185
title: "ARCHITECTURE.md does not know about --allow-type-literal"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-15
started: ""
closed: ""
outcome: ""
---

# T-0185 · ARCHITECTURE.md does not know about --allow-type-literal

## Goal

Section 8's flag table and section 11.1's type-literal paragraph both predate the per-type opt-out the T-REDFIX review's fourth finding required: 11.1 still names --skip-table as the escape for an enum label or a domain definition refused at exit 13, and --skip-table cannot clear that refusal (it drops a table to schema only, and the type is recreated regardless). The flag exists and is honoured by internal/plan (checkTypeLiterals) and internal/verify (catalogExempt); ARCHITECTURE.md was outside the task's paths. Add the row to section 8's table and correct section 11.1.

## Acceptance



## Log

- 2026-09-15 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
