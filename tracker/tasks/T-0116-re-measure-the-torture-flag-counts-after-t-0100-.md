---
id: T-0116
title: "Re-measure the torture flag counts after T-0100 and T-0104"
epic: E5
phase: 5
status: done
owner: ""
created: 2026-09-08
started: ""
closed: 2026-09-09
outcome: "done: in T-HARD-C (4f9a186)"
---

# T-0116 · Re-measure the torture flag counts after T-0100 and T-0104

## Goal

T-HARD-B changed two things the torture catalogue's flags were measured against. T-0100: textsig.LooksSecret no longer reads a URL as a credential and internal/classify has a ValidURL validator ahead of the secrets one, so mastodon's accounts.uri and statuses.uri are online_id and not credential -- internal/invariants/torture_catalogue_test.go's two --unmask flags for them are now about a different category, and the session_id one is unchanged. T-0104 widened the credential and online_id name rules, which may add masked columns to supabase-auth and gitlab and so may add or remove refusals. Owed: re-run 'make torture', re-count the twenty-seven flags in ROADMAP.md's gate-5 line and docs/TORTURE.md, and correct the comments in torture_catalogue_test.go. internal/invariants/, docs/ and ROADMAP.md were outside T-HARD-B's paths.

## Acceptance



## Log

- 2026-09-08 created

- 2026-09-09 moved to E5 phase 5

- 2026-09-09 closed: done: in T-HARD-C (4f9a186)

## Post-mortem

Went well: landed. Went badly: nothing. Change: none.
