---
id: T-0236
title: "internal/plan: the special-category rule's loose ends: similar_escape's second argument, the bare 'aids' term, enum default lookup by bare name, and the address false-positive controls"
epic: E9
phase: ""
status: open
owner: sonnet
created: 2026-09-17
started: ""
closed: ""
outcome: ""
---

# T-0236 · internal/plan: the special-category rule's loose ends: similar_escape's second argument, the bare 'aids' term, enum default lookup by bare name, and the address false-positive controls

## Goal

T-0198's reviewer (lows and one medium left after the narrowing): afterPatternOperator's comment claims the scanner only sees similar_escape's first argument, which is false, accept both arguments as Pattern; the special-category vocabulary matches bare 'aids' ('visual aids', 'hearing aids'), require the phrase or the uppercase spelling with negative cases in the vocabulary test; enumDefaultLabels' bare-name fallback iterates a map and is nondeterministic when two schemas declare an enum of the same name, qualify the lookup or exempt only when exactly one matches; TestAddressStrongHitNeedsAStreetSuffixWord's four business-label controls all refuse now on a masked column, keep one on an unmasked column asserting nil. Files: internal/pipeline/ddlliteral.go, internal/textsig/special.go, internal/plan/ddlliteral.go and its test.

## Acceptance



## Log

- 2026-09-17 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
