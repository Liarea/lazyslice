---
id: T-0265
title: "verify's per-check pass lines never reach the terminal or --json"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-17
started: ""
closed: ""
outcome: ""
---

# T-0265 · verify's per-check pass lines never reach the terminal or --json

## Goal

T-0260's quickstart found a real run (internal/core/run.go's verify.New(...).Verify call, stage 7) never prints anything between 'stage.start' and 'stage.done' for the verify stage on a clean pass: internal/verify's state.pass/state.report (internal/verify/verify.go) only append to pipeline.Report.Checks, and nothing in internal/core/run.go sends those checks to the event sink, so verify.fk.passed, verify.residual.passed, verify.second_net.passed, verify.catalog.passed, verify.row_count.passed, verify.sequences.passed and verify.sample.passed (all kind: info in internal/event/catalogue.yml, and documented as such in docs/ERRORS.md) are dead letters on a successful run — same for --json. The only user-visible evidence a run's checks passed is the marker row's lazyslice_meta.status = 'complete' (internal/core/run.go's closeRun), which needs a direct database query to see. Fix: after Verify returns a report with no error, internal/core/run.go should iterate report.Checks and send each as its own event (respecting THREAT_MODEL.md T4: no value fields, which Report.Checks already avoids), or otherwise render the seven passed-check lines the catalogue already defines messages for. Proves itself: an integration test running lazyslice against a real target and asserting the seven verify.*.passed codes appear in the event stream.

## Acceptance



## Log

- 2026-09-17 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
