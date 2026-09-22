---
id: T-0065
title: "20-second VHS GIF of the first run on Pagila"
epic: E6
phase: 6
status: done
owner: sonnet
created: 2026-09-15
started: ""
closed: 2026-09-22
outcome: done
---

# T-0065 · 20-second VHS GIF of the first run on Pagila

## Goal

Gate 4 item carried into launch material; vhs is installed

## Acceptance

—

## Log

- 2026-09-22 2026-09-22 2026-09-22 phase 6 opens with this. Decided: the tape lives at docs/media/first-run.tape and the GIF at docs/media/first-run.gif, committed once (regenerate only for a v0.x minor); it records the real binary against the Pagila fixture in two disposable postgres:16 containers (testdata/pagila, the same files internal/testutil loads), 1200x700, under 25 s, with a pause on the per-column reasons; make gif runs it and refuses if the GIF is over 4 MB; nothing in the tape types a password or a real value.
- 2026-09-22 closed: done

## Post-mortem

went well: make gif records the real binary against the Pagila fixture in two disposable containers bound to loopback, waits on the prompt rather than sleeping, refuses a GIF over 4 MB and fails unless the run's own exit status reads 0; the GIF is 1.35 MB and 9 s, the reasons wall is readable at a pause (commit c029db3) | went badly: the first recording ended on the recording's own status-capture line, because a Hide/Show pair stops frames but leaves the typed line on the terminal; nobody looked at the final frame before merging, and the orchestrator re-recorded it with the capture inside a hidden shell function (533e446). Also learned the hard way: VHS strings take no backslash escapes | change next time: a task that produces an image proves it by reading the first and last frames back, not the exit code (commits c029db3, 533e446)
