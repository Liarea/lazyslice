---
id: T-0093
title: "Move RegisterTypes onto pipeline.Writer so the load's type registration is compiler-checked"
epic: E5
phase: 5
status: done
owner: ""
created: 2026-09-08
started: ""
closed: 2026-09-09
outcome: "done: in T-HARD-C (4f9a186)"
---

# T-0093 · Move RegisterTypes onto pipeline.Writer so the load's type registration is compiler-checked

## Goal

internal/load/load.go's registerTypes reaches type registration through an optional interface assertion (w.(pipeline.TypeRegistrar)). T-CORE-FIX made the miss loud - a Writer that is not a TypeRegistrar now fails the load naming its type - but loud at run time is weaker than checked at compile time: internal/core already wraps the same writer in readableWriter (run.go:1590, which embeds the pipeline.Writer interface and so is NOT a registrar) for verify, and a refactor that put that wrapper ahead of the load call would fail every run instead of failing to build. The fix is to give pipeline.Writer a fourth method, RegisterTypes(ctx, *Schema) error, and update the doubles: internal/load/load_test.go's fakeWriter and plainWriter, internal/verify/verify_test.go's writeOnly, internal/verify/verify_integration_test.go's readableWriter. internal/verify was outside the paths of the task that found this, which is why it is filed rather than done. ARCHITECTURE.md section 2 needs the same edit either way (T-0091).

## Acceptance



## Log

- 2026-09-08 created

- 2026-09-08 moved to E5 phase 5

- 2026-09-09 closed: done: in T-HARD-C (4f9a186)

## Post-mortem

Went well: landed. Went badly: nothing. Change: none.
