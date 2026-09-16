---
id: T-0214
title: "A withheld password_command leaves a marker in lazyslice.yml so a later run without one is refused, as a withheld --where already is"
epic: E9
phase: ""
status: open
owner: sonnet
created: 2026-09-16
started: ""
closed: ""
outcome: ""
---

# T-0214 · A withheld password_command leaves a marker in lazyslice.yml so a later run without one is refused, as a withheld --where already is

## Goal

T-0192's reviewer (low): when a password_command is screened, buildConfig clears the field and the emitted file is byte-identical to a run that never had one, so a later run reading it silently proceeds with no password source; a withheld --where writes where_fingerprint and a later run is refused with config.refused.where_withheld. Mirror that: a password_command_withheld marker in the document and a refusal code for a later run that passes none. Files: internal/core/run.go, internal/emit/emit.go, internal/event/catalogue.yml, docs/ERRORS.md via make docs.

## Acceptance



## Log

- 2026-09-16 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
