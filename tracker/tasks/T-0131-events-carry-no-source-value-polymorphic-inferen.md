---
id: T-0131
title: "Events carry no source value: polymorphic inference reports unknown type values by count and keyed digest, and an output-sink canary test proves it"
epic: E5
phase: 5
status: open
owner: sonnet
created: 2026-09-14
started: ""
closed: ""
outcome: ""
---

# T-0131 · Events carry no source value: polymorphic inference reports unknown type values by count and keyed digest, and an output-sink canary test proves it

## Goal

internal/plan/polymorphic.go:290 and :370 put a raw _type value into a reason string through showValue and core emits it as a warning (internal/core/run.go:1182), so a source value reaches stdout, --json and any log even when classification masks that column: docs/reviews/2026-09-09/REVIEW.md finding 2, evidence/polymorphic_log.log prints poly.canary@example.org. Rule: no event, reason, refusal or explanation string ever carries a value read from a source row; it may carry identifiers (schema, table, column, constraint), counts, and when values must be told apart a keyed digest (HMAC under the run key, first 8 hex). Fix polymorphic to report the count of distinct unmapped values plus digests; audit every other path in internal/plan and internal/classify that quotes a sample into an explanation; extend the value-free serialisation test in internal/event to cover reason arguments. Add a canary test in cmd/lazyslice: seed every text column of a small fixture with a unique canary per column, run the binary with --json and at the highest verbosity, and assert no canary appears in stdout, stderr, the emitted lazyslice.yml, or any file the run writes.

## Acceptance



## Log

- 2026-09-14 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
