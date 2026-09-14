---
id: T-0141
title: "README.md and SECURITY.md no longer claim every stage is a no-op"
epic: E5
phase: 5
status: done
owner: haiku
created: 2026-09-14
started: 2026-09-14
closed: 2026-09-14
outcome: done
---

# T-0141 · README.md and SECURITY.md no longer claim every stage is a no-op

## Goal

README.md:9 and SECURITY.md:8 still say every pipeline stage is a documented no-op; phases 4 and 5 shipped the pipeline: docs/reviews/2026-09-09/REVIEW.md, collapse the sources of truth. Replace both sentences with the current state in one paragraph each (pre-release, Postgres only, what the gate and the masking do, where the limitations list is); no README rewrite, that is phase 6.

## Acceptance



## Log

- 2026-09-14 created

- 2026-09-14 started

- 2026-09-14 closed: done

## Post-mortem

went well: two paragraphs, done by the orchestrator in minutes once going public moved into phase 5 | change next time: a README status line that names the tagged version instead of a phase would not have gone stale
