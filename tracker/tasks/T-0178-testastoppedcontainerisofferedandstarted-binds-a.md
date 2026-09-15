---
id: T-0178
title: "TestAStoppedContainerIsOfferedAndStarted binds a fixed host port (127.0.0.1:5433) and collides under parallel container runs"
epic: E9
phase: ""
status: open
owner: sonnet
created: 2026-09-15
started: ""
closed: ""
outcome: ""
---

# T-0178 · TestAStoppedContainerIsOfferedAndStarted binds a fixed host port (127.0.0.1:5433) and collides under parallel container runs

## Goal

The T-PERF verify run of make integration failed once with Bind for 127.0.0.1:5433 failed: port is already allocated from internal/discover/discover_integration_test.go:170 (provisioning rung 4); alone it passes. The 2026-09-09 review saw the same fixed-port conflict. Pick a free loopback port per run (or a testcontainers-mapped one) and leave the rung-4 semantics unchanged. internal/discover, internal/discover/provision.

## Acceptance



## Log

- 2026-09-15 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
