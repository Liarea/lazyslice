# lazyslice threat model

Written 2026-09-05 against docs/adr/001 to 006 and ARCHITECTURE.md; revised the same day after the phase 2 adversarial review (tracker T-0018). Each threat names the research that evidences it, its likelihood and impact, the concrete control we build, and whether that control blocks v1. "Blocks v1" means the phase 4 gate cannot be ticked without it (docs/BUILD_PLAN.md Gate 2 requires at least three; this document has thirteen threats and every one carries a blocking control). The phase 5 red team (docs/BUILD_PLAN.md PROMPT 5.4) updates this file; nothing is removed from it, only marked.

The promise this document defends is CONCEPT.md's: the output is pseudonymised, not anonymised. Deterministic masking preserves frequency; a snapshot is personal data under GDPR Art. 4(5) and must be regenerated after an erasure request (research/HARD_PROBLEMS.md §2.3). The README says so in those words.

## Assets

| Asset | Where it is | Who must not get it |
|---|---|---|
| A1 Production row values | the source database; in the process between extract and transform; in the `RowBatch` channels | anything outside the process: logs, events, the yml, the terminal, a masker we did not write, the network |
| A2 The target database | the developer's local or CI Postgres | it must never be production, and must never be half-loaded and green |
| A3 Source credentials | the DSN, `.env`, `PGPASSWORD`, `~/.pgpass`, container env, `--password-command` output | the yml, events, `--json`, `lazyslice_meta`, error text |
| A4 The masking key `K` | `./lazyslice.secret` or `$LAZYSLICE_SECRET` | git, the yml, the target, events; anyone who can confirm guesses against fakes |
| A5 The snapshot on disk | none by default: lazyslice writes to a database, not a file. `snapshots/` is reserved and gitignored for a future export | git, synced folders |
| A6 `mapping_file:` CSVs | paths named in the yml; original values in plaintext; must reach every machine that reproduces the snapshot, which is the audience the snapshot exists for (ADR-006, revised) | git; anyone who would not be given source access — the file is distributed like a source credential, by hand or through the same secret store as `LAZYSLICE_SECRET`, never through the repository |
| A7 The emitted `lazyslice.yml` | the repository | must carry references and decisions, never A3, A4 or A1 |
| A8 The release binary | GitHub releases, the Homebrew tap | must be the binary CI built from the tagged commit |
| A9 The source database's health | the source's xmin horizon, a standby's replay | a snapshot held longer than the plan said |

## Threats and controls

Each control is a mechanism, not a sentence; where a test enforces it, the test is named. Exit codes are ADR-005's.

### T1 The classifier misses a personal-data column, and the run exits 0 with cleartext in the target

Likelihood: high. Column-level detectors average macro-F1 0.61 to 0.87 and their authors call false-negative rates too high for practice (research/HARD_PROBLEMS.md §3.2); silent success is the most-evidenced failure class in the corpus, seven reports across five tools (research/SYNTHESIS.md §1 fact 3; research/COMPLAINTS.md "Frequency ranking"). Impact: the project ends; "you can't un-leak it" (research/SYNTHESIS.md §2 risk 2).

Controls. The classifier's recall is defended by exactly two things, and this row does not claim a third:
- Recall bias: `possible` and above is masked, after the neighbouring-column rule and FK propagation; a name hit alone is `possible`; special categories mask on name alone; free text and every JSON leaf are masked whole; there is no exemption by type (ARCHITECTURE.md §4, revised: the enum exemption is gone); the neighbouring-column rule raises `low` to `possible` in any table with a `likely` column.
- Second net: the same validators over the full contents of every column the target holds unmasked, and the string leaves of masked JSON columns; `possible` is exit 9 (ARCHITECTURE.md §6 item 4). It catches a column the 200-row sample under-represented. It does **not** catch a category outside the rule pack, because it runs the same rules; a category the pack lacks is missed twice.
- No pass-through mode exists; the CI grep on `no-mask|disable-mask|skip-mask|unsafe` fails the build (ADR-004).
- Invariant I2 in CI on both fixtures, including `nasty.sql`'s `ref` column holding emails and its JSONB and notes traps (docs/BUILD_PLAN.md PROMPT 3.3).
- The README lists what the green tick does not prove, beginning with preserved row identifiers (ARCHITECTURE.md §6 item 6).

