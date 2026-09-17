---
id: T-0267
title: "Workflow scripts carry the checkout's absolute home path in a REPO constant"
epic: E9
phase: ""
status: open
owner: sonnet
created: 2026-09-17
started: ""
closed: ""
outcome: ""
---

# T-0267 · Workflow scripts carry the checkout's absolute home path in a REPO constant

## Goal

Each of the six files in .claude/workflows/ starts with a REPO constant holding the absolute path of the maintainer's checkout, which puts a home-directory name into a public repository and ties the scripts to one machine. Replace it with something the runtime resolves (the working directory the workflow is launched from) or a relative path, and prove it with a scratch run of one step as .claude/CLAUDE.md's Test section asks. Found on 2026-09-17 while sweeping the same path out of docs/reviews/.

## Acceptance



## Log

- 2026-09-17 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
