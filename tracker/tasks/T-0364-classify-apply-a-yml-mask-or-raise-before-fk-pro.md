---
id: T-0364
title: "classify: apply a yml mask or raise before FK propagation, so a masked natural key masks its children"
epic: E6
phase: 6
status: done
owner: ""
created: 2026-09-24
started: ""
closed: 2026-09-24
outcome: done
---

# T-0364 · classify: apply a yml mask or raise before FK propagation, so a masked natural key masks its children

## Goal

internal/classify/classify.go runs keyChildren and foreignKeys before applyPrior, so a --mask or yml raise on a natural key never propagates to its FK children; internal/core/mask.go (checkMasks, T-0319 review) refuses such a run naming each child, but a key-family child markNeverMasked exempted (a uuid natural key's child) then has no --mask that clears it. Apply the prior's raises before keyChildren/foreignKeys (or rerun both after applyPrior) and drop the child check from core once it holds.

## Acceptance

—

## Log

- 2026-09-24 2026-09-24 created
- 2026-09-24 2026-09-24 moved to E6 phase 6
- 2026-09-24 closed: done

## Post-mortem

went well: landed as c40546a after one review round (one medium, three low), Opus developer: a --mask or a yml mask: on a natural key now reaches every FK child through classify's second propagation sweep after applyPrior, so parent and child share one masker and the join survives; the reviewer caught the first round removing core's child refusal on the strength of the common case, and it came back as a fail-closed backstop with a test that fails without it | went badly: the developer did not run make torture as the acceptance asked (the reviewer ran it: pass, 171 s; torture prints no flag count); three lows filed as a follow-up (a propagated child with a second, unmasked FK parent breaks that edge at load, exit 8; the byte-identical claim in CLAUDE.md is not strict for reason strings in a cycle; no end-to-end fixture with a --mask on a natural key) | change next time: before removing a fail-closed check, list every branch of the replacing code that leaves the guarded state unchanged, and keep the check unless each is shown impossible
