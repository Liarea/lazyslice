---
id: T-0298
title: "Add a digit-run/phone-without-plus signal so plain 10-12 digit columns are not left to Luhn chance"
epic: E9
phase: ""
status: cancelled
owner: ""
created: 2026-09-22
started: ""
closed: 2026-09-22
outcome: cancelled
---

# T-0298 · Add a digit-run/phone-without-plus signal so plain 10-12 digit columns are not left to Luhn chance

## Goal

T-0297's ValidMAC narrowing (internal/textsig/textsig.go) means a column with no name match holding 12-digit phone numbers without '+' is masked only if it happens to hit Luhn (~10% of 12-19 digit values). Add a validator/signal in internal/textsig for a plain phone-shaped digit run (no separators, no leading '+', 10-12 digits) that internal/classify's value signals and internal/verify's second net (validators.go, catalog.go) and internal/plan/ddlliteral.go can score, so such a column is not left unmasked on chance alone. See internal/textsig/textsig.go's ValidMAC comment and internal/textsig/CLAUDE.md's T-0297 section for the narrowing this closes.

## Acceptance

—

## Log

- 2026-09-22 2026-09-22 created
- 2026-09-22 cancelled: Moot: 5711c6b reverted the ValidMAC narrowing (87aea49) that created the gap; a digit-run column with no name is caught as network_id exactly as before.

## Post-mortem

Cancelled. Reason: Moot: 5711c6b reverted the ValidMAC narrowing (87aea49) that created the gap; a digit-run column with no name is caught as network_id exactly as before.
