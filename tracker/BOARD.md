# Board · 2026-09-09

| Epic | Phase | Open | In progress | Done | Cancelled | Blocked |
|---|---|---|---|---|---|---|
| E0 Frame | 0 | 0 | 0 | 3 | 0 | 0 |
| E1 Research | 1 | 0 | 0 | 10 | 0 | 0 |
| E2 Architecture | 2 | 0 | 0 | 4 | 1 | 0 |
| E3 Foundations | 3 | 1 | 0 | 9 | 0 | 0 |
| E4 Vertical slice | 4 | 0 | 0 | 20 | 0 | 0 |
| E5 Hardening | 5 | 4 | 2 | 46 | 0 | 0 |
| E6 Launch | 6 | 1 | 0 | 0 | 0 | 0 |
| E9 Later | later | 13 | 0 | 14 | 1 | 0 |

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
- T-0089 [open] E5 · T-FAILUX: failure UX and error catalogue drift test (opus)
- T-0090 [open] E5 · T-PERF: performance baseline and CI throughput guard (opus)
- T-0102 [open] E9 · A text column holding a JSON document is invisible to ARCHITECTURE.md 4's JSON rule ()
- T-0112 [open] E5 · Re-measure docs/TORTURE.md's flag counts and the catalogue's flags-by-kind after the credential_unique masker ()
- T-0118 [in_progress] E5 · internal/transform: mask an array whose sample arrives as a text literal element-wise ()
- T-0119 [open] E9 · A table-scoped name rule, for refresh_tokens.parent and its kind ()
- T-0124 [open] E9 · testdata/regressions covers plan.refused.unique_domain no longer ()
- T-0125 [open] E9 · Makefile's vet-tagged comment still says .golangci.yml does not lint the torture tag ()
- T-0126 [open] E9 · internal/textsig/CLAUDE.md still says internal/verify has no URL entry (T-0122 has landed) ()
- T-0127 [open] E9 · internal/plan: drop the arrayArrivesAsLiteral stand-in now that transform masks a literal array element-wise ()
- T-0128 [open] E9 · A multidimensional array carried as a text literal is flattened to one dimension at CopyFrom ()
- T-0129 [open] E9 · internal/verify: an array column that arrives as a text literal is not residual-scanned element-wise ()

## Recently closed

- T-0093 [done] E5 · Move RegisterTypes onto pipeline.Writer so the load's type registration is compiler-checked → done: in T-HARD-C (4f9a186)
- T-0094 [done] E5 · A composite column now loads, and no rule pack category accepts its type family: decide refuse or mask field-wise (THREAT_MODEL.md T1) → done: in T-HARD-B (78530ef)
- T-0096 [done] E5 · testdata/CLAUDE.md and testdata/README.md still say 'two fixtures and nothing else' → done: in T-HARD-C (4f9a186)
- T-0100 [done] E5 · textsig.LooksSecret classifies a URL as a credential → done: in T-HARD-B (78530ef)
- T-0103 [done] E5 · An array of an extension type is sampled as one opaque string, so the classifier never sees the values inside it → done: in T-HARD-B (78530ef)
- T-0104 [done] E5 · The credential and online_id name rules miss the spellings an auth schema actually uses → done: in T-HARD-B (78530ef)
- T-0105 [done] E5 · .golangci.yml does not lint the torture build tag → done: in T-HARD-C (4f9a186)
- T-0109 [done] E5 · T-HARD-B: composite fail-closed, extension-type array splitter, auth-schema rules, URL not credential (T-0094, T-0103, T-0104, T-0100) → done: 78530ef; composite fail-closed, array-literal splitter, auth-schema rules, URL is online_id, key-child exemption
- T-0110 [done] E5 · T-HARD-C: RegisterTypes on Writer, torture tag linted, testdata docs (T-0093, T-0105, T-0096) → done: 4f9a186; RegisterTypes on Writer, torture tag linted, testdata docs, online_id in the second net, regressions re-cut, make torture exits 0, torture re-measured, public_key is credential
- T-0113 [done] E5 · Regressions 004 and 007 expect exit 12 and now exit 0: credential_unique made their headers stale → done: in T-HARD-C (4f9a186)
- T-0114 [done] E5 · mask/CLAUDE.md and gen_credential.go still say the torture counts are un-re-measured → done: in T-HARD-C (4f9a186)
- T-0115 [done] E5 · docs/TORTURE.md: re-measure supabase-auth recall after T-0104's name rules → done: in T-HARD-C (4f9a186)
- T-0116 [done] E5 · Re-measure the torture flag counts after T-0100 and T-0104 → done: in T-HARD-C (4f9a186)
- T-0117 [done] E9 · THREAT_MODEL.md T1 and ARCHITECTURE.md owe the composite decision (T-0094 closed) → done: THREAT_MODEL T1 and ARCHITECTURE §4 record the composite fail-closed decision
- T-0120 [done] E9 · internal/classify: a masked FK child whose parent is copied orphans the row → done: in T-HARD-B (78530ef)
- T-0121 [done] E5 · Decide public_key: credential, online_id, or neither → done: in T-HARD-C (4f9a186)
- T-0122 [done] E5 · internal/verify: the second net lost URL coverage when T-0100 narrowed textsig.LooksSecret → done: in T-HARD-C (4f9a186)
- T-0123 [done] E9 · ARCHITECTURE.md section 2: Writer has four methods now, and pipeline.TypeRegistrar is gone → done: §2 Writer block has four methods with the reason; §11 note rewritten; seven citation sites stripped
- T-0046 [done] E5 · Fixture: deferrable unique on a partitioned root, leaf-local key, and an edge referencing the leaf → done: merged in the backlog run wf_e6bd43ea-99d (see git log for the hash)
- T-0049 [done] E5 · Mask module low findings from T-MASK review (see T-0040 log) and a registry test that every rules.yml masker id resolves → done: merged in the backlog run wf_e6bd43ea-99d (see git log for the hash)
- T-0050 [done] E5 · Extract and transform hand-offs from T-EXTRACT review (see T-0041 log): shape-template identifier escaping, KeySet chunk iterator, pgbouncer testcontainer, text-keyed big fixture → done: 2694a03; shape-template identifiers escaped, KeySet FirstChunk and EachChunk with extract and verify using them, pgbouncer testcontainer, text-keyed big fixture; §2 reconciled by the orchestrator
- T-0052 [done] E5 · Flake: TestKillNineLeavesEveryTableEmptyOrComplete races container teardown (port 5432/tcp not found) → done: merged in the backlog run wf_e6bd43ea-99d (see git log for the hash)
- T-0053 [done] E5 · pg gate follow-ups from T-FPR: regression test pinning the fingerprinter transaction (SAVEPOINT must not 25P01), rollback failure routed through endTx discipline, comment corrections; schema-only Introspector variant so the gate skips sampling → done: merged in the backlog run wf_e6bd43ea-99d (see git log for the hash)
- T-0055 [done] E5 · Shared leaf package for value validators including the name dictionary; register person_name and free_text in verify's second net with a verify-side false-positive threshold decision → done: 654bfd4; internal/textsig leaf holds validators and the name dictionary; person_name and free_text in the second net with a multi-token or hit-rate threshold
- T-0063 [done] E5 · Q1, the controlling terminal, and provisioning: widen pipeline.Provisioner to carry the generated POSTGRES_PASSWORD, then wire --create-target and the one blocking question → done: folded into T-PROVISION
