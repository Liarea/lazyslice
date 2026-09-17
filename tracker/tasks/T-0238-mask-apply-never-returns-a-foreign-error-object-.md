---
id: T-0238
title: "mask.Apply never returns a foreign error object: a masker's error is rebuilt by the module with only its identity carried across"
epic: E5
phase: 5
status: open
owner: sonnet
created: 2026-09-17
started: ""
closed: ""
outcome: ""
---

# T-0238 · mask.Apply never returns a foreign error object: a masker's error is rebuilt by the module with only its identity carried across

## Goal

Round-4 replay (docs/reviews/2026-09-15-redteam/round4-still-leaking.json, the variant against T-0223): maskCell passes a masker's error through unwrapped when isModuleError says it wraps one of the module's sentinels, so fmt.Errorf("cannot fit %q: %w", value, mask.ErrNoRoom) carries the value out of the public module through the escape hatch the fix added. Always construct the module's own error: %w on the matched module sentinel (ErrMaskerFailed when none), the category, the masker id and the error's type, and a module-built NoRoomError or DomainError rebuilt from the masker's value-free fields, so errors.Is and errors.As still answer while no foreign text survives. Test: a masker wrapping ErrNoRoom around a canary; assert errors.Is true and the canary absent. Files: mask/mask.go, mask/apply_guards_test.go, mask/CLAUDE.md; internal/transform's maskReason comment names this attack and stays.

## Acceptance



## Log

- 2026-09-17 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
