---
id: T-0220
title: "internal/discover/password.go: the deadline check reads the derived context, the URL branch of injectPassword swallows a parse error, and the stop reason repeats the prefix"
epic: E9
phase: ""
status: open
owner: sonnet
created: 2026-09-16
started: ""
closed: ""
outcome: ""
---

# T-0220 · internal/discover/password.go: the deadline check reads the derived context, the URL branch of injectPassword swallows a parse error, and the stop reason repeats the prefix

## Goal

T-0213's reviewer (three lows): the 30 second budget cannot be told from the caller's cancellation, so a Ctrl-C reports 'exited signal: killed' as a timeout; injectPassword's URL branch falls through on a url.Parse error and appends a keyword to a postgres:// string; passwordCommandStop seeds its reason with an error that already carries the '--password-command' prefix. Files: internal/discover/password.go, internal/core/run.go, with a unit test per case.

## Acceptance



## Log

- 2026-09-16 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
