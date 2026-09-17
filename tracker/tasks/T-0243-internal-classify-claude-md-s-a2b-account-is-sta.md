---
id: T-0243
title: "internal/classify/CLAUDE.md's A2b account is stale after T-0239"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-17
started: ""
closed: ""
outcome: ""
---

# T-0243 · internal/classify/CLAUDE.md's A2b account is stale after T-0239

## Goal

T-0239 lowered unknownColumnsBesideCertain's minUnknownLen from sixteen to two characters and cut the exclusion list from five to four (internal/classify/classify.go, the unknownColumnsBesideCertain doc comment). internal/classify/CLAUDE.md's own 'The 2026-09-15 red team' section (the A2b bullet list, 'Five exclusions...a declared length under sixteen') was outside T-0239's named paths and still describes the old five-exclusion, sixteen-character shape. Update that bullet list to four exclusions and record T-0239's correction (free_text's filler fits any declared length on a non-unique column; the floor is now two characters, a backstop under the two-letter-code value check for a column with too few samples), the way the file already records T-0221 and T-0198's corrections in the same section.

## Acceptance



## Log

- 2026-09-17 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
