# Board · 2026-09-16

| Epic | Phase | Open | In progress | Done | Cancelled | Blocked |
|---|---|---|---|---|---|---|
| E0 Frame | 0 | 0 | 0 | 3 | 0 | 0 |
| E1 Research | 1 | 0 | 0 | 10 | 0 | 0 |
| E2 Architecture | 2 | 0 | 0 | 4 | 1 | 0 |
| E3 Foundations | 3 | 0 | 0 | 10 | 0 | 0 |
| E4 Vertical slice | 4 | 0 | 0 | 20 | 0 | 0 |
| E5 Hardening | 5 | 5 | 1 | 79 | 0 | 0 |
| E6 Launch | 6 | 1 | 0 | 0 | 0 | 0 |
| E9 Later | later | 51 | 0 | 26 | 4 | 0 |

## Open and in progress

- T-0019 [open] E9 · Go vs Python COPY throughput benchmark to validate ADR-001 (opus)
- T-0031 [open] E9 · Licence for the lazyslice.yml schema and docs; confirm copyright holder statement (fable)
- T-0048 [open] E9 · Explicit --key on an uncomparable column type surfaces a raw pgx error instead of a refusal (opus)
- T-0064 [open] E5 · Dogfood: two sessions against a real project of the maintainer's choosing, logged in docs/DOGFOOD_LOG.md (human)
- T-0065 [open] E6 · 20-second VHS GIF of the first run on Pagila (sonnet)
- T-0083 [in_progress] E5 · Target type registration for CopyFrom: no owner since internal/load shipped ()
- T-0087 [open] E9 · internal/classify's JSON leaf signal never consults the name dictionary, so verify's second net cannot score person_name or free_text over document leaves ()
- T-0102 [open] E9 · A text column holding a JSON document is invisible to ARCHITECTURE.md 4's JSON rule ()
- T-0124 [open] E9 · testdata/regressions covers plan.refused.unique_domain no longer ()
- T-0125 [open] E9 · Makefile's vet-tagged comment still says .golangci.yml does not lint the torture tag ()
- T-0126 [open] E9 · internal/textsig/CLAUDE.md still says internal/verify has no URL entry (T-0122 has landed) ()
- T-0128 [open] E9 · A multidimensional array carried as a text literal is flattened to one dimension at CopyFrom ()
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
- T-0162 [open] E9 · move the SQL literal scanner out of internal/pipeline into a leaf package beside internal/textsig ()
- T-0164 [open] E9 · uniqueColumn is copied in internal/plan and internal/transform; give it a shared home ()
- T-0166 [open] E9 · wire dsn param-drop warnings into internal/core and internal/pg's own dsn.Parse call sites ()
- T-0168 [open] E9 · dsn.Ref.Params misses rung-2 (env/PGSERVICE) settings, so a first run through libpq env alone reruns with no sslmode at rung 0 ()
- T-0170 [open] E9 · torture regression 013 (json-object-key-email) fails on main ()
- T-0173 [open] E9 · Stop.Args is never populated by core.wrap, so many transcript error lines render generic while only the final exit line is specific (opus)
- T-0174 [open] E9 · Wire --debug to print the statement trace (Source.Trace) on an ordinary failure ()
- T-0175 [open] E9 · Give nasty.sql's stream fixtures a size parameter for perf profiling ()
- T-0176 [open] E9 · mask.Apply's HMAC-SHA256 derivation is the largest CPU cost in the extract/transform/load pipeline ()
- T-0178 [open] E9 · TestAStoppedContainerIsOfferedAndStarted binds a fixed host port (127.0.0.1:5433) and collides under parallel container runs (sonnet)
- T-0182 [open] E9 · A text column holding a JSON document is refused, not masked ()
- T-0183 [open] E9 · verify does not assert the object its catalog pass refused is gone after the quarantine ()
- T-0184 [open] E5 · Should a headless run auto-select a target on the source's own cluster? ()
- T-0185 [open] E9 · ARCHITECTURE.md does not know about --allow-type-literal ()
- T-0186 [open] E5 · --allow-type-literal is not recorded in lazyslice.yml ()
- T-0195 [open] E9 · classify: national_id's checksum-only formats can weak-ratio-mask an ordinary numeric business key ()
- T-0196 [open] E9 · Migrate the tracker to GitHub Issues and Projects behind the existing tools/tracker.py command surface (sonnet)
- T-0197 [open] E9 · R2-05/A10 residual: a name in a language names.txt does not carry still reports "no name or value signal" as a clean bill of health (sonnet)
- T-0198 [open] E9 · special_category has no value validator; a digit/name-free special-category sentence still crosses unseen (sonnet)
- T-0199 [open] E9 · internal/load's gate test holds its own copy of the cluster-identity SQL ()
- T-0200 [open] E9 · A cluster identity test that reaches one server over a genuinely different socket ()
- T-0201 [open] E9 · internal/pipeline/ddlliteral.go: validate a pattern operand both raw and with metacharacters stripped, and strip _ only for LIKE-family operators (sonnet)
- T-0202 [open] E9 · internal/plan/ddlliteral.go: judge only the indexes the loader recreates, not the ones backing PRIMARY KEY, UNIQUE and EXCLUDE constraints (sonnet)
- T-0203 [open] E9 · testdata/regressions: end-to-end fixtures for the round-2 DDL attempts (index predicate, pattern operand, enum label) (sonnet)
- T-0204 [open] E9 · internal/pg: ClusterID takes the system identifier core already read instead of reading pg_control_system a second time (sonnet)
- T-0205 [open] E9 · internal/testutil/proxy.go: close accepted connections in the same Cleanup that closes the listener (sonnet)
- T-0206 [open] E9 · Decide the cluster identity's field set now that the maintenance-database oid is the constant 5 on PostgreSQL 15 and newer (opus)
- T-0207 [open] E9 · verify reports its coverage and exclusions in the result: which columns the residual scan tested, which domains and tables it skipped, and why (sonnet)
- T-0208 [open] E9 · Measured differentiation: time to a working application on the same schemas against Greenmask, overrides and dependencies counted (sonnet)
- T-0209 [open] E9 · ARCHITECTURE.md describes the design as PostgreSQL-specific until a second engine is real (sonnet)
- T-0210 [open] E9 · Why a row is included, boundary crossings and cap omissions visible before copy, and a decision on which job the default slice serves (opus)
- T-0211 [open] E9 · ROADMAP.md stops naming docs/BUILD_PLAN.md as the sequencing authority (sonnet)
- T-0212 [open] E9 · renderSafe redacts by default, and prints a transform refusal's reason under the values flag ()
- T-0213 [open] E5 · Wire --password-command to actually run, or refuse it as unimplemented ()
- T-0214 [open] E9 · A withheld password_command leaves a marker in lazyslice.yml so a later run without one is refused, as a withheld --where already is (sonnet)
- T-0215 [open] E9 · Secret-file and password_command tests pin the bypass shapes, not only the happy attack shapes (sonnet)

