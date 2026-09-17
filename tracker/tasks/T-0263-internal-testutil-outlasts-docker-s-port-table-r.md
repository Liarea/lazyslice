---
id: T-0263
title: "internal/testutil outlasts Docker's port-table race: a longer bounded wait for the mapped port and one container restart before failing"
epic: E5
phase: 5
status: open
owner: sonnet
created: 2026-09-17
started: ""
closed: ""
outcome: ""
---

# T-0263 · internal/testutil outlasts Docker's port-table race: a longer bounded wait for the mapped port and one container restart before failing

## Goal

The documented T-0052 race (a container reports ready before Docker's port table has its mapping) failed six torture subtests in one gate run on 2026-09-17 and blocked two landings this week on a 2 GB Docker VM shared with other containers; the helper gives up after 30 attempts over 30 s. Wait with a bounded backoff for up to 120 s, and on exhaustion terminate the container and start it once more before failing; keep the failure message naming T-0052 and say how long it waited and that it restarted. No product code changes. Proof: make torture three times in a row with the run times recorded. Files: internal/testutil.

## Acceptance



## Log

- 2026-09-17 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