The residual scan is not a control for this threat: its filter holds only cells the classifier already decided to mask, so a column the classifier missed is never in it. It is T12's control.

Blocks v1: **yes** (all of the above).

### T2 The target is production

Likelihood: medium. The transposed-arguments report is real (research/COMPLAINTS.md PII-13); a pooler alias, a DNS name and a loopback can all name one cluster. Impact: masked fake rows written over production, "the worst possible failure" (research/SQLIT_STUDY.md §5.0).

Controls (ADR-005 "Target", revised; ARCHITECTURE.md §9 "Target" is the ordered rule):
- **Identity** is a disjunction over sameness, not a conjunction over difference: the target is refused iff normalised `host:port/database` equals the source's, or `system_identifier` and `current_database()` both equal the source's. A second database on the same cluster is eligible and the header prints `same cluster as source` as a warning. Exit 2 before any connection is used for writing.
- **Locality**: the target is `Candidate.Local` or named by `--allow-remote-target HOST`; otherwise exit 4. Every other rule is satisfied by an empty, freshly provisioned production database during a migration, and the transposed-argument report (research/COMPLAINTS.md PII-13) is as likely to name an empty remote database as a full one. The argument that overruled a loopback restriction for the source does not apply to the side that is written.
- **Marker**: `lazyslice_meta` authorises truncation only when it is bound — the latest row's `source_fingerprint` (and `source_system_id` when both sides have it) matches the current source and its `schema_fingerprint` matches the target's current catalog (ARCHITECTURE.md §11.2). A marker left by a run against another source, planted by a colleague, or sitting beside a table we did not create authorises nothing and falls through to the emptiness check. The marker is therefore not a persistent artefact that can convert a database into a truncate-on-sight target after it has acquired a life of its own; this is the sentence the previous draft got wrong.
- **Emptiness**: every user table empty by `SELECT EXISTS (SELECT 1 FROM t)`, bookkeeping and extension tables exempt, over at most 2,000 user tables. **Above the cap the target is refused** (exit 4, `target.refused.table_cap`, table count printed); it is never sampled, whatever research/SQLIT_STUDY.md §5.2 proposed. `Eligibility.Verdict` is tri-state and `NotProbed` is never eligible. A table with row-level security enabled or forced, or on which the probe errors for any reason, counts as not empty and is named: under `FORCE ROW LEVEL SECURITY` with no matching policy, `EXISTS` returns false over millions of rows while `DROP TABLE` still succeeds — the same trap as `n_live_tup`, by a different mechanism. Never `pg_stat_user_tables`.
- No flag reaches a non-empty unmarked database. `--target` names, it does not bypass; there is no `--allow-nonempty-target` and no `--replace`. The refusal (exit 4) prints the row counts and the command that would clear the database.
- The gate is re-run on every run, including Scenario D with a committed yml; the file supplies the target's name and nothing else, and the marker binding above is what stops a file plus a marker from authorising a write to a database that has since acquired rows.
- Headless: anything that would create or destroy needs its flag (`--create-target`); nothing is truncated silently except rows in a bound marked target, which is printed.
- Tests: `TestGateRefusesPopulatedUnanalysedTarget` (a `pg_restore`d fixture with statistics reset), `TestGateAllowsSecondDatabaseOnSameCluster`, `TestGateRefusesTargetAboveTableCap`, `TestGateRefusesRLSTable`, `TestGateRefusesRemoteTargetWithoutFlag`, `TestGateIgnoresUnboundMarker`.

Blocks v1: **yes**.

### T3 A committed `lazyslice.yml` goes stale and a new column passes through

Likelihood: high over time. research/COMPLAINTS.md PII-1 is the highest-signal complaint in the corpus and is longitudinal; CB-13 is the vendors filing it against themselves. Impact: same as T1, later and quieter.

