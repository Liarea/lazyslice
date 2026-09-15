---
id: T-0180
title: "mask.Apply has no post-condition: a masker that returns its input is accepted"
epic: E5
phase: 5
status: done
owner: ""
created: 2026-09-15
started: 2026-09-15
closed: 2026-09-15
outcome: done
---

# T-0180 · mask.Apply has no post-condition: a masker that returns its input is accepted

## Goal

The 2026-09-15 red team registered a passthrough masker against the public mask module and mask.Apply (mask/mask.go) returned the source value with Masked:true and no error, so the residual filter is fed a digest for a cell that was never masked. The per-generator 'output never equals input' rule lives only in unit tests. Add the check to Apply itself: after m.Mask, if the output is non-empty and byte-equal to the input or its canonical form, return a refusal, the same hard stop transform already gives a masker error. mask/ is its own module and was outside T-REDFIX's paths.

## Acceptance



## Log

- 2026-09-15 created

- 2026-09-15 moved to E5 phase 5

- 2026-09-15 started

- 2026-09-15 closed: done

## Post-mortem

went well: implemented inside T-0191 (f5ec705), the mask.Apply post-condition | went badly: sat as a separate open task while its code landed elsewhere | change next time: fold a red-team follow-up into the fix task that owns the path when one is queued
