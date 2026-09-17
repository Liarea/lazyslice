---
id: T-0238
title: "mask.Apply never returns a foreign error object: a masker's error is rebuilt by the module with only its identity carried across"
epic: E5
phase: 5
status: done
owner: sonnet
created: 2026-09-17
started: 2026-09-17
closed: 2026-09-17
outcome: done
---

# T-0238 · mask.Apply never returns a foreign error object: a masker's error is rebuilt by the module with only its identity carried across

## Goal

Round-4 replay (docs/reviews/2026-09-15-redteam/round4-still-leaking.json, the variant against T-0223): maskCell passes a masker's error through unwrapped when isModuleError says it wraps one of the module's sentinels, so fmt.Errorf("cannot fit %q: %w", value, mask.ErrNoRoom) carries the value out of the public module through the escape hatch the fix added. Always construct the module's own error: %w on the matched module sentinel (ErrMaskerFailed when none), the category, the masker id and the error's type, and a module-built NoRoomError or DomainError rebuilt from the masker's value-free fields, so errors.Is and errors.As still answer while no foreign text survives. Test: a masker wrapping ErrNoRoom around a canary; assert errors.Is true and the canary absent. Files: mask/mask.go, mask/apply_guards_test.go, mask/CLAUDE.md; internal/transform's maskReason comment names this attack and stays.

## Acceptance



## Log

- 2026-09-17 created

- 2026-09-17 started

- 2026-09-17 closed: done

## Post-mortem

went well: the module now rebuilds every masker error itself; the Opus reviewer caught, twice, that the rebuilt DomainError still trusted the masker, first by copying its count fields and then by recomputing the domain through Admissible, which re-enters the masker's own Domain method on the error path; the final rule is that no masker method runs on the error path, and the test drives a hostile registered masker through the public Apply | went badly: two fix rounds because the first fix restated a derivation as safe without reading what it called; a test name now states the opposite of the contract (filed) | change next time: a fix that cites another function as evidence of safety reads that function first, and says in the comment what it calls
