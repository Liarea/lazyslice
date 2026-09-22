---
id: T-0293
title: "plan equality groups ignore person_name Role, letting an FK pair mask two roles alike"
epic: E9
phase: ""
status: done
owner: sonnet
created: 2026-09-22
started: ""
closed: 2026-09-22
outcome: done
---

# T-0293 · plan equality groups ignore person_name Role, letting an FK pair mask two roles alike

## Goal

internal/plan/equality.go:100 (maskedMembers/fitsGroup) builds mask.Constraints via constraintsOf plus Unique/Rows but never carries Decision.Role, while internal/transform masks each column with its own Role (T-0287). Two person_name columns joined by an FK with different roles (e.g. child.given_name REFERENCES names(name): RoleGiven vs RoleFull) report equal Domain() at plan time and pass the equality-group check, then transform masks them to different shapes from one digest -- the FK breaks at load (exit 8) instead of being refused at plan. Carry d.Role onto m.cons in maskedMembers the same way Unique is carried, so fitsGroup's Domain comparison and Admissible see the role; better, refuse a group whose members disagree on Role. Same file's Admissible/Required for a unique person_name column is also computed against the pair domain rather than the single-list domain and needs the same fix. Add a regression: two FK-linked person_name columns with different roles must refuse at plan (or be brought into agreement), not load and break the constraint. Found in T-0287's fix-round review (internal/plan is outside that task's paths).

## Acceptance

—

## Log

- 2026-09-22 2026-09-22 created
- 2026-09-22 closed: done

## Post-mortem

went well: 8894b3c: maskedMembers carries Decision.Role; a group whose roles disagree is brought to RoleFull on members and decisions (agreeOnRole) instead of refused, since no flag sets a role; TestEqualityGroupBringsDisagreeingRolesToFull fails without the fix | went badly: nothing | change next time: nothing
