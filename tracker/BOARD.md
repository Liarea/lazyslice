# Board · 2026-09-08

| Epic | Phase | Open | In progress | Done | Cancelled | Blocked |
|---|---|---|---|---|---|---|
| E0 Frame | 0 | 0 | 0 | 3 | 0 | 0 |
| E1 Research | 1 | 0 | 0 | 10 | 0 | 0 |
| E2 Architecture | 2 | 0 | 0 | 4 | 1 | 0 |
| E3 Foundations | 3 | 1 | 0 | 9 | 0 | 0 |
| E4 Vertical slice | 4 | 0 | 0 | 20 | 0 | 0 |
| E5 Hardening | 5 | 5 | 0 | 19 | 0 | 0 |
| E6 Launch | 6 | 1 | 0 | 0 | 0 | 0 |
| E9 Later | later | 4 | 0 | 0 | 0 | 0 |

## Open and in progress

- T-0019 [open] E9 · Go vs Python COPY throughput benchmark to validate ADR-001 (opus)
- T-0028 [open] E3 · Create GitHub repository Liarea/lazyslice and homebrew-tap, push main, add HOMEBREW_TAP_TOKEN secret (human)
- T-0029 [open] E9 · Before going public: git-crypt the AI-specific paths and rewrite pre-encryption history (fable)
- T-0031 [open] E9 · Licence for the lazyslice.yml schema and docs; confirm copyright holder statement (fable)
- T-0048 [open] E9 · Explicit --key on an uncomparable column type surfaces a raw pgx error instead of a refusal (opus)
- T-0055 [open] E5 · Shared leaf package for value validators including the name dictionary; register person_name and free_text in verify's second net with a verify-side false-positive threshold decision (opus)
- T-0064 [open] E5 · Dogfood: two sessions against a real project of Gareth's choosing, logged in docs/DOGFOOD_LOG.md (human)
- T-0065 [open] E6 · 20-second VHS GIF of the first run on Pagila (sonnet)
- T-0070 [open] E5 · T-PIN: the run that writes the target is pinned to the reviewed snapshot and endpoints (core.Request.Reviewed, core.refused.reviewed_changed) (opus)
- T-0076 [open] E5 · T-0076: source read-only setting per transaction, never a session GUC that leaks through a transaction-pooling PgBouncer; T9 reworded (opus)
- T-0077 [open] E5 · T-0077: stream_docs lifted above the nasty.sql big gate; introspect table list updated; extract workaround removed (sonnet)

## Recently closed

