---
id: T-0311
title: "The neighbouring-column sweep spares enum-like, identifier-shaped and unique-indexed columns"
epic: E6
phase: 6
status: done
owner: opus
created: 2026-09-23
started: ""
closed: 2026-09-24
outcome: done
---

# T-0311 · The neighbouring-column sweep spares enum-like, identifier-shaped and unique-indexed columns

## Goal

Dogfood session 1 (2026-09-23, a production Rails schema of 143 tables): a table with one certain column (an email, an IP-address column, a search-token column) had every signal-less character column swept into free_text: a users table's role and ui_mode, a devices table's os_type, state, log_level, timezone, screen orientation, serial number and a text uuid, an ads table's colours and paths. The copy holds 255-character word salad where the app expects 'admin' or 'linux', so it does not boot. The sweep (ARCHITECTURE section 4, the certain-neighbour rule; README 'How it decides') should skip a column whose samples are enum-like (few distinct values, each repeated; say the threshold), whose values are identifier-shaped (uuid, hex digest, path, hostname, semantic version), or which is under a unique index; each skip prints its reason. This narrows recall on a real shape, so it is a THREAT_MODEL T1 change: measure on the torture corpus and the supabase-auth truth set, and say in the reason line why the column was spared. Pin with a two-table fixture: an email column beside role, state and uuid columns.

## Acceptance

—

## Log

- 2026-09-23 2026-09-23 created
- 2026-09-24 closed: done

## Post-mortem

went well: ec339f6 through implement.js, two Opus reviewers, two fix rounds: the certain-neighbour sweep now spares enum-like, identifier-shaped and unique-indexed columns, each with its reason; the reviewer's ZIP+4 and local-phone probe led to a digit-run guard and a control that fails with the old guard restored; T1 amendment, ARCHITECTURE section 4, README and the package CLAUDE.md say the rule | went badly: the first control columns were named after what they held (zip4, phone_local) and tripped the name rules, costing a round; the hex-digest spare covers 8-to-31-character hex values textsig.LooksSecret does not claim, so a short API key or reset token beside a certain column is now copied, which is a recall loss the review called low and the orchestrator files as a phase-6 task | change next time: a spare's controls get neutral names, and every new spare states its own residual in THREAT_MODEL before it lands
