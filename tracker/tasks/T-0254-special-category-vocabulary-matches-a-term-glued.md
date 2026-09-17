---
id: T-0254
title: "Special-category vocabulary matches a term glued to the next token by an underscore, and stripping a LIKE metacharacter leaves a separator"
epic: E5
phase: 5
status: open
owner: sonnet
created: 2026-09-17
started: ""
closed: ""
outcome: ""
---

# T-0254 · Special-category vocabulary matches a term glued to the next token by an underscore, and stripping a LIKE metacharacter leaves a separator

## Goal

Round-5 replay (docs/reviews/2026-09-15-redteam/round5-still-leaking.json, the secrets attacker's catalog variant): CHECK (note <> 'HIV_POSITIVE') on a masked column survives into the target because the vocabulary regexp uses word boundaries and _ is a word character; HIV_STATUS and TRADE_UNION_MEMBER are how a status code is spelled. Normalise before matching: map every non-alphanumeric run and camel-case boundary to one space, lowercase, then match; and make StripPatternMeta leave a separator where it removes LIKE's _ instead of gluing the tokens. Add the three canaries to the vocabulary test and a plan-level case asserting exit 13. Files: internal/textsig/special.go, internal/pipeline/ddlliteral.go, internal/plan/ddlliteral_test.go.

## Acceptance



## Log

- 2026-09-17 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
