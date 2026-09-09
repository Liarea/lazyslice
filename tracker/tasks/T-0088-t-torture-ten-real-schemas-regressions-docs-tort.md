---
id: T-0088
title: "T-TORTURE: ten real schemas, regressions, docs/TORTURE.md"
epic: E5
phase: 5
status: done
owner: opus
created: 2026-09-08
started: ""
closed: 2026-09-08
outcome: "done: 08e4159; ten real schemas, nine clean, tenth named; docs/TORTURE.md; 45 flags measured (37 unmask, 7 skip-table, 1 key); twelve findings filed"
---

# T-0088 · T-TORTURE: ten real schemas, regressions, docs/TORTURE.md

## Goal



## Acceptance



## Log

- 2026-09-08 created

- 2026-09-08 closed: done: 08e4159; ten real schemas, nine clean, tenth named; docs/TORTURE.md; 45 flags measured (37 unmask, 7 skip-table, 1 key); twelve findings filed

## Post-mortem

Went well: the suite found a defect that affects every auth schema (unique credentials cannot be masked) and a T1 gap (composite columns copied silently), both invisible to the fixtures. Went badly: two review rounds ended on orchestrator-only asks. Change: ADR-011 lands the unique-index rules; the real defects are the next harden tasks.
