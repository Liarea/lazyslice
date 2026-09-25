---
id: T-0394
title: "The phone region reaches document keys and leaves, and a key whose sampled leaves are phone numbers masks them"
epic: E6
phase: 6
status: done
owner: opus
created: 2026-09-25
started: ""
closed: 2026-09-25
outcome: done
---

# T-0394 · The phone region reaches document keys and leaves, and a key whose sampled leaves are phone numbers masks them

## Goal

JSON red-team round 1 (docs/reviews/2026-09-25-redteam-json/round1.json entries 2 and 26): keyCategory in internal/transform/json.go and strongKeyShape in internal/classify/validators.go read only textsig.ValidPhone, the international-only check, while the second net reads keys and leaves under --phone-region, so a run with --phone-region GB over a document whose keys are national-format phone numbers fails closed at exit 9 (verify.refused.second_net_document_masked) with --skip-table as the only remedy, and without a region a national-format key survives verbatim; and a national-format phone under a leaf key the name rules miss, with no --phone-region, is copied while the same value in a scalar column of the same name is masked on a guessed-region hit. Fix: pass pipeline.Classification.PhoneRegion (T-0272) into keyCategory and strongKeyShape so keys use ValidPhoneRegion; give jsonKeyCategories a value half: when one key's string leaves across the samples parse as phone numbers under the guessed-region list at the ratio and corroboration the scalar path uses (a personal key or neighbour in the same document or table), that key gets CatPhone so transform masks every leaf under it and the net's skip stays sound. Tests with and without --phone-region GB over the round's by_phone and rosters shapes; a regression fixture. Paths internal/transform, internal/classify, internal/verify, testdata/regressions, THREAT_MODEL.md, SECURITY.md, docs. Nothing may be masked less than before; make check, the three integration packages and make torture prove it.

## Acceptance

—

## Log

- 2026-09-25 2026-09-25 created
- 2026-09-25 closed: done

## Post-mortem

went well: landed as 0a24223 by Opus with no fix round: keys read the run's --phone-region in transform and classify, so a national-format phone key is masked to an international number the net's key skip passes; the leaf map gained a value half, so a key whose sampled leaves clear the guessed-region ratio beside a personal neighbour or key becomes phone and every leaf under it is masked; fixtures 047 (no region) and 048 (GB) fail on the old tree | went badly: a national-format phone key with no region still survives, filed as T-0401 and stated in SECURITY item 8 and T1 (the guessed-region key needs a Decision field outside the paths); the phone key masker's 44,700-value domain collides at a few hundred keys per document (T-0400); four lows filed as a follow-up (fixture 048 cannot see a copied leaf under the national key; the value half skips the T-0317 name veto, so a license_key leaf key can become phone, which only masks more; guessedPhoneRatio's cost on wide documents; the collision refusal's wording) | change next time: a fixture whose documents also carry a key the old code already masked cannot prove the new key is masked; the brief names the one thing the fixture must be able to see
