# Adversarial re-review — 2026-09-15

**Reviewed commit:** `c8ec5e2124d9f5fea2387a6028d152b80c2d8db3`

**Verdict:** the eleven findings from the 2026-09-09 review have either been fixed, explicitly refused, or kept open as a documented product decision. The exact destructive-race, sink-canary, text-FK, verify-lifecycle, DDL-default, sparse-hit, and torture probes now hold. That is substantial progress.

The broader guarantees still do not hold. This review reproduced two new ways for an eligible target to lose data, two PostgreSQL equality cases that turn valid foreign keys into invalid target data, and a strong email literal crossing in catalog DDL under exit 0. The advertised `--password-command` also does not execute. These are release blockers for a tool whose core promise is a safe, working local database. **Do not cut `v0.1.0` from this state.**

The strongest positive signal is that failure paths usually fail closed: the two FK cases ended non-zero with a failed marker, and the unique-date case refused at plan. The strongest negative signal is that the target boundary is still inferred from a partial catalog query and a mutable visibility rule. A destructive tool needs a complete, transactionally rechecked authorization boundary.

This review is diagnostic. It adds no tracker tasks and changes no implementation. The working tree was changing concurrently, so all code claims and executable probes are pinned to the commit above.

## Priority findings

| Priority | Finding | Observed result | Tracked at reviewed commit? |
|---|---|---|---|
| P1 | A populated materialized view does not make an unmarked target ineligible | exit 0; materialized view dropped; source table loaded | No |
| P1 | Enabling and forcing RLS after the gate hides a newly inserted target row from the destructive recheck | exit 0; stranger row deleted | No |
| P1 | `citext` FK equality is converted to text equality during planning | exit 8; parent omitted; target FK invalid; marker failed | No |
| P1 | Numerically equal values with different scales mask differently | exit 8; valid numeric FK becomes invalid; marker failed | No; T-0159 does not cover same-type scale equivalence |
| P1 | A strong email in a `LIKE`/`NOT LIKE` catalog pattern crosses unchanged | exit 0; email remains in target constraint | T-0201, but scheduled in E9 |
| P1 | `--password-command` is advertised and persisted but never supplies a password | exit 3; source authentication fails | T-0192 covers screening before persistence, not execution/refusal explicitly |
| P2 | The unique-domain formula says a one-row finite-domain column is impossible | exit 12; “no row count small enough” | No |
| P2 | `--memory-budget` is not a process-memory budget | documented peak 458–481 MiB with a 256 MiB default | T-0144, E9 |
| P3 | The repository's gitleaks control is red on a committed synthetic fixture | one `generic-api-key` finding | No |
| P3 | The exact reviewed commit is red on hosted Windows CI | two permission-mode tests fail | Active code area, no separate task found |

### 1. The target gate ignores destructible materialized data

The target contained an empty `items` table and a populated materialized view named `archive`. The view retained row `999` after the base table was emptied. Lazyslice accepted the database, ran with exit 0, dropped `archive` through `DROP TABLE ... CASCADE`, and replaced `items` with the source row.

The gate query in `internal/pg/target.go:196-203` inventories only `pg_class.relkind IN ('r', 'p')`. It excludes materialized views (`m`) and every other user object. The loader then deliberately uses `CASCADE`; `internal/load/ddl/ddl.go:73-75` says a view left behind is not a reason to stop. Those choices are individually documented but jointly violate the concept's rule that the target is empty or was written by lazyslice.

This is broader than materialized views. A target can contain views, sequences, functions, types, policies, or dependent objects that the gate never considers and `CASCADE` or type cleanup may destroy. “Every user table has zero visible rows” is not equivalent to “this database is empty.”

**Required change:** define eligibility over every object the load or quarantine path can destroy. For an unmarked database, refuse when any non-bookkeeping user object exists, or prove a narrower complete allowlist. Re-read that object inventory after acquiring all destructive locks. The simpler and safer long-term rule is that destructive reloads require a database-level marker bound to this tool; a newly created empty target can receive that marker before any schema action.

### 2. The target race survives through row-level security

The original post-gate insert probe now refuses correctly: a concurrent row inserted after eligibility remained present and the run exited 4. An adapted probe changed the target's visibility before the lock-and-recheck:

1. An unmarked, empty target owned by a non-superuser passed the gate.
2. While source introspection was blocked, another session inserted row `999`, enabled RLS, and forced RLS.
3. Lazyslice acquired its table lock and ran the recheck.
4. The owner saw zero rows under forced RLS, so the recheck passed; `DROP TABLE` deleted the hidden row; the run exited 0.

