---
id: T-0187
title: "National-identifier validators with checksums in textsig; classify and the second net treat them as strong; a digits-family SSN shape under the ratio rule"
epic: E5
phase: 5
status: open
owner: sonnet
created: 2026-09-15
started: ""
closed: ""
outcome: ""
---

# T-0187 · National-identifier validators with checksums in textsig; classify and the second net treat them as strong; a digits-family SSN shape under the ratio rule

## Goal

Red team 2026-09-15 round 2 items R2-01, R2-02, R2-03, R2-04 in docs/reviews/2026-09-15-redteam/round2-still-leaking.json: there is no national_id value validator at all, so plain US SSNs (078-05-1001), UK NI numbers (AB100001D) and their carriers (text[] elements, a JSON document in a text column, an SSN stored as bigint) cross verbatim in columns whose names miss the rule pack, reported as no name or value signal. Add to internal/textsig precise validators with the check rules each format has: US SSN (area, group and serial rules, not 000/666/9xx), UK NI (prefix rules and suffix A-D), Polish PESEL (checksum), Italian codice fiscale (check character), Dutch BSN (11-test), Spanish DNI and NIE (letter check), French NIR (key), Brazilian CPF (two check digits), Canadian SIN (Luhn), Indian Aadhaar (Verhoeff), Australian TFN (weighted check). Classify: a hit decides national_id at the same confidence email gets; the second net treats them as strong on text families; on numeric families a nine-digit SSN-shaped value is a digits validator under the ratio rule like Luhn (T-0136). Each format gets positive and negative test vectors; regression fixtures for a plain SSN in a column called code, a text[] of NI numbers, and a bigint SSN column. Run make torture; record any torture schema that now needs a flag in docs/TORTURE.md.

## Acceptance



## Log

- 2026-09-15 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
