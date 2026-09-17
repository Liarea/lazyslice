---
id: T-0250
title: "Pin T-0197's nothing_recognised/sub_threshold_signal split with a regression test"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-17
started: ""
closed: ""
outcome: ""
---

# T-0250 · Pin T-0197's nothing_recognised/sub_threshold_signal split with a regression test

## Goal

T-0197 review finding 2: internal/classify has no test asserting that a sampled character column with zero validator matches renders sub_threshold_signal vs nothing_recognised correctly (classify.go's default branch, ~line 1230; reasons.go's nothing_recognised/sub_threshold_signal fragments). Add a *_test.go case in internal/classify classifying a prior with a sampled character column carrying no signal and asserting Decision.Reason holds render("nothing_recognised", n), plus a companion for a column with a sub-threshold validator hit asserting render("sub_threshold_signal", n), plus one for a non-character or zero-sample column still rendering render("no_signal"). Precedent: TestReasonGrammarCoversPhoneRegion (classify_test.go:626), added for T-0221.

## Acceptance



## Log

- 2026-09-17 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
