---
id: T-0196
title: "Migrate the tracker to GitHub Issues and Projects behind the existing tools/tracker.py command surface"
epic: E9
phase: ""
status: open
owner: sonnet
created: 2026-09-15
started: ""
closed: ""
outcome: ""
---

# T-0196 · Migrate the tracker to GitHub Issues and Projects behind the existing tools/tracker.py command surface

## Goal

Gareth 2026-09-16: use the GitHub project system instead of text files. Decided: at the gate-5 close, before v0.1.0. Keep every tracker.py subcommand (new, start, log, block, move, cancel, close --postmortem, list, validate) and reimplement over gh: epics as labels, phases as milestones (v0.1.0, v0.2.0), owner tiers as labels, a Projects v2 board with the status columns BOARD.md has, the post-mortem as the closing comment from the same template, agent filings restricted to Later and labelled filed-by-agent, and tracker/BOARD.md regenerated from GitHub on every command so the repo keeps an offline versioned snapshot. Migration script imports open tasks; closed history stays under tracker/ as an archive. Needs the project scope on the token (gh auth refresh -s project, human) and the decision whether agent-filed issues are public (default yes). Skills lazyslice-tracker and lazyslice-commit, briefs in .claude/workflows, and CONTRIBUTING.md update in the same change.

## Acceptance



## Log

- 2026-09-15 created

- 2026-09-15 2026-09-16 Gareth: project scope granted; internal AI tasks are shown publicly. Orchestrator created one GitHub Project (lazyslice) with fields Epic, Owner, Type, Tracker id and Status columns Backlog/Ready/In progress/Blocked/Done/Cancelled; repo labels epic:*, owner:*, type:*, area:*, filed-by-agent; milestones v0.1.0, v0.2.0, v1.0.0, Later. Views (Board by Status, Table, Roadmap by milestone) are added in the UI. The wrapper maps: tracker status open->Ready or Backlog (E9), blocked->Blocked, done->Done, cancelled->Cancelled; epic->Epic field and epic label; phase->milestone; owner->Owner; post-mortem->closing comment.

- 2026-09-15 2026-09-16 Gareth: project scope granted; internal AI tasks are shown publicly. Orchestrator created GitHub Project 3 (lazyslice, linked to the repo) with fields Epic, Owner, Kind (Type is a reserved name), Tracker id, and Status columns Backlog/Ready/In progress/Blocked/Done/Cancelled; repo labels epic:*, owner:*, type:*, area:*, filed-by-agent; milestones v0.1.0, v0.2.0, v1.0.0, Later. Views (Board by Status, Table, Roadmap by milestone) are added in the UI. Wrapper mapping: tracker open -> Ready (or Backlog for E9), blocked -> Blocked, done -> Done, cancelled -> Cancelled; epic -> Epic field and epic label; phase -> milestone; owner -> Owner field and label; post-mortem -> closing comment.

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
