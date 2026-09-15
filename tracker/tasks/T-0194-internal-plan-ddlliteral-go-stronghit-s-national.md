---
id: T-0194
title: "internal/plan/ddlliteral.go: strongHit's national_id entry should call the narrower textsig.ValidNationalIDStructured"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-15
started: ""
closed: ""
outcome: ""
---

# T-0194 · internal/plan/ddlliteral.go: strongHit's national_id entry should call the narrower textsig.ValidNationalIDStructured

## Goal

T-0187 review round (2026-09-15, finding 2): strongHit's national_id entry (internal/plan/ddlliteral.go:109) calls textsig.ValidNationalID, the twelve-format union, whose five checksum-only formats (PESEL, BSN, SIN, TFN, Aadhaar) clear a random digit run of the right length 9.1-25.7% of the time (measured); a plan-time refusal on one hit therefore refuses roughly one ordinary schema in four that carries a nine-digit reference code, order number or date in a CHECK or DEFAULT. internal/verify/catalog.go's own strongCatalogHit and internal/verify/validators.go's strong text-family entry were switched to textsig.ValidNationalIDStructured (the six shape-constrained formats: SSN, NINO, codice fiscale, DNI, NIE, NIR) in this same review round; internal/plan is outside that task's paths and is owed the identical one-line change plus ddlliteral_test.go coverage that a bare checksum-only-shaped literal (e.g. a PESEL-valid 11-digit run with no other signal) does not refuse a plan.

## Acceptance



## Log

- 2026-09-15 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
