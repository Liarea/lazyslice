---
id: T-0266
title: "The second net's surrogate-key exemption is measured against a key with no sequence or identity default"
epic: E9
phase: ""
status: open
owner: opus
created: 2026-09-17
started: ""
closed: ""
outcome: ""
---

# T-0266 · The second net's surrogate-key exemption is measured against a key with no sequence or identity default

## Goal

Rounds 5 and 6 of the red team (docs/reviews/2026-09-15-redteam/round6.json, the taxref attempt): a table whose own primary key is a contiguous block of nine-digit identifiers, in a column no name rule matches, beside a masked phone, is copied verbatim under exit 0, because internal/verify/secondnet.go keeps the dense-sequence exemption unconditional for Decision.NeverMasked and never consults corroboration. THREAT_MODEL.md T1 names it as accepted because a rule over values alone would also refuse every large table whose generated id has reached nine digits beside an email. The discriminator worth measuring: a generated surrogate has a sequence or identity default and an imported identifier does not, so introspect could carry that fact and the exemption could stay unconditional only for a key that has one (and for a validated FK child of an unmasked parent, regressions/021). Measure against the ten torture schemas before deciding, pin the taxref schema as a regression either way, and expect the outcome to be a plan refusal with the --unmask escape, since a personal primary key cannot be masked without breaking the key.

## Acceptance



## Log

- 2026-09-17 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
