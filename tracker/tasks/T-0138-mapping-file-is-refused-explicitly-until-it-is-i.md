---
id: T-0138
title: "mapping_file is refused explicitly until it is implemented; ADR-012 records the deferral"
epic: E5
phase: 5
status: open
owner: sonnet
created: 2026-09-14
started: ""
closed: ""
outcome: ""
---

# T-0138 · mapping_file is refused explicitly until it is implemented; ADR-012 records the deferral

## Goal

internal/pipeline/config.go:81 reads mapping_file, emit round-trips it (internal/emit/document.go:293) and the unique-domain refusal recommends it (internal/plan/unique.go:165), but no code reads the CSV or applies a replacement: docs/reviews/2026-09-09/REVIEW.md finding 10. Until it is implemented: a yml naming mapping_file exits 2 saying it is not supported in this version and pointing at --unmask with a reason or a lower --take or --cap; the unique-domain refusal drops that remedy; emit stops writing the field; repo keeps protecting a named path. Write docs/adr/012-mapping-file-deferred.md (proposed) that supersedes the mapping_file paragraph of ADR-006 for v1 and names the E9 task that implements the full contract.

## Acceptance



## Log

- 2026-09-14 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
