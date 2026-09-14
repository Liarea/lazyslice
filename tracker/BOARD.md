# Board · 2026-09-14

| Epic | Phase | Open | In progress | Done | Cancelled | Blocked |
|---|---|---|---|---|---|---|
| E0 Frame | 0 | 0 | 0 | 3 | 0 | 0 |
| E1 Research | 1 | 0 | 0 | 10 | 0 | 0 |
| E2 Architecture | 2 | 0 | 0 | 4 | 1 | 0 |
| E3 Foundations | 3 | 1 | 0 | 9 | 0 | 0 |
| E4 Vertical slice | 4 | 0 | 0 | 20 | 0 | 0 |
| E5 Hardening | 5 | 12 | 1 | 57 | 0 | 0 |
| E6 Launch | 6 | 1 | 0 | 0 | 0 | 0 |
| E9 Later | later | 21 | 0 | 20 | 4 | 0 |

## Open and in progress

- T-0019 [open] E9 · Go vs Python COPY throughput benchmark to validate ADR-001 (opus)
- T-0028 [open] E3 · Create GitHub repository Liarea/lazyslice and homebrew-tap, push main, add HOMEBREW_TAP_TOKEN secret (human)
- T-0031 [open] E9 · Licence for the lazyslice.yml schema and docs; confirm copyright holder statement (fable)
- T-0048 [open] E9 · Explicit --key on an uncomparable column type surfaces a raw pgx error instead of a refusal (opus)
- T-0064 [open] E5 · Dogfood: two sessions against a real project of Gareth's choosing, logged in docs/DOGFOOD_LOG.md (human)
- T-0065 [open] E6 · 20-second VHS GIF of the first run on Pagila (sonnet)
- T-0083 [in_progress] E5 · Target type registration for CopyFrom: no owner since internal/load shipped ()
- T-0087 [open] E9 · internal/classify's JSON leaf signal never consults the name dictionary, so verify's second net cannot score person_name or free_text over document leaves ()
- T-0089 [open] E5 · T-FAILUX: failure UX and error catalogue drift test (opus)
- T-0090 [open] E5 · T-PERF: performance baseline and CI throughput guard (opus)
- T-0102 [open] E9 · A text column holding a JSON document is invisible to ARCHITECTURE.md 4's JSON rule ()
- T-0119 [open] E5 · A table-scoped name rule, for refresh_tokens.parent and its kind ()
- T-0124 [open] E9 · testdata/regressions covers plan.refused.unique_domain no longer ()
- T-0125 [open] E9 · Makefile's vet-tagged comment still says .golangci.yml does not lint the torture tag ()
- T-0126 [open] E9 · internal/textsig/CLAUDE.md still says internal/verify has no URL entry (T-0122 has landed) ()
- T-0128 [open] E9 · A multidimensional array carried as a text literal is flattened to one dimension at CopyFrom ()
- T-0135 [open] E5 · dsn.Ref keeps the non-secret transport parameters so a rerun preserves sslmode and certificate paths (sonnet)
- T-0136 [open] E5 · Second net: one strong hit in an unmasked column is a finding; classify masks a mixed column that carries a strong hit (sonnet)
- T-0137 [open] E5 · JSON object keys that a strong validator hits are masked (sonnet)
- T-0138 [open] E5 · mapping_file is refused explicitly until it is implemented; ADR-012 records the deferral (sonnet)
- T-0139 [open] E5 · Torture suite fingerprints the source before the run, with a negative control (sonnet)
- T-0140 [open] E5 · CI runs the torture suite on main and the release workflow requires a green CI run for the tagged commit (sonnet)
- T-0142 [open] E9 · Implement the mapping_file contract of ADR-006 (opus)
- T-0143 [open] E9 · Decide the arbitrary-JSON policy: structure-preserving masking versus whole-document replacement (human)
- T-0144 [open] E9 · The memory budget accounts for samples, pending traversal, channels and batch bytes; rename the flag help to what it measures (sonnet)
- T-0145 [open] E9 · A read-only verify command that checks the current target without dropping it (opus)
- T-0146 [open] E9 · internal/verify: a residual hit on an array element cannot be confirmed by either probe of section 6 item 3 ()
- T-0154 [open] E9 · Equality groups over inferred edges: virtual_fks and polymorphic pairs ()
- T-0155 [open] E5 · v0.0.1 proves the release pipeline end to end: goreleaser, the tap cask, brew install prints a version (sonnet)
- T-0157 [open] E9 · Performance: the 20,000,000-row child run and byte-budget accounting (0.2) (sonnet)
- T-0158 [open] E9 · Export the closed-value label list from mask so plan compares labels, not CHECK text ()
- T-0159 [open] E9 · FK equality group: members with different type families can still mask differently ()
- T-0161 [open] E5 · core fills pipeline.PlanRequest.Key so a masked column's DEFAULT is masked instead of refused ()
- T-0162 [open] E9 · move the SQL literal scanner out of internal/pipeline into a leaf package beside internal/textsig ()
- T-0163 [open] E9 · the plan-time DDL literal rule does not read index predicates or domain CHECKs ()
- T-0164 [open] E9 · uniqueColumn is copied in internal/plan and internal/transform; give it a shared home ()

