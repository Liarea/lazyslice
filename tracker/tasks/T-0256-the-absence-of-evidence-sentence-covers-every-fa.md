---
id: T-0256
title: "The absence-of-evidence sentence covers every family the validators ran over, and pluralises its count"
epic: E9
phase: ""
status: open
owner: sonnet
created: 2026-09-17
started: ""
closed: ""
outcome: ""
---

# T-0256 · The absence-of-evidence sentence covers every family the validators ran over, and pluralises its count

## Goal

T-0197's reviewer (two lows): the new sentence is gated on the character family while the validators also run over enum, uuid and domain-rendered columns, which still print 'no name or value signal' after being sampled; and 'nothing recognised in 1 samples' does not pluralise. Widen the gate to every family bestSignal did not early-return on, and render the count naturally. Files: internal/classify/classify.go, internal/classify/reasons.go, with T-0250's fixture pinning the strings.

## Acceptance



## Log

- 2026-09-17 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
