---
id: T-0424
title: "A masked person_name column can be masked under the address masker despite its reason line naming person_name"
epic: E9
phase: ""
status: cancelled
owner: ""
created: 2026-09-25
started: ""
closed: 2026-10-02
outcome: cancelled
---

# T-0424 · A masked person_name column can be masked under the address masker despite its reason line naming person_name

## Goal

internal/transform's colPlan for public.customers.full_name in the shop quickstart schema (docs/QUICKSTART_TRANSCRIPT.md, README.md) carries cat=address, id=address (confirmed with a debug print at internal/transform/transform.go's maskScalar, reverted before commit) even though the classify reason line and internal/classify's printed decision both say 'name matches person_name'. The masked output is street-address-shaped ('5100 Fir Walk') rather than a Census given/surname pair, which is what mask.personNameMasker (mask/gen_text.go) always produces when invoked directly under CatPersonName/MaskerPersonName (verified with a standalone mask.Apply call, which returned 'Hudson Colon'). So the Decision.Category/Decision.Masker the transform stage actually applies to this column differs from the one classify's finalise() records and prints, somewhere between internal/classify/classify.go's decide passes and internal/classify/rulepack.go:3144's w.d.Masker = st.pack.Masker[w.d.Category] assignment (or a later overwrite this reader has not found). This is not a masking-miss (address-shaped output is still fake, not the source value or ADR-015's real-name promise), but it makes the README/QUICKSTART_TRANSCRIPT.md's own explanatory note about 'mask's own generator choice for person_name' inaccurate, since person_name's generator cannot produce an address. Needs a classify/transform maintainer to trace why Decision.Category ends up address for a column whose reason line and rule-pack match say person_name, on the shop quickstart schema (docs/QUICKSTART_TRANSCRIPT.md's schema.sql), and fix the mismatch or the reason line, whichever is wrong.

## Acceptance

—

## Log

- 2026-09-25 2026-09-25 created
- 2026-10-02 cancelled: Wrong premise, per T-0407's review: the column's values decide the masker (classify's values-win rule, README 'the values decide'), so a column whose reason line names person_name by name and whose sampled values validate as addresses is masked as an address by design; the reason line names both signals. No defect.

## Post-mortem

Cancelled. Reason: Wrong premise, per T-0407's review: the column's values decide the masker (classify's values-win rule, README 'the values decide'), so a column whose reason line names person_name by name and whose sampled values validate as addresses is masked as an address by design; the reason line names both signals. No defect.