## Recently closed

- T-0029 [cancelled] E9 · Before going public: git-crypt the AI-specific paths and rewrite pre-encryption history → cancelled
- T-0118 [done] E5 · internal/transform: mask an array whose sample arrives as a text literal element-wise → done
- T-0127 [done] E5 · internal/plan: drop the arrayArrivesAsLiteral stand-in now that transform masks a literal array element-wise → done
- T-0129 [done] E5 · internal/verify: an array column that arrives as a text literal is not residual-scanned element-wise → done
- T-0130 [done] E5 · Target ownership: a run lease on the target and a lock-and-recheck before every destructive DDL → done
- T-0131 [done] E5 · Events carry no source value: polymorphic inference reports unknown type values by count and keyed digest, and an output-sink canary test proves it → done
- T-0132 [done] E5 · The masker is chosen per FK-connected equality group, not per column → done
- T-0133 [done] E5 · Core owns the run lifecycle: complete is written only after verify passes, and a residual failure empties the target → done
- T-0134 [done] E5 · Recreated DDL carries no sensitive literal: defaults on masked columns are masked, strong hits elsewhere refuse, verify scans the target catalog → partial
- T-0141 [done] E5 · README.md and SECURITY.md no longer claim every stage is a no-op → done
- T-0147 [done] E9 · internal/classify/CLAUDE.md array-literal section is stale after T-0118/T-0129/T-0127 → done
- T-0148 [done] E9 · Regenerate docs/ERRORS.md for the four exit-4 target-ownership codes → done
- T-0149 [done] E9 · Regenerate docs/ERRORS.md for T-0133's two new codes → done
- T-0150 [done] E9 · ARCHITECTURE.md §3.2 amendment (2026-09-08) still says a _type value is truncated and printed at 64 bytes → done
- T-0151 [cancelled] E9 · Key the polymorphic value digest on the run's actual mask key, not the published schema fingerprint → cancelled
- T-0152 [cancelled] E9 · ADR for T-0131's published-fingerprint value digest, or promote T-0151 → cancelled
- T-0153 [done] E9 · plan.refused.equality_group: a code of its own for the FK equality-group refusal → done
- T-0156 [done] E5 · Go public: git-crypt decision, tap initial commit, flip visibility, branch protection → done
- T-0160 [done] E9 · Regenerate docs/ERRORS.md for plan.refused.equality_group → done
- T-0093 [done] E5 · Move RegisterTypes onto pipeline.Writer so the load's type registration is compiler-checked → done: in T-HARD-C (4f9a186)
- T-0094 [done] E5 · A composite column now loads, and no rule pack category accepts its type family: decide refuse or mask field-wise (THREAT_MODEL.md T1) → done: in T-HARD-B (78530ef)
- T-0096 [done] E5 · testdata/CLAUDE.md and testdata/README.md still say 'two fixtures and nothing else' → done: in T-HARD-C (4f9a186)
- T-0100 [done] E5 · textsig.LooksSecret classifies a URL as a credential → done: in T-HARD-B (78530ef)
- T-0103 [done] E5 · An array of an extension type is sampled as one opaque string, so the classifier never sees the values inside it → done: in T-HARD-B (78530ef)
- T-0104 [done] E5 · The credential and online_id name rules miss the spellings an auth schema actually uses → done: in T-HARD-B (78530ef)
