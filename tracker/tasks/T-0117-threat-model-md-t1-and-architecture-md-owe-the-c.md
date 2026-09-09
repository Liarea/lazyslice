---
id: T-0117
title: "THREAT_MODEL.md T1 and ARCHITECTURE.md owe the composite decision (T-0094 closed)"
epic: E9
phase: ""
status: done
owner: ""
created: 2026-09-08
started: ""
closed: 2026-09-09
outcome: "done: THREAT_MODEL T1 and ARCHITECTURE §4 record the composite fail-closed decision"
---

# T-0117 · THREAT_MODEL.md T1 and ARCHITECTURE.md owe the composite decision (T-0094 closed)

## Goal

T-HARD-B closed T-0094 in code: internal/classify gives a composite its own type family, runs the validators over the record's fields, and reaches 'possible' on a name or a field hit; internal/plan/writeback.go then refuses at exit 12 under plan.refused.unwritable naming the column, --skip-table and a reasoned --unmask; a composite with no hit is copied with the reason 'composite type: its fields were read and none is personal data'. Two documents owe the record and both were outside those paths: THREAT_MODEL.md T1's list of known phase-4/5 gaps, which T-0094 asked to have the composite gap added to and can now record it as closed with the fail-closed behaviour; and ARCHITECTURE.md, whose section 4 'accepted types per category' has no composite family and whose section 5 does not say that a record is unmaskable by construction.

## Acceptance



## Log

- 2026-09-08 created

- 2026-09-09 closed: done: THREAT_MODEL T1 and ARCHITECTURE §4 record the composite fail-closed decision

## Post-mortem

Went well: doc. Went badly: nothing. Change: none.
