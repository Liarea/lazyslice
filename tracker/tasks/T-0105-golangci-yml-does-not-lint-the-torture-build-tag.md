---
id: T-0105
title: ".golangci.yml does not lint the torture build tag"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-08
started: ""
closed: ""
outcome: ""
---

# T-0105 · .golangci.yml does not lint the torture build tag

## Goal

run.build-tags in .golangci.yml lists 'integration' alone, so the ~820 lines of internal/invariants/torture_test.go and torture_catalogue_test.go behind 'integration && torture' are linted by nothing in make lint or CI. Add 'torture' to run.build-tags. .golangci.yml was outside T-TORTURE's paths; the Makefile's vet-tagged target is the interim guard (it vets but does not lint them).

## Acceptance



## Log

- 2026-09-08 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
