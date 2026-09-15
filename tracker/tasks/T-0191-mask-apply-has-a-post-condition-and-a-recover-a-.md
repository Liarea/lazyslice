---
id: T-0191
title: "mask.Apply has a post-condition and a recover; a masker error message never reaches the operator with the value in it"
epic: E5
phase: 5
status: open
owner: opus
created: 2026-09-15
started: ""
closed: ""
outcome: ""
---

# T-0191 · mask.Apply has a post-condition and a recover; a masker error message never reaches the operator with the value in it

## Goal

Red team 2026-09-15 R2-11, R2-12, R2-13 (tracker T-0180, T-0181): mask.Apply returns a custom masker output unchecked, so a passthrough masker yields Masked:true and the residual filter is fed a digest for a cell never masked; mask.Apply does not recover, so a masker that panics with the value in its message crosses the module boundary; and internal/transform/codes.go interpolates a masker error string into Refusal.Error(), so the same value crosses as an error where the panic path now redacts it. Fix inside mask/ (its own module, ADR-006): a post-condition that the output differs from the input except on the two paths mask/CLAUDE.md records as allowed to return a member (a masked enum label, a collapsed special_category), returning a named error otherwise; a recover that converts a panic into an error carrying the masker id and no value; and in transform, Refusal.Error() carries the masker id and a fixed phrase, with the message available only under --debug through core.PanicSummary-style redaction. THREAT_MODEL.md T7 and T12 amendments updated from owed to landed.

## Acceptance



## Log

- 2026-09-15 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
