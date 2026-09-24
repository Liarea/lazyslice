---
id: T-0319
title: "After a second-net refusal the operator can mask the column: --mask, and every failing column reported at once"
epic: E6
phase: 6
status: done
owner: opus
created: 2026-09-23
started: ""
closed: 2026-09-24
outcome: done
---

# T-0319 · After a second-net refusal the operator can mask the column: --mask, and every failing column reported at once

## Goal

Dogfood session 1: verify.refused.second_net named one copied column per run (a cached HTTP body with URLs, then a feed payload, then an external interaction id), the message names no flag, --skip-table was refused because the table is a parent, and the only green path was --unmask on the column the net had just flagged as personal (internal/verify/verify.go:70 says 'no green path short of --unmask'). Two changes: verify scans every unmasked column and reports all failures before exiting 9; and a --mask TABLE.COL[=category] flag, the counterpart of --unmask, records a masked decision the way an unmask is recorded (in the yml with by: flag; category defaults to free_text or the net's own category), so the operator can answer the net without declaring anything safe. The refusal message names --mask and --skip-table. Root CLAUDE.md: every TUI action is reachable by a flag first; the classification screen presumably toggles this already, so the flag is overdue.

## Acceptance

—

## Log

- 2026-09-23 2026-09-23 created
- 2026-09-24 closed: done

## Post-mortem

went well: --mask enters as a classifier raise so internal/classify needed no change; every second-net failure is now its own Error event; two review rounds closed a real T1 gap (an FK child of a masked natural key stayed in clear) with an exit-2 refusal naming each child, and both fixes have tests that fail without them | went badly: the workflow's verify agent was forced to report before make torture finished and returned passed=false on a green tree, so the task blocked and the orchestrator reran the whole gate (make check, verify/core/cmd/emit/invariants integration, make torture, all green) and landed it by hand as 9f6b1a1; the FK-child check works around classify's pass order rather than fixing it (T-0364, E6); the review's three lows are T-0319's follow-up task in E9 and the second_net hint task in E6 | change next time: implement.js's verify step must skip make torture when nothing under testdata/ changed, as the task rule already says, and when it does run it its turn budget must cover the 60-minute timeout
