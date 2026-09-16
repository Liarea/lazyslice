---
id: T-0215
title: "Secret-file and password_command tests pin the bypass shapes, not only the happy attack shapes"
epic: E9
phase: ""
status: open
owner: sonnet
created: 2026-09-16
started: ""
closed: ""
outcome: ""
---

# T-0215 · Secret-file and password_command tests pin the bypass shapes, not only the happy attack shapes

## Goal

T-0192's reviewer (low): internal/core/secretfile_test.go has no case where the symlink target contains .git, none where the secret path is nested two levels below the symlink, and internal/emit has no case for a path-shaped password literal; the only negative control would pass a check that refused every symlinked component regardless of repository. Add those cases so the suite fails on the bypasses. Files: internal/core/secretfile_test.go, internal/emit/emit_test.go.

## Acceptance



## Log

- 2026-09-16 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
