---
id: T-0140
title: "CI runs the torture suite on main and the release workflow requires a green CI run for the tagged commit"
epic: E5
phase: 5
status: done
owner: sonnet
created: 2026-09-14
started: 2026-09-14
closed: 2026-09-14
outcome: done
---

# T-0140 · CI runs the torture suite on main and the release workflow requires a green CI run for the tagged commit

## Goal

.github/workflows/ci.yml runs make integration but never make torture, and release.yml runs make check only, so a tagged commit need not have passed the database suites it relies on: docs/reviews/2026-09-09/REVIEW.md, release reality. Add a torture job to ci.yml on pushes to main only (it needs Docker and about a minute), and make the release build refuse unless the ci workflow succeeded on the same sha (a first step that queries the checks API and fails otherwise). Do not widen the matrix. Note the gate in docs/RUNBOOK.md.

## Acceptance



## Log

- 2026-09-14 created

- 2026-09-14 started

- 2026-09-14 closed: done

## Post-mortem

went well: torture job on pushes to main; release refuses a tag whose commit has no green push-to-main ci run, filtered server-side and re-checked (6cae95e, one fix round) | went badly: nothing | change next time: nothing
