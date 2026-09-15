---
id: T-0171
title: "Add TestTortureNegativeControl to Makefile TORTURE_TESTS guard list"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-14
started: ""
closed: ""
outcome: ""
---

# T-0171 · Add TestTortureNegativeControl to Makefile TORTURE_TESTS guard list

## Goal

Makefile:108 TORTURE_TESTS omits TestTortureNegativeControl, so a rename/deletion of the negative control (the test proving the I4 comparison can actually fail) would leave 'make torture' silently reporting success. Append it to TORTURE_TESTS in Makefile. Found during T-0139 review; Makefile is outside internal/invariants/ so out of scope for that task.

## Acceptance



## Log

- 2026-09-14 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
