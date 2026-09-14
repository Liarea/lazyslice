---
id: T-0134
title: "Recreated DDL carries no sensitive literal: defaults on masked columns are masked, strong hits elsewhere refuse, verify scans the target catalog"
epic: E5
phase: 5
status: open
owner: opus
created: 2026-09-14
started: ""
closed: ""
outcome: ""
---

# T-0134 · Recreated DDL carries no sensitive literal: defaults on masked columns are masked, strong hits elsewhere refuse, verify scans the target catalog

## Goal

load/ddl copies a column default verbatim (internal/load/ddl/ddl.go:642), so DEFAULT ddl.canary@example.org on a masked email column survives into the target and a later INSERT resurrects the original: docs/reviews/2026-09-09/REVIEW.md finding 5, evidence/ddl_default.log. Rule: string literals inside a column default, a CHECK constraint or a generated-column expression are inside the data boundary. For a masked column, every string literal in its default is masked through the column masker (deterministic, so the default stays a working default); a CHECK or generated expression on a masked column that carries a literal the plan cannot rewrite refuses at exit 13 naming the object. For an unmasked column, a literal in any of those objects that a strong validator hits (email, phone, credit card) refuses at plan with exit 12 naming the object and offering --unmask with a reason. Verify gains a catalog pass: it reads pg_attrdef and pg_constraint text in the target and fails exit 9 on a strong hit. Regression under testdata/regressions: the review schema, expected ok, target default holds a masked address. Dated amendments in ARCHITECTURE.md section 11.1 and THREAT_MODEL.md T1.

## Acceptance



## Log

- 2026-09-14 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
