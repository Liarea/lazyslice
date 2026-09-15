---
id: T-0119
title: "A table-scoped name rule, for refresh_tokens.parent and its kind"
epic: E5
phase: 5
status: done
owner: ""
created: 2026-09-08
started: 2026-09-14
closed: 2026-09-14
outcome: done
---

# T-0119 · A table-scoped name rule, for refresh_tokens.parent and its kind

## Goal

T-0104's tenth miss has no fix in T-HARD-B and needs a rule-pack feature rather than a rule. supabase-auth's refresh_tokens.parent holds another refresh token in a column named after a tree edge: a credential pattern matching 'parents?' would match parent_id in every schema there is, at priority 80, and mask half a database's join keys to the fixed literal. internal/classify's name rules see the column name alone, so 'parent, in a table called refresh_tokens' is not expressible; rules.yml's tables: section carries one rule (log_shaped) and is not a general table+column matcher. Owed: decide whether the rule pack gains a table-scoped pattern (table regexp plus column regexp, one category), and if so add it with the confusion matrix re-measured. internal/classify/supabase_misses_test.go pins the column as still copied and says why.

## Acceptance



## Log

- 2026-09-08 created

- 2026-09-09 Orchestrator 2026-09-09: stays in Later; a table-scoped rule is a rule-pack refinement, not a gap in coverage (the column is masked by its type family today).

- 2026-09-14 moved to E5 phase 5

- 2026-09-14 started

- 2026-09-14 closed: done

## Post-mortem

went well: rules.yml gains a table-scoped pattern; refresh_tokens.parent is masked; two tests kill the reviewer's mutation of the table gate; docs/TORTURE.md provenance stated (802395a, one fix round) | went badly: make torture not re-run by the task; a pre-existing torture failure on regression 013 surfaced as T-0172 | change next time: any classify change re-runs make torture before returning
