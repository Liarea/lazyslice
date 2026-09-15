---
id: T-0186
title: "--allow-type-literal is not recorded in lazyslice.yml"
epic: E5
phase: 5
status: open
owner: ""
created: 2026-09-15
started: ""
closed: ""
outcome: ""
---

# T-0186 · --allow-type-literal is not recorded in lazyslice.yml

## Goal

--unmask TABLE.COL=REASON round-trips through lazyslice.yml (pipeline.Config.Columns[..].Unmask) so a re-run keeps the opt-out; --allow-type-literal TYPE=REASON, added for the T-REDFIX review's fourth finding, is flag-only, so every run of a schema with an opted-out enum label must pass it again or refuse at exit 13. Needs a types: block in internal/emit's reader and writer plus internal/core reading it as a prior, and a decision about ADR-004's only-tighten rule (an opt-out loosens).

## Acceptance



## Log

- 2026-09-15 created

- 2026-09-15 moved to E5 phase 5

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
