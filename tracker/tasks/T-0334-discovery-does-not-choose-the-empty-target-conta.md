---
id: T-0334
title: "Discovery does not choose the empty target container lazyslice itself created, and offers to start a duplicate on the next port"
epic: E6
phase: 6
status: done
owner: opus
created: 2026-09-23
started: ""
closed: 2026-09-24
outcome: done
---

# T-0334 · Discovery does not choose the empty target container lazyslice itself created, and offers to start a duplicate on the next port

## Goal

Dogfood session 3 (2026-09-23, the maintainer at a terminal; reproduced headless by the orchestrator with --plan): a second run from the same directory, with the container the first run created still running (labels lazyslice.project and lazyslice.working_dir set, 0 tables), printed 'container lazyslice-target-<project>: postgres@127.0.0.1:5433/postgres -- postgres 16, 0 table(s), probably empty' and on the next line 'no local postgres found to load into. start one? postgres:16 as lazyslice-target-<project> on port 5434 [Y/n]'. Headless the same run exits 4 target.refused.none. Y then produced no second container (the name exists) and the run went on with the existing one, so the question was both wrong and harmless, but a stranger reads it as 'my container was not found' and headless CI cannot get past it without --create-target every time. The ladder (ADR-008 rungs, internal/discover) must choose a running container carrying this project's lazyslice.project label when it is empty or carries lazyslice's own marker for this source, without a question; 'probably empty' (0 tables in the connected database) is enough for a container lazyslice created, and if the qualifier means another database in the cluster could hold data, say that and check it rather than refuse. Never propose a container name that already exists. Pin with a discover test (running labelled empty container -> chosen, no question; stopped labelled container -> Q1' as today) and a core integration test that runs twice from one directory with no flags but --source.

## Acceptance

—

## Log

- 2026-09-23 2026-09-23 created
- 2026-09-24 closed: done

## Post-mortem

went well: landed as 800e31b after one review round (one medium, four low): discovery ranks the running container this directory's first run created instead of asking to start a duplicate on the next port, and the fix round made 'ours' mean both labels, project and working directory, so a same-basename checkout's container is refused at exit 4 rather than adopted | went badly: the first landing trusted the project label alone although the working_dir label exists to tell directories apart; four lows filed as a follow-up (reuseOwn and the ranking define 'ours' differently; found.own lifts the whole maintenance-database rule rather than only postgres; the own container is rankable, not preferred over a name-pattern candidate; the second-run integration test's row check is vacuous) | change next time: when a check decides something belongs to this directory, read what each label encodes first and test a same-basename second checkout from the start
