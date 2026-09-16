---
id: T-0217
title: "internal/discover/provision: drop Force from the failed-attempt container removal, fix the retry count's off-by-one, and keep the daemon's port error when the range is exhausted"
epic: E9
phase: ""
status: open
owner: sonnet
created: 2026-09-16
started: ""
closed: ""
outcome: ""
---

# T-0217 · internal/discover/provision: drop Force from the failed-attempt container removal, fix the retry count's off-by-one, and keep the daemon's port error when the range is exhausted

## Goal

T-0178's reviewer (three lows): ContainerRemove uses Force: true where the container never started, removing the daemon's own refusal as a backstop for a misclassified error; the guard attempt >= portAllocationRetries runs six creates while CLAUDE.md and the constant say five; when freePortExcluding exhausts 5433 to 5632 its error is returned bare and the 'port is already allocated' cause is lost. Also the older Decisions bullet in provision/CLAUDE.md still says the chosen value is passed back so question and container cannot disagree. Files: internal/discover/provision/provision.go and CLAUDE.md.

## Acceptance



## Log

- 2026-09-16 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
