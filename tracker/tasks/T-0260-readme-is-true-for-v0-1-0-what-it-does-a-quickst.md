---
id: T-0260
title: "README is true for v0.1.0: what it does, a quickstart that was run, first-run flags, exit codes at a glance, and the residuals in step with the threat model; SECURITY.md reconciled"
epic: E5
phase: 5
status: done
owner: sonnet
created: 2026-09-17
started: ""
closed: 2026-09-17
outcome: done
---

# T-0260 · README is true for v0.1.0: what it does, a quickstart that was run, first-run flags, exit codes at a glance, and the residuals in step with the threat model; SECURITY.md reconciled

## Goal

The maintainer, 2026-09-17, and docs/reviews/2026-09-09 ('a readable README is needed now, not only for launch'): README.md has a status paragraph, a false-negative quote and build commands, and nothing about using the tool. Write it for a stranger installing the first pre-release: what lazyslice does in one paragraph and what it refuses to do; install from source today and from the tap once v0.1.0 exists; a quickstart against a container that was actually run, transcript trimmed, with the target question, the masked-columns report and the green verify; the first-run flags (the generated table from the docgen task) and when to reach for each; exit codes at a glance with the link to docs/ERRORS.md; the safety model in six sentences (read-only source, owned empty target under a lease, deterministic masking with a local key never in git, verify before complete, refuse rather than guess); and 'What a snapshot will not hide' brought into step with THREAT_MODEL.md's accepted residuals as they stand after five red-team rounds (row identifiers preserved; a bare nine-digit identifier with no corroboration; names in scripts the dictionary cannot carry; a shape no validator knows in a column no rule names; the marker-bound reload window). Reconcile SECURITY.md's stated false negatives with the same list. No claim the binary does not keep: every command in the README is run and every flag checked against --help. Files: README.md, SECURITY.md, docs/ (a screenshot-free transcript only).

## Acceptance



## Log

- 2026-09-17 created

- 2026-09-17 closed: done

## Post-mortem

went well: README went from 55 stale lines to a front page that says what the tool does, shows a quickstart that was actually run against two containers, carries the generated first-run flags, the exit codes and the five accepted residuals in step with THREAT_MODEL.md; review caught two psql tables that had been retyped rather than pasted, and the fix round re-ran the quickstart and pasted real output | went badly: the quickstart called a bare lazyslice the install section never put on PATH, the marker query had no ordering for a table that keeps one row per run, and SECURITY.md still said doctor prints the limitations, which no code does; all three found at the orchestrator's end-to-end read, which is what that read is for | change next time: a README task's proof includes following it top to bottom in a clean shell (commit ffd65b1; T-0265 and T-0268 filed)