Controls:
- The classifier always runs; the file supplies opt-outs and defaults only; `TestConfigCannotLowerConfidence` (ADR-004).
- A column absent from the file is classified fresh and masked whenever its confidence is `possible` or above after the neighbouring-column rule and FK propagation, copied below that, and printed under `drift:`; `--strict-schema` makes drift exit 10 (ADR-004 "Drift", revised into the confidence scale that exists).
- An opt-out records the column's type fingerprint and is ignored when it changes; a renamed column is a new column.
- The schema fingerprint in the yml is compared and printed on every run.

Blocks v1: **yes** (tighten-only merge, drift masking, fingerprint expiry). `--strict-schema` is v1 but not blocking.

### T4 Values leave the process: logs, events, the TUI, error text, a masker, the network

Likelihood: medium. Postgres puts the offending row into `PgError.Detail` (`Key (email)=(alice@example.com) already exists`), so an unhandled unique violation prints cleartext into a CI log; verbose logging is on the phase 5 red-team list. Impact: cleartext in a CI log or a pasted ticket.

Controls:
- `event.Event` has no free-form string field; messages are codes plus identifier-only args; `TestNoValueBearingFieldSerialised` asserts that the transitive field types of `event.Event`, `pipeline.Config`, `pipeline.Plan` and `pipeline.Report` exclude `dsn.DSN`, `pipeline.RowBatch` and `pipeline.Table` (and so `Table.Samples`) under the `ref ← event ← pipeline` import graph, and that no code's template references an arg outside the enum (ARCHITECTURE.md §2 "Value-free types", §7).
- `Decision.Reason` is rendered from a template set whose placeholders admit identifiers and counts only; `TestReasonGrammar` parses every emitted reason back. `Schema.Samples` is never serialised by any subcommand, renderer or `--debug` path; `introspect --json` emits counts and fingerprints.
- `PgError.Detail`, `Where` and `Hint` are dropped unless `--show-row-values-in-errors` (ADR-005).
- No runtime-loaded masker exists; the masker is a compiled module we review (ADR-006).
- The binary opens exactly three kinds of connection — the source, the target, and the Docker socket — and spawns exactly two kinds of subprocess — `--password-command`, and `git` for the tracked-file check (T6). There is no telemetry, update check, model call or registry (ADR-007). A test runs the binary under a network namespace that allows only those endpoints, and the subprocess allowlist is a test in `internal/repo` and `internal/discover`.
- `--debug` prints the statement trace, which carries statement shapes with parameters elided, never values.
- **Values we send to the source.** Residual confirmation binds a candidate value as a parameter to a `SELECT EXISTS` on production, and under `log_statement = 'all'` or a slow probe under `log_min_duration_statement` that parameter is written to the source's log file, with the normalised text in `pg_stat_statements`. This is a value-egress path through the source's own logging, not ours. Controls: the indexable equality probe runs first and the case-folded probe only when it is false; probes are capped at 1,000 per run (`--residual-probe-cap`) with the count printed; the README's false-negative list says that confirmation sends candidate values to the source (ARCHITECTURE.md §6 items 3 and 6). The residual is accepted: the alternative is not confirming, which is T12's failure.

Blocks v1: **yes** (event model, reason grammar, `Detail` dropping, no runtime maskers, no outbound calls, probe cap). The network-namespace test is phase 5.

### T5 Secrets in config or logs: a DSN password or the masking key lands in `lazyslice.yml`, events, `--json` or `lazyslice_meta`

Likelihood: medium. Every tool in the category has had a credential in a config file; the key in a committed file makes any guessable value reversible (research/HARD_PROBLEMS.md §2.3). Impact: source credentials or the key in a repository.

