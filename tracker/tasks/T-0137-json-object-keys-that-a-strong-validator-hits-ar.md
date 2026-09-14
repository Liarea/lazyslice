---
id: T-0137
title: "JSON object keys that a strong validator hits are masked"
epic: E5
phase: 5
status: open
owner: sonnet
created: 2026-09-14
started: ""
closed: ""
outcome: ""
---

# T-0137 · JSON object keys that a strong validator hits are masked

## Goal

internal/transform/json.go:246 keeps every object key and masks values only, so a document keyed by an address keeps the address: docs/reviews/2026-09-09/REVIEW.md finding 8, evidence/json_keys.log; SECURITY.md:75 discloses this. Narrow the disclosure: a key that parses as an email, phone or credit card is masked through that category masker (deterministic, so equal keys stay equal); other keys stay. The JSON walk in verify scans keys as well as values. Update the SECURITY.md limitation to what remains (arbitrary identifiers as keys). Regression: the review document, expected ok, target key masked. Whole-document replacement as a policy is a product decision filed separately in E9.

## Acceptance



## Log

- 2026-09-14 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
