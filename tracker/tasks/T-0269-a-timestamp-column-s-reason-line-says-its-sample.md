---
id: T-0269
title: "A timestamp column's reason line says its samples look like secrets"
epic: E6
phase: 6
status: done
owner: sonnet
created: 2026-09-17
started: ""
closed: 2026-09-22
outcome: done
---

# T-0269 · A timestamp column's reason line says its samples look like secrets

## Goal

docs/QUICKSTART_TRANSCRIPT.md shows 'public.orders.placed_at: 200/200 samples look like secrets; timestamp is not an accepted type for credential; no name signal', and T-0260's re-run printed 'no name or value signal' for the same column, so the sentence depends on the sample. The decision is right either way (the column is not masked); the reason line is noise that reads like an alarm. internal/classify should not score the credential entropy signal on a type credential can never be, or internal/render should not print a signal the type check discarded.

## Acceptance

—

## Log

- 2026-09-22 2026-09-22 moved to E6 phase 6
- 2026-09-22 closed: done

## Post-mortem

went well: ea8f5a7; a timestamp column now reads 'no name or value signal'; the fix round kept every masking decision identical by keeping a bare silencedStrong flag, so the raising passes still stay off such a column; the GIF's first screen confirms it | went badly: the first cut dropped the refused-signal state and widened recall, which the reviewer caught | change next time: a reason-line task's brief should say in its first sentence that no decision may change, and ask for a before/after decision diff over Pagila
