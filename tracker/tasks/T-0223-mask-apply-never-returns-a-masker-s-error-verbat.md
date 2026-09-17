---
id: T-0223
title: "mask.Apply never returns a masker's error verbatim: the module wraps it without the value"
epic: E5
phase: 5
status: done
owner: sonnet
created: 2026-09-16
started: 2026-09-17
closed: 2026-09-17
outcome: done
---

# T-0223 · mask.Apply never returns a masker's error verbatim: the module wraps it without the value

## Goal

Round-3 replay (R2-13, the module half): maskCell passes a third-party masker's error straight out of the public module, and mask/ is importable on its own under ADR-006, so its contract cannot rely on the binary's redaction; 'cannot mask "victim.canary@bigcorp.example": unsupported shape' reaches any caller of Apply. Wrap it the way the panic already is: a sentinel error naming the masker id, the category and the error's type, the original not kept. A test registers a masker whose error quotes its input and asserts the returned error does not contain it. Files: mask/mask.go, mask/apply_guards_test.go, mask/CLAUDE.md.

## Acceptance



## Log

- 2026-09-16 created

- 2026-09-17 started

- 2026-09-17 closed: done

## Post-mortem

went well: the wrap mirrored the panic path exactly, with the red team's own victim string as the test; the Opus reviewer caught that the unconditional wrap swallowed the module's own sentinels, ErrNoRoom foremost, which would have broken the most common run-time refusal's exit reason; one fix round | went badly: the developer filed T-0229 with a goal that described the wrong bug and the fixer had to file T-0230 to correct it because the tracker cannot edit a goal; the test asserted the canary's absence but not that the original error is dropped | change next time: the tracker gains a way to correct a goal, or a developer's filing is reviewed in the same round
