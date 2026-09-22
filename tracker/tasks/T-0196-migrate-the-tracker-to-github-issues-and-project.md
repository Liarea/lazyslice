---
id: T-0196
title: "Migrate the tracker to GitHub Issues and Projects behind the existing tools/tracker.py command surface"
epic: E5
phase: 5
status: done
owner: sonnet
created: 2026-09-15
started: ""
closed: 2026-09-22
outcome: done
---

# T-0196 · Migrate the tracker to GitHub Issues and Projects behind the existing tools/tracker.py command surface

## Goal

The maintainer 2026-09-16: use the GitHub project system instead of text files. Decided: at the gate-5 close, before v0.1.0. Keep every tracker.py subcommand (new, start, log, block, move, cancel, close --postmortem, list, validate) and reimplement over gh: epics as labels, phases as milestones (v0.1.0, v0.2.0), owner tiers as labels, a Projects v2 board with the status columns BOARD.md has, the post-mortem as the closing comment from the same template, agent filings restricted to Later and labelled filed-by-agent, and tracker/BOARD.md regenerated from GitHub on every command so the repo keeps an offline versioned snapshot. Migration script imports open tasks; closed history stays under tracker/ as an archive. Needs the project scope on the token (gh auth refresh -s project, human) and the decision whether agent-filed issues are public (default yes). Skills lazyslice-tracker and lazyslice-commit, briefs in .claude/workflows, and CONTRIBUTING.md update in the same change.

## Acceptance

—

## Log

- 2026-09-22 2026-09-22 2026-09-22 the orchestrator ran the live migrate on 4c50999 after a clean dry run: 84 open task files matched their issues, 0 created, 84 removed; this note is the smoke test of a mutating command over gh
- 2026-09-22 2026-09-22 moved to E5 phase 5
- 2026-09-22 closed: done

## Post-mortem

went well: every pre-existing subcommand kept its arguments and stdout contract while moving over gh, with show added; the migration matched all 84 open task files to the issues the one-way mirror had already created, created nothing, and removed the files in one pass after a clean dry run; 39 unit tests drive every command against a fake gh so nothing during development touched the live repository; the live smoke test (log, move, and this close) worked first time (commit 4c50999) | went badly: review found migrate deleting a file on a closed or unmatched card, and the first landing swept two pyc files into the commit because __pycache__ was not ignored; seven low findings remain, of which two matter before heavy use: gh calls have no timeout, and close closes the issue before writing the archive file | change next time: a developer brief for a tool that writes to a shared service names the failure ordering it must keep (write the durable record first, then the remote state) as an explicit rule, not something a reviewer has to catch
