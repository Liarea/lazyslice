---
id: T-0143
title: "Decide the arbitrary-JSON policy: structure-preserving masking versus whole-document replacement"
epic: E9
phase: ""
status: open
owner: human
created: 2026-09-14
started: ""
closed: ""
outcome: ""
---

# T-0143 · Decide the arbitrary-JSON policy: structure-preserving masking versus whole-document replacement

## Goal

docs/reviews/2026-09-09/REVIEW.md finding 8: preserving keys preserves identity-keyed maps; replacing the whole document breaks the application. Product decision for the maintainer after dogfood; the E5 task masks strong-hit keys meanwhile. internal/transform/json.go, SECURITY.md.

## Acceptance



## Log

- 2026-09-14 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
