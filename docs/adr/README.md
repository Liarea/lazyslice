# Architecture decision records

Decisions live here so they survive session compaction, model changes and contributors who were not present. CLAUDE.md's rule: to change a decision, add a new ADR that supersedes the old one; never edit an accepted ADR.

## Format

One file per decision, `NNN-short-name.md`, numbered in the order they were opened. Every ADR has these sections, in this order:

- **Status** line: `accepted, YYYY-MM-DD`, `superseded by ADR-NNN, YYYY-MM-DD`, or `proposed`.
- **Context.** What forced the decision, citing the research documents by path and heading and the proposals by path and section. A claim without a citation is an opinion and is labelled as one.
- **Options considered.** Each with its cost. An option nobody argued for is not listed.
- **Decision.** What we do, in enough detail to build against. Where the judges or proposals disagreed, the ADR says which side it took and why.
- **Consequences.** What becomes true, what becomes impossible, what other documents change.
- **Reversal condition.** An observable trigger, not a feeling. "If users complain" is not a reversal condition; "if two dogfood sessions show X" is. An ADR that cannot be reversed says so.

Phase gates check that every ADR has a stated reversal condition (docs/BUILD_PLAN.md Gate 2).

## Index

| ADR | Title | Status |
|---|---|---|
| [000](000-name.md) | The project is named lazyslice | accepted 2026-09-05 |
| [001](001-language.md) | Go, one static binary, no cgo | accepted 2026-09-05 |
| [002](002-tui.md) | Bubble Tea v2, but the default is a line printer | accepted 2026-09-05 |
| [003](003-database-order.md) | PostgreSQL 14 to 18, one adapter, one definition of done, then nothing until the gate | accepted 2026-09-05 |
| [004](004-config-model.md) | Configuration is emitted after a run, never required before it, and can only tighten | accepted 2026-09-05 |
| [005](005-pipeline.md) | Nine stages, one core, one event channel, and the safety controls that live in them | accepted 2026-09-05 |
| [006](006-extension-model.md) | Maskers are a library, rules are data, nothing is loaded at runtime | accepted 2026-09-05 |
| [007](007-tool-not-company.md) | v1 is a tool, not a company | accepted 2026-09-05 |
| [011](011-unique-index-domain-rule.md) | Unique-index domain rule for composite and partial indexes | accepted 2026-09-08 |
| [012](012-mapping-file-deferred.md) | mapping_file is deferred past v1 | accepted 2026-09-22 |
| [010](010-type-gate-on-value-signals.md) | The accepted-types gate silences value signals too; derived_text for tsvector | accepted 2026-09-06 |
| [009](009-schema-fingerprint.md) | One definition of the schema fingerprint: the generated DDL text | accepted 2026-09-06 |
| [008](008-first-run.md) | First-run experience: the discovery ladder, the one-question rule and the question catalogue | accepted 2026-09-08 |
| [013](013-headless-target-never-on-the-source-cluster.md) | A headless run's target is never chosen on the source's own cluster | accepted 2026-09-22 |
| [014](014-question-mark-at-q2-prints-and-tui-stays-a-flag.md) | `?` at the root-table question prints the ranked candidates; `--tui` stays the only way into the two screens | accepted 2026-09-22 |
| [015](015-residual-hit-in-the-name-vocabulary-is-explained-not-probed.md) | A residual hit inside the name masker's own vocabulary is explained by count and row identity, not confirmed by a column probe | proposed 2026-09-22 |
| [016](016-a-committed-target-container-is-the-container.md) | A committed target record of lazyslice's own container names the container, not its port | proposed 2026-09-24 |

## Numbering note

docs/BUILD_PLAN.md's phase 2 prompts numbered the ADRs differently (003 classifier, 004 masking, 006 first run). The numbering here follows the phase 2 gate's list of six decisions: language, TUI, database order, config model, pipeline, extension model. The classifier and masking designs live in ARCHITECTURE.md §4 to §6 under ADR-005 and ADR-006; the first run is ADR-008.

## Where things are decided

- What we build and why: CONCEPT.md.
- How we work: docs/OPERATING_MODEL.md.
- Interfaces, types, algorithms, flags, layout, dependencies: ARCHITECTURE.md.
- Threats and the controls that block v1: THREAT_MODEL.md.
- Anything not yet decided: research/OPEN_QUESTIONS.md, then the tracker.

## Errata (frozen ADRs are not edited; corrections live here)

- ADR-008 names `pipeline.Provisioner`. The interface was never implemented; the type that exists is `provision.Provisioner` in internal/discover/provision (T-0071, 2026-09-08). Behaviour as described is unchanged.
- ADR-005's exit-code table lists four exit-12 causes; T-PIN (2026-09-08) added a fifth, `core.refused.reviewed_changed`: the writing run's snapshot or endpoints differ from the ones the operator reviewed in `--tui`.
