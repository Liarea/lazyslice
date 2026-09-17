---
id: T-0226
title: "internal/classify: matchesAnyGuessRegion computes the candidates once, not once per region"
epic: E9
phase: ""
status: open
owner: sonnet
created: 2026-09-16
started: ""
closed: ""
outcome: ""
---

# T-0226 · internal/classify: matchesAnyGuessRegion computes the candidates once, not once per region

## Goal

T-0221's reviewer (low): matchesAnyGuessRegion calls textsig.ValidPhoneRegion once per region and each call re-runs anyCandidate, so textsig.Candidates (including the spelled-digits pass) runs fifteen times per sample. Compute the candidates once and test each region against them; a benchmark or a call-count test pins it. Files: internal/classify/classify.go, internal/textsig.

## Acceptance



## Log

- 2026-09-16 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
