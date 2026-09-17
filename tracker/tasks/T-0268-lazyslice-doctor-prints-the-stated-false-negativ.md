---
id: T-0268
title: "lazyslice doctor prints the stated false negatives, as ARCHITECTURE.md says it does"
epic: E9
phase: ""
status: open
owner: sonnet
created: 2026-09-17
started: ""
closed: ""
outcome: ""
---

# T-0268 · lazyslice doctor prints the stated false negatives, as ARCHITECTURE.md says it does

## Goal

ARCHITECTURE.md's T1 control list (the line beginning 'Stated false negatives, printed in the README and by lazyslice doctor') promises the list in two places, and only the README carries it: no Go file outside a test holds the text, and SECURITY.md said doctor printed it until the gate-5 read on 2026-09-17 corrected the sentence. Give internal/render one list, print it from ModeDoctor and as a --json event, and have a test compare its headings with README.md's 'What a snapshot will not hide' so the two cannot drift.

## Acceptance



## Log

- 2026-09-17 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
