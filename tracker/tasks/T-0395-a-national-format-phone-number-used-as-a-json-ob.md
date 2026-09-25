---
id: T-0395
title: "A national-format phone number used as a JSON object key is masked under the run's --phone-region, as the second net already reads it"
epic: E9
phase: ""
status: cancelled
owner: ""
created: 2026-09-25
started: ""
closed: 2026-09-25
outcome: cancelled
---

# T-0395 · A national-format phone number used as a JSON object key is masked under the run's --phone-region, as the second net already reads it

## Goal

JSON red team round 1 (docs/reviews/2026-09-25-redteam-json/round1.json, attempts_list entry 2, attacker 1's A11, problem 2): internal/transform/json.go's keyCategory and internal/classify/validators.go's strongKeyShape read only textsig.ValidPhone (international form), while internal/verify's second net reads a document key under --phone-region too. So a jsonb key '07911 120001' survives verbatim with no region (SECURITY.md item 8's key residual) and, with --phone-region GB, the run refuses at exit 9 (verify.refused.second_net_document_masked) with --skip-table as its only escape. Fix: pass pipeline.Classification.PhoneRegion to keyCategory and to strongKeyShape and ask textsig.ValidPhoneRegion as well as ValidPhone, so transform masks the key the net would refuse and classify leaves it out of LeafKeys; mirror in internal/verify's strongKeyCategory. Prove it with a transform unit test (national key masked under GB, copied with no region), a verify test (no exit 9 on the masked key under GB) and a testdata/regressions fixture with phone-region: GB. T-0393 left this out of scope; its THREAT_MODEL.md T1 amendment names this task.

## Acceptance

—

## Log

- 2026-09-25 2026-09-25 created
- 2026-09-25 cancelled: duplicate of T-0394 (the phone region reaches document keys and leaves), filed by T-0393's developer for the second half of the same red-team attempt before that task was visible on the board

## Post-mortem

Cancelled. Reason: duplicate of T-0394 (the phone region reaches document keys and leaves), filed by T-0393's developer for the second half of the same red-team attempt before that task was visible on the board
