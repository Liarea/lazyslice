---
id: T-0075
title: "CI red from provisioning: password-file mode assertion on Windows; provisioned container not ready within 60 s on GitHub runners (image pull inside the deadline)"
epic: E5
phase: 5
status: done
owner: opus
created: 2026-09-08
started: 2026-09-08
closed: 2026-09-08
outcome: "done: merged in the backlog run wf_e6bd43ea-99d (see git log for the hash)"
---

# T-0075 · CI red from provisioning: password-file mode assertion on Windows; provisioned container not ready within 60 s on GitHub runners (image pull inside the deadline)

## Goal

Run 34207236918: test (windows-latest) TestProvisionCreatesTheContainerSection9Describes 'password file is -rw-rw-rw-, want 0o600'; integration (14, 18) TestAStoppedContainerIsOfferedAndStarted, TestASecondRunOpensTheTargetItProvisioned, TestProvisionCreatesAndReusesARealContainer, TestARecreatedContainerCanStillLogIntoItsSurvivingVolume: 'did not accept a connection within 1m0s'

## Acceptance



## Log

- 2026-09-08 created

- 2026-09-08 started

- 2026-09-08 closed: done: merged in the backlog run wf_e6bd43ea-99d (see git log for the hash)

## Post-mortem

Went well: merged through the review loop. Went badly: see the run's journal for the per-task lows carried into CLAUDE.md notes. Change: none.
