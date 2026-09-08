---
id: T-0060
title: "Carry discovery provenance into the emitted yml: discover.Result and core.Request keep the rung and label, so a committed source: compose / source_label: db is not rewritten as source: flag"
epic: E5
phase: ""
status: done
owner: opus
created: 2026-09-07
started: 2026-09-07
closed: 2026-09-07
outcome: "done: discover.Result carries provenance and label through core.Request into emit; a committed source: compose survives argument-free reruns"
---

# T-0060 · Carry discovery provenance into the emitted yml: discover.Result and core.Request keep the rung and label, so a committed source: compose / source_label: db is not rewritten as source: flag

## Goal

internal/core builds sourceCand/targetCand as pipeline.FromFlag with an empty Label (internal/core/run.go:307, :358) and internal/emit writes those straight into lazyslice.yml (internal/emit/emit.go:114-118), with no merge against r.prior. T-DISCOVER made this reachable: an argument-free run now walks the ladder instead of stopping at exit 3, so every re-run in a directory that already has a lazyslice.yml downgrades the operator's committed provenance to from: flag with no service name, against ARCHITECTURE.md section 10's example. Carry Provenance and Label from discover.Result into core.Request (or into the Candidate core builds) so emit records the rung the endpoint actually came from. The cheap containment, if the carrier is deferred again, is for core to fall back to r.prior.Source/SourceLabel when the endpoint was not named on the command line, so an existing file is at least not downgraded.

## Acceptance

A run in a directory whose lazyslice.yml records source: compose / source_label: db, given no --source, re-emits the same two lines. A run given --source on the command line still emits source: flag. Needs internal/core, internal/discover and cmd/lazyslice in one change.

## Log

- 2026-09-07 created

- 2026-09-07 started

- 2026-09-07 closed: done: discover.Result carries provenance and label through core.Request into emit; a committed source: compose survives argument-free reruns

## Post-mortem

Went well: the round trip is pinned by a test. Went badly: blocked on two doc lines outside paths; the reviewers were right that the ADR wording invited a T2 fall-through. Change: orchestrator amended §9 and ADR-008 before the freeze.
