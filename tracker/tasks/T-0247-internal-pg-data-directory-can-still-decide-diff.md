---
id: T-0247
title: "internal/pg: data_directory can still decide difference, the discover wiring of the standby rails has no test, and a comment on the role grant is garbled"
epic: E9
phase: ""
status: open
owner: sonnet
created: 2026-09-17
started: ""
closed: ""
outcome: ""
---

# T-0247 · internal/pg: data_directory can still decide difference, the discover wiring of the standby rails has no test, and a comment on the role grant is garbled

## Goal

T-0241's reviewer (three lows): clusterIDDifferenceUnreliableField lets a readable data_directory that differs decide a confident difference between two clusters that are one; every T-0241 test is a pure-function pin and nothing drives the standby header or the headless refusal through resolveEndpoints; the comment on the new pg_control_system grant in internal/core/names.go contradicts itself. Files: internal/pg/source.go, internal/core/run.go and names.go, an integration test over a normal container for the wiring.

## Acceptance



## Log

- 2026-09-17 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
