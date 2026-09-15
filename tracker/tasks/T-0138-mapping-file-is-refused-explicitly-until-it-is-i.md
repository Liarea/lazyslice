---
id: T-0138
title: "mapping_file is refused explicitly until it is implemented; ADR-012 records the deferral"
epic: E5
phase: 5
status: done
owner: sonnet
created: 2026-09-14
started: 2026-09-14
closed: 2026-09-14
outcome: done
---

# T-0138 · mapping_file is refused explicitly until it is implemented; ADR-012 records the deferral

## Goal

internal/pipeline/config.go:81 reads mapping_file, emit round-trips it (internal/emit/document.go:293) and the unique-domain refusal recommends it (internal/plan/unique.go:165), but no code reads the CSV or applies a replacement: docs/reviews/2026-09-09/REVIEW.md finding 10. Until it is implemented: a yml naming mapping_file exits 2 saying it is not supported in this version and pointing at --unmask with a reason or a lower --take or --cap; the unique-domain refusal drops that remedy; emit stops writing the field; repo keeps protecting a named path. Write docs/adr/012-mapping-file-deferred.md (proposed) that supersedes the mapping_file paragraph of ADR-006 for v1 and names the E9 task that implements the full contract.

## Acceptance



## Log

- 2026-09-14 created

- 2026-09-14 started

- 2026-09-14 closed: done

## Post-mortem

went well: mapping_file is exit 2 at read with a two-step remedy, emit never writes it, ADR-012 proposed, unique refusal names two escapes (99e3fe2, one fix round) | went badly: first landing's remedy named only flags, which cannot clear a refusal driven by file content; ADR cited a stale line number and a merge test that does not exist (low, to fix before the ADR freezes) | change next time: write the exact remedy text first, verify every line reference in an ADR against the checkout
