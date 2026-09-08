---
id: T-0074
title: "Unsafe-flag rail enforced over the registered flag set: main_test's forbidden list permits exactly unmask and rejects every other name containing it; make unsafe-flags runs that test"
epic: E5
phase: 5
status: done
owner: sonnet
created: 2026-09-08
started: 2026-09-08
closed: 2026-09-08
outcome: "done: 2a35a36; forbidden rule walks every registered flag set recursively, permits exactly unmask, self-test proves it fires; make unsafe-flags runs the test"
---

# T-0074 · Unsafe-flag rail enforced over the registered flag set: main_test's forbidden list permits exactly unmask and rejects every other name containing it; make unsafe-flags runs that test

## Goal



## Acceptance



## Log

- 2026-09-08 created

- 2026-09-08 started

- 2026-09-08 closed: done: 2a35a36; forbidden rule walks every registered flag set recursively, permits exactly unmask, self-test proves it fires; make unsafe-flags runs the test

## Post-mortem

Went well: the rail now enforces over what cobra registers, not over source text. Went badly: the workflow's last agent returned without structured output after committing, so the run reported failure for a merged task. Change: none in the code; implement.js could tolerate a missing commit report when git log shows the commit.
