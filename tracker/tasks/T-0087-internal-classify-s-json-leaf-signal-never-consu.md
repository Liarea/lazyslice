---
id: T-0087
title: "internal/classify's JSON leaf signal never consults the name dictionary, so verify's second net cannot score person_name or free_text over document leaves"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-08
started: ""
closed: ""
outcome: ""
---

# T-0087 · internal/classify's JSON leaf signal never consults the name dictionary, so verify's second net cannot score person_name or free_text over document leaves

## Goal

internal/classify/classify.go's jsonSignal short-circuits every json/jsonb/hstore column into jsonLeafIsPersonal, which asks the rule pack's key patterns plus email, phone, IP, IBAN and Luhn about a leaf and never reads textsig's name dictionary. internal/verify's second net therefore excludes its two dictionary-backed validators from leaves mode (applies, in internal/verify/secondnet.go, T-0055's review): scoring person_name or free_text over leaves would refuse a loaded target at exit 9 on evidence the classifier is structurally unable to have seen, with no green path short of --unmask on a column the classifier had no reason to mask. Either give jsonSignal a dictionary question of its own (LooksLikeName/Prose over string leaves, scored into CatSemiStruct as the other leaf signals are) and then remove the exclusion in verify's applies() in the same commit, or record the asymmetry as intended in internal/classify/CLAUDE.md. A document holding written person names in its leaves is unmasked by classify and unseen by verify until this is decided.

## Acceptance



## Log

- 2026-09-08 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
