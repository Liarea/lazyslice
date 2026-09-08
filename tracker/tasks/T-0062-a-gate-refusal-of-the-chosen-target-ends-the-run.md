---
id: T-0062
title: "A gate refusal of the chosen target ends the run instead of falling through to the runner-up: core.Request carries one target, not a list"
epic: E5
phase: ""
status: open
owner: opus
created: 2026-09-07
started: ""
closed: ""
outcome: ""
---

# T-0062 · A gate refusal of the chosen target ends the run instead of falling through to the runner-up: core.Request carries one target, not a list

## Goal

internal/discover's chooseTarget applies ADR-008 section 5's order among reachable non-source candidates and hands one winner to internal/core. ADR-008 section 5's tie-break is written 'among eligible targets' and eligibility is Target.Gate's verdict, which needs a live connection discovery's 1s dial budget does not allow for. So when the gate refuses the winner, the run stops rather than trying the next candidate, because core.Request carries one target string and not a list. Reported by T-DISCOVER's review round, not fixed there.

## Acceptance

Either the ranked candidate list reaches the gate so a refusal can fall through to the runner-up (with each refusal still printed), or ADR-008 section 5 is amended to say the tie-break runs before the gate and one refusal ends the run. Whichever is chosen, internal/discover/CLAUDE.md's deviation note is updated to match.

## Log

- 2026-09-07 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
