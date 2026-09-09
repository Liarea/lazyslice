---
id: T-0111
title: "ARCHITECTURE.md owes three sentences after T-HARD-A: 5's determinism scope, 5's credential line, 11.1's raise site"
epic: E9
phase: ""
status: done
owner: ""
created: 2026-09-08
started: ""
closed: 2026-09-08
outcome: "done: §5 and §2 carry the credential_unique escalation and the post-plan fingerprint sentences"
---

# T-0111 · ARCHITECTURE.md owes three sentences after T-HARD-A: 5's determinism scope, 5's credential line, 11.1's raise site

## Goal

T-HARD-A (T-0108) changed three behaviours ARCHITECTURE.md states, and the file was outside its paths.

(1) 5 'Determinism scope' says: "lazyslice.yml and lazyslice_meta carry sha256(K)[:8] and Classification.Fingerprint; a marked target whose latest classification_fingerprint or tool_version differs from the run's prints ...". That sentence, together with 2's declaration of Classification.Fingerprint as 'sha256 over (rule-pack version, and per column: category, masker)[:16]', now under-describes what is hashed: internal/plan writes the escalated masker onto Decision.Masker after Classify returns, so internal/core recomputes the fingerprint after the plan (classify.Refingerprint, core.refingerprint, T-0101). The exact edit: 2's comment on Classification.Fingerprint gains 'computed after the plan's unique-index picks, not inside Classify', and 5's determinism-scope paragraph gains a sentence saying the fingerprint covers the classification plus the plan's generator picks, so a column that becomes unique between two runs is announced.

(2) 5 says 'Credentials become a fixed unusable value ($lazyslice$invalid).' mask now registers a second CatCredential generator, credential_unique, which a column under a unique index escalates to: 'lazyslice-invalid-' plus 13 base32 symbols of h, domain 2^65, length-fitted to the column (T-0098). 5's unique-index bullet lists email/phone/network_id/url escalations and should list credential too.

(3) 11.1's 'A refusal (exit 13, target.schema.not_recreatable) is raised at plan' is now true of ddl.Recreatable as well as of the FK case: internal/core's planStage calls it before planRequest and before Plan (T-0097). No wording change is required there, but 12/14 and any text that says the check lives in load.Load should be checked.

Paths this needs: ARCHITECTURE.md. mask/CLAUDE.md, internal/core/CLAUDE.md, internal/load/CLAUDE.md and internal/event/catalogue.yml already record all three at their own sites.

## Acceptance



## Log

- 2026-09-08 created

- 2026-09-08 Addition to item (1) of this task's Goal, from the T-HARD-A review round. Item (1) stops at 'covers the classification plus the plan's generator picks' and omits the consequence that makes the change worth documenting: because the value is computed after the plan, it is a function of PLAN INPUTS too, not of the classification alone. A different root, --take, --depth or --skip-table changes the planned row count, which changes what d_required escalates, which moves the fingerprint - so 'classification changed - masked values will differ' can print for a table whose rows are not in this target at all. That is the conservative direction, but it is why the line can fire without the rule pack or the schema having moved. Both ARCHITECTURE.md sites named in item (1) need that sentence: 5's determinism-scope paragraph and 2's comment on Classification.Fingerprint. internal/pipeline/classify.go's comment on the Fingerprint field, and internal/core/CLAUDE.md's 'A consequence worth knowing before reading a warning' paragraph, already carry the exact wording to copy - those two are the in-paths sites T-HARD-A could write, and ARCHITECTURE.md is now the only place in the tree that contradicts the code.

- 2026-09-08 closed: done: §5 and §2 carry the credential_unique escalation and the post-plan fingerprint sentences

## Post-mortem

Went well: doc. Went badly: nothing. Change: none.
