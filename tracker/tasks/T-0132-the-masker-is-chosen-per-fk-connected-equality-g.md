---
id: T-0132
title: "The masker is chosen per FK-connected equality group, not per column"
epic: E5
phase: 5
status: done
owner: opus
created: 2026-09-14
started: 2026-09-14
closed: 2026-09-14
outcome: done
---

# T-0132 · The masker is chosen per FK-connected equality group, not per column

## Goal

classify propagates a category along foreign keys (internal/classify/classify.go:1065) but the unique escalation in the plan (internal/plan/unique.go:138) replaces one column masker at a time, so tokens.token (unique) received credential_unique while items.token, its FK child, kept the fixed literal; equal inputs masked to different outputs and the load failed at exit 8: docs/reviews/2026-09-09/REVIEW.md finding 3, evidence/fk_masker.log. Fix: the plan computes equality groups (columns connected by any foreign key, transitively, plus whatever classify already unified) and picks one masker for the whole group: the widest generator any member needs, checked to fit every member type and length; if no single generator fits every member, refuse at exit 12 naming the group and every column in it. Transform masks every member identically once Decision.Masker agrees. Regression under testdata/regressions with the two-table schema from the review, expected ok, target FK valid, both columns holding the same masked token. Record the rule as a dated amendment in ARCHITECTURE.md section 5 and in internal/plan/CLAUDE.md.

## Acceptance



## Log

- 2026-09-14 created

- 2026-09-14 started

- 2026-09-14 closed: done

## Post-mortem

went well: equality groups over declared FKs with one masker per group and a code of its own (plan.refused.equality_group); regression 010 passes under make torture; ARCHITECTURE section 5 amended in place; two bounds recorded as T-0158 and T-0159 rather than hidden | went badly: blocked after two fix rounds on docs/ERRORS.md regeneration alone, the third task to do so; the orchestrator landed it by hand after make check, make torture and the three integration packages passed (Opus dev, 760k tokens) | change next time: done, generated docs are inside every task's paths and the check gate is make check (implement.js, 2026-09-14)
