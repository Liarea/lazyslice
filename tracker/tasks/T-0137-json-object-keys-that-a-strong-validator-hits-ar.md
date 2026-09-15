---
id: T-0137
title: "JSON object keys that a strong validator hits are masked"
epic: E5
phase: 5
status: done
owner: sonnet
created: 2026-09-14
started: 2026-09-14
closed: 2026-09-14
outcome: done
---

# T-0137 · JSON object keys that a strong validator hits are masked

## Goal

internal/transform/json.go:246 keeps every object key and masks values only, so a document keyed by an address keeps the address: docs/reviews/2026-09-09/REVIEW.md finding 8, evidence/json_keys.log; SECURITY.md:75 discloses this. Narrow the disclosure: a key that parses as an email, phone or credit card is masked through that category masker (deterministic, so equal keys stay equal); other keys stay. The JSON walk in verify scans keys as well as values. Update the SECURITY.md limitation to what remains (arbitrary identifiers as keys). Regression: the review document, expected ok, target key masked. Whole-document replacement as a policy is a product decision filed separately in E9.

## Acceptance



## Log

- 2026-09-14 created

- 2026-09-14 started

- 2026-09-14 closed: done

## Post-mortem

went well: keys that parse as email, phone or credit card are masked deterministically; verify scans keys at their JSON path; regression 013; SECURITY.md narrowed (c991086, one fix round) | went badly: the first landing recorded masked keys under an empty path, a blind spot below a masked key that the reviewer caught; a key collision refuses under a new json_key masker id rather than a code of its own | change next time: record every residual entry at its real path from the start
