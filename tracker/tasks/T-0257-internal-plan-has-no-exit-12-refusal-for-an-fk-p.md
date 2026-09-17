---
id: T-0257
title: "internal/plan has no exit-12 refusal for an FK pair unknownColumnsBesideCertain cannot raise"
epic: E9
phase: ""
status: done
owner: ""
created: 2026-09-17
started: 2026-09-17
closed: 2026-09-17
outcome: done
---

# T-0257 · internal/plan has no exit-12 refusal for an FK pair unknownColumnsBesideCertain cannot raise

## Goal

T-0253: internal/classify/classify.go's fkPairs (unknownColumnsBesideCertain) detects a validated foreign key whose partner cannot be raised the same way as the column beside a certain neighbour -- the partner already carries a type-conflicting decision ARCHITECTURE.md section 4 forbids overriding, or is a measured two-letter-code lookup -- and correctly leaves both ends unmasked rather than raising one alone and copying the pair. That is a residual, not a fix: the pair is still copied verbatim end to end, the same leak T-0253 closed for the common (both-blank) case. Give pipeline.Decision a field classify can set naming the blocked pair (extend the existing, currently-unread Decision.Refused, or add one), and have internal/plan read it and refuse the run at exit 12 naming both columns with the --unmask escape, the way plan/writeback.go already refuses a unique-indexed column free_text cannot fill. TestFKPairRefusedWhenPartnerIsTwoLetterCodes and TestFKPairRefusedWhenPartnerHasTypeConflict (internal/classify/redteam_test.go) pin today's leave-both-unmasked behaviour and need updating alongside the fix; a torture regression naming the exit code is owed once the plan-side refusal exists.

## Acceptance



## Log

- 2026-09-17 created

- 2026-09-17 started

- 2026-09-17 closed: done

## Post-mortem

went well: the planner read landed inside T-0253's by-hand finish, reusing the write-back and unique-domain refusal shape; a plan-level test and regression 037 pin it | went badly: filed as Later by a developer who could not reach the planner, which is what blocked its parent | change next time: as T-0253
