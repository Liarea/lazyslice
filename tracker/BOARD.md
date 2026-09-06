# Board · 2026-09-06

| Epic | Phase | Open | In progress | Done | Cancelled | Blocked |
|---|---|---|---|---|---|---|
| E0 Frame | 0 | 0 | 0 | 3 | 0 | 0 |
| E1 Research | 1 | 0 | 0 | 10 | 0 | 0 |
| E2 Architecture | 2 | 0 | 0 | 4 | 1 | 0 |
| E3 Foundations | 3 | 1 | 0 | 9 | 0 | 0 |
| E4 Vertical slice | 4 | 2 | 1 | 14 | 0 | 0 |
| E5 Hardening | 5 | 7 | 0 | 0 | 0 | 0 |
| E9 Later | later | 4 | 0 | 0 | 0 | 0 |

## Open and in progress

- T-0019 [open] E9 · Go vs Python COPY throughput benchmark to validate ADR-001 (opus)
- T-0028 [open] E3 · Create GitHub repository Liarea/lazyslice and homebrew-tap, push main, add HOMEBREW_TAP_TOKEN secret (human)
- T-0029 [open] E9 · Before going public: git-crypt the AI-specific paths and rewrite pre-encryption history (fable)
- T-0031 [open] E9 · Licence for the lazyslice.yml schema and docs; confirm copyright holder statement (fable)
- T-0044 [open] E4 · T-CORE: core run, emit, render, repo, CLI end to end; integration job blocking again (opus)
- T-0045 [open] E4 · T-DISCOVER: discovery rungs 0-3 and the first-run ladder (opus)
- T-0046 [open] E5 · Fixture: deferrable unique on a partitioned root, leaf-local key, and an edge referencing the leaf (sonnet)
- T-0048 [open] E9 · Explicit --key on an uncomparable column type surfaces a raw pgx error instead of a refusal (opus)
- T-0049 [open] E5 · Mask module low findings from T-MASK review (see T-0040 log) and a registry test that every rules.yml masker id resolves (sonnet)
- T-0050 [open] E5 · Extract and transform hand-offs from T-EXTRACT review (see T-0041 log): shape-template identifier escaping, KeySet chunk iterator, pgbouncer testcontainer, text-keyed big fixture (opus)
- T-0052 [open] E5 · Flake: TestKillNineLeavesEveryTableEmptyOrComplete races container teardown (port 5432/tcp not found) (sonnet)
- T-0053 [open] E5 · pg gate follow-ups from T-FPR: regression test pinning the fingerprinter transaction (SAVEPOINT must not 25P01), rollback failure routed through endTx discipline, comment corrections; schema-only Introspector variant so the gate skips sampling (opus)
- T-0054 [in_progress] E4 · Classify decides text categories on timestamp and tsvector columns (pagila last_update credential, film.fulltext address); transform then refuses at exit 7 mid-run (opus)
- T-0055 [open] E5 · Shared leaf package for value validators including the name dictionary; register person_name and free_text in verify's second net with a verify-side false-positive threshold decision (opus)
- T-0056 [open] E5 · Second net: weak threshold (0.5) plus neighbouring-column raise; the faithful reading is decorative, the alternative is a different control and needs a T1 review first (opus)

## Recently closed

