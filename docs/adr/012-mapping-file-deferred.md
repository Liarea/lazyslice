# ADR-012: mapping_file is deferred past v1

Status: proposed, 2026-09-14. Supersedes the `mapping_file:` paragraph of ADR-006 for v1. ADR-006 is accepted and frozen (its own header: "After Gate 2 this file is frozen and a change is a superseding ADR"), so that paragraph is not edited; this ADR is the correction and takes effect on its own gate.

## Context

ADR-006's decision section named `mapping_file:` per column as one of two escape hatches for a unique column whose admissible domain is too small to mask safely (research/HARD_PROBLEMS.md §2.2 rule 4): a CSV of `original,replacement`, written by the user, read at transform, gitignored, protected against being tracked by git, and refusing at exit 12 on a value the mapping does not cover.

docs/reviews/2026-09-09/REVIEW.md finding 10 ("`mapping_file` is an advertised escape hatch without a consuming implementation") traced the whole tree and found the first two-thirds of that contract built and the third missing: `internal/pipeline/config.go:81` read the field into `ColumnConfig`, `internal/emit/document.go:293` round-tripped it on every re-run, and `internal/plan/unique.go:165` (with `internal/plan/equality.go`'s group form) recommended it by name in the unique-domain refusal — but no stage read the CSV or applied a replacement. `Decision` carries no mapping, `internal/transform` receives decisions and nothing else, and `internal/core/run.go:1539`'s call into `repo.Protect` passes `nil` for the mapping-paths argument. A column an operator gave a mapping file to was masked by its ordinary masker instead, silently: the yml said the escape was taken, the refusal it answered never fires twice to say otherwise, and the column that reached `mapping_file:` in the first place is exactly the one whose default masker was refused for being unsafe. This is the false-confidence failure THREAT_MODEL.md's masking rows exist to prevent, produced by the one field written to prevent it.

The review also names this as evidence "that the listed remaining hardening tasks do not exhaust the specified v1 work" — ARCHITECTURE.md §14's v1 cut line assumed `mapping_file:` was inside it because ADR-006 said so, and the gate this ADR is proposed under is the one that has to say it is not, before hardening closes.

## Options considered

- **Implement the full contract now**: read the CSV at transform, validate 1:1-ness and coverage against the plan's row count, wire FK consistency so both ends of an equality group use the same mapping, and pass the file's path into `repo.Protect`'s `mappingPaths` so it is gitignored and tracked-checked like the secret file. This is real, scoped work — a new transform input, a new refusal shape for missing values (exit 12, per ADR-006's revision), and a threat-model row for a file that is a source credential in another shape — not a fit for the task that found the gap.
- **Remove the field and its mentions everywhere**, retracting ADR-006's escape hatch rather than deferring it. Rejected: the escape hatch is real and research/HARD_PROBLEMS.md §2.2 rule 4 names it as one of exactly two for a domain too small to mask, the other being `--unmask`. A unique column with `--take` already at the floor and an operator unwilling to unmask it has no third option once this is gone, and CONCEPT.md's "safe by default" refusal (exit 12, not a silent copy) is the alternative to weakening the domain rule itself — not a reason to remove the one escape besides `--unmask` and a lower row count.
- **Refuse the field explicitly and leave everything else as found**, deferring the implementation to a named task while the config decoder, emit and the unique-domain refusal are corrected to stop asserting a contract that does not exist. This is what the review's own recommendation gives as the alternative to implementing it ("either implement and test the full mapping contract... or reject the field explicitly and remove the proposed remedy"), and it is scoped to what T-0138 can build and verify: three call sites and a read-time refusal, not a transform-stage feature.

## Decision

**`mapping_file:` is refused, not implemented, until T-0142.**

- A `lazyslice.yml` naming `mapping_file:` for any column exits 2 at read, naming the table and column and pointing at the remedy that actually clears the refusal: remove the `mapping_file:` line for that column from the file, then use `--unmask TABLE.COL=REASON` or a lower `--take`/`--cap` (`internal/emit`'s `*MappingFileError`, `internal/core`'s `config.refused.mapping_file`). Naming only the two escapes and not the file edit reintroduces the shape this ADR's own Decision below calls out as worse than printing nothing — an operator who follows the flag advice alone still reads the same `mapping_file:` line on the next run and exits 2 again. This applies to every yml lazyslice reads, including one written before this ADR that still carries the key from when emit wrote it.
- The unique-domain refusal (`internal/plan/unique.go`) and the equality-group refusal (`internal/plan/equality.go`) no longer recommend `mapping_file:` as an escape. Printing a remedy that exits 2 if taken is worse than not printing it: an operator who takes the advice loses a run to a usage error instead of being told the two escapes that work.
- `internal/emit` never writes `mapping_file:` into a fresh file. `pipeline.ColumnConfig` carries no field for it, so there is nothing to round-trip and nothing for the tighten-only merge (ADR-004) to carry forward.
- `internal/repo`'s protection of a named mapping-file path (`repo.Protect`'s `mappingPaths` argument, ARCHITECTURE.md §9, THREAT_MODEL.md T6) is unchanged. It already receives `nil` today, because nothing has ever collected a mapping path to pass it — this ADR does not add or remove that capability, only confirms the shape T-0142 fills in stays where it is.

**T-0142** implements the deferred contract: read the CSV at transform (a new `Constraints` input, since `internal/transform` currently receives decisions and nothing carrying original values), validate 1:1-ness and full coverage of the column's domain against the planned row count, apply the equality-group consistency ADR-006's revision implies (`mapping_file:` on one end of a foreign key and not the other still leaves the key unvalidatable, the same failure `internal/plan/equality.go`'s group escape exists to prevent), wire the file's path into `repo.Protect`'s `mappingPaths` so it is gitignored and tracked-checked, and restore `mapping_file:` to `pipeline.ColumnConfig`, `internal/emit`'s write and merge, and the two refusal messages this ADR strips it from. Until T-0142 lands, this ADR's refusal stands; T-0142's own change record is where the restored behaviour and its tests are documented, not here.

## Consequences

- A column whose only registered generator is too small for its row count has exactly one operator-facing escape in this build: `--unmask TABLE.COL=REASON`, or accepting a smaller slice through `--take`/`--cap`. A team unwilling to do either cannot slice that table until T-0142.
- A yml written by a build before this ADR that already carries `mapping_file:` (there was a merge test round-tripping it; no shipped release did, since v1 has not reached Gate 4) is refused at the next read, not silently accepted. This is the intended direction: refusing is what finding 10 asked for in place of the false confidence of accepting it.
- ARCHITECTURE.md §14's v1 cut line is owed the same correction finding 10 named — `mapping_file:` recorded as deferred rather than included — which is outside this task's paths and is filed below.

## Reversal condition

T-0142 lands and is verified: the CSV is read at transform, an absent value is exit 12 naming the column and the unmapped count (never a fallback to the refused masker, per ADR-006's own revision), the equality-group case is covered, and `repo.Protect` receives the path. At that point this ADR's refusal is superseded in the same commit that restores the field, and the superseding ADR says so.

## Filed

**T-0169** (epic E9): ARCHITECTURE.md §14's v1 cut line lists `mapping_file` inside v1 on ADR-006's word; this ADR defers it to T-0142, and §14 needs the same correction docs/reviews/2026-09-09/REVIEW.md finding 10 asked for. Outside this task's paths (ARCHITECTURE.md), so filed rather than edited here.
