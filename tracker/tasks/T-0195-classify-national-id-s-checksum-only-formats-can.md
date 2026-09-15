---
id: T-0195
title: "classify: national_id's checksum-only formats can weak-ratio-mask an ordinary numeric business key"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-15
started: ""
closed: ""
outcome: ""
---

# T-0195 · classify: national_id's checksum-only formats can weak-ratio-mask an ordinary numeric business key

## Goal

T-0187 review round (2026-09-15, finding 2, consequence 1): internal/classify/classify.go:326's national_id entry calls textsig.ValidNationalID (the twelve-format union) uniformly across every accepted family, with no text/digits split; PESEL/BSN/SIN/TFN/Aadhaar clear 9-25% of random digit runs of the right length, so a numeric business-key column (the reviewed probe: public.probe_orders.order_no, 6/10 samples) can cross classify's weakThreshold (0.5) on that alone and get raised to possible/masked by the neighbouring-column rule, overwriting an ordinary bigint order number with a fake national-id-shaped value. Splitting the entry into a strong/structured half and a non-strong/checksum-only half (as this review round did in internal/verify/validators.go, mirroring T-0136's Luhn split) does not fix this: classify.go's sig.weak assignment (classify.go's decide, ~line 625) is unconditional on the strong flag, so a checksum-only ratio at or above weakThreshold still sets sig.weak regardless of which entry carries it. A real fix needs either a per-validator type-family gate classify does not have today (the way rules.yml's accepts: gates a name hit, not a value hit) or a materially higher within-column threshold scoped to this one entry, and either is a recall-affecting change to classify's scoring that internal/classify/CLAUDE.md's own rule says needs a T1 review and a TestPagilaPrecisionAndRecall measurement, not a quiet threshold edit bundled into an unrelated task.

## Acceptance



## Log

- 2026-09-15 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
