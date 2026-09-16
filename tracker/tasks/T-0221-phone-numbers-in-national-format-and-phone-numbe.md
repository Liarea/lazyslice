---
id: T-0221
title: "Phone numbers in national format, and phone numbers spelled out in words, are recognised: a configured phone region, and corroboration when none is configured"
epic: E5
phase: 5
status: open
owner: sonnet
created: 2026-09-16
started: ""
closed: ""
outcome: ""
---

# T-0221 · Phone numbers in national format, and phone numbers spelled out in words, are recognised: a configured phone region, and corroboration when none is configured

## Goal

Round-3 replay (docs/reviews/2026-09-15-redteam/round3-still-leaking.json, the A2 residual and its plain variant): a correctly formatted domestic number such as '020 7946 0958' or '07911 123456' in a column called kontaktnr crosses verbatim under exit 0 because internal/textsig/textsig.go sets PhoneRegionHint to ZZ, so only a +E.164 number parses; a number dictated in words crosses for the same reason once candidates.go's spelledDigits has turned it into digits, and Dict.Prose requires a name word so the free_text rule never fires either. Decision (orchestrator, 2026-09-16), on T-0187's precedent: add --phone-region REGION (yml phone_region, shown in the reasons output as the region assumed); the row path parses a candidate under the configured region, and with none configured under a short built-in list of regions but only with corroboration (the column's name matches the phone rule, or a proven personal neighbour); verify's second net keeps its strong footing on +E.164 and on the configured region only, never the guessed list, so a ten-digit account column cannot refuse a loaded run. Regressions: the plain national number, the dictated number, and an account-number column that must pass. Files: internal/textsig, internal/classify, internal/verify/validators.go, cmd/lazyslice, internal/pipeline config and emit, docs via make docs, THREAT_MODEL.md T1.

## Acceptance



## Log

- 2026-09-16 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
