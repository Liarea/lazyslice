---
id: T-0184
title: "Should a headless run auto-select a target on the source's own cluster?"
epic: E5
phase: 5
status: open
owner: ""
created: 2026-09-15
started: ""
closed: ""
outcome: ""
---

# T-0184 · Should a headless run auto-select a target on the source's own cluster?

## Goal

The 2026-09-15 red team pointed --source at production with no --target and --yes, and the discovery ladder chose the production container and wrote the masked slice into its postgres maintenance database, exit 0. T-REDFIX fixed what it could without superseding a frozen decision: internal/core now emits the target.warn.same_cluster line ARCHITECTURE.md section 9 rule 1 has always promised and nothing read, internal/discover no longer ranks a maintenance database as a target at all, and a candidate off the source's cluster now outranks one on it. What is left is a product decision: should a --yes run with no --target refuse when every reachable candidate is on the source's own cluster, and name --target? ARCHITECTURE.md section 9 rule 1 and ADR-008 section 5 both say a second database on one cluster is eligible, and internal/core/gate_integration_test.go's TestAGateRefusalEndsTheRunInsteadOfTryingTheRunnerUp pins a run whose only candidates are exactly that, so changing it is a superseding ADR and not an edit.

## Acceptance



## Log

- 2026-09-15 created

- 2026-09-15 moved to E5 phase 5

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
