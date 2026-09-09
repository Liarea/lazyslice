---
id: T-0114
title: "mask/CLAUDE.md and gen_credential.go still say the torture counts are un-re-measured"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-08
started: ""
closed: ""
outcome: ""
---

# T-0114 · mask/CLAUDE.md and gen_credential.go still say the torture counts are un-re-measured

## Goal

T-0112 has now stripped the eighteen (T-0098) --unmask flags and re-run make torture, so two notes in mask/ — a directory T-0112 could not write — are stale and say the opposite of what is true:

- mask/gen_credential.go, the package comment above CredentialUniquePrefix: 'That eighteen counts the flags still standing in the catalogue; it is not a re-run measurement... Until tracker T-0112 does that, the thirty-seven/seven/one split in docs/TORTURE.md and the flags-by-kind assertion in internal/invariants/torture_test.go both still quote the pre-fix numbers, and credential_unique has no end-to-end torture coverage — only the unit tests in this package.'
- mask/CLAUDE.md, the 'A unique credential column escalates rather than being refused' bullet: 'docs/TORTURE.md's counts are stale until somebody re-runs make torture — the eighteen flags, the thirty-seven/seven/one split, the per-schema table...'

What is true now: the split is 19 --unmask / 7 --skip-table / 1 --key over twenty-seven flags; internal/invariants/torture_test.go asserts that; docs/TORTURE.md, ROADMAP.md:54, internal/invariants/CLAUDE.md and testdata/torture/CLAUDE.md all carry it. credential_unique does have end-to-end torture coverage, and it is worth stating precisely rather than generously: of the eighteen columns, four hold masked values in a target (auth.refresh_tokens.token 136/136, auth.users.confirmation_token 100/100, auth.users.recovery_token 100/100, public.user_security_keys.credential_id 100/100, every one prefixed lazyslice-invalid- with distinct count equal to row count), two are the empty string mask.Apply passes through, ten are NULL in their fixture, and mastodon's two are in the run that refuses at exit 13 before the plan. docs/TORTURE.md's 'Flags' section carries that breakdown.

Paths: mask/ only. Edit is two comment blocks; no code change.

Filed by T-0112, which could not write mask/.

## Acceptance



## Log

- 2026-09-08 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
