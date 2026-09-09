---
id: T-0120
title: "internal/classify: a masked FK child whose parent is copied orphans the row"
epic: E9
phase: ""
status: done
owner: ""
created: 2026-09-08
started: ""
closed: 2026-09-09
outcome: "done: in T-HARD-B (78530ef)"
---

# T-0120 · internal/classify: a masked FK child whose parent is copied orphans the row

## Goal

T-HARD-B review. propagateKeys runs parent to child only, so a key-family child that reaches ConfPossible on a name hit alone loses the surrogate-key exemption (classify.go markNeverMasked) while the parent primary key keeps it: the child is masked, the parent is copied verbatim, the load adds the edge NOT VALID, and internal/verify/fk.go counts the orphans and fails the run at exit 8. Masking a child whose parent is copied also protects no value, since the value is still in the parent. The online_id_idp rule in rules.yml is anchored to the whole column name to keep the common compound spellings (sso_provider_id and friends) out of it, which is a narrowing and not the fix; a column called exactly provider_id on a uuid foreign key still orphans. Decide the reconciliation: lift the child back to exempt when its parent is exempt, or propagate the child's category up to the parent, and say which in internal/classify/CLAUDE.md.

## Acceptance



## Log

- 2026-09-08 created

- 2026-09-09 closed: done: in T-HARD-B (78530ef)

## Post-mortem

Went well: landed. Went badly: nothing. Change: none.
