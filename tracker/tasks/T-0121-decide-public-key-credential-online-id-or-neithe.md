---
id: T-0121
title: "Decide public_key: credential, online_id, or neither"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-08
started: ""
closed: ""
outcome: ""
---

# T-0121 · Decide public_key: credential, online_id, or neither

## Goal

T-0104 named it one of two columns that deserve a decision rather than a pattern, and T-HARD-B's review found the decision had been taken inside a fix task and taken in the credential direction while every comment written to support it made the online_id argument. It is now out of internal/classify/rules.yml again and the column is copied (supabase_misses_test.go pins it, names_test.go labels both public_key columns not-personal, which is where the label has always been). Settle it: a public key is published by design (mastodon's actor document, every WebAuthn relying party) so the credential fixed literal at priority 80 masks a value that is not secret and outranks everything else, but it is also a stable identifier for exactly one person. Whichever way it goes it moves the hand labels in internal/classify/names_test.go, so the rule and the label must be moved by someone other than the rule's author, with the precision/recall movement recorded.

## Acceptance



## Log

- 2026-09-08 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