`internal/load/load.go:477-489` only repeats `SELECT count(*)` after the lock. It does not repeat the gate's `relrowsecurity`/`relforcerowsecurity` test and does not force row-security evaluation to fail closed. Locking closes the row-write race; it does not freeze or validate the authorization facts used by the query.

**Required change:** after the `ACCESS EXCLUSIVE` lock, re-read both RLS flags and refuse if either is set. Use `SET LOCAL row_security = off` for the count so PostgreSQL errors instead of silently filtering rows when policy visibility is incomplete. Treat every gate fact that can change as part of the locked recheck, including object kind and identity.

### 3. `citext` breaks the planner's completeness semantics

The source held a valid FK from child `originalcredentialabc123` to parent `OriginalCredentialABC123`, both typed `citext`. PostgreSQL considers the values equal. Lazyslice omitted the parent, masked the child to `$lazyslice$invalid`, failed FK validation at exit 8, and left the target marked `failed`.

`internal/plan/keyset.go:91-94` says `citext` is text “in every way that matters” and transports it through a `::text[]` key set. Case-insensitive equality is exactly what matters for reachability. The planner thereby changes PostgreSQL's equality operator and loses a required parent.

**Required change:** make traversal use the FK operator semantics PostgreSQL resolved, including domains, collations, and extension types. A type-name family table cannot stand in for operator semantics. At minimum, retain and cast to the actual comparison type for `citext`; the durable design records the referenced equality operator/opfamily during introspection and tests it with the values PostgreSQL considers equal.

### 4. Mask determinism does not preserve numeric equality

The source held a valid numeric FK whose parent was `123451234.0` and child was `123451234.00`. Both `tax_id` columns were classified `national_id`, placed in one equality group, and assigned the same masker. The target received `802938720` and `562764925`; FK validation failed at exit 8 and the marker was `failed`.

The equality-group fix chooses one generator, but `mask/canonical.go:131-136` canonicalizes financial and national identifiers by removing punctuation and then trimming leading zeros. It retains scale zeros, so PostgreSQL-equal numerics get different HMAC inputs. T-0159 addresses different type families; this failure occurs inside one numeric family.

The necessary invariant is stronger than “FK-connected columns use the same generator”:

> If PostgreSQL's FK equality operator says `a = b`, masking must produce equal outputs for `a` and `b`.

**Required change:** canonicalize from the decoded PostgreSQL value and its equality semantics. For numeric values, normalize sign, coefficient, and scale so equal numerics have one representation. Add equality-law tests for numeric scale, `citext` case, `bpchar` padding, domains, collations, timestamps, UUID text forms, and every accepted cross-type FK.

### 5. T-0189 still admits a strong literal in a `LIKE` pattern

The source constraint was:

```sql
CHECK (label NOT LIKE '%pattern.canary@example.org%')
```

The target retained `CHECK ((label !~~ '%pattern.canary@example.org%'::text))` and the run exited 0. The scanner does recognize PostgreSQL's `!~~`; the miss is in normalization. `internal/pipeline/ddlliteral.go:122-180` applies one regular-expression metacharacter set to every pattern family. It removes the unescaped dots from a `LIKE` operand, reducing the email to `patterncanary@exampleorg`, which the email validator no longer recognizes. `internal/plan/ddlliteral.go:278-291` validates only that stripped form.

T-0201 describes the right shape—validate raw and stripped text, and strip metacharacters according to the actual operator—but placing an observed exit-0 source-literal crossing in E9 understates it. It belongs in the current Gate 5 red-team fix set. The latest partial-index probe with `Grace Hopper` correctly refuses at exit 12, so T-0189 fixed one route without closing the class.

### 6. `--password-command` is a no-op contract

A password-protected PostgreSQL instance was invoked with a passwordless source DSN, a valid synthetic password on the target DSN, and `--password-command 'printf synthetic_pw'`. The source connection failed authentication at exit 3.

The flag is registered in `cmd/lazyslice/main.go:855-856`, carried in request/config types, and emitted into YAML. There is no execution path that reads its stdout and supplies it to discovery. In contrast, `ARCHITECTURE.md:997` and `THREAT_MODEL.md:102` count it as one of exactly two subprocess classes, and the user-facing help promises that its stdout is the password.

This contract can push users toward putting passwords in DSNs or environment variables, while also persisting an unevaluated shell command. T-0192 screens the command before writing YAML and hardens secret-file paths, but its title and acceptance need to say whether the flag is implemented or refused. Shipping a credential feature that silently does nothing is worse than an explicit unsupported error.

### 7. One-row unique columns are falsely impossible

