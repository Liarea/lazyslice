---
id: T-0320
title: "A green run prints one verify summary line and how to reach the target"
epic: E6
phase: 6
status: done
owner: sonnet
created: 2026-09-23
started: ""
closed: 2026-09-24
outcome: done
---

# T-0320 · A green run prints one verify summary line and how to reach the target

## Goal

Dogfood session 1: a successful run prints the load counts and then 'wrote ./lazyslice.yml (commit it for CI)'; nothing says that foreign keys validated, N row counts matched, the residual scan found nothing, or the second net scanned M columns, and nothing says how to connect to the target lazyslice --create-target provisioned (the password lives in the container's environment; psql fails until docker inspect). Print one verify line (checks and counts) and one target line (host, port, database, user, and where the password is; never the password) at the end of every green run; the README transcript follows.

## Acceptance

—

## Log

- 2026-09-23 2026-09-23 created
- 2026-09-24 closed: done

## Post-mortem

went well: landed as 4689821 after one review round (two medium, three low): a green run ends with one verify summary line and a target line the operator can paste; the fix round made the docker-exec hint key on a real container id (and found that containers.go never captured the id for the common running-container rung) and made the summary match checks by code rather than by name | went badly: the developer's first cut read the display label as a container name, which for a compose service is the service name; two lows are filed as a follow-up (the password-source hint is chosen from flag presence rather than from where the password came from; the summary line claims to fold every section 6 check while omitting four, and a QUICKSTART_TRANSCRIPT paragraph is garbled) | change next time: a brief that adds a command the operator will paste says which value is validated and which is display text
