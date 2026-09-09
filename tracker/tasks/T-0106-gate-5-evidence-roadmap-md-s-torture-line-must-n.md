---
id: T-0106
title: "Gate 5 evidence: ROADMAP.md's torture line must name the unmask/skip-table split, not the flag total"
epic: E9
phase: ""
status: done
owner: ""
created: 2026-09-08
started: ""
closed: 2026-09-08
outcome: "done: ROADMAP gate line carries the 37/7/1 split"
---

# T-0106 · Gate 5 evidence: ROADMAP.md's torture line must name the unmask/skip-table split, not the flag total

## Goal

docs/TORTURE.md now states the torture result as forty-five flags between the nine clean schemas, of which thirty-seven are --unmask, seven are --skip-table (plausible 1, metabase 2, discourse 4) and one is --key (discourse); twenty of the thirty-seven are the single T-0098 defect. Counted from internal/invariants/torture_catalogue_test.go. ROADMAP.md's gate-5 checkbox 'Nine of ten torture schemas snapshot cleanly, and the tenth fails with a message that says exactly why' is still unticked and carries no flag count at all. When it is ticked it must carry the split rather than the total: --unmask copies a column of personal data into the target verbatim and --skip-table drops a table, so the two are not interchangeable evidence for a gate about masking. ROADMAP.md is outside T-TORTURE's paths, which is why this is filed rather than done.

## Acceptance



## Log

- 2026-09-08 created

- 2026-09-08 closed: done: ROADMAP gate line carries the 37/7/1 split

## Post-mortem

Went well: doc. Went badly: nothing. Change: none.