Controls:
- `dsn.Ref.String()` redacts; the yml records references (`from: env, var: DATABASE_URL`; `password_command` as the command string), never values; `lazyslice_meta` holds `sha256(host:port/database)[:16]`, `sha256(K)[:8]` and catalog fingerprints only (ARCHITECTURE.md §10, §11.2).
- The key is never in the yml, the target, or an event; the emit stage's writer refuses to serialise a struct field tagged `secret:"true"` and a test asserts every such field.
- **`--where` is operator SQL, and a predicate is exactly where a real value appears** (`email = 'ceo@bigcorp.com'`, `customer_id = 84213`): a value in a `where:` line is A1 in A7 by another route. Emit withholds any predicate containing a string or numeric literal, records `where_fingerprint` instead, and prints that it did; a later run finding a fingerprint and no `--where` is exit 2 asking for the predicate (ARCHITECTURE.md §10).
- `./lazyslice.secret` is created 0600 and appended to `.gitignore` in the same step, or not created at all when it cannot be protected (T6); the tool refuses when the file is tracked by git.
- A discovered password is never persisted without being asked; there is no keyring write in v1 (ADR-004). After a prompted password the run prints the two ways to stop being asked (`~/.pgpass`, `--password-command`), so the remedy is in the transcript.

Blocks v1: **yes**.

### T6 A snapshot, the secret, or a mapping file is committed to git

Likelihood: medium; it is how every "prod dump on the shared drive" begins (CONCEPT.md "The moment they reach for it"). Impact: A4 or A6 in a repository forever.

Controls, implemented in `internal/repo` and specified in ARCHITECTURE.md §9 "The repository", including the three degenerate cases:
- First run appends `lazyslice.secret`, `snapshots/` and every `mapping_file:` path to `.gitignore` when it creates or first reads them, and prints that it did. Outside a git repository the secret is written and the header says it is unprotected. When `.gitignore` is absent or unwritable inside a repository, the secret file is **not created**: the run uses an ephemeral key and prints why, `--require-key` makes that exit 5, and a mapping file in that state is exit 2. Refusing to write beats writing un-ignored.
- The run refuses (exit 2) when `lazyslice.secret` or a mapping file is tracked by git (`git -C <root> ls-files --error-unmatch`), naming the file and the `git rm --cached` command; exit 1 from git means untracked, exit 128 means no repository, and only exit 0 refuses, so the fail-closed check does not fail everywhere. With no `git` on `PATH` the check is skipped and printed; the `.gitignore` entry still protects a never-tracked file.
- lazyslice writes no snapshot file in v1; the target is a database. `snapshots/` is reserved so the rule exists before the feature does.

Blocks v1: **yes** for the `.gitignore` write, the not-created fallback, and the tracked-file refusal.

### T7 A malicious or buggy custom masker

Likelihood: low in v1 because none can be loaded; high the day one can. A masker sees every personal value and can write it anywhere (docs/BUILD_PLAN.md PROMPT 2.5). Impact: a pass-through mode with extra steps.

Controls:
- No `.so`, subprocess, expression language or WASM in v1; maskers are compiled into the `mask` module and registered at build time (ADR-006).
- A masker is a pure function of `(h, value, constraints)`; the transform stage passes it no connection, no file handle and no context.
- The reversal path (a WASM sandbox after three independent requests) requires a row in this table before code.

Blocks v1: **yes** (by absence; the CI grep and ADR-006 keep it absent).

### T8 A half-loaded target that looks complete

Likelihood: medium. Interrupted extracts, a dropped connection mid-COPY, disk full on the target; four tools shipped exit-0 no-ops (research/COMPLAINTS.md TR-15, TR-16, FK-10). Impact: a developer or CI trusts a target that is missing rows, has sequences at 1, or has broken FKs.

Controls:
- One transaction per table; FKs created after data as `NOT VALID` then `VALIDATE CONSTRAINT` per constraint (exit 8 naming the constraint and the cap that caused it).
- `setval(seq, coalesce(max(id), 1), max(id) IS NOT NULL)` after commit, never a literal 0 (research/HARD_PROBLEMS.md §4.1).
- Every `ChildOK` or `ParentOnly` step holds exactly its key count, lookups hold their bounded count, unmasked columns are byte-identical to the source on a sample fetched by identity (a source that changed since the snapshot is reported, not failed), and every failed check is a non-zero exit (ARCHITECTURE.md §6 item 5).
- **After any failure the target is either empty or carries a marker row whose `status` is `running` or `failed`, and the next run truncates it.** That is the property as it is, not as the previous draft wished it: load commits one transaction per table, so a failure at table 5 leaves tables 1 to 4 committed, and under SIGKILL, an OOM kill, a container stop or power loss no cleanup runs and the row stays at `running`. The gate treats a bound marker at `running` exactly as it treats `complete`: truncate and print (ARCHITECTURE.md §11.2). A clean failure additionally rolls back the open table, marks the row `failed` and exits non-zero; SIGINT rolls back and exits 130. A target is never left with some tables loaded and `status = complete`, because `complete` is written last.
- Invariants I1, I5 and I6 in CI, plus `TestKillNineLeavesRunningMarker`: `kill -9` mid-load, then a second run must truncate and complete.

