---
id: T-0212
title: "renderSafe redacts by default, and prints a transform refusal's reason under the values flag"
epic: E5
phase: 5
status: done
owner: ""
created: 2026-09-15
started: 2026-09-17
closed: 2026-09-17
outcome: done
---

# T-0212 · renderSafe redacts by default, and prints a transform refusal's reason under the values flag

## Goal

T-0191 made internal/transform's Refusal.Error() withhold the masker's own message and exposed it as ReasonMessage() for the one caller allowed to print it, but cmd/lazyslice/main.go is outside that task's paths, so nothing calls ReasonMessage yet and a reason is now unreachable even under --show-row-values-in-errors. Round 2 R2-13(b) also asks for the wider change in cmd/lazyslice/main.go renderSafe: make redaction the default rather than an allowlist of two error types — errors known to be value-free (core.Stop with a catalogue code, errUsage, pipeline.ErrNotImplemented) print in full, everything else prints its type plus the --show-row-values-in-errors hint — so a new error type in the tree is not value-bearing until someone notices. Match *transform.Refusal structurally there the way PanicValue() already is.

## Acceptance



## Log

- 2026-09-15 created

- 2026-09-16 moved to E5 phase 5

- 2026-09-16 re-homed to E5 by the orchestrator, 2026-09-16: round 3 names renderSafe's two-type allowlist as the CLI half still owed after T-0191; redact by default, allowlist what may print.

- 2026-09-17 started

- 2026-09-17 closed: done

## Post-mortem

went well: the inversion was well scoped by the round-3 finding and the transform package's own forward reference; the Opus reviewer caught that the core.Stop allowlist entry defeated the inversion because the catch-all path copied an unknown error's whole message into the Stop, and the fixer fixed the root in core rather than gating in the CLI; the second reverify asked for a shape assertion, added by the orchestrator, and the gate ran green with cmd, core and transform integration | went badly: two fix rounds and a by-hand finish for a one-line assertion; the canary end-to-end test could not be extended because no real stage returns an unclaimed error today; two lows filed (T-0237) | change next time: a reverify that finds only a missing assertion states the exact assertion so the fix round is one edit
