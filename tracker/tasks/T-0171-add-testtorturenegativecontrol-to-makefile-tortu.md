---
id: T-0171
title: "Add TestTortureNegativeControl to Makefile TORTURE_TESTS guard list"
epic: E9
phase: ""
status: done
owner: ""
created: 2026-09-14
started: 2026-09-14
closed: 2026-09-14
outcome: done
---

# T-0171 · Add TestTortureNegativeControl to Makefile TORTURE_TESTS guard list

## Goal

Makefile:108 TORTURE_TESTS omits TestTortureNegativeControl, so a rename/deletion of the negative control (the test proving the I4 comparison can actually fail) would leave 'make torture' silently reporting success. Append it to TORTURE_TESTS in Makefile. Found during T-0139 review; Makefile is outside internal/invariants/ so out of scope for that task.

## Acceptance



## Log

- 2026-09-14 created

- 2026-09-14 started

- 2026-09-14 closed: done

## Post-mortem

went well: one word in the Makefile, by the orchestrator | change next time: nothing
