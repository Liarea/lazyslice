---
id: T-0051
title: "T-FPR: apply ADR-009, delete introspect fingerprint, core-side Schema.Fingerprint, pg gate runs fingerprinter in its own transaction"
epic: E4
phase: 4
status: done
owner: opus
created: 2026-09-06
started: 2026-09-06
closed: 2026-09-06
outcome: "done: e85be26; introspect fingerprint deleted, pg gate runs the injected fingerprinter inside its own BEGIN/ROLLBACK, load's test workaround removed, marker binds end to end"
---

# T-0051 · T-FPR: apply ADR-009, delete introspect fingerprint, core-side Schema.Fingerprint, pg gate runs fingerprinter in its own transaction

## Goal



## Acceptance



## Log

- 2026-09-06 created

- 2026-09-06 started

- 2026-09-06 closed: done: e85be26; introspect fingerprint deleted, pg gate runs the injected fingerprinter inside its own BEGIN/ROLLBACK, load's test workaround removed, marker binds end to end

## Post-mortem

Went well: one round; the reviewer caught stale present-tense comments and the developer fixed them. Went badly: the gate now runs a full introspection with sampling over the target on every marked run because Introspector has one method. Change: a schema-only introspect variant is queued; pg's own suite needs a regression test for the BEGIN.
