---
id: T-0398
title: "One log-shape rule: the classifier's replaced-whole verdict is the one transform and verify apply"
epic: E6
phase: 6
status: done
owner: sonnet
created: 2026-09-25
started: ""
closed: 2026-09-25
outcome: done
---

# T-0398 · One log-shape rule: the classifier's replaced-whole verdict is the one transform and verify apply

## Goal

JSON red-team round 1 (docs/reviews/2026-09-25-redteam-json/round1.json entry 25): the classifier's log-shape rule (the rule pack's regex over the normalised table name) and transform's logTableWords disagree: a CamelCase AuditLog table (Prisma's default) and an *_activity table are reported by the plan as 'replaced whole' while transform walks them leaf by leaf and copies signal-free leaves; before T-0272 the mismatch was harmless because every walked leaf was masked. Fix: a LogShaped flag on pipeline.Decision, set by internal/classify from the rule pack (inside Classification.Fingerprint), read by transform's maskDocument and by verify instead of logTableWords, which goes; a test asserting that every table the reasons line calls 'replaced whole' comes out as {} in the target, and one that a table the rule does not match is walked; ARCHITECTURE section 4 names the single rule. Paths internal/pipeline, internal/classify, internal/transform, internal/verify, testdata/regressions, ARCHITECTURE.md, docs. Nothing may be masked less than before; make check, the three integration packages and make torture prove it.

## Acceptance

—

## Log

- 2026-09-25 2026-09-25 created
- 2026-09-25 closed: done

## Post-mortem

went well: landed as 25ba8c8 after one review round: one log-shape rule, a LogShaped flag on the decision set by the classifier's rule pack inside the fingerprint and read by transform and verify, with regression 050; the reviewer caught that the normalised name alone misses an all-caps plural (EVENTs, LOGs, user_LOGs), which the deleted transform copy used to catch, and the fix ORs a plain lower-underscore fold so no name that matched before stops matching | went badly: the first developer's run stalled for two hours on a directory-access request after writing its torture log to /tmp, and was stopped and relaunched from the tree (its torture had passed: 645 s, 50 regressions); two lows filed as a follow-up (the section 4 amendment says verify had a copy it never had; no single test runs the classifier's output through transform for the replaced-whole names) | change next time: implement.js now tells developers where scratch files go and never to request directory access
