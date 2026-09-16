---
id: T-0212
title: "renderSafe redacts by default, and prints a transform refusal's reason under the values flag"
epic: E5
phase: 5
status: open
owner: ""
created: 2026-09-15
started: ""
closed: ""
outcome: ""
---

# T-0212 · renderSafe redacts by default, and prints a transform refusal's reason under the values flag

## Goal

T-0191 made internal/transform's Refusal.Error() withhold the masker's own message and exposed it as ReasonMessage() for the one caller allowed to print it, but cmd/lazyslice/main.go is outside that task's paths, so nothing calls ReasonMessage yet and a reason is now unreachable even under --show-row-values-in-errors. Round 2 R2-13(b) also asks for the wider change in cmd/lazyslice/main.go renderSafe: make redaction the default rather than an allowlist of two error types — errors known to be value-free (core.Stop with a catalogue code, errUsage, pipeline.ErrNotImplemented) print in full, everything else prints its type plus the --show-row-values-in-errors hint — so a new error type in the tree is not value-bearing until someone notices. Match *transform.Refusal structurally there the way PanicValue() already is.

## Acceptance



## Log

- 2026-09-15 created

- 2026-09-16 moved to E5 phase 5

- 2026-09-16 re-homed to E5 by the orchestrator, 2026-09-16: round 3 names renderSafe's two-type allowlist as the CLI half still owed after T-0191; redact by default, allowlist what may print.

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
