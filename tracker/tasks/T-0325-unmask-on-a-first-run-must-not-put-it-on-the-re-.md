---
id: T-0325
title: "--unmask on a first run must not put it on the re-run path; the drift message names the real decision"
epic: E6
phase: 6
status: done
owner: sonnet
created: 2026-09-23
started: ""
closed: 2026-09-24
outcome: done
---

# T-0325 · --unmask on a first run must not put it on the re-run path; the drift message names the real decision

## Goal

Dogfood session 1: with no ./lazyslice.yml in the directory, a run with --unmask flags printed 1,835 lines of '! public.T.C is not in ./lazyslice.yml: classified fresh and masked at or above possible', one per column not named by a flag; the same run without --unmask printed none. The flags build an in-memory config that the drift pass then treats as a committed file. Drift is only reported against a file that exists; and the message must say what actually happened to the column (copied, or masked as CATEGORY), since it printed 'masked at or above possible' for boolean columns that were copied. Pin both.

## Acceptance

—

## Log

- 2026-09-23 2026-09-23 created
- 2026-09-24 closed: done

## Post-mortem

went well: landed as 37fe2c8 with no review finding above low; the first developer's diagnosis was right and minimal (drift is reported only when a committed yml exists, and the drift line names the run's verdict, copied or masked as CATEGORY) | went badly: the first developer's return call put the changelog inside the summary text and failed the return schema five times, so the batch died with the work complete in the tree and T-0326 never ran; a second implement.js run, briefed to read the saved report and verify the diff, landed it in nine minutes | change next time: implement.js's developer prompt says in one line that every field is a top-level key of the returned object and never text inside another field
