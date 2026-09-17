---
id: T-0253
title: "The certain-neighbour rail raises both ends of a validated foreign key together, or refuses the pair"
epic: E5
phase: 5
status: open
owner: sonnet
created: 2026-09-17
started: ""
closed: ""
outcome: ""
---

# T-0253 · The certain-neighbour rail raises both ends of a validated foreign key together, or refuses the pair

## Goal

Round-5 replay (docs/reviews/2026-09-15-redteam/round5-still-leaking.json, the classifier attacker's FK variant): T-0239's fix-round exclusion takes a validated FK child out of the rail's reach so a rail-masked child cannot disagree with an unmasked parent, but a varchar(12) name column made an FK child of a dimension table that holds the same names, beside a certain email column, now copies verbatim with no name hit, no value hit and nothing to propagate; the exclusion's comment claims a personal FK column is reached by every other pass, and this schema is the disproof. When a raisable column is at either end of a validated FK, raise both ends under the same category so the join stays in agreement; where the partner is itself excluded for another reason, refuse at exit 12 naming the pair and --unmask, the way plan refuses a unique column free_text cannot fill; never copy the pair. Regression: the native-script FK child with a certain neighbour and a parent with none; regression 031 keeps pinning the safe direction. Files: internal/classify, testdata/regressions, docs/TORTURE.md if a schema moves, THREAT_MODEL.md T1's 2026-09-17 amendment.

## Acceptance



## Log

- 2026-09-17 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
