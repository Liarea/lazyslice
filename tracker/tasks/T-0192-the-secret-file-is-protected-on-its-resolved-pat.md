---
id: T-0192
title: "The secret file is protected on its resolved path: symlinked parent directories and hard links refuse; password_command is screened before it is written to the yml"
epic: E5
phase: 5
status: done
owner: sonnet
created: 2026-09-15
started: 2026-09-16
closed: 2026-09-16
outcome: done
---

# T-0192 · The secret file is protected on its resolved path: symlinked parent directories and hard links refuse; password_command is screened before it is written to the yml

## Goal

Red team 2026-09-15 R2-14, R2-15, R2-16: internal/core/run.go checks the secret file with one os.Lstat on the last path component, so a symlinked parent directory carries the key into a cloud-synced folder past the .gitignore rail; a hard link defeats every path check; and --password-command is written verbatim into lazyslice.yml, whose header says the file never contains a secret, so a command such as echo followed by the password commits the password. Fix: resolve the whole path with filepath.EvalSymlinks (or walk every component) and refuse when it leaves the repository or resolves differently from the given path; stat the file and refuse when nlink is above 1 or mode grants group or other any permission (the 0644 case from round 1 is the same check); screen the emitted password_command and withhold it with a warning naming the yml field when its argv carries a token that is not a program name or a path (a heuristic; document it in THREAT_MODEL.md T5 and T6).

## Acceptance



## Log

- 2026-09-15 created

- 2026-09-16 started

- 2026-09-16 closed: done

## Post-mortem

went well: the three checks (a symlinked parent resolved before the mode check, a hard link refused, a password_command screened for a literal secret) each landed with a unit test, and the Opus reviewer's round-2 reproduction of a slash-bearing password recorded verbatim as a path was fixed by dropping the unanchored path branch, which made the code simpler | went badly: two fix rounds for a Sonnet task; a withheld password_command leaves lazyslice.yml byte-identical to a run that never had one, unlike the --where precedent (filed); the new tests do not pin the bypass shapes the reviewer named (filed) | change next time: a screening rule's brief lists the exact strings it must accept and refuse before code, the same lesson as T-0187
