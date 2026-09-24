---
id: T-0361
title: "The second net's secret exemption agrees with the classifier: the '_type' normalisation and the JSON leaves"
epic: E6
phase: 6
status: done
owner: sonnet
created: 2026-09-24
started: ""
closed: 2026-09-24
outcome: done
---

# T-0361 · The second net's secret exemption agrees with the classifier: the '_type' normalisation and the JSON leaves

## Goal

T-0315's review (7268376) found the second net looser than the classifier in two places, both fail-open. (1) internal/verify/validators.go's snakeColumnName drops leading non-alphanumerics, so '_type' becomes 'type' and the credential entry is exempt in verify, while internal/classify's normaliseName keeps '_' and does not exempt '_type': make secretExemptColumns' lookup reproduce normaliseName exactly (share one exported helper) and add a '_type' case to secret_floor_test.go. (2) internal/verify/secondnet.go's exemptColumns skip also drops the credential entry for the JSON-leaf tally of a column named type, klass or component_name, while classify's JSON path (jsonSignal) never honours the exemption: apply the exemption only to direct values, or make classify's JSON path honour the same names, so the two nets agree; pin with a jsonb 'type' column.

## Acceptance

—

## Log

- 2026-09-24 2026-09-24 created
- 2026-09-24 closed: done

## Post-mortem

went well: landed as 48738f7 after one review round (one medium, two low): the second net's secret exemption now agrees with the classifier on the _type normalisation and on JSON leaves, and the fix round ported classify's rune-based word-boundary fold so acronym-run names (TYpe, ComponentNAme) fold identically in both nets | went badly: sharing the fold through internal/textsig lay outside the task's paths, so it is hand-mirrored for now (T-0365, E9); the developer predicted a tracker id in prose before filing and had to correct it | change next time: a task that aligns two packages' normalisation gets internal/textsig in its paths so the shared helper lands with it, and ids are written after filing
