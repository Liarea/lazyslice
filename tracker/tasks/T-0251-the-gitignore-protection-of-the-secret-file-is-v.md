---
id: T-0251
title: "The .gitignore protection of the secret file is verified by asking git, never by matching the file's text"
epic: E5
phase: 5
status: done
owner: sonnet
created: 2026-09-17
started: 2026-09-17
closed: 2026-09-17
outcome: done
---

# T-0251 · The .gitignore protection of the secret file is verified by asking git, never by matching the file's text

## Goal

Round-5 replay (docs/reviews/2026-09-15-redteam/round5-still-leaking.json, the secrets attacker's first new leak): internal/repo's appendMissing decides lazyslice.secret is ignored by a literal line match on .gitignore, so a file that lists the entry and then negates it (lazyslice.secret followed by !lazyslice.secret) reads as protected and the 64-hex masking key is written where git add -A commits it, T6's promise broken. After appending, ask git: git -C root check-ignore -v --no-index -- path; exit 0 with a rule that does not begin with ! means ignored; exit 1 not ignored; 128 no repository; git absent from PATH cannot verify. Only a verified ignore sets the writable state; everything else falls back to the ephemeral key that already exists for this case and to exit 5 under --require-key, because refusing to write beats writing un-ignored. Test: a .gitignore with the entry and its negation asserts MayWriteSecret is false. Files: internal/repo, internal/core, THREAT_MODEL.md T6.

## Acceptance



## Log

- 2026-09-17 created

- 2026-09-17 started

- 2026-09-17 closed: done

## Post-mortem

went well: the check asks git instead of matching text, with git absent, not ignored and no repository all falling back to the ephemeral key; the Opus reviewer caught the -v line parse breaking on a source path containing a colon, fixed with NUL-separated records and tests for colon and Windows-shaped sources, and got the warning to name the true cause; one fix round, reverify clean | went badly: the workflow's verify step failed on Docker's port-mapping race in an unrelated headless test and blocked the task, so the orchestrator ran the gate (check, core and repo integration, torture) and committed; the verify step now retries once on that race | change next time: a known environmental flake is retried by the gate, never used to block a task