- T-0046 [done] E5 · Fixture: deferrable unique on a partitioned root, leaf-local key, and an edge referencing the leaf → done: merged in the backlog run wf_e6bd43ea-99d (see git log for the hash)
- T-0049 [done] E5 · Mask module low findings from T-MASK review (see T-0040 log) and a registry test that every rules.yml masker id resolves → done: merged in the backlog run wf_e6bd43ea-99d (see git log for the hash)
- T-0050 [done] E5 · Extract and transform hand-offs from T-EXTRACT review (see T-0041 log): shape-template identifier escaping, KeySet chunk iterator, pgbouncer testcontainer, text-keyed big fixture → done: 2694a03; shape-template identifiers escaped, KeySet FirstChunk and EachChunk with extract and verify using them, pgbouncer testcontainer, text-keyed big fixture; §2 reconciled by the orchestrator
- T-0052 [done] E5 · Flake: TestKillNineLeavesEveryTableEmptyOrComplete races container teardown (port 5432/tcp not found) → done: merged in the backlog run wf_e6bd43ea-99d (see git log for the hash)
- T-0053 [done] E5 · pg gate follow-ups from T-FPR: regression test pinning the fingerprinter transaction (SAVEPOINT must not 25P01), rollback failure routed through endTx discipline, comment corrections; schema-only Introspector variant so the gate skips sampling → done: merged in the backlog run wf_e6bd43ea-99d (see git log for the hash)
- T-0063 [done] E5 · Q1, the controlling terminal, and provisioning: widen pipeline.Provisioner to carry the generated POSTGRES_PASSWORD, then wire --create-target and the one blocking question → done: folded into T-PROVISION
- T-0066 [done] E5 · T-TUI: Bubble Tea reasons and plan screens → done: reasons and plan screens from the event stream, every binding carries its flag, footer strikes unavailable keys, leaving echoes flags into scrollback; --tui only on a TTY
- T-0067 [done] E5 · T-PROVISION: --create-target and rung 4 (T-0063) → done: provisioning, rung 4, Q1 prompter on the controlling terminal; the four core-side joins (Yes into Options, Provisioner interface, ArgContainer, unreachable args) are in a verified patch applied by the follow-up
- T-0068 [done] E5 · T-POLY: polymorphic association inference → done: inference of _type/_id and content_type/object_id pairs, virtual parent-direction edges under the caps, unmapped values reported once, over-cap pairs explained; full integration green
- T-0069 [done] E5 · T-CI5: five-major CI matrix, govulncheck, SBOM, docs drift, unsafe-flag grep → done: 91eab92; Postgres 14 to 18 matrix, blocking govulncheck, SBOM and signing in the release, tools/docgen generating FLAGS.md, KEYBINDINGS.md, ERRORS.md with a drift job, unsafe-flag rail
- T-0071 [done] E5 · Apply T-PROVISION's core patch: --yes reaches the ladder, delete the dead pipeline.Provisioner, ArgContainer in the not-ready refusal, unreachable-target args at all three sites, CodeTargetNone for the no-target refusal → done: 7a6b96d; --yes reaches the ladder, dead Provisioner interface removed, not-ready refusal names the container and a docker logs command, unreachable refusals carry host and cause at all three sites, no-target refusal has its own code
- T-0072 [done] E5 · Print inferred polymorphic edges in the plan (plan.polymorphic.inferred in core, catalogue, tui); emit virtual_fks test and comments; fixture and README promises updated; Rails-spelled trap-6 row → done: inferred edges printed as plan.polymorphic.inferred in core, catalogue, and the TUI; emit test over virtual_fks; trap 6 fixture and README describe the shipped behaviour with a Rails-spelled row
- T-0073 [done] E5 · T-0073: fixture email values out of the masker's documentation-domain output space (I2 intermittent collision) → done: merged in the backlog run wf_e6bd43ea-99d (see git log for the hash)
- T-0074 [done] E5 · Unsafe-flag rail enforced over the registered flag set: main_test's forbidden list permits exactly unmask and rejects every other name containing it; make unsafe-flags runs that test → done: 2a35a36; forbidden rule walks every registered flag set recursively, permits exactly unmask, self-test proves it fires; make unsafe-flags runs the test
- T-0075 [done] E5 · CI red from provisioning: password-file mode assertion on Windows; provisioned container not ready within 60 s on GitHub runners (image pull inside the deadline) → done: merged in the backlog run wf_e6bd43ea-99d (see git log for the hash)
- T-0044 [done] E4 · T-CORE: core run, emit, render, repo, CLI end to end; integration job blocking again → done: core merged at 9bd9e24, blockers closed by T-0058
- T-0045 [done] E4 · T-DISCOVER: discovery rungs 0-3 and the first-run ladder → done: 5381042; rungs 0 to 3, six-step Docker endpoint resolution, compose and .env as naming sources, one blocking question, headless asks nothing; unit tests green
- T-0056 [done] E5 · Second net: weak threshold (0.5) plus neighbouring-column raise; the faithful reading is decorative, the alternative is a different control and needs a T1 review first → decided: the second net keeps the strong threshold only; recorded in THREAT_MODEL T1 and ARCHITECTURE §6
- T-0058 [done] E4 · T-CORE-FIX: fail closed on low-cardinality columns (verify minValues, introspect full-read below the TABLESAMPLE floor), I6 pagila counted root, I2 derived_text exemption, event.go embeds the catalogue, drop the ci allowlist; then make integration must be fully green → done: tiny tables sampled by bounded read, verify fails closed below minValues, I6 counts a parent-free root, derived_text exempt from the loaded guard, event.go embeds the catalogue, CI allowlist and continue-on-error removed; make integration green end to end
- T-0059 [done] E4 · Fixture inet values out of RFC 5737 so masked IPs cannot equal source values; remove the flake paragraphs; mask test pins the output space → done: 65ab0a3; fixture inet values are RFC 1918, flake paragraphs replaced by a pointer to §5, mask range test added
- T-0060 [done] E5 · Carry discovery provenance into the emitted yml: discover.Result and core.Request keep the rung and label, so a committed source: compose / source_label: db is not rewritten as source: flag → done: discover.Result carries provenance and label through core.Request into emit; a committed source: compose survives argument-free reruns
- T-0061 [done] E5 · Move firstRun from cmd/lazyslice into internal/core's discover stage, so cmd/ calls only core.Run again; update cmd/CLAUDE.md in the same change → done: first-run ladder lives in core's discover stage; cmd/ calls only core.Run; cmd/CLAUDE.md true again
- T-0062 [done] E5 · A gate refusal of the chosen target ends the run instead of falling through to the runner-up: core.Request carries one target, not a list → done: one target per request, gate refusal ends the run at exit 4, pinned by an integration test; ARCHITECTURE.md §9 and ADR-008 §5 now say so
- T-0035 [done] E4 · Verify negative control: a source email planted in a masked target column makes lazyslice verify exit 9 naming table and column → done: implemented inside T-VERIFY's integration suite
- T-0036 [done] E4 · T-PG: source and target connections, target gate, statement-shape allowlist, dsn → done: connections, marker, target gate, statement-shape tracer, dsn; package integration tests green
