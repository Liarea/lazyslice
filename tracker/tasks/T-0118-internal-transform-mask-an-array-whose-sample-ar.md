---
id: T-0118
title: "internal/transform: mask an array whose sample arrives as a text literal element-wise"
epic: E5
phase: 5
status: blocked
owner: ""
created: 2026-09-08
started: 2026-09-09
closed: ""
outcome: ""
---

# T-0118 · internal/transform: mask an array whose sample arrives as a text literal element-wise

## Goal

T-HARD-B landed the classify half of T-0103: an array of an extension type (citext[]) arrives from the source as the single string {a@b.test,c@d.test} because the source pool registers no user types, and internal/classify now splits that literal so the validators see the addresses. The transform half is still owed and is now reachable: transform.maskArray only fires on a []any, so such a column is masked as one scalar string, and CopyFrom is handed a plain string for an _citext column and fails with 'cannot find encode plan' at exit 7 mid-load (T-0103's second log entry). So a citext[] of real addresses now fails loudly instead of copying silently -- the right direction, and not the end state. Owed: in internal/transform, when the column shape says array and the value is a text literal, split it, mask element-wise and re-render the literal. testdata/regressions/005 steers around this and its comment says so.

## Acceptance



## Log

- 2026-09-08 created

- 2026-09-09 moved to E5 phase 5

- 2026-09-09 started

- 2026-09-09 blocked: Code complete and green; unreachable from the CLI because internal/plan's writeback still refuses an array that arrives as a text literal (exit 12). Sequence: T-0129 (verify refuses or flags a masked array column whose target value does not decode to a slice) first or together with T-0127 (plan drops arrayArrivesAsLiteral); then regression 009's header flips to ok and its leak assertion runs. Paused 2026-09-09.

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
