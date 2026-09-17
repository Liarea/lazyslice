---
id: T-0258
title: "internal/plan's fk-pair refusal can fire on a pair a later classify pass already reconciles"
epic: E5
phase: 5
status: open
owner: ""
created: 2026-09-17
started: ""
closed: ""
outcome: ""
---

# T-0258 · internal/plan's fk-pair refusal can fire on a pair a later classify pass already reconciles

## Goal

internal/plan/fkpair.go's checkFKPairRefusal (T-0257) refuses whenever Decision.Refused is non-empty, but internal/classify sets that field at the point fkPairs runs (inside neighbouringColumns) and never clears it if a later, separate pass -- propagateKeys (FK propagation) or sameColumnName -- goes on to bring both ends of the pair into agreement anyway. Measured case: testdata/torture/metabase, public.core_session.id (decided credential on its own value signal before fkPairs runs) and its FK child public.login_history.session_id (no samples of its own, blocked by fkPairs because the parent already carries a decision) -- propagateKeys then carries credential from core_session.id onto login_history.session_id regardless, so both end up Masked=true under the identical Category, and the refusal costs the run an --unmask that buys nothing (internal/invariants/torture_catalogue_test.go's metabase entry, docs/TORTURE.md's own T-0257 section has the full account). Fix: in checkFKPairRefusal, before refusing, read the FINAL decisions for the named column and its RefusedPartner (after classify's own passes have all run, which plan already has) and skip the refusal when both are Masked and share the same non-none Category -- matching what internal/plan/equality.go's own group mechanism will unify the masker over downstream anyway. Add a plan-level test over the metabase shape (or a reduced regression) asserting the refusal does NOT fire once a later pass reconciles the pair, and keep the existing tests asserting it still fires when the partner never gets masked at all (the type-conflict and two-letter-code shapes). Do not weaken the check for the case that is not reconciled.

## Acceptance



## Log

- 2026-09-17 created

- 2026-09-17 moved to E5 phase 5

- 2026-09-17 re-homed to E5 by the orchestrator, 2026-09-17: a needless --unmask on a real schema (Metabase) is a first-run cost v0.1.0 should not carry; narrow checkFKPairRefusal to skip a pair whose two ends are already masked under the same final category, keep the refusal for type conflicts and two-letter codes, and drop the second Metabase flag with the count back to 27

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
