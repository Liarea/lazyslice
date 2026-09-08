---
id: T-0068
title: "T-POLY: polymorphic association inference"
epic: E5
phase: 5
status: done
owner: opus
created: 2026-09-07
started: 2026-09-08
closed: 2026-09-08
outcome: "done: inference of _type/_id and content_type/object_id pairs, virtual parent-direction edges under the caps, unmapped values reported once, over-cap pairs explained; full integration green"
---

# T-0068 · T-POLY: polymorphic association inference

## Goal



## Acceptance



## Log

- 2026-09-07 created

- 2026-09-08 started

- 2026-09-08 closed: done: inference of _type/_id and content_type/object_id pairs, virtual parent-direction edges under the caps, unmapped values reported once, over-cap pairs explained; full integration green

## Post-mortem

Went well: three of five findings closed in-package with fixtures for each. Went badly: the followed edges are not printed by core, so the plan shows less than before; two design numbers lived only in comments. Change: §3.2 amended by the orchestrator; a follow-up prints the edges and fixes the fixture promises.