## Recently closed

- T-0192 [done] E5 · The secret file is protected on its resolved path: symlinked parent directories and hard links refuse; password_command is screened before it is written to the yml → done
- T-0028 [done] E3 · Create GitHub repository Liarea/lazyslice and homebrew-tap, push main, add HOMEBREW_TAP_TOKEN secret → done
- T-0090 [done] E5 · T-PERF: performance baseline and CI throughput guard → done
- T-0163 [done] E5 · the plan-time DDL literal rule does not read index predicates or domain CHECKs → done
- T-0177 [done] E5 · Record real ubuntu-latest bench baseline and flip bench job to blocking → done
- T-0179 [done] E5 · Bench CI gate compares head against its parent on the same runner; absolute baseline becomes a catastrophic floor → done
- T-0180 [done] E5 · mask.Apply has no post-condition: a masker that returns its input is accepted → done
- T-0181 [done] E5 · mask.Apply does not recover: a panicking masker escapes the module → done
- T-0187 [done] E5 · National-identifier validators with checksums in textsig; classify and the second net treat them as strong; a digits-family SSN shape under the ratio rule → done
- T-0188 [done] E5 · Multilingual given-name and surname lists in textsig, sourced under CC0, so a non-English name in a column with no name rule is recognised → done
- T-0189 [done] E5 · Catalog literals: every validator over string literals in CHECK, domain, enum and generated expressions; pattern operands detected but not rewritten; plan reads partial-index predicates → done
- T-0190 [done] E5 · Cluster identity does not depend on the transport: sqlClusterID uses values that are the same for every session on the cluster → done
- T-0191 [done] E5 · mask.Apply has a post-condition and a recover; a masker error message never reaches the operator with the value in it → done
- T-0193 [done] E9 · THREAT_MODEL.md T1: national_id is now a row-path control, not only DDL-literal → done
- T-0194 [done] E9 · internal/plan/ddlliteral.go: strongHit's national_id entry should call the narrower textsig.ValidNationalIDStructured → done
- T-0029 [cancelled] E9 · Before going public: git-crypt the AI-specific paths and rewrite pre-encryption history → cancelled
- T-0089 [done] E5 · T-FAILUX: failure UX and error catalogue drift test → done
- T-0118 [done] E5 · internal/transform: mask an array whose sample arrives as a text literal element-wise → done
- T-0119 [done] E5 · A table-scoped name rule, for refresh_tokens.parent and its kind → done
- T-0127 [done] E5 · internal/plan: drop the arrayArrivesAsLiteral stand-in now that transform masks a literal array element-wise → done
- T-0129 [done] E5 · internal/verify: an array column that arrives as a text literal is not residual-scanned element-wise → done
- T-0130 [done] E5 · Target ownership: a run lease on the target and a lock-and-recheck before every destructive DDL → done
- T-0131 [done] E5 · Events carry no source value: polymorphic inference reports unknown type values by count and keyed digest, and an output-sink canary test proves it → done
- T-0132 [done] E5 · The masker is chosen per FK-connected equality group, not per column → done
- T-0133 [done] E5 · Core owns the run lifecycle: complete is written only after verify passes, and a residual failure empties the target → done
