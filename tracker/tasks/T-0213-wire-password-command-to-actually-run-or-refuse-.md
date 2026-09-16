---
id: T-0213
title: "Wire --password-command to actually run, or refuse it as unimplemented"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-16
started: ""
closed: ""
outcome: ""
---

# T-0213 · Wire --password-command to actually run, or refuse it as unimplemented

## Goal

R2-16 (docs/reviews/2026-09-15-redteam/round2-still-leaking.json) part (b): --password-command is accepted, printed in --help and the post-prompt remedy text, and recorded into lazyslice.yml (now screened by T-0192's heuristic), but nothing in internal/discover or internal/dsn ever executes it, so a run that relies on it fails authentication. Either implement it in internal/discover (exec the command, trim one trailing newline, never log stdout) or make it exit 2 as unimplemented in this build; THREAT_MODEL.md T5's 2026-09-16 amendment records the gap and should be updated once this closes.

## Acceptance



## Log

- 2026-09-16 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
