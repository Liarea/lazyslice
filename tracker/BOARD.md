# Board · generated 2026-09-23 by tools/tracker.py (source: GitHub project 3 plus tracker/tasks/) — do not hand-edit

| Epic | Phase | Open | In progress | Done | Cancelled | Blocked |
|---|---|---|---|---|---|---|
| E0 Frame | 0 | 0 | 0 | 3 | 0 | 0 |
| E1 Research | 1 | 0 | 0 | 10 | 0 | 0 |
| E2 Architecture | 2 | 0 | 0 | 4 | 1 | 0 |
| E3 Foundations | 3 | 0 | 0 | 10 | 0 | 0 |
| E4 Vertical slice | 4 | 0 | 0 | 20 | 0 | 0 |
| E5 Hardening | 5 | 0 | 0 | 108 | 0 | 0 |
| E6 Launch | 6 | 19 | 0 | 14 | 0 | 0 |
| E9 Later | later | 101 | 0 | 35 | 7 | 0 |

## Open and in progress

- T-0019 [open] E9 · Go vs Python COPY throughput benchmark to validate ADR-001 (opus)
- T-0031 [open] E9 · Licence for the lazyslice.yml schema and docs; confirm copyright holder statement (fable)
- T-0048 [open] E9 · Explicit --key on an uncomparable column type surfaces a raw pgx error instead of a refusal (opus)
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
- T-0157 [open] E9 · Performance: the 20,000,000-row child run and byte-budget accounting (0.2) (sonnet)
- T-0158 [open] E9 · Export the closed-value label list from mask so plan compares labels, not CHECK text ()
- T-0159 [open] E9 · FK equality group: members with different type families can still mask differently ()
- T-0162 [open] E9 · move the SQL literal scanner out of internal/pipeline into a leaf package beside internal/textsig ()
- T-0164 [open] E9 · uniqueColumn is copied in internal/plan and internal/transform; give it a shared home ()
- T-0166 [open] E9 · wire dsn param-drop warnings into internal/core and internal/pg's own dsn.Parse call sites ()
- T-0168 [open] E9 · dsn.Ref.Params misses rung-2 (env/PGSERVICE) settings, so a first run through libpq env alone reruns with no sslmode at rung 0 ()
- T-0173 [open] E9 · Stop.Args is never populated by core.wrap, so many transcript error lines render generic while only the final exit line is specific (opus)
- T-0174 [open] E9 · Wire --debug to print the statement trace (Source.Trace) on an ordinary failure ()
- T-0175 [open] E9 · Give nasty.sql's stream fixtures a size parameter for perf profiling ()
- T-0176 [open] E9 · mask.Apply's HMAC-SHA256 derivation is the largest CPU cost in the extract/transform/load pipeline ()
- T-0182 [open] E9 · A text column holding a JSON document is refused, not masked ()
- T-0183 [open] E9 · verify does not assert the object its catalog pass refused is gone after the quarantine ()
- T-0185 [open] E9 · ARCHITECTURE.md does not know about --allow-type-literal ()
- T-0195 [open] E9 · classify: national_id's checksum-only formats can weak-ratio-mask an ordinary numeric business key ()
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
- T-0243 [open] E9 · internal/classify/CLAUDE.md's A2b account is stale after T-0239 ()
- T-0244 [open] E9 · mask: rename the test that says a sentinel passes through unwrapped, since it no longer does (sonnet)
- T-0245 [open] E9 · docs/TORTURE.md and internal/classify comments: the lowered floor can still force a refusal through an equality group's masker (sonnet)
- T-0246 [open] E9 · internal/verify: sequence observation limited to digit-like values, struct comments updated, fixture headers claim only what a revert proves (sonnet)
- T-0247 [open] E9 · internal/pg: data_directory can still decide difference, the discover wiring of the standby rails has no test, and a comment on the role grant is garbled (sonnet)
- T-0248 [open] E9 · docs/READ_ONLY_ROLE.md exists, with the role snippet ARCHITECTURE.md section 9 points at (sonnet)
- T-0249 [open] E9 · internal/core populates Run.TargetTables so the loader's extra-tables path is not dead (sonnet)
- T-0250 [open] E9 · Pin T-0197's nothing_recognised/sub_threshold_signal split with a regression test ()
- T-0256 [open] E9 · The absence-of-evidence sentence covers every family the validators ran over, and pluralises its count (sonnet)
- T-0261 [open] E9 · internal/core/CLAUDE.md's T-0241 section is stale after T-0255 ()
- T-0262 [open] E9 · The special-category rule pins the LIKE spelling at plan level, and its normalisation comment matches the code (sonnet)
- T-0264 [open] E9 · internal/testutil's restart helper: capture the original container by value, report which container to clean up on every return, and normalise a typed-nil replacement (sonnet)
- T-0265 [open] E9 · verify's per-check pass lines never reach the terminal or --json ()
- T-0266 [open] E9 · The second net's surrogate-key exemption is measured against a key with no sequence or identity default (opus)
- T-0267 [open] E9 · Workflow scripts carry the checkout's absolute home path in a REPO constant (sonnet)
- T-0268 [open] E9 · lazyslice doctor prints the stated false negatives, as ARCHITECTURE.md says it does (sonnet)
- T-0272 [open] E9 · Per-leaf categories for JSON documents on the Decision, so an email leaf is replaced by a fake email rather than filler (opus)
- T-0274 [open] E9 · The root decision line and the plan's own RootReason give different reasons when a name preference breaks a tie (sonnet)
- T-0275 [open] E9 · tracker.py: close writes the archive file before it closes the issue, and every gh call has a timeout (sonnet)
- T-0276 [open] E9 · tracker.py: five small robustness findings from T-0196's review (sonnet)
- T-0277 [open] E9 · The generated cask's quarantine hook uses postflight, which Homebrew deprecates in favour of postflight_steps (sonnet)
- T-0278 [open] E9 · Release notes name the range as 'since the first commit' when the previous tag is the root (sonnet)
- T-0282 [open] E6 · Write the 'why I built this' paragraph for the README (human)
- T-0291 [open] E9 · Makefile's .SHELLFLAGS (-eu -o pipefail) is silently ignored by macOS's GNU Make 3.81 (sonnet)
- T-0292 [open] E9 · person_name role masking can collide with real name corpora on ordinary tables ()
- T-0295 [open] E9 · second net's dictionary rule has no single-word check, and T-0287 makes that shape normal (sonnet)
- T-0296 [open] E9 · tracker.py retries a GitHub GraphQL secondary rate limit with backoff instead of failing the write (sonnet)
- T-0299 [open] E9 · internal/classify: two stale anyMatched comments and a Pagila test that no longer proves the samples look like secrets (sonnet)
- T-0300 [open] E9 · A name column whose values carry digits is masked as an address (sonnet)
- T-0301 [open] E9 · A go install build prints 'lazyslice dev' instead of its module version (sonnet)
- T-0304 [open] E6 · Masked given names, surnames, full names and email local parts draw from the Census lists (ADR-015, mask half; mask/v0.3.0) (opus)
- T-0305 [open] E9 · person_date refuses correct runs by cross-row coincidence and by the keyed self-draw (opus)
- T-0306 [open] E9 · A masked enum with more than 128 labels reaches the residual filter and refuses on a true statement about its domain (sonnet)
- T-0307 [open] E9 · A second name corpus (INSEE, ONS) and a run-time locale choice for masked names (sonnet)
- T-0308 [open] E9 · internal/textsig/names.txt's English section could be regenerated from the Census CSVs (sonnet)
- T-0309 [open] E9 · tools/names: assert no duplicate rows, a filled cut, and a real drops test (sonnet)
- T-0310 [open] E9 · A minimal docs site (mkdocs-material on GitHub Pages) once the first ten issues show what strangers look for (sonnet)
- T-0311 [open] E6 · The neighbouring-column sweep spares enum-like, identifier-shaped and unique-indexed columns (opus)
- T-0312 [open] E6 · The same-column-name rule does not propagate a decision that was itself only a neighbour sweep (sonnet)
- T-0313 [open] E6 · A bare 'name' column and a '*_file_name' column are not a person's name without corroboration (opus)
- T-0314 [open] E6 · Framework metadata tables are copied whole and never masked (schema_migrations, ar_internal_metadata and their kin) (sonnet)
- T-0315 [open] E6 · The entropy validator does not read filenames, hex digests, namespaced class names or a handful of samples as secrets (sonnet)
- T-0316 [open] E6 · The Luhn check needs a card length and issuer prefix before it masks an id, number or version column (sonnet)
- T-0317 [open] E6 · A four-part version string is not an IP address, and a digit-only license key is not a phone number (sonnet)
- T-0318 [open] E6 · The plan reports every refusal in one run, and a no-identity hint names the columns (sonnet)
- T-0319 [open] E6 · After a second-net refusal the operator can mask the column: --mask, and every failing column reported at once (opus)
- T-0320 [open] E6 · A green run prints one verify summary line and how to reach the target (sonnet)
- T-0321 [open] E6 · The reasons dump and the plan end with a summary, and a re-run from a committed yml is quiet (sonnet)
- T-0322 [open] E9 · Infer foreign keys from naming conventions as virtual_fks candidates, so a Rails schema is reachable (opus)
- T-0324 [open] E9 · Geo leaves inside JSON get in-range numbers (sonnet)
- T-0325 [open] E6 · --unmask on a first run must not put it on the re-run path; the drift message names the real decision (sonnet)
- T-0326 [open] E6 · A stray positional argument is a usage error, not 'the source did not report a server version' (sonnet)
- T-0327 [open] E6 · A committed yml whose target is a container lazyslice created reconnects or re-provisions, and says which (opus)
- T-0328 [open] E6 · The free_text masker fits its output to the input's length (opus)
- T-0329 [open] E9 · internal/core/core_test.go's recorder Residual double needs AddEmitted and Emitted (T-0302 blocks make check) (haiku)
- T-0330 [open] E9 · internal/plan's chooseGroupMasker overwrites every masked column's masker with mask.Pick's default, including one the yml or a build named (sonnet)
- T-0331 [open] E6 · Decide whether a first run may ask both the target question and the root question (human)
- T-0332 [open] E9 · internal/invariants/CLAUDE.md's contract says the suite imports only internal/testutil of ours; I2 now imports the mask module (T-0302) (haiku)
- T-0333 [open] E6 · --create-target's container name doubles the prefix for a directory named lazyslice-* (sonnet)

