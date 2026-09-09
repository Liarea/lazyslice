# ADR-011: The unique-index domain rule for composite and partial indexes

Status: accepted, 2026-09-08. Authorises the rules T-TORTURE had to decide (T-0099, T-0107).

## Context

ARCHITECTURE.md §5 states the unique-index rule for a single-column unique index: a masked column under one needs a generator whose domain satisfies d_required = n² / 2ε, the plan picks the widest registered generator in the category, and refuses at exit 12 when none fits. Real schemas in the torture suite carry two shapes §5 does not cover, and without a rule the loader died on them: composite unique indexes with a masked column among several, and single-column partial unique indexes.

## Decision

(a) A composite unique index raises every masked column it covers to the unique-domain rule, unless an unmasked key column's sample shows no repeated value, in which case the unmasked column already guarantees uniqueness and the masked columns are not raised. This is an approximation on the safe side: the true rule concerns the product of the masked columns' domains against the largest group of rows that agree on the unmasked columns, a statistic nothing collects yet.

(b) A single-column partial unique index raises its column, and d_required is computed over the whole table's row count. This over-estimates, because the predicate admits fewer rows, which is the safe direction.

Both live in internal/classify (raiseCompositeUnique, indexKeys) and feed internal/plan's exit-12 refusal, internal/transform's uniqueness constraint, and the emitted config. §5 is amended to state them.

## Consequences

A few columns are refused or given a wider generator than strictly necessary. No column that needs the rule escapes it.

## Reversal condition

Introspect collects the agreeing-group statistic for composite keys, at which point (a) becomes exact and a superseding ADR says so.
