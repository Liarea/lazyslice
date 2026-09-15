---
id: T-0198
title: "special_category has no value validator; a digit/name-free special-category sentence still crosses unseen"
epic: E9
phase: ""
status: open
owner: sonnet
created: 2026-09-15
started: ""
closed: ""
outcome: ""
---

# T-0198 · special_category has no value validator; a digit/name-free special-category sentence still crosses unseen

## Goal

ARCHITECTURE.md section 11.1's 2026-09-15 amendment (item 1, T-0189) and THREAT_MODEL.md:47 list R2-09's special-category sentence as closed. It is closed only by accident: pipeline.CatSpecial has no value validator anywhere -- internal/plan/ddlliteral.go's strongValidators, internal/verify/catalog.go's strongCatalogHit and internal/verify/validators.go's second-net list all have no CatSpecial entry. R2-09's own canary 'Priya Raghunathan disclosed her HIV diagnosis on 2019-04-02' is caught only because the date supplies textsig.AddressShape's digit; remove the date ('...last spring') and every strongHit call returns "" -- exit 0 on a health/special-category sentence with no name pair and no digit, in a DDL literal or a row. Fix: give special_category its own value validator (vocabulary or shape signal for disease/condition/orientation/religion sentences) in internal/textsig, and wire it into the three call sites above; pin the date-stripped canary in testdata/regressions at expect: refused, both as a DDL literal and as a row. No new flag needed unless the validator proves too broad over ordinary prose, in which case the existing --unmask/--allow-type-literal escapes are the answer.

## Acceptance



## Log

- 2026-09-15 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
