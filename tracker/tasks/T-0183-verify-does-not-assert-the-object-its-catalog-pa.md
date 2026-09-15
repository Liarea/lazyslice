---
id: T-0183
title: "verify does not assert the object its catalog pass refused is gone after the quarantine"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-15
started: ""
closed: ""
outcome: ""
---

# T-0183 · verify does not assert the object its catalog pass refused is gone after the quarantine

## Goal

load.DropLoaded now drops the types and sequences the run created as well as its tables (T-REDFIX, the 2026-09-15 red team's A07), so THREAT_MODEL.md T8's 'the target ends the run either empty or holding nothing this run wrote' is true again for the object classes the loader creates. What is still missing is the assertion: internal/verify refuses at exit 9 naming a domain or an enum, core quarantines, and nothing re-reads the target to confirm the named object is gone. Add that check before the run returns, and have the refusal say so when it is not.

## Acceptance



## Log

- 2026-09-15 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
