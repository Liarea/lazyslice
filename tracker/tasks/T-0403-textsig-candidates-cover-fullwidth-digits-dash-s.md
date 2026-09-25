---
id: T-0403
title: "textsig candidates cover fullwidth digits, dash-separated groups, mailto and tel schemes, percent-encoding and base64"
epic: E6
phase: 6
status: done
owner: opus
created: 2026-09-25
started: ""
closed: 2026-09-25
outcome: done
---

# T-0403 · textsig candidates cover fullwidth digits, dash-separated groups, mailto and tel schemes, percent-encoding and base64

## Goal

JSON red-team round 1 (docs/reviews/2026-09-25-redteam-json/round1.json entries 16 and 21): under neutral keys, an email written with a fullwidth at sign and fullwidth digits, a card number with en-dash or underscore separators, a trailing-dot domain, a zero-width space inside a value, backslash-u escapes, and mailto:, percent-encoded and base64 spellings of an email were copied; the 2026-09-15 A2 amendment promises de-obfuscation and T1's residual names unknown shapes, not known shapes in another code point or a reversible encoding. Fix in internal/textsig Candidates: an NFKC-normalised candidate (fullwidth at, plus and digits to ASCII); zero-width characters stripped; Unicode dash punctuation, underscore and slash treated as group separators before collapseGroups; a candidate with a leading mailto:, tel: or sms: scheme stripped; a percent-decoded candidate when the value holds %XX; a base64-decoded candidate when the decode is printable UTF-8 of a plausible length (bounded so it cannot be abused for cost). Both nets and leafValueCategory pick the candidates up; TestLeafValueCategoryIsPinned stays in step; regression fixtures for each spelling; state the covered spellings in the A2 amendment. Paths internal/textsig, internal/classify, internal/verify, internal/transform, testdata/regressions, THREAT_MODEL.md, docs. Only masks more; make check, the three integration packages and make torture prove it.

## Acceptance

—

## Log

- 2026-09-25 2026-09-25 created
- 2026-09-25 closed: done

## Post-mortem

went well: landed as 4ccfba6 by Opus after two review rounds: textsig now offers each value once in a decoded spelling (NFKC, format characters removed, Unicode dashes, underscores and slashes as group separators outside calendar dates, mailto:, tel: and sms: stripped, percent and backslash-u escapes undone, a trailing dot dropped, base64 decoded when the result is printable), which both nets and the leaf reader pick up through anyCandidate; regression 053 leaked nine columns before the fix; make torture's flag count stayed at 28 and no emitted yml decision moved | went badly: the last round was spent on a policy question: importing unicode/norm makes go.mod list x/text as a direct dependency, which the acceptance line forbade; the orchestrator accepted it (same module, same version, already in the graph through mask) and updated ARCHITECTURE section 13 and the import-graph lines, closing T-0418; two lows filed as a follow-up (non-ASCII decimal digits, the ideographic full stop and the middle dot are not folded and not named as uncovered; every value now runs the separator and base64 checks seven to ten times per net) | change next time: the acceptance line allows a go mod tidy that only moves a module between the direct and indirect blocks, and a finding marked for the orchestrator goes to the orchestrator, not a developer round
