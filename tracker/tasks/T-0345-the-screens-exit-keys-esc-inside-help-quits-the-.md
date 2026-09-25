---
id: T-0345
title: "The screens' exit keys: esc inside help quits the program, enter runs unconfirmed, ctrl+c exits 0"
epic: E6
phase: 6
status: done
owner: sonnet
created: 2026-09-24
started: ""
closed: 2026-09-25
outcome: done
---

# T-0345 · The screens' exit keys: esc inside help quits the program, enter runs unconfirmed, ctrl+c exits 0

## Goal

Orchestrator's pseudo-terminal evaluation of --tui (2026-09-24, docs/DOGFOOD_LOG.md session 3): (1) esc while the help overlay is open quits the whole program instead of closing help, because Cancel is matched before Help in internal/tui/model.go's pressed() (model.go:208); (2) enter is 'leave and run' with no confirmation, so a stranger pressing enter to open a row starts the real run that drops and rewrites the target (model.go:211-213, cmd/lazyslice/main.go:746): enter must first show what will happen ('run: drop and rewrite <target>, N tables, M rows; the flags these screens set: ...') and ask once, or use a key nobody presses by habit; (3) ctrl+c is bound to Cancel and exits 0 rather than 130, the documented interrupt code (internal/tui/keys.go:134); (4) quitting prints no closing line: say 'stopped before writing anything; nothing changed in <target>' (and drop the truncated plan echo, or make it the full line-printer text). Pin each with the tui tests; these land whatever T-03xx's screen decision is.

## Acceptance

—

## Log

- 2026-09-24 2026-09-24 created
- 2026-09-25 closed: done

## Post-mortem

went well: landed as 792ea21 after one review round (two medium, three low): esc inside help closes help, enter first shows what the run will do and asks once, ctrl+c leaves as an interrupt at exit 130, and quitting prints a closing line; the fix round kept a plan-only tui run from claiming a drop-and-rewrite and made ctrl+c inside the reason prompt an interrupt | went badly: three lows filed as a follow-up (the closing-line test never sets a target; nothing pins that an interrupted leave maps to exit 130 at the command; esc and ctrl+c compare the raw Mod field where every other key goes through key.Matches) | change next time: a brief that names an exit code names the test layer that pins it, the command and not the model
