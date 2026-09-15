---
id: T-0174
title: "Wire --debug to print the statement trace (Source.Trace) on an ordinary failure"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-14
started: ""
closed: ""
outcome: ""
---

# T-0174 · Wire --debug to print the statement trace (Source.Trace) on an ordinary failure

## Goal

cmd/lazyslice/main.go's report() now prints the wrapped error chain and a recovered panic's stack under --debug (T-FAILUX fix round, 2026-09-14 review finding 3), but docs/FLAGS.md's other half of the promise -- 'the statement trace on error' -- is still unwired: internal/pipeline.Source.Trace() exists for invariant I4 but no failing run threads the tracer out to report. Needs core.Run's return path (or the Stop it returns) to carry the tracer's statements, which is bigger than this fix round's paths (internal/, cmd/, docs/ERRORS.md) should take on silently.

## Acceptance



## Log

- 2026-09-14 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
