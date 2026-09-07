---
id: T-0049
title: "Mask module low findings from T-MASK review (see T-0040 log) and a registry test that every rules.yml masker id resolves"
epic: E5
phase: 5
status: open
owner: sonnet
created: 2026-09-06
started: ""
closed: ""
outcome: ""
---

# T-0049 · Mask module low findings from T-MASK review (see T-0040 log) and a registry test that every rules.yml masker id resolves

## Goal



## Acceptance



## Log

- 2026-09-06 created

- 2026-09-06 started

- 2026-09-07 Added from T-0059 review: mask/network_id_range_test.go input generator has dead arithmetic (every input is 1.0.X.Y) and never feeds documentation-range inputs; it asserts membership only, so a constant generator passes; count distinct outputs and reuse wantIPIn from format_test.go. internal/transform/writeback_test.go probe values 203.0.113.7/32 and 203.0.113.0/24 are deterministic today but belong in RFC 1918.

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
