---
id: T-0038
title: "T-CLASSIFY: classify stage with rule pack"
epic: E4
phase: 4
status: done
owner: opus
created: 2026-09-05
started: 2026-09-06
closed: 2026-09-06
outcome: "done: classify with embedded rule pack, validators, English dictionary, accepted-type gate, FK propagation to a fixpoint, yml raise gate; unit tests green"
---

# T-0038 · T-CLASSIFY: classify stage with rule pack

## Goal



## Acceptance



## Log

- 2026-09-05 created

- 2026-09-06 started

- 2026-09-06 closed: done: classify with embedded rule pack, validators, English dictionary, accepted-type gate, FK propagation to a fixpoint, yml raise gate; unit tests green

## Post-mortem

Went well: reviewers' probes were reproduced verbatim and the developer proved the new tests fail on pre-fix code. Went badly: blocked on a go.mod tidy the developer was forbidden to run; the first fix round applied the FK rule from the parent side only. Change: tidy without new modules is allowed; probe exemption rules from the exempt side first.
