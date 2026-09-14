---
id: T-0119
title: "A table-scoped name rule, for refresh_tokens.parent and its kind"
epic: E5
phase: 5
status: open
owner: ""
created: 2026-09-08
started: ""
closed: ""
outcome: ""
---

# T-0119 · A table-scoped name rule, for refresh_tokens.parent and its kind

## Goal

T-0104's tenth miss has no fix in T-HARD-B and needs a rule-pack feature rather than a rule. supabase-auth's refresh_tokens.parent holds another refresh token in a column named after a tree edge: a credential pattern matching 'parents?' would match parent_id in every schema there is, at priority 80, and mask half a database's join keys to the fixed literal. internal/classify's name rules see the column name alone, so 'parent, in a table called refresh_tokens' is not expressible; rules.yml's tables: section carries one rule (log_shaped) and is not a general table+column matcher. Owed: decide whether the rule pack gains a table-scoped pattern (table regexp plus column regexp, one category), and if so add it with the confusion matrix re-measured. internal/classify/supabase_misses_test.go pins the column as still copied and says why.

## Acceptance



## Log

- 2026-09-08 created

- 2026-09-09 Orchestrator 2026-09-09: stays in Later; a table-scoped rule is a rule-pack refinement, not a gap in coverage (the column is masked by its type family today).

- 2026-09-14 moved to E5 phase 5

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
