---
id: T-0178
title: "TestAStoppedContainerIsOfferedAndStarted binds a fixed host port (127.0.0.1:5433) and collides under parallel container runs"
epic: E9
phase: ""
status: done
owner: sonnet
created: 2026-09-15
started: 2026-09-16
closed: 2026-09-16
outcome: done
---

# T-0178 · TestAStoppedContainerIsOfferedAndStarted binds a fixed host port (127.0.0.1:5433) and collides under parallel container runs

## Goal

The T-PERF verify run of make integration failed once with Bind for 127.0.0.1:5433 failed: port is already allocated from internal/discover/discover_integration_test.go:170 (provisioning rung 4); alone it passes. The 2026-09-09 review saw the same fixed-port conflict. Pick a free loopback port per run (or a testcontainers-mapped one) and leave the rung-4 semantics unchanged. internal/discover, internal/discover/provision.

## Acceptance



## Log

- 2026-09-15 created

- 2026-09-16 started

- 2026-09-16 closed: done

## Post-mortem

went well: the package's own CLAUDE.md had diagnosed the race and prescribed the fix; testing against a real permanently colliding port caught FreePort reading a Docker-proxied port as free; the Opus reviewer caught the retry being switched off for every interactive create because the target question always names a port; one fix round | went badly: the workflow's commit step was blocked by a safety classifier, so the files were swept into T-0186's commit by git add -A and the orchestrator rewrote the two unpushed commits to separate them; three low findings filed | change next time: the commit agent stages the task's paths rather than git add -A, so a blocked commit cannot bleed into the next task
