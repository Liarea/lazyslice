---
id: T-0287
title: "The person_name masker respects the column's role: a first-name column gets a given name, a last-name column a surname"
epic: E6
phase: 6
status: done
owner: sonnet
created: 2026-09-22
started: ""
closed: 2026-09-22
outcome: done
---

# T-0287 · The person_name masker respects the column's role: a first-name column gets a given name, a last-name column a surname

## Goal

The first-run GIF's last frame (docs/media/first-run.gif, 2026-09-22) shows public.customer.first_name masked as 'Emma Popescu' and last_name as 'Oscar Adler': the person_name masker emits a full 'Given Family' string for every column in the category, so a first-name column holds two words and a last-name column holds a given name. A stranger reading the landing page sees it in the first ten seconds. In mask/ (ADR-006's module), give the person_name masker a role read from the column's name at classify time (first/given/forename -> given name only; last/family/surname -> surname only; anything else -> full name), carried on the Decision the way the phone region is, deterministic under the same key; the same input still masks the same way for the same role. Pin it in mask/'s tests and in a regression under testdata/regressions/ with the three column names; re-record the GIF with make gif afterwards (the README's frame changes) and paste the readback rows.

## Acceptance

—

## Log

- 2026-09-22 2026-09-22 created
- 2026-09-22 closed: done

## Post-mortem

went well: cc68086 (mask half, tagged mask/v0.2.0 after green CI) and 8894b3c (tool half with go.mod on mask v0.2.0); the review caught that a synthetic role vocabulary still contained real names (34 in our own dictionary) and the fix round filtered it; make check, classify/transform/verify/plan/core integration and make torture (038, 039) green; GIF re-recorded, last frame shows Gebi | Lojuis | maya.schulz@example.org | went badly: blocked on two plan-side gaps outside its paths (FK groups and DEFAULT rewrites ignored the role, T-0293, T-0294), fixed by the orchestrator; the committed GIF had been recorded before the final fix round and showed real names the code no longer emits; the filter used a name corpus found in a third-party app cache, so it is not reproducible from a clean checkout (the in-repo test checks names.txt) | change next time: a task that adds a field to mask.Constraints must have internal/plan in its paths, since plan builds Constraints in three places; re-record media after the last fix round, not the first
