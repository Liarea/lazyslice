---
id: T-0263
title: "internal/testutil outlasts Docker's port-table race: a longer bounded wait for the mapped port and one container restart before failing"
epic: E5
phase: 5
status: done
owner: sonnet
created: 2026-09-17
started: ""
closed: 2026-09-17
outcome: done
---

# T-0263 · internal/testutil outlasts Docker's port-table race: a longer bounded wait for the mapped port and one container restart before failing

## Goal

The documented T-0052 race (a container reports ready before Docker's port table has its mapping) failed six torture subtests in one gate run on 2026-09-17 and blocked two landings this week on a 2 GB Docker VM shared with other containers; the helper gives up after 30 attempts over 30 s. Wait with a bounded backoff for up to 120 s, and on exhaustion terminate the container and start it once more before failing; keep the failure message naming T-0052 and say how long it waited and that it restarted. No product code changes. Proof: make torture three times in a row with the run times recorded. Files: internal/testutil.

## Acceptance



## Log

- 2026-09-17 created

- 2026-09-17 closed: done

## Post-mortem

went well: the helper now waits 120 s for the mapped port, restarts the container once through the reaper-retry path, and gives the fresh container a shorter 30 s budget; three consecutive make torture runs passed at 127-131 s, and a mutation of the fast-path guard proved the new no-restart test bites | went badly: the first submission skipped the integration and torture runs its own brief named as the proof, so the review's high finding was about missing evidence and the fix round had to produce it from scratch | change next time: when a task's proof names a gate, the developer runs it last and pastes it; a report without it is sent back before review rather than reviewed (commit 59425d8)
