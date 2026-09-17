# Board · 2026-09-17

| Epic | Phase | Open | In progress | Done | Cancelled | Blocked |
|---|---|---|---|---|---|---|
| E0 Frame | 0 | 0 | 0 | 3 | 0 | 0 |
| E1 Research | 1 | 0 | 0 | 10 | 0 | 0 |
| E2 Architecture | 2 | 0 | 0 | 4 | 1 | 0 |
| E3 Foundations | 3 | 0 | 0 | 10 | 0 | 0 |
| E4 Vertical slice | 4 | 0 | 0 | 20 | 0 | 0 |
| E5 Hardening | 5 | 4 | 1 | 91 | 0 | 0 |
| E6 Launch | 6 | 1 | 0 | 0 | 0 | 0 |
| E9 Later | later | 70 | 0 | 29 | 5 | 0 |

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
- T-0182 [open] E9 · A text column holding a JSON document is refused, not masked ()
- T-0183 [open] E9 · verify does not assert the object its catalog pass refused is gone after the quarantine ()
- T-0185 [open] E9 · ARCHITECTURE.md does not know about --allow-type-literal ()
- T-0195 [open] E9 · classify: national_id's checksum-only formats can weak-ratio-mask an ordinary numeric business key ()
- T-0196 [open] E9 · Migrate the tracker to GitHub Issues and Projects behind the existing tools/tracker.py command surface (sonnet)
- T-0197 [open] E5 · R2-05/A10 residual: a name in a language names.txt does not carry still reports "no name or value signal" as a clean bill of health (sonnet)
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
- T-0214 [open] E9 · A withheld password_command leaves a marker in lazyslice.yml so a later run without one is refused, as a withheld --where already is (sonnet)
- T-0215 [open] E9 · Secret-file and password_command tests pin the bypass shapes, not only the happy attack shapes (sonnet)
- T-0216 [open] E9 · THREAT_MODEL.md T1 does not know --allow-type-literal is recorded in the yml ()
- T-0217 [open] E9 · internal/discover/provision: drop Force from the failed-attempt container removal, fix the retry count's off-by-one, and keep the daemon's port error when the range is exhausted (sonnet)
- T-0218 [open] E9 · ADR-004's 'no unsafe mode' clause lists the types: block among the ways a committed file may reduce enforcement (sonnet)
- T-0219 [open] E9 · --allow-type-literal is recorded only for a type that actually carried a literal a strong validator hit (sonnet)
- T-0220 [open] E9 · internal/discover/password.go: the deadline check reads the derived context, the URL branch of injectPassword swallows a parse error, and the stop reason repeats the prefix (sonnet)
- T-0224 [open] E9 · ARCHITECTURE.md/THREAT_MODEL.md/internal/pg/CLAUDE.md claim EXECUTE on pg_control_system is not granted to PUBLIC, which is false on stock postgres:16 ()
- T-0225 [open] E9 · internal/classify: phoneGuessRaisable applies the same exclusions as raisableUnknown (sonnet)
- T-0226 [open] E9 · internal/classify: matchesAnyGuessRegion computes the candidates once, not once per region (sonnet)
- T-0227 [open] E9 · A phone_region read from a hand-edited lazyslice.yml is validated the way the flag is (sonnet)
- T-0228 [open] E9 · ARCHITECTURE.md's --phone-region flag row and the phone_region_guessed reason fragment describe the removed name-rule corroboration arm (sonnet)
- T-0229 [open] E9 · internal/transform/codes.go: add mask.ErrMaskerFailed to maskReason's known-sentinel list ()
- T-0233 [open] E9 · internal/pg: data_directory is a weak identity field, and a savepoint failure no longer collapses the whole identity (sonnet)
- T-0234 [open] E9 · internal/pg: two code comments still say EXECUTE on pg_control_system is not granted to PUBLIC (sonnet)
- T-0235 [open] E9 · mask: the masker-error wrap test asserts the original error is dropped, not only that the canary is absent (sonnet)
- T-0236 [open] E9 · internal/plan: the special-category rule's loose ends: similar_escape's second argument, the bare 'aids' term, enum default lookup by bare name, and the address false-positive controls (sonnet)
- T-0237 [open] E9 · cmd/lazyslice: renderSafe prints a sentinel's own text rather than its wrapping chain, and internal/load's refusal is value-free without a SQLSTATE (sonnet)
- T-0242 [open] E5 · Before the first drop, the whole target is re-checked for emptiness under the run lease, and a table that appeared since the gate refuses the load (sonnet)
- T-0243 [open] E9 · internal/classify/CLAUDE.md's A2b account is stale after T-0239 ()
- T-0244 [open] E9 · mask: rename the test that says a sentinel passes through unwrapped, since it no longer does (sonnet)
- T-0245 [open] E9 · docs/TORTURE.md and internal/classify comments: the lowered floor can still force a refusal through an equality group's masker (sonnet)
- T-0246 [open] E9 · internal/verify: sequence observation limited to digit-like values, struct comments updated, fixture headers claim only what a revert proves (sonnet)
- T-0247 [open] E9 · internal/pg: data_directory can still decide difference, the discover wiring of the standby rails has no test, and a comment on the role grant is garbled (sonnet)
- T-0248 [open] E9 · docs/READ_ONLY_ROLE.md exists, with the role snippet ARCHITECTURE.md section 9 points at (sonnet)
- T-0249 [open] E9 · internal/core populates Run.TargetTables so the loader's extra-tables path is not dead (sonnet)

