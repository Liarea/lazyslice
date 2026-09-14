---
id: T-0164
title: "uniqueColumn is copied in internal/plan and internal/transform; give it a shared home"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-14
started: ""
closed: ""
outcome: ""
---

# T-0164 · uniqueColumn is copied in internal/plan and internal/transform; give it a shared home

## Goal

internal/plan/ddlliteral.go's uniqueColumn is a hand copy of internal/transform/constraints.go's, added by T-0134's review round so a masked column's DEFAULT is masked under the same mask.Constraints its rows are. A stage package may not import another, so nothing but a comment and internal/plan/ddlliteral_test.go's TestUniqueColumnIsSpeltAsTransformSpellsIt holds the two spellings together: a change to transform's rule that is not made here masks a default with the wrong generator. The home is a leaf beside internal/textsig, the same one T-0162 owes the SQL literal scanner.

## Acceptance



## Log

- 2026-09-14 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
