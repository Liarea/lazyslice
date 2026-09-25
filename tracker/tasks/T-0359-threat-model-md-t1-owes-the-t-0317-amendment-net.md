---
id: T-0359
title: "THREAT_MODEL.md T1 owes the T-0317 amendment (network_id/version veto, phone-guess key/code/license/serial/token veto)"
epic: E6
phase: 6
status: done
owner: ""
created: 2026-09-24
started: ""
closed: 2026-09-25
outcome: done
---

# T-0359 · THREAT_MODEL.md T1 owes the T-0317 amendment (network_id/version veto, phone-guess key/code/license/serial/token veto)

## Goal

THREAT_MODEL.md T1 (repo root, outside internal/classify's paths) does not yet record the T-0317 change: internal/classify no longer offers the network_id validators to a column named for its own version/build/release, and no longer offers the guessed-region phone corroboration to a column named key/code/license/serial/token. ARCHITECTURE.md section 4's T-0317 amendment (2026-09-24) has the full account and the measurement to summarise; add the matching T1 amendment the way T-0311/T-0313/T-0315/T-0316's amendments already do.

## Acceptance

—

## Log

- 2026-09-24 2026-09-24 created
- 2026-09-24 2026-09-24 moved to E6 phase 6
- 2026-09-25 closed: done

## Post-mortem

went well: landed as 65c81fb after two review rounds: THREAT_MODEL's T1 carries the T-0317 amendment (the network_id version veto and the phone-guess key/code/license/serial/token veto), says its residuals widen README and SECURITY's residual 4 as T-0316's did for cards, and no longer claims verify lacks the veto, since T-0360 landed it | went badly: two rounds for a one-paragraph docs task, because the first cut misstated where the residuals live and the verify state; the README and SECURITY amendment itself went to T-0381 rather than landing here although both files were in the task's paths; one low, that no corpus-wide before/after measurement was taken for T-0317 unlike T-0311 and T-0316, is folded into T-0381 | change next time: a docs task that adds a residual to THREAT_MODEL names the README and SECURITY sentences it changes in the same landing
