---
id: T-0184
title: "Should a headless run auto-select a target on the source's own cluster?"
epic: E5
phase: 5
status: done
owner: ""
created: 2026-09-15
started: 2026-09-16
closed: 2026-09-16
outcome: done
---

# T-0184 · Should a headless run auto-select a target on the source's own cluster?

## Goal

The 2026-09-15 red team pointed --source at production with no --target and --yes, and the discovery ladder chose the production container and wrote the masked slice into its postgres maintenance database, exit 0. T-REDFIX fixed what it could without superseding a frozen decision: internal/core now emits the target.warn.same_cluster line ARCHITECTURE.md section 9 rule 1 has always promised and nothing read, internal/discover no longer ranks a maintenance database as a target at all, and a candidate off the source's cluster now outranks one on it. What is left is a product decision: should a --yes run with no --target refuse when every reachable candidate is on the source's own cluster, and name --target? ARCHITECTURE.md section 9 rule 1 and ADR-008 section 5 both say a second database on one cluster is eligible, and internal/core/gate_integration_test.go's TestAGateRefusalEndsTheRunInsteadOfTryingTheRunnerUp pins a run whose only candidates are exactly that, so changing it is a superseding ADR and not an edit.

## Acceptance



## Log

- 2026-09-15 created

- 2026-09-15 moved to E5 phase 5

- 2026-09-16 started

- 2026-09-16 closed: done

## Post-mortem

went well: the discover-level refusal, the gate-stage escalation and the explicit TargetNamed bit are sound; the Opus reviewer caught a committed-yml target wrongly refused at the gate because rung 0 never carries FromYml provenance, and caught ADR-013 asserting a test still passed while it was red; the by-hand finish was small, export one field and thread one seam, and the revert-and-watch reproduced the reverify's failure exactly | went badly: two fix rounds and a by-hand landing; the brief said --yes while the product's headless predicate is --yes or no controlling terminal, so the reviewer widened the rule in round 1 and the docs lagged the code to the end; the discover integration suite cannot pass on this machine while another container holds host port 5433, so four provisioning tests were skipped in the landing run (T-0178 lands next) | change next time: a brief that names a predicate uses the product's existing definition by name instead of restating it; an ADR paragraph that claims a test passes is written after running that test
