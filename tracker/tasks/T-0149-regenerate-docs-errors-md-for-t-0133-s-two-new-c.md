---
id: T-0149
title: "Regenerate docs/ERRORS.md for T-0133's two new codes"
epic: E9
phase: ""
status: done
owner: ""
created: 2026-09-14
started: 2026-09-14
closed: 2026-09-14
outcome: done
---

# T-0149 · Regenerate docs/ERRORS.md for T-0133's two new codes

## Goal

T-0133 added load.target.quarantine_dropping and run.quarantine.failed to internal/event/catalogue.yml (core closes the marker after verify and quarantines the target on a residual-class failure); docs/ERRORS.md is generated from that file by tools/docgen and docs/ was outside T-0133's paths, so 'make docs-check' (and therefore 'make check') fails until someone runs 'make docs' and commits docs/ERRORS.md. This is additional to the still-open T-0148, which named T-0130's four codes only.

## Acceptance



## Log

- 2026-09-14 created

- 2026-09-14 started

- 2026-09-14 closed: done

## Post-mortem

went well: make docs regenerated docs/ERRORS.md with the six rows T-0130 and T-0133 added; committed by the orchestrator | change next time: docs/ERRORS.md in the paths of any task that touches internal/event/catalogue.yml
