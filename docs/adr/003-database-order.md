# ADR-003: PostgreSQL 14 to 18, one adapter, one definition of done, then nothing until the gate

Status: accepted, 2026-09-05

## Context

CONCEPT.md non-goals: databases other than PostgreSQL wait "until the invariant suite passes on every Postgres fixture". research/SYNTHESIS.md §4 item 16 warns that "both companies wrote that sentence and then did not obey it" and asks for a testable condition; research/POSTMORTEMS.md §10 item 4 is the discipline both dead companies broke. research/COMPLAINTS.md DB-1 ("and then noticed it didn't support MSSQL") heads sixteen entries about discovering unsupportedness after installing, and Greenmask's MySQL epic has been open two years (DB-4).

The proposals differ on the supported majors (research/proposals/mvp-first.md "D3": 14 to 18, 13 being end-of-life; research/proposals/user-first.md "3. Database order": 13 to 18 with a CI job per major; research/proposals/risk-first.md "3. Database order": none named, which the first judge docked). They agree an adapter needs a written definition of done, and each contributes a piece of it.

## Options considered

- **PostgreSQL 13 to 18.** 13 has been end-of-life since 2025-11-13; a CI job for it is cost with no user.
- **PostgreSQL 14 to 18, CI on 14 and 18 only.** The cheapest matrix; misses a regression on 15 to 17 until a user hits it.
- **PostgreSQL 14 to 18, CI on every major.** Five containers per integration run. testcontainers-go makes the matrix a loop, not five configs.

## Decision

PostgreSQL 14, 15, 16, 17 and 18 are supported. Phase 4 CI runs 14 and 18; Gate 5 requires the full matrix. A source or target on an unsupported major is refused at discover with exit 2 naming the version. Any DSN whose scheme is not `postgres://` or `postgresql://` exits 2 naming the engine and pointing at the support table, which sits above the fold in the README (research/proposals/user-first.md "3. Database order", answering DB-1).

All engine-specific code lives in `internal/pg` behind the interfaces in ADR-005; `switch engine` appears nowhere in the tree.

**An adapter is done when all of the following hold**, merged from the three proposals:

1. It implements every engine-facing interface in ARCHITECTURE.md (`Source`, `Target`, `Introspector`, `Extractor`, `Loader`, `Verifier`) including the privilege report, the statement-allowlist tracer, the snapshot primitive or a named refusal, the target eligibility gate and the marker table (research/proposals/risk-first.md "3").
2. Invariants I1 to I6 pass on Pagila and on `nasty.sql` on every supported major, in the adapter's own CI job against a real server (research/proposals/mvp-first.md "D3"; research/AI_PROJECT_PRACTICES.md §4 item 8).
3. `make integration` runs it under testcontainers-go with no manual setup.
4. `lazyslice doctor` prints seven capability rows for it, none of them a silent "n/a": snapshot, bulk load, FK and unique introspection, sequence reset, cycle handling, read-only check, residual scan (research/proposals/user-first.md "3").
5. Every error it raises appears in `docs/ERRORS.md` with its exit code, enforced by a test (research/proposals/mvp-first.md "D3"; docs/BUILD_PLAN.md PROMPT 5.2).
6. THREAT_MODEL.md has a row per control the adapter implements (research/proposals/risk-first.md "3").
7. `docs/ADDING_A_DATABASE.md` was corrected by the port (research/proposals/user-first.md "3").

**A second engine begins only when** the Postgres adapter has had no open subsetting or masking issue for thirty days, and download data, not stars, names the engine (dbslice: 143 stars, 33 downloads a month, research/SYNTHESIS.md §1 fact 10). Order after that, if the signal is silent on order: MySQL, SQLite, SQL Server, as docs/BUILD_PLAN.md recommends.

## Consequences

- `testdata/` carries both fixtures from phase 3, and the invariant suite is the definition of correct (docs/BUILD_PLAN.md PROMPTS 3.3 and 3.4).
- Phase 7 adapters each get their own worktree and CI job (docs/OPERATING_MODEL.md "Parallelism").
- `lazyslice doctor` is a v1 subcommand; it exists to make "done" printable, not only to help users.
- Nobody adds a fifteenth masker or a Slack integration while a Postgres subsetting issue is open. The tracker enforces this by putting such tasks in a later epic with one line of reasoning (CLAUDE.md).

## Reversal condition

None before v1. If after v1 the thirty-day condition is met and downloads name an engine, the next adapter starts; that is the plan working, not a reversal. Dropping a supported Postgres major follows the PostgreSQL end-of-life calendar and needs no ADR.
