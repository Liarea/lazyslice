---
id: T-0337
title: "Move internal/verify's tests off mask.RoleWords, then delete RoleWords in the next mask minor"
epic: E9
phase: ""
status: cancelled
owner: ""
created: 2026-09-23
started: ""
closed: 2026-09-24
outcome: cancelled
---

# T-0337 · Move internal/verify's tests off mask.RoleWords, then delete RoleWords in the next mask minor

## Goal

T-0304 was asked to delete the exported mask.RoleWords, but internal/verify/explain_test.go (givenWords, lines ~32-43 and ~692) and internal/verify/verify_integration_test.go (~885) still call it, and T-0304's paths stop at mask/ and may not change an exported identifier the root uses (mask lands first as its own commit). Since T-0304 RoleWords(RoleGiven/RoleFamily) returns the Census given/surname lists and is marked Deprecated in mask/words.go. Replace those callers with a list of Census names of their own (or the words mask.Emits accepts), then delete RoleWords from mask/words.go and name the removal in the next mask minor's release notes.

## Acceptance

—

## Log

- 2026-09-23 2026-09-23 created
- 2026-09-24 cancelled: Duplicate of T-0335 (move internal/verify's tests off mask.RoleWords, then delete it), filed twice by two agents of the same run.

## Post-mortem

Cancelled. Reason: Duplicate of T-0335 (move internal/verify's tests off mask.RoleWords, then delete it), filed twice by two agents of the same run.
