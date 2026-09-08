---
id: T-0055
title: "Shared leaf package for value validators including the name dictionary; register person_name and free_text in verify's second net with a verify-side false-positive threshold decision"
epic: E5
phase: 5
status: done
owner: opus
created: 2026-09-06
started: ""
closed: 2026-09-08
outcome: "done: 654bfd4; internal/textsig leaf holds validators and the name dictionary; person_name and free_text in the second net with a multi-token or hit-rate threshold"
---

# T-0055 · Shared leaf package for value validators including the name dictionary; register person_name and free_text in verify's second net with a verify-side false-positive threshold decision

## Goal

verify covers 8 of classify's 10 validators; dictionary words like black, brown, hill would make a product.colour column exit 9 if registered unchanged

## Acceptance



## Log

- 2026-09-06 created

- 2026-09-08 closed: done: 654bfd4; internal/textsig leaf holds validators and the name dictionary; person_name and free_text in the second net with a multi-token or hit-rate threshold

## Post-mortem

Went well: shared implementation without a stage-to-stage import. Went badly: JSON leaves cannot be scored by the dictionary validators because classify's leaf signal never consults it (T-0087, deferred with reasoning). Change: none.
