# ADR-001: Go, one static binary, no cgo

Status: accepted, 2026-09-05

## Context

CONCEPT.md "Terminal first" requires one static binary, a one-line install, no native dependencies and no runtime call to anything we operate. research/SYNTHESIS.md §1 fact 9 records that installation is the top-voted request in this category (pg_sample's "How do I install this?", open since 2020; Snaplet Snapshot's top two issues are Apple Silicon install failures) and that a native `better-sqlite3` pin is what ended Snapshot's afterlife. research/HARD_PROBLEMS.md §2.1 notes `crypto/hkdf` is standard library from Go 1.24, so the masker needs no third-party crypto; §4.1 and §4.2 rest on pgx's typed `CopyFrom` and streaming `Query`.

All three proposals chose Go (research/proposals/mvp-first.md "D1", research/proposals/risk-first.md "1. Language", research/proposals/user-first.md "1. Language"). The judges did not dispute the choice; they disputed the reversal condition, because every proposal cited `research/BENCHMARK_LANGUAGE.md`, which does not exist, and the user-first proposal was the only one honest that "after phase 3 this is a rewrite, not a reversal".

Two costs are known up front. Go's Docker client does not resolve Docker contexts, so OrbStack and Colima users would see "Docker not running" (research/SQLIT_STUDY.md §2.1 and §5.1; research/OPEN_QUESTIONS.md item 7). No maintained Go format-preserving-encryption library exists (research/HARD_PROBLEMS.md §2.2).

## Options considered

- **Go, `CGO_ENABLED=0`.** Static binary on darwin, linux and windows from one goreleaser config; pgx v5 is pure Go and streams COPY from a bounded channel; the Docker SDK is first party; Bubble Tea is the lazygit lineage. Cost: we write Docker context resolution ourselves (about sixty lines) and FPE is struck.
- **Python with Textual.** Thirty databases faster via SQLAlchemy, as sqlit shows (research/SQLIT_STUDY.md §1), but distribution is the category's top complaint, a single-file build needs a native toolchain per platform, and streaming throughput is worse. Breadth is a phase 7 problem, not a phase 4 one (docs/BUILD_PLAN.md "Recommended decisions").
- **Rust.** Static binary and speed, but no Bubble Tea equivalent with the same maturity, slower contribution, and no advantage on the actual bottleneck, which is the network round trip (research/HARD_PROBLEMS.md §4.2 "Batch size and memory").

## Decision

Go. Toolchain `go 1.27.1` pinned in `go.mod` (current stable per go.dev/dl, checked 2026-09-05). `CGO_ENABLED=0` is enforced in the Makefile and in CI; a dependency that needs cgo is rejected at review, not worked around.

Docker context resolution is ours: `internal/discover/dockerctx` resolves `--docker-host` → `DOCKER_HOST` → `DOCKER_CONTEXT` → `currentContext` in `~/.docker/config.json` with `~/.docker/contexts/meta/*/meta.json` → the default socket paths for Docker Desktop, OrbStack, Colima and Rancher Desktop, printing each endpoint tried when none answers. It is the first integration test written in phase 3, because research/SQLIT_STUDY.md §5.1 calls it "the highest-risk single defect in the first-run path". The client is `github.com/moby/moby/client` v0.6.0; `github.com/docker/docker` stopped at v28.5.2+incompatible (2025-11-05) and is not used (verified against proxy.golang.org 2026-09-05).

Format-preserving encryption is struck for v1 (research/SYNTHESIS.md §3). Word lists for fakes are our own, `go:embed`ed, never gofakeit's, because a faker's list changing between versions silently changes the mapping (research/HARD_PROBLEMS.md §2.3 "Faker version drift").

The dependency list with versions and one reason each is in ARCHITECTURE.md "Dependencies"; every module is pinned to an exact version verified against proxy.golang.org on the day it is added.

## Consequences

- One `goreleaser` config produces darwin/linux/windows binaries and a Homebrew formula; `lazyslice --version` on every built target is a CI job (research/SQLIT_STUDY.md §5.9).
- The masker ships as a nested Go module, `github.com/Liarea/lazyslice/mask`, depending only on the standard library, `golang.org/x/text` and `nyaruka/phonenumbers` (ADR-006).
- A second engine (ADR-003) is a Go adapter, not a driver swap; SQLAlchemy-style breadth is not available and is not wanted before Gate 5.
- Nobody in this project may add a cgo dependency, a Go plugin, or a runtime download.

## Reversal condition

Reversal means a rewrite once phase 3 has shipped; this ADR says so rather than pretending otherwise. The only condition under which we would still do it: the phase 5 performance task (docs/BUILD_PLAN.md PROMPT 5.3, 5,000 root rows against a 20M-row child table) shows extract plus mask below 20,000 rows per second or peak RSS growing with table size, profiling blames the runtime rather than our code, and the fix does not fit in one task. No language benchmark document exists and none is scheduled before that task; the phase 5 numbers are the benchmark.
