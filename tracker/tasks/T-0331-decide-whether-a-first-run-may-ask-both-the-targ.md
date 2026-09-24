---
id: T-0331
title: "Decide whether a first run may ask both the target question and the root question"
epic: E6
phase: 6
status: done
owner: human
created: 2026-09-23
started: ""
closed: 2026-09-24
outcome: done
---

# T-0331 · Decide whether a first run may ask both the target question and the root question

## Goal

Dogfood session 3 (2026-09-23, the maintainer at a terminal, brew v0.2.0, a fresh directory): the only question asked was Q1, 'no local postgres found to load into. start one? postgres:16 as lazyslice-target-<project> on port 5433 [Y/n]'. The root question never appeared and nothing could be answered with '?': ADR-008's one-question rule (docs/adr/008-first-run.md, 'The one-question rule': at most one blocking question per run, the first open item on the ladder source, target, root table, row count, masking) let Q1 take the slot, and internal/core/root.go:101 skips Q2 when discovery asked Q1 or Q1' (r.askedQ1). So on the very first run against a new machine, the case ADR-008 was written for, the operator never sees the one decision that shapes the slice (users versus clients on this schema); the default is printed afterwards as 'root public.users (18 inbound - 0 outbound FKs) -- --root', which a stranger does not read as a choice. Options: (a) ask both when both are open, on the ground that a first run needing a target container is exactly when the root also needs choosing and two questions is not zero-config's enemy (a headless run still defaults both); (b) make Q1 non-blocking when its default is Y (print 'starting postgres:16 as ...' and go on) so Q2 stays the one question; (c) keep the rule and print the root line as a question-shaped confirmation the operator can interrupt. The maintainer decides; the change is an ADR-008 amendment (a new ADR that narrows the one-question rule) plus internal/core/root.go and README's zero-config sentence. ADR-002's reversal condition about '?' is not triggered: there was no prompt to press it at.

## Acceptance

—

## Log

- 2026-09-23 2026-09-23 created
- 2026-09-24 closed: done

## Post-mortem

went well: decided by the maintainer 2026-09-24: option (a), a first run asks both the target question and the root question when both are open; dogfood session 3 showed the one-question rule letting Q1 swallow Q2 on the very first run against a new machine | went badly: nothing; the finding needed a terminal session to surface, which the two headless sessions could not | change next time: the implementation is its own task (ADR-016 narrowing ADR-008's one-question rule)