## Recently closed

- T-0198 [done] E5 · special_category has no value validator; a digit/name-free special-category sentence still crosses unseen → done
- T-0212 [done] E5 · renderSafe redacts by default, and prints a transform refusal's reason under the values flag → done
- T-0222 [done] E5 · Cluster identity degrades field by field, and two unknown identities are treated as possibly the same cluster → done
- T-0223 [done] E5 · mask.Apply never returns a masker's error verbatim: the module wraps it without the value → done
- T-0230 [done] E9 · T-0229's goal mischaracterises the R2-13 regression; correct it before acting on it → done
- T-0231 [done] E9 · special_category has no value validator in the row-level second net → done
- T-0232 [cancelled] E9 · Curate --unmask flags for gitlab, odoo, discourse and supabase-auth under T-0198's broadened DDL-literal rule → cancelled
- T-0238 [done] E5 · mask.Apply never returns a foreign error object: a masker's error is rebuilt by the module with only its identity carried across → done
- T-0239 [done] E5 · The certain-neighbour rail no longer skips a column declared shorter than sixteen characters: it masks under a generator that fits or refuses → done
- T-0240 [done] E5 · National-identifier corroboration counts a masked personal neighbour at any confidence and outranks the dense-sequence exemption; character-family identifiers get the same path → done
- T-0241 [done] E5 · A streaming standby as source, with the target on its own primary, is refused: a start-time disagreement without the system identifier is unknown, a standby source is announced, and a headless run with no --target refuses on a standby → done
- T-0178 [done] E9 · TestAStoppedContainerIsOfferedAndStarted binds a fixed host port (127.0.0.1:5433) and collides under parallel container runs → done
- T-0184 [done] E5 · Should a headless run auto-select a target on the source's own cluster? → done
- T-0186 [done] E5 · --allow-type-literal is not recorded in lazyslice.yml → done
- T-0192 [done] E5 · The secret file is protected on its resolved path: symlinked parent directories and hard links refuse; password_command is screened before it is written to the yml → done
- T-0213 [done] E5 · Wire --password-command to actually run, or refuse it as unimplemented → done
- T-0221 [done] E5 · Phone numbers in national format, and phone numbers spelled out in words, are recognised: a configured phone region, and corroboration when none is configured → done
- T-0028 [done] E3 · Create GitHub repository Liarea/lazyslice and homebrew-tap, push main, add HOMEBREW_TAP_TOKEN secret → done
- T-0090 [done] E5 · T-PERF: performance baseline and CI throughput guard → done
- T-0163 [done] E5 · the plan-time DDL literal rule does not read index predicates or domain CHECKs → done
- T-0177 [done] E5 · Record real ubuntu-latest bench baseline and flip bench job to blocking → done
- T-0179 [done] E5 · Bench CI gate compares head against its parent on the same runner; absolute baseline becomes a catastrophic floor → done
- T-0180 [done] E5 · mask.Apply has no post-condition: a masker that returns its input is accepted → done
- T-0181 [done] E5 · mask.Apply does not recover: a panicking masker escapes the module → done
- T-0187 [done] E5 · National-identifier validators with checksums in textsig; classify and the second net treat them as strong; a digits-family SSN shape under the ratio rule → done
