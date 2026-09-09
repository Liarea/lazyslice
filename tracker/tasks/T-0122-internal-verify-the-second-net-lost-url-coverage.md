---
id: T-0122
title: "internal/verify: the second net lost URL coverage when T-0100 narrowed textsig.LooksSecret"
epic: E5
phase: 5
status: open
owner: ""
created: 2026-09-08
started: ""
closed: ""
outcome: ""
---

# T-0122 · internal/verify: the second net lost URL coverage when T-0100 narrowed textsig.LooksSecret

## Goal

T-HARD-B review, high severity. T-0100 excluded a URL from textsig.LooksSecret and added the replacement textsig.ValidURL to internal/classify only; internal/verify was outside that task's paths. verify/validators.go's second net has one entry reading LooksSecret ({CatCredential, credential}) and no online_id or URL entry at all, so a mastodon-shaped profile URI that reaches the target unmasked used to fail the run at exit 9 as a residual credential and is now seen by nothing: email, phone, ip, mac, luhn, iban, LooksSecret, NameShape, AddressShape and ProseName all return false for https://home.social.test/users/bea_donnelly1. THREAT_MODEL.md T1 lists the second net as a blocking control for the column the 200-row sample under-represented, and the two packages score independently, so classify gaining ValidURL does not compensate. Add {category: pipeline.CatOnlineID, name: online_id, text: true, ok: textsig.ValidURL} immediately ahead of the credential entry, and correct the 'all ten of internal/classify's value validators ... folded into eight entries' sentence at validators.go:68 and internal/verify/CLAUDE.md:25 -- classify has eleven now -- naming which classify validator has no counterpart here and why.

## Acceptance



## Log

- 2026-09-08 created

- 2026-09-09 moved to E5 phase 5

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
