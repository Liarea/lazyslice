---
id: T-0172
title: "testdata/regressions/013-json-object-key-that-parses-as-an-email.sql fails make torture on main"
epic: E5
phase: 5
status: done
owner: ""
created: 2026-09-14
started: 2026-09-14
closed: 2026-09-14
outcome: done
---

# T-0172 · testdata/regressions/013-json-object-key-that-parses-as-an-email.sql fails make torture on main

## Goal

Pre-existing failure on main (confirmed by stashing T-0119's changes and re-running), unrelated to T-0119: internal/invariants exits 9 verify.refused.second_net on public.reg013_items.payload instead of the exit-0 masked outcome the fixture's own comment expects. Blocks make torture from reporting a clean run and needs a look in internal/verify or internal/classify's JSON handling, outside T-0119's paths (internal/classify/, testdata/, docs/TORTURE.md, internal/invariants/ -- but this needs a fix in classify's JSON masking of jsonb keys or the fixture's own expectation, not just a doc update).

## Acceptance



## Log

- 2026-09-14 created

- 2026-09-14 moved to E5 phase 5

- 2026-09-14 started

- 2026-09-14 closed: done

## Post-mortem

went well: root cause found in one pass (the net was refusing the masker's own output on masked jsonb keys), fix narrowed to the three categories json.go masks so network_id, online_id, credential and address keys stay covered; thirteen regressions green under make torture; the wider skip-all-keys shape and the global output-space exclusion both rejected with reasons in internal/verify/CLAUDE.md | went badly: T-0137 returned without running make torture despite its brief, so a red regression sat on main across four landings until T-0119's developer noticed; CI's torture job on main (T-0140) now catches this, as it did on 6ba9d4a | change next time: the workflow's verify step runs make torture for any task whose paths include testdata/regressions
