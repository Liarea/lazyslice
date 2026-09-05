# Board · 2026-09-05

| Epic | Phase | Open | In progress | Done | Cancelled | Blocked |
|---|---|---|---|---|---|---|
| E0 Frame | 0 | 0 | 0 | 3 | 0 | 0 |
| E1 Research | 1 | 0 | 0 | 10 | 0 | 0 |
| E2 Architecture | 2 | 0 | 0 | 4 | 1 | 0 |
| E3 Foundations | 3 | 2 | 1 | 6 | 0 | 0 |
| E4 Vertical slice | 4 | 0 | 0 | 0 | 0 | 0 |
| E5 Hardening | 5 | 0 | 0 | 0 | 0 | 0 |
| E9 Later | later | 2 | 0 | 0 | 0 | 0 |

## Open and in progress

- T-0019 [open] E9 · Go vs Python COPY throughput benchmark to validate ADR-001 (opus)
- T-0022 [in_progress] E3 · Invariant suite I1-I6, black box (opus)
- T-0026 [open] E3 · Foundation review and fixes (opus)
- T-0028 [open] E3 · Create GitHub repository Liarea/lazyslice and homebrew-tap, push main, add HOMEBREW_TAP_TOKEN secret (human)
- T-0029 [open] E9 · Before going public: git-crypt the AI-specific paths and rewrite pre-encryption history (fable)

## Recently closed

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
- T-0017 [done] E2 · ADRs 001-006, ARCHITECTURE.md, THREAT_MODEL.md → done: ADRs 001-006, ARCHITECTURE.md (14 sections, v1 cut line), THREAT_MODEL.md (T1-T13), ADR README
- T-0018 [done] E2 · Adversarial review of the decision set and one revision → done: 42 findings from three lenses, all addressed; import cycle, target gate, planner determinism, JSON masking, frequency leaks fixed
- T-0020 [done] E3 · Repository scaffold per ARCHITECTURE.md sections 12 and 13 → done: 8254d0f; 20 packages, interfaces, Makefile, CI, goreleaser, SECURITY, CONTRIBUTING; make check green
- T-0021 [done] E3 · Golden fixtures: Pagila and nasty.sql → done: 31aa82e; Pagila pinned v3.1.0 with checksums, nasty.sql 21 tables, 22 traps documented, 2M-row generator, testutil loaders
- T-0023 [done] E3 · Per-directory CLAUDE.md files → done: 3b18a1e; 22 CLAUDE.md files across cmd, internal packages, testdata, docs, tracker, .claude
- T-0024 [done] E3 · ADR-008 first run from lazygit, lazydocker, k9s source → done: research/FIRST_RUN_STUDY.md from lazygit, lazydocker, k9s source; docs/adr/008-first-run.md proposed with six-step Docker order, question ladder, locality predicate
- T-0025 [done] E3 · ROADMAP.md with gates and Later → done: ROADMAP.md with phases 4 to 8 gates, Not-in-this-phase lists, Later seeded from non-goals
- T-0027 [done] E3 · Apply ADR-008 consequences to ARCHITECTURE.md, catalogue, ADR index → done: 94466d0; six-step Docker order, gate order, locality predicate, three exit-4 event codes, ADR index updated
- T-0001 [done] E0 · Write CONCEPT.md → done
- T-0012 [done] E0 · Operating model, root CLAUDE.md, tracker tool → done
