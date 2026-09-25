---
id: T-0326
title: "A stray positional argument is a usage error, not 'the source did not report a server version'"
epic: E6
phase: 6
status: done
owner: sonnet
created: 2026-09-23
started: ""
closed: 2026-09-24
outcome: done
---

# T-0326 · A stray positional argument is a usage error, not 'the source did not report a server version'

## Goal

Dogfood session 1: lazyslice --source DSN --create-target ' --unmask x' (one unsplit shell variable) exited 4 with 'no local postgres found to load into: pass --target' and 'the source did not report a server version, so postgres:<major> cannot be chosen', without touching the source. Cobra accepts positional arguments the command never reads; the root command should take none (cobra.NoArgs) and exit 2 naming the argument. Pin with cmd/lazyslice's flag tests.

## Acceptance

—

## Log

- 2026-09-23 2026-09-23 created
- 2026-09-24 closed: done

## Post-mortem

went well: landed as 4ba7027 by a second implement.js run with no review finding above low; the first run's two fix rounds each narrowed the leak by heuristic (parsed DSNs redacted, then a scheme or an equals sign plus an at sign), and the orchestrator's decision replaced the heuristic with a rule (never quote an argument holding =, @, : or /, because no connection-string form carries a password without one), which the second run applied as one shared helper on the root and every subcommand, with the docs (ARCHITECTURE sections 8 and 11, core.go comments, docgen fixtures) in the same landing, closing T-0368 | went badly: two fix rounds were spent on a heuristic a reviewer refuted each time, because the goal said 'exit 2 naming the argument' and the first developer read that as echoing it; three lows filed as a follow-up (a TestExitCodes row that never reaches noArgs, the subcommand never-quote case unpinned, stale positional-DSN comments in discover and core) | change next time: a brief whose message repeats user input says what may never be repeated (anything that can carry a credential) instead of leaving it to review
