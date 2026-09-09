---
id: T-0112
title: "Re-measure docs/TORTURE.md's flag counts and the catalogue's flags-by-kind after the credential_unique masker"
epic: E5
phase: 5
status: done
owner: ""
created: 2026-09-08
started: ""
closed: 2026-09-09
outcome: "done: 3b03050; eighteen credential opt-outs stripped, counts re-measured, make torture green after T-HARD-C"
---

# T-0112 · Re-measure docs/TORTURE.md's flag counts and the catalogue's flags-by-kind after the credential_unique masker

## Goal

T-HARD-A (T-0108) landed mask's credential_unique generator (T-0098), which is the defect docs/TORTURE.md attributes twenty of its thirty-seven --unmask flags to. The measurement and the flags themselves are in files T-HARD-A could not write:

- internal/invariants/torture_catalogue_test.go carries the twenty --unmask flags whose reason string is literally 'credential has no unique-safe masker (T-0098)' (supabase-auth 6, mastodon 2, gitlab 8, calcom 1, discourse 1, plausible/metabase/odoo the rest). Each should be removed and the run re-made; a unique credential column now escalates rather than refusing at exit 12, so the flag should no longer be demanded. Remove them one schema at a time and re-run, because a column may have been flagged for a second reason.
- internal/invariants/torture_test.go's TestTortureCatalogueMatchesTheFixtures asserts map[string]int{"--unmask": 37, "--skip-table": 7, "--key": 1}. Those numbers move with the flags above and the test's own message says to re-measure the split rather than edit the number alone.
- docs/TORTURE.md quotes the split in four places (the headline paragraph, the 'Flags' paragraph, the per-schema table's Flags column and total, and the T-0098 row of 'Found and not fixed', which should move to 'Defects found and fixed' or be struck). The T-0098 line in the two-things-the-nine-clean-runs-do-not-say paragraph goes with it.
- ROADMAP.md's gate-5 line quotes the same split (T-0106).

The re-measurement is a full 'make torture' run: twenty containers, about 2.5 GB of images, roughly an hour. It is a manual gate and no CI job runs it. Nothing is broken until it is done - a superfluous --unmask still parses and the suite still passes - but the headline number of phase 5 is stale, and the table says it is 'what will say by how much it improves when that lands'.

Evidence the fix works, without the containers: mask's TestAUniqueCredentialColumnIsCarriedRatherThanRefused (a varchar(255) unique credential column at 20,000 rows, no collisions) and internal/plan's TestUniqueCredentialColumnEscalates.

Paths this needs: internal/invariants/, docs/TORTURE.md, ROADMAP.md.

## Acceptance



## Log

- 2026-09-08 created

- 2026-09-08 Correction to this task's Goal, from the T-HARD-A review round (do not work the numbers above as written). The T-0098-tagged flag count is EIGHTEEN, not twenty. Measured in the tree at 512d712: grep -c '"--unmask"' internal/invariants/torture_catalogue_test.go is 37, and the subset whose reason names (T-0098) is 18. Per-schema, those eighteen are supabase-auth 6 (lines 145-150), calcom 1 (202), mastodon 2 (217-218), gitlab 8 (237, 239, 242-247), discourse 1 (285) - plausible, metabase and odoo carry no T-0098 flags at all, so the Goal's 'plausible/metabase/odoo the rest' is wrong as well as unnecessary. Scope also gains internal/invariants/CLAUDE.md:296-298, which repeats the same 'twenty of the thirty-seven' sentence and is not a _test.go file. Worth knowing while re-measuring: make test reports internal/invariants as '[no test files]' because the suite is behind a build tag, so no routine check will ever catch this drift - the numbers in docs/TORTURE.md, ROADMAP.md:54, internal/invariants/CLAUDE.md and torture_test.go's flags-by-kind map agree with each other and are all pre-fix, so nothing fails today. mask/CLAUDE.md:297-301 and testdata/torture/CLAUDE.md:45-50 already carry the corrected eighteen and the corrected per-schema split.

- 2026-09-08 moved to E5 phase 5

- 2026-09-09 closed: done: 3b03050; eighteen credential opt-outs stripped, counts re-measured, make torture green after T-HARD-C

## Post-mortem

Went well: the fix is now exercised on real auth schemas. Went badly: the gate was ticked while make torture still failed, caught by review. Change: a gate item is ticked only when its evidence command exits 0.
