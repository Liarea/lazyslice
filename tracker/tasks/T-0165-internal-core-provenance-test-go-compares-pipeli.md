---
id: T-0165
title: "internal/core/provenance_test.go compares pipeline.Candidate with == after T-0135 added dsn.Ref.Params"
epic: E9
phase: ""
status: done
owner: ""
created: 2026-09-14
started: 2026-09-14
closed: 2026-09-14
outcome: done
---

# T-0165 · internal/core/provenance_test.go compares pipeline.Candidate with == after T-0135 added dsn.Ref.Params

## Goal

T-0135 (internal/dsn, internal/emit, internal/discover: allowlisted Params map on dsn.Ref, docs/reviews/2026-09-09/REVIEW.md finding 6) makes dsn.Ref, and so pipeline.Candidate, no longer comparable with == because Params is a map[string]string. internal/core/provenance_test.go:180 and :187 (fillCandidates) do 'r.sourceCand == (pipeline.Candidate{})' / 'r.targetCand == (pipeline.Candidate{})', which fails to compile as of that change (go vet/go test ./internal/core/... : "invalid operation: ... (struct containing dsn.Ref cannot be compared)"). internal/core is outside T-0135's paths, so it could not be fixed there. Fix: replace both with reflect.DeepEqual(r.sourceCand, pipeline.Candidate{}) / reflect.DeepEqual(r.targetCand, pipeline.Candidate{}) (add "reflect" import), matching the same fix already applied in internal/dsn/dsn_test.go, internal/emit/emit_test.go and internal/discover/discover_integration_test.go for the same reason. Blocks make check/make test passing on main until fixed.

## Acceptance



## Log

- 2026-09-14 created

- 2026-09-14 started

- 2026-09-14 closed: done

## Post-mortem

went well: dsn.Ref.IsZero and pipeline.Candidate.IsZero, both comparison sites converted, no other == on the types in the tree | change next time: nothing