`mask.Required(n)` uses `n² × 500,000`. For `n = 1`, it therefore demands a 500,000-value domain. `person_date` has a smaller finite domain and the plan refuses a one-row unique date with the message that no row count is small enough. There cannot be a collision among one generated value.

The birthday bound is based on pairs, `n(n-1)/(2ε)`, and the exact cases `n <= 1` should require only one admissible output. The present formula is a documented conservative approximation, but an approximation that rejects the smallest possible slice damages the zero-config promise and masks unrelated equality tests behind a plan refusal.

### 8. The memory flag names a guarantee it does not provide

`internal/plan/plan.go:827` counts only key and residual-filter estimates. Runtime batching now has an 8 MiB lower-bound cap, a real improvement over the original 2 GiB batch. The project's own measurements in `docs/PERF.md:217-221` still show 458 MiB peak RSS for text and 481 MiB for JSONB while the default `--memory-budget` is 256 MiB and its help says “estimated resident set.”

T-0144 is correctly open. Until it lands, rename the flag and output to the resources it actually budgets. If the product keeps the current name, it needs end-to-end accounting or an enforced process-level limit plus headroom. A batch-size limit and a planning estimate are useful controls; neither is a resident-set ceiling.

### 9. Security automation is not green at the reviewed commit

`make check`, `make integration`, `make torture`, and `make vulncheck` passed locally. `govulncheck` reported no reachable vulnerability. Two controls remain red:

- `gitleaks git . --config .gitleaks.toml --redact` reports one `generic-api-key` in `docs/reviews/2026-09-15-redteam/round1.json`. It is a synthetic canary, not a real credential, but the repository's “scan before public push” claim is stale and no blocking gitleaks CI job protects it. Narrowly allowlist the fixture/fingerprint and add the scanner to CI.
- Hosted CI run [35015962532](https://github.com/Liarea/lazyslice/actions/runs/35015962532) fails only on `windows-latest`: `TestResolveKeyIfPresentWithExistingSecretFillsTheKey` and `TestAnOrdinarySecretFileIsAccepted` create a nominal `0600` file which is observed as `0666`, so the secret guard refuses it. The release workflow's exact-commit gate is doing its job, but the commit is not releasable.

## Recheck of the eleven earlier findings

| Earlier finding | Re-review result |
|---|---|
| 1. Eligibility expires before drop | **Original reproduction fixed.** The post-gate inserted row survives and the run exits 4. The RLS variant above remains. |
| 2. Warning prints a polymorphic source value | **Fixed.** The exact sink canary is absent while the target value is masked. |
| 3. Wider unique masker breaks an FK | **Exact text case fixed.** Parent and child receive the same unique credential output and the run completes. PostgreSQL equality variants remain. |
| 4. `complete` written before verify | **Fixed for the reproduced residual failure.** Exit 9, marker `failed`, and the recreated table is quarantined. Load/FK failures retain a partial failed target by documented design. |
| 5. DDL defaults carry source literals | **Exact default fixed.** A later partial-index path now refuses. The `LIKE` catalog path above still leaks. |
| 6. TLS settings lost on round trip | **Explicit DSN parameters fixed.** Environment-only libpq and `PGSERVICE` state remains open as T-0168. |
| 7. Second net passes a rare strong hit | **Fixed.** One email among nineteen ordinary values is masked; zero source survivors. |
| 8. Arbitrary JSON keys preserved | **Partially fixed as stated.** Strong validator hits in keys are masked. A key such as `Grace Hopper` still survives while its value is replaced; T-0143 is the unresolved policy decision. |
| 9. Memory budget is not resident-set limit | **Still open.** Runtime batches are much smaller, but the flag remains a partial planning estimate. |
| 10. `mapping_file` advertised but unimplemented | **Resolved by explicit refusal** under ADR-012. |
| 11. Torture baseline taken after run | **Fixed.** Source fingerprints precede the run and a negative control is present; the suite passed. |

The statement “every reproduced finding has landed” is fair only when “finding” means each original minimal reproduction. It should not be read as proof of the larger safety property those reproductions sampled.

## Architecture review

### The target authorization model is too permissive for a destructive default

The current model lets an unmarked database become destructible after a partial emptiness test. That creates a permanent obligation to enumerate every object, every visibility mode, and every race that can make “empty” untrue. The materialized-view and RLS probes show how easily the proof decomposes.

Move ownership earlier and make it explicit. A newly provisioned database can be marked before schema creation. An existing database should either be fully object-empty under a fail-closed catalog transaction or require an explicit initialization action that records a database identity. Subsequent runs can then rely on a locked marker binding. The marker should authorize a database, not merely justify truncating tables one at a time.

### This is a PostgreSQL program throughout

`ARCHITECTURE.md:44` calls classify, plan, and transform engine-agnostic, but PostgreSQL types, casts, catalogs, operators, collations, arrays, RLS, extensions, and error behavior appear throughout those stages. The new `citext` and numeric failures are consequences of abstracting away PostgreSQL equality before it is understood.

T-0209 should change more than wording. Treat PostgreSQL semantics as first-class domain objects now. A future second engine should earn an interface by presenting a genuinely shared semantic boundary. Designing toward a hypothetical common interface today makes it easier to erase exactly the database-specific facts safety depends on.

### Schema recreation and “working application” pull in opposite directions

`ARCHITECTURE.md:1230-1248` deliberately reimplements a narrow subset of `pg_dump --schema-only` and omits views, materialized views, functions, procedures, triggers, RLS policies, rules, comments, privileges, publications, foreign tables, custom operators, collations, and partitions. Those omissions are printed rather than refused. A database can pass every lazyslice invariant and still be unusable by the application.

This is the largest product/architecture decision still hidden behind implementation detail. There are three coherent products:

1. A **data fixture** that recreates a documented subset of PostgreSQL schema and does not promise a working application.
2. A **working local application database** that delegates high-fidelity schema movement to PostgreSQL's own `pg_dump`/`pg_restore`, accepting native tooling as a dependency.
3. A **data loader into an already migrated target**, making application migrations responsible for schema fidelity.

The current concept promises the second outcome while the architecture implements the first. “One static binary, no native dependencies” is a delivery preference; “working local environment today” is the user outcome. Dogfood should decide which constraint yields.

### Safety verification reports a verdict without its proof boundary

`pipeline.Report` contains checks, rows, time, exit, unconfirmed hits, and probe count. It cannot tell a caller which columns were tested, which values were sampled, which types were skipped, or which object classes are outside the scan. T-0207 correctly identifies this. It belongs before a stranger can interpret exit 0 as evidence of safety.

The report needs machine-readable coverage and exclusions for classifier, transform postcondition, residual confirmation, target second net, and catalog scan. The result should distinguish “tested and clean,” “masked by construction,” “sampled,” “unsupported,” and “not examined.” Without that, the exit code is stronger than the evidence behind it.

T-0207's description calls T-0195 the classifier side of this work. T-0195 is narrower: it covers checksum-only national-ID rules falsely masking ordinary numeric business keys. It does not report classifier coverage or exclusions. The classifier half still needs an explicit result contract.

### The conceptual safety promise is internally inconsistent

`CONCEPT.md:15-17` says an unseen column is masked or the run stops, uncertainty masks more, and no unclassified column is copied as-is. The implemented classifier intentionally passes ordinary unclassified scalar columns through, while README and SECURITY list unknown personal data, arbitrary JSON keys, binary values, and other misses as limitations.

Rewrite the principle around a bounded, testable guarantee: which values are always transformed, which detectors run, which object classes cross, and which exclusions are reported. An absolute promise that the architecture cannot establish invites users to infer anonymity from pseudonymization.

### The project is accumulating control surface faster than user evidence

Between the previous reviewed commit `936ceec` and this commit, the repository added roughly 50 commits, 286 changed files, and 57,857 inserted lines in about two days. The reviewed tree has about 86,000 lines of Go including about 41,000 test lines, 30,000 lines of Markdown, 27 Go packages, and more than 220 tracker files. Much of that work directly hardened real defects. It also raises the cost of changing the product boundary before the required dogfood has happened.

Freeze feature expansion. Spend the next cycle on the destructive boundary, PostgreSQL semantic laws, one real application, and a same-schema comparison with the leading alternative. Delete controls that do not protect a named risk or a measured user outcome.

## Roadmap and product challenges

### Dogfood and differentiation are prerequisites, not later polish

T-0064's two real-project sessions remain open. Gate 5 does not name them clearly even though `v0.1.0` is defined as the first version a stranger may install. Add an explicit release condition: at least two applications boot and exercise a representative workflow against the generated database, with every override, omission, failure, and manual repair recorded.

T-0208 schedules a Greenmask comparison in E9 and says not to do it before Gate 5 closes. Reverse that dependency. Greenmask already documents [PostgreSQL subsetting and deterministic transformers](https://docs.greenmask.io/latest/configuration/) and uses PostgreSQL's dump/restore workflow; its [public repository](https://github.com/GreenmaskIO/greenmask) gives a concrete install and configuration baseline. Measure both tools on the same applications before investing further in the architecture. Count elapsed time until the application works, dependencies installed, configuration/overrides, schema objects lost, and safety refusals—not just rows per second.

### Choose the default slice's job

The default currently mixes several goals: reproduce a bug, seed development, create CI data, and preserve representative shapes. First-N-by-key tends to select old tenants and correlated history; it is neither random nor necessarily representative. T-0210 is the right decision point. Pick one primary job and make inclusion reasons, boundary crossings, cap omissions, and lookup expansion visible before copying.

### Make the JSON-key decision before claiming safe default

The current behavior preserves an arbitrary object key such as a person's name unless a strong validator recognizes its shape. JSON keys often carry identifiers, tenant names, emails, feature subjects, and medical codes. If the default promise is “safe local copy,” the conservative policy is to mask arbitrary non-structural keys and require a schema/rule to preserve application-defined keys. If compatibility requires key preservation, state that keys are outside the guarantee and surface that exclusion in verification. T-0143 should be resolved before `v0.1.0`.

### Stop treating future engines as a success metric

`ROADMAP.md:109-117` still schedules MySQL, SQLite, and SQL Server and makes four databases a Gate 7 requirement. That contradicts the claim that the roadmap now measures repeat runs rather than engine count. Remove the engine-count gate. Add a second engine only after repeated external demand and only after PostgreSQL's safety semantics are stable enough to define what is truly portable.

### Fix the sequencing source contradiction

`ROADMAP.md:3` calls `docs/BUILD_PLAN.md` the source of truth for sequencing while saying it is obsolete when later decisions supersede it. T-0211 correctly records the problem. Make ROADMAP plus tracker state authoritative and label BUILD_PLAN as historical input.

### Performance gates need one real database path

The relative CI benchmark is useful for catching CPU regressions in the extractor path, and the manual wide-row profile found a real problem. It does not measure discovery, planning SQL, source network, masking mix, target COPY, index construction, or verification. Keep the microbenchmark, but do not describe it as enforcement of total run performance. Add one periodic or release smoke run against the real 2-million-row fixture with elapsed time and peak RSS.

### The release ecosystem has not completed its smallest proof

The main repository is public, but there are no Git tags or GitHub releases. The Homebrew tap contains only its README, and the `v0.0.1` installation proof remains open as T-0155. That is a useful sequencing signal: prove the packaging path on a throwaway tag after CI is green, then return to safety and dogfood. Do not combine the first packaging exercise with the first version a stranger is invited to trust.

## Recommended sequence

1. **Freeze scope and block `v0.1.0`.** Keep the release guard; fix the Windows CI failure and gitleaks fixture/CI integration.
2. **Repair target authorization.** Inventory every destructible user object, recheck RLS and the full authorization state under destructive locks, and decide whether unmarked existing databases are ever eligible.
3. **Define PostgreSQL equality laws.** Fix `citext` reachability and numeric canonicalization, then test masking and traversal against the operator semantics for every supported FK type.
4. **Finish the current security set.** Move T-0201 into Gate 5; complete transform postconditions/recovery and T-0192; explicitly implement or refuse `--password-command`.
5. **Make exit 0 bounded and legible.** Land verify coverage/exclusions, resolve JSON keys, and rewrite the concept's absolute safety language.
6. **Dogfood two applications.** Measure time to a working app, schema losses, flags, overrides, and repair work. Run the same exercise with Greenmask.
7. **Choose the product boundary.** Decide data fixture versus full working schema versus migrated-target loader, and align the one-binary constraint, architecture, README, and gate with it.
8. **Only then cut `v0.1.0`.** Replace future engine counts with repeat use and externally supplied schemas as the next evidence gate.

## Validation record

| Check | Result |
|---|---|
| `make check` at reviewed commit | Pass |
| `make integration` at reviewed commit | Pass |
| `make torture` | Pass, including negative control |
| `make vulncheck` | Pass; no reachable vulnerabilities |
| gitleaks history scan | Fail on one committed synthetic red-team canary |
| Hosted CI for exact commit | Fail only on Windows secret-file permission tests |
| Original post-gate row race | Exit 4; inserted row retained |
| RLS race | Exit 0; inserted row deleted |
| Populated materialized-view target | Exit 0; view deleted |
| `citext` FK equality | Exit 8; target FK invalid; marker failed |
| Numeric-scale FK equality | Exit 8; target FK invalid; marker failed |
| Strong email inside `NOT LIKE` | Exit 0; literal retained in target catalog |
| One-row unique date | Exit 12; false domain refusal |
| Password command | Exit 3; supplied command not used |

The minimal reproduction scripts and exact observed outputs are in [evidence/README.md](evidence/README.md).
