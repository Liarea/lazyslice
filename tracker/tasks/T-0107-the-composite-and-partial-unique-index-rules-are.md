---
id: T-0107
title: "The composite and partial unique-index rules are a decision with no ADR: land one or record the authorisation before gate 5"
epic: E9
phase: ""
status: done
owner: ""
created: 2026-09-08
started: ""
closed: 2026-09-08
outcome: "done: ADR-011 accepted"
---

# T-0107 · The composite and partial unique-index rules are a decision with no ADR: land one or record the authorisation before gate 5

## Goal

internal/classify's raiseCompositeUnique and indexKeys decide which columns carry Decision.UniqueIndex, which feeds internal/plan's exit-12 refusal, internal/transform's Constraints.Unique and the emitted lazyslice.yml. Root CLAUDE.md says decisions live in docs/adr/. No ADR and no ARCHITECTURE.md section 5 edit came with them because T-TORTURE's paths reached neither file; internal/classify/CLAUDE.md discloses this and names T-0099, whose Goal paragraph states both approximations precisely enough to be the ADR's text. Before gate 5 closes, the orchestrator does one of two things: land the ADR with T-0099's Goal as its Decision (T-0099 then only tightens the rules later), or record in the tracker that T-TORTURE was authorised to decide these rules. No code change is implied - the fail-towards-refusal direction is the one the reviewers agreed with.

## Acceptance



## Log

- 2026-09-08 created

- 2026-09-08 closed: done: ADR-011 accepted

## Post-mortem

Went well: T-0099's text became the ADR. Went badly: nothing. Change: none.
