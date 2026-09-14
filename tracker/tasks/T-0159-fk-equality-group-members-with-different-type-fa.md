---
id: T-0159
title: "FK equality group: members with different type families can still mask differently"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-14
started: ""
closed: ""
outcome: ""
---

# T-0159 · FK equality group: members with different type families can still mask differently

## Goal

internal/plan/equality.go's fitsGroup does not compare mask.Constraints.TypeTag across a group; generators branch on the tag (inet vs cidr in gen_net.go, date vs timestamp in gen_number.go, uuid in gen_text.go), so two FK-joined members of different families emit different text from one h and the key fails at exit 8. A blanket tag-equality rule would wrongly refuse the ordinary text-vs-varchar pair, so the rule needs deciding.

## Acceptance



## Log

- 2026-09-14 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
