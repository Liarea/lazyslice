---
id: T-0227
title: "A phone_region read from a hand-edited lazyslice.yml is validated the way the flag is"
epic: E9
phase: ""
status: open
owner: sonnet
created: 2026-09-16
started: ""
closed: ""
outcome: ""
---

# T-0227 · A phone_region read from a hand-edited lazyslice.yml is validated the way the flag is

## Goal

T-0221's reverify (low): --phone-region rejects a region libphonenumber does not know at the flag surface, but a phone_region read back from the committed yml reaches Config.PhoneRegion unvalidated, so a hand-edited file can carry a region that masks nothing. Validate on read with the same message and exit 2. Files: internal/emit/document.go, internal/core/run.go, with a test.

## Acceptance



## Log

- 2026-09-16 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
