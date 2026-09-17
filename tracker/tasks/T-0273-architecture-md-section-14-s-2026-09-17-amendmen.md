---
id: T-0273
title: "ARCHITECTURE.md section 14's 2026-09-17 amendment still says the egress test 'had not shipped at all'"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-17
started: ""
closed: ""
outcome: ""
---

# T-0273 · ARCHITECTURE.md section 14's 2026-09-17 amendment still says the egress test 'had not shipped at all'

## Goal

ARCHITECTURE.md section 14's 2026-09-17 amendment (the gate-5 audit) says 'the network-namespace test (T-0270) and ADR-008's root-table question, Q2 (T-0271)' 'had not shipped at all and stay inside phase 5'. T-0270 shipped as make egress (tools/egress/run.sh) plus the blocking CI job egress in .github/workflows/ci.yml, and THREAT_MODEL.md's T4 section now names both; this file was outside T-0270's authorized paths (tools/egress/, Makefile, .github/workflows/ci.yml, THREAT_MODEL.md, CONTRIBUTING.md), so the sentence needs its own correction in ARCHITECTURE.md section 14, naming make egress and the CI job the way THREAT_MODEL.md's T4 section now does.

## Acceptance



## Log

- 2026-09-17 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
