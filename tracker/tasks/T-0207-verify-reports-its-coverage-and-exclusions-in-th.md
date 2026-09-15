---
id: T-0207
title: "verify reports its coverage and exclusions in the result: which columns the residual scan tested, which domains and tables it skipped, and why"
epic: E9
phase: ""
status: open
owner: sonnet
created: 2026-09-15
started: ""
closed: ""
outcome: ""
---

# T-0207 · verify reports its coverage and exclusions in the result: which columns the residual scan tested, which domains and tables it skipped, and why

## Goal

docs/reviews/2026-09-09/REVIEW.md, 'The residual scanner cannot prove an entire database is safe': coverage and exclusions should be part of the result rather than implicit design choices, so a reader knows what the exit code does and does not vouch for. T-0195 exposes the classifier's weak-ratio side; this is the verify side. Files: internal/verify's result type, internal/render's verify summary, and the docs that describe exit 9.

## Acceptance



## Log

- 2026-09-15 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
