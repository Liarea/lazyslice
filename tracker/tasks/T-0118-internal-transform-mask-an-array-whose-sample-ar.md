---
id: T-0118
title: "internal/transform: mask an array whose sample arrives as a text literal element-wise"
epic: E5
phase: 5
status: done
owner: ""
created: 2026-09-08
started: 2026-09-09
closed: 2026-09-14
outcome: done
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

- 2026-09-14 2026-09-14 unblocked: T-0129 (f40de6a) and T-0127 (819ef4e) landed; make torture exits 0 with 009-citext-array-of-addresses-masked-as-one-string.sql at expect ok and its leak assertion passing (61.3 s, ten schemas, catalogue, nine regressions)

- 2026-09-14 closed: done

## Post-mortem

went well: the transform half landed first as a partial with unit coverage and the sequencing note in the tracker made the two follow-ups (verify first, then plan) land in the right order without re-discussion | went badly: three tasks and two usage windows for one column class, because each stage package refused to reach into the next and the array-literal grammar now has three copies | change next time: scope a cross-stage safety change as one task with all the paths it needs, as the 2026-09-09 review also recommends
