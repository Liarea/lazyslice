---
id: T-0329
title: "internal/core/core_test.go's recorder Residual double needs AddEmitted and Emitted (T-0302 blocks make check)"
epic: E9
phase: ""
status: done
owner: haiku
created: 2026-09-23
started: ""
closed: 2026-09-23
outcome: done
---

# T-0329 · internal/core/core_test.go's recorder Residual double needs AddEmitted and Emitted (T-0302 blocks make check)

## Goal

T-0302 (ADR-015) added AddEmitted(col, path, canonical []byte) and Emitted(col, path, canonical []byte) int64 to pipeline.Residual (internal/pipeline/transform.go), as the ADR and ARCHITECTURE.md section 2 require. internal/core/core_test.go's recorder (line ~154, used at line 148 as smallDomainAware{Residual: recorder{}}) is a pipeline.Residual test double outside T-0302's paths and no longer compiles: go vet and go test ./internal/core fail with 'recorder does not implement pipeline.Residual (missing method AddEmitted)'. Add the two no-op methods beside its others: func (recorder) AddEmitted(ref.ColumnRef, string, []byte) {} and func (recorder) Emitted(ref.ColumnRef, string, []byte) int64 { return 0 }. Production code needs no change: internal/core/run.go's smallDomainAware embeds pipeline.Residual and forwards both methods. Proof: make check green (T-0302's own run verified this exact patch through go test -overlay). Must land with T-0302, not after.

## Acceptance

—

## Log

- 2026-09-23 closed: done

## Post-mortem

went well: landed with T-0302 in eb4657b (first as b75b33f, re-landed after the split): the recorder double in internal/core/core_test.go gained no-op AddEmitted and Emitted, the only change that made make check green on the live tree | went badly: nothing | change next time: nothing
