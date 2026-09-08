---
id: T-0067
title: "T-PROVISION: --create-target and rung 4 (T-0063)"
epic: E5
phase: 5
status: done
owner: opus
created: 2026-09-07
started: 2026-09-08
closed: 2026-09-08
outcome: "done: provisioning, rung 4, Q1 prompter on the controlling terminal; the four core-side joins (Yes into Options, Provisioner interface, ArgContainer, unreachable args) are in a verified patch applied by the follow-up"
---

# T-0067 · T-PROVISION: --create-target and rung 4 (T-0063)

## Goal



## Acceptance



## Log

- 2026-09-07 created

- 2026-09-08 started

- 2026-09-08 closed: done: provisioning, rung 4, Q1 prompter on the controlling terminal; the four core-side joins (Yes into Options, Provisioner interface, ArgContainer, unreachable args) are in a verified patch applied by the follow-up

## Post-mortem

Went well: reviewers found --yes was never joined to the ladder, which would have hung a TTY-bearing CI runner on a prompt. Went badly: four findings sat outside the task's paths again; the developer answered with a patch the reviewer verified. Change: provisioning tasks get internal/core in paths; a follow-up applies the patch.
