---
id: T-0297
title: "A phone column's reason line says its digits parse as MAC addresses"
epic: E6
phase: 6
status: done
owner: sonnet
created: 2026-09-22
started: ""
closed: 2026-09-22
outcome: done
---

# T-0297 · A phone column's reason line says its digits parse as MAC addresses

## Goal

The first-run GIF (docs/media/first-run.gif, 2026-09-22) shows 'public.address.phone: name matches phone; 180/200 samples parse as MAC addresses': Pagila's phone values are 10 to 12 plain digits, which internal/textsig's MAC candidate accepts as bare hex. The decision (phone, masked) is right; the reason line reads as a misfire on the landing page. A MAC signal should need separators (colon, dash or dot groups) or at least one hex letter, and a column the name already classifies should not print a value signal of a different category that did not decide it. Pin it with a classify test on Pagila-shaped phone samples.

## Acceptance

—

## Log

- 2026-09-22 2026-09-22 created
- 2026-09-22 closed: done

## Post-mortem

went well: 5711c6b; Pagila's address.phone is classified phone (it was network_id: twelve plain digits parse as a bare-hex MAC, so its masked values were MAC addresses) and the reason line is clean; TestPagilaPhoneIsAPhoneNotAMAC fails without the fix | went badly: the workflow's landing 87aea49 fixed the wording by narrowing textsig.ValidMAC, which the second net and both DDL-literal passes share, and filed the coverage loss as T-0298 instead of refusing the trade; the orchestrator reverted it and fixed the decision in classify only; the brief itself invited the shared change | change next time: a brief must never point a cosmetic fix at internal/textsig or internal/verify; a change that narrows any validator is a THREAT_MODEL T1 change and needs Opus and the maintainer's eye
