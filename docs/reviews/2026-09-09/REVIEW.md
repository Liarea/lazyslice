# Lazyslice project review — 2026-09-09

**Verdict: keep developing the tool, but do not treat the current hardening gate as evidence that its safety promise is satisfied.** The project has a substantial implementation, useful tests, and sensible constraints. Its largest problems are incomplete safety contracts across stages, a mismatch between automatic masking and application usefulness, and a roadmap that deferred direct user evidence while accumulating machinery.

This review covers the checkout at `936ceec5c3a45bd7cda660546ea1a93834fa5f88`, the nested `mask` module, the core pipeline and database boundaries, test harnesses, CI/release configuration, concept, architecture, threat model, ADRs, roadmap, tracker, operating model, and selected research and workflow material. GitHub inspection covered `Liarea/lazyslice` and its referenced `Liarea/homebrew-tap`. This is a broad adversarial review with targeted execution, not a claim that every line or every research citation was audited. No production database was used. Implementation files, accepted decisions, and the roadmap were left unchanged.

**Evidence.** `make check` passed, including lint, raced tests in both modules, generated-document checks, and tagged vet. `make torture` passed in 65.8 seconds. The complete local `make integration` invocation failed in four tests during Docker setup: one fixed-port conflict and three missing port mappings. The invariant package passed. On isolated/sequential reruns, discovery, load and pooler tests passed; `TestTwoExtractionsOverOneSnapshotAgree` still failed while obtaining its container's port, before its extraction assertion. The full suite therefore remains unverified here. GitHub's latest CI run did not start its jobs because of an account payment/spending-limit restriction. That is an external blocker, not proof of a code failure. [Latest CI run](https://github.com/Liarea/lazyslice/actions/runs/34317022483), [last successful CI found](https://github.com/Liarea/lazyslice/actions/runs/34185450720).

The seven targeted runtime probes below used invented data in disposable PostgreSQL 18.6 containers. Six demonstrate implementation or safety-contract problems; the JSON-key probe demonstrates an explicitly accepted limitation that deserves reconsideration. Their results and logs are retained in [results.json](<repo>/docs/reviews/2026-09-09/evidence/results.json).

## Findings to address before calling the output safe

**1. [P1, reproduced] Target eligibility expires before the destructive operation.**

`Target.Gate` checks emptiness and releases its connection. Core then introspects, classifies, and plans before obtaining a writer and dropping tables. Nothing locks the target against another application or another lazyslice run through that interval. The `gated` state is a remembered decision, not continuing ownership of the target.

I started with an empty, unmarked target table, held up source introspection, waited for the target-selection event, inserted row `999` into the target, and released the source. Lazyslice deleted row `999`, loaded its own row, and exited **0**. This is a successful run deleting data that did not exist when deletion was approved.

Fix the ownership model, not just the timing of one check. Serialize lazyslice runs with a target-scoped lease; protect the final emptiness check and destructive DDL in one lock/transaction boundary; or load into a dedicated staging database/schema and promote an explicitly owned result. An advisory lock alone coordinates only cooperating clients and cannot prevent an application's ordinary INSERT. PostgreSQL documents that distinction in its [locking reference](https://www.postgresql.org/docs/current/explicit-locking.html).

Code: [gate](<repo>/internal/pg/target.go:198), [gate call](<repo>/internal/core/run.go:622), [writer acquisition](<repo>/internal/core/run.go:1314), [drop](<repo>/internal/load/load.go:278). Evidence: [race.log](<repo>/docs/reviews/2026-09-09/evidence/race.log).

**2. [P1, reproduced] Polymorphic inference leaks flagged source values into ordinary logs.**

A table with `thing_id` and `thing_type` triggers inference. When a type value cannot be mapped to a table, `showValue` truncates and quotes it, and core emits that string as a warning. It does not matter that classification has already decided to mask that column.

The probe put `poly.canary@example.org` in `thing_type`. The target held a masked email, but the normal transcript printed the original in `value "poly.canary@example.org" maps to no table`; exit **0**. The supposedly value-free event boundary is bypassed by putting data into a string called a reason. Restrict inference to trusted schema vocabulary and report identifiers/counts or keyed fingerprints for unknown values. Add runtime canaries to every output sink; static field-type checks cannot prove a string contains no source value.

Code: [raw value added to the plan](<repo>/internal/plan/polymorphic.go:290), [quoting](<repo>/internal/plan/polymorphic.go:370), [event emission](<repo>/internal/core/run.go:1182). Evidence: [polymorphic_log.log](<repo>/docs/reviews/2026-09-09/evidence/polymorphic_log.log).

**3. [P1, reproduced] Choosing a wider unique masker breaks foreign keys.**

Classification propagates categories along foreign keys. Later, planning changes the masker on a unique column independently of its referencing columns. Equal inputs then receive different outputs.

The minimal schema was `tokens(id PRIMARY KEY, token TEXT UNIQUE)` and `items(id PRIMARY KEY, token TEXT REFERENCES tokens(token))`, with one matching value. The parent became `lazyslice-invalid-snvurpjganvkv`; the child became `$lazyslice$invalid`. Loading ended at **exit 8** because the FK no longer resolved.

Plan masking over connected sets of columns whose equality must survive. Pick one compatible algorithm, canonicalization, domain and output representation for the whole set, using all participating constraints. If no such mapping exists, refuse before writes. Merely propagating the category, or rerunning classification after each individual mutation, is insufficient.

Code: [FK propagation](<repo>/internal/classify/classify.go:1065), [individual masker replacement](<repo>/internal/plan/unique.go:138), [transform consumes the decision](<repo>/internal/transform/transform.go:145). Evidence: [fk_masker.log](<repo>/docs/reviews/2026-09-09/evidence/fk_masker.log).

**4. [P1, reproduced] A failed verification leaves a completed marker and leaked data.**

The loader writes `status = complete` before core starts verification. A verification error does not change that status. In a two-row text column containing one email and one ordinary string, verification correctly returned **exit 9**, but the email remained in the target and its latest marker said **complete**.

Core should own the run lifecycle. Distinguish loading, loaded-but-unverified, verified, and failed states, and write successful completion only after required checks finish. A failed target should be clearly quarantined or marked failed. The CLI exit code currently tells the truth while persistent database metadata tells a different story. This contradicts the threat model's general empty-or-running/failed failure property.

Code: [premature completion](<repo>/internal/load/load.go:184), [subsequent verification](<repo>/internal/core/run.go:1405), [failure contract](<repo>/THREAT_MODEL.md:128). Evidence: [verify_failure.log](<repo>/docs/reviews/2026-09-09/evidence/verify_failure.log).

**5. [P1, reproduced] Masking rows leaves sensitive literals in recreated DDL.**

I used an email column with a default of `ddl.canary@example.org` and a different email in its row. The row was masked, the original default survived in `pg_attrdef`, and the run exited **0**. A subsequent INSERT using the default can materialize the original value again. Row scans cannot establish that the database artifact contains no sensitive literals.

Bring recreated defaults and expressions into the data boundary. Inspect or refuse sensitive literal-bearing defaults, generated expressions, constraints, and type definitions; define narrowly which forms are safely preserved. This is not a reason to strip every default: many are essential to application behavior. It is a reason to make the distinction explicit and test the catalog as well as rows.

Code: [default copied verbatim](<repo>/internal/load/ddl/ddl.go:642), [verification checks](<repo>/internal/verify/verify.go:164). Evidence: [ddl_default.log](<repo>/docs/reviews/2026-09-09/evidence/ddl_default.log).

**6. [P1, traced in code] Config round trips discard explicit TLS settings.**

`dsn.Ref` retains only host, port, database and user. Emit serializes those fields; discovery reconstructs a URI without the original transport parameters. A first run with `sslmode=verify-full` therefore does not preserve that requirement in the emitted configuration. On a rerun, the pinned pgx implementation defaults an unspecified mode to `prefer`, subject to environment/service overrides. Certificate paths and other connection options are also lost. The exact transport outcome depends on the server and environment; I did not run a TLS interception test.

Separate printable identity from a reusable connection specification. Preserve an allowlist of non-secret security options and references to secret material, or require a stable external DSN reference. A secret-free config must still preserve the user's authentication and transport requirements.

Code: [reference fields](<repo>/internal/dsn/dsn.go:70), [endpoint serialization](<repo>/internal/emit/document.go:459), [reconstructed URI](<repo>/internal/discover/discover.go:530), [pinned driver default](<home>/go/pkg/mod/github.com/jackc/pgx/v5@v5.10.0/pgconn/config.go:787).

**7. [P1, reproduced design gap] The second net recognizes rare personal values and deliberately passes them.**

One email plus nineteen `ok` values in a generic text column survived unchanged with **exit 0**. One email plus two `ok` values also passed. Reducing that latter table to one email plus one `ok` produced **exit 9**. The scan is exhaustive, but its verdict is driven by an 80% column ratio once there are three non-null values.

An 80% threshold can be useful for inferring a column's semantic category. It is a poor threshold for determining whether an already-loaded database contains a recognized source email. A single strong hit is still a hit. Treat precise parsers differently from ambiguous heuristics, add source-aware confirmation where needed, and mask/refuse mixed columns on strong evidence. Do not solve this by globally lowering every heuristic threshold: surname dictionaries and address guesses will create different failures. The important distinction is category inference versus residual detection.

Code: [classification ratios](<repo>/internal/classify/classify.go:450), [verification ratio](<repo>/internal/verify/secondnet.go:204), [three-value threshold](<repo>/internal/verify/validators.go:44). Evidence: [sparse_email.log](<repo>/docs/reviews/2026-09-09/evidence/sparse_email.log).

**8. [Accepted limitation, reproduced] Preserving arbitrary JSON keys is too broad for the product promise.**

`{"canary.person@example.org":"ok"}` retained the email key, replaced its value, and exited **0**. This is explicitly disclosed in SECURITY.md, so it is not a newly discovered implementation deviation. It is still a design decision worth rejecting for identity-keyed maps, contact indexes, or arbitrary user payloads.

Offer a conservative whole-document replacement for arbitrary JSON, with explicit preservation of known application structures when needed. Detecting email-shaped keys alone will not cover arbitrary personal identifiers. The same choice highlights the product tension: preserving structure can preserve personal data; replacing all structure can break the application. That requires an explicit contract and user evidence.

Code: [keys retained](<repo>/internal/transform/json.go:246), [known limitation](<repo>/SECURITY.md:75). Evidence: [json_keys.log](<repo>/docs/reviews/2026-09-09/evidence/json_keys.log).

**9. [P2, traced in code] The memory budget is not an estimated resident-set limit.**

Planning compares selected-key memory plus estimated Bloom-filter memory with the budget. It does not include stored samples, pending traversal work, the raw and masked channels, decoded JSON, or batch contents. Extraction batches 2,000 rows regardless of byte size. Even one batch of 2,000 one-MiB values can require roughly two GiB before transformation overhead. Text and document width estimates are fixed small constants. There is no demonstrated general RSS bound here; I did not provoke an OOM.

Use byte-limited batches, bounded handling/refusal of very large cells, and an accounting model that includes samples and pending work. Measure peak RSS with wide rows and skewed JSON, not only millions of narrow rows. Until then rename the flag/help to the quantity it actually budgets. The source snapshot remains open while slow transformation/loading backpressures extraction, so the fixed rows-per-second hold estimate also needs measured limits.

Code: [budget calculation](<repo>/internal/plan/plan.go:801), [row batching](<repo>/internal/extract/extract.go:307), [width estimates](<repo>/internal/plan/query.go:187), [snapshot release](<repo>/internal/core/run.go:1337).

**10. [P2, traced in code] `mapping_file` is an advertised escape hatch without a consuming implementation.**

The config decoder reads it, emit preserves it, and unique-domain refusals recommend it. I found no production code that reads the mapping CSV or applies its replacements. `Decision` carries no mapping, transform receives decisions, and core passes no mapping paths to `repo.Protect`. Configurable masker fields likewise should be checked against what the runtime actually honors rather than assumed executable because they round-trip.

Either implement and test the full mapping contract, including uniqueness, missing values, FK consistency and secret-file protection, or reject the field explicitly and remove the proposed remedy. Silently retaining unsupported configuration creates false confidence. This is also evidence that the listed remaining hardening tasks do not exhaust the specified v1 work.

Code: [mapping contract](<repo>/internal/pipeline/config.go:81), [decoder](<repo>/internal/emit/document.go:293), [recommended remedy](<repo>/internal/plan/unique.go:165), [final masker assignment](<repo>/internal/classify/classify.go:1301), [protection call](<repo>/internal/core/run.go:1539).

**11. [P2, direct test defect] The torture suite takes its source baseline after running lazyslice.**

`runTool` runs at line 56; the supposed before-row and before-catalog fingerprints are collected at lines 72–73. They are compared against another post-run read. This cannot detect mutations made during the run. Other source-safety tests exist and the main invariant suite still matters, but the torture suite's claimed I4 evidence is weaker than documented.

Capture both baselines before execution, and add a negative control proving that an injected source mutation fails this test. This is precisely the kind of defect a self-consistent passing suite can hide.

Code: [test ordering](<repo>/internal/invariants/torture_test.go:56).

## Architecture decisions I would challenge

**Keep the single Go binary, local operation, explicit source transactions, narrow runtime extension surface, and standalone masker tests.** These choices reduce deployment and secret-egress complexity. The source tracer is useful supporting evidence; the read-only transaction and appropriately privileged source role remain the stronger controls. I found no reason to rewrite the project in another language or turn it into a service.

**Replace stage-local safety facts with one compiled execution contract.** The nine stages are a reasonable description of work, but their interfaces do not enforce the most important obligations. A source/target choice is approved earlier than use; classification is mutated by planning; a loader declares success before verification; type families and array/domain handling are reconstructed separately in classify, transform, verify and plan. Comments in these packages repeatedly acknowledge the duplicated vocabulary and owed handoffs.

Introduce a shared normalized type description and an immutable prepared plan containing equality groups, maskers, writable constraints, catalog policy, budgets and target ownership. Give load only a validated target capability and prepared plan. Keep orchestration responsible for lifecycle status. Do this incrementally around the reproduced failures rather than by inventing a new generic framework.

**The engine abstraction is aspirational.** ADR-003 says engine-specific code lives in `internal/pg`; PostgreSQL catalog SQL, key queries, casts, schema reconstruction and verification are spread across several stage packages. Adding MySQL is not just another implementation of today's `Source`. Describe the current design as PostgreSQL-specific. Extract an engine backend only after a real second-engine requirement reveals the necessary boundary. [ADR-003](<repo>/docs/adr/003-database-order.md:29), [planner SQL](<repo>/internal/plan/sql.go), [DDL reconstruction](<repo>/internal/load/ddl/ddl.go).

**Revisit how much PostgreSQL DDL this project should own.** Omitting views, functions, triggers and policies narrows the supported application set, while reconstructing types, defaults, indexes and sequences still incurs much of a schema-dump implementation's complexity. A table-loading demo cannot validate that tradeoff. Choose a supported schema contract based on dogfood. Evaluate a standard-tool-assisted path only if the compatibility benefit justifies revisiting the no-native-dependencies principle; do not blindly replay arbitrary schema code to solve it.

**The residual scanner cannot prove an entire database is safe.** It tests only selected masking decisions, intentionally excludes some domains, scans a limited vocabulary, and confirms Bloom hits against a later source snapshot. A source value deleted between extraction and confirmation can be indistinguishable from a filter false positive. These are explicit design choices, not independent privacy guarantees. Consider whether exact keyed digests for a bounded subset would simplify verification enough to justify their memory cost. Either way, expose coverage and exclusions as part of the result.

**A `verify` command should be observational.** The current command drops and reloads the target; its help honestly says so. Requiring a new extraction because the residual filter is ephemeral explains the implementation, but does not make that user interface appropriate. Rename the destructive operation to reflect rerunning, and provide an explicitly limited read-only check of the current target. A developer investigating a suspicious copy should not destroy the evidence as the default verification action. [Command definition](<repo>/cmd/lazyslice/main.go).

## Product and roadmap challenges

**“Zero config,” “safe by default,” and “realistic enough to run the application” are competing objectives.** A blanket JSON rewrite changes feature settings and embedded references; broad name heuristics mask application vocabulary; invalidated credentials impede login; dropped schema objects change application behavior. These may be acceptable, but they cannot all be decided from catalog correctness. The first-run contract should be something like “automatic proposal, conservative handling, explicit review of exceptions, reproducible policy.” A useful blocking question is preferable to silently making a consequential decision to meet a one-question quota.

**The torture suite is a valuable regression corpus, not validation of the zero-config claim.** Its documented result is nine successful schemas and one refusal with **19 unmask flags, seven skips and one explicit key**. The source schemas are real, while rows are generated and some relevant columns are empty. “Clean” primarily establishes selected relational properties and email/phone canary absence. The three truth sets have **66 labelled personal columns**, with **65 detected and 35 false positives** in the recorded measurement. That precision/recall is column-level on a small, repeatedly repaired corpus, not an estimate for arbitrary production data. [Measurements and definitions](<repo>/docs/TORTURE.md:337).

**A known credential leak belongs ahead of feature completion.** The current truth set still records `auth.refresh_tokens.parent` being copied, and a test intentionally pins that known miss. Recording a known failure is useful; accepting it as compatible with a safety gate is the problem. The red-team criterion “nothing leaks unmasked flagged data” excludes classifier misses by construction. Replace it with explicit coverage of independently labelled sensitive data, including credentials, identifiers, JSON keys, DDL literals, mixed-value columns, and output logs. [Known miss](<repo>/docs/TORTURE.md:454), [gate](<repo>/ROADMAP.md:57).

**Dogfood should precede the next architectural expansion.** Gate 4 was declared closed while its two real-project sessions remained undone. That deferred the evidence most capable of disproving the design. Run one representative application through a copy, then prove login, a meaningful read, a meaningful write, and a second snapshot after schema drift. Record every override, skipped table, broken workflow, manual action and minute spent. Use reviewed or synthetic sensitive data until the demonstrated safety gaps are fixed.

**A deterministic first-N slice is reproducible, but may be the wrong slice.** Default key ordering favors early rows and does not target the records that reproduce the user's bug. Graph centrality is not business importance. Parent completeness is not tenant isolation: shared parents and cross-tenant references can widen the result. “Why included,” boundary crossings, cap omissions, and expected application scenarios should be visible before copy. Keep simple defaults, but validate whether the actual job is “seed my environment,” “reproduce this incident,” or “generate CI data”; their best defaults differ.

**The research is a hypothesis generator.** Two predecessor outcomes do not establish that every commercial version of this problem is unviable, nor that a free local implementation will have sustained users. Complaint frequency in selected issue trackers is not market prevalence. Conversely, lack of funding demand is not a reason to abandon a useful personal tool. Keep the no-service decision if it matches the maintainer's goals; assess it as a deliberate scope choice rather than a universal market theorem. The underlying research itself often states these sampling caveats.

**The differentiation needs a measured result.** Greenmask's own current material already describes subsetting and deterministic transformations. Lazyslice's hypothesis is lower setup burden with understandable safety decisions. Compare time to a working application on the same schemas, with all required overrides and dependencies counted. If this does not improve that experience, more classification rules alone do not establish a reason to switch. [Greenmask capabilities](https://www.greenmask.io/), [subsetting documentation](https://docs.greenmask.io/latest/database_subset/).

**The agent operating model creates coordination costs that should be measured.** The repository tracks 231 Markdown files; the operating model records a 3.5-million-token research run and a later usage-budget correction. File count alone says nothing about quality. The meaningful evidence is repeated cross-package defects, partial landings, documentation owed to other owners, and contradictory active rules. For example, the correction to one reviewer remains next to a blanket three-reviewer policy. Directory isolation has become a constraint on fixing end-to-end behavior. Use tasks scoped to one observable invariant across all necessary files, one accountable owner, and an independent adversarial reproduction for high-risk changes. Treat changed-line limits as review-size guidance, not a reason to split a safety fix into temporarily incompatible stages. [Operating model](<repo>/docs/OPERATING_MODEL.md:29).

**Collapse the sources of truth.** README and SECURITY still claim every stage is a no-op; ROADMAP includes old unchecked gates beside a current closure; BUILD_PLAN is retained as sequencing authority while being declared architecturally obsolete; local CLAUDE files contain additional decisions. Keep one current roadmap, a short current architecture, and historical ADRs. Archive superseded prompt plans and move implementation notes out of live policy. A readable README is needed for private dogfood now, not only for launch.

**Release reality is behind the stated gates.** At inspection, the code repo was private, the public Homebrew tap was empty, no GitHub releases were listed, and the main branch's `protected` flag was false. The current GitHub CI is blocked before execution. The regular CI invokes integration but not `make torture`, whose tests require both build tags. The release workflow invokes `make check`, not the database suites, so a newly tagged commit need not have passed the safety tests it claims to rely on. Add an explicit tested-commit release gate, preserve practical local verification, and provision the external release dependencies before claiming installability. Do not increase the CI matrix while its current jobs cannot start. [CI](<repo>/.github/workflows/ci.yml:218), [release gate](<repo>/.github/workflows/release.yml:38).

**Do not make four engines or community counts the next definition of success.** The ADR already requires a stability interval and demand before another engine; the roadmap's fixed four-database milestone can pressure the project away from that discipline. Unsolicited PRs and posts are outcomes, not tasks the implementation controls. Better next measures are successful repeat runs, reduced intervention, application workflows preserved, and users choosing the tool a second time. Keep the mask module separately testable, but defer a separate library growth strategy until someone consumes it.

## Replacement sequence I recommend

These are proposed priorities, not changes to accepted ADRs or the saved roadmap.

1. **Close the demonstrated safety failures.** Target ownership, log egress, equality-group masking, lifecycle status, DDL literals and transport-config preservation. Decide and implement the mixed-value and arbitrary-JSON policy. Fix the source-baseline test. Gate: the minimal probes become regressions and pass with the intended safe behavior; no known populated credential miss remains in the accepted corpus.
2. **Establish a supported, useful workflow.** Complete two real-project dogfood sessions with explicit data review. Gate: the copied application starts and completes named read/write/login scenarios, all exceptions are recorded, and the second run is understandable after schema drift. Measure zero-override success separately from operator-assisted success.
3. **Harden only what those workflows require.** Finish array coverage across classify/plan/transform/verify, resolve or reject unsupported config fields, normalize type handling, measure byte budgets and snapshot hold time, and make refusals actionable. Gate: seeded adversarial cases across types and constraints, plus a held-out corpus not used to tune the classifier; report runtime and memory percentiles and intervention counts.
4. **Release a narrow PostgreSQL beta.** Fix live docs, restore CI execution, require the tested commit, verify one clean-machine install and one unattended run, and publish explicit supported/unsupported behavior. Gate: a new user can get a useful copy and repeat it without maintainer intervention, while knowingly accepting documented residual risks.
5. **Let demonstrated usage decide expansion.** Another engine, richer mappings, key remapping, a library release, and any hosted work each need their own user evidence. Do not pre-commit to delivering all of them.

The product question to resolve next is concrete: **can a developer get a copy that is both acceptable to handle and useful for their actual application, with less intervention than their current approach?** The current gates prove substantial engineering progress, but do not yet answer that question.

## Reproduction and limits

Build the reviewed binary with `go build -o /tmp/lazyslice-review ./cmd/lazyslice` from the repository. [probes.py](<repo>/docs/reviews/2026-09-09/evidence/probes.py) runs the synthetic masking, DDL, lifecycle, FK and log probes. [target_race.py](<repo>/docs/reviews/2026-09-09/evidence/target_race.py) reproduces the target race. Both use a disposable `postgres:18` container with a random loopback host port, a fixed test-only key, scratch directories, and cleanup in `finally`. They require Docker and the built binary. These scripts demonstrate the reviewed behavior; they are not committed regression tests or a patch.

Stored suite logs: [check](<repo>/docs/reviews/2026-09-09/evidence/check.log), [integration](<repo>/docs/reviews/2026-09-09/evidence/integration.log), [torture](<repo>/docs/reviews/2026-09-09/evidence/torture.log), [discovery rerun](<repo>/docs/reviews/2026-09-09/evidence/discover-rerun.log), [sequential rerun](<repo>/docs/reviews/2026-09-09/evidence/sequential-rerun.log). TLS/config, memory accounting, unused mapping fields, and the incorrect torture baseline are code-trace findings, not claims of additional runtime demonstrations. No claim is made that this review exhausted all defects, benchmarked competing tools, or established application compatibility.
