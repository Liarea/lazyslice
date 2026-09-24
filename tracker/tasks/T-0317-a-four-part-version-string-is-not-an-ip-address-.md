---
id: T-0317
title: "A four-part version string is not an IP address, and a digit-only license key is not a phone number"
epic: E6
phase: 6
status: done
owner: sonnet
created: 2026-09-23
started: ""
closed: 2026-09-24
outcome: done
---

# T-0317 · A four-part version string is not an IP address, and a digit-only license key is not a phone number

## Goal

Dogfood session 1: a last_player_version column (93/168 samples 'parse as IP addresses': 1.2.3.4-shaped version strings) was masked as free_text, and a license_key column of ten digit samples (8/10 'parse as phone numbers under a guessed region') was masked as phone, the wrong shape for a key. A column whose name carries version, build or release vetoes the network_id value signal; the guessed-region phone corroboration should not decide on a column whose name says key, code, license, serial or token. Pin both.

## Acceptance

—

## Log

- 2026-09-23 2026-09-23 created
- 2026-09-24 2026-09-24 Also seen in the orchestrator's TUI evaluation on Pagila (2026-09-24): public.staff.password prints '2/2 samples parse as MAC addresses': a hex password digest of the right length reads as a bare-hex MAC (T-0297 fixed the digit-only case under a phone name; this is the hex-letters case under a credential name). The decision is right (credential by name); the reason line should not name MAC addresses for a column the name already decides, or for a fixed-length hex digest (T-0315's shape).
- 2026-09-24 closed: done

## Post-mortem

went well: the classify fix (a version column is not an IP address, a digit-only license key is not a phone number) took one review round, and its twin in the second net, the network_id validator vetoed by a version, build or release token in the column name, landed by hand as T-0360 in the same commit 44412d1, CI green on main | went badly: the task blocked in the workflow because the reviewer's medium finding lay in internal/verify, outside its paths, the same pattern as T-0314; the hand landing cost the orchestrator a full gate run, and the close itself was then refused by GitHub's secondary rate limit and redone a batch later | change next time: a classify task whose finding is a validator's recall gets internal/verify/validators.go in its paths from the start, with the brief naming the second-net twin
