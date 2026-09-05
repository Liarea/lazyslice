# ADR-004: Configuration is emitted after a run, never required before it, and can only tighten

Status: accepted, 2026-09-05. Revised 2026-09-05 in the phase 2 adversarial review (tracker T-0018), before Gate 2, which is the one revision the phase workflow allows; the "Revisions" section at the end lists what changed and why. After Gate 2 this file is frozen and a change is a superseding ADR.

## Context

CONCEPT.md "Zero config": first run asks at most one blocking question; headless runs ask none; configuration is emitted after a run as a record; "a committed `lazyslice.yml` never excuses an unseen column: it is masked or the run stops". research/COMPLAINTS.md PII-1 is the highest-signal complaint in the corpus and it is longitudinal: the config was right when written and wrong three months later; CB-1 ("Say I have nearly a hundred tables...") and CB-5 (deny-by-default) are the config-burden and safety halves of the same request. research/SYNTHESIS.md §4 item 4: once the yml is committed and used in CI, "the committed file *is* the stale config". research/OPEN_QUESTIONS.md item 4 decides that the key never appears in the yml, only a fingerprint, and asks where the key lives and what a keyless CI run gets.

The proposals agree on emission and on tightening-only. They disagree on three things: what happens in CI when a column the file has never seen appears; whether the OS keyring is a v1 key store; and whether a changed column type should revoke an opt-out.

## Options considered

**Drift in headless runs.**
- Never fail: an unseen column is classified fresh and masked at or above `possible`; the run prints it and exits 0 (research/proposals/mvp-first.md "D4").
- Fail by default: `--strict-schema` is the default with no TTY, so a new column is exit 9 (research/proposals/user-first.md "4"). The third judge noted this fails CI on every benign new column and that research/COMPLAINTS.md CI-1 asked for a flag, which Greenmask shipped as `--strict`, not a default.
- Fail unless accepted: exit 10 unless `--accept-drift`, because "a new prod column needs a human's eyes" (research/proposals/risk-first.md "4"). The second judge favoured this; the first and third called it friction on the most-repeated path.

**Key store.**
- `./lazyslice.secret` plus an environment variable, no keyring (mvp-first).
- Add the OS keyring via `zalando/go-keyring` with `lazyslice key save` (risk-first, user-first). The first judge corrected mvp-first's reason for cutting it (go-keyring is pure Go) and supplied a better one: headless CI has no D-Bus secret service, and it is a first-run question with no Gate 4 benefit.

**Opt-out expiry.**
- An opt-out is valid while the column's recorded category and type still match (risk-first).
- An opt-out is ignored and the column re-masked with a warning when the column's type fingerprint changes (mvp-first "D4"). The second judge named this "a structural answer none of the other proposals has".

## Decision

**Emission.** No config is read on a first run. `lazyslice.yml` is written after a successful run. `--plan` alone writes nothing, because an exploratory "what would it do" must not leave a file that turns the next plain run into a second run bound to that root and those caps; `--plan --config PATH` writes the file with `plan_only: true` and no `snapshot_id`, and a later run takes such a file's defaults and priors, re-classifies, and prints `from lazyslice.yml (plan only)`. The file holds: tool version; source and target as references, never credentials (`{from: env, var: DATABASE_URL}` or `{host, port, database}`, plus `password_command` as the command string when `--password-command` was used); root; `take`; `where` unless it contained a literal, in which case `where_fingerprint` is recorded and the predicate must be re-supplied on the next run (THREAT_MODEL.md T5); caps, depth and budgets used; `keys:` and `skipped:` for every `--key` and `--skip-table`, so a run that needed a flag reproduces without a human; the schema fingerprint; the classification fingerprint; the source snapshot id; the masking key fingerprint (`sha256(K)[:8]`); and one line per column with category, confidence, reason, masker and type fingerprint, or `unmask: {reason, by}` for an opt-out. Every reason is rendered from a template that admits only identifiers and counts. The full example is in ARCHITECTURE.md "The emitted lazyslice.yml".

**Reading it back can only tighten.** On a re-run the classifier always runs. The file supplies defaults (source, target, root, take, caps) and per-column opt-outs; it can raise a category or confidence, add a masker choice, or add `classify.extra_patterns`; it can never lower a column below the mask threshold except through a recorded per-column `unmask` with a reason. `TestConfigCannotLowerConfidence` enforces this (research/proposals/user-first.md "6").

**Drift.** A column present in the source and absent from the file is classified fresh and treated exactly as the classifier treats any column: after the neighbouring-column rule and FK propagation have run, it is masked when `Decision.Confidence` is `possible` or above and copied when it is `low` or `none`, and it is printed under `drift:` with its reason. The run continues and exits 0; the emitted file now includes the column. The earlier wording, "copied only when classified `none` at `certain`", was written in a scale that does not exist — `Confidence` is `none | low | possible | likely | certain` and a `none` category has no confidence above `none` — and was reaching for "the classifier ran and found no signal", which is what `none` and `low` mean. A forced mask with no category never arises under this rule: every decision at `low` or above carries the category of the signal that produced it, and `none` is never masked. `--strict-schema` makes any drift exit 10, naming the columns; it is a flag, not a headless default, because CI-1 asked for a flag and because masking is the safety property, not stopping. Where the judges disagreed, this is the second judge's posture applied only to the case that matters: a new column with no signal in a table that already has a column at `likely` or above is raised from `low` to `possible` by the neighbouring-column rule (research/HARD_PROBLEMS.md §3.2) and therefore masked, so the pass-through case in CI is a new column with no PII signal in a table with no PII. That is the case a human's eyes add least to. ARCHITECTURE.md "Classification" states the same threshold in the same words.

