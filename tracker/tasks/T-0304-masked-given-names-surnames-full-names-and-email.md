---
id: T-0304
title: "Masked given names, surnames, full names and email local parts draw from the Census lists (ADR-015, mask half; mask/v0.3.0)"
epic: E6
phase: 6
status: done
owner: opus
created: 2026-09-23
started: ""
closed: 2026-09-23
outcome: done
---

# T-0304 · Masked given names, surnames, full names and email local parts draw from the Census lists (ADR-015, mask half; mask/v0.3.0)

## Goal

Waits on the verify half of ADR-015 (the residual scan's coincidence rule) and on tools/names (mask/words_corpus.go). Then: person_name has one vocabulary. RoleGiven draws from censusGivenWords, RoleFamily from censusSurnameWords, RoleFull's pair and single-given forms from the same two lists; givenNames and surnames (which the email masker's local parts read) become the Census lists too, so the module holds one pair of name lists and mask.Emits has one rule (the maintainer's call; if they keep emails byte-identical, leave givenWords/surnameWords for the email masker only and say so). Delete roleGivenWords, roleFamilyWords, roleGivenNames, roleFamilyNames and the exported RoleWords (an API removal in the same 0.x minor, named in the release notes); retire mask/role_test.go's exclusion tests and internal/classify's TestRoleWordsExcludeCurrentNameDictionary with a comment naming ADR-015, and replace them with: every list word is in mask.Emits for its role in three spellings; Apply over every list word for all three roles never fold-equals its input; no fold-duplicates within a list; small_domain does not fire for a 200-row name sample (d = list size >= 400). Domain() reports the new counts; the unique-index rule is unchanged in effect (a unique single-name column was refused at plan before and still is). mask/CLAUDE.md: replace the 'byte-for-byte v0.1.0' sentences with the v0.3.0 change and the 0.x reading of 'a major version of the module' (before mask/v1.0.0 it is the next minor, tagged and named in the notes); add a corpus section pointing at THIRD_PARTY_NOTICES.md. Rewrite testdata/regressions/039's header (its seeded real names are now the ordinary case; it proves the verify rule end to end) and add 041: a generated full_name over masked name columns, expect ok. Goldens updated. Release-note bullets: every masked person name (and email local part, if moved) changes in this release; every existing target prints the 'lazyslice version changed' line. Re-record docs/media/first-run.gif with make gif and paste the readback rows; README.md and SECURITY.md where they describe masked names. The orchestrator tags mask/v0.3.0, bumps go.mod and cuts v0.3.0; do not edit go.mod. Then T-0292 closes on this fixture.

## Acceptance

Pagila's first run exits 0 with explained counts above zero for public.customer.first_name and last_name; 039 and 041 pass; I2 passes on Pagila; make check and make torture clean.

## Log

- 2026-09-23 2026-09-22 created
- 2026-09-23 closed: done

## Post-mortem

went well: 7c52c6c merged through implement.js with one fix round; the reviewer caught a regression in the first cut (a varchar(2) name column had stopped being refused) and the fix put Domain and Mask behind one chooser with ADR-015's 400-name floor, so narrow columns are refused rather than masked over a tiny domain; one vocabulary for every role and the email local parts, the synthetic lists gone | went badly: the brief asked for RoleWords to be deleted while also forbidding a change to any export the root uses, and internal/verify's tests use it; the developer kept it deprecated and repointed, which the orchestrator accepts, and the follow-up was filed twice (T-0335 kept, T-0337 cancelled); the 039/040/041 regression headers claim an explained count the harness does not assert | change next time: a brief must not both delete an export and forbid changing exports; say which wins, and name the callers
