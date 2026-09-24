---
id: T-0321
title: "The reasons dump and the plan end with a summary, and a re-run from a committed yml is quiet"
epic: E6
phase: 6
status: done
owner: sonnet
created: 2026-09-23
started: ""
closed: 2026-09-24
outcome: done
---

# T-0321 · The reasons dump and the plan end with a summary, and a re-run from a committed yml is quiet

## Goal

Dogfood session 1: 1,845 reason lines scrolled past before each refusal, and the plan listed 114 of 143 tables as schema_only; unreachable one per line; nothing summarised either. Session 2: a re-run from the committed yml (drift 0) printed all 1,806 reason lines again. Print one line after the reasons (N columns: A masked, B copied, C never-masked keys) and one after the plan (T tables reached, U unreachable, R rows); on a re-run with a yml print only drift and changes plus a one-line 'N decisions from ./lazyslice.yml, D drift'; --json emits the same as events.

## Acceptance

—

## Log

- 2026-09-23 2026-09-23 created
- 2026-09-24 closed: done

## Post-mortem

went well: landed as da64ebb after one review round (two high, one medium, three low): the reasons dump and the plan end with a summary line and a re-run from a committed yml is quiet in the transcript; the review caught that the first cut silenced the --json stream and the reasons screen too, and the fix keeps every column event on the sink and marks settled ones so only the human transcript skips them | went badly: three lows filed as a follow-up (the reused count includes expired opt-outs; the never-masked bucket keys on a substring of classify's reason grammar; plan.summary counts skipped and unreadable tables as unreachable) | change next time: a brief that quiets output says where the quiet applies (the transcript) and names the sinks that must stay complete (--json, the screens)
