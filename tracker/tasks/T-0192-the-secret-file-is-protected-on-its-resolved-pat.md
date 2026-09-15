---
id: T-0192
title: "The secret file is protected on its resolved path: symlinked parent directories and hard links refuse; password_command is screened before it is written to the yml"
epic: E5
phase: 5
status: open
owner: sonnet
created: 2026-09-15
started: ""
closed: ""
outcome: ""
---

# T-0192 · The secret file is protected on its resolved path: symlinked parent directories and hard links refuse; password_command is screened before it is written to the yml

## Goal

Red team 2026-09-15 R2-14, R2-15, R2-16: internal/core/run.go checks the secret file with one os.Lstat on the last path component, so a symlinked parent directory carries the key into a cloud-synced folder past the .gitignore rail; a hard link defeats every path check; and --password-command is written verbatim into lazyslice.yml, whose header says the file never contains a secret, so a command such as echo followed by the password commits the password. Fix: resolve the whole path with filepath.EvalSymlinks (or walk every component) and refuse when it leaves the repository or resolves differently from the given path; stat the file and refuse when nlink is above 1 or mode grants group or other any permission (the 0644 case from round 1 is the same check); screen the emitted password_command and withhold it with a warning naming the yml field when its argv carries a token that is not a program name or a path (a heuristic; document it in THREAT_MODEL.md T5 and T6).

## Acceptance



## Log

- 2026-09-15 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
