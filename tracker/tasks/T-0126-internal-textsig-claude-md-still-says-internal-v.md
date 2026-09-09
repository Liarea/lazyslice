---
id: T-0126
title: "internal/textsig/CLAUDE.md still says internal/verify has no URL entry (T-0122 has landed)"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-09
started: ""
closed: ""
outcome: ""
---

# T-0126 · internal/textsig/CLAUDE.md still says internal/verify has no URL entry (T-0122 has landed)

## Goal

T-HARD-C (T-0122) added the online_id entry -- category pipeline.CatOnlineID, text, ok textsig.ValidURL -- to internal/verify/validators.go immediately ahead of the credential entry, with TestAProfileURLFailsTheSecondNetAsAnOnlineID holding it and internal/verify/CLAUDE.md's count corrected to eleven validators in nine entries. internal/textsig/CLAUDE.md was outside that task's paths and its paragraph '**internal/verify is the second consumer and it has not been given the other half -- T-0122**' now describes a hole that is closed: it says verify's second net 'reads LooksSecret and has no online_id or URL entry anywhere in it' and that this package's narrowing 'is a narrowing of a blocking control until T-0122 lands'. Owed: rewrite that paragraph to say both consumers have the URL answer now and keep the rule it exists to teach -- a validator narrowed in textsig is narrowed for both nets and both need an answer in the same change -- with T-0122 as the worked example of what happens when they do not.

## Acceptance



## Log

- 2026-09-09 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
