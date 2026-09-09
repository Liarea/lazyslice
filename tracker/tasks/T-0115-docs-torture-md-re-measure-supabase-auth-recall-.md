---
id: T-0115
title: "docs/TORTURE.md: re-measure supabase-auth recall after T-0104's name rules"
epic: E5
phase: 5
status: done
owner: ""
created: 2026-09-08
started: ""
closed: 2026-09-09
outcome: "done: in T-HARD-C (4f9a186)"
---

# T-0115 · docs/TORTURE.md: re-measure supabase-auth recall after T-0104's name rules

## Goal

T-HARD-B widened internal/classify/rules.yml's credential and online_id patterns with the auth spellings T-0104 listed (auth_code, otp_code, code_hash, authorization_code, code_verifier, credential_id, public_key, external_id, provider_id, extern_uid). Nine of the ten columns docs/TORTURE.md records as supabase-auth's misses at recall 0.800 are now masked; internal/classify/supabase_misses_test.go has been flipped to assert that, and internal/classify/names_test.go scores them in the held-out matrix (precision 0.958, recall 0.979, n=64). docs/TORTURE.md still prints 0.800 and still lists the ten as missed. Owed: re-run the labelled truth set and correct the number and the list. docs/ was outside T-HARD-B's paths.

## Acceptance



## Log

- 2026-09-08 created

- 2026-09-09 moved to E5 phase 5

- 2026-09-09 closed: done: in T-HARD-C (4f9a186)

## Post-mortem

Went well: landed. Went badly: nothing. Change: none.
