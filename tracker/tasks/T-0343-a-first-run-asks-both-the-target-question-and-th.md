---
id: T-0343
title: "A first run asks both the target question and the root question when both are open (ADR-016 narrows ADR-008's one-question rule)"
epic: E6
phase: 6
status: done
owner: opus
created: 2026-09-24
started: ""
closed: 2026-09-25
outcome: done
---

# T-0343 · A first run asks both the target question and the root question when both are open (ADR-016 narrows ADR-008's one-question rule)

## Goal

Decided by the maintainer 2026-09-24 (T-0331, option a). Dogfood session 3 at a terminal: the only question asked was Q1 ('no local postgres found to load into. start one? ... [Y/n]'); the root question never appeared because ADR-008's one-question rule (docs/adr/008-first-run.md, 'The one-question rule': at most one blocking question per run, the first open item on the ladder) lets discovery's Q1 or Q1' take the slot, and internal/core/root.go:101 skips Q2 when r.askedQ1 is true. Write ADR-016 (proposed): a first run at a terminal asks every open question on the ladder that has no safe default the operator has seen, which today means Q1/Q1' and Q2 both when both are open; a headless run still defaults both and asks none; the rule 'at most one blocking question' becomes 'no question whose default was already shown', and README's zero-config sentence ('A first run asks at most one blocking question') and ARCHITECTURE's copy change with it. In internal/core/root.go remove the askedQ1 gate (keep the yml-recorded root, --root, headless and --tui second-pass skips); keep Q2's '?' candidates. Tests: core's question tests for the four combinations (target open/settled x root open/settled) at a terminal and headless; the README quickstart transcript stays true (its target existed, so it asked one question). Pin that a headless run with neither settled still exits naming --create-target.

## Acceptance

—

## Log

- 2026-09-24 2026-09-24 created
- 2026-09-25 2026-09-24 2026-09-24 the ADR this task writes is ADR-017, not ADR-016: T-0327 (b092f3a) took 016 for the committed target container. The title still says 016; read it as 017.
- 2026-09-25 closed: done

## Post-mortem

went well: landed as d57c456 with no review finding above low: ADR-017 (proposed) replaces ADR-008's one-question rule with 'no question whose default was already shown', so a first run at a terminal asks Q1 or Q1' and then Q2 in ladder order, a headless run asks nothing, and a whole-Run test pins that a headless run with nothing settled still exits 4 naming --create-target, against a fake loopback Docker endpoint | went badly: Q1 answered yes at a terminal cannot be reached in a unit test, so one unexported seam (run.resolve) was added; two lows filed as a follow-up (no test drives the real Q1 prompter and the reopened tty for Q2 in sequence, which is the exact dogfood sequence; ARCHITECTURE section 9's Q4 cell still says 'outside the budget'); CONCEPT.md's zero-config sentence went to T-0384 because the file was outside the paths; the issue title still says ADR-016 | change next time: a task that writes an ADR gets CONCEPT.md in its paths when the concept states the rule being changed
