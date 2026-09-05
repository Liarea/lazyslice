# Open questions from the phase 1 completeness review

Written by the orchestrator from the completeness critic's findings, 2026-09-05. Phase 2 proposals must take a position on each. Items marked "decided" are the orchestrator's call and need only be implemented.

1. **Pooled source endpoints.** The extract design relies on one pg_export_snapshot under REPEATABLE READ shared by every source connection. PgBouncer in transaction mode does not support SET TRANSACTION SNAPSHOT or WITH HOLD cursors. Proposals must say what happens when the source DSN is a pooler: detect and fall back to single-connection extract, refuse with a message, or something else.
2. **Target side has no evidence base.** research/COMPLAINTS.md found no user complaints about target-side failures, yet the target is where the catastrophic write lives. Proposals must specify the target eligibility gate precisely (empty database, or one carrying our marker table, or --target override) and what "empty" means when the compose test database is already migrated but has no rows.
3. **What a green verification tick does not prove.** The residual-PII scan must have a specified algorithm and a stated false-negative surface. Proposals must define the scan and list what it cannot catch, so the README can say so.
4. **Masking key lifecycle.** No research document covers key rotation, loss, compromise, or distribution to a team. Proposals must specify where the key lives (lazyslice.secret, environment, keyring), what a keyless CI run gets, and what happens on rotation. Decided: the key never appears in lazyslice.yml; only a fingerprint does.
5. **SQLIT_STUDY rests on one repository.** Before ADR-006 (first run) is written, the decide agent must also consult lazygit's and lazydocker's first-run behaviour and k9s's context handling, from their source, not memory.
6. **Tonic pricing is single-sourced** (Vendr). Decided: no external pricing claim is quoted anywhere until it has a first-party source. Irrelevant to architecture.
7. **Docker context resolution.** Go's Docker client FromEnv reads only environment variables, so OrbStack and Colima users whose socket lives elsewhere would see "Docker not running". The claim that every such user is affected is overstated; the underlying gap is real. Proposals must specify how discovery resolves the active Docker context, in order: DOCKER_HOST, the current context in ~/.docker/config.json and its meta, then the default socket paths, and how it degrades when none respond.
8. **No user has been asked.** Decided: phase 5 adds a dogfood task against a real project of the human's choosing, and the phase 8 design-partner interviews remain as user research per ADR-007. Not an architecture question.

Also from research/SYNTHESIS.md section 5, the twenty-three questions listed there are binding on the proposals. Two are decided here to save a round:

- **Flag naming:** the root-count flag is `--take`, matching invariant I6 and the build plan. Short form `-n`.
- **Hosted service:** struck, see docs/adr/007-tool-not-company.md.
