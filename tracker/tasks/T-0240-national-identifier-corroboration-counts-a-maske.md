---
id: T-0240
title: "National-identifier corroboration counts a masked personal neighbour at any confidence and outranks the dense-sequence exemption; character-family identifiers get the same path"
epic: E5
phase: 5
status: open
owner: sonnet
created: 2026-09-17
started: ""
closed: ""
outcome: ""
---

# T-0240 · National-identifier corroboration counts a masked personal neighbour at any confidence and outranks the dense-sequence exemption; character-family identifiers get the same path

## Goal

Round-4 replay (docs/reviews/2026-09-15-redteam/round4-still-leaking.json, the A9b replays and the varchar(9) variant): a column of nine-digit national identifiers, as bigint or as varchar(9), beside a column the run masked as a phone on its name, reports 'no name or value signal' and crosses verbatim, because the corroboration gate counts only likely or certain neighbours and the dense-sequence exemption is applied before corroboration. Count a masked person-identifying neighbour at possible or above; apply the dense-sequence exemption only when nothing corroborates; run the character family through the same validator path as the digits family. Regression fixtures: varchar(9) identifiers beside a masked phone, and bigint identifiers beside a name-matched phone; every existing fixture 018 to 023 stores the identifier in a numeric family. Keep the account-number control (regression 026) passing. Files: internal/classify, internal/verify/validators.go and secondnet.go, internal/textsig, testdata/regressions, THREAT_MODEL.md T1's T-0187 amendment.

## Acceptance



## Log

- 2026-09-17 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
