# Board · 2026-09-08

| Epic | Phase | Open | In progress | Done | Cancelled | Blocked |
|---|---|---|---|---|---|---|
| E0 Frame | 0 | 0 | 0 | 3 | 0 | 0 |
| E1 Research | 1 | 0 | 0 | 10 | 0 | 0 |
| E2 Architecture | 2 | 0 | 0 | 4 | 1 | 0 |
| E3 Foundations | 3 | 1 | 0 | 9 | 0 | 0 |
| E4 Vertical slice | 4 | 0 | 0 | 20 | 0 | 0 |
| E5 Hardening | 5 | 1 | 1 | 26 | 0 | 0 |
| E6 Launch | 6 | 1 | 0 | 0 | 0 | 0 |
| E9 Later | later | 5 | 0 | 5 | 0 | 0 |

## Open and in progress

- T-0019 [open] E9 · Go vs Python COPY throughput benchmark to validate ADR-001 (opus)
- T-0028 [open] E3 · Create GitHub repository Liarea/lazyslice and homebrew-tap, push main, add HOMEBREW_TAP_TOKEN secret (human)
- T-0029 [open] E9 · Before going public: git-crypt the AI-specific paths and rewrite pre-encryption history (fable)
- T-0031 [open] E9 · Licence for the lazyslice.yml schema and docs; confirm copyright holder statement (fable)
- T-0048 [open] E9 · Explicit --key on an uncomparable column type surfaces a raw pgx error instead of a refusal (opus)
- T-0064 [open] E5 · Dogfood: two sessions against a real project of Gareth's choosing, logged in docs/DOGFOOD_LOG.md (human)
- T-0065 [open] E6 · 20-second VHS GIF of the first run on Pagila (sonnet)
- T-0083 [in_progress] E5 · Target type registration for CopyFrom: no owner since internal/load shipped ()
- T-0087 [open] E9 · internal/classify's JSON leaf signal never consults the name dictionary, so verify's second net cannot score person_name or free_text over document leaves ()

## Recently closed

