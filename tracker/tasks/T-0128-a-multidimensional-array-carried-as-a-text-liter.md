---
id: T-0128
title: "A multidimensional array carried as a text literal is flattened to one dimension at CopyFrom"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-09
started: ""
closed: ""
outcome: ""
---

# T-0128 · A multidimensional array carried as a text literal is flattened to one dimension at CopyFrom

## Goal

Measured on postgres:16 with citext and _citext registered the way internal/pg/types.go registers them: CopyFrom given the Go string '{{a,b},{c,d}}' for a citext[] column stores a one-dimensional four-element array (array_ndims 1). pgx v5.10.0's encodeCopyValue falls back to tryScanStringCopyValueThenEncode (values.go), which scans the literal as text into an 'any' and re-encodes it in binary, and that round trip loses the nesting; the '[0:1]=' dimension prefix is dropped the same way. This is independent of masking -- an unmasked citext[][] column takes the same path -- and internal/transform now preserves both the nesting and the prefix in the literal it emits, so the loss is entirely in the load. internal/load and internal/pg were outside T-0118's paths. Decide whether the loader should hand CopyFrom a nested Go slice for an array that arrived as a literal, or refuse a multidimensional one, and add a load-side test.

## Acceptance



## Log

- 2026-09-09 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
