# ADR-007: v1 is a tool, not a company

Status: accepted, 2026-09-05

## Context

research/POSTMORTEMS.md establishes that Snaplet and Neosync, the two funded companies that built this exact product, died of demand rather than execution, and that buyers with budgets rejected the hosted model in public. Neosync's founder wrote that there was not enough demand. research/SYNTHESIS.md fact 1 and risk 1 conclude: be a tool, not a company. The build plan's phase 8 hosted service is the shape that failed twice.

## Decision

Phase 8 of docs/BUILD_PLAN.md is deferred indefinitely and its epic is not created. The binary makes no runtime call to anything we operate. There is no control plane, no account, no login. The deterministic masker is designed as a separately importable module so it outlives the tool, as copycat outlived Snaplet. The licence is Apache-2.0 with DCO, no CLA, no enterprise directory, so Homebrew core and corporate adoption are not blocked.

The hosted question reopens only when download data, not stars, shows unattended CI use: a GitHub Action or CI mode being run on a schedule by teams we do not know.

## Consequences

The pitch changes from "open core with a hosted tier" to "a tool people trust". Phase 8 research prompts about pricing and hosted architecture are not run. Design-partner interviews in phase 8 may still happen, as user research, not sales.

## Reversal condition

The adoption signal above, or the human deciding to fund a company anyway with full knowledge of the two post-mortems.
