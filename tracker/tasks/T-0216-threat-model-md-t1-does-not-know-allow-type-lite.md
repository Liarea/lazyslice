---
id: T-0216
title: "THREAT_MODEL.md T1 does not know --allow-type-literal is recorded in the yml"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-16
started: ""
closed: ""
outcome: ""
---

# T-0216 · THREAT_MODEL.md T1 does not know --allow-type-literal is recorded in the yml

## Goal

T1's A4b row (THREAT_MODEL.md line 46) still describes --allow-type-literal as a per-run flag whose typed reason is the only record; T-0186 gave it a persistent types: block in lazyslice.yml, honoured by a run that passes no flag, with fingerprint expiry (line 88) that is currently stated for columns only. Update T1/A4b and line 88 to name the yml route and its expiry, per ARCHITECTURE.md section 11.1's paired-amendment convention.

## Acceptance



## Log

- 2026-09-16 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
