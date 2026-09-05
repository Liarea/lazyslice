---
id: T-0018
title: "Adversarial review of the decision set and one revision"
epic: E2
phase: 2
status: done
owner: opus
created: 2026-09-05
started: 2026-09-05
closed: 2026-09-05
outcome: "done: 42 findings from three lenses, all addressed; import cycle, target gate, planner determinism, JSON masking, frequency leaks fixed"
---

# T-0018 · Adversarial review of the decision set and one revision

## Goal



## Acceptance



## Log

- 2026-09-05 created

- 2026-09-05 started

- 2026-09-05 closed: done: 42 findings from three lenses, all addressed; import cycle, target gate, planner determinism, JSON masking, frequency leaks fixed

## Post-mortem

Went well: reviewers ran the design as code in their heads and found a non-compiling import cycle and a gate that refused every same-cluster target. Went badly: the revision had to edit ADRs already marked accepted; resolved by declaring the phase-2 review round the one allowed revision before the gate. Change: ADRs are proposed until the gate closes; the decide agent should write Status: proposed.
