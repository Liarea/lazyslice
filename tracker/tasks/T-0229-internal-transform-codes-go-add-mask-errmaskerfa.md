---
id: T-0229
title: "internal/transform/codes.go: add mask.ErrMaskerFailed to maskReason's known-sentinel list"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-16
started: ""
closed: ""
outcome: ""
---

# T-0229 · internal/transform/codes.go: add mask.ErrMaskerFailed to maskReason's known-sentinel list

## Goal

T-0223 added mask.ErrMaskerFailed, wrapping a masker's returned error the way ErrMaskerPanic wraps a panic. internal/transform/codes.go's maskReason() (around line 136) lists ErrMaskerPanic and the other mask/ sentinels by name so their exit-code reason text is the sentinel's own safe words; ErrMaskerFailed is not in that list, so it currently falls through to reasonSummary's generic 'an error of type %T' fallback, which is safe but less specific than the other refusals get. Add mask.ErrMaskerFailed to the sentinel slice in maskReason (internal/transform/codes.go) and mention it alongside ErrMaskerPanic in internal/transform/CLAUDE.md's list of named sentinels.

## Acceptance



## Log

- 2026-09-16 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
