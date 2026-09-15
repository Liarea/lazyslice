---
id: T-0193
title: "THREAT_MODEL.md T1: national_id is now a row-path control, not only DDL-literal"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-15
started: ""
closed: ""
outcome: ""
---

# T-0193 · THREAT_MODEL.md T1: national_id is now a row-path control, not only DDL-literal

## Goal

T-0187 wired textsig.ValidNationalID/ValidNationalIDDigits into internal/classify's ordered validators list and internal/verify's second net (validators.go), closing red-team round 2 R2-01..R2-04 (docs/reviews/2026-09-15-redteam/round2-still-leaking.json). THREAT_MODEL.md T1's 2026-09-15 amendment (around line 47) still says national_id 'joins the strong set used by both the plan-time and the catalog pass' -- both DDL passes -- which is now stale/misleading about the row path exactly the way the round-2 red team's A2 finding called out ('a miss-twice of exactly the shape that row's own amendment describes'). Update T1 to record that national_id is a live control on both the row-scanning path (classify decides it at the same confidence email gets; verify's second net fails a proven column on any hit as a strong text entry, and refuses a numeric column over validatorThreshold on the digits entry) and the DDL-literal path, and note the accepted asymmetry: internal/classify has no digits-family recovery for a national ID whose separators a numeric column drops (e.g. an SSN's leading zero), so such a column is refused at verify's exit 9 rather than masked -- a refusal instead of a mask, the same shape T-0136 already accepts for Luhn. File: THREAT_MODEL.md, not in T-0187's writable paths.

## Acceptance



## Log

- 2026-09-15 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
