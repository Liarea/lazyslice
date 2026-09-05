# ADR-006: Maskers are a library, rules are data, nothing is loaded at runtime

Status: accepted, 2026-09-05. Revised 2026-09-05 in the phase 2 adversarial review (tracker T-0018), before Gate 2, which is the one revision the phase workflow allows; the "Revisions" section at the end lists what changed and why. After Gate 2 this file is frozen and a change is a superseding ADR.

## Context

docs/BUILD_PLAN.md recommends "masking is pluggable, classification is not" and PROMPT 2.4 asks us to pick Go plugin, subprocess or expression language for user maskers; PROMPT 2.5 names "a malicious custom masker" as a threat. CONCEPT.md "Safe by default" refuses any mode that copies an unclassified column as-is, and ADR-007 requires the deterministic masker to be a separately importable module so it outlives the tool, as copycat outlived Snaplet at 121,478 weekly downloads (research/SYNTHESIS.md §1 fact 4; research/POSTMORTEMS.md §11 item 1). research/COMPLAINTS.md CB-3 ("client had no proper idea of what the field were") is the argument that value shape is local knowledge. research/HARD_PROBLEMS.md §2.2 rule 4 names two escape hatches for a unique column whose admissible domain is too small to mask safely: opt the column out, or supply an explicit 1:1 mapping.

All three proposals reject runtime code loading in v1 (research/proposals/mvp-first.md "D6", research/proposals/risk-first.md "6", research/proposals/user-first.md "6"). They differ on whether a compiled-in registry counts as "pluggable", on the module's path, and on whether the 1:1 mapping ships.

## Options considered

- **Go plugins (`.so`).** Breaks the static binary and `CGO_ENABLED=0`. Rejected on ADR-001 alone.
- **Subprocess maskers.** A masker sees every personal value; a subprocess is code we did not review holding production data, and it turns "no pass-through mode" into a pass-through mode with extra steps.
- **Expression language or WASM sandbox.** An interpreter is a week we do not have before Gate 4, and a sandbox is a v2 item with a threat-model row.
- **Compiled-in registry in a separate module, extension through data.** Users pick shipped maskers per column in the yml, supply a 1:1 mapping file for the one case the research names, and add classifier patterns that can only raise. Adding a masker means a pull request to the `mask` module or a build with `mask.Register`.

## Decision

**Maskers are a library.** The masker is a nested Go module, `github.com/Liarea/lazyslice/mask`, in this repository with its own `go.mod`, its own tests and its own release tags (`mask/v0.x`). It depends only on the standard library, `golang.org/x/text` v0.41.0 and `github.com/nyaruka/phonenumbers` v1.8.1. One repository, because research/LICENSE_DECISION.md's recommendation is one repo and no `ee/` directory; a nested module gives the separate import path and release cadence ADR-007 asks for without a second repository to abandon. A masker is a pure function `func(h [32]byte, in Value, c Constraints) (Value, error)`; the registry is populated at build time by `mask.Register`, and the built-in set covers every v1 category in ARCHITECTURE.md "Classification".

**Extension happens in `lazyslice.yml`, as data:**
- `masker:` per column chooses among shipped maskers by name (`email`, `phone`, `person_name`, `fixed:REDACTED`, `null`, and the rest of the registry).
- `mapping_file:` per column names a CSV of `original,replacement` for a column refused under a unique index (research/HARD_PROBLEMS.md §2.2 rule 4; research/proposals/user-first.md "6"). The file is written by the user, read at transform, must be gitignored (the tool appends the path to `.gitignore` when the yml first names it and refuses to run if the file is tracked by git), and is listed as an asset in THREAT_MODEL.md (A6) because it contains original values in plaintext. **Distribution is the problem the gitignore rule does not solve, and this ADR says so:** the snapshot reproduces only where the file is, and the file must therefore reach every laptop and CI job that runs the yml — the audience the snapshot is built for, which THREAT_MODEL.md A6 says must not receive original values without source access. The position is that a mapping file is a source credential in another shape: it is distributed by hand or through the same secret store as `LAZYSLICE_SECRET`, never through the repository, and only to people who could be given source access; a team that cannot accept that uses `--unmask` with a reason or lowers the row count instead. **A value absent from the mapping is a named error, exit 12, naming the column and the count of unmapped values**, never a fallback to the column's default masker: the column reached `mapping_file:` because that masker's domain was refused at plan, so falling back to it would produce at load exactly the collision the refusal existed to prevent, with a `PgError.Detail` we drop. The earlier "masked by the column's default masker" wording is withdrawn.
- `classify.extra_patterns:` adds name patterns, dictionaries and validator bindings; a pattern may add a category or raise a confidence and can never lower or remove one. `TestConfigCannotLowerConfidence` fails otherwise.

**The classifier is not pluggable**, by design and not by omission: a pluggable classifier is a supported way to see less personal data, which is the mode CONCEPT.md refuses (research/proposals/user-first.md "6"). The rule pack is an embedded YAML file in `internal/classify`; contributions to it are pull requests.

**Nothing is loaded at runtime.** No `.so`, no subprocess, no Lua, no WASM, no expression language, no `--masker-exec`. The binary's only I/O is the two databases, the Docker socket, the files it names, and the terminal (THREAT_MODEL.md T4).

Where the judges disagreed: risk-first's section heading said maskers are "pluggable in-process" while its mechanism was build-time registration, which the third judge called a blemish; this ADR calls the mechanism what it is. user-first's "stdlib only" claim conflicted with its use of phonenumbers; the dependency list above is the honest one.

## Consequences

- `github.com/Liarea/lazyslice/mask` can be imported by a Go program that has never heard of lazyslice, and its tests run without a database.
- A category with no shipped masker cannot exist: adding a category to the rule pack requires a masker in the registry, enforced by a test that walks both.
- Feature requests for a custom masker go to the tracker with the reversal counter below, not into the release.
- The masker's determinism scheme (ARCHITECTURE.md "Deterministic masking"), including the length-prefixed HMAC input encoding and each generator's declared `Domain()`, is part of the module's public contract; changing it is a major version of the module.

## Reversal condition

Three independent requests, from users we do not know, for a masker we will not ship and cannot express as a mapping file, reopen the question as a v2 WASM sandbox with the same pure-function signature, fed canonical bytes, with a THREAT_MODEL.md row before the first line of code. Still no classifier plugin; that has no reversal condition.

## Revisions

2026-09-05, phase 2 adversarial review (T-0018), before Gate 2:
- `mapping_file:` distribution stated as a source-credential-shaped problem rather than left implied by a gitignore rule; an absent mapping entry is exit 12, not a fallback to the refused masker.
- The HMAC input encoding and `Domain()` named as part of the module contract.
