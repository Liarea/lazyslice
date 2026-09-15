---
id: T-0161
title: "core fills pipeline.PlanRequest.Key so a masked column's DEFAULT is masked instead of refused"
epic: E5
phase: 5
status: done
owner: ""
created: 2026-09-14
started: 2026-09-14
closed: 2026-09-14
outcome: done
---

# T-0161 · core fills pipeline.PlanRequest.Key so a masked column's DEFAULT is masked instead of refused

## Goal

T-0134 landed ARCHITECTURE.md 11.1's literal rule in internal/plan/ddlliteral.go: a masked column's default has its literals masked through the column's own masker, and one it cannot rewrite that a strong validator hits is exit 13. The masking needs the run key, which arrives on pipeline.PlanRequest.Key; internal/core was outside T-0134's paths, so core.run's planRequest() (internal/core/run.go) does not fill it and every masked default with a literal is refused at 13 rather than masked. Fill Key from r.key in planRequest, and flip testdata/regressions/011-masked-column-default-holds-a-literal.sql's header from 'exit 13 target.schema.literal_not_rewritable' to 'ok' in the same change, asserting the target's pg_attrdef holds a masked address. internal/plan's TestAMaskedColumnsDefaultIsMaskedThroughItsOwnMasker is the unit half that already passes.

## Acceptance



## Log

- 2026-09-14 created

- 2026-09-14 moved to E5 phase 5

- 2026-09-14 started

- 2026-09-14 closed: done

## Post-mortem

went well: key resolved before the plan stage for writing runs; plan-only runs use a present key or print which defaults await one; regression 011 flipped to ok with a pg_attrdef assertion; an AST test pins that all three dispatch sites read one planOnly predicate; move refuses on an empty key fingerprint (420f2c3, two fix rounds) | went badly: --require-key is silently ignored on a plan-only run (low, recorded); the torture assertion reads only the first literal of a default (low) | change next time: nothing structural
