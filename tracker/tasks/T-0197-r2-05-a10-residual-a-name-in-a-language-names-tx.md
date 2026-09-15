---
id: T-0197
title: "R2-05/A10 residual: a name in a language names.txt does not carry still reports \"no name or value signal\" as a clean bill of health"
epic: E9
phase: 5
status: open
owner: sonnet
created: 2026-09-15
started: ""
closed: ""
outcome: ""
---

# T-0197 · R2-05/A10 residual: a name in a language names.txt does not carry still reports "no name or value signal" as a clean bill of health

## Goal

The 2026-09-15 round-2 red team's A10 (Khmer, Lao and Amharic transliterations in a column called label, no other personal column in the table) still leaks: names.txt cannot be widened to close it (T-0188 measured Wikidata's P407 coverage for Khmer/Lao/Amharic given+family names at 3/12/13 items total, nowhere near usable, and the class is unbounded by language regardless). The reviewer's own two suggestions, neither in T-0188's paths/scope: (1) internal/classify/reasons.go — change the 'no name or value signal' reason string for a character column that was scanned and matched nothing to something that reads as absence-of-evidence rather than a clean bill of health, e.g. 'nothing recognised; not proof the column is impersonal'; (2) internal/classify/classify.go's unknownColumnsBesideCertain rail — currently fires only in a table that already holds a certain person-identifying column, which is exactly the shape A10's table does not have; extending it to raise a short free-text column with no unique index and no type conflict as its table's only unexplained column is an ADR-level recall/precision trade (THREAT_MODEL.md T1), not a patch.

## Acceptance



## Log

- 2026-09-15 created

- 2026-09-15 front matter re-serialised with escaped inner quotes (T-0188 review, low finding)

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
