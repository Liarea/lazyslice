---
id: T-0047
title: "T-PGSHAPES: statement-shape grammar placeholders for plan and extract; plan exports Shapes()"
epic: E4
phase: 4
status: done
owner: opus
created: 2026-09-06
started: ""
closed: 2026-09-06
outcome: "done: grammar gains select-list item, variable-arity cast list, --where predicate with exclusions; plan exports Shapes() and its suite runs through pg.Source; extract shapes staged"
---

# T-0047 · T-PGSHAPES: statement-shape grammar placeholders for plan and extract; plan exports Shapes()

## Goal



## Acceptance



## Log

- 2026-09-06 created

- 2026-09-06 closed: done: grammar gains select-list item, variable-arity cast list, --where predicate with exclusions; plan exports Shapes() and its suite runs through pg.Source; extract shapes staged

## Post-mortem

Went well: reviewers probed the {where} placeholder with injection shapes and found a SELECT INTO the old cast regex admitted; fixed before any extract statement existed. Went badly: blocked on a THREAT_MODEL.md wording outside paths; extract's shapes had to live in pg temporarily. Change: orchestrator amends threat model; extract moves its shapes home.
