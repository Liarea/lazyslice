---
id: T-0125
title: "Makefile's vet-tagged comment still says .golangci.yml does not lint the torture tag"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-09
started: ""
closed: ""
outcome: ""
---

# T-0125 · Makefile's vet-tagged comment still says .golangci.yml does not lint the torture tag

## Goal

T-HARD-C (T-0105) added 'torture' to .golangci.yml's run.build-tags, so make lint now runs the full linter set over internal/invariants/torture_test.go and torture_catalogue_test.go as well as the integration files. The Makefile's vet-tagged recipe comment still says the opposite: ".golangci.yml's run.build-tags lists integration alone, so make lint still does not lint the torture files; that file was outside T-TORTURE's paths and T-0105 is the task that adds the tag there." The Makefile was outside T-HARD-C's paths. Owed: that paragraph says vet-tagged is now the type-check-and-vet half beside a lint that does cover both tag sets, and drops the T-0105 pointer. Nothing about the recipe itself changes; vet-tagged is still worth keeping, because go vet type-checks the tagged packages under both tag sets and golangci-lint is configured with one build-tags list.

## Acceptance



## Log

- 2026-09-09 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
