---
id: T-0327
title: "A committed yml whose target is a container lazyslice created reconnects or re-provisions, and says which"
epic: E6
phase: 6
status: done
owner: opus
created: 2026-09-23
started: ""
closed: 2026-09-24
outcome: done
---

# T-0327 · A committed yml whose target is a container lazyslice created reconnects or re-provisions, and says which

## Goal

Dogfood session 2: the session-1 lazyslice.yml (target from: container lazyslice-target-<project>) copied to a new directory, run with only --source, failed at exit 4 with a raw 'password authentication failed for user "postgres" (SQLSTATE 28P01)': the yml records the container's host, port and user but the password lives only in that container's environment, and the ladder does not read it for a yml-recorded target. --create-target did not override the recorded target (same failure). On a second machine the container does not exist at all. And when --target with the password reached the gate, the refusal (right: the target held lazyslice's copy of a different source) rendered a literal '{table}' placeholder and said 'not empty' instead of why. Decide (ADR-008 territory) what a committed yml's target record means: re-discover a container by name and read its environment when lazyslice created it, re-provision when it is absent and --create-target or a terminal permits, otherwise stop naming --target and --create-target; a raw SQLSTATE is never the whole message; fill the not_empty template and say 'a copy of another source'. Pin with a yml pointing at a missing container and with a marked target of another source.

## Acceptance

—

## Log

- 2026-09-23 2026-09-23 created
- 2026-09-24 closed: done

## Post-mortem

went well: landed as b092f3a after one review round (one medium, five low): a committed yml whose target is a container lazyslice created reconnects to it or restarts it and says which, and the restart now connects to the recorded database rather than POSTGRES_DB; the stopped rule is stated in the new container-target ADR while it is still proposed | went badly: five lows filed as a follow-up (no unit test on the stopped branch, and a paused container waits out the readiness budget; two tests that do not pin the documented fallback; a doubled unreadable-parameter warning; a not_empty line that renders with an empty database name for a DSN with no path) | change next time: when two branches build the same target (reconnect and restart), the brief asks for one test per branch
