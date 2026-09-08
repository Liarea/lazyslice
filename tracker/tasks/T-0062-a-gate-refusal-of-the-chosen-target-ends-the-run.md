---
id: T-0062
title: "A gate refusal of the chosen target ends the run instead of falling through to the runner-up: core.Request carries one target, not a list"
epic: E5
phase: ""
status: done
owner: opus
created: 2026-09-07
started: ""
closed: 2026-09-07
outcome: "done: one target per request, gate refusal ends the run at exit 4, pinned by an integration test; ARCHITECTURE.md §9 and ADR-008 §5 now say so"
---

# T-0062 · A gate refusal of the chosen target ends the run instead of falling through to the runner-up: core.Request carries one target, not a list

## Goal

internal/discover's chooseTarget applies ADR-008 section 5's order among reachable non-source candidates and hands one winner to internal/core. ADR-008 section 5's tie-break is written 'among eligible targets' and eligibility is Target.Gate's verdict, which needs a live connection discovery's 1s dial budget does not allow for. So when the gate refuses the winner, the run stops rather than trying the next candidate, because core.Request carries one target string and not a list. Reported by T-DISCOVER's review round, not fixed there.

## Acceptance

Either the ranked candidate list reaches the gate so a refusal can fall through to the runner-up (with each refusal still printed), or ADR-008 section 5 is amended to say the tie-break runs before the gate and one refusal ends the run. Whichever is chosen, internal/discover/CLAUDE.md's deviation note is updated to match.

## Log

- 2026-09-07 created

- 2026-09-07 closed: done: one target per request, gate refusal ends the run at exit 4, pinned by an integration test; ARCHITECTURE.md §9 and ADR-008 §5 now say so

## Post-mortem

Went well: the safe behaviour was already the code's; the docs were the risk. Went badly: nothing. Change: none.
