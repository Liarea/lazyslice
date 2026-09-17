---
id: T-0260
title: "README is true for v0.1.0: what it does, a quickstart that was run, first-run flags, exit codes at a glance, and the residuals in step with the threat model; SECURITY.md reconciled"
epic: E5
phase: 5
status: open
owner: sonnet
created: 2026-09-17
started: ""
closed: ""
outcome: ""
---

# T-0260 · README is true for v0.1.0: what it does, a quickstart that was run, first-run flags, exit codes at a glance, and the residuals in step with the threat model; SECURITY.md reconciled

## Goal

The maintainer, 2026-09-17, and docs/reviews/2026-09-09 ('a readable README is needed now, not only for launch'): README.md has a status paragraph, a false-negative quote and build commands, and nothing about using the tool. Write it for a stranger installing the first pre-release: what lazyslice does in one paragraph and what it refuses to do; install from source today and from the tap once v0.1.0 exists; a quickstart against a container that was actually run, transcript trimmed, with the target question, the masked-columns report and the green verify; the first-run flags (the generated table from the docgen task) and when to reach for each; exit codes at a glance with the link to docs/ERRORS.md; the safety model in six sentences (read-only source, owned empty target under a lease, deterministic masking with a local key never in git, verify before complete, refuse rather than guess); and 'What a snapshot will not hide' brought into step with THREAT_MODEL.md's accepted residuals as they stand after five red-team rounds (row identifiers preserved; a bare nine-digit identifier with no corroboration; names in scripts the dictionary cannot carry; a shape no validator knows in a column no rule names; the marker-bound reload window). Reconcile SECURITY.md's stated false negatives with the same list. No claim the binary does not keep: every command in the README is run and every flag checked against --help. Files: README.md, SECURITY.md, docs/ (a screenshot-free transcript only).

## Acceptance



## Log

- 2026-09-17 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
