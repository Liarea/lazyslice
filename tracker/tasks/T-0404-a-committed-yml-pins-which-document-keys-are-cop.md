---
id: T-0404
title: "A committed yml pins which document keys are copied, and a key the samples newly show counts as drift"
epic: E6
phase: 6
status: done
owner: opus
created: 2026-09-25
started: ""
closed: 2026-09-25
outcome: done
---

# T-0404 · A committed yml pins which document keys are copied, and a key the samples newly show counts as drift

## Goal

JSON red-team round 1 (docs/reviews/2026-09-25-redteam-json/round1.json entry 28): the per-leaf map is outside Classification.Fingerprint (ARCHITECTURE section 5, determinism scope), so on a re-run from a committed lazyslice.yml a key the source's documents newly show in the samples gets a fresh copy decision without a word, and --strict-schema never sees it; CONCEPT says a committed yml never excuses an unseen column. Fix: internal/emit records the sorted copied leaf keys on each document column's yml entry (leaf_keys:); on a re-run, a key the samples show that the yml does not list is masked (as a key the samples never showed is today) and reported as drift (classify.column.drift naming the column and the key), and under --strict-schema the run exits 10; a key listed in the yml keeps its decision; ARCHITECTURE section 5 and section 8 (the yml format), README's description of the yml and of drift, SECURITY item 8 and the T1 amendment follow; a yml change in a 0.x minor, named in the release notes. Paths internal/emit, internal/core, internal/classify, internal/pipeline, internal/transform, testdata/regressions, ARCHITECTURE.md, README.md, SECURITY.md, THREAT_MODEL.md, docs. Nothing may be masked less than before; make check, the core, classify and transform integration packages and make torture prove it.

## Acceptance

—

## Log

- 2026-09-25 2026-09-25 created
- 2026-09-25 closed: done

## Post-mortem

went well: landed as 5a77f1b by Opus with two reviewers after one review round: a committed yml lists each document column's copied leaf keys (leaf_keys:), a key the samples newly show is masked and reported as drift, and under --strict-schema the run exits 10; the review caught the first draft writing drifted keys back into the list (an approval the next run would trust) and writing too many keys as themselves, so a list is now rewritten unchanged and a key is fingerprinted unless it is identifier-shaped, short and one of at most 64 | went badly: a file written before this change has no list, so its first run adopts every copied key after masking and reporting it once, an exception the orchestrator accepted because it still masks more than before; a --strict-schema pipeline must run once without the flag to write the list; four lows filed as a follow-up (the key drift line reuses the column template's 'classified fresh' wording; the key fingerprint is an unsalted 64-bit sha256 a guess can confirm; the upgrade path is not in README or the release notes; no test runs a drifted key through transform); T-0419, T-0420 and T-0422 filed by the developer | change next time: when a file is rewritten in place on every run, the brief says which of its fields are approvals and which are records