- T-0035 [done] E4 · Verify negative control: a source email planted in a masked target column makes lazyslice verify exit 9 naming table and column → done: implemented inside T-VERIFY's integration suite
- T-0036 [done] E4 · T-PG: source and target connections, target gate, statement-shape allowlist, dsn → done: connections, marker, target gate, statement-shape tracer, dsn; package integration tests green
- T-0037 [done] E4 · T-INTROSPECT: introspect stage → done: 2dd2102 (stage) after fix2; PG18 contype filter, tolerant sampling, bounded TABLESAMPLE, FK end filters, extension walk narrowed, partition edges re-pointed only when the root can carry them
- T-0038 [done] E4 · T-CLASSIFY: classify stage with rule pack → done: classify with embedded rule pack, validators, English dictionary, accepted-type gate, FK propagation to a fixpoint, yml raise gate; unit tests green
- T-0039 [done] E4 · T-PLAN: subset planner → done: 86c5ee1; FIFO worklist with provenance, caps, budgets, identity ladder with §3.4 pseudo-keys, unreadable tables, SCC order, not-recreatable refusal; unit and integration tests green
- T-0040 [done] E4 · T-MASK: mask module → done: 2d1de18; key, HKDF, HMAC, every category generator with Domain(), small-domain reporting, unique-domain refusal, format preservation
- T-0041 [done] E4 · T-EXTRACT: extract and transform stages → done: 4fd5c28; chunked typed unnest extract with bounded memory (2M rows at 19 MiB growth), transform with JSON leaf masking and per-leaf residual digests, source pool read-only by session SET, extract shapes moved home
- T-0042 [done] E4 · T-LOAD: load stage → done: load committed; ddl generation of §11.1 object classes, COPY in plan order, NOT VALID then validate, setval, marker, empty-or-complete on kill -9 proved by test
- T-0043 [done] E4 · T-VERIFY: verify stage, includes T-0035 negative control → done: a57f584; FK validation, residual scan with capped confirmation, second net, sequences, row counts, sample compare, negative control (T-0035) exit 9 naming table and column; package suite green
- T-0047 [done] E4 · T-PGSHAPES: statement-shape grammar placeholders for plan and extract; plan exports Shapes() → done: grammar gains select-list item, variable-arity cast list, --where predicate with exclusions; plan exports Shapes() and its suite runs through pg.Source; extract shapes staged
- T-0051 [done] E4 · T-FPR: apply ADR-009, delete introspect fingerprint, core-side Schema.Fingerprint, pg gate runs fingerprinter in its own transaction → done: e85be26; introspect fingerprint deleted, pg gate runs the injected fingerprinter inside its own BEGIN/ROLLBACK, load's test workaround removed, marker binds end to end
- T-0002 [done] E1 · COMPETITORS.md teardown → done: 636-line teardown of 23 tools, 156 sources, critiqued and revised
- T-0003 [done] E1 · POSTMORTEMS.md Snaplet and Neosync → done: Snaplet and Neosync post-mortems with founder quotes, pricing history, five do-not-copy and three must-copy
- T-0004 [done] E1 · HARD_PROBLEMS.md technical survey → done: four hard problems surveyed with simplest-correct, refinement, and trap for each
- T-0005 [done] E1 · COMPLAINTS.md user quotes → done: 25+ verbatim complaints ranked by theme; silent success is the top failure class
- T-0006 [done] E1 · LICENSE_DECISION.md → done: Apache-2.0 with DCO, no CLA, no enterprise directory
- T-0007 [done] E1 · SQLIT_STUDY.md first-run design → done: sqlit first-run mechanics documented, question ladder specified
- T-0008 [done] E1 · AI_PROJECT_PRACTICES.md → done: practices to adopt and avoid from AI-assisted OSS projects, each sourced
- T-0009 [done] E1 · NAME.md collision check → done: lazysnap rejected (Go module, npm, GitHub account, DBSnapper adjacency, wrong semantics); renamed to lazyslice per ADR-000
- T-0010 [done] E1 · Prompting cheat sheets, five models plus index → done: five cheat sheets and a README with role templates
- T-0011 [done] E1 · SYNTHESIS.md and CONCEPT.md revision → done: ten facts, three risks, CONCEPT.md rewritten (pseudonymised, fail closed, 14 non-goals), 23 questions for architecture
- T-0013 [done] E0 · ADR-007 tool not company; OPEN_QUESTIONS.md; rename to lazyslice → done
- T-0014 [cancelled] E2 · Language throughput benchmark, Go vs Python → cancelled
- T-0015 [done] E2 · Three architecture proposals: mvp-first, risk-first, user-first → done: three proposals, each under 2000 words with verified library versions
- T-0016 [done] E2 · Judge panel scoring → done: judges split (user-first 33, risk-first 36, three-way tie 31/30/30); fatal flaws found in every proposal
