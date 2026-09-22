---
id: T-0294
title: "plan-time DEFAULT rewrite for a person_name column ignores Role"
epic: E9
phase: ""
status: done
owner: sonnet
created: 2026-09-22
started: ""
closed: 2026-09-22
outcome: done
---

# T-0294 · plan-time DEFAULT rewrite for a person_name column ignores Role

## Goal

internal/plan/ddlliteral.go:773 rewrites a masked DEFAULT literal by building its mask.Constraints from constraintsOf plus Unique and omits Role (T-0287), so a first_name column's recreated DEFAULT is rewritten as a two-word 'Given Family' pair (mask.RoleFull) while the column's own rows are masked one word at a time under RoleGiven/RoleFamily -- the comment at that file's line 838 names this exact class of defect (a default masked under different Constraints than the column's own rows) but this specific case was not caught. Set c.Role = d.Role before mask.Apply in the default-rewrite path, and check the CHECK-literal rewrite paths in the same file for the same omission. Add a regression: a first_name/last_name column with a masked DEFAULT must recreate a one-word default matching the column's own Role, the way testdata/regressions/011 pins masked-default: today. Found in T-0287's fix-round review (internal/plan is outside that task's paths).

## Acceptance

—

## Log

- 2026-09-22 2026-09-22 created
- 2026-09-22 closed: done

## Post-mortem

went well: 8894b3c: the DEFAULT rewrite sets c.Role = d.Role; unique-domain runs before the DDL pass so an agreed role reaches it; TestAMaskedNameDefaultIsMaskedUnderTheColumnsRole fails without the fix; no CHECK-literal path calls mask.Apply | went badly: nothing | change next time: nothing
