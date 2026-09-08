---
id: T-0093
title: "Move RegisterTypes onto pipeline.Writer so the load's type registration is compiler-checked"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-08
started: ""
closed: ""
outcome: ""
---

# T-0093 · Move RegisterTypes onto pipeline.Writer so the load's type registration is compiler-checked

## Goal

internal/load/load.go's registerTypes reaches type registration through an optional interface assertion (w.(pipeline.TypeRegistrar)). T-CORE-FIX made the miss loud - a Writer that is not a TypeRegistrar now fails the load naming its type - but loud at run time is weaker than checked at compile time: internal/core already wraps the same writer in readableWriter (run.go:1590, which embeds the pipeline.Writer interface and so is NOT a registrar) for verify, and a refactor that put that wrapper ahead of the load call would fail every run instead of failing to build. The fix is to give pipeline.Writer a fourth method, RegisterTypes(ctx, *Schema) error, and update the doubles: internal/load/load_test.go's fakeWriter and plainWriter, internal/verify/verify_test.go's writeOnly, internal/verify/verify_integration_test.go's readableWriter. internal/verify was outside the paths of the task that found this, which is why it is filed rather than done. ARCHITECTURE.md section 2 needs the same edit either way (T-0091).

## Acceptance



## Log

- 2026-09-08 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
