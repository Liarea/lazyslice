---
id: T-0402
title: "Document numbers are decoded exactly, so a card number stored as a json number keeps its digits"
epic: E6
phase: 6
status: done
owner: sonnet
created: 2026-09-25
started: ""
closed: 2026-09-25
outcome: done
---

# T-0402 · Document numbers are decoded exactly, so a card number stored as a json number keeps its digits

## Goal

JSON red-team round 1 (docs/reviews/2026-09-25-redteam-json/round1.json entry 14): a 19-digit Luhn-valid card number stored as a json number leaf reaches the target, because pgx decodes jsonb numbers to float64 and the digits past 2 to the 53rd are rounded before any validator runs, in transform, in classify's decodeSampleDocument and in verify's leaf reader. Fix: decode json and jsonb from the raw text with json.Decoder.UseNumber in all three places so validators see the exact digits and the existing json.Number path applies; the masked output for a numeric card leaf keeps the leaf numeric where the masker's output is digits; regression with a 19-digit and a 16-digit PAN leaf and an integer phone leaf. Paths internal/transform, internal/classify, internal/verify, internal/pg (only if the sample reader must hand back raw text), testdata/regressions, docs. Nothing may be masked less than before; make check, the three integration packages and make torture prove it.

## Acceptance

—

## Log

- 2026-09-25 2026-09-25 created
- 2026-09-25 closed: done

## Post-mortem

went well: landed as b021eab after one review round: json and jsonb are read as raw text from the source and decoded with UseNumber in transform, classify and verify, so a 19-digit card number stored as a json number keeps its digits; regression 052 reconstructs Go's float64 spelling in SQL and fails with the source fix reverted; the reviewer caught that json[] and jsonb[] columns took the raw-text path they cannot parse, and the fix answers false for arrays | went badly: the array carrier for number-leaf precision on the extract side is T-0416; three lows filed as a follow-up (classify's jsonLeaves still decodes with plain Unmarshal, so jsonSignal sees rounded numbers; jsonTextRows.Scan overwrites the caller's variadic slice in place, latent today; an unmasked json column now copies the source text byte for byte, so a duplicate key's first value reaches the target unexamined) | change next time: a leaf-level assertion in a regression fixture should come from a harness header, not a per-fixture SQL reconstruction; T-0416's landing is the place to add one