- T-0046 [done] E5 · Fixture: deferrable unique on a partitioned root, leaf-local key, and an edge referencing the leaf → done: merged in the backlog run wf_e6bd43ea-99d (see git log for the hash)
- T-0049 [done] E5 · Mask module low findings from T-MASK review (see T-0040 log) and a registry test that every rules.yml masker id resolves → done: merged in the backlog run wf_e6bd43ea-99d (see git log for the hash)
- T-0050 [done] E5 · Extract and transform hand-offs from T-EXTRACT review (see T-0041 log): shape-template identifier escaping, KeySet chunk iterator, pgbouncer testcontainer, text-keyed big fixture → done: 2694a03; shape-template identifiers escaped, KeySet FirstChunk and EachChunk with extract and verify using them, pgbouncer testcontainer, text-keyed big fixture; §2 reconciled by the orchestrator
- T-0052 [done] E5 · Flake: TestKillNineLeavesEveryTableEmptyOrComplete races container teardown (port 5432/tcp not found) → done: merged in the backlog run wf_e6bd43ea-99d (see git log for the hash)
- T-0053 [done] E5 · pg gate follow-ups from T-FPR: regression test pinning the fingerprinter transaction (SAVEPOINT must not 25P01), rollback failure routed through endTx discipline, comment corrections; schema-only Introspector variant so the gate skips sampling → done: merged in the backlog run wf_e6bd43ea-99d (see git log for the hash)
- T-0055 [done] E5 · Shared leaf package for value validators including the name dictionary; register person_name and free_text in verify's second net with a verify-side false-positive threshold decision → done: 654bfd4; internal/textsig leaf holds validators and the name dictionary; person_name and free_text in the second net with a multi-token or hit-rate threshold
- T-0063 [done] E5 · Q1, the controlling terminal, and provisioning: widen pipeline.Provisioner to carry the generated POSTGRES_PASSWORD, then wire --create-target and the one blocking question → done: folded into T-PROVISION
- T-0066 [done] E5 · T-TUI: Bubble Tea reasons and plan screens → done: reasons and plan screens from the event stream, every binding carries its flag, footer strikes unavailable keys, leaving echoes flags into scrollback; --tui only on a TTY
- T-0067 [done] E5 · T-PROVISION: --create-target and rung 4 (T-0063) → done: provisioning, rung 4, Q1 prompter on the controlling terminal; the four core-side joins (Yes into Options, Provisioner interface, ArgContainer, unreachable args) are in a verified patch applied by the follow-up
- T-0068 [done] E5 · T-POLY: polymorphic association inference → done: inference of _type/_id and content_type/object_id pairs, virtual parent-direction edges under the caps, unmapped values reported once, over-cap pairs explained; full integration green
- T-0069 [done] E5 · T-CI5: five-major CI matrix, govulncheck, SBOM, docs drift, unsafe-flag grep → done: 91eab92; Postgres 14 to 18 matrix, blocking govulncheck, SBOM and signing in the release, tools/docgen generating FLAGS.md, KEYBINDINGS.md, ERRORS.md with a drift job, unsafe-flag rail
- T-0070 [done] E5 · T-PIN: the run that writes the target is pinned to the reviewed snapshot and endpoints (core.Request.Reviewed, core.refused.reviewed_changed) → done: af911ac; core.Preview, core.Request.Reviewed, core.refused.reviewed_changed; structural test pins the wiring
- T-0071 [done] E5 · Apply T-PROVISION's core patch: --yes reaches the ladder, delete the dead pipeline.Provisioner, ArgContainer in the not-ready refusal, unreachable-target args at all three sites, CodeTargetNone for the no-target refusal → done: 7a6b96d; --yes reaches the ladder, dead Provisioner interface removed, not-ready refusal names the container and a docker logs command, unreachable refusals carry host and cause at all three sites, no-target refusal has its own code
- T-0072 [done] E5 · Print inferred polymorphic edges in the plan (plan.polymorphic.inferred in core, catalogue, tui); emit virtual_fks test and comments; fixture and README promises updated; Rails-spelled trap-6 row → done: inferred edges printed as plan.polymorphic.inferred in core, catalogue, and the TUI; emit test over virtual_fks; trap 6 fixture and README describe the shipped behaviour with a Rails-spelled row
- T-0073 [done] E5 · T-0073: fixture email values out of the masker's documentation-domain output space (I2 intermittent collision) → done: merged in the backlog run wf_e6bd43ea-99d (see git log for the hash)
- T-0074 [done] E5 · Unsafe-flag rail enforced over the registered flag set: main_test's forbidden list permits exactly unmask and rejects every other name containing it; make unsafe-flags runs that test → done: 2a35a36; forbidden rule walks every registered flag set recursively, permits exactly unmask, self-test proves it fires; make unsafe-flags runs the test
- T-0075 [done] E5 · CI red from provisioning: password-file mode assertion on Windows; provisioned container not ready within 60 s on GitHub runners (image pull inside the deadline) → done: merged in the backlog run wf_e6bd43ea-99d (see git log for the hash)
- T-0076 [done] E5 · T-0076: source read-only setting per transaction, never a session GUC that leaks through a transaction-pooling PgBouncer; T9 reworded → done: 2d57b00; session GUC removed, SystemID in a read-only transaction, pgbouncer neighbour test proves a second client can CREATE TABLE after lazyslice exits, T9 reworded
- T-0077 [done] E5 · T-0077: stream_docs lifted above the nasty.sql big gate; introspect table list updated; extract workaround removed → done: 7142c52; stream_docs declared beside stream_rows above the gate, only the fill is gated; introspect table list updated; extract workaround removed
- T-0078 [done] E5 · internal/load/ddl: drop the dangling back-reference to target.refused.start_timeout's deleted comment → done: back-reference dropped in T-0081's commit
- T-0079 [done] E9 · Record the review pin outside internal/core: ARCHITECTURE.md entry points, cmd/CLAUDE.md, and ADR-005's exit 12 → done: ARCHITECTURE.md §1 records the three entry points and the review pin; ADR README errata carries the fifth exit-12 cause; cmd/CLAUDE.md names core.Preview
- T-0080 [done] E9 · Anchor /lazyslice in .gitignore so a bare go build cannot commit a 32MB binary → done: /lazyslice anchored in .gitignore (2d57b00)
- T-0081 [done] E5 · Stale default_transaction_read_only prose outside internal/pg after T-0076 → done: 3df6a22; dial inside a read-only transaction, shape miss beats the transaction rule, probe acts on its tracer verdict, stale prose rewritten
- T-0082 [done] E5 · No mechanism detects a source statement sent outside a transaction → done: folded into T-0081; the tracer refuses any source statement outside a transaction, pinned by unit and real-server tests
- T-0084 [done] E9 · THREAT_MODEL.md T9 still says the discover dial sends its reads outside a transaction → done: THREAT_MODEL T9 gained the dial bullet
