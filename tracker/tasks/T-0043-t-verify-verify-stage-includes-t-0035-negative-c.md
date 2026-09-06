---
id: T-0043
title: "T-VERIFY: verify stage, includes T-0035 negative control"
epic: E4
phase: 4
status: done
owner: opus
created: 2026-09-05
started: 2026-09-06
closed: 2026-09-06
outcome: "done: a57f584; FK validation, residual scan with capped confirmation, second net, sequences, row counts, sample compare, negative control (T-0035) exit 9 naming table and column; package suite green"
---

# T-0043 · T-VERIFY: verify stage, includes T-0035 negative control

## Goal



## Acceptance



## Log

- 2026-09-05 created

- 2026-09-06 started

- 2026-09-06 closed: done: a57f584; FK validation, residual scan with capped confirmation, second net, sequences, row counts, sample compare, negative control (T-0035) exit 9 naming table and column; package suite green

## Post-mortem

Went well: the negative control exists and fails loudly; reviewers forced the suite to use the product's own --unmask prior instead of a test-side type list. Went badly: the second net is narrower than the spec in three documented ways, and a classify bug (text categories on timestamp and tsvector) surfaced only here. Change: spec and threat model now state the real coverage; the classify blocker is a task before core.
