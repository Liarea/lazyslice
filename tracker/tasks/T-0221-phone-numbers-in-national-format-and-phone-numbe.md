---
id: T-0221
title: "Phone numbers in national format, and phone numbers spelled out in words, are recognised: a configured phone region, and corroboration when none is configured"
epic: E5
phase: 5
status: done
owner: sonnet
created: 2026-09-16
started: 2026-09-16
closed: 2026-09-16
outcome: done
---

# T-0221 · Phone numbers in national format, and phone numbers spelled out in words, are recognised: a configured phone region, and corroboration when none is configured

## Goal

Round-3 replay (docs/reviews/2026-09-15-redteam/round3-still-leaking.json, the A2 residual and its plain variant): a correctly formatted domestic number such as '020 7946 0958' or '07911 123456' in a column called kontaktnr crosses verbatim under exit 0 because internal/textsig/textsig.go sets PhoneRegionHint to ZZ, so only a +E.164 number parses; a number dictated in words crosses for the same reason once candidates.go's spelledDigits has turned it into digits, and Dict.Prose requires a name word so the free_text rule never fires either. Decision (orchestrator, 2026-09-16), on T-0187's precedent: add --phone-region REGION (yml phone_region, shown in the reasons output as the region assumed); the row path parses a candidate under the configured region, and with none configured under a short built-in list of regions but only with corroboration (the column's name matches the phone rule, or a proven personal neighbour); verify's second net keeps its strong footing on +E.164 and on the configured region only, never the guessed list, so a ten-digit account column cannot refuse a loaded run. Regressions: the plain national number, the dictated number, and an account-number column that must pass. Files: internal/textsig, internal/classify, internal/verify/validators.go, cmd/lazyslice, internal/pipeline config and emit, docs via make docs, THREAT_MODEL.md T1.

## Acceptance



## Log

- 2026-09-16 created

- 2026-09-16 started

- 2026-09-16 closed: done

## Post-mortem

went well: the corroboration gate transferred from T-0187's national-id precedent to the phone signal, and running the ten-schema torture suite early caught a collision where the guessed-region pass masked an integer column before the national-id check saw it; the Opus reviewer caught an unvalidated --phone-region silently masking nothing and a configured region switching off the fallback for every other country, both fixed with a usage error and a kept fallback; two fix rounds, verify green with torture | went badly: the workflow's commit step was blocked by the safety classifier for the second time, the run went on to the next task on a dirty tree, and the computer restarted mid-way, so the orchestrator separated the two tasks' hunks in the shared docs by hand and committed the verified state as a0cda99; the fifteen-region guess list has no cited source; the name-rule corroboration arm was unreachable and was removed | change next time: the commit step writes outside .git and stages by path, and a task whose commit did not happen is blocked, not merged (landed in the same session); a brief that adds a region list names its source
