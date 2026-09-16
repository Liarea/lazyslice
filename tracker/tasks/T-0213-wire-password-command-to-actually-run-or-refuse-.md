---
id: T-0213
title: "Wire --password-command to actually run, or refuse it as unimplemented"
epic: E5
phase: 5
status: done
owner: ""
created: 2026-09-16
started: 2026-09-16
closed: 2026-09-16
outcome: done
---

# T-0213 · Wire --password-command to actually run, or refuse it as unimplemented

## Goal

R2-16 (docs/reviews/2026-09-15-redteam/round2-still-leaking.json) part (b): --password-command is accepted, printed in --help and the post-prompt remedy text, and recorded into lazyslice.yml (now screened by T-0192's heuristic), but nothing in internal/discover or internal/dsn ever executes it, so a run that relies on it fails authentication. Either implement it in internal/discover (exec the command, trim one trailing newline, never log stdout) or make it exit 2 as unimplemented in this build; THREAT_MODEL.md T5's 2026-09-16 amendment records the gap and should be updated once this closes.

## Acceptance



## Log

- 2026-09-16 created

- 2026-09-16 moved to E5 phase 5

- 2026-09-16 re-homed to E5 by the orchestrator, 2026-09-16: a documented flag that silently fails authentication cannot ship in v0.1.0; decision: implement it in discover (run the command, trim one trailing newline, stdout never reaches a sink), added to the redfix list after T-0178

- 2026-09-16 started

- 2026-09-16 closed: done

## Post-mortem

went well: the flag now runs the command with its output kept out of every sink; the Opus reviewer caught that the ladder-resolved endpoint, the case the red team reported, still never ran it, plus an unenforced timeout, a Windows carriage return and an egress test that never reached the sinks; two fix rounds, reverify clean | went badly: the developer's structured return was almost empty (changelog 'a', post-mortem 'ok') and the workflow's commit body carried it, so the commit on main has no usable release-note body and an overrides file now supplies one; fix round 1 introduced a nil-pointer panic on every probe outside walk, caught by the reverify | change next time: the developer schema enforces a minimum length on changelog bullets and the post-mortem (done in the same landing)
