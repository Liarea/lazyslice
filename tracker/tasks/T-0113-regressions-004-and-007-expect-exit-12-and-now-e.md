---
id: T-0113
title: "Regressions 004 and 007 expect exit 12 and now exit 0: credential_unique made their headers stale"
epic: E5
phase: 5
status: done
owner: ""
created: 2026-09-08
started: ""
closed: 2026-09-09
outcome: "done: in T-HARD-C (4f9a186)"
---

# T-0113 · Regressions 004 and 007 expect exit 12 and now exit 0: credential_unique made their headers stale

## Goal

make torture fails two of its eight regressions, and has since T-HARD-A (ecc42ae) — measured at that commit with T-0112's catalogue changes stashed, so it is not T-0112's doing.

testdata/regressions/004-composite-unique-index-all-masked.sql and 007-partial-unique-index-masked-column.sql both carry '-- expect: exit 12 plan.refused.unique_domain'. Both reduce a *credential* column under a unique index (004: public.reg4_grant.access_token, 'name matches credential; 200/200 samples look like secrets', under a composite unique index whose every key column is masked; 007: public.reg7_account.confirmation_token under a partial unique index). Both were written when CatCredential's only generator was the fixed literal, whose Domain() is 1, so the plan could not satisfy d_required and refused at exit 12. mask/gen_credential.go's credential_unique now escalates them and both runs exit 0 with the column masked, which is the fix working. TestTortureRegressions compares the exit code to the header and fails.

What is owed is a decision, not a one-line edit, and it belongs to whoever owns testdata/regressions/ and internal/classify:

1. Does each file still reduce a defect? 004's defect is 'a composite unique index whose every key column is masked collided at load' and 007's is the partial-index version of the same. The *classifier* rules those files pinned (raiseCompositeUnique, the single-column partial-index raise) are unchanged and still right; what changed is that the plan can now satisfy the raise for one category instead of refusing. So the reduction should probably keep its schema and change its header to 'expect: ok' plus a masked-and-distinct assertion — a run that exits 0 having *copied* the column would then still fail it, which a bare 'expect: ok' would not.
2. Or the reduction should be re-cut over a category that still cannot escalate, so that 'exit 12 plan.refused.unique_domain' keeps a regression at all. plan.refused.unique_domain is a real refusal with a real message and, after this change, nothing in testdata/regressions/ covers it.

Paths: testdata/regressions/ (both .sql files and README.md if the header grammar gains an assertion), internal/invariants/torture_test.go if option 1 needs a per-file masked-column check. docs/TORTURE.md's 'Defects found and fixed' rows 4 and 7 quote the exit-12 behaviour and would move with it.

Found by T-0112 while re-measuring the flag counts; recorded in internal/invariants/CLAUDE.md.

## Acceptance



## Log

- 2026-09-08 created

- 2026-09-09 moved to E5 phase 5

- 2026-09-09 closed: done: in T-HARD-C (4f9a186)

## Post-mortem

Went well: landed. Went badly: nothing. Change: none.
