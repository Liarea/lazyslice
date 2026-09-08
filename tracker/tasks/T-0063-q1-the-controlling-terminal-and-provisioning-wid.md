---
id: T-0063
title: "Q1, the controlling terminal, and provisioning: widen pipeline.Provisioner to carry the generated POSTGRES_PASSWORD, then wire --create-target and the one blocking question"
epic: E5
phase: ""
status: open
owner: opus
created: 2026-09-07
started: ""
closed: ""
outcome: ""
---

# T-0063 · Q1, the controlling terminal, and provisioning: widen pipeline.Provisioner to carry the generated POSTGRES_PASSWORD, then wire --create-target and the one blocking question

## Goal

ARCHITECTURE.md section 14 puts rung 4 and provisioning in phase 5, so T-DISCOVER shipped rungs 0 to 3 and made the no-target state a refusal (exit 4 naming --target, or target.refused.docker_not_local for a non-local Docker endpoint). ADR-008 section 6 makes Q1 fire only where a container can be created, so Options carries no Provisioner, no Prompter and no Yes today, and ask.go / tty_unix.go / tty_windows.go were removed rather than left as an unreachable seam. pipeline.Provisioner has to widen first: it returns a Candidate whose Ref holds no password, while ARCHITECTURE.md section 9 'Provisioning' gives the container a random POSTGRES_PASSWORD that the run then has to connect with.

## Acceptance

pipeline.Provisioner returns the created endpoint including its generated credential (and THREAT_MODEL.md A3's handling of it is stated). --create-target creates a container behind provision/ only. Q1 is asked once, only with a controlling terminal, only where a container can be created, and --yes answers it. Rung 4 stops being a count-and-say.

## Log

- 2026-09-07 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