Blocks v1: **yes**.

### T9 The source is written to, or pinned, by us

Likelihood: low for writes, medium for pinning. pg_sample requires write access and condenser creates a temp database on the source (research/COMPETITORS.md "Orientation"); a held snapshot pins the xmin horizon and a standby cancels after `max_standby_streaming_delay`, default 30 s (research/HARD_PROBLEMS.md §4.3). Impact: bloat or a cancelled run on production; in the worst case a write.

Controls:
- Every source transaction is `REPEATABLE READ READ ONLY`; that transaction is the enforcement. The tracer is the evidence: all five pgx tracers (`QueryTracer`, `CopyFromTracer`, `BatchTracer`, `PrepareTracer`, `ConnectTracer`) are registered on the source pool; a statement whose text does not match a registered shape (the fixed set lazyslice generates: catalog introspection, `TABLESAMPLE` sampling, the `unnest` join shapes, `pg_export_snapshot`, `SET TRANSACTION SNAPSHOT`, the bounded count, the confirmation probes) is refused by returning a cancelled context from `TraceQueryStart` and recorded as a violation that fails the run. A first-keyword allowlist was rejected because `WITH x AS (DELETE ... RETURNING *) SELECT ...` and `SELECT lo_import(...)` both begin with an allowed keyword. `TestSourceNeverCopiesOrBatches` fails if `internal/pg` ever calls `CopyFrom`, `SendBatch` or `Prepare` on a `Source`. Invariant I4 replays the trace and checks source row counts and a checksum (ADR-005, revised).
- The role's privileges are checked with `has_table_privilege` and printed; a writable role is the loudest header line; `--require-read-only-role` makes it exit 6. Blocking by default was rejected (ADR-005) because it fails CONCEPT.md's headline use case.
- Key lookups join against typed `unnest($1, ..., $k)` arrays in chunks; no temp table is ever created on the source. Lookup detection uses a bounded count (`LIMIT 1001`) gated by `reltuples`, never `count(*)` over a large table, because an unbounded count on a `users` table inside the holder transaction is the xmin pin this row exists to prevent (ARCHITECTURE.md §3).
- The plan prints the estimated snapshot hold; the snapshot is released at the end of extract, before load and verify; residual confirmation uses a new short transaction (ARCHITECTURE.md §6). Standby cancellation is exit 7 with a retry message.
- Pooled endpoints fall back to a single-connection serialised extract rather than extracting without a snapshot.

Blocks v1: **yes** (READ ONLY transactions, tracer, privilege report, snapshot release before verify). The hold estimate is v1; its accuracy is a phase 5 item.

### T10 Supply chain of releases

Likelihood: low per release, certain over the project's life. A one-line unmerged fix ended Snapshot's afterlife (research/POSTMORTEMS.md §8); a compromised release is the category's trust-ending event. Impact: a binary that is not what CI built, or a dependency that phones home.

Controls:
- Tag-triggered goreleaser builds in GitHub Actions with cosign keyless signing and a published SBOM. Release credentials are short-lived wherever a short-lived one exists: `GITHUB_TOKEN` is minted per job and cosign signs keylessly with no stored key. The one exception is `HOMEBREW_TAP_TOKEN`, a scoped PAT with write access to `Liarea/homebrew-tap` held as a repository secret, because publishing the cask crosses a repository boundary and the GitHub App that would mint an installation token does not exist yet. This is the accepted position, not an oversight; reversal condition: the App exists on `Liarea/homebrew-tap` with both secrets set, at which point the tap-token step in .github/workflows/release.yml is restored and the PAT deleted (research/SQLIT_STUDY.md §5.9; amended 2026-09-06).
- Every dependency pinned exactly; `go.sum` verified; `govulncheck` in CI; dependency bumps are their own pull requests with the reason.
- A CI job installs the built artefact on every target and runs `lazyslice --version`; the Homebrew formula pins the checksum.
- `CGO_ENABLED=0` and the no-outbound-call test (T4) mean a compromised dependency has no native code path and no network path to use.
- Agent commits are denied by hook; the orchestrator commits; accepted ADRs cannot be edited (CLAUDE.md; research/AI_PROJECT_PRACTICES.md §4 items 7 and 9).

