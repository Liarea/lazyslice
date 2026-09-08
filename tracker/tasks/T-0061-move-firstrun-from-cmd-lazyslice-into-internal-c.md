---
id: T-0061
title: "Move firstRun from cmd/lazyslice into internal/core's discover stage, so cmd/ calls only core.Run again; update cmd/CLAUDE.md in the same change"
epic: E5
phase: ""
status: open
owner: opus
created: 2026-09-07
started: ""
closed: ""
outcome: ""
---

# T-0061 · Move firstRun from cmd/lazyslice into internal/core's discover stage, so cmd/ calls only core.Run again; update cmd/CLAUDE.md in the same change

## Goal

T-DISCOVER put the first-run ladder in cmd/lazyslice's firstRun, which imports internal/discover and calls discover.Resolve and discover.AsRefusal. cmd/CLAUDE.md line 5 ('no stage is reached directly - this file builds core.Request and calls core.Run') and its Never list ('call an internal/<stage> package directly (only core.Run)') state the rule absolutely, so that directory's contract is currently false about its one file. internal/core was outside T-DISCOVER's paths, which is why it landed in cmd/. The natural home is core.discover, and the move changes no behaviour in internal/discover.

## Acceptance

cmd/lazyslice no longer imports internal/discover. core.Run walks the ladder for the root command and still stops at exit 3 / exit 4 with the same codes for the five stage subcommands. cmd/CLAUDE.md is true again, or records the deviation if one survives. cmd/lazyslice/main_test.go and internal/discover's tests still pass.

## Log

- 2026-09-07 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
