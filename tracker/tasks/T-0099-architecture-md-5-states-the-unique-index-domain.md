---
id: T-0099
title: "ARCHITECTURE.md 5 states the unique-index domain rule for a column and says nothing about composite or partial indexes"
epic: E9
phase: ""
status: done
owner: ""
created: 2026-09-08
started: ""
closed: 2026-09-08
outcome: "done: ADR-011 and §5 state the composite and partial rules"
---

# T-0099 · ARCHITECTURE.md 5 states the unique-index domain rule for a column and says nothing about composite or partial indexes

## Goal

internal/plan/unique.go now implements 5's rule (d_required = n squared / 2 epsilon, pick the widest generator, refuse at exit 12), and internal/classify decides which columns it applies to. Two cases 5 does not cover had to be approximated to stop real schemas dying in the loader, and the approximations are in internal/classify/classify.go's raiseCompositeUnique and indexKeys with their reasoning: (a) a composite unique index raises its masked columns unless an unmasked key column's sample has no repeats - the true rule is about the product of the masked columns' domains against the largest group of rows agreeing on the unmasked ones, which needs a source statistic nothing collects; (b) a single-column partial unique index raises its column and d_required is computed over the whole table's row count, which over-estimates, because the predicate admits fewer rows. Evidence for both: testdata/regressions/003, 004 and 007, from rails-activestorage, django and supabase-auth. Owed: decide the two rules in ARCHITECTURE.md 5 (an ARCHITECTURE.md edit, outside T-TORTURE's paths), then tighten or delete the approximations.

## Acceptance



## Log

- 2026-09-08 created

- 2026-09-08 closed: done: ADR-011 and §5 state the composite and partial rules

## Post-mortem

Went well: doc. Went badly: nothing. Change: none.