Blocks v1: signing, pinning and the install job **yes** (Gate 3 requires `brew install` to work); SBOM and `govulncheck` are v1 but not blocking.

### T11 The runaway slice: memory, time, and the plan that lied

Likelihood: high without controls. 200 MB source, 3 GB RSS, OOM (research/COMPLAINTS.md SP-3); "tried to collect data indefinitely" (research/HARD_PROBLEMS.md §1.2); Jailer #126 put other customers' orders in customer 1's slice (a privacy bug, not only a size bug). Impact: an OOM-killed laptop, or a slice containing rows the root does not own.

Controls:
- `CHILD_OK`/`PARENT_ONLY` provenance with a row's mode fixed at first push; children only from `CHILD_OK` rows; per-parent-key cap 100; depth 3 (ARCHITECTURE.md §3).
- Row budget 1,000,000 and memory budget 256 MiB checked during planning; exceeding either is exit 11 naming the table and the flag.
- Bounded channels between extract, transform and load; sorted key sets sized and printed; the residual filter sized and printed.
- The plan and its estimate print before extraction, with `--plan` to stop there.
- Fixtures: no cycles, self-cycle, two-table cycle, overlapping SCC, keyless junction, polymorphic pair, 300-column table, 2M-row table under a memory cap (docs/BUILD_PLAN.md PROMPTS 3.3 and 4.4).

Blocks v1: **yes**.

### T12 The masker passes a value through, and nothing downstream notices

Likelihood: low per masker, certain over sixteen of them. A generator that returns its input on an unhandled type, a canonicaliser that trims to empty and falls into the `''` rule, a JSON walker that skips a leaf kind: each ships cleartext under a column the report calls masked. research/COMPLAINTS.md PII-4 and PII-5 (parallel masking reporting success over cleartext) are this class. Impact: the same as T1, under a green tick that names the column as masked.

Controls:
- Residual scan: every masked cell, and every masked JSON leaf keyed by path, adds `HMAC(runKey, enc(column, path, canonical))` to a Bloom filter at 10⁻⁶; verify tests every masked target value against it and confirms a hit on the source over a short second transaction; a confirmed hit is exit 9 (ARCHITECTURE.md §6 items 1 to 3). This is the one check that tests the masker rather than the classifier, which is why it is its own row.
- A hit that cannot be confirmed — the confirmation channel will not open, the probe errors, the role lacks `SELECT`, or the probe cap is reached with hits untested — is exit 9 naming the reason, never a printed note. A hit the source denies is printed as unconfirmed and counted.
- `mask` module tests per generator: output never equals input for non-empty input; output length is uncorrelated with input length for `free_text`; the colliding-pair encoding fixtures (§5).
- Stated limit, in the README: the filter tests canonical equality, so a value that was truncated, reformatted or embedded in a longer string is not detected.

Blocks v1: **yes**.

### T13 The shared masking key is a guess-confirmation oracle held by everyone

Likelihood: certain by construction. Cross-run determinism (ARCHITECTURE.md §5 "Determinism scope") holds only when a team shares `K`; the file is gitignored, so the only distribution paths are a shared secret store or a CI environment variable. `K` plus any snapshot confirms, for every enumerable category (names, emails, phone numbers), whether a guessed real value is present: it is held by N developers and M CI jobs, never expires, and has no rotation trigger on offboarding. Impact: a former team member, or a leaked CI variable, can confirm any guessable customer against every snapshot ever produced with that key.

