---
id: T-0407
title: "The v0.4.0 truth pass over README, SECURITY, CONCEPT, the quickstart transcript and the launch drafts"
epic: E6
phase: 6
status: done
owner: sonnet
created: 2026-09-25
started: ""
closed: 2026-10-02
outcome: done
---

# T-0407 · The v0.4.0 truth pass over README, SECURITY, CONCEPT, the quickstart transcript and the launch drafts

## Goal

docs/reviews/2026-09-25-docs-truth/findings.json holds 45 findings from three readers who read README.md, SECURITY.md and docs/launch/*.md end to end against the tree at 94f2809 (each with file, line, the sentence as written, what is true today with its source, and the replacement). Apply every finding except the version-string ones, which the orchestrator applies at tag time: README's Status heading and paragraph, the go install @v0.4.0 line, README line 267's version, SECURITY line 9's current-release sentence, and the four launch drafts' v0.1.0 lines; apply the rest verbatim where the fix gives a sentence, including the SECURITY item 8 rewording only if the task that masks every leaf of a personally named document column has not landed first (check git log for it; if it has, keep the promise and fix only the special-category wording). Also: CONCEPT.md lines 17 and 53 still say free text and JSON columns are masked whole, stale since T-0272 (JSON is masked leaf by leaf; free text is masked whole); and README lines 153 to 155 show masked full_name values as street addresses, which predates ADR-015's real names, so re-record docs/QUICKSTART_TRANSCRIPT.md and README's run excerpt against two scratch postgres:16 containers at HEAD with the same commands (the excerpt gains the classify.summary and plan.summary lines and the prompt reads root table? [public.customers]); remove the containers. Paths README.md, SECURITY.md, CONCEPT.md, docs/; make check (docs-check) proves it; do not change code.

## Acceptance

—

## Log

- 2026-09-25 2026-09-25 created
- 2026-10-02 closed: done

## Post-mortem

went well: landed as f7b5f56 in one fix round with one low finding; the truth pass read README, SECURITY, CONCEPT, the quickstart transcript and the launch drafts end to end against ADR-016, ADR-017, T-0272 and the eight jsonfix tasks, and the findings are on disk (docs/reviews/2026-09-25-docs-truth/findings.json) | went badly: the developer filed T-0424 on a wrong premise (classify's values-win rule explains the reason line it reported), and the review's low (CONCEPT and R_POSTGRESQL did not name the two cases that mask every leaf) was left for the orchestrator's release commit | change next time: a docs task that files a defect against the classifier reads the rule it is contradicting first; the version-string pass belongs in the same task as the truth pass, with the tag name passed in