**Opt-out expiry.** Each opt-out records the column's type fingerprint (type OID, typmod, nullability, domain). When the fingerprint changes, the opt-out is ignored and the column re-masked with a warning naming the change. A renamed column is a new column and gets drift treatment.

**Key lifecycle** (research/OPEN_QUESTIONS.md item 4; THREAT_MODEL.md T13).
- `K` is 32 random bytes written to `./lazyslice.secret` on first run, mode 0600, appended to `.gitignore` on creation together with `snapshots/`, and printed once by path. When `.gitignore` cannot be written inside a repository the file is not created and the run uses an ephemeral key, printing why (ARCHITECTURE.md "First run", "The repository"). `LAZYSLICE_SECRET` (64 hex characters) overrides the file; `--secret-file` names another path.
- **`K` is production-grade secret material, not a convenience seed.** `K` plus any snapshot is a guess-confirmation oracle over every enumerable category for the whole customer base, and cross-run determinism holds only when everyone who produces snapshots holds the same `K`. Distribution is therefore `LAZYSLICE_SECRET` from the team's secret manager or CI secret store; `./lazyslice.secret` is the single-laptop form and is never copied between machines or pasted into chat. The README says this beside the pseudonymisation sentence.
- No OS keyring in v1. Rotation is a new value: delete the file or set a new environment value, regenerate every snapshot, and expect `lazyslice_meta` in each marked target, which holds the previous fingerprint, to print `secret changed — masked values will differ` and truncate before loading. **Rotation triggers**, each observable: anyone who held `K` leaves the team; `LAZYSLICE_SECRET` appears in any log or ticket; an erasure request arrives and the snapshots are regenerated anyway.
- A run with no key generates an ephemeral one, prints `masking key: ephemeral — fakes will not match your laptop`, keeps joins consistent within the run, and exits 0, because failing CI on a missing secret is how secrets get committed (research/proposals/user-first.md "8"). `--require-key` makes that exit 5.
- Loss costs cross-run stability only. Compromise lets the holder confirm guesses against the fakes; the README says pseudonymisation under GDPR Art. 4(5), not anonymisation, and that a snapshot is personal data to regenerate after an erasure request (research/HARD_PROBLEMS.md §2.3; research/proposals/risk-first.md "Deterministic masking").

**No unsafe mode exists.** There is no flag, mode or default that copies an unclassified column as-is. A CI grep fails the build on any flag string matching `no-mask|disable-mask|skip-mask|unsafe` (research/SYNTHESIS.md §2 risk 2; research/AI_PROJECT_PRACTICES.md §4 item 11). The only ways to see less masking are `--unmask table.col=REASON` and the yml's per-column `unmask`, both recorded with a reason; the flag's bare form without `=REASON` is exit 2 showing the required shape, so `unmask.reason` is never empty and `TestConfigCannotLowerConfidence` has something to assert against.

**Machine-local state** lives in `$LAZYSLICE_CONFIG_DIR`, else `$XDG_CONFIG_HOME/lazyslice`, else `~/.config/lazyslice`; deleting it never breaks a run (research/SQLIT_STUDY.md §5.6). `docker-compose.yml` is a naming source only, never parsed for a DSN (research/SQLIT_STUDY.md §5.10), which contradicts docs/BUILD_PLAN.md PROMPT 4.6 and is decided here.

## Consequences

- Scenario D of research/SQLIT_STUDY.md §5.5 (second run, zero questions) is what CI runs, with the same binary and command.
- The yml is a record with references in it; the `mapping_file:` escape hatch in ADR-006 is the one config input that contains original values, and it is gitignored and threat-modelled as an asset.
- `--reconfigure` ignores an existing yml and re-enters the first-run path.
- The neighbouring-column rule and the confidence scale are specified in ARCHITECTURE.md "Classification"; the classifier design prompt in docs/BUILD_PLAN.md (PROMPT 2.3) is answered there rather than in a separate ADR.

## Reversal condition

- Drift: if a dogfood session or the phase 5 red team shows a column copied at `low` or `none` carrying personal data into a target, `--strict-schema` becomes the headless default and this ADR is superseded.
- Keyring: if two dogfood sessions show `lazyslice.secret` committed by accident despite the `.gitignore` entry, the default store moves to the OS keyring on TTY hosts, the file stays as `--secret-file`, and CI keeps the environment variable.
- If the yml is hand-edited more often than regenerated, add `lazyslice explain --write`; never make the file a precondition.

## Revisions

2026-09-05, phase 2 adversarial review (T-0018), before Gate 2:
- Drift restated in the confidence scale ARCHITECTURE.md defines; the "none at certain" wording was unimplementable and contradicted ARCHITECTURE.md "Classification".
- `--plan` no longer writes the yml unless `--config PATH` is explicit, and then as `plan_only: true`.
- `keys:`, `skipped:`, `password_command` and `where_fingerprint` added to the emitted file so a run that needed a flag reproduces in CI; `where` literals withheld.
- Key distribution and rotation triggers stated; `K` named as secret material (THREAT_MODEL.md T13).
- `--unmask` requires a reason.
