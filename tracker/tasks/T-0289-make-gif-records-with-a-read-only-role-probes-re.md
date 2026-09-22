---
id: T-0289
title: "make gif records with a read-only role, probes readiness from the host, and pace.awk fails when it paused nothing"
epic: E6
phase: 6
status: done
owner: sonnet
created: 2026-09-22
started: ""
closed: 2026-09-22
outcome: done
---

# T-0289 · make gif records with a read-only role, probes readiness from the host, and pace.awk fails when it paused nothing

## Goal

Three lows from T-0065's review, all in the recording pipeline. (1) The demo connects as the container superuser, so the GIF's second line is the four-line 'the role postgres can write to 15 table(s)' warning: have make gif create the read-only role the warning itself recommends in the source container and hand the tape that DSN, so the flagship recording shows the recommended shape. (2) Readiness is probed with pg_isready inside the container, which answers during the entrypoint's temporary socket-only server; probe from the host over the published port instead. (3) docs/media/pace.awk hard-codes the two-space Decision marker and exits 0 when it matched nothing, so a message change silently removes the pause; anchor on the column name and exit non-zero if fewer than three lines matched. Re-record and read the first and last frames back.

## Acceptance

—

## Log

- 2026-09-22 2026-09-22 created
- 2026-09-22 2026-09-22 moved to E6 phase 6
- 2026-09-22 closed: done

## Post-mortem

went well: 70c496a merged through implement.js on the first pass: make gif creates a read-only role (no write warning in the recording), probes readiness from the host, pace.awk anchors on column names and fails loudly; the orchestrator read the first and last frames back (one-word synthetic names, no warning) | went badly: nothing in the task; the run's next task failed on its output schema, not this one | change next time: nothing
