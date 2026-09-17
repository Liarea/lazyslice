---
id: T-0269
title: "A timestamp column's reason line says its samples look like secrets"
epic: E9
phase: ""
status: open
owner: sonnet
created: 2026-09-17
started: ""
closed: ""
outcome: ""
---

# T-0269 · A timestamp column's reason line says its samples look like secrets

## Goal

docs/QUICKSTART_TRANSCRIPT.md shows 'public.orders.placed_at: 200/200 samples look like secrets; timestamp is not an accepted type for credential; no name signal', and T-0260's re-run printed 'no name or value signal' for the same column, so the sentence depends on the sample. The decision is right either way (the column is not masked); the reason line is noise that reads like an alarm. internal/classify should not score the credential entropy signal on a type credential can never be, or internal/render should not print a signal the type check discarded.

## Acceptance



## Log

- 2026-09-17 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
