---
id: T-0127
title: "internal/plan: drop the arrayArrivesAsLiteral stand-in now that transform masks a literal array element-wise"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-09
started: ""
closed: ""
outcome: ""
---

# T-0127 · internal/plan: drop the arrayArrivesAsLiteral stand-in now that transform masks a literal array element-wise

## Goal

T-0118 landed the transform half: internal/transform/array.go parses a Postgres array literal, masks each element with h per element and re-emits a literal CopyFrom writes (verified against postgres:16 with citext registered). internal/plan/writeback.go's arrayArrivesAsLiteral refusal was explicitly a stand-in for that masker -- internal/transform/CLAUDE.md says it 'goes when T-0118 does' -- and it now refuses at exit 12 a column the pipeline can mask. It is what makes testdata/regressions/009-citext-array-of-addresses-masked-as-one-string.sql exit 12 instead of 0, so make torture is red on that one file until this lands. internal/plan was outside T-0118's paths. Remove arrayArrivesAsLiteral and its call site, keep the composite refusal above it, and update the plan-side tests and internal/plan/CLAUDE.md.

## Acceptance



## Log

- 2026-09-09 created

- 2026-09-09 Two constraints from T-0118's review, both to be met in the same change. (1) testdata/regressions/009-citext-array-of-addresses-masked-as-one-string.sql now headers 'expect: exit 12 plan.refused.unwritable', which is the tree as it stands; removing arrayArrivesAsLiteral must flip that header to 'ok' in the same commit, or make torture goes red. That flip is also what first runs the file's leak check: the harness returns before assertTortureNoLiteralSurvives for any file expecting a non-zero exit, so until this task lands internal/transform/array.go has unit coverage only and no end-to-end evidence. (2) Removing the refusal moves the failure for an *unparseable* array literal from plan time (exit 12, before a key is fetched) to load time (transform.refused.masker, exit 7, mid-stream with earlier tables already committed) -- the half-loaded target writeback.go's own comment says the stand-in exists to prevent. The two grammars are not the same: internal/classify/literal.go is deliberately liberal (it accepts {a,} and doubled quotes and flattens nesting) and its acceptance is what makes the column reach transform at all, so classify can decide a column array.go then refuses. Either keep a plan-time parse check over the samples, or establish and record that any literal classify accepts, array.go also accepts.

- 2026-09-09 internal/plan/CLAUDE.md line 518 still says the branch stands 'until T-0118 lands' and credits T-HARD-B; T-0118 has landed, so that line is stale on the tree today and this task is what removes it rather than re-dating it.

- 2026-09-09 Third constraint from T-0118's re-review, and it is an ordering, not a code change: this task does not land before T-0129, or it lands together with internal/verify refusing or flagging a masked array column whose target value does not decode to []any. Reason: removing arrayArrivesAsLiteral is what makes T-0129's blindness live. Today no *masked* array column arriving as a text literal can reach the target at all -- this refusal stops it at exit 12 -- so verify's inability to see inside the literal (internal/verify/residual.go, arrayHits falls back to scalarHits on the whole value when it is not a []any) costs nothing. The commit that removes the refusal is the first commit under which such a column loads, and it loads with ARCHITECTURE.md section 6 item 1's second net inert over exactly the column class T-0118 enables, while the run report calls the column masked. That is the fail-open THREAT_MODEL.md T12 is about, and section 6 item 1 is the only control against it. Land T-0129 first, or land the two together, so the blindness is loud rather than a green tick.

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
