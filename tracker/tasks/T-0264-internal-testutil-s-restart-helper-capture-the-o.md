---
id: T-0264
title: "internal/testutil's restart helper: capture the original container by value, report which container to clean up on every return, and normalise a typed-nil replacement"
epic: E9
phase: ""
status: open
owner: sonnet
created: 2026-09-17
started: ""
closed: ""
outcome: ""
---

# T-0264 · internal/testutil's restart helper: capture the original container by value, report which container to clean up on every return, and normalise a typed-nil replacement

## Goal

Three low findings from T-0263's review, all in internal/testutil/postgres.go: the terminate closure captures the variable ctr that is reassigned three lines later (capture orig := ctr instead); when terminate or recreate fails, portEndpointWithRestart returns nil and t.Cleanup then terminates a container that is already gone, logging a spurious No such container on top of the real failure; and if newCtr != nil compares an interface that can wrap a nil *DockerContainer. Test-harness noise on already-failing paths, not a leak, so it waits in Later.

## Acceptance



## Log

- 2026-09-17 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
