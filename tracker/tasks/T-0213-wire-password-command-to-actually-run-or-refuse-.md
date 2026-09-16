---
id: T-0213
title: "Wire --password-command to actually run, or refuse it as unimplemented"
epic: E5
phase: 5
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

- 2026-09-16 moved to E5 phase 5

- 2026-09-16 re-homed to E5 by the orchestrator, 2026-09-16: a documented flag that silently fails authentication cannot ship in v0.1.0; decision: implement it in discover (run the command, trim one trailing newline, stdout never reaches a sink), added to the redfix list after T-0178

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
