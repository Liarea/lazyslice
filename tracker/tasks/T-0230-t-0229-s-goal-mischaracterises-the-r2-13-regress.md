---
id: T-0230
title: "T-0229's goal mischaracterises the R2-13 regression; correct it before acting on it"
epic: E9
phase: ""
status: done
owner: ""
created: 2026-09-16
started: 2026-09-17
closed: 2026-09-17
outcome: done
---

# T-0230 · T-0229's goal mischaracterises the R2-13 regression; correct it before acting on it

## Goal

T-0229 (internal/transform/codes.go: add mask.ErrMaskerFailed to maskReason's known-sentinel list) says ErrMaskerFailed 'falls through to reasonSummary's generic fallback, which is safe but less specific' and prescribes adding it to maskReason's sentinel slice. That is not the bug the T-0223 round-3 fix introduced: the actual regression (fixed in round 4, R3-1) was that mask.maskCell wrapped every error Mask returned, including this module's own documented sentinels (ErrNoRoom foremost, the most common run-time mask failure) and typed errors (*NoRoomError, *DomainError), so errors.Is(err, mask.ErrNoRoom) on an Apply error went from true to false and internal/transform/codes.go's maskReason() (which matches ErrNoRoom by errors.Is to print its safe words) silently fell back to the generic 'an error of type %T', degrading the most common column-too-short refusal's diagnosability. mask/mask.go now has an isModuleError helper so maskCell returns the module's own sentinels/typed errors unwrapped and only wraps a *generator's* free-form residue in ErrMaskerFailed (see mask/CLAUDE.md's T-0223 bullet, updated in the same change). T-0229's goal as written, if actioned, would add ErrMaskerFailed's own text to maskReason's known list without addressing why ErrNoRoom stopped being reachable, and would not by itself have fixed the regression. Rewrite T-0229's goal via tools/tracker.py (never by hand) to: (1) note the mask/ half is already fixed (isModuleError, T-0223 round-4 replay R3-1), and (2) keep only the legitimate remaining ask — internal/transform/codes.go's maskReason() still does not name ErrMaskerFailed itself, so a *third-party* masker's own error (as opposed to mask/'s own sentinels, which now pass through fine) still gets the generic 'an error of type %T' text instead of 'the masker returned an error'; deciding whether that's worth a dedicated case is the actual open task.

## Acceptance



## Log

- 2026-09-16 created

- 2026-09-17 started

- 2026-09-17 closed: done

## Post-mortem

went well: the correction was acted on by the orchestrator; T-0229 keeps only the specificity improvement | went badly: a filing about a filing | change next time: a tracker edit command
