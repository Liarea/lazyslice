---
id: T-0224
title: "ARCHITECTURE.md/THREAT_MODEL.md/internal/pg/CLAUDE.md claim EXECUTE on pg_control_system is not granted to PUBLIC, which is false on stock postgres:16"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-16
started: ""
closed: ""
outcome: ""
---

# T-0224 · ARCHITECTURE.md/THREAT_MODEL.md/internal/pg/CLAUDE.md claim EXECUTE on pg_control_system is not granted to PUBLIC, which is false on stock postgres:16

## Goal

T-0222's own integration test (internal/pg/gate_integration_test.go, TestGateRecognisesTheSourceClusterWithPostmasterStartTimeDenied) measured EXECUTE on pg_control_system() as PUBLIC-executable by default on a fresh postgres:16 container with a freshly created NOSUPERUSER role and no grant beyond CONNECT on the database -- has_function_privilege('pg_control_system()','EXECUTE') returned true and the call itself succeeded with no explicit REVOKE. ARCHITECTURE.md section 9, THREAT_MODEL.md T2 (the 2026-09-15 amendment) and internal/pg/CLAUDE.md's T-0190 section all repeat the opposite claim -- 'EXECUTE on pg_control_system is not granted to PUBLIC' -- as the premise for why rule 1's system_identifier disjunct is unavailable under the recommended SELECT-only role, and none of them cites a Postgres version or a source. Check whether this was ever true on a version ARCHITECTURE.md section 14 / docs/TORTURE.md's supported matrix (14, 16, 18) still covers, or is inherited prose debt; correct the claim in all three files (and any other internal/pg/CLAUDE.md section repeating it) either to name the actual default per version or to drop the specific claim in favour of 'the recommended role should not be assumed to have it, and the identity works either way' -- restating the load-bearing point without staking it on the potentially-wrong premise. If EXECUTE really is PUBLIC-granted by default on 14/16/18, rule 1's own two-disjunct design (a second cluster identity an ordinary role can read) is still correct defense in depth for a cluster that revokes it explicitly, but the docs should not claim the SELECT-only role is silently denied it when it is not.

## Acceptance



## Log

- 2026-09-16 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
