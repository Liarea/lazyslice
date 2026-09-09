---
id: T-0105
title: ".golangci.yml does not lint the torture build tag"
epic: E5
phase: 5
status: done
owner: ""
created: 2026-09-08
started: ""
closed: 2026-09-09
outcome: "done: in T-HARD-C (4f9a186)"
---

# T-0105 · .golangci.yml does not lint the torture build tag

## Goal

run.build-tags in .golangci.yml lists 'integration' alone, so the ~820 lines of internal/invariants/torture_test.go and torture_catalogue_test.go behind 'integration && torture' are linted by nothing in make lint or CI. Add 'torture' to run.build-tags. .golangci.yml was outside T-TORTURE's paths; the Makefile's vet-tagged target is the interim guard (it vets but does not lint them).

## Acceptance



## Log

- 2026-09-08 created

- 2026-09-08 moved to E5 phase 5

- 2026-09-09 closed: done: in T-HARD-C (4f9a186)

## Post-mortem

Went well: landed. Went badly: nothing. Change: none.
