---
id: T-0358
title: "README and SECURITY: say the card signal wants an issuer prefix, and what a card outside the table costs"
epic: E6
phase: 6
status: done
owner: ""
created: 2026-09-24
started: ""
closed: 2026-09-25
outcome: done
---

# T-0358 · README and SECURITY: say the card signal wants an issuer prefix, and what a card outside the table costs

## Goal

T-0316 made the classifier's and the second net's card signal require a known issuer prefix (and, under an id/number/version/reference column name, the issuer's own length); THREAT_MODEL.md T1's T-0316 amendment states the widened residual (a card from a range the IIN table lacks, or of an unlisted length under an identifier's name, is copied). README.md's 'a Luhn check for card numbers' (How it decides) and its accepted residual 4, and SECURITY.md's residual 2, list 'card number' among the recognised shapes without that bound; add the one clause each. Neither file was in T-0316's paths.

## Acceptance

—

## Log

- 2026-09-24 2026-09-24 created
- 2026-09-24 2026-09-24 moved to E6 phase 6
- 2026-09-25 closed: done

## Post-mortem

went well: landed as 9abc269 with no review finding above low: README's card-signal description and both residual lists say the card signal wants an issuer prefix and what a card outside the table costs | went badly: three lows filed as a follow-up (the eight identifier suffixes, the sentence's scope against the JSON-leaf and DDL-literal callers that still use the bare check digit, a rewrap) | change next time: nothing