Controls:
- `K` is production-grade secret material, not a convenience seed, and ADR-004 (revised) says so: it is distributed only through `LAZYSLICE_SECRET` from the team's secret manager or CI secret store, never by copying `lazyslice.secret` between machines; the file is the single-laptop form.
- Rotation trigger: anyone who held `K` leaving the team, or `LAZYSLICE_SECRET` appearing in any log, rotates it; rotation is a new value, old snapshots are regenerated, and `lazyslice_meta` prints `secret changed` on the next run into every marked target.
- The README states the oracle in one sentence beside the pseudonymisation sentence.
- Nothing weakens the oracle in v1: per-entity keys and crypto-shredding are the reversal path if the phase 5 red team demonstrates confirmation at scale, and they are listed in E9.

Blocks v1: **yes** for the ADR-004 text and the README sentence; the rotation runbook is v1 but not blocking.

## Positions this document takes

- **The snapshot preserves production row identifiers.** Surrogate keys and the FK columns that reference them are copied verbatim (ARCHITECTURE.md §4), so anyone with any other reference to a production record — an admin URL, a support ticket, a log line, a payment or CRM record, an analytics replica — re-identifies every masked row exactly, without cryptanalysis and without `K`. The default `--take 500` copies the same lowest-id rows to every laptop on every run. This is the strongest correlation path in the design and the first entry in the README's false-negative list. A keyed remap of surrogate keys is a v2 item (ARCHITECTURE.md §14): it touches every FK, every sequence reset and the reproducibility invariants, and shipping v1 without it is a decision this document records rather than leaves silent.
- **Pseudonymisation, not anonymisation.** Frequency is preserved; quasi-identifiers are not handled; k-anonymity is not promised (research/SYNTHESIS.md §3). A masked column with a small admissible domain is a stable substitution that frequency recovers, and the run names every such column (ARCHITECTURE.md §5).
- **Real email domains are not kept.** Keeping `@bigcorp.com` leaks employer; fakes land in RFC 2606 domains (research/HARD_PROBLEMS.md §2.1).
- **Prefix, length and first character never survive** a mask, free text included (research/HARD_PROBLEMS.md §2.3, the Replibyte first-character transformer). `NULL` and `''` are the two stated exceptions.
- **Erasure requests.** A snapshot is personal data; the README tells users to regenerate snapshots after an erasure request and that rotating `K` makes old snapshots uncorrelatable with new ones.
- **Key compromise** lets the holder confirm guesses against fakes, not decrypt them (T13); rotation is a new value, and old snapshots are regenerated.
- **The read-only source is a checked precondition, not a guarantee** (research/SYNTHESIS.md §1 fact 7). The `READ ONLY` transaction enforces; the tracer proves what we sent; the privilege report says what the role could have done.
- **The target is local** unless a flag names the remote host (T2).
- **What the green tick does not prove** is printed, not buried (ARCHITECTURE.md §6 item 6).

## v1-blocking controls, collected

For docs/BUILD_PLAN.md Gate 2 and the phase 4 scope (PROMPT 4.5; the cut line is ARCHITECTURE.md §14): recall-biased classifier with no type exemption, no pass-through mode, the CI grep, and the full-coverage second net (T1); the ordered eligibility gate — disjunctive identity, locality, bound marker, `SELECT EXISTS` emptiness that fails closed above the cap and on RLS, no override flag (T2); tighten-only yml with drift masking in the real confidence scale and type-fingerprint expiry (T3); value-free events, config, plan and report, the reason grammar, `Detail` dropping, and the capped confirmation probes (T4); no runtime maskers, no outbound calls, two named subprocesses (T4, T7); redacting DSNs, fingerprints only in the yml and marker, `where` literals withheld (T5); `.gitignore` writes with the not-created fallback and tracked-file refusal (T6); per-table transactions, post-data FKs, strict-NULL `setval`, empty-or-marked on failure with `running` forcing truncation (T8); `READ ONLY` transactions, the shape-allowlist tracers, the printed privilege report, the bounded lookup count and snapshot release before verify (T9); signed, pinned, install-tested releases (T10); deterministic provenance-tagged worklist with `Bytes()`-based budgets (T11); residual scan with leaf digests and fail-closed confirmation (T12); `K` distributed as a secret with a rotation trigger (T13).
