---
id: T-0100
title: "textsig.LooksSecret classifies a URL as a credential"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-08
started: ""
closed: ""
outcome: ""
---

# T-0100 · textsig.LooksSecret classifies a URL as a credential

## Goal

LooksSecret matches any 16-to-512-character string with two character classes and Shannon entropy at or above 3.2 and no space or @. A URL clears all of it: mastodon's accounts.uri (https://home.social.test/users/bea_donnelly1) and statuses.uri are both decided credential, which masks them to the fixed literal - safe, but wrong, and under a unique index it becomes a plan refusal the operator has to --unmask (two of mastodon's five flags in the torture catalogue). ARCHITECTURE.md 5 already describes a URL output space ('URLs under example.invalid with a hash-derived path') and no validator produces the category that would use it. Owed: decide whether a URL-shaped value is online_id (its masker's domain is large, so a unique URL column would need no flag) or a category of its own, add the validator to internal/classify's ordered list ahead of the secrets one, and give mask a generator that emits a URL shape. Not fixed in T-TORTURE because excluding URLs from LooksSecret without adding the validator would drop accounts.uri to 'none' and copy a username-bearing URL verbatim, which is THREAT_MODEL.md T1's direction.

## Acceptance



## Log

- 2026-09-08 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