## Recently closed

- T-0064 [done] E5 · Dogfood: two sessions against a real project of the maintainer's choosing, logged in docs/DOGFOOD_LOG.md → done
- T-0283 [done] E6 · Decide whether v0.2.0 ships a docs site (mkdocs-material on GitHub Pages) or the in-repo docs stay the docs → done
- T-0303 [done] E6 · tools/names regenerates the masker's name lists from the 2020 Census top-1000 files (public domain), checked in CI → done
- T-0323 [done] E9 · Decide the free_text masker's length policy for short enum-like values → done
- T-0065 [done] E6 · 20-second VHS GIF of the first run on Pagila → done
- T-0155 [done] E5 · v0.0.1 proves the release pipeline end to end: goreleaser, the tap cask, brew install prints a version → done
- T-0196 [done] E5 · Migrate the tracker to GitHub Issues and Projects behind the existing tools/tracker.py command surface → done
- T-0269 [done] E6 · A timestamp column's reason line says its samples look like secrets → done
- T-0279 [done] E6 · README as the landing page: GIF first, install, the run, why, a verified comparison table, how PII is decided, the rails → done
- T-0280 [done] E6 · Launch-post drafts in docs/launch/: Show HN, r/PostgreSQL, r/devops, r/webdev, a blog post from the post-mortems, and ten places a listing PR is welcome → done
- T-0281 [done] E6 · Issue templates that keep personal data out of reports, and CHANGELOG.md as the pointer at releases → done
- T-0284 [cancelled] E9 · go install of lazyslice fails: go.mod's mask replace directive rejects @v0.1.0/@latest → cancelled
- T-0285 [done] E6 · go install works: the mask module gets its own tag and go.mod requires it by version, with go.work for local development → done
- T-0286 [done] E6 · research/POSTMORTEMS.md carries two stale facts the blog draft inherited → done
- T-0287 [done] E6 · The person_name masker respects the column's role: a first-name column gets a given name, a last-name column a surname → done
- T-0288 [done] E6 · A root table can hold more rows than --take names; say so in the flag's help or stop it → done
- T-0289 [done] E6 · make gif records with a read-only role, probes readiness from the host, and pace.awk fails when it paused nothing → done
- T-0290 [done] E6 · goreleaser release build still compiles mask/ from go.work, not the tagged version go.mod requires → done
- T-0293 [done] E9 · plan equality groups ignore person_name Role, letting an FK pair mask two roles alike → done
- T-0294 [done] E9 · plan-time DEFAULT rewrite for a person_name column ignores Role → done
- T-0297 [done] E6 · A phone column's reason line says its digits parse as MAC addresses → done
- T-0298 [cancelled] E9 · Add a digit-run/phone-without-plus signal so plain 10-12 digit columns are not left to Luhn chance → cancelled
- T-0083 [done] E5 · Target type registration for CopyFrom: no owner since internal/load shipped → done
- T-0170 [done] E9 · torture regression 013 (json-object-key-email) fails on main → done
- T-0197 [done] E5 · R2-05/A10 residual: a name in a language names.txt does not carry still reports "no name or value signal" as a clean bill of health → done
