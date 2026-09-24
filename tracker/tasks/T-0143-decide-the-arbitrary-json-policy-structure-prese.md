---
id: T-0143
title: "Decide the arbitrary-JSON policy: structure-preserving masking versus whole-document replacement"
epic: E9
phase: ""
status: done
owner: human
created: 2026-09-15
started: ""
closed: 2026-09-24
outcome: done
---

# T-0143 · Decide the arbitrary-JSON policy: structure-preserving masking versus whole-document replacement

## Goal

docs/reviews/2026-09-09/REVIEW.md finding 8: preserving keys preserves identity-keyed maps; replacing the whole document breaks the application. Product decision for the maintainer after dogfood; the E5 task masks strong-hit keys meanwhile. internal/transform/json.go, SECURITY.md.

## Acceptance

—

## Log

- 2026-09-23 2026-09-23 Dogfood session 1 (2026-09-23) evidence: JSON is already masked leaf-wise with keys preserved (masker semi_structured). On a real Rails schema 30 json/jsonb columns were masked and every string leaf became word salad: an AI model's config, ad-server settings, trigger actions, deployment-condition rules, brand-kit styles, a device's location document. Structure is preserved, the app still breaks, because configuration leaves with no personal signal are masked like everything else. The decision this needs is per-leaf classification (T-0272): a leaf with a personal signal gets its category's masker, a leaf with none is copied, the document's keys stay. Whole-document replacement would be strictly worse for these columns.
- 2026-09-24 closed: done

## Post-mortem

went well: decided by the maintainer 2026-09-24 with dogfood evidence (T-0143 log, 2026-09-23): option (a), per-leaf classification: a JSON leaf with a personal signal is masked by its category's masker, a leaf with no signal is copied, the document's keys stay, and the log-shaped-table rule (replace the whole document) stays for audit tables | went badly: the decision waited from 2026-09-09 to a dogfood run that showed configuration documents destroyed | change next time: T-0272 is the implementation and moves to phase 6
