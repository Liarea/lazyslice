---
id: T-0124
title: "testdata/regressions covers plan.refused.unique_domain no longer"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-09
started: ""
closed: ""
outcome: ""
---

# T-0124 · testdata/regressions covers plan.refused.unique_domain no longer

## Goal

T-0113 (in T-HARD-C) re-cut regressions 004 and 007 from 'expect: exit 12 plan.refused.unique_domain' to 'expect: ok' plus the new unique-masked: assertion, because mask/gen_credential.go's credential_unique escalation makes both runs load rather than refuse. That is the fix working, and both files keep their schema and their classifier rule; but plan.refused.unique_domain is a real refusal with a real message and a real exit code, reachable whenever a masked unique column's category has no generator wide enough for d_required, and nothing in testdata/regressions/ exercises it any more. Owed: one reduction that still produces it -- a masked unique column in a category whose only registered masker has a narrow domain -- so the refusal's exit code, its event code and the three escapes it prints stay under test. testdata/regressions/README.md's file table and internal/invariants/torture_test.go need nothing new beyond the file itself; the header grammar already carries what it needs.

## Acceptance



## Log

- 2026-09-09 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
